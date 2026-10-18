package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopaste/internal/capture"
	"gopaste/internal/history"
	"gopaste/internal/lansync"
	"gopaste/internal/settings"

	"github.com/egoist/mygo/ui"
)

// waitUntil polls a condition for a short while, for work that a goroutine does
// after the call returns.
func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the condition never became true")
}

// receivedFileIdentity is the sending device's path list, which is what a received
// file item derives its ID from.
var receivedFileIdentity = []string{"/Users/bytecolt/Documents/报表.txt"}

// receivedFileItem is one file item as it arrives from a paired device, holding
// its content at local.
func receivedFileItem(local string) history.Item {
	clip := lansync.Clip{
		Kind:       history.KindFile,
		Files:      receivedFileIdentity,
		LocalFiles: []string{local},
		FileSize:   12,
	}
	return clip.HistoryItem("peer-a", time.Now())
}

// TestReceiveClipWritesFilesAndIgnoresTheirEcho 检查收到的文件项写进剪贴板的是文件
// 本身（不是路径文本），而捕获路径读回同一批文件时会被认成回声，不会多出一条本机
// 记录。
func TestReceiveClipWritesFilesAndIgnoresTheirEcho(t *testing.T) {
	app, clipboard := syncApp()
	from := remoteDevice("peer-a", "工作室 PC")
	local := []string{"/Users/bytecolt/Library/Application Support/GoPaste/Files/abc/报表.txt"}
	clip := lansync.Clip{
		Kind:       history.KindFile,
		Files:      []string{"/Users/bytecolt/Documents/报表.txt"},
		LocalFiles: local,
		FileSize:   12,
	}

	app.receiveClip(clip, from)

	if len(clipboard.files) != 1 || len(clipboard.files[0]) != 1 || clipboard.files[0][0] != local[0] {
		t.Fatalf("clipboard files = %v, want %v", clipboard.files, local)
	}
	if len(clipboard.texts) != 0 {
		t.Errorf("a file clip must not be written as text: %q", clipboard.texts)
	}
	items := app.history.Snapshot(history.Query{}).Items
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.Text != local[0] {
		t.Errorf("text = %q, want the local path %q", item.Text, local[0])
	}
	if item.FileSize != 12 {
		t.Errorf("size = %d, want 12", item.FileSize)
	}
	if item.SourceDeviceID != from.ID {
		t.Errorf("source = %q, want %q", item.SourceDeviceID, from.ID)
	}

	// 剪贴板里现在是这些文件：捕获路径读回来的是同一条文件读数，应当被当成回声。
	echo, ok := (capture.Snapshot{Kind: kindFile, Files: local}).Item()
	if !ok {
		t.Fatal("the clipboard files must form a file item")
	}
	if !app.consumeClipboardEcho(echo.ID) {
		t.Error("capturing the files just written must be recognised as an echo")
	}
}

// TestDiscardReceivedFilesOnlyTouchesReceivedItems 检查只有「来自设备的文件条目」才
// 有落盘目录：本机文件项指向用户自己的文件，删记录不能删它们。
func TestDiscardReceivedFilesOnlyTouchesReceivedItems(t *testing.T) {
	app, _ := syncApp()
	app.receivedFilesDir = t.TempDir()

	if _, ok := receivedFilesPath(history.Item{ID: "abc", Kind: kindFile, Text: "/Users/me/report.pdf"}); ok {
		t.Error("a locally captured file item must not own a directory")
	}
	if _, ok := receivedFilesPath(history.Item{ID: "abc", Kind: kindImage, SourceDeviceID: "peer-a"}); ok {
		t.Error("only file items own a directory")
	}
	item := history.Item{ID: "abc", Kind: kindFile, SourceDeviceID: "peer-a",
		Text: filepath.Join(app.receivedFilesDir, "abc", "report.pdf")}
	dir, ok := receivedFilesPath(item)
	if !ok {
		t.Fatal("a received file item must own a directory")
	}
	if want := filepath.Join(app.receivedFilesDir, "abc"); dir != want {
		t.Errorf("directory = %q, want %q", dir, want)
	}
}

// TestDeletingAReceivedFileItemKeepsItsFilesUntilTheUndoEnds 检查撤销窗口里文件还在，
// 撤销机会结束才删：窗口内撤销仍然能把记录和文件一起放回。
func TestDeletingAReceivedFileItemKeepsItsFilesUntilTheUndoEnds(t *testing.T) {
	app, _ := syncApp()
	app.receivedFilesDir = t.TempDir()
	id := history.ContentID(history.Content{Kind: history.KindFile, Files: receivedFileIdentity})
	local := filepath.Join(app.receivedFilesDir, id, "报表.txt")
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		t.Fatalf("create the item directory: %v", err)
	}
	if err := os.WriteFile(local, []byte("content"), 0o600); err != nil {
		t.Fatalf("write the received file: %v", err)
	}
	item := receivedFileItem(local)

	discarded := false
	app.pendingUndos = append(app.pendingUndos, pendingUndo{
		toastID: "toast-1",
		restore: func() { t.Error("the undo chance was dropped, not restored") },
		discard: func() { discarded = true; app.discardReceivedFiles(item) },
	})

	app.dropPendingUndo("toast-1")

	if !discarded {
		t.Fatal("dropping the undo chance must run the cleanup")
	}
	waitUntil(t, func() bool {
		_, err := os.Stat(filepath.Dir(local))
		return errors.Is(err, os.ErrNotExist)
	})
}

// TestClearHistoryRemovesReceivedFiles 检查清空历史时收到的文件一起走：清空没有撤销，
// 留下文件只会在磁盘上堆垃圾。
func TestClearHistoryRemovesReceivedFiles(t *testing.T) {
	app, _ := syncApp()
	app.receivedFilesDir = t.TempDir()
	id := history.ContentID(history.Content{Kind: history.KindFile, Files: receivedFileIdentity})
	local := filepath.Join(app.receivedFilesDir, id, "报表.txt")
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		t.Fatalf("create the item directory: %v", err)
	}
	if err := os.WriteFile(local, []byte("content"), 0o600); err != nil {
		t.Fatalf("write the received file: %v", err)
	}
	app.history = history.New([]history.Item{receivedFileItem(local)})

	app.clearHistory()

	// 只删记录自己的目录：保存位置本身可能是用户自己的文件夹，不能被整棵删掉。
	waitUntil(t, func() bool {
		_, err := os.Stat(filepath.Dir(local))
		return errors.Is(err, os.ErrNotExist)
	})
	if _, err := os.Stat(app.receivedFilesDir); err != nil {
		t.Errorf("保存位置的根目录不该被整棵删掉: %v", err)
	}
}

// TestFileSyncLimitDefaultsAndEditing 检查上限的默认值、非法输入保留上一个有效值、
// 以及合法输入立刻生效。
func TestFileSyncLimitDefaultsAndEditing(t *testing.T) {
	app := newApplication()
	if app.fileSyncMaxMB != "2048" {
		t.Errorf("default text = %q, want 2048", app.fileSyncMaxMB)
	}
	if want := int64(settings.DefaultFileSyncMaxMB) << 20; app.fileSyncLimitBytes != want {
		t.Errorf("default limit = %d, want %d", app.fileSyncLimitBytes, want)
	}

	for _, text := range []string{"0", "-3", "", "abc", "1.5"} {
		app.editFileSyncLimit(text)
		if want := int64(settings.DefaultFileSyncMaxMB) << 20; app.fileSyncLimitBytes != want {
			t.Errorf("editing %q changed the limit to %d, want %d", text, app.fileSyncLimitBytes, want)
		}
	}

	app.fileSyncMaxMB = "64"
	app.editFileSyncLimit(app.fileSyncMaxMB)
	if want := int64(64) << 20; app.fileSyncLimitBytes != want {
		t.Errorf("limit = %d, want %d", app.fileSyncLimitBytes, want)
	}
}

// TestSaveSettingsWritesTheFileSyncLimit 检查改过的上限落盘、默认值不写进文件：
// 与设备名一样，缺省字段就是默认值。
func TestSaveSettingsWritesTheFileSyncLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	app := newApplication()
	app.settingsPath = path
	app.deviceName = app.deviceNameDefault

	app.applyFileSyncLimit(64)
	app.saveSettings()
	file, err := settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.FileSyncMaxMB != 64 {
		t.Errorf("stored limit = %d, want 64", file.FileSyncMaxMB)
	}

	app.applyFileSyncLimit(settings.DefaultFileSyncMaxMB)
	app.saveSettings()
	file, err = settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.FileSyncMaxMB != 0 {
		t.Errorf("the default limit must stay out of the file, got %d", file.FileSyncMaxMB)
	}
}

// TestSettingsSyncShowsTheFileSyncLimit 检查上限输入框跟着文件同步一起搬到了
// 「同步」分栏。测试器在 key 重复时会 panic，所以这也是新控件的一次冒烟检查。
func TestSettingsSyncShowsTheFileSyncLimit(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsSync

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	for _, want := range []string{
		app.t(msgSettingsFileSyncLimit),
		app.t(msgSettingsFileSyncLimitDesc),
		app.t(msgSettingsFileSyncLimitUnit),
	} {
		if !tt.HasText(want) {
			t.Errorf("同步分栏应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}
	// 输入框里的值不参与文本列表，所以直接看状态：默认值就是设置文件缺省时的值。
	if app.fileSyncMaxMB != "2048" {
		t.Errorf("输入框的值 = %q，期望 2048", app.fileSyncMaxMB)
	}
}

// TestPruningHistoryAlsoRemovesReceivedFiles 检查保留策略自动裁剪时，收到的文件记录
// 落盘的目录也一起清掉：裁剪不是用户点的删除，没有撤销窗口，所以立刻删。
func TestPruningHistoryAlsoRemovesReceivedFiles(t *testing.T) {
	app, _ := syncApp()
	app.receivedFilesDir = t.TempDir()
	id := history.ContentID(history.Content{Kind: history.KindFile, Files: receivedFileIdentity})
	local := filepath.Join(app.receivedFilesDir, id, "报表.txt")
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		t.Fatalf("create the item directory: %v", err)
	}
	if err := os.WriteFile(local, []byte("content"), 0o600); err != nil {
		t.Fatalf("write the received file: %v", err)
	}
	item := receivedFileItem(local)
	// 超过默认的 90 天，下一次裁剪就会删掉它。
	item.CreatedAt = time.Now().AddDate(0, 0, -400)
	app.history = history.New([]history.Item{item})

	app.pruneHistory()

	if items := app.history.Snapshot(history.Query{}).Items; len(items) != 0 {
		t.Fatalf("items = %d, want the record pruned", len(items))
	}
	waitUntil(t, func() bool {
		_, err := os.Stat(filepath.Dir(local))
		return errors.Is(err, os.ErrNotExist)
	})
}

// TestPruningHistoryLeavesLocalFilesAlone 检查自动裁剪不会去删本机文件的目录：本机
// 文件项只是指向用户自己的文件。
func TestPruningHistoryLeavesLocalFilesAlone(t *testing.T) {
	app, _ := syncApp()
	app.receivedFilesDir = t.TempDir()
	// 与收到的条目同 ID，但它是本机捕获的：目录里恰好有这个 ID 也不该被动。
	id := history.ContentID(history.Content{Kind: history.KindFile, Files: receivedFileIdentity})
	dir := filepath.Join(app.receivedFilesDir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the item directory: %v", err)
	}
	item := history.Item{
		ID:        id,
		Kind:      kindFile,
		Text:      "/Users/bytecolt/report.pdf",
		CreatedAt: time.Now().AddDate(0, 0, -400),
	}
	app.history = history.New([]history.Item{item})

	app.pruneHistory()

	if items := app.history.Snapshot(history.Query{}).Items; len(items) != 0 {
		t.Fatalf("items = %d, want the record pruned", len(items))
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("a locally captured item must not delete anything under Files: %v", err)
	}
}
