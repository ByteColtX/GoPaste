package main

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestZoomStepWalksTheSteps(t *testing.T) {
	cases := []struct {
		name      string
		current   float32
		direction int
		want      float32
	}{
		{"放大一档", 1, 1, 1.1},
		{"缩小一档", 1, -1, 0.9},
		{"到顶不再放大", zoomMaximum, 1, zoomMaximum},
		{"到底不再缩小", zoomMinimum, -1, zoomMinimum},
		{"非整数比例先归到最近的档", 1.04, 1, 1.1},
		{"从中间档缩小", 1.25, -1, 1.1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := zoomStep(tc.current, tc.direction); got != tc.want {
				t.Errorf("zoomStep(%v, %d) = %v，期望 %v", tc.current, tc.direction, got, tc.want)
			}
		})
	}
}

func TestZoomedWindowMinimumFollowsTheZoom(t *testing.T) {
	cases := []struct {
		zoom   float32
		width  int
		height int
	}{
		{1, minimumWindowWidth, minimumWindowHeight},
		{1.25, 1250, 700},
		{0.8, 800, 448},
	}
	for _, tc := range cases {
		width, height := zoomedWindowMinimum(tc.zoom)
		if width != tc.width || height != tc.height {
			t.Errorf("zoomedWindowMinimum(%v) = %d×%d，期望 %d×%d",
				tc.zoom, width, height, tc.width, tc.height)
		}
	}
}

func TestZoomCeilingFitsTheScreen(t *testing.T) {
	// 1280 宽放不下 1.5 倍（需要 1500），高度有余量，所以上限由宽度决定。
	area := mygo.Rectangle{X: 0, Y: 0, Width: 1280, Height: 900}
	if got, want := zoomCeilingForArea(area), float32(1.28); got != want {
		t.Errorf("zoomCeilingForArea(1280×900) = %v，期望 %v", got, want)
	}
	if got := zoomCeilingForArea(mygo.Rectangle{}); got != 0 {
		t.Errorf("屏幕未知时不应给上限，得到 %v", got)
	}
}

func TestWindowSizeForZoomGrowsButNeverShrinks(t *testing.T) {
	area := mygo.Rectangle{X: 0, Y: 0, Width: 2560, Height: 1440}

	// 窗口比新的最小尺寸小：长到刚好放得下。
	if width, height := windowSizeForZoom(1000, 560, 1.25, area); width != 1250 || height != 700 {
		t.Errorf("放大后窗口 = %d×%d，期望 1250×700", width, height)
	}
	// 窗口已经更大：不动它。
	if width, height := windowSizeForZoom(1800, 1100, 1.1, area); width != 1800 || height != 1100 {
		t.Errorf("窗口够大时不应改变尺寸，得到 %d×%d", width, height)
	}
	// 屏幕装不下：按可用区域截断。
	small := mygo.Rectangle{X: 0, Y: 0, Width: 1100, Height: 620}
	if width, height := windowSizeForZoom(1000, 560, 1.25, small); width != 1100 || height != 620 {
		t.Errorf("窗口应不超过屏幕可用区域，得到 %d×%d", width, height)
	}
}

// TestZoomShortcutsResizeTheInterface 走一遍真实的界面：⌘= 放大、⌘- 缩小、
// ⌘0 回到 100%，字号与 4 DIP 网格一起变，而且同一档位多画几帧不会复利放大。
func TestZoomShortcutsResizeTheInterface(t *testing.T) {
	app := previewApp("")

	var fontSize, spacing float32
	view := func(c *ui.Context) {
		app.view(c)
		fontSize, spacing = c.Theme().FontSize, c.Theme().Spacing
	}
	tt := ui.NewTester(view, defaultWindowWidth, defaultWindowHeight)

	baseFont, baseSpacing := fontSize, spacing
	if app.zoom != 1 {
		t.Fatalf("初始缩放 = %v，期望 100%%", app.zoom)
	}

	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Frame()
	if app.zoom != 1.1 {
		t.Fatalf("⌘= 后缩放 = %v，期望 1.1", app.zoom)
	}
	if fontSize <= baseFont || spacing <= baseSpacing {
		t.Errorf("放大后字号与网格应变大：%v/%v → %v/%v", baseFont, baseSpacing, fontSize, spacing)
	}

	// 停在同一档位多画几帧：主题每帧都从平台基准值重算，不会越乘越大。
	for range 3 {
		tt.Frame()
	}
	if fontSize != app.platformFontSize*app.zoom || spacing != app.platformSpacing*app.zoom {
		t.Errorf("同一档位的字号/网格应保持不变，得到 %v/%v", fontSize, spacing)
	}

	tt.Key(ui.Cmd, ui.KeyMinus)
	tt.Frame()
	if app.zoom != 1 {
		t.Errorf("⌘- 后缩放 = %v，期望回到 1", app.zoom)
	}
	if fontSize != baseFont || spacing != baseSpacing {
		t.Errorf("回到 100%% 后字号/网格应恢复：%v/%v，期望 %v/%v", fontSize, spacing, baseFont, baseSpacing)
	}

	tt.Key(ui.Cmd, ui.KeyEqual)
	tt.Key(ui.Cmd, ui.Key0)
	tt.Frame()
	if app.zoom != 1 {
		t.Errorf("⌘0 后缩放 = %v，期望 1", app.zoom)
	}
}

// TestZoomStopsAtTheScreenCeiling 检查缩放上限：运行时给出的屏幕上限优先于
// 内置上限，避免放大到界面最小尺寸都放不下。
func TestZoomStopsAtTheScreenCeiling(t *testing.T) {
	app := previewApp("")
	app.onMaxZoom = func() float32 { return 1.1 }

	app.zoomIn()
	if app.zoom != 1.1 {
		t.Fatalf("缩放 = %v，期望被屏幕上限截在 1.1", app.zoom)
	}
	app.zoomIn()
	if app.zoom != 1.1 {
		t.Errorf("已到上限时不应继续放大，得到 %v", app.zoom)
	}
}

// TestZoomNotifiesTheRuntime 检查缩放变化会通知运行时（调整窗口最小尺寸与
// 大小），并且重复设成同一个值不会重复通知。
func TestZoomNotifiesTheRuntime(t *testing.T) {
	app := previewApp("")
	var calls []float32
	app.onZoomChanged = func(zoom float32) { calls = append(calls, zoom) }

	app.zoomIn()
	app.zoomIn()
	app.resetZoom()
	if len(calls) != 3 || calls[0] != 1.1 || calls[1] != 1.25 || calls[2] != 1 {
		t.Fatalf("通知的缩放序列 = %v，期望 [1.1 1.25 1]", calls)
	}

	// 已经在 100%：再按实际大小不应产生通知。
	app.resetZoom()
	if len(calls) != 3 {
		t.Errorf("重复设置同一个比例不应再通知，得到 %v", calls)
	}
}
