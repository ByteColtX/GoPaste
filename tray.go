package main

import (
	"runtime"
	"time"

	"github.com/egoist/mygo"
)

const trayDoubleClickWindow = 500 * time.Millisecond

// systemTrayOptions 把托盘事件转换成应用层回调，避免托盘直接依赖 application。
type systemTrayOptions struct {
	syncEnabled  func() bool
	onToggleSync func()
	onOpen       func()
	onSettings   func()
	onRelaunch   func()
	onQuit       func()
}

// systemTray 只保存托盘自身的原生对象和交互状态。
type systemTray struct {
	native       *mygo.Tray
	syncMenuItem *mygo.MenuItem
	options      systemTrayOptions
	lastClick    time.Time
}

func newSystemTray(options systemTrayOptions) (*systemTray, error) {
	tray := &systemTray{options: options}
	tray.syncMenuItem = &mygo.MenuItem{
		ID:    "tray-toggle-sync",
		Label: tray.syncMenuLabel(),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if tray.options.onToggleSync != nil {
				tray.options.onToggleSync()
			}
		},
	}

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

func (t *systemTray) menu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		t.syncMenuItem,
		mygo.Separator(),
		{ID: "tray-open", Label: "打开", Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onOpen != nil {
				t.options.onOpen()
			}
		}},
		{ID: "tray-settings", Label: "设置", Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onSettings != nil {
				t.options.onSettings()
			}
		}},
		mygo.Separator(),
		{ID: "tray-relaunch", Label: "重启", Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onRelaunch != nil {
				t.options.onRelaunch()
			}
		}},
		{ID: "tray-quit", Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) {
			if t.options.onQuit != nil {
				t.options.onQuit()
			}
		}},
	})
}

func (t *systemTray) syncMenuLabel() string {
	if t.options.syncEnabled != nil && t.options.syncEnabled() {
		return "关闭同步"
	}
	return "打开同步"
}

// syncStateChanged 在同步开关被窗口或托盘菜单改变后更新菜单文案。
func (t *systemTray) syncStateChanged() {
	if t != nil && t.syncMenuItem != nil {
		t.syncMenuItem.SetLabel(t.syncMenuLabel())
	}
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
