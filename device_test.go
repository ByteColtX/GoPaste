package main

import (
	"bytes"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestDeviceSidebarHeaderFitsItsRefreshButton 检查设备页分组标题行的高度容得下
// 刷新按钮。这一行曾经是「固定高度 + 上内边距」：内边距把内容盒从 36 DIP 压到
// 24 DIP，36 DIP 的按钮居中后上下各溢出 6 DIP，悬停高亮因此压住下面第一台设备
// 的选中高亮。留白写成 Margin 后行高与按钮同高，两侧都不再越界。
func TestDeviceSidebarHeaderFitsItsRefreshButton(t *testing.T) {
	app := previewApp("")
	app.pairedDevices = []pairedDevice{
		{id: "peer-a", name: "MacBook Pro", platform: "macOS", online: true, syncEnabled: true},
		{id: "peer-b", name: "工作室 PC", platform: "Windows", online: true, syncEnabled: true},
	}
	app.selectedDeviceID = "peer-a"
	app.setPage(pageDevices)

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	refresh, ok := tt.Find(app.t(msgDevicesRefresh))
	if !ok {
		t.Fatalf("设备页应显示刷新按钮 %q，实际显示：%q", app.t(msgDevicesRefresh), tt.Texts())
	}
	local, ok := tt.Find(app.deviceName)
	if !ok {
		t.Fatalf("设备页应显示本机条目 %q，实际显示：%q", app.deviceName, tt.Texts())
	}
	first, ok := tt.Find("MacBook Pro")
	if !ok {
		t.Fatalf("设备页应显示已配对设备，实际显示：%q", tt.Texts())
	}

	if refresh.Y < local.Y+local.H {
		t.Errorf("刷新按钮顶边 %.1f 越过了本机条目的底边 %.1f", refresh.Y, local.Y+local.H)
	}
	if refresh.Y+refresh.H > first.Y {
		t.Errorf("刷新按钮底边 %.1f 越过了第一台设备行的顶边 %.1f", refresh.Y+refresh.H, first.Y)
	}
}

// TestRefreshButtonSpinsWhileRefreshing 检查刷新按钮真的会转：点击后图标在
// 转圈窗口内被旋转，窗口过后停下。像素对比只取图标本身的 16 DIP 方块，并让
// 前后两次取样都处在同一个悬停状态，否则悬停底色会污染对比结果。
//
// 取样必须早于 MyGo 的 600 ms 提示延迟：提示浮层出现后图标方块会被盖住。
func TestRefreshButtonSpinsWhileRefreshing(t *testing.T) {
	app := previewApp("")
	app.pairedDevices = []pairedDevice{
		{id: "peer-a", name: "MacBook Pro", platform: "macOS", online: true, syncEnabled: true},
	}
	app.setPage(pageDevices)

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	label := app.t(msgDevicesRefresh)
	button, ok := tt.Find(label)
	if !ok {
		t.Fatalf("设备页应显示刷新按钮 %q，实际显示：%q", label, tt.Texts())
	}
	glyph := image.Rect(int(button.X)+10, int(button.Y)+10, int(button.X)+26, int(button.Y)+26)
	sample := func() []byte {
		img := tt.Image()
		out := make([]byte, 0, glyph.Dx()*glyph.Dy()*4)
		for y := glyph.Min.Y; y < glyph.Max.Y; y++ {
			for x := glyph.Min.X; x < glyph.Max.X; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				out = append(out, byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8))
			}
		}
		return out
	}

	// 悬停底色自己有一段 120 ms 的过渡：等它走完再取静止姿态，否则下面比到
	// 的是底色渐变，而不是图标旋转。
	tt.Move(button.X+button.W/2, button.Y+button.H/2)
	time.Sleep(150 * time.Millisecond)
	tt.Frame()
	rest := sample()
	if app.deviceRefreshSpinning() {
		t.Fatal("点击之前刷新按钮不应转圈")
	}

	if err := tt.Click(label); err != nil {
		t.Fatalf("点击刷新失败：%v", err)
	}
	tt.Frame()
	if !app.deviceRefreshSpinning() {
		t.Fatal("点击刷新后刷新按钮应转圈")
	}

	time.Sleep(200 * time.Millisecond)
	tt.Frame()
	if got := sample(); bytes.Equal(got, rest) {
		t.Error("转圈期间图标应被旋转，实际与静止姿态完全相同")
	}

	time.Sleep(motionSpinPeriod)
	tt.Frame()
	if app.deviceRefreshSpinning() {
		t.Error("最短转圈时间过后刷新按钮应停下")
	}
}
