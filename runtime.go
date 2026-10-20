package main

import (
	"context"
	"errors"
	"log"
	"runtime"
	"time"

	"gopaste/internal/capture"
	"gopaste/internal/filescache"
	"gopaste/internal/history"
	"gopaste/internal/lansync"

	"gopaste/internal/settings"

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
	// databasePath 是命令行或环境变量指定的历史数据库文件；为空时用默认位置。
	databasePath string
	// appMenuSettings 是 macOS 应用菜单里的「设置…」项。语言变化时只更新它的
	// 标签，不必重建整个菜单。
	appMenuSettings *mygo.MenuItem
	// viewMenu、menuLabels 是自建的视图菜单与它的静态文案表，语言变化时逐项改
	// 标签。
	viewMenu   *mygo.MenuItem
	menuLabels map[*mygo.MenuItem]messageKey
	// viewFilterRailItem 是视图菜单里的「收起/展开侧栏」项。它的文案随侧栏展开
	// 状态变化，所以不能放进 menuLabels 的固定文案表。
	viewFilterRailItem *mygo.MenuItem
	// viewRefreshItem、fileAddDeviceItem 把 Cmd+R 与 Cmd+N 摆上菜单栏，让快捷键
	// 可发现。刷新只在设备页有意义，因此随分区切换置灰；新增设备在任何分区都能
	// 用，点击会先切到设备页。
	viewRefreshItem   *mygo.MenuItem
	fileAddDeviceItem *mygo.MenuItem
	// editUndo 是编辑菜单里的「撤销」项。GoPaste 自己的撤销优先于输入框的文本
	// 撤销，没有可撤销的操作时置灰，Cmd+Z 于是落回输入框自身。
	editUndo *mygo.MenuItem
	tray     *systemTray
	quitting bool
	// captureCancel 结束剪贴板监听；为 nil 表示监听没有启动。
	captureCancel context.CancelFunc
	// captureSuspended 记下暂停捕获那一刻监听是否真的在跑：数据库没打开时监听
	// 根本没启动，恢复时不能凭空把它启动起来。
	captureSuspended bool
	// syncCancel 结束局域网同步服务；为 nil 表示服务没有启动。
	syncCancel context.CancelFunc
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

// WriteFiles 把收到的文件本身写进剪贴板（文件列表，不是路径文本），对方因此能直接
// 粘贴出文件。MyGo 只接受绝对路径。
func (mygoClipboardWriter) WriteFiles(paths ...string) error {
	return mygo.Clipboard.WriteFiles(paths...)
}

func newApplicationRuntime(databasePath string) *applicationRuntime {
	return &applicationRuntime{app: newApplication(), databasePath: databasePath}
}

// runApplication 注册应用级事件，并把控制权交给 MyGo 的事件循环。
// databasePath 为空时使用默认的历史数据库位置。
func runApplication(databasePath string) error {
	controller := newApplicationRuntime(databasePath)
	// 先告诉 MyGo 应用写了哪些语言，它才能把用户的偏好语言解析成 Locale()：
	// 不声明的话 MyGo 认为应用只写了英语，「跟随系统」永远解析成 en。
	mygo.App.SetLanguages(supportedLanguages()...)
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
	r.app.onSyncStateChanged = r.syncStateChanged
	r.app.onCapturePausedChanged = r.capturePausedChanged
	r.app.onLanguageChanged = r.languageChanged
	r.app.onDevicesChanged = r.devicesChanged
	r.app.onFilterRailToggled = r.filterRailToggled
	r.app.onPageChanged = r.pageChanged
	r.app.onMeasureStorageUsage = r.measureStorageUsage
	r.app.onCleanFileCache = r.cleanFileCache
	r.app.onChooseDirectory = r.chooseDirectory
	r.app.onUndoAvailabilityChanged = r.undoAvailabilityChanged
	r.app.onZoomChanged = r.zoomChanged
	r.app.onMaxZoom = r.maxZoom
	// 系统语言变了 MyGo 会实时上报：跟随系统时界面语言跟着换，固定某种语言时
	// 不动——用户的显式选择优先。
	mygo.App.OnLocaleChanged(func(string) {
		if r.app.languagePreference != languagePreferenceSystem {
			return
		}
		r.app.applyLanguage()
	})
	r.app.clipboard = mygoClipboardWriter{}
	// 设置要在窗口打开之前读：首帧就知道要不要显示首次启动向导，用哪种语言、哪种
	// 外观，以及缩放多少，否则界面会先闪一下默认值再切过去。语言与外观要在主线程
	// 上解析并应用，所以放在读完之后、开窗之前。
	r.loadSettings()
	r.app.applyLanguage()
	r.app.applyAppearance()
	r.window = openMainWindow(r.app)
	// 恢复的缩放可能来自更大的屏幕：窗口有了所在屏幕之后按可用区域再夹一次，再按
	// 这个比例调整窗口最小尺寸与尺寸。
	r.app.clampZoomToScreen(r.maxZoom())
	r.zoomChanged(r.app.zoom)
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
		// 先停同步服务，再停监听、等写队列落盘、关掉数据库：退出时既不会丢掉刚
		// 复制的记录，也不会再有同步回调打到正在关闭的窗口上。
		r.stopSync()
		r.stopCapture()
		if r.app.persistence != nil {
			r.app.persistence.Close()
		}
		if r.tray != nil {
			r.tray.Destroy()
		}
	})

	tray, err := newSystemTray(systemTrayOptions{
		syncEnabled:   func() bool { return r.app.syncEnabled },
		capturePaused: func() bool { return r.app.capturePaused },
		devices:       r.app.trayDevices,
		text:          r.app.t,
		onToggleSync: func() {
			r.app.setSyncEnabled(!r.app.syncEnabled)
		},
		onToggleCapture: func() {
			r.app.setCapturePaused(!r.app.capturePaused)
		},
		onToggleDeviceSync: r.app.toggleDeviceSync,
		onOpen:             r.showMainWindow,
		onSettings:         r.showSettings,
		onRelaunch:         mygo.App.Relaunch,
		onQuit:             mygo.App.Quit,
	})
	if err != nil {
		log.Printf("创建系统托盘失败，继续使用窗口模式: %v", err)
	} else {
		r.tray = tray
	}

	// 捕获与同步的上下文都在主线程上创建，退出时由 stopCapture / stopSync 取消。
	captureCtx, captureCancel := context.WithCancel(context.Background())
	r.captureCancel = captureCancel
	r.startHistoryPersistence(captureCtx)

	syncCtx, syncCancel := context.WithCancel(context.Background())
	r.syncCancel = syncCancel
	r.startSyncService(syncCtx)
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
	// 视图菜单自己搭，不用 RoleViewMenu：那套默认项（重新加载、开发者工具、
	// 页面缩放）面向网页，原生界面里没有意义。
	zoomIn := &mygo.MenuItem{ID: "view-zoom-in", Label: r.app.t(msgMenuZoomIn), Accelerator: "Cmd+Plus",
		Click: func(*mygo.MenuItem, *mygo.Window) { r.app.zoomIn(); r.invalidate() }}
	zoomOut := &mygo.MenuItem{ID: "view-zoom-out", Label: r.app.t(msgMenuZoomOut), Accelerator: "Cmd+-",
		Click: func(*mygo.MenuItem, *mygo.Window) { r.app.zoomOut(); r.invalidate() }}
	zoomReset := &mygo.MenuItem{ID: "view-zoom-reset", Label: r.app.t(msgMenuZoomReset), Accelerator: "Cmd+0",
		Click: func(*mygo.MenuItem, *mygo.Window) { r.app.resetZoom(); r.invalidate() }}
	// 侧栏项与标题栏按钮、Cmd+Ctrl+S 走同一条路径；文案随状态在「收起」与
	// 「展开」之间切换，与 macOS 的 Show/Hide Sidebar 一致。
	r.viewFilterRailItem = &mygo.MenuItem{
		ID:          "view-filter-rail",
		Label:       r.app.t(r.filterRailMenuKey()),
		Accelerator: "Ctrl+Cmd+S",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			r.app.toggleFilterRail()
			r.invalidate()
		},
	}
	// 刷新只在设备页有可见反馈（侧栏的设备列表与转圈图标），所以离开设备页即
	// 置灰，与窗口快捷键的作用范围一致。
	r.viewRefreshItem = &mygo.MenuItem{
		ID:          "view-refresh-devices",
		Label:       r.app.t(msgDevicesRefresh),
		Accelerator: "Cmd+R",
		Disabled:    r.app.page != pageDevices,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			r.app.requestDeviceRefresh()
			r.invalidate()
		},
	}
	r.viewMenu = &mygo.MenuItem{
		ID:    "view-menu",
		Label: r.app.t(msgMenuView),
		Submenu: []*mygo.MenuItem{
			r.viewFilterRailItem,
			mygo.Separator(),
			zoomIn, zoomOut, zoomReset,
			mygo.Separator(),
			r.viewRefreshItem,
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		},
	}
	// 文件菜单也自己搭：在默认的「关闭窗口」前面加一项「新增设备」，Cmd+N 因此
	// 在菜单栏可见。从任何分区都能用，点击先切到设备页再开配对对话框。
	r.fileAddDeviceItem = &mygo.MenuItem{
		ID:          "file-add-device",
		Label:       r.app.t(msgDevicesAdd),
		Accelerator: "Cmd+N",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			r.app.setPage(pageDevices)
			r.app.openAddDeviceDialog()
			r.invalidate()
		},
	}
	fileMenu := &mygo.MenuItem{
		Role: mygo.RoleFileMenu,
		Submenu: []*mygo.MenuItem{
			r.fileAddDeviceItem,
			mygo.Separator(),
			{Role: mygo.RoleClose},
		},
	}
	// 编辑菜单自己搭，只为把「撤销」换成 GoPaste 自己的撤销：默认的 RoleUndo 走
	// 响应链，只认得输入框里的文本撤销，会先把 Cmd+Z 吃掉。其余项沿用系统角色，
	// 剪贴板、选择与重做行为不变（系统朗读子菜单不适用于本应用，故不再列出）。
	r.editUndo = &mygo.MenuItem{
		ID:          "edit-undo",
		Label:       r.app.t(msgMenuUndo),
		Accelerator: "Cmd+Z",
		Disabled:    true,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			r.app.requestUndo()
			r.invalidate()
		},
	}
	editMenu := &mygo.MenuItem{
		Role: mygo.RoleEditMenu,
		Submenu: []*mygo.MenuItem{
			r.editUndo,
			{Role: mygo.RoleRedo},
			mygo.Separator(),
			{Role: mygo.RoleCut},
			{Role: mygo.RoleCopy},
			{Role: mygo.RolePaste},
			{Role: mygo.RolePasteAndMatchStyle},
			{Role: mygo.RoleDelete},
			mygo.Separator(),
			{Role: mygo.RoleSelectAll},
		},
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
		fileMenu,
		editMenu,
		r.viewMenu,
		{Role: mygo.RoleWindowMenu},
	}))
	r.menuLabels = map[*mygo.MenuItem]messageKey{
		zoomIn:              msgMenuZoomIn,
		zoomOut:             msgMenuZoomOut,
		zoomReset:           msgMenuZoomReset,
		r.editUndo:          msgMenuUndo,
		r.fileAddDeviceItem: msgDevicesAdd,
		r.viewRefreshItem:   msgDevicesRefresh,
	}
}

// loadSettings 读取设置文件并应用到界面状态。它在主窗口打开之前同步运行：
// 设置文件只有几百字节，读它换来的是首帧就知道要不要显示向导。
//
// 拿不到位置或文件读不出来都只记日志：读不出来按首次启动处理（用户会重新看到
// 向导），比带着一份坏设置继续跑要好。
func (r *applicationRuntime) loadSettings() {
	path, err := settingsPath()
	if err != nil {
		log.Printf("无法确定设置文件位置，本次不读写设置: %v", err)
		return
	}
	r.app.settingsPath = path
	file, err := settings.Load(path)
	if err != nil {
		log.Printf("无法读取设置 %s，按首次启动处理: %v", path, err)
		file = settings.File{}
	}
	r.app.applySettings(file)
}

// zoomChanged 在界面缩放变化后调整窗口：最小尺寸按比例放大，窗口不够大就长到
// 刚好放得下，并且不超过所在屏幕的可用区域，最后刷新窗口。
func (r *applicationRuntime) zoomChanged(zoom float32) {
	if r.window == nil {
		return
	}
	minWidth, minHeight := zoomedWindowMinimum(zoom)
	r.window.SetMinimumSize(minWidth, minHeight)
	width, height := r.window.Size()
	if nextWidth, nextHeight := windowSizeForZoom(width, height, zoom, r.workArea()); nextWidth != width || nextHeight != height {
		r.window.SetSize(nextWidth, nextHeight)
	}
	r.window.Invalidate()
}

// maxZoom 返回窗口所在屏幕允许的最大缩放：界面最小尺寸按比例放大后仍要装得
// 进屏幕可用区域。屏幕未知时返回 0，由应用侧的内置上限决定。
func (r *applicationRuntime) maxZoom() float32 {
	return zoomCeilingForArea(r.workArea())
}

// workArea 返回主窗口所在屏幕的可用区域（不含菜单栏、程序坞与任务栏）。
func (r *applicationRuntime) workArea() mygo.Rectangle {
	if r.window == nil {
		return mygo.Rectangle{}
	}
	bounds := r.window.Bounds()
	display := mygo.Screen.DisplayNearestPoint(mygo.Point{
		X: bounds.X + bounds.Width/2,
		Y: bounds.Y + bounds.Height/2,
	})
	return display.WorkArea
}

// invalidate 在窗口存在时请求重绘。菜单项与缩放回调都在窗口可能尚未创建或
// 已经关闭时运行，所以这里统一挡一层。
func (r *applicationRuntime) invalidate() {
	if r.window != nil {
		r.window.Invalidate()
	}
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

// capturePausedChanged 在用户暂停或恢复捕获后真正停下或重启剪贴板监听。
//
// 暂停是隐私开关，所以这里取消捕获上下文：Loop 随之退出，平台侧的监听也跟着结束，
// 暂停期间不读剪贴板。恢复时重新开始监听，但不补记暂停期间复制的内容（见
// capture.Options.SkipStartRead）。
func (r *applicationRuntime) capturePausedChanged(paused bool) {
	if paused {
		r.captureSuspended = r.captureCancel != nil
		r.stopCapture()
	} else if r.captureSuspended {
		r.captureSuspended = false
		ctx, cancel := context.WithCancel(context.Background())
		r.captureCancel = cancel
		r.startClipboardCapture(ctx, capture.Options{SkipStartRead: true})
	}
	if r.tray != nil {
		r.tray.captureStateChanged()
	}
	r.invalidate()
}

// languageChanged 在界面语言变化后重建托盘菜单并重绘窗口。
func (r *applicationRuntime) languageChanged() {
	if r.tray != nil {
		r.tray.refreshMenu()
	}
	if r.appMenuSettings != nil {
		r.appMenuSettings.SetLabel(r.app.t(msgAppMenuSettings))
	}
	if r.viewMenu != nil {
		r.viewMenu.SetLabel(r.app.t(msgMenuView))
	}
	for item, key := range r.menuLabels {
		item.SetLabel(r.app.t(key))
	}
	r.updateFilterRailItem()
	if r.window != nil {
		r.window.Invalidate()
	}
}

// pageChanged 让只在设备页有意义的菜单项跟上启用状态：刷新设备离开设备页即
// 置灰，与窗口快捷键的作用范围一致。
func (r *applicationRuntime) pageChanged() {
	if r.viewRefreshItem == nil {
		return
	}
	r.viewRefreshItem.SetEnabled(r.app.page == pageDevices)
}

// filterRailMenuKey 返回侧栏菜单项当前的文案：展开时说「收起」，收起时说「展开」。
func (r *applicationRuntime) filterRailMenuKey() messageKey {
	if r.app.filterRailOpen {
		return msgFilterRailCollapse
	}
	return msgFilterRailExpand
}

// updateFilterRailItem 让侧栏菜单项的文案跟上展开状态与界面语言。
func (r *applicationRuntime) updateFilterRailItem() {
	if r.viewFilterRailItem == nil {
		return
	}
	r.viewFilterRailItem.SetLabel(r.app.t(r.filterRailMenuKey()))
}

// filterRailToggled 在侧栏被标题栏按钮、快捷键或菜单项切换后更新菜单文案。
func (r *applicationRuntime) filterRailToggled() {
	r.updateFilterRailItem()
}

// undoAvailabilityChanged 让编辑菜单里的「撤销」跟着撤销栈启用或置灰：置灰时
// Cmd+Z 没有菜单项接手，会落回窗口，输入框仍能自己撤销文本。
func (r *applicationRuntime) undoAvailabilityChanged() {
	if r.editUndo == nil {
		return
	}
	r.editUndo.SetEnabled(len(r.app.pendingUndos) > 0)
}

// devicesChanged 在已配对设备增删后重建托盘菜单，让「设备同步」子菜单跟上列表。
func (r *applicationRuntime) devicesChanged() {
	if r.tray != nil {
		r.tray.refreshMenu()
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

// startSyncService 启动局域网同步服务，并把它的结果经 Window.Update 送回界面线程。
//
// 服务自己开 goroutine 做发现、配对与投递，回调都不在界面线程上：这里统一转交给
// Window.Update，界面状态因此仍然只在主线程上改动。
func (r *applicationRuntime) startSyncService(ctx context.Context) {
	path, err := deviceDatabasePath()
	if err != nil {
		log.Printf("无法确定设备配对信息位置，本次不启用局域网同步: %v", err)
		r.publishDiscoveryFailure(r.app.t(msgDevicesDiscoveryUnavailable))
		return
	}
	// 收到的文件默认落在数据目录下，与历史数据库、配对信息同一处；用户在同步页
	// 选过别的目录就用那一个。拿不到默认位置又不接收文件时，其它类型的同步照常，
	// 而不是把文件写到某个猜出来的地方。
	if dir, err := receivedFilesDirectory(); err != nil {
		log.Printf("无法确定接收文件的位置，本次不接收同步文件: %v", err)
		r.app.setDefaultFilesDir("")
	} else {
		r.app.setDefaultFilesDir(dir)
	}
	options := lansync.Options{
		StorePath:      path,
		Name:           r.app.deviceName,
		Platform:       r.app.devicePlatform,
		Version:        settingsVersion,
		FilesDir:       r.app.receivedFilesDir,
		FileLimitBytes: r.app.fileSyncLimitBytes,
		OnPeersChanged: func(peers []lansync.Peer) {
			r.update(func() { r.app.applySyncPeers(peers) })
		},
		OnDevicesChanged: func(devices []lansync.Device) {
			r.update(func() {
				r.app.applySyncDevices(devices)
				r.app.devicesChanged()
			})
		},
		OnPairRequest: func(request lansync.PairRequest) {
			r.update(func() {
				r.app.applyPairRequest(request)
				r.window.Show()
			})
		},
		OnPairingCode: func(deviceName, code string) {
			r.update(func() { r.app.applyPairingCode(deviceName, code) })
		},
		OnPairingFailed: func(deviceName string, err error) {
			r.update(func() { r.app.applyPairingFailed(deviceName, err) })
		},
		OnClip: func(clip lansync.Clip, from lansync.Device) {
			r.update(func() { r.app.receiveClip(clip, from) })
		},
		OnDelivery: func(itemID string, deliveries []history.DeviceDelivery) {
			r.update(func() { r.app.applyDelivery(itemID, deliveries) })
		},
	}
	service, err := openSyncService(ctx, func() (*lansync.Service, error) {
		return lansync.NewService(options)
	})
	if err != nil {
		// 配对信息读不出来时不覆盖原文件：修好之后还能接着用。
		log.Printf("无法读取设备配对信息，本次不启用局域网同步: %v", err)
		r.publishDiscoveryFailure(r.app.t(msgDevicesDiscoveryUnavailable))
		return
	}
	r.app.syncService = service
	// 服务构造时拿到的是初始值；设置文件里关掉的开关在这里补推一次，否则重启后
	// 界面显示关闭、服务却照常收发。
	service.SetSyncEnabled(r.app.syncEnabled)
	service.SetFileSyncEnabled(r.app.fileSyncEnabled)
	r.app.devicePeerID = service.Identity().ID
	r.app.deviceDiscovery = deviceDiscoveryLoading
	r.update(func() { r.app.applySyncDevices(service.Devices()) })

	go func() {
		if err := service.Start(ctx); err != nil {
			log.Printf("局域网发现未启动: %v", err)
			// 文案要读界面语言，所以在界面线程上取，不在这里读。
			noNetwork := errors.Is(err, lansync.ErrNoInterfaces)
			r.update(func() {
				message := r.app.t(msgDevicesErrorMessage)
				if noNetwork {
					message = r.app.t(msgDevicesDiscoveryNoNetwork)
				}
				r.app.deviceDiscovery = deviceDiscoveryError
				r.app.deviceDiscoveryMessage = message
			})
		}
	}()
}

// 身份文件被另一个实例短暂占用时重试的次数与间隔。一个实例正在退出、热重载刚重启、
// 或者用户快速点两下图标，都会让下一个实例在启动时撞上这个锁；Windows 上读一个正被
// 独占打开的文件直接报错。为了一次几百毫秒的争用就让整次运行没有同步，不值得。
const (
	syncServiceOpenAttempts = 10
	syncServiceOpenDelay    = 300 * time.Millisecond
)

// openSyncService 打开同步服务，遇到「文件暂时被占用」这类可恢复的错误就短暂重试。
// 内容本身读不出来（ErrStoreUnreadable）时立刻放弃：那不是等一等就会好的事。
func openSyncService(ctx context.Context, open func() (*lansync.Service, error)) (*lansync.Service, error) {
	var err error
	for attempt := 0; attempt < syncServiceOpenAttempts; attempt++ {
		var service *lansync.Service
		service, err = open()
		if err == nil {
			return service, nil
		}
		if errors.Is(err, lansync.ErrStoreUnreadable) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(syncServiceOpenDelay):
		}
	}
	return nil, err
}

// stopSync 停止局域网同步服务。返回后不会再有同步回调。
func (r *applicationRuntime) stopSync() {
	if r.syncCancel != nil {
		r.syncCancel()
		r.syncCancel = nil
	}
	if r.app.syncService != nil {
		r.app.syncService.Stop()
		r.app.syncService = nil
	}
}

// publishDiscoveryFailure 把发现层的失败告诉界面：设备页据此显示错误状态，而不是
// 一个看起来空荡荡的列表。
func (r *applicationRuntime) publishDiscoveryFailure(message string) {
	r.update(func() {
		r.app.deviceDiscovery = deviceDiscoveryError
		r.app.deviceDiscoveryMessage = message
	})
}

// measureStorageUsage 在界面线程之外统计磁盘占用，结果经 Window.Update 回填。
// after 非空时先等它关闭：清除历史时收到的文件还在后台删。
//
// 路径在界面线程上先算好：数据库可能是命令行指定的预览文件，数据目录也可能读不到。
func (r *applicationRuntime) measureStorageUsage(after <-chan struct{}) {
	paths := r.storageUsagePaths()
	go func() {
		if after != nil {
			<-after
		}
		usage := measureStorageUsage(paths)
		r.update(func() { r.app.applyStorageUsage(usage) })
	}()
}

// storageUsagePaths 返回用量统计要看的几个位置。拿不到的位置留空，对应那一类按 0
// 算：没有数据目录时本来就什么都没落盘。文件那一类可能有多个根目录，见
// fileCacheRoots。
func (r *applicationRuntime) storageUsagePaths() storageUsagePaths {
	paths := storageUsagePaths{files: r.app.fileCacheRoots()}
	if database, err := r.databaseFile(); err == nil {
		paths.database = database
	}
	return paths
}

// chooseDirectory 在界面线程之外打开系统目录选择框，选完经 Window.Update 回到界面
// 线程。
//
// 对话框会阻塞到用户作出选择，所以不能直接在手势回调里等它：那会让界面线程卡在
// 事件循环里重入，实测有概率根本弹不出来。窗口作为 Parent 传进去，macOS 上它是窗口的
// sheet，不会跑到别的窗口后面。
func (r *applicationRuntime) chooseDirectory(defaultPath string) {
	title := r.app.t(msgSettingsFileSyncDir)
	button := r.app.t(msgSettingsFileSyncDirChoose)
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            r.window,
			Title:             title,
			DefaultPath:       defaultPath,
			ButtonLabel:       button,
			Directory:         true,
			CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		r.update(func() { r.app.applyDirectoryChoice(paths[0]) })
	}()
}

// cleanFileCache 在界面线程之外测量收到的文件，并按保留期限与配额挑出该删的记录。
// 结果经 Window.Update 回到界面线程，删记录与落盘仍在那里做。
func (r *applicationRuntime) cleanFileCache(entries []filescache.Entry, policy filescache.Policy) {
	go func() {
		removed := filescache.Plan(filescache.Measure(entries), policy, time.Now())
		r.update(func() { r.app.applyFileCacheCleanup(removed) })
	}()
}

// update 在窗口存在时把结果送回界面线程。窗口可能已经关闭（退出过程中），所以
// 这里统一挡一层。
func (r *applicationRuntime) update(work func()) {
	if r.window == nil {
		return
	}
	r.window.Update(work)
}

func openMainWindow(app *application) *mygo.Window {
	// 初始尺寸按恢复的缩放算：缩放是界面偏好，重启后窗口不该先小后大。
	width, height := windowSizeForZoom(defaultWindowWidth, defaultWindowHeight, app.zoom, mygo.Rectangle{})
	minWidth, minHeight := zoomedWindowMinimum(app.zoom)
	return mygo.NewWindow(mygo.WindowOptions{
		Title:                "GoPaste",
		Width:                width,
		Height:               height,
		MinWidth:             minWidth,
		MinHeight:            minHeight,
		TitleBarStyle:        mygo.TitleBarHiddenInset,
		TrafficLightPosition: &mygo.Point{X: windowControlsInset, Y: windowControlsTopInset},
		BackgroundColor:      "light-dark(#ffffff, #1c1c1e)",
		Vibrancy:             mygo.VibrancyNone,
		StateKey:             "main",
		Content:              ui.View(app.view),
	})
}
