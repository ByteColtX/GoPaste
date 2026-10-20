package main

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
)

const trayDoubleClickWindow = 500 * time.Millisecond

// trayDevice 是托盘「设备同步」子菜单需要的一台已配对设备。
type trayDevice struct {
	id     string
	name   string
	syncOn bool
}

// systemTrayOptions 把托盘事件转换成应用层回调，避免托盘直接依赖 application。
type systemTrayOptions struct {
	syncEnabled func() bool
	// capturePaused 报告捕获是否被用户暂停，菜单项据此在「暂停捕获」与「恢复
	// 捕获」之间切换文案。
	capturePaused func() bool
	// devices 返回当前已配对设备，托盘据此重建「设备同步」子菜单。
	devices            func() []trayDevice
	text               func(messageKey) string
	onToggleSync       func()
	onToggleDeviceSync func(id string)
	onToggleCapture    func()
	onOpen             func()
	onSettings         func()
	onRelaunch         func()
	onQuit             func()
}

// systemTray 只保存托盘自身的原生对象和交互状态。
type systemTray struct {
	native *mygo.Tray
	// captureMenuItem 是暂停捕获项，文案随暂停状态切换。
	captureMenuItem *mygo.MenuItem
	syncMenuItem    *mygo.MenuItem
	// deviceItems 记录每台设备在子菜单里的勾选项，用于在状态变化后同步勾选。
	deviceItems map[string]*mygo.MenuItem
	options     systemTrayOptions
	lastClick   time.Time
}

func newSystemTray(options systemTrayOptions) (*systemTray, error) {
	tray := &systemTray{options: options}
	native, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           embeddedTrayIcon,
		IconIsTemplate: runtime.GOOS == "darwin",
		ToolTip:        "GoPaste",
		Menu:           tray.menu(),
	})
	if err != nil {
		return nil, err
	}
	tray.native = native
	if runtime.GOOS == "windows" {
		native.OnClick(tray.handleClick)
		native.OnRightClick(tray.resetClick)
	}
	return tray, nil
}

// text 返回当前界面语言下的托盘文案。
func (t *systemTray) text(key messageKey) string {
	if t.options.text == nil {
		return ""
	}
	return t.options.text(key)
}

func (t *systemTray) menu() *mygo.Menu {
	// 暂停捕获放在最上面：它是本地隐私开关，不跟下面的同步菜单混在一起。
	t.captureMenuItem = &mygo.MenuItem{
		ID:    "tray-toggle-capture",
		Label: t.captureMenuLabel(),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onToggleCapture != nil {
				t.options.onToggleCapture()
			}
		},
	}
	t.syncMenuItem = &mygo.MenuItem{
		ID:    "tray-toggle-sync",
		Label: t.syncMenuLabel(),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onToggleSync != nil {
				t.options.onToggleSync()
			}
		},
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		t.captureMenuItem,
		t.syncMenuItem,
		{
			ID:      "tray-device-sync",
			Label:   t.text(msgTrayDeviceSync),
			Submenu: t.deviceSubmenu(),
		},
		mygo.Separator(),
		{ID: "tray-open", Label: t.text(msgTrayOpen), Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onOpen != nil {
				t.options.onOpen()
			}
		}},
		{ID: "tray-settings", Label: t.text(msgTraySettings), Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onSettings != nil {
				t.options.onSettings()
			}
		}},
		mygo.Separator(),
		{ID: "tray-relaunch", Label: t.text(msgTrayRelaunch), Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onRelaunch != nil {
				t.options.onRelaunch()
			}
		}},
		{ID: "tray-quit", Label: t.text(msgTrayQuit), Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onQuit != nil {
				t.options.onQuit()
			}
		}},
	})
}

// deviceSubmenu 为每台已配对设备生成一个勾选项：勾选表示这台设备参与同步。
// 没有已配对设备时放一个禁用的占位项，让子菜单仍然可展开并说明原因。
func (t *systemTray) deviceSubmenu() []*mygo.MenuItem {
	t.deviceItems = make(map[string]*mygo.MenuItem)
	var devices []trayDevice
	if t.options.devices != nil {
		devices = t.options.devices()
	}
	if len(devices) == 0 {
		return []*mygo.MenuItem{{
			ID:       "tray-device-sync-empty",
			Label:    t.text(msgTrayNoDevices),
			Disabled: true,
		}}
	}
	items := make([]*mygo.MenuItem, 0, len(devices))
	for _, device := range devices {
		device := device
		item := &mygo.MenuItem{
			ID:      "tray-device-sync-" + device.id,
			Label:   device.name,
			Type:    mygo.MenuItemCheckbox,
			Checked: device.syncOn,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				if t.options.onToggleDeviceSync != nil {
					t.options.onToggleDeviceSync(device.id)
				}
			},
		}
		t.deviceItems[device.id] = item
		items = append(items, item)
	}
	return items
}

func (t *systemTray) syncMenuLabel() string {
	if t.options.syncEnabled != nil && t.options.syncEnabled() {
		return t.text(msgTrayTurnSyncOff)
	}
	return t.text(msgTrayTurnSyncOn)
}

// captureMenuLabel 返回暂停项当前的文案：暂停时说「恢复捕获」，否则说「暂停捕获」。
func (t *systemTray) captureMenuLabel() string {
	if t.options.capturePaused != nil && t.options.capturePaused() {
		return t.text(msgCaptureResume)
	}
	return t.text(msgCapturePause)
}

// captureStateChanged 在捕获被暂停或恢复后更新菜单文案。与 syncStateChanged 一样
// 只改文案不重建菜单：该方法在托盘菜单跟踪点击的过程中同步执行。
func (t *systemTray) captureStateChanged() {
	if t == nil {
		return
	}
	if t.captureMenuItem != nil {
		t.captureMenuItem.SetLabel(t.captureMenuLabel())
	}
}

// syncStateChanged 在同步开关被窗口或托盘菜单改变后更新菜单文案和勾选状态。
// 这里不重建菜单：该方法会在托盘菜单跟踪点击的过程中同步执行，重建可能打断
// 原生菜单的展开状态。
func (t *systemTray) syncStateChanged() {
	if t == nil {
		return
	}
	if t.syncMenuItem != nil {
		t.syncMenuItem.SetLabel(t.syncMenuLabel())
	}
	if t.options.devices == nil {
		return
	}
	for _, device := range t.options.devices() {
		if item, ok := t.deviceItems[device.id]; ok {
			item.SetChecked(device.syncOn)
		}
	}
}

// refreshMenu 重建整个托盘菜单。界面语言变化或已配对设备增删后调用，让所有文案
// 和子菜单项都跟上最新状态。
func (t *systemTray) refreshMenu() {
	if t == nil || t.native == nil {
		return
	}
	t.native.SetMenu(t.menu())
}

// handleClick 在 Windows 上把两次连续左键点击识别为双击。
// MyGo 的托盘 API 没有暴露原生双击事件，Windows 后端会分别上报两次点击。
func (t *systemTray) handleClick() {
	now := time.Now()
	if !t.lastClick.IsZero() && now.Sub(t.lastClick) <= trayDoubleClickWindow {
		t.lastClick = time.Time{}
		if t.options.onOpen != nil {
			t.options.onOpen()
		}
		return
	}
	t.lastClick = now
}

func (t *systemTray) resetClick() {
	t.lastClick = time.Time{}
}

func (t *systemTray) Destroy() {
	if t != nil && t.native != nil {
		t.native.Destroy()
		t.native = nil
	}
}
