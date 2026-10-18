package main

import (
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"gopaste/internal/history"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func newApplication() *application {
	app := &application{
		page:             pageHistory,
		filterRailOpen:   true,
		historyPaneWidth: defaultHistoryPaneWidth,
		// 历史记录由 SQLite 仓库异步装载，见 startHistoryPersistence。
		history:            history.New(nil),
		devicePlatform:     localPlatformName(),
		deviceSyncEnabled:  true,
		deviceFileSync:     true,
		pairedDevices:      previewDevices(),
		languagePreference: languagePreferenceSystem,
		language:           languageSimplifiedChinese,
		appearance:         appearanceSystem,
		settingsArea:       settingsGeneral,
		zoom:               1,
		// 删除单条记录默认先弹二次确认；勾选弹窗里的「不再询问」可关掉，
		// 设置页的「删除前确认」开关可以再打开。
		confirmBeforeDelete: true,
	}
	// 设备名默认就是这台机器在局域网里的名字：别的机器看到的就是它，用户不必
	// 自己起一个。设备页显示时再去掉 .local 后缀，见 deviceDisplayName。
	app.deviceNameDefault = defaultDeviceName(localHostname(), localPlatformSlug(), app.devicePeerID)
	app.deviceName = app.deviceNameDefault
	app.sortPairedDevices()
	return app
}

// defaultDeviceName 返回本机设备名的默认值。系统主机名直接拿来用：macOS 上是
// 带 .local 的局域网名，Windows 上是计算机名。读不到主机名时退回「平台名-peer
// ID 前 6 位」，peer ID 也还没有时只剩平台名。
func defaultDeviceName(hostname, platformSlug, peerID string) string {
	if name := strings.TrimSpace(hostname); name != "" {
		return name
	}
	if prefix := peerIDPrefix(peerID); prefix != "" {
		return platformSlug + "-" + prefix
	}
	return platformSlug
}

// peerIDPrefixLength 是兜底设备名里带几位设备 ID。
const peerIDPrefixLength = 6

// peerIDPrefix 取设备 ID 的前几个字符，用作兜底设备名的后缀；设备 ID 为空时
// 返回空串。
func peerIDPrefix(peerID string) string {
	runes := []rune(peerID)
	if len(runes) <= peerIDPrefixLength {
		return peerID
	}
	return string(runes[:peerIDPrefixLength])
}

// localHostname 返回系统主机名。读不到时返回空串，交给 defaultDeviceName 兜底。
func localHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

// sortPairedDevices 让在线设备排在离线设备之前，同组内保持原有顺序。
// 侧栏按切片顺序渲染，选中状态始终以设备 ID 为准，重排不影响当前选择。
func (a *application) sortPairedDevices() {
	sort.SliceStable(a.pairedDevices, func(i, j int) bool {
		return a.pairedDevices[i].online && !a.pairedDevices[j].online
	})
}

func localPlatformName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return runtime.GOOS
	}
}

// localPlatformSlug 返回平台名的小写形式，用来拼兜底的设备名：
// macOS → macos、Windows → windows、Linux → linux。不从展示用的
// localPlatformName 反推，避免改展示文案时连带改掉兜底名。
func localPlatformSlug() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "windows":
		return "windows"
	case "linux":
		return "linux"
	default:
		return strings.ToLower(runtime.GOOS)
	}
}

// commandShortcutLabel 返回带平台修饰键的快捷键显示文本：macOS 用 ⌘，其他平台
// 用 Ctrl+，让提示与系统习惯一致。
func commandShortcutLabel(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌘" + key
	}
	return "Ctrl+" + key
}

// searchShortcutLabel 返回聚焦搜索框的快捷键显示文本。
func searchShortcutLabel() string {
	return commandShortcutLabel("K")
}

func truncateSingleLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

// setPage 统一切换当前页面，避免窗口控制代码直接修改业务状态字段。真的换页时
// 才通知外部，让只在某些分区有意义的菜单项跟上启用状态。
func (a *application) setPage(next page) {
	if a.page == next {
		return
	}
	a.page = next
	if a.onPageChanged != nil {
		a.onPageChanged()
	}
}

// setFilterRailOpen 设置筛选栏的展开状态，并通知外部。标题栏按钮、Cmd+Ctrl+S
// 与 macOS 视图菜单里的「收起/展开侧栏」项都经过这里，菜单文案因此总能跟上。
func (a *application) setFilterRailOpen(open bool) {
	if a.filterRailOpen == open {
		return
	}
	a.filterRailOpen = open
	if a.onFilterRailToggled != nil {
		a.onFilterRailToggled()
	}
}

// toggleFilterRail 翻转筛选栏的展开状态。
func (a *application) toggleFilterRail() {
	a.setFilterRailOpen(!a.filterRailOpen)
}

// historyFiltered 报告筛选栏当前是否收窄了历史列表（不含搜索词）。
func (a *application) historyFiltered() bool {
	return a.filterKind != kindFilterAny || a.filterTag != tagFilterAny ||
		a.filterFavorite || a.filterSource.IsSet() || a.filterPeriod != periodFilterAny
}

// resetHistoryFilters 清空筛选栏的每一个维度，回到“所有”。搜索词由调用方
// 决定是否一起清除，因此这里不动 a.query。
func (a *application) resetHistoryFilters() {
	a.filterKind = kindFilterAny
	a.filterTag = tagFilterAny
	a.filterFavorite = false
	a.filterSource = history.SourceFilter{}
	a.filterPeriod = periodFilterAny
}

// tagKind 返回一个标签筛选成立时载荷必须属于的类型。链接和代码的载荷都是文本，
// 所以它们只能和「文本」同时成立。
func tagKind(filter historyTagFilter) historyKindFilter {
	switch filter {
	case tagFilterLink, tagFilterCode:
		return kindFilterText
	case tagFilterImage:
		return kindFilterImage
	default:
		return kindFilterAny
	}
}

// setKindFilter 选中一个类型，并让标签跟着退让：图像、文件、富文本里不可能有
// 链接或代码，两者同时生效会把列表筛成 0 条。
func (a *application) setKindFilter(filter historyKindFilter) {
	a.filterKind = filter
	if filter != kindFilterAny && filter != tagKind(a.filterTag) {
		a.filterTag = tagFilterAny
	}
}

// setTagFilter 选中一个标签。标签是某一种载荷的子类型，所以选中它就把类型收窄到
// 该类型，而不是与当前类型并存。
func (a *application) setTagFilter(filter historyTagFilter) {
	a.filterTag = filter
	if kind := tagKind(filter); kind != kindFilterAny {
		a.filterKind = kind
	}
}

// setSyncEnabled 同步更新本机、文件和所有已配对设备的同步开关。
func (a *application) setSyncEnabled(enabled bool) {
	a.deviceSyncEnabled = enabled
	a.deviceFileSync = enabled
	for index := range a.pairedDevices {
		a.pairedDevices[index].syncEnabled = enabled
	}
	a.syncStateChanged()
}

// toggleDeviceSync 翻转一台已配对设备的同步开关。设备已被取消配对时不做任何事。
func (a *application) toggleDeviceSync(id string) {
	device, ok := a.pairedDeviceByID(id)
	if !ok {
		return
	}
	device.syncEnabled = !device.syncEnabled
	a.syncStateChanged()
}

// trayDevices 把已配对设备投影成托盘「设备同步」子菜单需要的字段。
func (a *application) trayDevices() []trayDevice {
	devices := make([]trayDevice, 0, len(a.pairedDevices))
	for _, device := range a.pairedDevices {
		devices = append(devices, trayDevice{id: device.id, name: device.name, syncOn: device.syncEnabled})
	}
	return devices
}

// syncStateChanged 通知外部（托盘、窗口）同步开关已经变化。
func (a *application) syncStateChanged() {
	if a.onSyncStateChanged != nil {
		a.onSyncStateChanged()
	}
}

// devicesChanged 通知外部已配对设备列表已经变化。
func (a *application) devicesChanged() {
	if a.onDevicesChanged != nil {
		a.onDevicesChanged()
	}
}

func (a *application) loadSystemSettings() {
	a.launchAtLogin = mygo.App.OpenAtLogin()
}

// applyLanguage 把保存的语言偏好解析成实际使用的界面语言，再刷新派生状态：
// 托盘和窗口。启动时偏好是默认的「跟随系统」，设置页切换时选择控件已经写入了
// 新偏好，两个入口都走这里。设备名不跟着语言走：默认名是主机名，与界面语言
// 无关，切换语言时保持不动。
func (a *application) applyLanguage() {
	a.language = a.languagePreference.language(mygo.App.Locale)
	if a.onLanguageChanged != nil {
		a.onLanguageChanged()
	}
}

func (a *application) applyAppearance() {
	switch a.appearance {
	case appearanceLight:
		mygo.Theme.SetSource(mygo.ThemeLight)
	case appearanceDark:
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// appearanceModes 是外观模式的展示顺序，与设置页「模式」分段控件的顺序一致。
var appearanceModes = [...]int{appearanceSystem, appearanceLight, appearanceDark}

// setAppearance 选中某个外观并立即应用。侧栏气泡与设置页的模式分段控件都走
// 这里，两处入口因此始终落在同一个模式上。
func (a *application) setAppearance(mode int) {
	if a.appearance == mode {
		return
	}
	a.appearance = mode
	a.applyAppearance()
}

func (a *application) updateLaunchAtLogin() string {
	if err := mygo.App.SetOpenAtLogin(a.launchAtLogin); err != nil {
		a.launchAtLogin = !a.launchAtLogin
		return a.t(msgToastLaunchAtLoginUnsupported)
	}
	if a.launchAtLogin {
		return a.t(msgToastLaunchAtLoginOn)
	}
	return a.t(msgToastLaunchAtLoginOff)
}

// clearHistory 清空历史记录，并把删除同步到数据库。
func (a *application) clearHistory() {
	view := a.history.Snapshot(history.Query{})
	ids := make([]string, 0, len(view.Items))
	for _, item := range view.Items {
		ids = append(ids, item.ID)
	}
	a.history.Clear()
	a.historyBitmaps = nil
	a.codeHighlights = nil
	a.persistDelete(ids...)
}

func (a *application) requestDangerousAction(action dangerousAction, targetID string) {
	a.pendingDangerousAction = action
	a.pendingDangerousTargetID = targetID
	a.dangerDontAskAgain = false
	a.confirmDangerousAction = true
}

func (a *application) pairedDeviceByID(id string) (*pairedDevice, bool) {
	for index := range a.pairedDevices {
		if a.pairedDevices[index].id == id {
			return &a.pairedDevices[index], true
		}
	}
	return nil, false
}

func (a *application) executeDangerousAction(action dangerousAction, targetID string) string {
	switch action {
	case dangerousActionClearHistory:
		a.clearHistory()
		return a.t(msgToastHistoryCleared)
	}
	return ""
}

// undoWindow 是危险操作后可撤销的时间窗口：提示在这么久之后关闭，撤销机会随之结束。
const undoWindow = 5 * time.Second

// pendingUndo 是一次限时撤销：restore 放回被移除的内容，toastID 指向提供「撤销」
// 按钮的提示。提示一关（超时、点了撤销、或 Cmd+Z 已经撤销过），这次机会就作废。
// 撤销栈按操作先后排列，栈顶是最近一次，Cmd+Z 因此撤销最近一次操作。
type pendingUndo struct {
	toastID string
	restore func()
}

// addUndoToast 显示带「撤销」按钮的限时提示，并把这次撤销压入栈顶。提示按钮与
// Cmd+Z 都经 takePendingUndo 取用同一条恢复路径，因此不会重复恢复。
//
// 提示 ID 由 AddToast 逐次生成，快速连续操作时各自的撤销互不覆盖。
func (a *application) addUndoToast(c *ui.Context, title string, restore func()) {
	var toastID string
	toastID = c.AddToast(ui.Toast{
		Title:   title,
		Timeout: undoWindow,
		// 按钮上带着快捷键，与预览操作条的键帽同一套做法，撤销因此不必先发现。
		Action:   shortcutHint(a.t(msgToastUndo), commandShortcutLabel("Z")),
		OnAction: func() { a.undoByToast(toastID) },
		OnClose:  func() { a.dropPendingUndo(toastID) },
	})
	a.pendingUndos = append(a.pendingUndos, pendingUndo{toastID: toastID, restore: restore})
	a.undoAvailabilityChanged()
}

// undoByToast 是提示里「撤销」按钮触发的恢复。它先取出机会再恢复，随后提示关闭
// 触发的 dropPendingUndo 找不到它，不会重复处理。
func (a *application) undoByToast(toastID string) {
	entry, ok := a.takePendingUndo(toastID)
	if !ok {
		return
	}
	entry.restore()
}

// undoLast 撤销最近一次仍在窗口内的操作，并关掉对应的提示；没有可撤销的操作时
// 什么都不做。窗口快捷键与 macOS 菜单项都走这里。
func (a *application) undoLast(c *ui.Context) {
	if len(a.pendingUndos) == 0 {
		return
	}
	toastID := a.pendingUndos[len(a.pendingUndos)-1].toastID
	entry, ok := a.takePendingUndo(toastID)
	if !ok {
		return
	}
	c.CloseToast(toastID)
	entry.restore()
}

// takePendingUndo 取出并移除一条撤销机会，同时通知可用性变化。
func (a *application) takePendingUndo(toastID string) (pendingUndo, bool) {
	for index := range a.pendingUndos {
		if a.pendingUndos[index].toastID != toastID {
			continue
		}
		entry := a.pendingUndos[index]
		a.pendingUndos = append(a.pendingUndos[:index], a.pendingUndos[index+1:]...)
		a.undoAvailabilityChanged()
		return entry, true
	}
	return pendingUndo{}, false
}

// dropPendingUndo 在提示关闭后作废对应的撤销机会。
func (a *application) dropPendingUndo(toastID string) {
	a.takePendingUndo(toastID)
}

// requestUndo 由 macOS 的编辑菜单调用：菜单回调不在构建帧里，拿不到 ui.Context，
// 因此只排队一次撤销，交给下一帧的 flushPendingUndo 执行。
func (a *application) requestUndo() {
	a.undoRequested = true
}

// undoAvailabilityChanged 通知外部撤销机会的有无发生了变化。
func (a *application) undoAvailabilityChanged() {
	if a.onUndoAvailabilityChanged != nil {
		a.onUndoAvailabilityChanged()
	}
}

// deleteHistoryItem 删除一条记录，并弹出限时可撤销的提示。撤销会把记录放回原
// 有顺序并重新选中；超过 undoWindow 未撤销，删除即成为最终结果。
func (a *application) deleteHistoryItem(c *ui.Context, id string) {
	item, ok := a.history.Item(id)
	if !ok {
		return
	}
	if err := a.history.Delete(id); err != nil {
		return
	}
	delete(a.historyBitmaps, id)
	a.persistDelete(id)
	a.addUndoToast(c, a.t(msgToastDeleted), func() { a.restoreHistoryItem(item) })
}

// restoreHistoryItem 把撤销窗口内删除的记录放回历史记录，并安排下一帧提示。
func (a *application) restoreHistoryItem(item history.Item) {
	if err := a.history.Restore(item); err != nil {
		return
	}
	a.persistItem(item)
	a.pendingToast = a.t(msgToastRestored)
}

// unpairDeviceWithUndo 取消一台设备的配对，并弹出限时可撤销的提示。撤销会把设备
// 放回原有位置并重新选中；超过 undoWindow 未撤销，取消配对即成为最终结果。
func (a *application) unpairDeviceWithUndo(c *ui.Context, id string) {
	index, device, ok := a.removePairedDevice(id)
	if !ok {
		return
	}
	a.addUndoToast(c, a.t(msgToastUnpaired), func() { a.restorePairedDevice(index, device) })
}

// removePairedDevice 从已配对设备中移除一台并清空选中，返回它原来的下标，供撤销
// 时放回原位。
func (a *application) removePairedDevice(id string) (int, pairedDevice, bool) {
	for index, device := range a.pairedDevices {
		if device.id != id {
			continue
		}
		a.pairedDevices = append(a.pairedDevices[:index], a.pairedDevices[index+1:]...)
		a.selectedDeviceID = ""
		a.deviceInfoExpanded = false
		// 筛选栏可能正按这台设备过滤：设备没了就回到“所有来源”，
		// 避免历史列表卡在一个永远匹配不到的筛选上。
		if a.filterSource == history.DeviceSource(id) {
			a.filterSource = history.SourceFilter{}
		}
		a.devicesChanged()
		return index, device, true
	}
	return 0, pairedDevice{}, false
}

// restorePairedDevice 把撤销窗口内取消配对的设备放回原有位置并重新选中，再安排
// 下一帧提示。
func (a *application) restorePairedDevice(index int, device pairedDevice) {
	if index < 0 || index > len(a.pairedDevices) {
		index = len(a.pairedDevices)
	}
	a.pairedDevices = slices.Insert(a.pairedDevices, index, device)
	a.selectedDeviceID = device.id
	a.pendingToast = a.t(msgToastRestored)
	a.devicesChanged()
}

func (a *application) toggleFavorite(id string) string {
	favorite, err := a.history.ToggleFavorite(id)
	if err != nil {
		return ""
	}
	if item, ok := a.history.Item(id); ok {
		a.persistItem(item)
	}
	if favorite {
		return a.t(msgToastFavoriteAdded)
	}
	return a.t(msgToastFavoriteRemoved)
}

// favoriteLabel 返回收藏按钮的文案：已收藏用完成态，未收藏用动作文案。
func (a *application) favoriteLabel(favorite bool) string {
	if favorite {
		return a.t(msgFavoriteAdded)
	}
	return a.t(msgFavorite)
}
