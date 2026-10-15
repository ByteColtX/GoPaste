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
