package main

import (
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	defaultWindowWidth  = 1120
	defaultWindowHeight = 720
	minimumWindowWidth  = 900
	minimumWindowHeight = 560
)

type appRuntime struct {
	app      *application
	window   *mygo.Window
	tray     *mygo.Tray
	quitting bool
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
	}
}

func (r *appRuntime) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{ID: "tray-disable-sync", Label: "关闭同步", Click: func(*mygo.MenuItem, *mygo.Window) {
			r.disableSync()
		}},
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

func (r *appRuntime) disableSync() {
	r.app.deviceSyncEnabled = false
	r.app.deviceFileSync = false
	for index := range r.app.pairedDevices {
		r.app.pairedDevices[index].syncEnabled = false
	}
	if r.window != nil {
		r.window.Invalidate()
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
