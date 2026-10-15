package main

import (
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
	items, devices := previewData()
	app := &application{
		page:              pageHistory,
		filterRailOpen:    true,
		historyPaneWidth:  defaultHistoryPaneWidth,
		history:           history.New(items),
		deviceName:        newLocalizer(languageSimplifiedChinese).text(msgDevicesBadgeLocal),
		devicePlatform:    localPlatformName(),
		deviceSyncEnabled: true,
		deviceFileSync:    true,
		pairedDevices:     devices,
		language:          languageSimplifiedChinese,
		appearance:        appearanceSystem,
		settingsArea:      settingsGeneral,
		zoom:              1,
	}
	app.sortPairedDevices()
	return app
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

// searchShortcutLabel 返回聚焦搜索框的快捷键显示文本，
// macOS 使用 Command 符号，其他平台使用 Control 名称。
func searchShortcutLabel() string {
	if runtime.GOOS == "darwin" {
		return "⌘K"
	}
	return "Ctrl+K"
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

// setPage 统一切换当前页面，避免窗口控制代码直接修改业务状态字段。
func (a *application) setPage(next page) {
	a.page = next
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

// setSyncEnabled 同步更新本机、文件和所有已配对设备的同步开关。
func (a *application) setSyncEnabled(enabled bool) {
	a.deviceSyncEnabled = enabled
	a.deviceFileSync = enabled
	for index := range a.pairedDevices {
		a.pairedDevices[index].syncEnabled = enabled
	}
	if a.onSyncStateChanged != nil {
		a.onSyncStateChanged()
	}
}

func (a *application) loadSystemSettings() {
	a.launchAtLogin = mygo.App.OpenAtLogin()
}

// applySystemLanguage 依据系统语言选择界面语言，并把尚未修改过的本机名称
// 换成该语言下的默认名称。
func (a *application) applySystemLanguage() {
	a.language = languageFromLocale(mygo.App.Locale())
	a.renameDefaultDevice()
}

// applyLanguage 在设置页切换语言后刷新派生状态：默认设备名称、托盘和窗口。
// 选择控件已经写入了新的语言，这里只负责后续的联动。
func (a *application) applyLanguage() {
	a.renameDefaultDevice()
	if a.onLanguageChanged != nil {
		a.onLanguageChanged()
	}
}

// renameDefaultDevice 只在设备名称仍是某个语言的默认名时才替换，
// 避免覆盖用户自定义的名称。
func (a *application) renameDefaultDevice() {
	if a.isDefaultDeviceName(a.deviceName) {
		a.deviceName = a.t(msgDevicesBadgeLocal)
	}
}

// isDefaultDeviceName 判断名称是否等于任一支持语言下的默认本机名称。
func (a *application) isDefaultDeviceName(name string) bool {
	for _, l := range []language{languageSimplifiedChinese, languageEnglish} {
		if name == newLocalizer(l).text(msgDevicesBadgeLocal) {
			return true
		}
	}
	return false
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

func (a *application) clearHistory() {
	a.history.Clear()
	a.historyBitmaps = nil
	a.codeHighlights = nil
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
	// 每次删除使用独立的 toast ID，快速连续删除时各自的撤销互不覆盖。
	c.AddToast(ui.Toast{
		Title:    a.t(msgToastDeleted),
		Timeout:  undoWindow,
		Action:   a.t(msgToastUndo),
		OnAction: func() { a.restoreHistoryItem(item) },
	})
}

// restoreHistoryItem 把撤销窗口内删除的记录放回历史记录，并安排下一帧提示。
func (a *application) restoreHistoryItem(item history.Item) {
	if err := a.history.Restore(item); err != nil {
		return
	}
	a.pendingToast = a.t(msgToastRestored)
}

// unpairDeviceWithUndo 取消一台设备的配对，并弹出限时可撤销的提示。撤销会把设备
// 放回原有位置并重新选中；超过 undoWindow 未撤销，取消配对即成为最终结果。
func (a *application) unpairDeviceWithUndo(c *ui.Context, id string) {
	index, device, ok := a.removePairedDevice(id)
	if !ok {
		return
	}
	// 每次取消配对使用独立的 toast，快速连续操作时各自的撤销互不覆盖。
	c.AddToast(ui.Toast{
		Title:    a.t(msgToastUnpaired),
		Timeout:  undoWindow,
		Action:   a.t(msgToastUndo),
		OnAction: func() { a.restorePairedDevice(index, device) },
	})
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
}

func (a *application) toggleFavorite(id string) string {
	favorite, err := a.history.ToggleFavorite(id)
	if err != nil {
		return ""
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
