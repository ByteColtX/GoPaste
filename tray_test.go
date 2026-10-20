package main

import (
	"testing"

	"github.com/egoist/mygo"
)

// trayTestTray 构造一个不依赖原生托盘的 systemTray：只填充菜单生成需要的数据，
// 菜单模板本身不需要 MyGo 应用就绪。
func trayTestTray(devices []trayDevice, toggled *string) *systemTray {
	return &systemTray{options: systemTrayOptions{
		syncEnabled:   func() bool { return true },
		capturePaused: func() bool { return false },
		devices:       func() []trayDevice { return devices },
		text:          newLocalizer(languageSimplifiedChinese).text,
		onToggleDeviceSync: func(id string) {
			if toggled != nil {
				*toggled = id
			}
		},
	}}
}

// findMenuItem 在菜单顶层按 ID 查找菜单项。
func findMenuItem(menu *mygo.Menu, id string) *mygo.MenuItem {
	for _, item := range menu.Items() {
		if item.ID == id {
			return item
		}
	}
	return nil
}

// TestTrayDeviceSyncSubmenuFollowsToggle 检查「设备同步」子菜单紧跟同步开关，
// 并且每台已配对设备各占一个勾选项，勾选状态对应该设备的同步开关。
func TestTrayDeviceSyncSubmenuFollowsToggle(t *testing.T) {
	tray := trayTestTray([]trayDevice{
		{id: "peer-a", name: "MacBook Pro", syncOn: true},
		{id: "peer-b", name: "工作室 PC", syncOn: false},
	}, nil)

	menu := tray.menu()
	items := menu.Items()
	if len(items) < 3 {
		t.Fatalf("托盘菜单至少要有暂停捕获、同步开关和设备同步三项，实际 %d 项", len(items))
	}
	if items[0].ID != "tray-toggle-capture" {
		t.Fatalf("托盘菜单第一项应是暂停捕获，实际是 %q", items[0].ID)
	}
	if items[1].ID != "tray-toggle-sync" {
		t.Fatalf("托盘菜单第二项应是同步开关，实际是 %q", items[1].ID)
	}
	if items[2].ID != "tray-device-sync" {
		t.Fatalf("设备同步子菜单应紧跟在同步开关下面，实际第三项是 %q", items[2].ID)
	}
	if got := items[2].Label; got != "设备同步" {
		t.Errorf("子菜单标题 = %q，想要 %q", got, "设备同步")
	}

	submenu := items[2].Submenu
	if len(submenu) != 2 {
		t.Fatalf("子菜单应列出 2 台设备，实际 %d 项", len(submenu))
	}
	want := []struct {
		name    string
		checked bool
	}{
		{"MacBook Pro", true},
		{"工作室 PC", false},
	}
	for index, device := range want {
		if submenu[index].Type != mygo.MenuItemCheckbox {
			t.Errorf("%q 应是勾选项，实际类型 %q", device.name, submenu[index].Type)
		}
		if submenu[index].Label != device.name {
			t.Errorf("第 %d 项标签 = %q，想要 %q", index, submenu[index].Label, device.name)
		}
		if submenu[index].Checked != device.checked {
			t.Errorf("%q 的勾选状态 = %v，想要 %v", device.name, submenu[index].Checked, device.checked)
		}
	}
}

// TestTrayCaptureItemPausesAndResumes 检查托盘第一项是暂停捕获：文案在「暂停捕获」
// 与「恢复捕获」之间切换，点击把切换交回应用层。
func TestTrayCaptureItemPausesAndResumes(t *testing.T) {
	paused := false
	toggled := 0
	tray := &systemTray{options: systemTrayOptions{
		capturePaused: func() bool { return paused },
		text:          newLocalizer(languageSimplifiedChinese).text,
		onToggleCapture: func() {
			toggled++
			paused = !paused
		},
	}}

	item := tray.menu().Items()[0]
	if item.ID != "tray-toggle-capture" {
		t.Fatalf("托盘菜单第一项应是暂停捕获，实际是 %q", item.ID)
	}
	if got, want := item.Label, tray.text(msgCapturePause); got != want {
		t.Errorf("未暂停时的文案 = %q，想要 %q", got, want)
	}

	item.Click(item, nil)
	if toggled != 1 || !paused {
		t.Fatalf("点击后回调 %d 次，暂停状态 = %v", toggled, paused)
	}
	// 应用改完状态后让托盘跟上文案，与 syncStateChanged 同一个做法。
	tray.captureStateChanged()
	if got, want := item.Label, tray.text(msgCaptureResume); got != want {
		t.Errorf("暂停后的文案 = %q，想要 %q", got, want)
	}
}

// TestTrayDeviceSyncSubmenuClickTogglesDevice 检查点击勾选项会把设备 ID 交回应用层。
func TestTrayDeviceSyncSubmenuClickTogglesDevice(t *testing.T) {
	var toggled string
	tray := trayTestTray([]trayDevice{
		{id: "peer-a", name: "MacBook Pro", syncOn: true},
		{id: "peer-b", name: "工作室 PC", syncOn: false},
	}, &toggled)

	submenu := findMenuItem(tray.menu(), "tray-device-sync").Submenu
	submenu[1].Click(submenu[1], nil)
	if toggled != "peer-b" {
		t.Errorf("点击第二台设备后回调参数 = %q，想要 %q", toggled, "peer-b")
	}
}

// TestTrayDeviceSyncSubmenuEmptyState 检查没有已配对设备时子菜单仍可展开，
// 用一个禁用的占位项说明原因。
func TestTrayDeviceSyncSubmenuEmptyState(t *testing.T) {
	tray := trayTestTray(nil, nil)

	submenu := findMenuItem(tray.menu(), "tray-device-sync").Submenu
	if len(submenu) != 1 {
		t.Fatalf("没有已配对设备时子菜单应只有占位项，实际 %d 项", len(submenu))
	}
	if !submenu[0].Disabled {
		t.Errorf("占位项应是禁用的")
	}
	placeholder := newLocalizer(languageSimplifiedChinese).text(msgTrayNoDevices)
	if submenu[0].Label != placeholder {
		t.Errorf("占位项文案 = %q，想要 %q", submenu[0].Label, placeholder)
	}
}

// TestToggleDeviceSyncOnlyChangesTarget 检查托盘里单独开关一台设备时只改这台设备，
// 并且会通知运行时刷新界面。
func TestToggleDeviceSyncOnlyChangesTarget(t *testing.T) {
	app := newApplication()
	app.pairedDevices = []pairedDevice{
		{id: "peer-a", name: "MacBook Pro", syncEnabled: true},
		{id: "peer-b", name: "工作室 PC", syncEnabled: true},
	}
	notified := 0
	app.onSyncStateChanged = func() { notified++ }

	app.toggleDeviceSync("peer-b")

	if app.pairedDevices[0].syncEnabled != true {
		t.Errorf("未点击的设备同步开关不应改变")
	}
	if app.pairedDevices[1].syncEnabled != false {
		t.Errorf("被点击的设备同步开关应被关闭")
	}
	if notified != 1 {
		t.Errorf("状态变化通知次数 = %d，想要 1", notified)
	}

	// 设备已被取消配对时不应改到别的设备，也不应再通知。
	app.toggleDeviceSync("missing")
	if notified != 1 {
		t.Errorf("未知设备不应触发状态变化通知，实际次数 %d", notified)
	}
}
