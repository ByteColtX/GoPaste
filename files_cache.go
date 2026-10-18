package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopaste/internal/filescache"
	"gopaste/internal/history"
)

// 收到的文件缓存：按保留期限与配额决定删哪些记录。挑选规则在
// internal/filescache 里，这里只负责从历史里收集条目、把测量放到界面线程之外、
// 再回到界面线程删记录。

// receivedFileEntries 把历史里来自设备的文件条目投影成清理任务要看的条目。目录
// 从条目自己的路径反推，所以换过保存位置之后，旧目录里的文件照样算得进来。
func (a *application) receivedFileEntries() []filescache.Entry {
	items := a.history.Snapshot(history.Query{}).Items
	entries := make([]filescache.Entry, 0, len(items))
	for _, item := range items {
		dir, ok := receivedFilesPath(item)
		if !ok {
			continue
		}
		entries = append(entries, filescache.Entry{ID: item.ID, Dir: dir, ReceivedAt: item.CreatedAt})
	}
	return entries
}

// fileCacheRoots 返回收到的文件可能落在的所有根目录：当前生效的那个，加上已有
// 记录反推出的那些。用户换过保存位置之后，旧目录里的文件仍然算进用量。
func (a *application) fileCacheRoots() []string {
	roots := make([]string, 0, 2)
	seen := map[string]struct{}{}
	add := func(root string) {
		if root == "" {
			return
		}
		if _, ok := seen[root]; ok {
			return
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}
	add(a.receivedFilesDir)
	for _, entry := range a.receivedFileEntries() {
		// 记录目录是 <根目录>/<内容ID>，往上一层就是根目录。
		if root := filepath.Dir(entry.Dir); root != entry.Dir {
			add(root)
		}
	}
	return roots
}

// fileCachePolicy 返回当前生效的清理规则：配额按字节，期限按小时换算，0 表示
// 不限。
func (a *application) fileCachePolicy() filescache.Policy {
	return filescache.Policy{
		QuotaBytes: a.fileQuotaBytes,
		MaxAge:     time.Duration(a.fileRetentionHours) * time.Hour,
	}
}

// requestFileCacheCleanup 请运行时在界面线程之外测一遍缓存，再按规则挑出要删的
// 记录。
//
// 「自动清理」管的是保留期限：它关着时过期文件不自动删，但配额照旧守住——配额
// 是用户自己设的上限，不是一个提醒。手动「立即清理」（force）把两条规则都跑一遍。
//
// 同一时刻只有一次清理在飞：连点按钮、或者刚收完一个文件又改了期限，重复请求
// 直接忽略。
func (a *application) requestFileCacheCleanup(force bool) {
	if a.onCleanFileCache == nil || a.fileCachePending {
		return
	}
	policy := a.fileCachePolicy()
	if !force && !a.fileAutoClean {
		policy.MaxAge = 0
	}
	entries := a.receivedFileEntries()
	if len(entries) == 0 {
		if force {
			a.pendingToast = a.t(msgToastFileCacheNothing)
		}
		return
	}
	a.fileCachePending = true
	a.fileCacheManual = force
	a.onCleanFileCache(entries, policy)
}

// applyFileCacheCleanup 把一次清理的结果落进历史：删掉挑中的记录，再让文件在后台
// 跟着消失。它只在界面线程上调用，运行时测完经 Window.Update 回到这里。
func (a *application) applyFileCacheCleanup(removed []string) {
	a.fileCachePending = false
	manual := a.fileCacheManual
	a.fileCacheManual = false
	if len(removed) == 0 {
		if manual {
			a.pendingToast = a.t(msgToastFileCacheNothing)
		}
		return
	}
	dirs := make([]string, 0, len(removed))
	for _, id := range removed {
		item, ok := a.history.Item(id)
		if !ok {
			continue
		}
		if dir, ok := receivedFilesPath(item); ok {
			dirs = append(dirs, dir)
		}
		if err := a.history.Delete(id); err != nil {
			continue
		}
		delete(a.historyBitmaps, id)
		delete(a.codeHighlights, id)
		a.persistDelete(id)
	}
	if len(dirs) > 0 {
		// 删目录可能不小，放到界面线程之外。
		go func() {
			for _, dir := range dirs {
				_ = os.RemoveAll(dir)
			}
		}()
	}
	a.pendingToast = a.l10n().fileCacheCleaned(len(removed))
	// 文件刚被删掉，用量要等它们真的没了再测，否则刚清掉的那一份还会被算进去。
	a.requestStorageUsage(nil)
}

// receivedFilesPath 返回一条收到的文件记录在本机落盘的目录。只有来自设备的文件
// 条目才有这个目录：本机捕获的文件项只是指向用户自己的文件，不能删。
//
// 目录从条目自己的路径反推——沿着第一条本地路径往上找到 basename 等于内容 ID 的
// 那一层——而不是拼当前根目录：用户换过保存位置之后，旧目录里的记录照样能被删掉。
func receivedFilesPath(item history.Item) (string, bool) {
	if item.Kind != kindFile || item.SourceDeviceID == "" {
		return "", false
	}
	if item.ID == "" || strings.ContainsAny(item.ID, `/\`) {
		return "", false
	}
	first, _, _ := strings.Cut(item.Text, "\n")
	if first == "" {
		return "", false
	}
	for dir := filepath.Dir(first); ; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == item.ID {
			return dir, true
		}
		if parent := filepath.Dir(dir); parent == dir {
			return "", false
		}
	}
}
