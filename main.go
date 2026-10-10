package main

import (
	"log"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	defaultWindowWidth    = 1120
	defaultWindowHeight   = 720
	minimumWindowWidth    = 900
	minimumWindowHeight   = 560
	trayDoubleClickWindow = 500 * time.Millisecond
)

type appRuntime struct {
	app           *application
	window        *mygo.Window
	tray          *mygo.Tray
	syncMenuItem  *mygo.MenuItem
	lastTrayClick time.Time
	quitting      bool
}

func main() {
	controller := &appRuntime{app: newApplication()}

	mygo.App.WhenReady(func() {
		controller.ready()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (r *appRuntime) ready() {
	r.app.onSyncStateChanged = r.syncStateChanged
	r.window = openMainWindow(r.app)
	r.window.OnClose(func(event *mygo.CloseEvent) {
		if r.quitting || r.tray == nil {
			return
		}
		event.PreventDefault()
		r.window.Hide()
	})
	r.app.loadSystemSettings()

	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		r.quitting = true
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if r.tray != nil {
			r.tray.Destroy()
		}
	})

	var err error
	r.tray, err = mygo.NewTray(mygo.TrayOptions{
		Icon:           embeddedTrayIcon,
		IconIsTemplate: runtime.GOOS == "darwin",
		ToolTip:        "GoPaste",
		Menu:           r.trayMenu(),
	})
	if err != nil {
		log.Printf("创建系统托盘失败，继续使用窗口模式: %v", err)
		r.tray = nil
		return
	}
	if runtime.GOOS == "windows" {
		r.tray.OnClick(r.handleTrayClick)
		r.tray.OnRightClick(func() { r.lastTrayClick = time.Time{} })
	}
}

// handleTrayClick 在 Windows 上把两次连续左键点击识别为双击。
// MyGo 的托盘 API 没有暴露原生双击事件，Windows 后端会分别上报两次左键点击。
func (r *appRuntime) handleTrayClick() {
	now := time.Now()
	if !r.lastTrayClick.IsZero() && now.Sub(r.lastTrayClick) <= trayDoubleClickWindow {
		r.lastTrayClick = time.Time{}
		r.showMainWindow()
		return
	}
	r.lastTrayClick = now
}

func (r *appRuntime) trayMenu() *mygo.Menu {
	r.syncMenuItem = &mygo.MenuItem{
		ID:    "tray-toggle-sync",
		Label: r.syncMenuLabel(),
		Click: func(*mygo.MenuItem, *mygo.Window) {
			r.toggleSync()
		},
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		r.syncMenuItem,
		mygo.Separator(),
		{ID: "tray-open", Label: "打开", Click: func(*mygo.MenuItem, *mygo.Window) {
			r.showMainWindow()
		}},
		{ID: "tray-settings", Label: "设置", Click: func(*mygo.MenuItem, *mygo.Window) {
			r.showSettings()
		}},
		mygo.Separator(),
		{ID: "tray-relaunch", Label: "重启", Click: func(*mygo.MenuItem, *mygo.Window) {
			mygo.App.Relaunch()
		}},
		{ID: "tray-quit", Label: "退出", Click: func(*mygo.MenuItem, *mygo.Window) {
			mygo.App.Quit()
		}},
	})
}

func (r *appRuntime) syncMenuLabel() string {
	if r.app.deviceSyncEnabled {
		return "关闭同步"
	}
	return "打开同步"
}

func (r *appRuntime) toggleSync() {
	enabled := !r.app.deviceSyncEnabled
	r.app.deviceSyncEnabled = enabled
	r.app.deviceFileSync = enabled
	for index := range r.app.pairedDevices {
		r.app.pairedDevices[index].syncEnabled = enabled
	}
	r.syncStateChanged()
	if r.window != nil {
		r.window.Invalidate()
	}
}

func (r *appRuntime) syncStateChanged() {
	if r.syncMenuItem != nil {
		r.syncMenuItem.SetLabel(r.syncMenuLabel())
	}
}

func (r *appRuntime) showMainWindow() {
	if r.window == nil {
		return
	}
	r.window.Show()
}

func (r *appRuntime) showSettings() {
	if r.window == nil {
		return
	}
	r.app.page = pageSettings
	r.window.Show()
	r.window.Invalidate()
}

func openMainWindow(app *application) *mygo.Window {
	return mygo.NewWindow(mygo.WindowOptions{
		Title:                "GoPaste",
		Width:                defaultWindowWidth,
		Height:               defaultWindowHeight,
		MinWidth:             minimumWindowWidth,
		MinHeight:            minimumWindowHeight,
		TitleBarStyle:        mygo.TitleBarHiddenInset,
		TrafficLightPosition: &mygo.Point{X: 18, Y: 11},
		TitleBarHeight:       windowTitleBarHeight,
		BackgroundColor:      "light-dark(#f5f5f7, #181a1f)",
		Vibrancy:             mygo.VibrancyNone,
		StateKey:             "main",
		Content:              ui.View(app.view),
	})
}
