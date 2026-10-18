package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopaste/internal/filescache"
	"gopaste/internal/history"
)

// receivedFileIn writes one received file item whose files live under root/<id>,
// the way lansync lays them out.
func receivedFileIn(t *testing.T, root, id string, age time.Duration) history.Item {
	t.Helper()
	local := filepath.Join(root, id, "报表.txt")
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		t.Fatalf("create the item directory: %v", err)
	}
	if err := os.WriteFile(local, []byte("content"), 0o600); err != nil {
		t.Fatalf("write the received file: %v", err)
	}
	return history.Item{
		ID:             id,
		Kind:           kindFile,
		Text:           local,
		SourceDeviceID: "peer-a",
		CreatedAt:      time.Now().Add(-age),
	}
}

// TestApplyFileCacheCleanupDeletesRecordsAndFiles 检查清理结果落到历史：记录消失，
// 文件目录跟着删掉，并且弹一条汇总提示。
func TestApplyFileCacheCleanupDeletesRecordsAndFiles(t *testing.T) {
	app, _ := syncApp()
	root := t.TempDir()
	item := receivedFileIn(t, root, "old-id", 48*time.Hour)
	app.history = history.New([]history.Item{item})

	app.applyFileCacheCleanup([]string{item.ID})

	if items := app.history.Snapshot(history.Query{}).Items; len(items) != 0 {
		t.Fatalf("清理后还有 %d 条记录，期望 0 条", len(items))
	}
	waitUntil(t, func() bool {
		_, err := os.Stat(filepath.Join(root, item.ID))
		return errors.Is(err, os.ErrNotExist)
	})
	if app.pendingToast != app.tf(msgToastFileCacheCleaned, 1) {
		t.Errorf("提示 = %q，期望清理汇总", app.pendingToast)
	}
	if app.fileCachePending {
		t.Error("清理完成后 fileCachePending 应当已经复位")
	}
}

// TestRequestFileCacheCleanupHonoursTheAutoCleanSwitch 检查「自动清理」只挡住过期
// 规则：关着时配额照旧守住，手动清理则两条规则都跑。
func TestRequestFileCacheCleanupHonoursTheAutoCleanSwitch(t *testing.T) {
	app, _ := syncApp()
	app.history = history.New([]history.Item{receivedFileIn(t, t.TempDir(), "old-id", time.Hour)})
	app.fileRetentionHours = 24
	app.fileQuotaBytes = 5 << 20

	var got filescache.Policy
	calls := 0
	app.onCleanFileCache = func(_ []filescache.Entry, policy filescache.Policy) {
		calls++
		got = policy
	}

	app.fileAutoClean = false
	app.requestFileCacheCleanup(false)
	if calls != 1 {
		t.Fatalf("自动清理关着时配额仍该执行，实际调用 %d 次", calls)
	}
	if got.MaxAge != 0 {
		t.Errorf("自动清理关着时不应带上期限，实际 %v", got.MaxAge)
	}
	if got.QuotaBytes != 5<<20 {
		t.Errorf("配额 = %d，期望 %d", got.QuotaBytes, int64(5<<20))
	}

	// 同一时刻只有一次清理在飞。
	app.requestFileCacheCleanup(false)
	if calls != 1 {
		t.Errorf("还在飞的时候又发了一次请求，调用 %d 次", calls)
	}

	app.fileCachePending = false
	app.requestFileCacheCleanup(true)
	if calls != 2 {
		t.Fatalf("手动清理应当执行，实际调用 %d 次", calls)
	}
	if got.MaxAge != 24*time.Hour {
		t.Errorf("手动清理应带上期限，实际 %v", got.MaxAge)
	}
}

// TestRequestFileCacheCleanupWithoutRecordsToastsOnManualOnly 检查没有接收文件时
// 不发起任务：手动清理给一句提示，自动触发什么都不做。
func TestRequestFileCacheCleanupWithoutRecordsToastsOnManualOnly(t *testing.T) {
	app, _ := syncApp()
	calls := 0
	app.onCleanFileCache = func([]filescache.Entry, filescache.Policy) { calls++ }

	app.requestFileCacheCleanup(false)
	if calls != 0 || app.pendingToast != "" {
		t.Errorf("自动触发不该发起任务或提示，calls=%d toast=%q", calls, app.pendingToast)
	}

	app.requestFileCacheCleanup(true)
	if calls != 0 {
		t.Errorf("没有接收文件时不该发起任务，calls=%d", calls)
	}
	if app.pendingToast != app.t(msgToastFileCacheNothing) {
		t.Errorf("提示 = %q，期望「没有需要清理的文件」", app.pendingToast)
	}
}

// TestReceivedFilesPathFollowsTheRecordNotTheCurrentRoot 检查记录目录从条目自己的
// 路径反推：换过保存位置之后，旧目录里的记录仍然指向旧目录。
func TestReceivedFilesPathFollowsTheRecordNotTheCurrentRoot(t *testing.T) {
	app, _ := syncApp()
	oldRoot := t.TempDir()
	item := receivedFileIn(t, oldRoot, "old-id", time.Hour)
	app.history = history.New([]history.Item{item})
	app.receivedFilesDir = t.TempDir()

	dir, ok := receivedFilesPath(item)
	if !ok {
		t.Fatal("收到的文件条目应当能反推出自己的目录")
	}
	if want := filepath.Join(oldRoot, "old-id"); dir != want {
		t.Errorf("目录 = %q，期望 %q", dir, want)
	}

	// 用量统计要同时看当前根目录与旧根目录，否则旧文件会凭空消失。
	roots := app.fileCacheRoots()
	if len(roots) != 2 {
		t.Fatalf("根目录 = %v，期望当前目录与旧目录两个", roots)
	}
}

// TestReceivedFilesPathRejectsAnIDThatIsNotADirectory 检查路径里找不到内容 ID 那一层
// 时不猜目录：宁可不动，也不删别的文件夹。
func TestReceivedFilesPathRejectsAnIDThatIsNotADirectory(t *testing.T) {
	item := history.Item{
		ID:             "abc",
		Kind:           kindFile,
		SourceDeviceID: "peer-a",
		Text:           "/Users/me/Documents/report.pdf",
	}
	if dir, ok := receivedFilesPath(item); ok {
		t.Errorf("不该反推出目录，实际 %q", dir)
	}
}
