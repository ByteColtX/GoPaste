package main

import (
	"runtime"
	"strings"

	"gopaste/internal/history"

	"github.com/egoist/mygo"
)

func newApplication() *application {
	items, devices := previewData()
	return &application{
		page:              pageHistory,
		historyPaneWidth:  defaultHistoryPaneWidth,
		history:           history.New(items),
		deviceName:        "本机",
		devicePlatform:    localPlatformName(),
		deviceSyncEnabled: true,
		deviceFileSync:    true,
		pairedDevices:     devices,
		language:          languageOptions[0],
		appearance:        appearanceSystem,
		settingsArea:      settingsGeneral,
	}
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
		return "当前运行环境暂不支持开机自启"
	}
	if a.launchAtLogin {
		return "已开启开机自启"
	}
	return "已关闭开机自启"
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
		return "历史记录已清空"
	case dangerousActionUnpairDevice:
		if !a.unpairDevice(deviceID) {
			return ""
		}
		return "已取消配对"
	}
	return ""
}

func (a *application) toggleFavorite(id string) string {
	favorite, err := a.history.ToggleFavorite(id)
	if err != nil {
		return ""
	}
	if favorite {
		return favoriteAddedStatus
	}
	return favoriteRemovedStatus
}

func favoriteLabel(favorite bool) string {
	if favorite {
		return "已收藏"
	}
	return "收藏"
}
