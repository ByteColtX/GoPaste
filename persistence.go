package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"gopaste/internal/capture"
	"gopaste/internal/clipboard"
	"gopaste/internal/history"
	"gopaste/internal/history/sqlite"
)

// appDataDirectory 返回 GoPaste 在本机的数据目录，也就是历史数据库与设置文件所在
// 的那一层。两个文件共用它，应用文件夹的名字只写在这里。
func appDataDirectory() (string, error) {
	base, err := userDataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "GoPaste"), nil
}

// historyDatabasePath 返回历史数据库在本机的默认位置。
//
// 剪贴板内容属于敏感数据：数据库只放在当前用户的数据目录，不进仓库、不同步、
// 不上传。三个平台用同一套「GoPaste/history.db」相对路径，本机预览数据脚本
// 因此不必重复判断平台。
func historyDatabasePath() (string, error) {
	dir, err := appDataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.db"), nil
}

// settingsPath 返回设置文件在本机的默认位置，与历史数据库同目录。
func settingsPath() (string, error) {
	dir, err := appDataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// deviceDatabasePath 返回设备配对信息在本机的位置。它和历史数据库在同一个用户
// 数据目录下：文件里是本机私钥和钉住的公钥，因此和历史记录一样只属于当前用户。
func deviceDatabasePath() (string, error) {
	base, err := userDataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "GoPaste", "devices.json"), nil
}

// receivedFilesDirectory 返回接收到的同步文件落盘的根目录。每条记录一个以内容 ID
// 命名的子目录，所以同名文件不会互相覆盖；删除记录时整个目录一起清掉。
func receivedFilesDirectory() (string, error) {
	dir, err := appDataDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Files"), nil
}

// userDataDirectory 返回当前用户存放应用数据的基础目录。Go 标准库只有配置目录
// 和缓存目录，历史记录既不是配置也不能被系统清理，所以按平台约定自行拼出数据
// 目录。
func userDataDirectory() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return dir, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Local"), nil
	default:
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return dir, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}

// historyWrite 是一次仓库写入。写操作拿到仓库而不是自己持有它，是为了让所有
// 写入都在同一个 goroutine 里按入队顺序执行。
type historyWrite func(ctx context.Context, repository history.Repository) error

// writeQueueDepth 是待写队列的长度。历史写入又小又少，队列只用来吸收突发操作。
const writeQueueDepth = 64

// historyPersistence 把内存里的历史 Store 和 SQLite 仓库连起来。
//
// 写入全部排进一条队列，由唯一一个 goroutine 依次执行：界面线程不碰磁盘，
// 数据库里的顺序也和用户操作的顺序一致，例如删除一条记录和同一内容再次被
// 捕获不会互相覆盖。界面线程负责内存中的 Store，仓库只负责落盘。
type historyPersistence struct {
	repository history.Repository
	writes     chan historyWrite
	done       chan struct{}
	closeOnce  sync.Once
	mu         sync.Mutex
	closing    bool
}

// newHistoryPersistence 接管仓库并启动唯一的写入 goroutine。
func newHistoryPersistence(repository history.Repository) *historyPersistence {
	p := &historyPersistence{
		repository: repository,
		writes:     make(chan historyWrite, writeQueueDepth),
		done:       make(chan struct{}),
	}
	go p.run()
	return p
}

// enqueue 安排一次写入。关闭之后调用是空操作；队列排满时调用方会短暂等待，
// 这比丢掉一次写入要好。
func (p *historyPersistence) enqueue(write historyWrite) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return
	}
	p.writes <- write
}

// Close 等待已排队的写入落盘，然后关闭仓库。可以重复调用。
func (p *historyPersistence) Close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closing = true
		close(p.writes)
		p.mu.Unlock()
		<-p.done
	})
}

// run 是唯一的仓库写入者。失败只记录错误本身：剪贴板内容不写日志。
func (p *historyPersistence) run() {
	defer close(p.done)
	ctx := context.Background()
	for write := range p.writes {
		if err := write(ctx, p.repository); err != nil {
			log.Printf("历史记录写入失败: %v", err)
		}
	}
	if err := p.repository.Close(); err != nil {
		log.Printf("关闭历史数据库失败: %v", err)
	}
}

// putItem 是一次「新增或替换一条记录」。
func putItem(item history.Item) historyWrite {
	return func(ctx context.Context, repository history.Repository) error {
		return repository.Put(ctx, item)
	}
}

// deleteItems 是一次「删除若干条记录」。
func deleteItems(ids ...string) historyWrite {
	return func(ctx context.Context, repository history.Repository) error {
		return repository.Delete(ctx, ids...)
	}
}

// persistItem 把一条记录的新状态排进写队列。应用没有接入持久化时什么都不做。
func (a *application) persistItem(item history.Item) {
	if a.persistence == nil {
		return
	}
	a.persistence.enqueue(putItem(item))
}

// persistDelete 把若干条记录的删除排进写队列。
func (a *application) persistDelete(ids ...string) {
	if a.persistence == nil || len(ids) == 0 {
		return
	}
	a.persistence.enqueue(deleteItems(ids...))
}

// databaseFile 返回本次运行要用的历史数据库文件。
//
// 命令行参数和环境变量指定的文件优先，本机预览因此可以把应用指向一份生成的
// 演示数据；两者都没有时才用用户数据目录下的默认位置。
func (r *applicationRuntime) databaseFile() (string, error) {
	if r.databasePath != "" {
		return r.databasePath, nil
	}
	return historyDatabasePath()
}

// startHistoryPersistence 打开历史数据库、装载已有记录，然后开始监听剪贴板。
//
// 打开数据库、读取记录都在界面线程之外进行，结果通过 Window.Update 发布：界面
// 在数据到达前显示空状态。任何一步失败都只记录日志，应用继续以不落盘的方式
// 运行；此时剪贴板监听也不启动，避免用户以为复制的内容被保存了。
func (r *applicationRuntime) startHistoryPersistence(captureCtx context.Context) {
	go func() {
		path, err := r.databaseFile()
		if err != nil {
			log.Printf("无法确定历史数据库位置，本次不保存历史记录: %v", err)
			return
		}
		// 启动第一行就报出用的是哪个文件：界面里没有数据时，第一个要问的就是它。
		log.Printf("历史数据库：%s", path)
		// 目录只给当前用户，数据库里存的是剪贴板内容。
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			log.Printf("无法创建历史数据库目录，本次不保存历史记录: %v", err)
			return
		}
		ctx := context.Background()
		repository, err := sqlite.Open(ctx, path)
		if err != nil {
			log.Printf("无法打开历史数据库 %s，本次不保存历史记录: %v", path, err)
			return
		}
		items, err := repository.Load(ctx)
		if err != nil {
			// 仓库本身可用，只是这次没读出来：仍然接入持久化，从空列表开始。
			log.Printf("无法读取历史记录: %v", err)
			items = nil
		}

		persistence := newHistoryPersistence(repository)
		installed := make(chan struct{})
		r.window.Update(func() {
			r.app.installHistory(items, persistence)
			close(installed)
		})
		// 等装载结果落到界面状态之后再开始捕获，否则启动瞬间捕获到的内容会被
		// 装载结果覆盖掉。
		<-installed
		r.startClipboardCapture(captureCtx)
	}()
}

// startClipboardCapture 开始监听系统剪贴板，把新复制的内容并入历史记录。
//
// 监听只读剪贴板，不写入、不联网：捕获结果先落到内存里的 Store，再排进写队列。
// 本次构建不监听时直接返回，历史只来自已装载的数据库。
func (r *applicationRuntime) startClipboardCapture(ctx context.Context) {
	if !clipboardCaptureEnabled() {
		log.Printf("预览构建不监听剪贴板，历史只来自预览数据库")
		return
	}
	source, err := clipboard.New()
	if err != nil {
		log.Printf("剪贴板不可用，本次不自动记录复制内容: %v", err)
		return
	}
	capturer := capture.New(source, source, capture.Options{})
	go func() {
		if err := capturer.Loop(ctx, r.captured); err != nil {
			log.Printf("剪贴板监听已停止: %v", err)
		}
	}()
}

// captured 在捕获 goroutine 上收到一次读数，把结果发布到界面线程。
// Store 只在界面线程上改动，落盘则排进写队列，两边都不阻塞。
func (r *applicationRuntime) captured(snapshot capture.Snapshot) {
	item, ok := snapshot.Item()
	if !ok {
		return
	}
	r.window.Update(func() {
		r.app.appendCaptured(item)
	})
}

// stopCapture 停止剪贴板监听。捕获 goroutine 会随上下文结束退出。
func (r *applicationRuntime) stopCapture() {
	if r.captureCancel != nil {
		r.captureCancel()
		r.captureCancel = nil
	}
}

// installHistory 用持久化装载的结果替换内存中的历史记录，并接入后续写入。
// 装载只发生一次，替换后选中项回到最新一条。
//
// 装载完成之前收到的同步内容排在这里补上：那一刻内存里的历史是空的，替换之后它们
// 就只剩下剪贴板里的那一份，数据库里永远不会有。
func (a *application) installHistory(items []history.Item, persistence *historyPersistence) {
	a.history = history.New(items)
	a.persistence = persistence
	a.historyBitmaps = nil
	a.codeHighlights = nil
	a.historyReady = true
	for _, item := range a.pendingClips {
		if err := a.history.Append(item); err != nil {
			continue
		}
		a.persistItem(item)
	}
	a.pendingClips = nil
	// 装载完就先裁一次：升级后第一次启动，超过上限的旧记录在这一刻消失。
	a.pruneHistory()
}

// appendCaptured 把一次捕获结果并入历史记录。
//
// 同 ID 的内容已经在列表里时（重复复制，或应用自己刚写到剪贴板的内容）什么
// 都不做：Store 只保留一份，数据库也不重复写。
func (a *application) appendCaptured(item history.Item) {
	if a.consumeClipboardEcho(item.ID) {
		return
	}
	if err := a.history.Append(item); err != nil {
		return
	}
	a.persistItem(item)
	// 剪贴板只有最新一条有意义：新的内容进来后，一条都没送到的旧记录不再自动
	// 重试，界面上的重发按钮仍然可以手动补发。
	a.markSuperseded(item.ID)
	if a.syncService != nil {
		a.syncService.Send(item)
	}
	// 每来一条新内容就裁一次：条数上限靠它维持。
	a.pruneHistory()
}

// recordClipboardWrite 记下应用自己写到剪贴板的读数 ID，让紧接着的捕获忽略它。
//
// Store.Copy 对富文本和文件只能写回纯文本表示，剪贴板读回来的内容因此和原记录
// 的 ID 不同。这里按「写出去的内容会读回什么」预先算出 ID，捕获到同样的读数时
// 直接丢弃，避免复制一条记录就多出一条纯文本记录。
func (a *application) recordClipboardWrite(item history.Item) {
	echo, ok := clipboardEchoOf(item)
	if !ok {
		return
	}
	a.clipboardEcho = map[string]struct{}{echo.ID: {}}
}

// recordClipboardFiles 记下写进剪贴板的一批文件会读回什么，作用同
// recordClipboardWrite。接收同步文件时剪贴板里是文件本身而不是路径文本，所以回声
// 要按文件读数算：本地路径算出的 ID 与收到的条目 ID 不同，两个都要认得。
func (a *application) recordClipboardFiles(itemID string, paths []string) {
	a.clipboardEcho = map[string]struct{}{itemID: {}}
	if echo, ok := (capture.Snapshot{Kind: kindFile, Files: paths}).Item(); ok {
		a.clipboardEcho[echo.ID] = struct{}{}
	}
}

// recordClipboardText 记下应用直接写到剪贴板的一段文本，作用同 recordClipboardWrite。
// 用于没有整条记录可依据的复制入口：预览区里点单个文件路径就是这种，它写出去的
// 文本会被捕获路径当成一次新复制。
func (a *application) recordClipboardText(text string) {
	a.recordClipboardWrite(history.Item{Kind: kindText, Text: text})
}

// clipboardEchoOf 返回把一条记录写到剪贴板之后，捕获路径会读到的那条记录。
// 图片按原样写回，其余类型都只能写回 Text。
func clipboardEchoOf(item history.Item) (history.Item, bool) {
	if item.Kind == kindImage {
		return capture.Snapshot{Kind: kindImage, Image: item.Image}.Item()
	}
	return capture.Snapshot{Kind: kindText, Text: item.Text}.Item()
}

// consumeClipboardEcho 报告一次捕获是否正是应用自己写出去的内容，并清除标记。
// 标记只认最近一次写出，用户随后复制别的内容不会受到影响。
func (a *application) consumeClipboardEcho(id string) bool {
	if _, ok := a.clipboardEcho[id]; !ok {
		return false
	}
	delete(a.clipboardEcho, id)
	return true
}
