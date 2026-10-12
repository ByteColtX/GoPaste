package main

import (
	"fmt"
	"strings"
	"time"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

type historyFilterOption struct {
	kind  historyFilter
	label string
}

var historyFilterOptions = [...]historyFilterOption{
	{kind: filterAll, label: "全部"},
	{kind: filterText, label: "文本"},
	{kind: filterLinks, label: "链接"},
	{kind: filterCode, label: "代码"},
	{kind: filterImages, label: "图片"},
	{kind: filterFiles, label: "文件"},
	{kind: filterFavorites, label: "收藏"},
}

func (a *application) view(c *ui.Context) {
	theme := c.Theme()
	titleBar := c.TitleBar()
	chromeHeight := max(float32(windowChromeMinHeight), titleBar.Height)
	a.globalShortcuts(c)

	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		// 这条透明区域同时承担红绿灯避让和窗口拖拽两个职责。
		ui.Row(c).Height(chromeHeight).FillWidth().Shrink(0).
			Padding(0, titleBar.Right, 0, titleBar.Left).DragWindow()
		ui.Row(c).Grow(1).MinHeight(0).Children(func() {
			a.navigationRail(c, theme)
			ui.Box(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				a.pageContent(c, theme)
			})
		})
	})
	a.dangerousConfirmation(c, theme)
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
	title, message, confirmLabel, ok := a.dangerousActionDetails(action, a.pendingDangerousDeviceID)
	if !ok {
		a.confirmDangerousAction = false
		a.pendingDangerousAction = dangerousActionNone
		a.pendingDangerousDeviceID = ""
		return
	}

	if dangerConfirmDialog(c, theme, &a.confirmDangerousAction, title, message, confirmLabel) {
		if message := a.executeDangerousAction(action, a.pendingDangerousDeviceID); message != "" {
			c.Toast(message)
		}
	}
	if !a.confirmDangerousAction {
		a.pendingDangerousAction = dangerousActionNone
		a.pendingDangerousDeviceID = ""
	}
}

func (a *application) dangerousActionDetails(action dangerousAction, deviceID string) (string, string, string, bool) {
	switch action {
	case dangerousActionClearHistory:
		return "清空历史记录？", "GoPaste 会删除本机保存的全部剪贴板历史记录。此操作不可恢复。", "清空", true
	case dangerousActionUnpairDevice:
		name := "该设备"
		if device, ok := a.pairedDeviceByID(deviceID); ok {
			name = device.name
		}
		return "取消配对设备？", fmt.Sprintf("取消配对“%s”后，该设备将不再与本机同步。", name), "取消配对", true
	default:
		return "", "", "", false
	}
}

// navigationRail 是窄导航轨道，避免额外占用历史列表的横向空间。
func (a *application) navigationRail(c *ui.Context, theme *ui.Theme) {
	railWidth := navigationRailSize(c, theme)
	expanded := railWidth > navigationRailCompactWidth
	rail := ui.Column(c).Width(railWidth).FillHeight().Shrink(0).
		Padding(space(theme, columnTopInset), space(theme, 2), space(theme, 2.5), space(theme, 2)).
		Gap(space(theme, 1.5)).Background(sidebarFill(theme)).
		Border(1, dividerColor(theme)).
		DragWindow()
	rail.Children(func() {
		a.navigationButton(c, theme, pageHistory, icons.history, "历史记录", expanded)
		ui.Spacer(c)
		a.navigationButton(c, theme, pageDevices, icons.devices, "设备", expanded)
		a.navigationButton(c, theme, pageSettings, icons.settings, "设置", expanded)
	})
}

func (a *application) navigationButton(c *ui.Context, theme *ui.Theme, target page, icon *ui.SVG, label string, expanded bool) {
	button := ui.ButtonBase(c.Key("navigation-"+label)).
		Height(controlHeightSize(c, theme, controlHeight)).FillWidth().Shrink(0).
		Padding(space(theme, 1), space(theme, 2)).
		Radius(radius(theme, cardRadius)).AlignItems(ui.Center).
		Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).
		Label(label).Tooltip(label).Role(ui.RoleToggleButton).Selected(a.page == target)
	if a.page == target {
		button.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).TextColor(primaryText(theme))
	} else if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		if expanded {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(space(theme, 2)).Children(func() {
				ui.Icon(c, icon).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0)
				ui.Text(c, label).FontSize(font(theme, fontControl)).SingleLine()
			})
			return
		}
		ui.Icon(c, icon).Size(space(theme, 4.5), space(theme, 4.5))
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
	view := a.history.Snapshot(history.Query{Text: a.query, Filter: a.filter})
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
		Padding(space(theme, columnTopInset), 0, space(theme, 2.5), space(theme, 4))

	panel.Children(func() {
		a.historyHeader(c, theme, view)
		ui.Box(c).Margin(0, space(theme, historyTrailingInset), 0, 0).Children(func() {
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
	countLabel := fmt.Sprintf("%d 条记录", len(view.Items))
	if a.query != "" || a.filter != filterAll {
		countLabel = fmt.Sprintf("%d / %d 条记录", len(view.Items), view.Total)
	}
	ui.Row(c).MinHeight(controlHeightSize(c, theme, 11)).Shrink(0).AlignItems(ui.Start).
		Margin(0, space(theme, historyTrailingInset), 0, 0).Children(func() {
		ui.Column(c).Gap(space(theme, 0.25)).Padding(space(theme, 0.5), 0, 0, 0).Children(func() {
			ui.Text(c, "最近复制").FontSize(font(theme, fontSection)).FontWeight(600).SingleLine()
			ui.Text(c, countLabel).FontSize(font(theme, fontCaption)).
				TextColor(secondaryText(theme)).SingleLine()
		})
		ui.Spacer(c)
		ui.Icon(c, icons.history).Size(space(theme, 4.25), space(theme, 4.25)).
			AlignSelf(ui.Center).TextColor(secondaryText(theme))
	})
}

func (a *application) searchField(c *ui.Context, theme *ui.Theme) {
	field := ui.Row(c.Key("history-search")).Height(controlHeightSize(c, theme, controlHeight)).FillWidth().Shrink(0).
		AlignItems(ui.Center).Gap(space(theme, 1.5)).Padding(0, space(theme, 2.5)).
		Radius(radius(theme, controlRadius)).Background(theme.Surface).
		Border(1, borderColor(theme)).
		Label("搜索历史记录")

	field.Children(func() {
		ui.Icon(c, icons.search).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0).
			TextColor(secondaryText(theme))
		input := ui.TextInput(c.Key("history-search-input"), &a.query).
			Grow(1).MinWidth(0).Padding(0).Radius(0).
			Background(ui.Transparent).Border(0, ui.Transparent).
			Label("搜索历史记录").Placeholder("搜索").Bind(&a.historySearch).
			FocusRing(c.Focused(a.historySearch))
		input.Changed()

		if a.query == "" {
			if !c.Focused(a.historySearch) {
				searchShortcutHint(c, theme)
			}
			return
		}
		clearButton := ui.ButtonBase(c.Key("clear-history-search")).
			Size(space(theme, 5), space(theme, 5)).Shrink(0).Radius(radius(theme, 2.5)).Center().
			TextColor(secondaryText(theme)).
			Label("清除搜索").Tooltip("清除搜索").Role(ui.RoleButton)
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
		Margin(space(theme, historyControlGap), space(theme, historyTrailingInset), 0, 0).
		ScrollbarInsets(0, 0, 0, 0).Children(func() {
		ui.Row(c).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).AlignItems(ui.Center).
			Gap(space(theme, 0.5)).Children(func() {
			for _, option := range historyFilterOptions {
				a.filterButton(c, theme, option.kind, option.label)
			}
		})
	})
}

func (a *application) filterButton(c *ui.Context, theme *ui.Theme, filter historyFilter, label string) {
	button := ui.ButtonBase(c.Key("history-filter-"+label)).
		Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 0.75), space(theme, 1.75)).Radius(radius(theme, controlRadius)).
		Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).Label(label).
		Role(ui.RoleToggleButton).Selected(a.filter == filter)
	if a.filter == filter {
		button.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).TextColor(primaryText(theme))
	} else if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(font(theme, fontCaption)).SingleLine()
	})
	if button.Clicked() {
		a.filter = filter
	}
}

func (a *application) emptyHistory(c *ui.Context, theme *ui.Theme, view history.View) {
	firstUse := view.Total == 0 && a.query == "" && a.filter == filterAll
	searchEmpty := a.query != ""
	title := "没有匹配记录"
	message := "换个关键词或筛选条件试试"
	icon := icons.filter
	if firstUse {
		title = "这里还没有记录"
		message = "复制文本或图片后，历史记录会显示在这里。记录保存在本机。"
		icon = icons.history
	} else if searchEmpty {
		title = "没有找到匹配记录"
		message = fmt.Sprintf("没有包含“%s”的记录。可以清除搜索或换个关键词。", truncateSingleLine(a.query, 24))
		icon = icons.search
	} else if a.filter != filterAll {
		title = "这个筛选条件暂无记录"
		message = fmt.Sprintf("当前没有“%s”记录。可以显示全部记录。", historyFilterLabel(a.filter))
		icon = icons.filter
	}
	ui.Box(c).Grow(1).Center().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(320).Shrink(0).Center().Gap(space(theme, 1.75)).Children(func() {
			ui.Icon(c, icon).Size(space(theme, 5), space(theme, 5)).
				TextColor(secondaryText(theme))
			ui.Text(c, title).FontSize(font(theme, fontSection)).FontWeight(600).
				Role(ui.RoleHeading).SingleLine()
			ui.Text(c, message).FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme)).Wrap()
			if firstUse {
				return
			}
			resetLabel := "清除搜索和筛选"
			if searchEmpty && a.filter == filterAll {
				resetLabel = "清除搜索"
			} else if !searchEmpty && a.filter != filterAll {
				resetLabel = "显示全部记录"
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

func historyFilterLabel(filter historyFilter) string {
	for _, option := range historyFilterOptions {
		if option.kind == filter {
			return option.label
		}
	}
	return "当前"
}

func (a *application) historyRows(c *ui.Context, theme *ui.Theme, view history.View) {
	// 不使用 ListState.Selected，让列表的默认 Accent 背景不覆盖自定义卡片样式。
	// 键盘快捷键仍注册在列表容器上，因此焦点在任意条目时都能移动选择。
	a.listState.Selected = nil
	a.listState.Key = func(index int) any { return view.Items[index].ID }
	a.listState.Label = func(index int) string { return historyListText(view.Items[index]) }

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
	query := history.Query{Text: a.query, Filter: a.filter}
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
		MinHeight(controlHeightSize(c, theme, historyItemHeight/4)).AlignSelf(ui.Stretch).Shrink(0).
		Margin(space(theme, historyControlGap), space(theme, historyTrailingInset), space(theme, historyControlGap), 0).
		Padding(space(theme, 2.5), space(theme, 3)).Gap(space(theme, 1.5)).
		// 预留透明边框，切换选中态时不改变内容布局。
		Radius(radius(theme, cardRadius)).Border(1, ui.Transparent).Column().
		Justify(ui.Start).AlignItems(ui.Stretch).
		TextColor(primaryText(theme)).Label(historyListText(item)).
		Role(ui.RoleButton).Selected(selected)
	if selected {
		row.Background(selectedFill(theme)).
			BorderColor(selectionBorder(c, theme))
	} else if row.Hovered() {
		row.Background(hoverFill(theme)).BorderColor(ui.Transparent)
	} else {
		row.Background(ui.Transparent).BorderColor(ui.Transparent)
	}

	row.Children(func() {
		if item.Kind == kindImage && item.Image != nil {
			a.imageHistoryRow(c, theme, item)
			return
		}
		a.textHistoryRow(c, theme, item)
	})
	return row
}

func (a *application) textHistoryRow(c *ui.Context, theme *ui.Theme, item history.Item) {
	a.historyMetaRow(c, theme, item)
	ui.Text(c, historyListText(item)).FontSize(font(theme, fontBody)).
		TextColor(primaryText(theme)).SingleLine()
	a.autoTagRow(c, theme, item)
}

func (a *application) historyMetaRow(c *ui.Context, theme *ui.Theme, item history.Item) {
	ui.Row(c).MinHeight(controlHeightSize(c, theme, 4)).Shrink(0).AlignItems(ui.Center).
		Gap(space(theme, 1.25)).Children(func() {
		baseKind := item.Kind.BaseKind()
		kindColor := colorForKind(theme, baseKind)
		ui.Icon(c, iconForKind(baseKind)).Size(space(theme, 3.25), space(theme, 3.25)).
			Shrink(0).TextColor(kindColor)
		ui.Text(c, baseKind.DisplayName()).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(kindColor).SingleLine()
		if item.Favorite {
			ui.Icon(c, icons.star).Size(space(theme, 2.75), space(theme, 2.75)).Shrink(0).
				TextColor(favoriteColor(theme))
		}
		ui.Spacer(c)
		ui.Text(c, historyTimestamp(item.CreatedAt)).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).SingleLine()
	})
}

func (a *application) imageHistoryRow(c *ui.Context, theme *ui.Theme, item history.Item) {
	a.historyMetaRow(c, theme, item)
	ui.Row(c).MinHeight(controlHeightSize(c, theme, 10)).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2.5)).Children(func() {
		ui.Box(c).Size(space(theme, 10), space(theme, 10)).Shrink(0).Radius(radius(theme, controlRadius)).
			Background(subtleFill(theme)).Border(1, borderColor(theme)).Children(func() {
			if bitmap := a.historyBitmap(item); bitmap != nil {
				ui.Image(c, bitmap).Size(space(theme, 9.5), space(theme, 9.5)).Fit(ui.Cover).Radius(radius(theme, 1.75))
			}
		})
		ui.Text(c, historyResolutionLabel(item)).FontSize(font(theme, fontBody)).
			TextColor(secondaryText(theme)).SingleLine()
	})
}

func (a *application) autoTagRow(c *ui.Context, theme *ui.Theme, item history.Item) {
	tag := item.Kind.SmartTag()
	if tag == "" {
		return
	}
	tagColor := colorForKind(theme, item.Kind)
	ui.Row(c).MinHeight(controlHeightSize(c, theme, 4.5)).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, tag).FontSize(font(theme, fontCaption)).TextColor(tagColor).
			Padding(space(theme, 0.25), space(theme, 1.5)).Radius(radius(theme, controlRadius)).
			Background(tagColor.Alpha(0.10)).
			Border(1, tagColor.Alpha(0.24)).SingleLine()
	})
}

func (a *application) historyPreview(c *ui.Context, theme *ui.Theme, view history.View) {
	pane := ui.Column(c).Grow(1).FillHeight().
		MinWidth(minPreviewPaneWidth).Background(theme.Background)
	pane.Children(func() {
		item, ok := selectedHistoryItem(view)
		if !ok {
			a.emptyPreview(c, theme)
			return
		}

		ui.Column(c).Fill().Children(func() {
			a.previewHeader(c, theme, item)
			a.previewContent(c, theme, item)
		})
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
		ui.Column(c).Center().Gap(space(theme, 1.75)).Children(func() {
			ui.Icon(c, icons.history).Size(space(theme, 6), space(theme, 6)).
				TextColor(secondaryText(theme))
			ui.Text(c, "选择一条记录").FontSize(font(theme, fontSection)).FontWeight(600)
			ui.Text(c, "预览会显示在这里").FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme))
		})
	})
}

func (a *application) previewHeader(c *ui.Context, theme *ui.Theme, item history.Item) {
	// 与设备、设置详情一致：先限制最大宽度并居中，再应用左右内边距，
	// 让标题、操作按钮和正文卡片的左右边缘对齐。
	ui.Column(c).Shrink(0).FillWidth().MaxWidth(pageDetailMaxWidth).AlignSelf(ui.Center).
		Padding(space(theme, columnTopInset), space(theme, spacePageSide), space(theme, 3.5), space(theme, spacePageSide)).
		Gap(space(theme, 1.25)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
			baseKind := item.Kind.BaseKind()
			kindColor := colorForKind(theme, baseKind)
			ui.Icon(c, iconForKind(baseKind)).Size(space(theme, 3.5), space(theme, 3.5)).
				Shrink(0).TextColor(kindColor)
			ui.Text(c, baseKind.DisplayName()).FontSize(font(theme, fontMeta)).FontWeight(600).
				TextColor(kindColor).SingleLine()
			ui.Text(c, "·").FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme))
			if item.Kind == kindImage && item.Image != nil {
				ui.Text(c, "原始图片").FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme)).SingleLine()
			} else {
				ui.Textf(c, "%d 字符", len([]rune(item.Text))).FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme)).SingleLine()
			}
			ui.Spacer(c)
		})
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
			ui.Textf(c, "%s · %s", item.Source, historyTimestamp(item.CreatedAt)).
				FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).SingleLine()
			ui.Spacer(c)
			a.actionBar(c, theme, item)
		})
		a.autoTagRow(c, theme, item)
	})
}

func (a *application) previewContent(c *ui.Context, theme *ui.Theme, item history.Item) {
	content := ui.Scroll(c).Grow(1).MinHeight(0)
	content.Children(func() {
		ui.Column(c).FillWidth().MaxWidth(pageDetailMaxWidth).AlignSelf(ui.Center).
			Padding(0, space(theme, spacePageSide), space(theme, spacePageBottom), space(theme, spacePageSide)).
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
					ui.Text(c, "图片预览不可用").FontSize(font(theme, fontBody)).
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

func historyListText(item history.Item) string {
	if item.Kind == kindImage {
		return historyImageLabel(item)
	}
	content := strings.TrimSpace(item.Text)
	if content == "" {
		return item.Kind.DisplayName()
	}
	return truncateSingleLine(content, historyTextLimit)
}

func historyImageLabel(item history.Item) string {
	if item.Image == nil {
		return item.Kind.DisplayName()
	}
	return fmt.Sprintf("图片（%dx%d）", item.Image.Width, item.Image.Height)
}

func historyResolutionLabel(item history.Item) string {
	if item.Image == nil {
		return "图片"
	}
	return fmt.Sprintf("%dx%d", item.Image.Width, item.Image.Height)
}

func historyTimestamp(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	now := time.Now()
	date := func(value time.Time) time.Time {
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
	}
	daysAgo := int(date(now).Sub(date(value)).Hours() / 24)
	switch daysAgo {
	case 0:
		return "今天 " + value.Format("15:04")
	case 1:
		return "昨天 " + value.Format("15:04")
	}
	if daysAgo >= 2 && daysAgo < 7 {
		return "周" + weekdayLabel(value.Weekday()) + " " + value.Format("15:04")
	}
	if daysAgo >= 7 && daysAgo < 14 {
		return "上周" + weekdayLabel(value.Weekday()) + " " + value.Format("15:04")
	}
	return value.Format("2006-01-02 15:04")
}

func weekdayLabel(day time.Weekday) string {
	switch day {
	case time.Monday:
		return "一"
	case time.Tuesday:
		return "二"
	case time.Wednesday:
		return "三"
	case time.Thursday:
		return "四"
	case time.Friday:
		return "五"
	case time.Saturday:
		return "六"
	default:
		return "日"
	}
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

func (a *application) actionBar(c *ui.Context, theme *ui.Theme, item history.Item) {
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 0.5)).Children(func() {
		copyLabel := "复制"
		copyButton := actionButton(c, theme, "copy", icons.copy, copyLabel, false)
		if copyButton.Clicked() {
			a.copyToClipboard(c, item.ID)
		}

		favoriteButton := actionButton(c, theme, "favorite", icons.star, favoriteLabel(item.Favorite), item.Favorite)
		if favoriteButton.Clicked() {
			if message := a.toggleFavorite(item.ID); message != "" {
				c.Toast(message)
			}
		}
	})
}

func (a *application) copyToClipboard(c *ui.Context, id string) {
	if a.clipboard == nil {
		c.Toast("复制失败")
		return
	}
	if err := a.history.Copy(id, a.clipboard); err != nil {
		c.Toast("复制失败")
		return
	}
	c.Toast(copiedStatus)
}

func actionButton(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, active bool) ui.Element {
	button := ui.ButtonBase(c.Key("preview-action-"+key)).
		Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Gap(space(theme, spaceInline)).
		Radius(radius(theme, controlRadius)).
		TextColor(secondaryText(theme)).Label(label).Role(ui.RoleButton)
	if active {
		button.TextColor(favoriteColor(theme))
	}
	if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0)
		ui.Text(c, label).FontSize(font(theme, fontControl)).SingleLine()
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
