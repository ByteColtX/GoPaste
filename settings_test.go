package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSettingsAppearanceShowsZoom 检查设置页「外观」分栏真的渲染出缩放控件。
// 测试器在 key 重复时会 panic，所以这也是设置页新控件的一次冒烟检查。
func TestSettingsAppearanceShowsZoom(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsAppearance

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	for _, want := range []string{
		app.t(msgSettingsZoom),
		app.t(msgSettingsZoomOut),
		app.t(msgSettingsZoomIn),
		app.t(msgSettingsZoomReset),
		zoomLabel(1),
	} {
		if !tt.HasText(want) {
			t.Errorf("设置页应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}

	// 点「放大」按钮：界面缩放变大，按钮上的百分比随之更新。
	if err := tt.Click(app.t(msgSettingsZoomIn)); err != nil {
		t.Fatalf("点击放大失败：%v", err)
	}
	tt.Frame()
	if app.zoom != 1.1 {
		t.Fatalf("点击放大后缩放 = %v，期望 1.1", app.zoom)
	}
	if !tt.HasText(zoomLabel(1.1)) {
		t.Errorf("设置页应显示 %s，实际显示：%q", zoomLabel(1.1), tt.Texts())
	}

	// 点「实际大小」按钮：回到 100%。
	if err := tt.Click(app.t(msgSettingsZoomReset)); err != nil {
		t.Fatalf("点击实际大小失败：%v", err)
	}
	tt.Frame()
	if app.zoom != 1 {
		t.Fatalf("点击实际大小后缩放 = %v，期望 1", app.zoom)
	}
	if !tt.HasText(zoomLabel(1)) {
		t.Errorf("设置页应显示 %s，实际显示：%q", zoomLabel(1), tt.Texts())
	}
}

// TestSidebarAppearancePopover 检查侧栏底部的外观按钮改成气泡选择器后的行为：
// 点击展开气泡，气泡列出三个模式，再点按钮或按 Esc 收起。
//
// 这里不点选某个模式：选中会调用 mygo.Theme.SetSource，而测试器不在事件循环
// 的主线程上，该调用会一直等主线程。设置页的模式分段控件同理，测试只覆盖渲染。
func TestSidebarAppearancePopover(t *testing.T) {
	app := previewApp("")
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	if tt.HasText(app.appearanceModeLabel(appearanceDark)) {
		t.Fatalf("气泡未打开时不应显示模式列表：%q", tt.Texts())
	}

	// 点侧栏底部的外观按钮：气泡展开，列出三个模式。
	if err := tt.Click(app.t(msgSettingsAppearance)); err != nil {
		t.Fatalf("点击外观按钮失败：%v", err)
	}
	tt.Frame()
	if !app.appearancePopoverOpen {
		t.Fatal("点击外观按钮后气泡应展开")
	}
	for _, mode := range appearanceModes {
		if want := app.appearanceModeLabel(mode); !tt.HasText(want) {
			t.Errorf("气泡里应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}

	// 按钮在窗口下缘，气泡应翻到按钮上方，并且整体留在窗口里。
	button, ok := tt.Find(app.t(msgSettingsAppearance))
	if !ok {
		t.Fatalf("找不到外观按钮")
	}
	option, ok := tt.Find(app.appearanceModeLabel(appearanceDark))
	if !ok {
		t.Fatalf("找不到深色选项")
	}
	if option.Y+option.H > button.Y {
		t.Errorf("气泡应在按钮上方：选项 %v，按钮 %v", option, button)
	}
	// 选项行之间也留出同样规格的空隙，两行的底色不会贴成一片。
	pitch := textPitch(t, tt, app.appearanceModeLabel(appearanceLight), app.appearanceModeLabel(appearanceDark))
	if gap := pitch - space(&app.theme, compactHeight); gap < 2 {
		t.Errorf("外观气泡选项之间的空隙 = %.1f DIP，至少要 2 DIP", gap)
	}
	if option.X < 0 || option.X+option.W > float32(defaultWindowWidth) {
		t.Errorf("气泡应留在窗口里：选项 %v", option)
	}

	// 再点一次按钮：气泡收起。
	if err := tt.Click(app.t(msgSettingsAppearance)); err != nil {
		t.Fatalf("再次点击外观按钮失败：%v", err)
	}
	tt.Frame()
	if app.appearancePopoverOpen {
		t.Fatal("再次点击外观按钮后气泡应收起")
	}
}

// textPitch 返回同一帧里两段文字的纵向距离，用来量选项行的行距。
func textPitch(t *testing.T, tt *ui.Tester, upper, lower string) float32 {
	t.Helper()
	first, ok := tt.Find(upper)
	if !ok {
		t.Fatalf("找不到 %q，实际显示：%q", upper, tt.Texts())
	}
	second, ok := tt.Find(lower)
	if !ok {
		t.Fatalf("找不到 %q，实际显示：%q", lower, tt.Texts())
	}
	return second.Y - first.Y
}

// TestDropdownOptionRowsAreSpacedApart 检查语言下拉里相邻两个选项之间留了间距。
// 行距为 0 时，当前值和指针所在项的两块底色会贴成一片，看起来像同一块高亮；
// 间距取的是单行条目的规格，与外观气泡里的选项行一致。
func TestDropdownOptionRowsAreSpacedApart(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsGeneral
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	if err := tt.Click(app.t(msgSettingsLanguageSystem)); err != nil {
		t.Fatalf("展开语言下拉失败：%v", err)
	}
	tt.Frame()

	// 行高与选项行用同一套规格，所以行距减去行高就是行与行之间的空隙。
	// 断言一个不随实现移动的下限：设计系统规定单行条目之间留 2 DIP。
	rowHeight := space(&app.theme, compactHeight)
	if gap := textPitch(t, tt, "简体中文", "English") - rowHeight; gap < 2 {
		t.Errorf("下拉选项之间的空隙 = %.1f DIP，至少要 2 DIP（行高 %.1f）", gap, rowHeight)
	}
}

// TestSettingsLanguageSelectSwitchesTheInterface 检查设置页的语言下拉真的接上了偏好：
// 选 English 之后偏好与实际界面语言都变成英文。
//
// 这里只点固定的语言，不点「跟随系统」：后者要读系统 locale，而 MyGo 在主线程上
// 读它，测试器没有那个线程（同 TestSidebarAppearancePopover 的说明）。
func TestSettingsLanguageSelectSwitchesTheInterface(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsGeneral

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	if !tt.HasText(app.t(msgSettingsLanguage)) {
		t.Fatalf("设置页应显示语言设置，实际显示：%q", tt.Texts())
	}
	// 下拉的当前值就是默认的「跟随系统」。
	if !tt.HasText(app.t(msgSettingsLanguageSystem)) {
		t.Fatalf("语言下拉应显示「%s」，实际显示：%q", app.t(msgSettingsLanguageSystem), tt.Texts())
	}

	if err := tt.Click(app.t(msgSettingsLanguageSystem)); err != nil {
		t.Fatalf("展开语言下拉失败：%v", err)
	}
	tt.Frame()
	if err := tt.Click("English"); err != nil {
		t.Fatalf("选择 English 失败：%v", err)
	}
	tt.Frame()

	if app.languagePreference != languagePreferenceEnglish {
		t.Errorf("语言偏好 = %v，期望英文", app.languagePreference)
	}
	if app.language != languageEnglish {
		t.Errorf("界面语言 = %v，期望英文", app.language)
	}
	if !tt.HasText(app.t(msgSettingsLanguage)) {
		t.Errorf("切到英文后设置页应显示 %q，实际显示：%q", app.t(msgSettingsLanguage), tt.Texts())
	}
}

// TestEveryPageRenders 把三个页面和设置页的每个分栏都渲染一遍，确认缩放没有
// 破坏任何一页的构建（key 重复、空元素或越界都会在这里暴露），并在最大档位
// 再走一遍。
func TestEveryPageRenders(t *testing.T) {
	app := previewApp("")
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	for _, page := range []page{pageHistory, pageDevices, pageSettings} {
		app.setPage(page)
		tt.Frame()
	}
	for _, area := range []settingsArea{settingsGeneral, settingsSync, settingsAppearance, settingsStorage, settingsAbout} {
		app.settingsArea = area
		tt.Frame()
	}

	// 缩放到最大档再走一遍：放大后布局下限同步变大，页面仍需正常构建。
	app.setZoom(zoomMaximum)
	tt.Frame()
	for _, area := range []settingsArea{settingsGeneral, settingsSync, settingsAppearance, settingsStorage, settingsAbout} {
		app.settingsArea = area
		tt.Frame()
	}
}

// TestSyncSettingsSoundRow 检查同步页的提示音行：三档都渲染出来，默认停在「仅同步」，
// 点选另一档会改内存状态（测试器不在主线程上，落盘由 settingsPath 为空跳过）。
func TestSyncSettingsSoundRow(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsSync
	app.soundMode = soundSync

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	for _, want := range []string{
		app.t(msgSettingsSound),
		app.t(msgSettingsSoundOff),
		app.t(msgSettingsSoundSync),
		app.t(msgSettingsSoundAll),
	} {
		if !tt.HasText(want) {
			t.Errorf("同步页应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}

	if err := tt.Click(app.t(msgSettingsSoundAll)); err != nil {
		t.Fatalf("点击「所有新内容」失败：%v", err)
	}
	tt.Frame()
	if app.soundMode != soundAll {
		t.Errorf("点击后档位 = %v，期望 %v", app.soundMode, soundAll)
	}
}

// TestSegmentedControlSegmentsAreEven 检查分段控件的三段等宽：等分之后相邻文字的中心
// 等距，而文字本身的宽度各不相同。两处调用（外观、提示音）与两种语言都过一遍——正是
// 长短不一的标签才暴露「按内容定宽」的老行为。
func TestSegmentedControlSegmentsAreEven(t *testing.T) {
	for _, lang := range []languagePreference{languagePreferenceSimplifiedChinese, languagePreferenceEnglish} {
		for _, area := range []settingsArea{settingsSync, settingsAppearance} {
			app := previewApp("")
			app.languagePreference = lang
			app.applyLanguage()
			app.setPage(pageSettings)
			app.settingsArea = area

			labels := []string{
				app.appearanceModeLabel(appearanceSystem),
				app.appearanceModeLabel(appearanceLight),
				app.appearanceModeLabel(appearanceDark),
			}
			if area == settingsSync {
				labels = []string{app.t(msgSettingsSoundOff), app.t(msgSettingsSoundSync), app.t(msgSettingsSoundAll)}
			}

			tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
			var centers []float64
			for _, label := range labels {
				rect, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%v 语言下没找到分段 %q", lang, label)
				}
				centers = append(centers, float64(rect.X+rect.W/2))
			}
			first, second := centers[1]-centers[0], centers[2]-centers[1]
			if math.Abs(first-second) > 0.5 {
				t.Errorf("%v 语言、%v 栏：分段中心间距 %.2f / %.2f，不等分", lang, area, first, second)
			}
		}
	}
}
