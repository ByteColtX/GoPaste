package main

import (
	"runtime"
	"sort"
	"strings"

	"gopaste/internal/history"

	"github.com/egoist/mygo"
)

func newApplication() *application {
	items, devices := previewData()
	app := &application{
		page:              pageHistory,
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
}

func (a *application) requestDangerousAction(action dangerousAction, deviceID string) {
	a.pendingDangerousAction = action
	a.pendingDangerousDeviceID = deviceID
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

func (a *application) executeDangerousAction(action dangerousAction, deviceID string) string {
	switch action {
	case dangerousActionClearHistory:
		a.clearHistory()
		return a.t(msgToastHistoryCleared)
	case dangerousActionUnpairDevice:
		if !a.unpairDevice(deviceID) {
			return ""
		}
		return a.t(msgToastUnpaired)
	}
	return ""
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
