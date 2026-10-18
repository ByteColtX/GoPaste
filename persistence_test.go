package main

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"gopaste/internal/capture"
	"gopaste/internal/history"
	"gopaste/internal/history/sqlite"

	"github.com/egoist/mygo/ui"
)

// recordingRepository 记录写操作的顺序，用来验证写队列串行且保序。
type recordingRepository struct {
	mu     sync.Mutex
	ops    []string
	closed bool
}

func (r *recordingRepository) record(op string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ops = append(r.ops, op)
}

func (r *recordingRepository) Load(context.Context) ([]history.Item, error) { return nil, nil }

func (r *recordingRepository) Put(_ context.Context, items ...history.Item) error {
	for _, item := range items {
		r.record("put:" + item.ID)
	}
	return nil
}

func (r *recordingRepository) Delete(_ context.Context, ids ...string) error {
	for _, id := range ids {
		r.record("delete:" + id)
	}
	return nil
}

func (r *recordingRepository) Close() error {
	r.record("close")
	return nil
}

func (r *recordingRepository) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.ops...)
}

// TestHistoryPersistenceKeepsWriteOrder 检查写队列按入队顺序串行落盘，
// 并在 Close 时先写完再关闭仓库。
func TestHistoryPersistenceKeepsWriteOrder(t *testing.T) {
	repository := &recordingRepository{}
	persistence := newHistoryPersistence(repository)
	persistence.enqueue(putItem(history.Item{ID: "a"}))
	persistence.enqueue(deleteItems("a"))
	persistence.enqueue(putItem(history.Item{ID: "b"}))
	persistence.Close()

	want := []string{"put:a", "delete:a", "put:b", "close"}
	if got := repository.recorded(); !slices.Equal(got, want) {
		t.Errorf("写入顺序 = %v，期望 %v", got, want)
	}
}

// TestHistoryPersistenceIgnoresWritesAfterClose 检查关闭之后的写入是空操作，
// 重复关闭也不会 panic。
func TestHistoryPersistenceIgnoresWritesAfterClose(t *testing.T) {
	repository := &recordingRepository{}
	persistence := newHistoryPersistence(repository)
	persistence.Close()
	persistence.enqueue(putItem(history.Item{ID: "late"}))
	persistence.Close()

	if got := repository.recorded(); !slices.Equal(got, []string{"close"}) {
		t.Errorf("关闭后的写操作 = %v，期望只有 close", got)
	}
}

// TestHistoryDatabasePathStaysInTheUserDataDirectory 检查数据库固定落在
// 当前用户的数据目录里，而不是仓库或共享位置。
func TestHistoryDatabasePathStaysInTheUserDataDirectory(t *testing.T) {
	path, err := historyDatabasePath()
	if err != nil {
		t.Fatalf("historyDatabasePath() 出错: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("GoPaste", "history.db")) {
		t.Errorf("historyDatabasePath() = %q，期望以 GoPaste/history.db 结尾", path)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("historyDatabasePath() = %q，期望绝对路径", path)
	}
}

// TestSettingsFileSitsBesideTheHistoryDatabase 检查设置文件与历史数据库同目录：
// 两者都属于本机应用数据，路径拼接只应有一处。
func TestSettingsFileSitsBesideTheHistoryDatabase(t *testing.T) {
	settings, err := settingsPath()
	if err != nil {
		t.Fatalf("settingsPath() 出错: %v", err)
	}
	database, err := historyDatabasePath()
	if err != nil {
		t.Fatalf("historyDatabasePath() 出错: %v", err)
	}
	if got, want := filepath.Dir(settings), filepath.Dir(database); got != want {
		t.Errorf("设置文件目录 = %q，期望与数据库同目录 %q", got, want)
	}
	if filepath.Base(settings) != "settings.json" {
		t.Errorf("settingsPath() = %q，期望以 settings.json 结尾", settings)
	}
}

// TestParseArgsReadsTheDatabasePath 检查历史数据库文件既可以由参数指定，也可以
// 由环境变量提供，而且参数优先。
func TestParseArgsReadsTheDatabasePath(t *testing.T) {
	t.Setenv(historyDatabaseEnv, "/tmp/from-env.db")

	if got, rest := parseArgs(nil); got != "/tmp/from-env.db" || len(rest) != 0 {
		t.Errorf("没有参数时 parseArgs() = %q/%v，期望环境变量的值", got, rest)
	}
	if got, _ := parseArgs([]string{"-db", "/tmp/from-flag.db"}); got != "/tmp/from-flag.db" {
		t.Errorf("参数指定的路径被忽略了，parseArgs() = %q", got)
	}
}

// TestParseArgsLeavesMygoArgumentsAlone 检查留给 mygo 的参数原样保留：它把
// 剩下的参数当成要打开的链接和文件。
func TestParseArgsLeavesMygoArgumentsAlone(t *testing.T) {
	_, rest := parseArgs([]string{"-db", "preview.db", "gopaste://open?x=1"})
	if want := []string{"gopaste://open?x=1"}; !slices.Equal(rest, want) {
		t.Errorf("剩下的参数 = %v，期望 %v", rest, want)
	}
}

// TestDatabaseFilePrefersTheExplicitPath 检查命令行参数或环境变量指定的数据库
// 优先于用户数据目录下的默认位置。
func TestDatabaseFilePrefersTheExplicitPath(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "preview.db")
	got, err := newApplicationRuntime(explicit).databaseFile()
	if err != nil {
		t.Fatalf("databaseFile() 出错: %v", err)
	}
	if got != explicit {
		t.Errorf("指定文件时 databaseFile() = %q，期望 %q", got, explicit)
	}

	fallback, err := newApplicationRuntime("").databaseFile()
	if err != nil {
		t.Fatalf("databaseFile() 出错: %v", err)
	}
	want, err := historyDatabasePath()
	if err != nil {
		t.Fatalf("historyDatabasePath() 出错: %v", err)
	}
	if fallback != want {
		t.Errorf("未指定文件时 databaseFile() = %q，期望 %q", fallback, want)
	}
}

// TestClipboardEchoFollowsWhatTheCopyPathWrites 检查回声标记记录的是「写回
// 剪贴板之后监听会读到的那条记录」：纯文本、链接、代码和图片写回原样，
// 富文本和文件只能写回纯文本表示。
func TestClipboardEchoFollowsWhatTheCopyPathWrites(t *testing.T) {
	image := &history.Image{PNG: []byte("not-a-real-png"), Width: 4, Height: 3}
	richText, ok := capture.Snapshot{Kind: kindRichText, Text: "你好", HTML: "<b>你好</b>"}.Item()
	if !ok {
		t.Fatal("富文本读数没有生成记录")
	}
	file, ok := capture.Snapshot{Kind: kindFile, Files: []string{"/tmp/a.txt"}}.Item()
	if !ok {
		t.Fatal("文件读数没有生成记录")
	}
	text, ok := capture.Snapshot{Kind: kindText, Text: "hello"}.Item()
	if !ok {
		t.Fatal("纯文本读数没有生成记录")
	}

	cases := []struct {
		name string
		item history.Item
		// keepsID 表示写回剪贴板后读回来的 ID 和原记录相同。捕获生成的文本和
		// 图片记录满足这一点；链接、代码和预览数据的 ID 由各自的数据来源决定，
		// 和捕获摘要无关。
		keepsID bool
	}{
		{"纯文本", text, true},
		{"链接", history.Item{ID: "l", Kind: kindLink, Text: "https://example.com"}, false},
		{"代码", history.Item{ID: "c", Kind: kindCode, Text: "x := 1"}, false},
		{"图片", history.Item{ID: "i", Kind: kindImage, Image: image}, false},
		{"富文本", richText, false},
		{"文件", file, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			echo, ok := clipboardEchoOf(tc.item)
			if !ok {
				t.Fatal("没有算出回声记录")
			}
			var want capture.Snapshot
			if tc.item.Kind == kindImage {
				want = capture.Snapshot{Kind: kindImage, Image: tc.item.Image}
			} else {
				want = capture.Snapshot{Kind: kindText, Text: tc.item.Text}
			}
			expected, ok := want.Item()
			if !ok {
				t.Fatal("期望的读数没有生成记录")
			}
			if echo.ID != expected.ID {
				t.Errorf("回声 ID = %q，期望 %q", echo.ID, expected.ID)
			}
			if same := echo.ID == tc.item.ID; same != tc.keepsID {
				t.Errorf("回声 ID 与原记录相同 = %v，期望 %v", same, tc.keepsID)
			}
		})
	}
}

// TestConsumeClipboardEchoOnlyOnce 检查回声标记只挡住紧接着的那一次读数。
func TestConsumeClipboardEchoOnlyOnce(t *testing.T) {
	app := newApplication()
	item := history.Item{ID: "t", Kind: kindText, Text: "hello"}
	app.recordClipboardWrite(item)
	echo, ok := clipboardEchoOf(item)
	if !ok {
		t.Fatal("没有算出回声记录")
	}

	if !app.consumeClipboardEcho(echo.ID) {
		t.Fatal("第一次没有挡住应用自己写出的内容")
	}
	if app.consumeClipboardEcho(echo.ID) {
		t.Error("第二次仍然挡住了同一内容")
	}
	if len(app.clipboardEcho) != 0 {
		t.Errorf("回声标记没有清空: %v", app.clipboardEcho)
	}
}

// TestRecordClipboardWriteReplacesThePreviousEcho 检查标记只认最近一次写出，
// 用户随后复制的内容不会被上一次的标记影响。
func TestRecordClipboardWriteReplacesThePreviousEcho(t *testing.T) {
	app := newApplication()
	first := history.Item{ID: "t", Kind: kindText, Text: "hello"}
	second := history.Item{ID: "u", Kind: kindText, Text: "world"}
	app.recordClipboardWrite(first)
	app.recordClipboardWrite(second)

	firstEcho, _ := clipboardEchoOf(first)
	if app.consumeClipboardEcho(firstEcho.ID) {
		t.Error("旧的标记仍然生效")
	}
	secondEcho, _ := clipboardEchoOf(second)
	if !app.consumeClipboardEcho(secondEcho.ID) {
		t.Error("新的标记没有生效")
	}
}

// TestHistoryPersistenceRoundTripsThroughSQLite 检查排进写队列的变更真的落到
// SQLite 仓库，并且重新打开数据库后能原样读回来。
func TestHistoryPersistenceRoundTripsThroughSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	ctx := context.Background()

	repository, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("打开仓库出错: %v", err)
	}
	item, ok := capture.Snapshot{
		Kind: kindRichText,
		Text: "你好",
		HTML: "<b>你好</b>",
		At:   time.UnixMilli(1700000000000),
	}.Item()
	if !ok {
		t.Fatal("富文本读数没有生成记录")
	}
	item.Favorite = true
	item.Sync = syncStateSynced
	item.Deliveries = []history.DeviceDelivery{{DeviceID: "preview-mac", State: deliveryReceived}}

	persistence := newHistoryPersistence(repository)
	persistence.enqueue(putItem(item))
	persistence.enqueue(putItem(history.Item{ID: "preview-gone", Kind: kindText, Text: "临时"}))
	persistence.enqueue(deleteItems("preview-gone"))
	persistence.Close()

	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("重新打开仓库出错: %v", err)
	}
	defer reopened.Close()
	items, err := reopened.Load(ctx)
	if err != nil {
		t.Fatalf("读取历史记录出错: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("历史记录 = %d 条，期望 1 条", len(items))
	}
	got := items[0]
	if got.ID != item.ID || got.Kind != kindRichText || got.Text != "你好" || got.HTML != "<b>你好</b>" {
		t.Errorf("读回的内容 = %+v，期望 %+v", got, item)
	}
	if !got.Favorite || got.Sync != syncStateSynced {
		t.Errorf("读回的收藏/同步状态 = %v/%v，期望 true/%v", got.Favorite, got.Sync, syncStateSynced)
	}
	if len(got.Deliveries) != 1 || got.Deliveries[0].DeviceID != "preview-mac" || got.Deliveries[0].State != deliveryReceived {
		t.Errorf("读回的投递记录 = %+v，期望一条 preview-mac/received", got.Deliveries)
	}
}

// TestHistoryLoadedFromTheRepositoryRenders 从仓库写到界面走一遍：数据库里的记录
// 装载进 Store 之后真的出现在历史列表里，空状态随之消失。
func TestHistoryLoadedFromTheRepositoryRenders(t *testing.T) {
	const text = "预览用的文本记录"

	ctx := context.Background()
	repository, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("打开仓库出错: %v", err)
	}
	item, ok := capture.Snapshot{Kind: kindText, Text: text, At: time.Now()}.Item()
	if !ok {
		t.Fatal("读数没有生成记录")
	}
	if err := repository.Put(ctx, item); err != nil {
		t.Fatalf("写入记录出错: %v", err)
	}
	items, err := repository.Load(ctx)
	if err != nil {
		t.Fatalf("读取记录出错: %v", err)
	}
	_ = repository.Close()

	app := newApplication()
	app.installHistory(items, nil)
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	if !tt.HasText(text) {
		t.Errorf("列表里没有装载的记录 %q，实际显示：%q", text, tt.Texts())
	}
	if tt.HasText(app.t(msgHistoryEmptyFirstTitle)) {
		t.Error("装载之后仍然显示空状态")
	}
}

// TestAppendCapturedSkipsTheEchoOfOurOwnWrite 检查复制一条富文本记录之后，
// 监听把写出去的纯文本读回来时不会再添加一条记录。
func TestAppendCapturedSkipsTheEchoOfOurOwnWrite(t *testing.T) {
	app := newApplication()
	stored, ok := capture.Snapshot{Kind: kindRichText, Text: "你好", HTML: "<b>你好</b>", At: time.Now()}.Item()
	if !ok {
		t.Fatal("富文本读数没有生成记录")
	}
	app.history = history.New([]history.Item{stored})
	app.recordClipboardWrite(stored)

	reading, ok := capture.Snapshot{Kind: kindText, Text: stored.Text, At: time.Now()}.Item()
	if !ok {
		t.Fatal("纯文本读数没有生成记录")
	}
	app.appendCaptured(reading)

	view := app.history.Snapshot(history.Query{})
	if len(view.Items) != 1 || view.Items[0].ID != stored.ID {
		t.Errorf("历史记录 = %d 条，期望只保留原来的那条", len(view.Items))
	}
}

// TestAppendCapturedKeepsNewContentOnce 检查新复制的内容会进入历史记录，
// 同一内容再次被捕获时不会重复添加。
func TestAppendCapturedKeepsNewContentOnce(t *testing.T) {
	app := newApplication()
	item, ok := capture.Snapshot{Kind: kindText, Text: "fresh", At: time.Now()}.Item()
	if !ok {
		t.Fatal("读数没有生成记录")
	}

	app.appendCaptured(item)
	app.appendCaptured(item)

	view := app.history.Snapshot(history.Query{})
	if len(view.Items) != 1 || view.Items[0].ID != item.ID {
		t.Errorf("历史记录 = %d 条，期望只添加一次", len(view.Items))
	}
}

// TestRecordClipboardTextBlocksTheReadingItCauses 检查预览区里点单个文件路径复制
// 之后，捕获路径不会把刚写出去的那段文本当成一次新复制。
func TestRecordClipboardTextBlocksTheReadingItCauses(t *testing.T) {
	const path = "/Users/me/季度报表 2026.xlsx"

	app := newApplication()
	app.recordClipboardText(path)

	reading, ok := capture.Snapshot{Kind: kindText, Text: path, At: time.Now()}.Item()
	if !ok {
		t.Fatal("读数没有生成记录")
	}
	app.appendCaptured(reading)

	if view := app.history.Snapshot(history.Query{}); len(view.Items) != 0 {
		t.Errorf("复制路径之后多出了 %d 条记录", len(view.Items))
	}
}
