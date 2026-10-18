package main

import (
	"path/filepath"
	"testing"

	"gopaste/internal/settings"

	"github.com/egoist/mygo/ui"
)

// TestSyncSettingsPersist 检查同步页的每一项都写进设置文件：关掉的开关、改过的
// 配额、期限、自动清理与保存位置。
func TestSyncSettingsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	app := newApplication()
	app.settingsPath = path

	app.setSyncEnabled(false)
	app.setAutoSendEnabled(false)
	app.setFileSyncEnabled(false)
	app.editFileQuota("4096")
	app.editFileRetention(0)
	app.editFileAutoClean(false)
	app.setFileSyncDir("/Users/me/GoPasteFiles")

	file, err := settings.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.SyncEnabled() || file.AutoSendEnabled() || file.FileSyncEnabled() {
		t.Error("关掉的开关没有写进设置文件")
	}
	if got := file.FileQuotaMB(); got != 4096 {
		t.Errorf("配额 = %d，期望 4096", got)
	}
	if got := file.FileRetentionHours(); got != 0 {
		t.Errorf("期限 = %d，期望 0（不限）", got)
	}
	if file.FileAutoCleanEnabled() {
		t.Error("关掉的自动清理没有写进设置文件")
	}
	if got := file.FileSyncDir(); got != "/Users/me/GoPasteFiles" {
		t.Errorf("保存位置 = %q", got)
	}
}

// TestSyncSettingsRoundTrip 检查设置文件里的同步设置能原样回到内存状态。
func TestSyncSettingsRoundTrip(t *testing.T) {
	off := false
	unlimited := 0
	file := settings.File{
		Onboarding: settings.Onboarding{Completed: true},
		Sync: &settings.Sync{
			Enabled:  &off,
			AutoSend: &off,
			Files: &settings.SyncFiles{
				Enabled:        &off,
				Dir:            "/Users/me/GoPasteFiles",
				QuotaMB:        4096,
				RetentionHours: &unlimited,
				AutoClean:      &off,
			},
		},
	}

	app := newApplication()
	app.applySettings(file)

	if app.syncEnabled || app.autoSendEnabled || app.fileSyncEnabled || app.fileAutoClean {
		t.Error("关掉的开关没有读回内存状态")
	}
	if got := app.fileQuotaMB; got != "4096" {
		t.Errorf("配额文本 = %q，期望 4096", got)
	}
	if want := int64(4096) << 20; app.fileQuotaBytes != want {
		t.Errorf("配额 = %d，期望 %d", app.fileQuotaBytes, want)
	}
	if app.fileRetentionHours != 0 {
		t.Errorf("期限 = %d，期望 0（不限）", app.fileRetentionHours)
	}
	if app.fileSyncDir != "/Users/me/GoPasteFiles" {
		t.Errorf("保存位置 = %q", app.fileSyncDir)
	}
	// 还没启动同步服务，默认目录也还没解析；选过的目录直接生效。
	if app.receivedFilesDir != "/Users/me/GoPasteFiles" {
		t.Errorf("生效目录 = %q，期望选过的目录", app.receivedFilesDir)
	}
}

// TestFileSyncDirFallsBackToTheDefault 检查保存位置为空时用默认目录，恢复默认后
// 又回到它。
func TestFileSyncDirFallsBackToTheDefault(t *testing.T) {
	app := newApplication()
	app.setDefaultFilesDir("/data/GoPaste/Files")
	if got := app.receivedFilesDir; got != "/data/GoPaste/Files" {
		t.Fatalf("生效目录 = %q，期望默认目录", got)
	}

	app.setFileSyncDir("/Users/me/Files")
	if got := app.receivedFilesDir; got != "/Users/me/Files" {
		t.Errorf("生效目录 = %q，期望选过的目录", got)
	}

	app.resetFileSyncDir()
	if app.fileSyncDir != "" {
		t.Errorf("保存位置 = %q，期望恢复成空（用默认）", app.fileSyncDir)
	}
	if got := app.receivedFilesDir; got != "/data/GoPaste/Files" {
		t.Errorf("恢复默认后生效目录 = %q，期望默认目录", got)
	}
}

// TestDevicePageIsReadOnlyWhileSyncIsOff 检查总开关关着时设备页给出说明，打开后
// 说明消失：置灰之外还要告诉用户为什么。
func TestDevicePageIsReadOnlyWhileSyncIsOff(t *testing.T) {
	app := previewApp("")
	app.pairedDevices = []pairedDevice{
		{id: "peer-a", name: "MacBook Pro", platform: "macOS", online: true, syncEnabled: true},
	}
	app.selectedDeviceID = "peer-a"
	app.setPage(pageDevices)
	app.syncEnabled = false

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()
	if !tt.HasText(app.t(msgDevicesSyncPageOffNote)) {
		t.Errorf("总开关关着时设备页应说明原因，实际显示：%q", tt.Texts())
	}

	app.syncEnabled = true
	tt.Frame()
	if tt.HasText(app.t(msgDevicesSyncPageOffNote)) {
		t.Errorf("总开关打开后不该再显示说明，实际显示：%q", tt.Texts())
	}
}

// TestSyncPageShowsTheFileCacheUsage 检查同步页显示缓存占用与「立即清理」按钮。
func TestSyncPageShowsTheFileCacheUsage(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsSync
	app.applyStorageUsage(storageUsage{files: 5 << 20})

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	for _, want := range []string{
		app.t(msgSettingsFileCacheUsage),
		app.t(msgSettingsFileCacheUsageDesc),
		app.t(msgSettingsFileCacheClean),
		formatBytes(5 << 20),
	} {
		if !tt.HasText(want) {
			t.Errorf("同步页应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}
}

// TestFileRetentionChoices 检查期限档位：小时数与整天的文案分开，最后一个是「不限」。
func TestFileRetentionChoices(t *testing.T) {
	l := newLocalizer(languageSimplifiedChinese)
	choices := l.fileRetentionChoices()

	wantValues := []int{1, 6, 24, 72, 168, 0}
	if len(choices) != len(wantValues) {
		t.Fatalf("档位数 = %d，期望 %d", len(choices), len(wantValues))
	}
	for index, want := range wantValues {
		if choices[index].value != want {
			t.Errorf("第 %d 档 = %d，期望 %d", index, choices[index].value, want)
		}
	}
	if got := choices[0].label; got != "1 小时" {
		t.Errorf("1 小时的文案 = %q", got)
	}
	if got := choices[3].label; got != "3 天" {
		t.Errorf("72 小时的文案 = %q，期望按天显示", got)
	}
	if got := choices[5].label; got != l.text(msgSettingsRetentionUnlimited) {
		t.Errorf("不限的文案 = %q", got)
	}
}
