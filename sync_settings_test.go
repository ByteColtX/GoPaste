package main

import (
	"os"
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

// TestSyncPageAbbreviatesTheSaveLocation 检查保存位置把主目录缩成 ~，并且不在
// 页面上摊开绝对路径：默认位置一行就放得下，长路径靠悬停提示补全。
func TestSyncPageAbbreviatesTheSaveLocation(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("没有用户主目录，跳过")
	}
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsSync
	app.setDefaultFilesDir(filepath.Join(home, "Library", "GoPaste", "Files"))

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	if !tt.HasText("~/Library/GoPaste/Files") {
		t.Errorf("同步页应显示缩写后的路径，实际显示：%q", tt.Texts())
	}
	if tt.HasText(filepath.Join(home, "Library", "GoPaste", "Files")) {
		t.Errorf("同步页不该摊开绝对路径，实际显示：%q", tt.Texts())
	}
}

// TestSaveLocationButtonsAlignRight 检查「选择…」与「恢复默认」靠右对齐：按钮的右边缘
// 与路径（也就是卡片内容区）的右边缘齐平，而不是从路径左边开始排。
func TestSaveLocationButtonsAlignRight(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsSync
	app.setDefaultFilesDir("/data/GoPaste/Files")

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	path, ok := tt.Find("/data/GoPaste/Files")
	if !ok {
		t.Fatalf("同步页应显示保存位置，实际显示：%q", tt.Texts())
	}
	reset, ok := tt.Find(app.t(msgSettingsFileSyncDirReset))
	if !ok {
		t.Fatalf("同步页应显示恢复默认按钮，实际显示：%q", tt.Texts())
	}
	if gap := path.X + path.W - (reset.X + reset.W); gap > 4 {
		t.Errorf("按钮右边缘离路径右边缘还差 %.1f DIP，期望靠右对齐", gap)
	}
}

// TestAbbreviateHomePath 检查缩写只动主目录前缀：主目录本身、主目录下的路径、
// 只是字符串前缀相同但不在主目录里的路径，各有各的结果。
func TestAbbreviateHomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("没有用户主目录，跳过")
	}
	cases := []struct{ in, want string }{
		{"", ""},
		{home, "~"},
		{filepath.Join(home, "Library", "GoPaste", "Files"), "~/Library/GoPaste/Files"},
		{home + "-backup", home + "-backup"},
		{filepath.Join(string(filepath.Separator), "opt", "gopaste", "files"), filepath.Join(string(filepath.Separator), "opt", "gopaste", "files")},
	}
	for _, testCase := range cases {
		if got := abbreviateHomePath(testCase.in); got != testCase.want {
			t.Errorf("abbreviateHomePath(%q) = %q，期望 %q", testCase.in, got, testCase.want)
		}
	}
}

// TestChooseFileSyncDirAsksTheRuntime 检查「选择…」把对话框交给运行时：对话框会
// 阻塞，不能直接在手势回调里等它。
func TestChooseFileSyncDirAsksTheRuntime(t *testing.T) {
	app := newApplication()
	app.setDefaultFilesDir("/data/GoPaste/Files")

	var asked string
	app.onChooseDirectory = func(defaultPath string) { asked = defaultPath }
	app.chooseFileSyncDir()
	if asked != "/data/GoPaste/Files" {
		t.Errorf("交给运行时的默认目录 = %q，期望当前生效目录", asked)
	}

	// 没有运行时钩子（测试、没有窗口）时什么都不做，也不 panic。
	app.onChooseDirectory = nil
	app.chooseFileSyncDir()
}

// TestApplyDirectoryChoiceCreatesAndApplies 检查选中的目录先建出来再生效。
func TestApplyDirectoryChoiceCreatesAndApplies(t *testing.T) {
	app := newApplication()
	app.setDefaultFilesDir("/data/GoPaste/Files")
	dir := filepath.Join(t.TempDir(), "chosen")

	app.applyDirectoryChoice(dir)

	if app.fileSyncDir != dir || app.receivedFilesDir != dir {
		t.Errorf("保存位置 = %q / 生效目录 = %q，期望 %q", app.fileSyncDir, app.receivedFilesDir, dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("选中的目录应当已经建出来: %v", err)
	}
}

// TestApplyDirectoryChoiceKeepsTheOldLocationWhenUnusable 检查建不出来的目录不会
// 变成保存位置：换个只读的位置只会让之后收到的文件全部「未送达」。
func TestApplyDirectoryChoiceKeepsTheOldLocationWhenUnusable(t *testing.T) {
	app := newApplication()
	app.setDefaultFilesDir("/data/GoPaste/Files")
	// 一个普通文件下面的路径建不出来，任何平台都一样。
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatalf("写文件出错: %v", err)
	}

	app.applyDirectoryChoice(filepath.Join(file, "sub"))

	if app.fileSyncDir != "" || app.receivedFilesDir != "/data/GoPaste/Files" {
		t.Errorf("保存位置不该改变，实际 %q / %q", app.fileSyncDir, app.receivedFilesDir)
	}
	if app.pendingToast != app.t(msgToastFileDirUnusable) {
		t.Errorf("提示 = %q，期望「无法使用这个目录」", app.pendingToast)
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
