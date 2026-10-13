package main

import (
	"fmt"
	"strings"
	"time"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

type historyFilterOption struct {
	kind historyFilter
	key  string
}

var historyFilterOptions = [...]historyFilterOption{
	{kind: filterAll, key: "all"},
	{kind: filterText, key: "text"},
	{kind: filterLinks, key: "links"},
	{kind: filterCode, key: "code"},
	{kind: filterImages, key: "images"},
	{kind: filterFiles, key: "files"},
	{kind: filterFavorites, key: "favorites"},
}

// filterLabel 返回筛选条件在当前界面语言下的名称。
func (a *application) filterLabel(filter historyFilter) string {
	switch filter {
	case filterText:
		return a.t(msgFilterText)
	case filterLinks:
		return a.t(msgFilterLinks)
	case filterCode:
		return a.t(msgFilterCode)
	case filterImages:
		return a.t(msgFilterImages)
	case filterFiles:
		return a.t(msgFilterFiles)
	case filterFavorites:
		return a.t(msgFilterFavorites)
	case filterAll:
		return a.t(msgFilterAll)
	default:
		return a.t(msgFilterCurrent)
	}
}

func (a *application) view(c *ui.Context) {
	theme := a.applyTheme(c)
	a.globalShortcuts(c)
	a.previewShortcuts(c)

	ui.Row(c).Fill().Background(theme.Background).Children(func() {
		a.navigationRail(c, theme)
		ui.Box(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			a.pageContent(c, theme)
		})
	})
	a.dangerousConfirmation(c, theme)
	a.flushPendingToast(c)
}

// previewShortcuts 为预览区操作注册单键快捷键：C 复制、F 收藏、D 删除。它们只在
// 历史页且存在选中记录时生效。文本输入框聚焦时按键会先被输入框接管，因此在搜索框
// 里输入 c、f、d 不会误触发这些操作。
func (a *application) previewShortcuts(c *ui.Context) {
	if a.page != pageHistory {
		return
	}
	item, ok := selectedHistoryItem(a.history.Snapshot(a.historyQuery()))
	if !ok {
		return
	}
	c.OnShortcut(0, ui.KeyC, func() { a.copyToClipboard(c, item.ID) })
	c.OnShortcut(0, ui.KeyF, func() {
		if message := a.toggleFavorite(item.ID); message != "" {
			c.Toast(message)
		}
	})
	c.OnShortcut(0, ui.KeyD, func() { a.requestDeleteItem(c, item.ID) })
}

// requestDeleteItem 按“不再询问”偏好决定删除方式：已记住偏好就直接删除，
// 否则先弹二次确认框。按钮点击与 D 快捷键共用这条路径。
func (a *application) requestDeleteItem(c *ui.Context, id string) {
	if a.suppressDeleteConfirmation {
		a.deleteHistoryItem(c, id)
		return
	}
	a.requestDangerousAction(dangerousActionDeleteItem, id)
}

// flushPendingToast 展示由回调产生的提示（例如撤销删除后的确认），只在下一次
// 构建帧时消费一次。
func (a *application) flushPendingToast(c *ui.Context) {
	if a.pendingToast == "" {
		return
	}
	c.Toast(a.pendingToast)
	a.pendingToast = ""
}

// chromeHeight 返回窗口标题栏（macOS 红绿灯、Windows/Linux 窗口按钮）占用的高度。
// 各内容列用它作为顶部留白，让面板表面一直延伸到窗口顶边。高度完全交给
// 平台：Windows/Linux 取系统原生标题栏高度，macOS 由红绿灯位置决定。
func chromeHeight(c *ui.Context) float32 {
	return c.TitleBar().Height
}

// paneTop 是每个内容面板顶部的透明拖拽条：高度与标题栏一致，
// 让面板背景延伸到窗口顶边，同时提供整条的窗口拖动区。
func paneTop(c *ui.Context) {
	ui.Row(c).Height(chromeHeight(c)).FillWidth().Shrink(0).DragWindow()
}

func (a *application) globalShortcuts(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.Key1, func() { a.setPage(pageHistory) })
	c.OnShortcut(ui.Cmd, ui.Key2, func() { a.setPage(pageDevices) })
	c.OnShortcut(ui.Cmd, ui.Key3, func() { a.setPage(pageSettings) })
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.focusHistorySearch(c) })
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { a.focusHistorySearch(c) })
}

// focusHistorySearch 在窗口级别响应搜索快捷键，使焦点位于预览区或其他区域时也能聚焦搜索框。
func (a *application) focusHistorySearch(c *ui.Context) {
	if a.page != pageHistory {
		return
	}
	a.historySearch.Focus(c)
}

func (a *application) dangerousConfirmation(c *ui.Context, theme *ui.Theme) {
	if !a.confirmDangerousAction {
		return
	}

	action := a.pendingDangerousAction
	title, message, confirmLabel, ok := a.dangerousActionDetails(action, a.pendingDangerousTargetID)
	if !ok {
		a.confirmDangerousAction = false
		a.pendingDangerousAction = dangerousActionNone
		a.pendingDangerousTargetID = ""
		return
	}

	// 只有删除需要“不再询问”：勾选后本次确认即记住偏好，之后删除直接执行。
	dontAskLabel := ""
	if action == dangerousActionDeleteItem {
		dontAskLabel = a.t(msgDangerDontAskAgain)
	}

	if dangerConfirmDialog(c, theme, a.l10n(), &a.confirmDangerousAction, title, message, confirmLabel, dontAskLabel, &a.dangerDontAskAgain) {
		switch action {
		case dangerousActionDeleteItem:
			if a.dangerDontAskAgain {
				a.suppressDeleteConfirmation = true
			}
			a.deleteHistoryItem(c, a.pendingDangerousTargetID)
		default:
			if message := a.executeDangerousAction(action, a.pendingDangerousTargetID); message != "" {
				c.Toast(message)
			}
		}
	}
	if !a.confirmDangerousAction {
		a.pendingDangerousAction = dangerousActionNone
		a.pendingDangerousTargetID = ""
	}
}

func (a *application) dangerousActionDetails(action dangerousAction, targetID string) (string, string, string, bool) {
	switch action {
	case dangerousActionClearHistory:
		return a.t(msgDangerClearTitle), a.t(msgDangerClearMessage), a.t(msgDangerClearConfirm), true
	case dangerousActionDeleteItem:
		message := a.t(msgDangerDeleteMessage)
		if item, ok := a.history.Item(targetID); ok {
			message = a.tf(msgDangerDeleteMessageFormat, truncateSingleLine(a.historyListText(item), 40))
		}
		return a.t(msgDangerDeleteTitle), message, a.t(msgDangerDeleteConfirm), true
	case dangerousActionUnpairDevice:
		name := a.t(msgSyncThisDevice)
		if device, ok := a.pairedDeviceByID(targetID); ok {
			name = device.name
		}
		return a.t(msgDangerUnpairTitle), a.tf(msgDangerUnpairMessage, name), a.t(msgDangerUnpairConfirm), true
	default:
		return "", "", "", false
	}
}

// navigationRail 是窗口左侧的固定导航轨道：整列延伸到窗口顶边，顶部为标题栏留白，
// 因此它同时承担拖动窗口的职责。窄窗口只显示图标，宽窗口显示图标与文字。
func (a *application) navigationRail(c *ui.Context, theme *ui.Theme) {
	railWidth := navigationRailSize(c, theme)
	// 是否展开由窗口宽度决定，不能再用轨道宽度判断：紧凑轨道会为避开
	// 窗口控制按钮而加宽，那样会被误判成展开态而显示文字标签。
	expanded := contentWidth(c) >= responsiveExpandedWidth
	rail := ui.Column(c).Width(railWidth).FillHeight().Shrink(0).
		Padding(chromeHeight(c)+space(theme, 1.5), space(theme, 2), space(theme, 2.5), space(theme, 2)).
		Gap(space(theme, 1)).Background(sidebarFill(theme)).
		Border(1, dividerColor(theme)).
		DragWindow()
	rail.Children(func() {
		a.navigationButton(c, theme, "nav-history", pageHistory, icons.history, a.t(msgNavHistory), expanded)
		a.navigationButton(c, theme, "nav-devices", pageDevices, icons.devices, a.t(msgNavDevices), expanded)
		a.navigationButton(c, theme, "nav-settings", pageSettings, icons.settings, a.t(msgNavSettings), expanded)
	})
}

func (a *application) navigationButton(c *ui.Context, theme *ui.Theme, key string, target page, icon *ui.SVG, label string, expanded bool) {
	selected := a.page == target
	button := ui.ButtonBase(c.Key(key)).
		Height(controlHeightSize(c, theme, 10)).FillWidth().Shrink(0).
		Padding(space(theme, 1.5), space(theme, 2.5)).
		Radius(radius(theme, controlRadius)).AlignItems(ui.Center).
		Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).
		Label(label).Tooltip(label).Role(ui.RoleToggleButton).Selected(selected)
	switch {
	case selected:
		button.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).TextColor(primaryText(theme))
	case button.Hovered():
		button.Background(hoverFill(theme))
	default:
		button.Background(ui.Transparent)
	}
	iconColor := secondaryText(theme)
	if selected {
		iconColor = accentColor(theme)
	}
	button.Children(func() {
		if expanded {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(space(theme, 2.5)).Children(func() {
				ui.Icon(c, icon).Size(space(theme, 4.75), space(theme, 4.75)).Shrink(0).TextColor(iconColor)
				ui.Text(c, label).FontSize(font(theme, fontControl)).FontWeight(500).SingleLine()
			})
			return
		}
		ui.Icon(c, icon).Size(space(theme, 5), space(theme, 5)).TextColor(iconColor)
	})
	if button.Clicked() {
		a.page = target
	}
}

func (a *application) pageContent(c *ui.Context, theme *ui.Theme) {
	switch a.page {
	case pageDevices:
		a.devicesPage(c, theme)
	case pageSettings:
		a.settingsPage(c, theme)
	default:
		a.historyPage(c, theme)
	}
}

func (a *application) historyPage(c *ui.Context, theme *ui.Theme) {
	view := a.history.Snapshot(a.historyQuery())
	a.historySearch.OnShortcut(c, 0, ui.KeyEscape, func() {
		if a.query != "" {
			a.query = ""
			return
		}
		a.historySearch.CancelFocus(c)
	})

	a.constrainHistoryPaneWidth(c, theme)

	ui.Split(c, &a.historyPaneWidth,
		func() { a.historyList(c, theme, view) },
		func() { a.historyPreview(c, theme, view) },
	).Fill()
}

// constrainHistoryPaneWidth 在窗口缩放时保持左右面板都可用。
func (a *application) constrainHistoryPaneWidth(c *ui.Context, theme *ui.Theme) {
	if a.historyPaneWidth <= 0 {
		a.historyPaneWidth = defaultHistoryPaneWidth
	}

	windowWidth, _ := c.Size()
	maxWidth := windowWidth - navigationRailSize(c, theme) - minPreviewPaneWidth - splitDividerWidth
	maxWidth = max(maxWidth, float32(minHistoryPaneWidth))
	a.historyPaneWidth = min(max(a.historyPaneWidth, float32(minHistoryPaneWidth)), maxWidth)
}

func (a *application) historyList(c *ui.Context, theme *ui.Theme, view history.View) {
	panel := ui.Column(c).Fill().
		MinWidth(minHistoryPaneWidth).
		Background(sidebarFill(theme)).
		Padding(0, 0, space(theme, 2.5), space(theme, 4))

	panel.Children(func() {
		paneTop(c)
		a.historyHeader(c, theme, view)
		ui.Box(c).Margin(0, space(theme, sidebarTrailingInset), 0, 0).Children(func() {
			a.searchField(c, theme)
		})
		a.filterBar(c, theme)

		if len(view.Items) == 0 {
			a.emptyHistory(c, theme, view)
			return
		}

		a.historyRows(c, theme, view)
	})
}

func (a *application) historyHeader(c *ui.Context, theme *ui.Theme, view history.View) {
	countLabel := a.l10n().recordCount(len(view.Items), view.Total, a.query != "" || a.filter != filterAll)
	ui.Row(c).Shrink(0).AlignItems(ui.Start).
		Padding(space(theme, columnTopInset), space(theme, sidebarTrailingInset), space(theme, 1.5), 0).Children(func() {
		ui.Column(c).Gap(space(theme, 0.25)).Children(func() {
			ui.Text(c, a.t(msgHistoryRecent)).FontSize(font(theme, fontPaneTitle)).FontWeight(600).SingleLine()
			ui.Text(c, countLabel).FontSize(font(theme, fontCaption)).
				TextColor(secondaryText(theme)).SingleLine()
		})
	})
}

func (a *application) searchField(c *ui.Context, theme *ui.Theme) {
	focused := c.Focused(a.historySearch)
	field := ui.Row(c.Key("history-search")).Height(controlHeightSize(c, theme, controlHeight)).FillWidth().Shrink(0).
		AlignItems(ui.Center).Gap(space(theme, 1.5)).Padding(0, space(theme, 2.5)).
		Radius(radius(theme, controlRadius)).Background(theme.Background).
		Border(1, borderColor(theme)).
		Label(a.t(msgHistorySearchLabel))
	if focused {
		field.BorderColor(theme.Accent)
	}

	field.Children(func() {
		ui.Icon(c, icons.search).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0).
			TextColor(secondaryText(theme))
		input := ui.TextInput(c.Key("history-search-input"), &a.query).
			Grow(1).MinWidth(0).Padding(0).Radius(0).
			Background(ui.Transparent).Border(0, ui.Transparent).
			Label(a.t(msgHistorySearchLabel)).Placeholder(a.t(msgHistorySearchPlaceholder)).Bind(&a.historySearch).
			FocusRing(focused)
		input.Changed()

		if a.query == "" {
			if !focused {
				searchShortcutHint(c, theme)
			}
			return
		}
		clearButton := ui.ButtonBase(c.Key("clear-history-search")).
			Size(space(theme, 5), space(theme, 5)).Shrink(0).Radius(radius(theme, 2.5)).Center().
			TextColor(secondaryText(theme)).
			Label(a.t(msgHistoryClearSearch)).Tooltip(a.t(msgHistoryClearSearch)).Role(ui.RoleButton)
		if clearButton.Hovered() {
			clearButton.Background(hoverFill(theme))
		}
		clearButton.Children(func() {
			ui.Icon(c, icons.clear).Size(space(theme, 3), space(theme, 3))
		})
		if clearButton.Clicked() {
			a.query = ""
		}
	})
}

// searchShortcutHint 在搜索框空且未聚焦时提示聚焦搜索的快捷键，
// 快捷键本身在 historyPage 中注册（Cmd+K / Cmd+F）。
func searchShortcutHint(c *ui.Context, theme *ui.Theme) {
	ui.Text(c, searchShortcutLabel()).FontSize(font(theme, fontCaption)).FontWeight(500).
		TextColor(secondaryText(theme)).Padding(space(theme, 0.5), space(theme, 1)).
		Radius(radius(theme, controlRadius)).Background(subtleFill(theme)).SingleLine()
}

func (a *application) filterBar(c *ui.Context, theme *ui.Theme) {
	ui.ScrollHorizontal(c).Height(controlHeightSize(c, theme, compactHeight)).FillWidth().Shrink(0).
		Margin(space(theme, historyControlGap), space(theme, sidebarTrailingInset), 0, 0).
		ScrollbarInsets(0, 0, 0, 0).Children(func() {
		ui.Row(c).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).AlignItems(ui.Center).
			Gap(space(theme, 0.5)).Children(func() {
			for _, option := range historyFilterOptions {
				a.filterButton(c, theme, option.kind, option.key, a.filterLabel(option.kind))
			}
		})
	})
}

func (a *application) filterButton(c *ui.Context, theme *ui.Theme, filter historyFilter, key, label string) {
	selected := a.filter == filter
	button := ui.ButtonBase(c.Key("history-filter-"+key)).
		Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2)).Radius(radius(theme, controlRadius)).
		Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).Label(label).
		Role(ui.RoleToggleButton).Selected(selected)
	if selected {
		button.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).TextColor(primaryText(theme))
	} else if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(font(theme, fontCaption)).FontWeight(500).SingleLine()
	})
	if button.Clicked() {
		a.filter = filter
	}
}

func (a *application) emptyHistory(c *ui.Context, theme *ui.Theme, view history.View) {
	firstUse := view.Total == 0 && a.query == "" && a.filter == filterAll
	searchEmpty := a.query != ""
	title := a.t(msgHistoryEmptyTitle)
	message := a.t(msgHistoryEmptyMessage)
	icon := icons.filter
	if firstUse {
		title = a.t(msgHistoryEmptyFirstTitle)
		message = a.t(msgHistoryEmptyFirstMessage)
		icon = icons.history
	} else if searchEmpty {
		title = a.t(msgHistoryEmptySearchTitle)
		message = a.tf(msgHistoryEmptySearchMessage, truncateSingleLine(a.query, 24))
		icon = icons.search
	} else if a.filter != filterAll {
		title = a.t(msgHistoryEmptyFilterTitle)
		message = a.tf(msgHistoryEmptyFilterMessage, a.filterLabel(a.filter))
		icon = icons.filter
	}
	ui.Box(c).Grow(1).Center().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(320).Shrink(0).Center().Gap(space(theme, 2)).Children(func() {
			ui.Box(c).Size(space(theme, 12), space(theme, 12)).Shrink(0).Radius(radius(theme, largeRadius)).
				Background(subtleFill(theme)).Center().Children(func() {
				ui.Icon(c, icon).Size(space(theme, 5), space(theme, 5)).TextColor(secondaryText(theme))
			})
			ui.Text(c, title).FontSize(font(theme, fontSection)).FontWeight(600).
				Role(ui.RoleHeading).SingleLine()
			ui.Text(c, message).FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme)).Wrap()
			if firstUse {
				return
			}
			resetLabel := a.t(msgHistoryResetSearchAndFilt)
			if searchEmpty && a.filter == filterAll {
				resetLabel = a.t(msgHistoryResetSearch)
			} else if !searchEmpty && a.filter != filterAll {
				resetLabel = a.t(msgHistoryShowAll)
			}
			reset := neutralButton(c, theme, "history-empty-reset", resetLabel)
			reset.Children(func() {
				ui.Text(c, resetLabel).FontSize(font(theme, fontControl)).SingleLine()
			})
			if reset.Clicked() {
				a.query = ""
				a.filter = filterAll
			}
		})
	})
}

// historyQuery 构造当前历史视图的查询，并带上本地化的类型名称，
// 让搜索在任意界面语言下都能匹配内容类型。
func (a *application) historyQuery() history.Query {
	return history.Query{
		Text:       a.query,
		Filter:     a.filter,
		KindTerms:  a.kindSearchTerms,
		SourceName: a.sourceDeviceName,
	}
}

// sourceDeviceName 把来源设备 ID 解析成可搜索的设备名。history 包不持有
// 设备表，所以由界面层提供；未知设备返回空。
func (a *application) sourceDeviceName(deviceID string) string {
	if device, ok := a.pairedDeviceByID(deviceID); ok {
		return device.name
	}
	return ""
}

func (a *application) historyRows(c *ui.Context, theme *ui.Theme, view history.View) {
	// 不使用 ListState.Selected，让列表的默认 Accent 背景不覆盖自定义卡片样式。
	// 键盘快捷键仍注册在列表容器上，因此焦点在任意条目时都能移动选择。
	a.listState.Selected = nil
	a.listState.Key = func(index int) any { return view.Items[index].ID }
	a.listState.Label = func(index int) string { return a.historyListText(view.Items[index]) }

	list := ui.List(c, &a.listState, len(view.Items), func(index int) {
		item := view.Items[index]
		row := a.historyRow(c, theme, item, index == view.SelectedIndex)
		if row.Clicked() {
			a.history.Select(item.ID)
		}
	}).ScrollbarInsets(0, 0, 0, 0)
	list.Grow(1)
	a.historyListShortcuts(c, list, view)
}

// historyListShortcuts 连接 MyGo 列表的键盘事件和 history 模块的选择状态。
// 使用 OnShortcut 让 Enter 先由列表处理，避免被条目按钮当成普通点击吞掉。
func (a *application) historyListShortcuts(c *ui.Context, list ui.Element, view history.View) {
	query := a.historyQuery()
	move := func(delta int) {
		if a.history.Move(query, delta) {
			next := a.history.Snapshot(query)
			a.listState.ScrollIntoView(next.SelectedIndex)
		}
	}
	list.OnShortcut(0, ui.KeyDown, func() { move(1) })
	list.OnShortcut(0, ui.KeyUp, func() { move(-1) })
	list.OnShortcut(0, ui.KeyHome, func() {
		if len(view.Items) == 0 {
			return
		}
		changed := view.SelectedID != view.Items[0].ID
		a.history.Select(view.Items[0].ID)
		if changed {
			a.listState.ScrollIntoView(0)
		}
	})
	list.OnShortcut(0, ui.KeyEnd, func() {
		if len(view.Items) == 0 {
			return
		}
		last := len(view.Items) - 1
		changed := view.SelectedID != view.Items[last].ID
		a.history.Select(view.Items[last].ID)
		if changed {
			a.listState.ScrollIntoView(last)
		}
	})
	list.OnShortcut(0, ui.KeyEnter, func() {
		if view.SelectedID != "" {
			a.copyToClipboard(c, view.SelectedID)
		}
	})
}

func (a *application) historyRow(c *ui.Context, theme *ui.Theme, item history.Item, selected bool) ui.Element {
	row := ui.ButtonBase(c.Key("history-row-"+item.ID)).
		MinHeight(controlHeightSize(c, theme, 16)).AlignSelf(ui.Stretch).Shrink(0).
		Margin(space(theme, 0.5), space(theme, sidebarTrailingInset), space(theme, 0.5), 0).
		Padding(space(theme, 2), space(theme, 2.5)).Gap(space(theme, 3)).
		Radius(radius(theme, controlRadius)).Border(1, ui.Transparent).Row().
		AlignItems(ui.Center).
		TextColor(primaryText(theme)).Label(a.historyListText(item)).
		Role(ui.RoleButton).Selected(selected)
	switch {
	case selected:
		row.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme))
	case row.Hovered():
		row.Background(hoverFill(theme)).BorderColor(ui.Transparent)
	default:
		row.Background(ui.Transparent).BorderColor(ui.Transparent)
	}

	row.Children(func() {
		a.historyRowTile(c, theme, item)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 1)).Children(func() {
			a.historyRowTitle(c, theme, item)
			a.historyRowMeta(c, theme, item)
		})
	})
	return row
}

func (a *application) historyRowTile(c *ui.Context, theme *ui.Theme, item history.Item) {
	size := space(theme, 8.5)
	tile := ui.Box(c).Size(size, size).Shrink(0).Radius(radius(theme, controlRadius)).Center()
	if item.Kind == kindImage && item.Image != nil {
		tile.Background(subtleFill(theme)).Border(1, borderColor(theme)).Children(func() {
			if bitmap := a.historyBitmap(item); bitmap != nil {
				ui.Image(c, bitmap).Size(size-2, size-2).Fit(ui.Cover).Radius(radius(theme, controlRadius) - 1)
			}
		})
		return
	}
	// 列表行的图标按内容子类型着色（链接、代码各自不同），
	// 让一屏记录只靠图标就能区分，而不必逐行读文字。
	kindColor := colorForKind(theme, item.Kind)
	tile.Background(kindColor.Alpha(0.14)).Children(func() {
		ui.Icon(c, iconForKind(item.Kind)).Size(space(theme, 4.5), space(theme, 4.5)).TextColor(kindColor)
	})
}

func (a *application) historyRowTitle(c *ui.Context, theme *ui.Theme, item history.Item) {
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
		ui.Text(c, a.historyListText(item)).Grow(1).MinWidth(0).
			FontSize(font(theme, fontBody)).FontWeight(500).
			TextColor(primaryText(theme)).SingleLine()
		if item.Favorite {
			ui.Icon(c, icons.star).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0).
				TextColor(favoriteColor(theme))
		}
	})
}

func (a *application) historyRowMeta(c *ui.Context, theme *ui.Theme, item history.Item) {
	kindColor := colorForKind(theme, item.Kind)
	// 内容类型靠左、时间靠右：时间与行右侧留白对齐，扫视一列时更整齐。
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.kindLabel(item.Kind)).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(kindColor).SingleLine()
		ui.Spacer(c)
		ui.Text(c, a.timestampLabel(item.CreatedAt)).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).SingleLine()
	})
}

// autoTag 渲染内容子类型的自动标签（链接、代码），没有标签时不占空间。
// 预览标题把它紧跟在类型标签之后，让子类型与类型同处首行，不再单独占位。
func (a *application) autoTag(c *ui.Context, theme *ui.Theme, item history.Item) {
	tag := a.l10n().smartTag(item.Kind)
	if tag == "" {
		return
	}
	tagColor := colorForKind(theme, item.Kind)
	ui.Text(c, tag).FontSize(font(theme, fontCaption)).TextColor(tagColor).
		Padding(space(theme, 0.25), space(theme, 1.5)).Radius(radius(theme, controlRadius)).
		Background(tagColor.Alpha(0.10)).
		Border(1, tagColor.Alpha(0.24)).SingleLine()
}

func (a *application) historyPreview(c *ui.Context, theme *ui.Theme, view history.View) {
	pane := ui.Column(c).Grow(1).FillHeight().
		MinWidth(minPreviewPaneWidth).Background(theme.Background)
	pane.Children(func() {
		paneTop(c)
		item, ok := selectedHistoryItem(view)
		if !ok {
			a.emptyPreview(c, theme)
			return
		}

		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			a.previewHeader(c, theme, item)
			a.previewContent(c, theme, item)
		})
		a.previewActions(c, theme, item)
	})
}

func selectedHistoryItem(view history.View) (history.Item, bool) {
	if view.SelectedIndex < 0 || view.SelectedIndex >= len(view.Items) {
		return history.Item{}, false
	}
	return view.Items[view.SelectedIndex], true
}

func (a *application) emptyPreview(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).Grow(1).Center().Children(func() {
		ui.Column(c).Center().Gap(space(theme, 2)).Children(func() {
			ui.Box(c).Size(space(theme, 12), space(theme, 12)).Shrink(0).Radius(radius(theme, largeRadius)).
				Background(subtleFill(theme)).Center().Children(func() {
				ui.Icon(c, icons.history).Size(space(theme, 5), space(theme, 5)).TextColor(secondaryText(theme))
			})
			ui.Text(c, a.t(msgHistorySelectRecord)).FontSize(font(theme, fontSection)).FontWeight(600)
			ui.Text(c, a.t(msgHistoryPreviewHint)).FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme))
		})
	})
}

func (a *application) previewHeader(c *ui.Context, theme *ui.Theme, item history.Item) {
	// 与设备、设置详情一致：先限制最大宽度并居中，再应用左右内边距，
	// 让标题、操作按钮和正文卡片的左右边缘对齐。
	ui.Column(c).Shrink(0).FillWidth().MaxWidth(pageDetailMaxWidth).AlignSelf(ui.Center).
		Padding(space(theme, columnTopInset), space(theme, spacePageSide), space(theme, 3), space(theme, spacePageSide)).
		Gap(space(theme, 1.75)).Children(func() {
		// 本机条目带同步状态标记，局域网条目没有。标记并进来源「本机」一组，
		// 让「本机 已同步」读作这条记录的来源与去向；它带弹出浮层，且正好落在
		// 复制/收藏按钮上方，所以改成点击展开，指针滑过不会再误触发（见
		// syncIndicator）。元信息行预留固定高度，标记就不会撑高这一行，切换
		// 条目时下方的正文卡片也不再上下抖动。
		ui.Row(c).MinHeight(controlHeightSize(c, theme, 4.5)).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
			baseKind := item.Kind.BaseKind()
			kindColor := colorForKind(theme, baseKind)
			ui.Icon(c, iconForKind(baseKind)).Size(space(theme, 3.75), space(theme, 3.75)).
				Shrink(0).TextColor(kindColor)
			ui.Text(c, a.kindLabel(baseKind)).FontSize(font(theme, fontMeta)).FontWeight(600).
				TextColor(kindColor).SingleLine()
			a.autoTag(c, theme, item)
			for _, segment := range a.historyMetaSegments(item) {
				ui.Text(c, "·").FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme))
				ui.Text(c, segment).FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme)).SingleLine()
			}
			ui.Spacer(c)
			// 来源与同步标记收进同一子行并收紧间距，让「本机 已同步」读作一组：
			// 先交代记录从哪来，再交代同步到了哪。
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 0.5)).Children(func() {
				a.historySource(c, theme, item)
				a.syncIndicator(c, theme, item)
			})
			ui.Text(c, "·").FontSize(font(theme, fontCaption)).
				TextColor(secondaryText(theme))
			ui.Text(c, a.timestampLabel(item.CreatedAt)).FontSize(font(theme, fontCaption)).
				TextColor(secondaryText(theme)).SingleLine()
		})
	})
}

func (a *application) previewContent(c *ui.Context, theme *ui.Theme, item history.Item) {
	content := ui.Scroll(c).Grow(1).MinHeight(0)
	content.Children(func() {
		ui.Column(c).FillWidth().MaxWidth(pageDetailMaxWidth).AlignSelf(ui.Center).
			Padding(0, space(theme, spacePageSide), previewActionsClearance(c, theme)+space(theme, 2), space(theme, spacePageSide)).
			Gap(space(theme, 3)).Children(func() {
			switch item.Kind {
			case kindCode:
				ui.Text(c, item.Text).Font("monospace").FontSize(font(theme, fontCode)).
					TextColor(primaryText(theme)).Padding(space(theme, 4)).Radius(radius(theme, cardRadius)).
					Background(panelFill(theme)).Border(1, borderColor(theme)).Wrap().Selectable()
			case kindLink:
				ui.Row(c).Gap(space(theme, 2)).AlignItems(ui.Center).Padding(space(theme, 4)).
					Radius(radius(theme, cardRadius)).Background(panelFill(theme)).
					Border(1, borderColor(theme)).Selectable().Children(func() {
					ui.Icon(c, icons.link).Size(space(theme, 4), space(theme, 4)).Shrink(0).
						TextColor(colorForKind(theme, item.Kind))
					ui.Link(c, item.Text, item.Text).FontSize(font(theme, fontBody)).
						TextColor(theme.Accent).Wrap()
				})
			case kindImage:
				if item.Image == nil {
					ui.Text(c, a.t(msgHistoryImageUnavailable)).FontSize(font(theme, fontBody)).
						TextColor(secondaryText(theme)).Padding(space(theme, 4)).Radius(radius(theme, cardRadius)).
						Background(panelFill(theme)).Border(1, borderColor(theme))
					return
				}
				ui.Box(c).MaxWidth(pageDetailMaxWidth).Center().Padding(space(theme, 3)).Radius(radius(theme, largeRadius)).
					Background(panelFill(theme)).Border(1, borderColor(theme)).Children(func() {
					if bitmap := a.historyBitmap(item); bitmap != nil {
						ui.Image(c, bitmap).MaxWidth(pageDetailMaxWidth - space(theme, 6)).MaxHeight(520).
							Fit(ui.Contain).Radius(radius(theme, controlRadius))
					}
				})
			default:
				ui.Text(c, item.Text).FontSize(font(theme, fontBody)).
					TextColor(primaryText(theme)).Padding(space(theme, 4)).Radius(radius(theme, cardRadius)).
					Background(panelFill(theme)).Border(1, borderColor(theme)).Wrap().Selectable()
			}
		})
	})
}

// syncDeviceStatus is one paired device's delivery line in the sync popover.
type syncDeviceStatus struct {
	deviceID  string
	name      string
	label     string
	color     ui.Color
	textColor ui.Color
	canResend bool
}

const (
	// syncRowHeight is the fixed height every device line keeps, so lines with
	// and without a resend action share one vertical rhythm instead of the
	// taller action stretching only some of them.
	syncRowHeight float32 = 7
	// syncActionHeight is the height of the resend action inside a device line.
	// It stays shorter than syncRowHeight so the action reads as a control
	// centered in the row rather than as tall as the row itself.
	syncActionHeight float32 = 6
	// syncTrailingCol is the fixed trailing column the resend action sits in,
	// so the outcome text and the header action share one right edge whether
	// or not a device still offers a resend.
	syncTrailingCol float32 = 13
)

// syncIndicator renders the item's sync state as a compact chip in the preview
// meta row. The per-device detail and resend actions live in a popover the chip
// opens on click, so the section costs no space while it is idle and never
// springs open as the pointer travels past it.
func (a *application) syncIndicator(c *ui.Context, theme *ui.Theme, item history.Item) {
	// Only items captured on this machine are delivered to peers, so only
	// they carry a sync status. A peer item already arrived here.
	if !item.IsLocal() || item.Sync == syncStateNone {
		return
	}
	label, color := a.syncStateStyle(theme, item.Sync)
	chip := ui.Row(c.Key("sync-chip-"+item.ID)).Shrink(0).AlignItems(ui.Center).
		Gap(space(theme, 1.25)).Padding(space(theme, 0.5), space(theme, 1)).
		Radius(radius(theme, controlRadius))
	if chip.Hovered() {
		chip.Background(hoverFill(theme))
	}
	chip.Children(func() {
		statusDot(c, color, space(theme, 1.5))
		ui.Text(c, label).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(primaryText(theme)).SingleLine()
	})

	// 浮层由点击展开，点击别处或按 Esc 收起。标记正好落在复制/收藏按钮的上方，
	// 若改成指针悬停展开，鼠标移向按钮时会从它上面划过而误弹；展开状态按条目
	// 记忆，切换条目即收起。
	if a.syncPopoverItem != item.ID {
		a.syncPopoverItem = item.ID
		a.syncPopoverOpen = false
	}
	if chip.Clicked() {
		a.syncPopoverOpen = !a.syncPopoverOpen
	}
	open := a.syncPopoverOpen
	ui.PopoverBase(c, chip, &open, func(panel ui.Element) {
		panel.MinWidth(space(theme, 60)).MaxWidth(space(theme, 96)).
			Padding(space(theme, 2)).
			Radius(radius(theme, largeRadius)).
			Background(theme.Background).Border(1, borderColor(theme)).
			Shadow(0, space(theme, 1.5), space(theme, 5), 0, ui.RGBA(0, 0, 0, 0.18)).
			Transition(ui.ElementTransition{
				Duration: 140 * time.Millisecond,
				Ease:     ui.EaseOut,
				Enter:    &ui.Motion{Opacity: 0, Scale: 0.98, Y: -4},
			}).
			Margin(0)
		a.syncPopoverContent(c, theme, item)
	})
	a.syncPopoverOpen = open
}

// syncPopoverContent builds the popover: a header with the batch resend, then
// each paired device's outcome and per-device resend. A replaced item — never
// delivered to any device — adds one line explaining why, then the same list,
// so the user can still push it to any device by hand.
func (a *application) syncPopoverContent(c *ui.Context, theme *ui.Theme, item history.Item) {
	ui.Column(c).Gap(space(theme, 1)).Children(func() {
		a.syncPopoverHeader(c, theme, item)
		standardDivider(c, theme)
		if item.Sync == syncStateReplaced {
			a.syncReplacedNote(c, theme)
		}
		a.syncDeviceList(c, theme, item)
	})
}

func (a *application) syncPopoverHeader(c *ui.Context, theme *ui.Theme, item history.Item) {
	ui.Row(c).FillWidth().Height(controlHeightSize(c, theme, syncRowHeight)).
		Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2)).
		Padding(0, space(theme, 1.5)).Children(func() {
		ui.Text(c, a.t(msgSyncStatusTitle)).FontSize(font(theme, fontMeta)).FontWeight(600).
			TextColor(secondaryText(theme)).Role(ui.RoleHeading).SingleLine()
		ui.Spacer(c)
		if len(a.pairedDevices) > 0 {
			a.syncResendButton(c, theme, "batch", item.ID, "", a.t(msgSyncResendAll), a.t(msgSyncResendAllAccess))
		}
	})
}

func (a *application) syncDeviceList(c *ui.Context, theme *ui.Theme, item history.Item) {
	statuses := a.syncDeviceStatuses(theme, item)
	if len(statuses) == 0 {
		ui.Text(c, a.t(msgSyncNoDevices)).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).SingleLine()
		return
	}
	for _, status := range statuses {
		a.syncDeviceLine(c, theme, item, status)
	}
}

// syncDeviceLine is one device's line: a status dot, the device name, its
// delivery outcome, and a resend action while the item is still waiting. Every
// line keeps the same fixed height, so lines with and without a resend action
// share one vertical rhythm instead of the taller button stretching only some.
func (a *application) syncDeviceLine(c *ui.Context, theme *ui.Theme, item history.Item, status syncDeviceStatus) {
	row := ui.Row(c).FillWidth().Height(controlHeightSize(c, theme, syncRowHeight)).
		Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2)).
		Padding(0, space(theme, 1.5)).Radius(radius(theme, controlRadius))
	if row.Hovered() {
		row.Background(subtleFill(theme))
	}
	row.Children(func() {
		statusDot(c, status.color, space(theme, 1.5))
		ui.Text(c, status.name).Grow(1).MinWidth(0).FontSize(font(theme, fontControl)).
			FontWeight(500).TextColor(primaryText(theme)).SingleLine()
		ui.Text(c, status.label).Shrink(0).FontSize(font(theme, fontControl)).
			TextColor(status.textColor).SingleLine()
		a.syncResendSlot(c, theme, item, status)
	})
}

// syncResendSlot reserves a fixed trailing column so the status column stays
// aligned whether or not a device still shows a resend action.
func (a *application) syncResendSlot(c *ui.Context, theme *ui.Theme, item history.Item, status syncDeviceStatus) {
	ui.Row(c).Width(space(theme, syncTrailingCol)).Shrink(0).Justify(ui.End).Children(func() {
		if status.canResend {
			a.syncResendButton(c, theme, "device", item.ID, status.deviceID, a.t(msgSyncResend), a.tf(msgSyncResendAccess, status.name))
		}
	})
}

// syncReplacedNote explains a replaced item: it was superseded before it
// reached any device, so no automatic retry is coming and delivery is manual.
func (a *application) syncReplacedNote(c *ui.Context, theme *ui.Theme) {
	ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).
		Padding(space(theme, 0.75), space(theme, 1.5)).
		MinHeight(controlHeightSize(c, theme, syncRowHeight)).
		Radius(radius(theme, controlRadius)).Background(subtleFill(theme)).Children(func() {
		ui.Icon(c, icons.history).Size(space(theme, 3.5), space(theme, 3.5)).
			Shrink(0).TextColor(secondaryText(theme))
		ui.Text(c, a.t(msgSyncReplacedNote)).Grow(1).MinWidth(0).
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap().MaxLines(2)
	})
}

// syncResendButton is one resend action. label is the visible text; accessLabel
// is the accessible name, which names the target device for screen readers.
func (a *application) syncResendButton(c *ui.Context, theme *ui.Theme, scope, itemID, deviceID, label, accessLabel string) ui.Element {
	button := ui.ButtonBase(c.Key("sync-resend-"+scope+"-"+itemID+"-"+deviceID)).
		Height(controlHeightSize(c, theme, syncActionHeight)).Shrink(0).
		Padding(0, space(theme, 1.5)).Gap(space(theme, 0.75)).
		Radius(radius(theme, controlRadius)).
		Label(accessLabel).Role(ui.RoleButton).FocusRing(true)
	switch {
	case button.IsDisabled():
		button.Background(ui.Transparent).TextColor(secondaryText(theme))
	case button.Pressed():
		button.Background(theme.SurfacePressed).TextColor(primaryText(theme))
	case button.Hovered():
		button.Background(hoverFill(theme)).TextColor(primaryText(theme))
	default:
		button.Background(ui.Transparent).TextColor(secondaryText(theme))
	}
	button.Children(func() {
		ui.Icon(c, icons.refresh).Size(space(theme, 3), space(theme, 3)).Shrink(0)
		ui.Text(c, label).FontSize(font(theme, fontControl)).FontWeight(500).SingleLine()
	})
	if button.Clicked() {
		a.resendItem(c, itemID, deviceID)
	}
	return button
}

func (a *application) resendItem(c *ui.Context, itemID, deviceID string) {
	if a.onResendItem == nil {
		c.Toast(a.t(msgSyncServiceUnavailable))
		return
	}
	a.onResendItem(itemID, deviceID)
}

// syncDeviceStatuses lists every paired device with the item's delivery
// outcome. A device the item never reached counts as offline while the device
// is offline, and pending once it is back online.
func (a *application) syncDeviceStatuses(theme *ui.Theme, item history.Item) []syncDeviceStatus {
	statuses := make([]syncDeviceStatus, 0, len(a.pairedDevices))
	for index := range a.pairedDevices {
		device := &a.pairedDevices[index]
		state := deliveryStateForItem(item, device.id)
		if state != deliveryReceived && !device.online {
			state = deliveryOffline
		}
		label, color := a.deliveryStyle(theme, state, item.Sync == syncStateReplaced)
		statuses = append(statuses, syncDeviceStatus{
			deviceID:  device.id,
			name:      device.name,
			label:     label,
			color:     color,
			textColor: deliveryTextColor(theme, state),
			canResend: state != deliveryReceived,
		})
	}
	return statuses
}

// deliveryStateForItem returns a device's recorded outcome, or pending when
// the item's record has no entry for it.
func deliveryStateForItem(item history.Item, deviceID string) deliveryState {
	for _, delivery := range item.Deliveries {
		if delivery.DeviceID == deviceID {
			return delivery.State
		}
	}
	return deliveryPending
}

func (a *application) syncStateStyle(theme *ui.Theme, state syncState) (string, ui.Color) {
	switch state {
	case syncStateSynced:
		return a.t(msgSyncStateSynced), theme.Success
	case syncStatePartial:
		return a.t(msgSyncStatePartial), theme.Warning
	case syncStateOffline:
		return a.t(msgSyncStateOffline), theme.Warning
	case syncStateReplaced:
		return a.t(msgSyncStateReplaced), secondaryText(theme)
	default:
		return a.t(msgSyncStateNone), secondaryText(theme)
	}
}

// deliveryStyle names one device's outcome. A replaced item has no automatic
// retry left, so a device it never reached reads 未送达 rather than 待发送.
func (a *application) deliveryStyle(theme *ui.Theme, state deliveryState, replaced bool) (string, ui.Color) {
	switch state {
	case deliveryReceived:
		return a.t(msgSyncDeliveryReceived), theme.Success
	case deliveryOffline:
		return a.t(msgSyncDeliveryOffline), theme.Warning
	}
	if replaced {
		return a.t(msgSyncDeliveryNotSent), secondaryText(theme)
	}
	return a.t(msgSyncDeliveryPending), secondaryText(theme)
}

// deliveryTextColor keeps a device's outcome text neutral: a settled outcome
// reads in full, one still in flight stays muted. The state color lives on the
// status dot, so small text never leans on a low-contrast accent.
func deliveryTextColor(theme *ui.Theme, state deliveryState) ui.Color {
	switch state {
	case deliveryReceived, deliveryOffline:
		return primaryText(theme)
	default:
		return secondaryText(theme)
	}
}

// historySource names the device an item came from: this machine for a locally
// captured item, or the paired device it arrived from over the LAN. The device
// is resolved from the device list by its stable ID, so a renamed device still
// reads correctly.
func (a *application) historySource(c *ui.Context, theme *ui.Theme, item history.Item) {
	icon := icons.devices
	name := a.t(msgHistorySourceThisDevice)
	if !item.IsLocal() {
		icon = icons.lan
		if device, ok := a.pairedDeviceByID(item.SourceDeviceID); ok {
			name = a.tf(msgHistorySourceLanFormat, device.name)
		} else {
			name = a.t(msgHistorySourceLanDevice)
		}
	}
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		ui.Icon(c, icon).Size(space(theme, 3.25), space(theme, 3.25)).Shrink(0).
			TextColor(secondaryText(theme))
		ui.Text(c, name).Shrink(0).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).SingleLine()
	})
}

func (a *application) historyListText(item history.Item) string {
	if item.Kind == kindImage {
		return a.historyImageLabel(item)
	}
	content := strings.TrimSpace(item.Text)
	if content == "" {
		return a.kindLabel(item.Kind)
	}
	return truncateSingleLine(content, historyTextLimit)
}

func (a *application) historyImageLabel(item history.Item) string {
	if item.Image == nil {
		return a.kindLabel(item.Kind)
	}
	return a.tf(msgHistoryImageLabel, item.Image.Width, item.Image.Height)
}

func (a *application) historyResolutionLabel(item history.Item) string {
	if item.Image == nil {
		return a.t(msgHistoryImageShort)
	}
	return fmt.Sprintf("%dx%d", item.Image.Width, item.Image.Height)
}

// historyMetaSegments 返回预览标题里紧随类型名称的元信息片段：文本为字符数
// 和大小，图片为分辨率和大小，文件只保留大小（字符数对文件没有意义）。
// 片段由渲染方用间隔符连接，这里不拼接分隔符，空片段会被丢弃。
func (a *application) historyMetaSegments(item history.Item) []string {
	switch {
	case item.Kind == kindImage && item.Image != nil:
		return compactSegments(a.historyResolutionLabel(item), historySizeLabel(item))
	case item.Kind == kindFile:
		if size := historySizeLabel(item); size != "" {
			return []string{size}
		}
		return []string{"—"}
	default:
		return compactSegments(a.l10n().charCount(len([]rune(item.Text))), historySizeLabel(item))
	}
}

// compactSegments 丢弃空片段，避免渲染出多余的分隔符。
func compactSegments(values ...string) []string {
	segments := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			segments = append(segments, value)
		}
	}
	return segments
}

// historySizeLabel 返回记录内容的人类可读大小：图片按 PNG 负载字节数，
// 文本按 UTF-8 字节数，文件按记录的负载大小计算；未知时返回空字符串。
func historySizeLabel(item history.Item) string {
	switch item.Kind {
	case kindImage:
		if item.Image == nil || len(item.Image.PNG) == 0 {
			return ""
		}
		return formatByteSize(len(item.Image.PNG))
	case kindFile:
		if item.FileSize <= 0 {
			return ""
		}
		return formatByteSize(int(item.FileSize))
	default:
		return formatByteSize(len(item.Text))
	}
}

// formatByteSize 将字节数格式化为紧凑的大小标签，例如 512 B、1.2 KB、3.4 MB。
func formatByteSize(size int) string {
	if size < 0 {
		size = 0
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}

// timestampLabel 用当前界面语言渲染时间：今天和昨天带具体时刻，
// 更早的记录改用明确日期，跨年时补上年份，避免需要心算的模糊表述。
func (a *application) timestampLabel(value time.Time) string {
	return a.l10n().dateLabel(value)
}

// calendarDaysAgo 按本地日历日计算两个时间相差的整天数，
// 以 UTC 零点归一化，避免夏令时或具体时刻带来偏差。
func calendarDaysAgo(value, now time.Time) int {
	valueDay := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(nowDay.Sub(valueDay).Hours() / 24)
}

func (a *application) historyBitmap(item history.Item) *ui.Bitmap {
	if item.Image == nil || len(item.Image.PNG) == 0 {
		return nil
	}
	if bitmap, ok := a.historyBitmaps[item.ID]; ok {
		return bitmap
	}
	bitmap, err := ui.DecodeBitmap(item.Image.PNG)
	if err != nil {
		return nil
	}
	if a.historyBitmaps == nil {
		a.historyBitmaps = make(map[string]*ui.Bitmap)
	}
	a.historyBitmaps[item.ID] = bitmap
	return bitmap
}

// 预览区底部悬浮操作条的几何（单位与其它间距一致，×4 DIP）。
const (
	previewBarPadding     float32 = 1
	previewBarBottomInset float32 = 3
)

// previewActions 把预览区的操作放进底部的悬浮操作条：操作条固定在面板底部、
// 与正文列同宽并整体居中，所以药丸水平居中悬浮；正文滚动到它下方，它始终浮在
// 内容之上。删除（破坏性）居左，与收藏之间用竖线分隔；复制（主操作）居右，
// 符合主按钮靠右、破坏性操作远离主操作的习惯。
func (a *application) previewActions(c *ui.Context, theme *ui.Theme, item history.Item) {
	ui.Row(c).Absolute().Left(0).Right(0).Bottom(0).Shrink(0).
		Justify(ui.Center).Children(func() {
		ui.Row(c).FillWidth().MaxWidth(pageDetailMaxWidth).Justify(ui.Center).
			Padding(0, space(theme, spacePageSide), space(theme, previewBarBottomInset), space(theme, spacePageSide)).
			Children(func() {
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).
					Padding(space(theme, previewBarPadding)).
					Radius(radius(theme, largeRadius)).
					Background(theme.Background).Border(1, borderColor(theme)).
					Shadow(0, space(theme, 1.5), space(theme, 5), 0, ui.RGBA(0, 0, 0, 0.18)).
					Children(func() {
						deleteLabel := a.t(msgHistoryDelete)
						deleteButton := destructiveActionButton(c, theme, "delete", icons.trash, deleteLabel, "D")
						deleteButton.Tooltip(shortcutHint(deleteLabel, "D"))
						if deleteButton.Clicked() {
							a.requestDeleteItem(c, item.ID)
						}

						ui.Box(c).Width(1).Height(controlHeightSize(c, theme, 4.5)).Shrink(0).
							Background(dividerColor(theme))

						favoriteLabel := a.favoriteLabel(item.Favorite)
						favoriteButton := actionButton(c, theme, "favorite", icons.star, favoriteLabel, item.Favorite, "F")
						favoriteButton.Tooltip(shortcutHint(favoriteLabel, "F"))
						if favoriteButton.Clicked() {
							if message := a.toggleFavorite(item.ID); message != "" {
								c.Toast(message)
							}
						}

						copyLabel := a.t(msgHistoryCopy)
						copyButton := primaryButton(c, theme, "preview-action-copy", icons.copy, copyLabel, "C")
						copyButton.Tooltip(shortcutHint(copyLabel, "C"))
						if copyButton.Clicked() {
							a.copyToClipboard(c, item.ID)
						}
					})
			})
	})
}

// previewActionsClearance 是悬浮操作条占用的高度（药丸加底边距，含边框），
// 预览正文在底部预留同样多的空白，让最后一行内容能滚到操作条上方而不被遮挡。
func previewActionsClearance(c *ui.Context, theme *ui.Theme) float32 {
	return space(theme, previewBarBottomInset) +
		space(theme, previewBarPadding)*2 +
		controlHeightSize(c, theme, compactHeight) + 2
}

func (a *application) copyToClipboard(c *ui.Context, id string) {
	if a.clipboard == nil {
		c.Toast(a.t(msgHistoryCopyFailed))
		return
	}
	if err := a.history.Copy(id, a.clipboard); err != nil {
		c.Toast(a.t(msgHistoryCopyFailed))
		return
	}
	c.Toast(a.t(msgToastCopied))
}

// shortcutHint 在按钮提示里附带单键快捷键，例如“复制 (C)”，让快捷键可被发现。
func shortcutHint(label, key string) string {
	return label + " (" + key + ")"
}

func actionButton(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, active bool, shortcut string) ui.Element {
	button := ui.ButtonBase(c.Key("preview-action-"+key)).
		Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Gap(space(theme, spaceInline)).
		Radius(radius(theme, controlRadius)).
		TextColor(secondaryText(theme)).Label(label).Role(ui.RoleButton)
	fg := secondaryText(theme)
	if active {
		fg = favoriteColor(theme)
		button.TextColor(fg)
	}
	if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0)
		ui.Text(c, label).FontSize(font(theme, fontControl)).FontWeight(500).SingleLine()
		if shortcut != "" {
			keycap(c, theme, shortcut, fg)
		}
	})
	return button
}

// destructiveActionButton 是预览操作条里的删除按钮：尺寸与其它动作一致，但
// 文字与图标取危险色、悬停带一层危险色底，提示这是不可恢复的操作。
func destructiveActionButton(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, shortcut string) ui.Element {
	button := ui.ButtonBase(c.Key("preview-action-"+key)).
		Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Gap(space(theme, spaceInline)).
		Radius(radius(theme, controlRadius)).
		TextColor(theme.Danger).Label(label).Role(ui.RoleButton)
	if button.Hovered() {
		button.Background(theme.Danger.Alpha(0.12))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0)
		ui.Text(c, label).FontSize(font(theme, fontControl)).FontWeight(500).SingleLine()
		if shortcut != "" {
			keycap(c, theme, shortcut, theme.Danger)
		}
	})
	return button
}

func colorForKind(theme *ui.Theme, kind contentKind) ui.Color {
	switch kind {
	case kindLink:
		return theme.Accent
	case kindCode:
		return theme.Accent.Mix(theme.Text, 0.28)
	case kindImage:
		return theme.Success
	case kindFile:
		return theme.Warning
	default:
		return theme.TextMuted
	}
}
