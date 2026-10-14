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
	// 历史页现在由「筛选栏 + 历史列表 + 预览」三栏组成，最小宽度必须容下
	// 208 的筛选栏、300 的列表和 460 的预览。
	minimumWindowWidth  = 1000
	minimumWindowHeight = 560
)

// applicationRuntime 负责把应用状态和 MyGo 的窗口生命周期连接起来。
// 托盘菜单的具体实现放在 systemTray 中，这里只提供窗口和业务回调。
type applicationRuntime struct {
	app    *application
	window *mygo.Window
	// appMenuSettings 是 macOS 应用菜单里的「设置…」项。语言变化时只更新它的
	// 标签，不必重建整个菜单。
	appMenuSettings *mygo.MenuItem
	tray            *systemTray
	quitting        bool
}

// mygoClipboardWriter is the production adapter for history.Store's local
// clipboard seam. History itself does not depend on MyGo.
type mygoClipboardWriter struct{}

func (mygoClipboardWriter) WriteText(text string) {
	mygo.Clipboard.WriteText(text)
}

func (mygoClipboardWriter) WriteImage(png []byte) error {
	return mygo.Clipboard.WriteImage(png)
}

func newApplicationRuntime() *applicationRuntime {
	return &applicationRuntime{app: newApplication()}
}

// runApplication 注册应用级事件，并把控制权交给 MyGo 的事件循环。
func runApplication() error {
	controller := newApplicationRuntime()
	mygo.App.OnSecondInstance(func([]string, string) {
		controller.showMainWindow()
	})
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}

	mygo.App.WhenReady(controller.ready)
	return mygo.App.Run()
}

// ready 在 MyGo 完成平台初始化后运行，此时才能创建窗口和托盘。
func (r *applicationRuntime) ready() {
	r.app.applySystemLanguage()
	r.app.onSyncStateChanged = r.syncStateChanged
	r.app.onLanguageChanged = r.languageChanged
	r.app.clipboard = mygoClipboardWriter{}
	r.window = openMainWindow(r.app)
	r.window.OnClose(func(event *mygo.CloseEvent) {
		if r.quitting || r.tray == nil {
			return
		}
		event.PreventDefault()
		r.window.Hide()
	})
	// macOS：关闭（隐藏）主窗口后，点击程序坞图标应重新显示窗口。
	// 该回调由 applicationShouldHandleReopen 触发。
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if hasVisibleWindows {
			return
		}
		r.showMainWindow()
	})
	r.app.loadSystemSettings()
	r.installAppMenu()

	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		r.quitting = true
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if r.tray != nil {
			r.tray.Destroy()
		}
	})

	tray, err := newSystemTray(systemTrayOptions{
		syncEnabled: func() bool { return r.app.deviceSyncEnabled },
		text:        r.app.t,
		onToggleSync: func() {
			r.app.setSyncEnabled(!r.app.deviceSyncEnabled)
		},
		onOpen:     r.showMainWindow,
		onSettings: r.showSettings,
		onRelaunch: mygo.App.Relaunch,
		onQuit:     mygo.App.Quit,
	})
	if err != nil {
		log.Printf("创建系统托盘失败，继续使用窗口模式: %v", err)
		return
	}
	r.tray = tray
}

// installAppMenu 安装应用菜单，并在其中加入 HIG › Settings 约定的「设置…」项
// （标准快捷键 Cmd+,）。只在 macOS 安装：其他平台的设置入口是托盘菜单和窗口内
// 的 Ctrl+, 快捷键，而安装菜单会给每个窗口多出一条菜单栏。
func (r *applicationRuntime) installAppMenu() {
	if runtime.GOOS != "darwin" {
		return
	}
	r.appMenuSettings = &mygo.MenuItem{
		ID:          "app-settings",
		Label:       r.app.t(msgAppMenuSettings),
		Accelerator: "Cmd+,",
		Click:       func(*mygo.MenuItem, *mygo.Window) { r.showSettings() },
	}
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			mygo.Separator(),
			r.appMenuSettings,
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	}))
}

// syncStateChanged 让托盘文案和窗口在同步开关变化后同时刷新。
func (r *applicationRuntime) syncStateChanged() {
	if r.tray != nil {
		r.tray.syncStateChanged()
	}
	if r.window != nil {
		r.window.Invalidate()
	}
}

// languageChanged 在界面语言变化后重建托盘菜单并重绘窗口。
func (r *applicationRuntime) languageChanged() {
	if r.tray != nil {
		r.tray.refreshLabels()
	}
	if r.appMenuSettings != nil {
		r.appMenuSettings.SetLabel(r.app.t(msgAppMenuSettings))
	}
	if r.window != nil {
		r.window.Invalidate()
	}
}

func (r *applicationRuntime) showMainWindow() {
	if r.window == nil {
		return
	}
	r.window.Show()
}

func (r *applicationRuntime) showSettings() {
	if r.window == nil {
		return
	}
	r.app.setPage(pageSettings)
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
		TrafficLightPosition: &mygo.Point{X: windowControlsInset, Y: windowControlsTopInset},
		BackgroundColor:      "light-dark(#ffffff, #1c1c1e)",
		Vibrancy:             mygo.VibrancyNone,
		StateKey:             "main",
		Content:              ui.View(app.view),
	})
}
