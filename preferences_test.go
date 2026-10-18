package main

import (
	"testing"

	"gopaste/internal/settings"

	"github.com/egoist/mygo/ui"
)

// preferencesApp 返回一个接在临时设置文件上的应用：向导已完成，界面偏好来自传入的
// ui（nil 表示全是默认）。它走真实启动的同一条 applySettings 路径。
func preferencesApp(t *testing.T, ui *settings.UI) (*application, string) {
	t.Helper()
	return onboardingApp(t, settings.File{
		Onboarding: settings.Onboarding{Completed: true},
		UI:         ui,
	})
}

// TestSavedPreferencesComeBackOnTheNextLaunch 检查四项界面偏好都能从设置文件恢复：
// 语言、外观、删除前确认、缩放。这是「重启后回到默认」那个 bug 的回归测试。
func TestSavedPreferencesComeBackOnTheNextLaunch(t *testing.T) {
	off := false
	app, _ := preferencesApp(t, &settings.UI{
		Language:      "en",
		Appearance:    "dark",
		ConfirmDelete: &off,
		Zoom:          1.25,
	})

	if app.languagePreference != languagePreferenceEnglish {
		t.Errorf("语言偏好 = %v，期望英文", app.languagePreference)
	}
	if app.appearance != appearanceDark {
		t.Errorf("外观 = %v，期望深色", app.appearance)
	}
	if app.confirmBeforeDelete {
		t.Error("删除前确认应读回关闭状态")
	}
	if app.zoom != 1.25 {
		t.Errorf("缩放 = %v，期望 1.25", app.zoom)
	}
}

// TestMissingPreferencesKeepTheDefaults 检查没有 ui 的旧文件不会改动任何偏好：
// 缺字段就是默认值。
func TestMissingPreferencesKeepTheDefaults(t *testing.T) {
	app, _ := preferencesApp(t, nil)

	if app.languagePreference != languagePreferenceSystem {
		t.Errorf("语言偏好 = %v，期望跟随系统", app.languagePreference)
	}
	if app.appearance != appearanceSystem {
		t.Errorf("外观 = %v，期望跟随系统", app.appearance)
	}
	if !app.confirmBeforeDelete {
		t.Error("删除前确认默认应为开启")
	}
	if app.zoom != 1 {
		t.Errorf("缩放 = %v，期望 100%%", app.zoom)
	}
}

// TestUnknownPreferenceValuesFallBackToDefaults 检查认不出的取值不让应用带着怪状态
// 跑起来：语言与外观回到默认，缩放夹进允许范围。
func TestUnknownPreferenceValuesFallBackToDefaults(t *testing.T) {
	app, _ := preferencesApp(t, &settings.UI{Language: "de", Appearance: "sepia", Zoom: 9})

	if app.languagePreference != languagePreferenceSystem {
		t.Errorf("未知语言 = %v，期望回退到跟随系统", app.languagePreference)
	}
	if app.appearance != appearanceSystem {
		t.Errorf("未知外观 = %v，期望回退到跟随系统", app.appearance)
	}
	if app.zoom != zoomMaximum {
		t.Errorf("越界缩放 = %v，期望夹到 %v", app.zoom, zoomMaximum)
	}
}

// TestDefaultPreferencesStayOutOfTheFile 检查全是默认值时文件里不出现 ui 对象：
// 默认值就是缺字段。
func TestDefaultPreferencesStayOutOfTheFile(t *testing.T) {
	app, path := preferencesApp(t, nil)
	app.saveSettings()

	if file := readSettings(t, path); file.UI != nil {
		t.Errorf("默认偏好不该写进文件，实际写入 %+v", file.UI)
	}
}

// TestSavingAPreferenceKeepsTheOtherSettings 检查保存界面偏好是整份重写，不会丢掉
// 向导完成标记与改过的设备名。
func TestSavingAPreferenceKeepsTheOtherSettings(t *testing.T) {
	app, path := preferencesApp(t, nil)
	app.deviceName = "Studio Mac"
	app.languagePreference = languagePreferenceEnglish
	app.saveSettings()

	file := readSettings(t, path)
	if !file.Onboarding.Completed {
		t.Error("保存偏好时弄丢了向导完成标记")
	}
	if file.DeviceName != "Studio Mac" {
		t.Errorf("保存偏好时弄丢了设备名，实际 %q", file.DeviceName)
	}
	if file.UI == nil || file.UI.Language != "en" {
		t.Errorf("语言偏好没有落盘，实际 %+v", file.UI)
	}
}

// TestPreferencesAreProjectedFromTheCurrentState 检查内存状态到设置文件的映射：
// 外观、删除确认与缩放各自按约定写法落盘。
func TestPreferencesAreProjectedFromTheCurrentState(t *testing.T) {
	app, path := preferencesApp(t, nil)
	app.appearance = appearanceDark
	app.confirmBeforeDelete = false
	app.zoom = 1.1
	app.saveSettings()

	file := readSettings(t, path)
	if file.UI == nil {
		t.Fatal("改过的偏好没有落盘")
	}
	if file.UI.Appearance != "dark" {
		t.Errorf("外观写法 = %q，期望 dark", file.UI.Appearance)
	}
	if file.UI.ConfirmDelete == nil || *file.UI.ConfirmDelete {
		t.Errorf("删除前确认 = %v，期望写入明确的 false", file.UI.ConfirmDelete)
	}
	if file.UI.Zoom != 1.1 {
		t.Errorf("缩放 = %v，期望 1.1", file.UI.Zoom)
	}
}

// TestLanguageChoiceIsSavedImmediately 检查设置页换语言之后马上落盘：不用等退出，
// 崩了也不会丢这一次改动。
func TestLanguageChoiceIsSavedImmediately(t *testing.T) {
	app, path := preferencesApp(t, nil)
	app.setPage(pageSettings)
	app.settingsArea = settingsGeneral

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()
	if err := tt.Click(app.t(msgSettingsLanguageSystem)); err != nil {
		t.Fatalf("展开语言下拉失败：%v", err)
	}
	tt.Frame()
	if err := tt.Click("English"); err != nil {
		t.Fatalf("选择 English 失败：%v", err)
	}
	tt.Frame()

	file := readSettings(t, path)
	if file.UI == nil || file.UI.Language != "en" {
		t.Errorf("语言偏好没有立刻落盘，实际 %+v", file.UI)
	}
}

// TestZoomChangeIsSavedImmediately 检查设置页点放大之后缩放马上落盘。
func TestZoomChangeIsSavedImmediately(t *testing.T) {
	app, path := preferencesApp(t, nil)
	app.setPage(pageSettings)
	app.settingsArea = settingsAppearance

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()
	if err := tt.Click(app.t(msgSettingsZoomIn)); err != nil {
		t.Fatalf("点击放大失败：%v", err)
	}
	tt.Frame()

	file := readSettings(t, path)
	if file.UI == nil || file.UI.Zoom != 1.1 {
		t.Errorf("缩放没有立刻落盘，实际 %+v", file.UI)
	}
}

// TestDeviceNameChangeIsSaved 检查设置页改完设备名会落盘：以前只有向导结束会写设备名，
// 在设置页改名重启后就退回默认主机名。
func TestDeviceNameChangeIsSaved(t *testing.T) {
	app, path := preferencesApp(t, nil)
	app.deviceName = "Studio Mac"
	app.deviceNameChanged()

	if file := readSettings(t, path); file.DeviceName != "Studio Mac" {
		t.Errorf("设备名没有落盘，实际 %q", file.DeviceName)
	}
}

// TestRestoredZoomIsClampedToTheScreen 检查从更大的屏幕恢复的缩放会按当前屏幕夹住，
// 而夹过的值不写回文件：换回大屏幕仍恢复用户原来的比例。
func TestRestoredZoomIsClampedToTheScreen(t *testing.T) {
	app := newApplication()
	app.zoom = 1.25

	app.clampZoomToScreen(1.1)
	if app.zoom != 1.1 {
		t.Errorf("夹到屏幕上限后缩放 = %v，期望 1.1", app.zoom)
	}

	app.zoom = 1.1
	app.clampZoomToScreen(0)
	if app.zoom != 1.1 {
		t.Errorf("屏幕未知时缩放 = %v，期望保持不变", app.zoom)
	}
}
