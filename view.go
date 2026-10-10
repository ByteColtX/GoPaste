package main

import (
	"github.com/egoist/mygo"
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
	chromeHeight := float32(windowChromeMinHeight)

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
}

// navigationRail 是窄导航轨道，避免额外占用历史列表的横向空间。
func (a *application) navigationRail(c *ui.Context, theme *ui.Theme) {
	rail := ui.Column(c).Width(navigationRailWidth).FillHeight().Shrink(0).
		Padding(space(theme, 3), space(theme, 2), space(theme, 2.5), space(theme, 2)).
		Gap(space(theme, 1.5)).Background(sidebarFill(theme)).
		Border(1, dividerColor(theme)).
		DragWindow()
	rail.Children(func() {
		a.navigationButton(c, theme, pageHistory, icons.history, "历史记录")
		ui.Spacer(c)
		a.navigationButton(c, theme, pageDevices, icons.devices, "设备")
		a.navigationButton(c, theme, pageSettings, icons.settings, "设置")
	})
}

func (a *application) navigationButton(c *ui.Context, theme *ui.Theme, target page, icon *ui.SVG, label string) {
	button := ui.ButtonBase(c.Key("navigation-" + label)).
		Height(space(theme, 11)).FillWidth().Shrink(0).
		Radius(radius(theme, cardRadius)).Center().
		TextColor(secondaryText(theme)).
		Label(label).Tooltip(label)
	if a.page == target {
		button.Background(selectedFill(theme)).TextColor(primaryText(theme))
	} else if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(space(theme, 5), space(theme, 5))
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
	visible := a.visibleItems()
	a.keepSelection(visible)

	a.constrainHistoryPaneWidth(c)

	ui.Split(c, &a.historyPaneWidth,
		func() { a.historyList(c, theme, visible) },
		func() { a.historyPreview(c, theme, visible) },
	).Fill()
}

// constrainHistoryPaneWidth 在窗口缩放时保持左右面板都可用。
func (a *application) constrainHistoryPaneWidth(c *ui.Context) {
	if a.historyPaneWidth <= 0 {
		a.historyPaneWidth = defaultHistoryPaneWidth
	}

	windowWidth, _ := c.Size()
	maxWidth := windowWidth - navigationRailWidth - minPreviewPaneWidth - splitDividerWidth
	maxWidth = max(maxWidth, float32(minHistoryPaneWidth))
	a.historyPaneWidth = min(max(a.historyPaneWidth, float32(minHistoryPaneWidth)), maxWidth)
}

func (a *application) historyList(c *ui.Context, theme *ui.Theme, visible []clipboardItem) {
	panel := ui.Column(c).Fill().
		MinWidth(minHistoryPaneWidth).
		Background(sidebarFill(theme)).
		Padding(space(theme, 4), 0, space(theme, 2.5), space(theme, 4))

	panel.Children(func() {
		a.historyHeader(c, theme, len(visible))
		ui.Box(c).Margin(0, space(theme, historyTrailingInset), 0, 0).Children(func() {
			a.searchField(c, theme)
		})
		a.filterBar(c, theme)

		if len(visible) == 0 {
			a.emptyHistory(c, theme)
			return
		}

		a.historyRows(c, theme, visible)
	})
}

func (a *application) historyHeader(c *ui.Context, theme *ui.Theme, count int) {
	ui.Row(c).Height(space(theme, 11)).Shrink(0).AlignItems(ui.Center).
		Margin(0, space(theme, 4), 0, 0).Children(func() {
		ui.Column(c).Gap(space(theme, 0.25)).Children(func() {
			ui.Text(c, "最近复制").FontSize(font(theme, fontSection)).FontWeight(600).SingleLine()
			ui.Textf(c, "%d 条记录", count).FontSize(font(theme, fontCaption)).
				TextColor(secondaryText(theme)).SingleLine()
		})
		ui.Spacer(c)
		ui.Icon(c, icons.history).Size(space(theme, 4.25), space(theme, 4.25)).
			TextColor(secondaryText(theme))
	})
}

func (a *application) searchField(c *ui.Context, theme *ui.Theme) {
	field := ui.Row(c.Key("history-search")).Height(space(theme, controlHeight)).FillWidth().Shrink(0).
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
			Label("搜索历史记录").Placeholder("搜索")
		input.Changed()

		if a.query == "" {
			return
		}
		clearButton := ui.ButtonBase(c.Key("clear-history-search")).
			Size(space(theme, 5), space(theme, 5)).Shrink(0).Radius(radius(theme, 2.5)).Center().
			TextColor(secondaryText(theme)).
			Label("清除搜索").Tooltip("清除搜索")
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

func (a *application) filterBar(c *ui.Context, theme *ui.Theme) {
	ui.Row(c).Height(space(theme, controlHeight)).Shrink(0).AlignItems(ui.Center).
		Gap(space(theme, 0.5)).Margin(0, space(theme, historyTrailingInset), 0, 0).Children(func() {
		for _, option := range historyFilterOptions {
			a.filterButton(c, theme, option.kind, option.label)
		}
	})
}

func (a *application) filterButton(c *ui.Context, theme *ui.Theme, filter historyFilter, label string) {
	button := ui.ButtonBase(c.Key("history-filter-"+label)).
		Height(space(theme, compactHeight)).Shrink(0).
		Padding(space(theme, 0.75), space(theme, 1.75)).Radius(radius(theme, controlRadius)).
		TextColor(secondaryText(theme))
	if a.filter == filter {
		button.Background(selectedFill(theme)).TextColor(primaryText(theme))
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

func (a *application) emptyHistory(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).Grow(1).Center().Children(func() {
		ui.Column(c).Center().Gap(space(theme, 1.75)).Children(func() {
			ui.Icon(c, icons.filter).Size(space(theme, 5), space(theme, 5)).
				TextColor(secondaryText(theme))
			ui.Text(c, "没有匹配记录").FontSize(font(theme, fontSection)).FontWeight(600)
			ui.Text(c, "换个关键词或筛选条件试试").FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme))
		})
	})
}

func (a *application) historyRows(c *ui.Context, theme *ui.Theme, visible []clipboardItem) {
	// 列表自身只负责滚动和键盘选择，选中背景由历史条目统一绘制。
	a.listState.Selected = nil
	a.listState.Key = func(index int) any { return visible[index].id }
	a.listState.Label = func(index int) string { return visible[index].listText() }

	list := ui.List(c, &a.listState, len(visible), func(index int) {
		item := visible[index]
		row := a.historyRow(c, theme, item, index)
		if row.Clicked() {
			a.selectItem(index, item)
		}
	}).ScrollbarInsets(0, 0, 0, 0)
	list.Grow(1)
}

func (a *application) historyRow(c *ui.Context, theme *ui.Theme, item clipboardItem, index int) ui.Element {
	row := ui.ButtonBase(c.Key("history-row-"+item.id)).
		Height(historyItemHeight).AlignSelf(ui.Stretch).Shrink(0).
		Margin(space(theme, 1), space(theme, historyTrailingInset), space(theme, 1), 0).
		Padding(space(theme, 2.5), space(theme, 3)).Gap(space(theme, 1.5)).
		// 预留透明边框，切换选中态时不改变内容布局。
		Radius(radius(theme, cardRadius)).Border(1, ui.Transparent).Column().
		Justify(ui.Start).AlignItems(ui.Stretch).
		TextColor(primaryText(theme)).Label(item.listText())
	if a.selectedIndex == index {
		row.Background(selectedFill(theme)).
			BorderColor(theme.Accent.Alpha(0.22))
	} else if row.Hovered() {
		row.Background(hoverFill(theme)).BorderColor(ui.Transparent)
	} else {
		row.Background(ui.Transparent).BorderColor(ui.Transparent)
	}

	row.Children(func() {
		if item.kind == kindImage && item.image != nil {
			a.imageHistoryRow(c, theme, item)
			return
		}
		a.textHistoryRow(c, theme, item)
	})
	return row
}

func (a *application) textHistoryRow(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	a.historyMetaRow(c, theme, item)
	ui.Text(c, item.listText()).FontSize(font(theme, fontBody)).
		TextColor(primaryText(theme)).SingleLine()
	a.autoTagRow(c, theme, item)
}

func (a *application) historyMetaRow(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	ui.Row(c).Height(space(theme, 4)).Shrink(0).AlignItems(ui.Center).
		Gap(space(theme, 1.25)).Children(func() {
		baseKind := item.baseKind()
		kindColor := colorForKind(theme, baseKind)
		ui.Icon(c, iconForKind(baseKind)).Size(space(theme, 3.25), space(theme, 3.25)).
			Shrink(0).TextColor(kindColor)
		ui.Text(c, baseKind.displayName()).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(kindColor).SingleLine()
		if item.favorite {
			ui.Icon(c, icons.star).Size(space(theme, 2.75), space(theme, 2.75)).Shrink(0).
				TextColor(ui.Hex("#d69e2e"))
		}
		ui.Spacer(c)
		ui.Text(c, item.timestamp).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).SingleLine()
	})
}

func (a *application) imageHistoryRow(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	a.historyMetaRow(c, theme, item)
	ui.Row(c).Height(space(theme, 10)).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2.5)).Children(func() {
		ui.Box(c).Size(space(theme, 10), space(theme, 10)).Shrink(0).Radius(radius(theme, controlRadius)).
			Background(subtleFill(theme)).Border(1, borderColor(theme)).Children(func() {
			ui.Image(c, item.image.bitmap).Size(space(theme, 9.5), space(theme, 9.5)).Fit(ui.Cover).Radius(radius(theme, 1.75))
		})
		ui.Text(c, item.resolutionLabel()).FontSize(font(theme, fontBody)).
			TextColor(secondaryText(theme)).SingleLine()
	})
}

func (a *application) autoTagRow(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	tag := item.smartTag()
	if tag == "" {
		return
	}
	tagColor := colorForKind(theme, item.kind)
	ui.Row(c).Height(space(theme, 4.5)).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, tag).FontSize(font(theme, fontCaption)).TextColor(tagColor).
			Padding(space(theme, 0.25), space(theme, 1.5)).Radius(radius(theme, controlRadius)).
			Background(tagColor.Alpha(0.10)).
			Border(1, tagColor.Alpha(0.24)).SingleLine()
	})
}

func (a *application) historyPreview(c *ui.Context, theme *ui.Theme, visible []clipboardItem) {
	pane := ui.Column(c).Grow(1).FillHeight().
		MinWidth(minPreviewPaneWidth).Background(theme.Background)
	pane.Children(func() {
		item, ok := a.selectedVisibleItem(visible)
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

func (a *application) selectedVisibleItem(visible []clipboardItem) (clipboardItem, bool) {
	if a.selectedIndex < 0 || a.selectedIndex >= len(visible) {
		return clipboardItem{}, false
	}
	return visible[a.selectedIndex], true
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

func (a *application) previewHeader(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	ui.Column(c).Shrink(0).Padding(space(theme, 6), space(theme, 7), space(theme, 3.5), space(theme, 7)).
		Gap(space(theme, 1.25)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
			baseKind := item.baseKind()
			kindColor := colorForKind(theme, baseKind)
			ui.Icon(c, iconForKind(baseKind)).Size(space(theme, 3.5), space(theme, 3.5)).
				Shrink(0).TextColor(kindColor)
			ui.Text(c, baseKind.displayName()).FontSize(font(theme, fontMeta)).FontWeight(600).
				TextColor(kindColor).SingleLine()
			ui.Text(c, "·").FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme))
			if item.kind == kindImage && item.image != nil {
				ui.Text(c, "原始图片").FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme)).SingleLine()
			} else {
				ui.Textf(c, "%d 字符", len([]rune(item.content))).FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme)).SingleLine()
			}
			ui.Spacer(c)
			ui.Textf(c, "%s · %s", item.source, item.timestamp).
				FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).SingleLine()
			a.actionBar(c, theme, item)
		})
		a.autoTagRow(c, theme, item)
	})
}

func (a *application) previewContent(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	content := ui.Scroll(c).Grow(1).MinHeight(0).
		Padding(0, space(theme, 7), space(theme, spacePageBottom), space(theme, 7))
	content.Children(func() {
		ui.Column(c).MaxWidth(pageDetailMaxWidth).Gap(space(theme, 3)).Children(func() {
			switch item.kind {
			case kindCode:
				ui.Text(c, item.content).Font("monospace").FontSize(font(theme, fontCode)).
					TextColor(primaryText(theme)).Padding(space(theme, 4)).Radius(radius(theme, cardRadius)).
					Background(panelFill(theme)).Border(1, borderColor(theme)).Wrap().Selectable()
			case kindLink:
				ui.Row(c).Gap(space(theme, 2)).AlignItems(ui.Center).Padding(space(theme, 4)).
					Radius(radius(theme, cardRadius)).Background(panelFill(theme)).
					Border(1, borderColor(theme)).Selectable().Children(func() {
					ui.Icon(c, icons.link).Size(space(theme, 4), space(theme, 4)).Shrink(0).
						TextColor(colorForKind(theme, item.kind))
					ui.Link(c, item.content, item.content).FontSize(font(theme, fontBody)).
						TextColor(theme.Accent).Wrap()
				})
			case kindImage:
				if item.image == nil {
					ui.Text(c, "图片预览不可用").FontSize(font(theme, fontBody)).
						TextColor(secondaryText(theme)).Padding(space(theme, 4)).Radius(radius(theme, cardRadius)).
						Background(panelFill(theme)).Border(1, borderColor(theme))
					return
				}
				ui.Box(c).MaxWidth(pageDetailMaxWidth).Center().Padding(space(theme, 3)).Radius(radius(theme, largeRadius)).
					Background(panelFill(theme)).Border(1, borderColor(theme)).Children(func() {
					ui.Image(c, item.image.bitmap).MaxWidth(pageDetailMaxWidth - space(theme, 6)).MaxHeight(520).
						Fit(ui.Contain).Radius(radius(theme, controlRadius))
				})
			default:
				ui.Text(c, item.content).FontSize(font(theme, fontBody)).
					TextColor(primaryText(theme)).Padding(space(theme, 4)).Radius(radius(theme, cardRadius)).
					Background(panelFill(theme)).Border(1, borderColor(theme)).Wrap().Selectable()
			}
		})
	})
}

func (a *application) actionBar(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 0.5)).Children(func() {
		copyLabel := "复制"
		copyButton := actionButton(c, theme, "copy", icons.copy, copyLabel, false)
		if copyButton.Clicked() {
			a.copyToClipboard(c, item)
		}

		favoriteButton := actionButton(c, theme, "favorite", icons.star, favoriteLabel(item.favorite), item.favorite)
		if favoriteButton.Clicked() {
			if message := a.toggleFavorite(item.id); message != "" {
				c.Toast(message)
			}
		}
	})
}

func (a *application) copyToClipboard(c *ui.Context, item clipboardItem) {
	if item.kind == kindImage && item.image != nil {
		if err := mygo.Clipboard.WriteImage(item.image.png); err != nil {
			c.Toast("复制失败")
			return
		}
	} else {
		mygo.Clipboard.WriteText(item.content)
	}
	c.Toast(copiedStatus)
}

func actionButton(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, active bool) ui.Element {
	button := ui.ButtonBase(c.Key("preview-action-"+key)).
		Height(space(theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Gap(space(theme, spaceInline)).
		Radius(radius(theme, controlRadius)).
		TextColor(secondaryText(theme)).Label(label)
	if active {
		button.TextColor(ui.Hex("#d69e2e"))
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
	var color ui.Color
	switch kind {
	case kindLink:
		color = ui.Hex("#468fd2")
	case kindCode:
		color = ui.Hex("#8c78d2")
	case kindImage:
		color = ui.Hex("#50a06e")
	case kindFile:
		color = ui.Hex("#af8c64")
	default:
		color = ui.Hex("#8c96a0")
	}
	if theme.Dark {
		return color.Mix(theme.Text, 0.18)
	}
	return color
}
