package main

import (
	"testing"

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
