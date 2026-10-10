package main

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

func newApplication() *application {
	items, devices := previewData()
	app := &application{
		page:              pageHistory,
		historyPaneWidth:  defaultHistoryPaneWidth,
		selectedIndex:     -1,
		items:             items,
		deviceName:        "本机",
		devicePlatform:    localPlatformName(),
		deviceSyncEnabled: true,
		deviceFileSync:    true,
		pairedDevices:     devices,
		language:          languageOptions[0],
		appearance:        appearanceSystem,
		settingsArea:      settingsGeneral,
	}
	app.keepSelection(items)
	return app
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

// visibleItems 返回当前筛选条件和搜索词下的历史记录。
func (a *application) visibleItems() []clipboardItem {
	query := strings.ToLower(strings.TrimSpace(a.query))
	visible := make([]clipboardItem, 0, len(a.items))
	for _, item := range a.items {
		if !a.matchesFilter(item) || !matchesQuery(item, query) {
			continue
		}
		visible = append(visible, item)
	}
	return visible
}

func matchesQuery(item clipboardItem, query string) bool {
	if query == "" {
		return true
	}
	searchText := strings.ToLower(strings.Join([]string{
		item.content,
		item.kind.displayName(),
		item.source,
	}, " "))
	return strings.Contains(searchText, query)
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

func (a *application) matchesFilter(item clipboardItem) bool {
	switch a.filter {
	case filterText:
		return item.kind == kindText
	case filterLinks:
		return item.kind == kindLink
	case filterCode:
		return item.kind == kindCode
	case filterImages:
		return item.kind == kindImage
	case filterFiles:
		return item.kind == kindFile
	case filterFavorites:
		return item.favorite
	default:
		return true
	}
}

// keepSelection 保证筛选结果变化后，预览区仍指向一个有效条目。
func (a *application) keepSelection(visible []clipboardItem) {
	if len(visible) == 0 {
		a.selectedID = ""
		a.selectedIndex = -1
		return
	}

	for index, item := range visible {
		if item.id == a.selectedID {
			a.selectedIndex = index
			return
		}
	}

	a.selectedIndex = 0
	a.selectedID = visible[0].id
}

func (a *application) selectItem(index int, item clipboardItem) {
	a.selectedIndex = index
	a.selectedID = item.id
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

// selectVisibleIndex 选择筛选结果中的一项，并同步预览区使用的稳定 ID。
func (a *application) selectVisibleIndex(visible []clipboardItem, index int) bool {
	if len(visible) == 0 {
		return false
	}
	index = max(0, min(index, len(visible)-1))
	item := visible[index]
	changed := a.selectedIndex != index || a.selectedID != item.id
	a.selectItem(index, item)
	return changed
}

// moveSelection 按步长移动历史列表的键盘选择，并限制在可见结果范围内。
func (a *application) moveSelection(visible []clipboardItem, delta int) bool {
	if len(visible) == 0 {
		return false
	}
	index := a.selectedIndex
	if index < 0 {
		index = 0
	} else {
		index += delta
	}
	return a.selectVisibleIndex(visible, index)
}

// selectBoundary 把键盘选择移动到筛选结果的首项或末项。
func (a *application) selectBoundary(visible []clipboardItem, end bool) bool {
	if len(visible) == 0 {
		return false
	}
	index := 0
	if end {
		index = len(visible) - 1
	}
	return a.selectVisibleIndex(visible, index)
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

func (a *application) imageItemCount() int {
	count := 0
	for _, item := range a.items {
		if item.kind == kindImage {
			count++
		}
	}
	return count
}

func (a *application) clearHistory() {
	a.items = nil
	a.selectedID = ""
	a.selectedIndex = -1
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
	for index := range a.items {
		item := &a.items[index]
		if item.id != id {
			continue
		}

		item.favorite = !item.favorite
		if item.favorite {
			return favoriteAddedStatus
		} else {
			return favoriteRemovedStatus
		}
	}
	return ""
}

func favoriteLabel(favorite bool) string {
	if favorite {
		return "已收藏"
	}
	return "收藏"
}
