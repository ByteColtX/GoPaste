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
		Padding(12, 8, 10, 8).Gap(6).
		Background(theme.Surface.Alpha(0.28)).
		Border(1, theme.Border.Alpha(0.42)).
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
		Height(42).FillWidth().Shrink(0).
		Radius(10).Center().
		TextColor(theme.TextMuted).
		Label(label).Tooltip(label)
	if a.page == target {
		button.Background(theme.Accent.Alpha(0.16)).TextColor(theme.Text)
	} else if button.Hovered() {
		button.Background(theme.Text.Alpha(0.07))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(19, 19)
	})
	if button.Clicked() {
		a.page = target
		a.statusMessage = ""
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
		Background(theme.Surface.Alpha(0.22)).
		Padding(16, 0, 10, 16)

	panel.Children(func() {
		a.historyHeader(c, theme, len(visible))
		ui.Box(c).Margin(0, 16, 0, 0).Children(func() {
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
	ui.Row(c).Height(42).Shrink(0).AlignItems(ui.Center).
		Margin(0, 16, 0, 0).Children(func() {
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, "最近复制").FontSize(14).FontWeight(600).SingleLine()
			ui.Textf(c, "%d 条记录", count).FontSize(11).
				TextColor(theme.TextMuted).SingleLine()
		})
		ui.Spacer(c)
		ui.Icon(c, icons.history).Size(17, 17).
			TextColor(theme.TextMuted.Alpha(0.72))
	})
}

func (a *application) searchField(c *ui.Context, theme *ui.Theme) {
	field := ui.Row(c.Key("history-search")).Height(34).FillWidth().Shrink(0).
		AlignItems(ui.Center).Gap(6).Padding(0, 10).Radius(9).
		Background(theme.Surface.Alpha(0.58)).
		Border(1, theme.Border.Alpha(0.52)).
		Label("搜索历史记录")

	field.Children(func() {
		ui.Icon(c, icons.search).Size(14, 14).Shrink(0).
			TextColor(theme.TextMuted)
		input := ui.TextInput(c.Key("history-search-input"), &a.query).
			Grow(1).MinWidth(0).Padding(0).Radius(0).
			Background(ui.Transparent).Border(0, ui.Transparent).
			Label("搜索历史记录").Placeholder("搜索")
		input.Changed()

		if a.query == "" {
			return
		}
		clearButton := ui.ButtonBase(c.Key("clear-history-search")).
			Size(20, 20).Shrink(0).Radius(10).Center().
			TextColor(theme.TextMuted).
			Label("清除搜索").Tooltip("清除搜索")
		if clearButton.Hovered() {
			clearButton.Background(theme.SurfaceHover)
		}
		clearButton.Children(func() {
			ui.Icon(c, icons.clear).Size(12, 12)
		})
		if clearButton.Clicked() {
			a.query = ""
		}
	})
}

func (a *application) filterBar(c *ui.Context, theme *ui.Theme) {
	ui.Row(c).Height(34).Shrink(0).AlignItems(ui.Center).
		Gap(2).Margin(0, 16, 0, 0).Children(func() {
		for _, option := range historyFilterOptions {
			a.filterButton(c, theme, option.kind, option.label)
		}
	})
}

func (a *application) filterButton(c *ui.Context, theme *ui.Theme, filter historyFilter, label string) {
	button := ui.ButtonBase(c.Key("history-filter-"+label)).
		Height(26).Shrink(0).Padding(3, 7).Radius(6).
		TextColor(theme.TextMuted)
	if a.filter == filter {
		button.Background(theme.Accent.Alpha(0.16)).TextColor(theme.Text)
	} else if button.Hovered() {
		button.Background(theme.Text.Alpha(0.07))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Text(c, label).FontSize(11).SingleLine()
	})
	if button.Clicked() {
		a.filter = filter
	}
}

func (a *application) emptyHistory(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).Grow(1).Center().Children(func() {
		ui.Column(c).Center().Gap(7).Children(func() {
			ui.Icon(c, icons.filter).Size(20, 20).
				TextColor(theme.TextMuted.Alpha(0.35))
			ui.Text(c, "没有匹配记录").FontSize(14).FontWeight(600)
			ui.Text(c, "换个关键词或筛选条件试试").FontSize(12).
				TextColor(theme.TextMuted)
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
		Margin(4, 8, 4, 0).Padding(10, 12).Gap(6).
		// 预留透明边框，切换选中态时不改变内容布局。
		Radius(14).Border(1, ui.Transparent).Column().
		Justify(ui.Start).AlignItems(ui.Stretch).
		TextColor(theme.Text).Label(item.listText())
	if a.selectedIndex == index {
		row.Background(theme.Text.Alpha(0.08)).
			BorderColor(theme.Text.Alpha(0.14))
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
	ui.Text(c, item.listText()).FontSize(13).
		TextColor(theme.Text.Alpha(0.9)).SingleLine()
	a.autoTagRow(c, theme, item)
}

func (a *application) historyMetaRow(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	ui.Row(c).Height(16).Shrink(0).AlignItems(ui.Center).
		Gap(5).Children(func() {
		baseKind := item.baseKind()
		kindColor := colorForKind(theme, baseKind)
		ui.Icon(c, iconForKind(baseKind)).Size(13, 13).
			Shrink(0).TextColor(kindColor)
		ui.Text(c, baseKind.displayName()).FontSize(11).FontWeight(600).
			TextColor(kindColor).SingleLine()
		if item.favorite {
			ui.Icon(c, icons.star).Size(11, 11).Shrink(0).
				TextColor(ui.Hex("#d69e2e"))
		}
		ui.Spacer(c)
		ui.Text(c, item.timestamp).FontSize(11).
			TextColor(theme.TextMuted.Alpha(0.72)).SingleLine()
	})
}

func (a *application) imageHistoryRow(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	a.historyMetaRow(c, theme, item)
	ui.Row(c).Height(40).Shrink(0).AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Box(c).Size(40, 40).Shrink(0).Radius(8).
			Background(theme.Surface.Alpha(0.62)).
			Border(1, theme.Border.Alpha(0.42)).Children(func() {
			ui.Image(c, item.image.bitmap).Size(38, 38).Fit(ui.Cover).Radius(7)
		})
		ui.Text(c, item.resolutionLabel()).FontSize(13).
			TextColor(theme.TextMuted).SingleLine()
	})
}

func (a *application) autoTagRow(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	tag := item.smartTag()
	if tag == "" {
		return
	}
	tagColor := colorForKind(theme, item.kind)
	ui.Row(c).Height(18).Shrink(0).AlignItems(ui.Center).Children(func() {
		ui.Text(c, tag).FontSize(11).TextColor(tagColor).
			Padding(1, 6).Radius(6).
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
		ui.Column(c).Center().Gap(7).Children(func() {
			ui.Icon(c, icons.history).Size(24, 24).
				TextColor(theme.TextMuted.Alpha(0.3))
			ui.Text(c, "选择一条记录").FontSize(14).FontWeight(600)
			ui.Text(c, "预览会显示在这里").FontSize(12).
				TextColor(theme.TextMuted)
		})
	})
}

func (a *application) previewHeader(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	ui.Column(c).Shrink(0).Padding(24, 28, 14, 28).Gap(5).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			baseKind := item.baseKind()
			kindColor := colorForKind(theme, baseKind)
			ui.Icon(c, iconForKind(baseKind)).Size(14, 14).
				Shrink(0).TextColor(kindColor)
			ui.Text(c, baseKind.displayName()).FontSize(12).FontWeight(600).
				TextColor(kindColor).SingleLine()
			ui.Text(c, "·").FontSize(12).
				TextColor(theme.TextMuted.Alpha(0.55))
			if item.kind == kindImage && item.image != nil {
				ui.Text(c, "原始图片").FontSize(12).
					TextColor(theme.TextMuted).SingleLine()
			} else {
				ui.Textf(c, "%d 字符", len([]rune(item.content))).FontSize(12).
					TextColor(theme.TextMuted).SingleLine()
			}
			ui.Spacer(c)
			ui.Textf(c, "%s · %s", item.source, item.timestamp).
				FontSize(11).TextColor(theme.TextMuted).SingleLine()
			a.actionBar(c, theme, item)
		})
		a.autoTagRow(c, theme, item)
	})
}

func (a *application) previewContent(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	content := ui.Scroll(c).Grow(1).MinHeight(0).
		Padding(0, 28, 26, 28)
	content.Children(func() {
		ui.Column(c).MaxWidth(760).Gap(12).Children(func() {
			switch item.kind {
			case kindCode:
				ui.Text(c, item.content).Font("monospace").FontSize(13).
					TextColor(theme.Text.Alpha(0.92)).Padding(16).Radius(14).
					Background(theme.Surface.Alpha(0.62)).
					Border(1, theme.Border.Alpha(0.38)).Wrap().Selectable()
			case kindLink:
				ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(16).Radius(14).
					Background(theme.Surface.Alpha(0.62)).
					Border(1, theme.Border.Alpha(0.38)).Selectable().Children(func() {
					ui.Icon(c, icons.link).Size(16, 16).Shrink(0).
						TextColor(colorForKind(theme, item.kind))
					ui.Link(c, item.content, item.content).FontSize(14).
						TextColor(theme.Accent).Wrap()
				})
			case kindImage:
				if item.image == nil {
					ui.Text(c, "图片预览不可用").FontSize(14).
						TextColor(theme.TextMuted).Padding(16).Radius(14).
						Background(theme.Surface.Alpha(0.62)).
						Border(1, theme.Border.Alpha(0.38))
					return
				}
				ui.Box(c).MaxWidth(760).Center().Padding(12).Radius(16).
					Background(theme.Surface.Alpha(0.62)).
					Border(1, theme.Border.Alpha(0.38)).Children(func() {
					ui.Image(c, item.image.bitmap).MaxWidth(720).MaxHeight(520).
						Fit(ui.Contain).Radius(10)
				})
			default:
				ui.Text(c, item.content).FontSize(14).
					TextColor(theme.Text.Alpha(0.92)).Padding(16).Radius(14).
					Background(theme.Surface.Alpha(0.62)).
					Border(1, theme.Border.Alpha(0.38)).Wrap().Selectable()
			}
		})
	})
}

func (a *application) actionBar(c *ui.Context, theme *ui.Theme, item clipboardItem) {
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(2).Children(func() {
		copyLabel := "复制"
		copyIcon := icons.copy
		if a.statusMessage == copiedStatus {
			copyLabel = "已复制"
			copyIcon = icons.check
		}
		copyButton := actionButton(c, theme, "copy", copyIcon, copyLabel, false)
		if copyButton.Clicked() {
			a.copyToClipboard(item)
		}

		favoriteButton := actionButton(c, theme, "favorite", icons.star, favoriteLabel(item.favorite), item.favorite)
		if favoriteButton.Clicked() {
			a.toggleFavorite(item.id)
		}
	})
}

func (a *application) copyToClipboard(item clipboardItem) {
	if item.kind == kindImage && item.image != nil {
		if err := mygo.Clipboard.WriteImage(item.image.png); err != nil {
			return
		}
	} else {
		mygo.Clipboard.WriteText(item.content)
	}
	a.statusMessage = copiedStatus
}

func actionButton(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, active bool) ui.Element {
	button := ui.ButtonBase(c.Key("preview-action-"+key)).
		Height(28).Shrink(0).Padding(4, 10).Radius(8).
		TextColor(theme.TextMuted).Label(label)
	if active {
		button.TextColor(ui.Hex("#d69e2e"))
	}
	if button.Hovered() {
		button.Background(theme.Text.Alpha(0.07))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(14, 14).Shrink(0)
		ui.Text(c, label).FontSize(12).SingleLine()
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
