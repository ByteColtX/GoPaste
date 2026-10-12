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
	text         func(messageKey) string
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
		t.syncMenuItem,
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

func (t *systemTray) syncMenuLabel() string {
	if t.options.syncEnabled != nil && t.options.syncEnabled() {
		return t.text(msgTrayTurnSyncOff)
	}
	return t.text(msgTrayTurnSyncOn)
}

// syncStateChanged 在同步开关被窗口或托盘菜单改变后更新菜单文案。
func (t *systemTray) syncStateChanged() {
	if t != nil && t.syncMenuItem != nil {
		t.syncMenuItem.SetLabel(t.syncMenuLabel())
	}
}

// refreshLabels 在界面语言变化后重建菜单，让所有托盘文案切换到新语言。
func (t *systemTray) refreshLabels() {
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
