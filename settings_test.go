package main

import (
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
	for _, area := range []settingsArea{settingsGeneral, settingsAppearance, settingsStorage, settingsAbout} {
		app.settingsArea = area
		tt.Frame()
	}

	// 缩放到最大档再走一遍：放大后布局下限同步变大，页面仍需正常构建。
	app.setZoom(zoomMaximum)
	tt.Frame()
	for _, area := range []settingsArea{settingsGeneral, settingsAppearance, settingsStorage, settingsAbout} {
		app.settingsArea = area
		tt.Frame()
	}
}
