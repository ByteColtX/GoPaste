package main

import (
	"fmt"
	"strings"
	"time"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

// filterFacet 是一个筛选项：稳定的 key、图标、当前语言的标签、是否选中，
// 以及点击后如何更新筛选状态。筛选栏由若干这样的项按分区分组渲染。
type filterFacet struct {
	key      string
	icon     *ui.SVG
	label    string
	selected bool
	onSelect func()
}

// activeFilterLabel 返回当前生效的筛选条件名称，供空状态说明使用。它按
// “类型 → 标签 → 来源 → 时间 → 收藏”的顺序取第一个生效项。
func (a *application) activeFilterLabel() string {
	switch {
	case a.filterKind != kindFilterAny:
		return a.kindFilterLabel(a.filterKind)
	case a.filterTag != tagFilterAny:
		return a.tagFilterLabel(a.filterTag)
	case a.filterSource.IsLocal():
		return a.t(msgHistorySourceThisDevice)
	case a.filterSource.IsSet():
		if device, ok := a.pairedDeviceByID(a.filterSource.DeviceID()); ok {
			return device.name
		}
		return a.t(msgHistorySourceLanDevice)
	case a.filterPeriod != periodFilterAny:
		return a.periodFilterLabel(a.filterPeriod)
	case a.filterFavorite:
		return a.t(msgFilterFavorites)
	default:
		return a.t(msgFilterAll)
	}
}

func (a *application) kindFilterLabel(filter historyKindFilter) string {
	switch filter {
	case kindFilterText:
		return a.t(msgKindText)
	case kindFilterRichText:
		return a.t(msgKindRichText)
	case kindFilterImage:
		return a.t(msgKindImage)
	case kindFilterFile:
		return a.t(msgKindFile)
	default:
		return a.t(msgFilterAll)
	}
}

func (a *application) tagFilterLabel(filter historyTagFilter) string {
	switch filter {
	case tagFilterLink:
		return a.t(msgKindLink)
	case tagFilterCode:
		return a.t(msgKindCode)
	case tagFilterImage:
		return a.t(msgKindImage)
	default:
		return a.t(msgFilterAll)
	}
}

func (a *application) periodFilterLabel(filter historyPeriodFilter) string {
	switch filter {
	case periodFilterToday:
		return a.t(msgFilterTimeToday)
	case periodFilterYesterday:
		return a.t(msgFilterTimeYesterday)
	case periodFilterLast7Days:
		return a.t(msgFilterTimeLast7)
	case periodFilterLast30Days:
		return a.t(msgFilterTimeLast30)
	default:
		return a.t(msgFilterTimeAll)
	}
}

func (a *application) view(c *ui.Context) {
	theme := a.applyTheme(c)
	a.globalShortcuts(c)
	a.previewShortcuts(c)
	a.deviceShortcuts(c)

	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		ui.Row(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
			// 筛选栏是应用的常驻主轴：它既收窄历史列表，也是从设备/设置页
			// 回到历史页的入口（点任意筛选项都会切回历史页）。它收起时把
			// 宽度动画到 0，由标题栏的展开/收起按钮重新打开。
			a.filterRail(c, theme)
			// 内容区与筛选栏同步移动，展开/收起时不会瞬间跳到新位置。
			ui.Box(c).Grow(1).MinWidth(0).FillHeight().Transition(motionSlide).Children(func() {
				a.pageContent(c, theme)
			})
		})
		a.titleBarToggle(c, theme)
	})
	a.dangerousConfirmation(c, theme)
	a.flushPendingToast(c)
	a.flushPendingUndo(c)
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

// requestDeleteItem 按「删除前确认」偏好决定删除方式：偏好关闭就直接删除，
// 否则先弹二次确认框。按钮点击与 D 快捷键共用这条路径。
func (a *application) requestDeleteItem(c *ui.Context, id string) {
	if !a.confirmBeforeDelete {
		a.deleteHistoryItem(c, id)
		return
	}
	a.requestDangerousAction(dangerousActionDeleteItem, id)
}

// deviceShortcuts 为设备页的列表动作注册快捷键：Cmd+R 刷新、Cmd+N 新增设备。
// 只在设备页生效，在历史页按下时不会撞上无反馈的隐藏动作。
func (a *application) deviceShortcuts(c *ui.Context) {
	if a.page != pageDevices {
		return
	}
	c.OnShortcut(ui.Cmd, ui.KeyR, func() { a.refreshDevices(c) })
	c.OnShortcut(ui.Cmd, ui.KeyN, a.openAddDeviceDialog)
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

// flushPendingUndo 消费 macOS 菜单排队的一次撤销。菜单回调拿不到构建帧的
// ui.Context，无法直接关掉提示，所以先记一个标记，由下一帧在这里执行一次，
// 与 flushPendingToast 同一套做法。
func (a *application) flushPendingUndo(c *ui.Context) {
	if !a.undoRequested {
		return
	}
	a.undoRequested = false
	a.undoLast(c)
}

// chromeHeight 返回窗口标题栏（macOS 红绿灯、Windows/Linux 窗口按钮）占用的高度。
// 各内容列用它作为顶部留白，让面板表面一直延伸到窗口顶边。高度完全交给
// 平台：Windows/Linux 取系统原生标题栏高度，macOS 由红绿灯位置决定。
//
// 标题栏左侧常驻展开/收起按钮，因此这里给它留出最低高度：全屏时
// 平台报 0，面板的 paneTop 也不会缩到按钮下面。
func chromeHeight(c *ui.Context) float32 {
	height := c.TitleBar().Height
	if theme := c.Theme(); theme != nil {
		if needed := titleBarButtonHeight(c, theme) + space(theme, 1); needed > height {
			height = needed
		}
	}
	return height
}

// paneTop 是每个内容面板顶部的透明拖拽条：高度与标题栏一致，
// 让面板背景延伸到窗口顶边，同时提供整条的窗口拖动区。
func paneTop(c *ui.Context) {
	ui.Row(c).Height(chromeHeight(c)).FillWidth().Shrink(0).DragWindow()
}

// titleBarButtonHeight 是标题栏图标按钮的边长：用紧凑尺寸，刚好落在原生标题栏内。
func titleBarButtonHeight(c *ui.Context, theme *ui.Theme) float32 {
	return controlHeightSize(c, theme, 7)
}

// titleBarToggleLeft 是展开/收起按钮距窗口左边缘的位置：紧跟在窗口控制按钮之后，
// 中间留 12 DIP。这个距离取自 HIG › Accessibility › Mobility：「控件之间的间距与
// 尺寸同等重要……带边框的控件四周留约 12 点内边距，可以降低点错控件的概率」。
// 原来是 4 DIP，与红绿灯（尤其缩放按钮）挤在一起，容易误触。
func titleBarToggleLeft(c *ui.Context, theme *ui.Theme) float32 {
	return c.TitleBar().Left + space(theme, 3)
}

func (a *application) globalShortcuts(c *ui.Context) {
	c.OnShortcut(ui.Cmd, ui.Key1, func() { a.setPage(pageHistory) })
	c.OnShortcut(ui.Cmd, ui.Key2, func() { a.setPage(pageDevices) })
	c.OnShortcut(ui.Cmd, ui.Key3, func() { a.setPage(pageSettings) })
	// HIG › Settings 的标准快捷键。
	c.OnShortcut(ui.Cmd, ui.KeyComma, func() { a.setPage(pageSettings) })
	c.OnShortcut(ui.Cmd, ui.KeyK, func() { a.focusHistorySearch(c) })
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { a.focusHistorySearch(c) })
	// 界面缩放：Cmd +=、Cmd +-、Cmd 0。放大同时接受带 Shift 的等号键，
	// 因为多数键盘上「+」要按 Shift，浏览器也是这么绑的。
	c.OnShortcut(ui.Cmd, ui.KeyEqual, func() { a.zoomIn() })
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyEqual, func() { a.zoomIn() })
	c.OnShortcut(ui.Cmd, ui.KeyMinus, func() { a.zoomOut() })
	c.OnShortcut(ui.Cmd, ui.Key0, func() { a.resetZoom() })
	// 撤销：删除或取消配对后，提示还开着的几秒内 Cmd+Z 与点击提示里的「撤销」等价。
	// macOS 上 Cmd+Z 由编辑菜单的「撤销」项接管（见 runtime.installAppMenu），
	// 这里供其他平台和没有菜单的构建使用。
	c.OnShortcut(ui.Cmd, ui.KeyZ, func() { a.undoLast(c) })
	// 侧栏展开/收起：Cmd+Ctrl+S 是 macOS 的「显示/隐藏边栏」惯例，其他平台上
	// Cmd 与 Ctrl 同义，实际落到 Ctrl+S。
	c.OnShortcut(ui.Cmd|ui.Ctrl, ui.KeyS, func() { a.filterRailOpen = !a.filterRailOpen })
}

// focusHistorySearch 在窗口级别响应搜索快捷键，使焦点位于预览区或其他区域时也能聚焦搜索框。
// 搜索框在筛选栏顶部：栏已收起时先展开，从其他页面触发时先回到历史页。
func (a *application) focusHistorySearch(c *ui.Context) {
	changed := !a.filterRailOpen || a.page != pageHistory
	a.filterRailOpen = true
	a.setPage(pageHistory)
	if changed {
		c.Invalidate()
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
	//
	// 按键：D 与 Y 确认、N 取消；键帽只画 D 与 Esc，其余键照常生效但不显示，
	// 免得按钮上挤满键帽。
	dontAskLabel := ""
	keys := dangerDialogKeys{cancel: []ui.Key{ui.KeyN}}
	if action == dangerousActionDeleteItem {
		dontAskLabel = a.t(msgDangerDontAskAgain)
		keys.confirm = []ui.Key{ui.KeyD, ui.KeyY}
		keys.confirmShown = ui.KeyD
	}

	if dangerConfirmDialog(c, theme, a.l10n(), &a.confirmDangerousAction, title, message, confirmLabel, dontAskLabel, &a.dangerDontAskAgain, keys) {
		switch action {
		case dangerousActionDeleteItem:
			if a.dangerDontAskAgain {
				a.confirmBeforeDelete = false
			}
			a.deleteHistoryItem(c, a.pendingDangerousTargetID)
		case dangerousActionUnpairDevice:
			a.unpairDeviceWithUndo(c, a.pendingDangerousTargetID)
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

// filterRail 是窗口左侧的侧栏，也是应用唯一的侧栏。它的内容随当前分区切换：
// 历史页是筛选分组，设备页是设备列表，设置页是设置分栏。顶部是分区标题（历史页
// 另有搜索框），底部是全局导航（设置 / 外观切换 / 设备）。它整列延伸到窗口顶边，
// 顶部保留标题栏拖拽条，因此也承担拖动窗口的职责。收起时把宽度动画到 0 并裁剪
// 内容，由标题栏的展开/收起按钮重新打开。
func (a *application) filterRail(c *ui.Context, theme *ui.Theme) {
	// 收起不再让整列消失，而是动画宽度：弹簧从当前宽度继续走，中途再点一次
	// 会平滑地折回，而不是硬停。ClipX 把内容裁在收缩的列里。
	//
	// 这一列不留右内边距，滚动条才能贴到右侧分割线；搜索框、筛选项和底栏
	// 各自留出右留白。
	rail := ui.Column(c.Key("filter-rail")).
		Width(a.filterRailSize(c, theme)).FillHeight().Shrink(0).ClipX().
		Background(sidebarFill(theme)).Border(1, dividerColor(theme)).
		Transition(motionLayout)
	rail.Children(func() {
		// 内容保持固定宽度，由外层裁掉：收起读起来是「滑走」而不是「挤扁」。
		// 宽度取外层内容盒的宽度，滚动条因此仍贴在外层右缘。
		ui.Column(c.Key("filter-rail-content")).
			Width(filterRailContentWidth(theme)).FillHeight().Shrink(0).
			// 左右都用同一个留白，选中项的高亮框才在侧栏里居中；右留白同时要让开
			// 滚动条（滚动条占右侧 3–9 DIP），所以不能为了居中把它调小。滚动容器
			// 本身仍占满这一列（右内边距为 0），滚动条因此依旧贴近分割线。
			Padding(0, 0, space(theme, 2), spaceDIP(theme, paneSideInset)).Children(func() {
			paneTop(c)
			// 标题栏带下方统一留一段上边距：侧栏的第一个元素——历史页的搜索框、
			// 设备页的本机条目、设置页的第一个分栏——因此在每一页都落在同一条
			// 基线上，与列表、预览、详情列的首行对齐。
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).
				Padding(space(theme, columnTopInset), 0, 0, 0).Children(func() {
				// 搜索属于历史筛选，只在历史分区出现。
				if a.page == pageHistory {
					ui.Box(c).FillWidth().Shrink(0).
						Padding(0, spaceDIP(theme, paneSideInset), space(theme, 1.5), 0).Children(func() {
						a.searchField(c, theme)
					})
				}
				a.sidebarBody(c, theme)
				a.sidebarFooter(c, theme)
				a.sidebarNavigation(c, theme)
			})
		})
	})
}

// sidebarNavigation 是侧栏最下方的全局导航，也就是主窗口的分区切换器：历史、
// 设备、设置依次排列，外观切换在行尾。按钮顺序与 Cmd+1/2/3 的页面顺序一致，
// 位置和快捷键读起来是同一件事。它不属于任何分区，切换分区时保持不动。
//
// 侧栏随分区切换内容，设备页与设置页不再显示筛选项，所以切换器必须自己带上
// 「历史」入口，否则从这两页回到历史只剩快捷键。
//
// 已知取舍：HIG › Sidebars 建议不要把关键操作放在侧栏底部（窗口下缘可能被移出
// 屏幕）。这里按产品决定仍放在底部。
func (a *application) sidebarNavigation(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).FillWidth().Shrink(0).Padding(space(theme, 1.5), spaceDIP(theme, paneSideInset), 0, 0).Children(func() {
		standardDivider(c, theme)
	})
	// 底栏：左侧图标组左对齐，外观按钮单独右对齐。第 5 个图标位先留空占位，落地时
	// 位置和间距都不用再动。这里不能用 Spacer 顶开外观按钮——Spacer 自己也是一个子
	// 元素，会在两侧各吃一个 gap，6 个子元素共 5 个 gap 会把这一行撑溢出 7 DIP（外观
	// 按钮的右留白从 14 掉到 7）。改成 5 个子元素 + SpaceBetween 均分：侧栏宽度按
	// 「4 个图标位 + 外观 + 4 个间隙正好铺满」选定（见 filterRailWidth），间隙正好是
	// navButtonGap，左边仍贴左留白、外观仍贴右留白。
	ui.Row(c).FillWidth().Shrink(0).Padding(space(theme, 1.5), spaceDIP(theme, paneSideInset), 0, 0).
		Gap(spaceDIP(theme, navButtonGap)).Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
		a.filterRailNavButton(c, theme, "sidebar-history", pageHistory, icons.history, a.t(msgNavHistory))
		a.filterRailNavButton(c, theme, "sidebar-devices", pageDevices, icons.devices, a.t(msgNavDevices))
		a.filterRailNavButton(c, theme, "sidebar-settings", pageSettings, icons.settings, a.t(msgNavSettings))
		// 第 5 个图标位暂时留空：占住一个按钮位，落地时布局不会跳动。
		size := controlHeightSize(c, theme, compactHeight)
		ui.Box(c.Key("sidebar-reserved-slot")).Size(size, size).Shrink(0)
		a.filterRailAppearanceButton(c, theme)
	})
}

// sidebarBody 是侧栏的滚动区，按当前分区渲染历史筛选、设备列表或设置分栏。
// 每个分区用独立的 key，滚动位置不会跨分区串台。
func (a *application) sidebarBody(c *ui.Context, theme *ui.Theme) {
	ui.Scroll(c.Key(a.page)).Grow(1).MinHeight(0).FillWidth().
		ScrollbarInsets(0, 0, 0, 0).Children(func() {
		ui.Column(c).FillWidth().Gap(space(theme, 0.5)).
			Padding(0, spaceDIP(theme, paneSideInset), 0, 0).Children(func() {
			switch a.page {
			case pageDevices:
				a.sidebarDevices(c, theme)
			case pageSettings:
				a.sidebarSettings(c, theme)
			default:
				a.sidebarHistoryFilters(c, theme)
			}
		})
	})
}

// sidebarHistoryFilters 是历史分区的筛选分组。
func (a *application) sidebarHistoryFilters(c *ui.Context, theme *ui.Theme) {
	// 快捷：所有（清空所有筛选）与收藏。
	//
	// 「所有」用列表图标而不是底栏「历史」分区用的时钟图标：两者在历史页会同时
	// 出现在同一列里，同一枚字形担两个意思读起来像重复。分区图标要准确指向
	// 「历史」，而这里表达的是「不过滤、全部条目」。
	a.filterItemButton(c, theme, filterFacet{
		key:      "filter-all",
		icon:     icons.list,
		label:    a.t(msgFilterAll),
		selected: !a.historyFiltered(),
		onSelect: a.resetHistoryFilters,
	})
	a.filterItemButton(c, theme, filterFacet{
		key:      "filter-favorites",
		icon:     icons.star,
		label:    a.t(msgFilterFavorites),
		selected: a.filterFavorite,
		onSelect: func() { a.filterFavorite = !a.filterFavorite },
	})

	a.filterSection(c, theme, a.t(msgFilterSectionType), func() {
		a.filterItemButton(c, theme, a.kindFacet("filter-kind-text", icons.text, msgKindText, kindFilterText))
		a.filterItemButton(c, theme, a.kindFacet("filter-kind-rich-text", icons.richText, msgKindRichText, kindFilterRichText))
		a.filterItemButton(c, theme, a.kindFacet("filter-kind-image", icons.image, msgKindImage, kindFilterImage))
		a.filterItemButton(c, theme, a.kindFacet("filter-kind-file", icons.file, msgKindFile, kindFilterFile))
	})

	a.filterSection(c, theme, a.t(msgFilterSectionTag), func() {
		a.filterItemButton(c, theme, a.tagFacet("filter-tag-link", icons.link, msgKindLink, tagFilterLink))
		a.filterItemButton(c, theme, a.tagFacet("filter-tag-code", icons.code, msgKindCode, tagFilterCode))
	})

	a.filterSection(c, theme, a.t(msgFilterSectionSource), func() {
		a.filterItemButton(c, theme, filterFacet{
			key:      "filter-source-all",
			icon:     icons.lan,
			label:    a.t(msgFilterSourceAll),
			selected: !a.filterSource.IsSet(),
			onSelect: func() { a.filterSource = history.SourceFilter{} },
		})
		a.filterItemButton(c, theme, a.localSourceFacet())
		for index := range a.pairedDevices {
			a.filterSourceItem(c, theme, index)
		}
	})

	a.filterSection(c, theme, a.t(msgFilterSectionTime), func() {
		a.filterItemButton(c, theme, a.periodFacet("filter-time-all", msgFilterTimeAll, periodFilterAny))
		a.filterItemButton(c, theme, a.periodFacet("filter-time-today", msgFilterTimeToday, periodFilterToday))
		a.filterItemButton(c, theme, a.periodFacet("filter-time-yesterday", msgFilterTimeYesterday, periodFilterYesterday))
		a.filterItemButton(c, theme, a.periodFacet("filter-time-last7", msgFilterTimeLast7, periodFilterLast7Days))
		a.filterItemButton(c, theme, a.periodFacet("filter-time-last30", msgFilterTimeLast30, periodFilterLast30Days))
	})
}

// sidebarDevices 是设备分区的列表：本机条目、已配对设备（刷新在分组标题行）与
// 发现状态。新增设备不在这一列里，见 sidebarFooter。
func (a *application) sidebarDevices(c *ui.Context, theme *ui.Theme) {
	a.localDeviceEntry(c, theme)

	// 标题行的高度就是刷新按钮的高度，两者相等。上方的 12 DIP 留白必须写成
	// Margin 而不是 Padding：Padding 会从固定高度里扣掉它，内容盒只剩 24 DIP，
	// 36 DIP 的按钮居中后会上下各溢出 6 DIP，压住下面第一台设备的高亮。
	ui.Row(c).Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).AlignItems(ui.Center).
		Margin(space(theme, 3), 0, 0, 0).Children(func() {
		ui.Text(c, a.t(msgDevicesPaired)).FontSize(font(theme, fontSubheading)).FontWeight(600).
			TextColor(secondaryText(theme)).SingleLine()
		ui.Spacer(c)
		a.sidebarIconAction(c, theme, "refresh-paired-devices", icons.refresh, a.t(msgDevicesRefresh), a.deviceRefreshSpinning(), func() {
			a.refreshDevices(c)
		})
	})

	switch a.deviceDiscovery {
	case deviceDiscoveryLoading:
		a.deviceLoadingState(c, theme)
	case deviceDiscoveryError:
		a.deviceErrorState(c, theme)
	default:
		if len(a.pairedDevices) == 0 {
			a.deviceEmptyState(c, theme)
		}
		// 设备条目之间的间距由这一列的列间距提供，与历史列表的 4 DIP 节奏一致，
		// 且第一行不会被行自身的 margin 推低。
		ui.Column(c).FillWidth().Gap(space(theme, listRowGap)).Children(func() {
			for index := range a.pairedDevices {
				a.pairedDeviceEntry(c, theme, index)
			}
		})
	}
}

// sidebarSettings 是设置分区的分栏入口。
func (a *application) sidebarSettings(c *ui.Context, theme *ui.Theme) {
	for _, item := range a.settingsNavigationItems() {
		a.settingsNavButton(c, theme, item)
	}
}

// sidebarFooter 是侧栏在滚动区下方的分区专属底栏。目前只有设备分区有：左侧是
// 在线台数，右侧是「新增设备」宽按钮。两者都必须常驻——在线数是列表的汇总，
// 随列表一起滚走就失去意义；新增是这一页的主操作，长列表下不能滚出视野。
//
// 刷新留在列表分组标题行：它作用于列表本身，和列表挨着读才顺。
//
// 已知取舍：HIG › Sidebars 反对把关键操作放在侧栏底部（窗口被挪到屏幕下缘之外
// 时可能被遮住）。这里与分区切换器同一决定，按产品要求仍放在底部。
func (a *application) sidebarFooter(c *ui.Context, theme *ui.Theme) {
	if a.page != pageDevices {
		return
	}
	onlineCount := a.onlineDeviceCount()
	onlineColor := secondaryText(theme)
	if onlineCount > 0 {
		onlineColor = theme.Success
	}
	ui.Row(c).FillWidth().Shrink(0).
		Padding(space(theme, 1.5), spaceDIP(theme, paneSideInset), space(theme, 1.5), 0).
		Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
		deviceStatus(c, theme, a.l10n().onlineCount(onlineCount), onlineColor)
		a.sidebarWideButton(c, theme, "add-paired-device", icons.deviceAdd, a.t(msgDevicesAdd), a.openAddDeviceDialog)
	})
}

// kindFacet 构造一个类型筛选项：再次点击已选中的项会取消该筛选。
func (a *application) kindFacet(key string, icon *ui.SVG, label messageKey, filter historyKindFilter) filterFacet {
	return filterFacet{
		key:      key,
		icon:     icon,
		label:    a.t(label),
		selected: a.filterKind == filter,
		onSelect: func() {
			if a.filterKind == filter {
				a.filterKind = kindFilterAny
				return
			}
			a.filterKind = filter
		},
	}
}

// tagFacet 构造一个标签筛选项：再次点击已选中的项会取消该筛选。
func (a *application) tagFacet(key string, icon *ui.SVG, label messageKey, filter historyTagFilter) filterFacet {
	return filterFacet{
		key:      key,
		icon:     icon,
		label:    a.t(label),
		selected: a.filterTag == filter,
		onSelect: func() {
			if a.filterTag == filter {
				a.filterTag = tagFilterAny
				return
			}
			a.filterTag = filter
		},
	}
}

// periodFacet 构造一个时间筛选项。
func (a *application) periodFacet(key string, label messageKey, filter historyPeriodFilter) filterFacet {
	return filterFacet{
		key:      key,
		icon:     icons.clock,
		label:    a.t(label),
		selected: a.filterPeriod == filter,
		onSelect: func() { a.filterPeriod = filter },
	}
}

// localSourceFacet 构造「本机」来源筛选项。本机捕获的条目没有来源设备 ID，
// 所以这一项按“来源为空”过滤，名称与预览区的本机来源一致。
func (a *application) localSourceFacet() filterFacet {
	return filterFacet{
		key:      "filter-source-local",
		icon:     iconForPlatform(a.devicePlatform),
		label:    a.t(msgHistorySourceThisDevice),
		selected: a.filterSource.IsLocal(),
		onSelect: func() {
			if a.filterSource.IsLocal() {
				a.filterSource = history.SourceFilter{}
				return
			}
			a.filterSource = history.LocalSource()
		},
	}
}

// filterSourceItem 是一台已配对设备的来源筛选项。它按设备 ID 过滤，
// 因此设备改名后筛选仍然有效。
func (a *application) filterSourceItem(c *ui.Context, theme *ui.Theme, index int) {
	device := a.pairedDevices[index]
	source := history.DeviceSource(device.id)
	a.filterItemButton(c, theme, filterFacet{
		key:      "filter-source-device-" + device.id,
		icon:     iconForPlatform(device.platform),
		label:    device.name,
		selected: a.filterSource == source,
		onSelect: func() {
			if a.filterSource == source {
				a.filterSource = history.SourceFilter{}
				return
			}
			a.filterSource = source
		},
	})
}

// filterSection 渲染筛选栏的一个分区：小号分区标题加分区内的筛选项。
//
// 间距按 Apple 的 8 pt 基线网格（macOS 上标准间距都是 4 pt 的整数倍）：标题
// 上方 16 pt 把它与上一组拉开，标题下方 8 pt 是「标签到控件」的标准间距
// （spacing.m）。下方小于上方，标题才读得出属于它标注的那一组。标题左内边距
// 与筛选项一致，两者的左边缘对齐。
func (a *application) filterSection(c *ui.Context, theme *ui.Theme, title string, body func()) {
	ui.Column(c).FillWidth().Shrink(0).
		Padding(space(theme, 4), 0, 0, 0).Children(func() {
		ui.Text(c, title).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(secondaryText(theme)).
			Padding(0, space(theme, 2), space(theme, 2), space(theme, 2)).SingleLine()
		ui.Column(c).FillWidth().Gap(space(theme, 0.5)).Children(body)
	})
}

// filterItemButton 渲染筛选栏中的一行：单行条目的规格由 sidebarItemButton 统一
// 提供，与设置分栏逐项一致（高度、内边距、图标、间距、选中态）。
func (a *application) filterItemButton(c *ui.Context, theme *ui.Theme, facet filterFacet) {
	sidebarItemButton(c, theme, facet.key, facet.icon, facet.label, facet.selected, func() {
		facet.onSelect()
		// 筛选项只属于历史分区；这里再确认一次分区，让「点筛选即回到历史」
		// 这个约定不依赖调用方的渲染条件。
		a.setPage(pageHistory)
	})
}

// filterRailNavButton 是筛选栏底部的导航按钮：只有图标，文字放在提示与辅助
// 技术标签里。按钮顺序与 Cmd+1/2/3 一致：历史、设备、设置。历史记录是默认页面，由筛选栏本身直接呈现，
// 因此不再需要单独的历史入口。
func (a *application) filterRailNavButton(c *ui.Context, theme *ui.Theme, key string, target page, icon *ui.SVG, label string) {
	selected := a.page == target
	size := controlHeightSize(c, theme, compactHeight)
	button := ui.ButtonBase(c.Key(key)).
		Size(size, size).Shrink(0).Center().
		Radius(radius(theme, controlRadius)).Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).Label(label).Tooltip(label).
		Role(ui.RoleToggleButton).Selected(selected).Transition(motionColors)
	switch {
	case selected:
		button.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme))
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
		ui.Icon(c, icon).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0).TextColor(iconColor)
	})
	if button.Clicked() {
		a.setPage(target)
	}
}

// filterRailAppearanceButton 是侧栏底部的外观入口：点击在按钮上方弹出气泡，
// 气泡里列出跟随系统 / 浅色 / 深色三个模式。按钮图标与气泡里的勾选都表示当前
// 模式——它现在是一个选择器，不再按「点一下会变成什么」循环。设置页的「模式」
// 分段控件是同一批模式的另一个入口，两处读写同一个字段。
func (a *application) filterRailAppearanceButton(c *ui.Context, theme *ui.Theme) {
	size := controlHeightSize(c, theme, compactHeight)
	label := a.t(msgSettingsAppearance)
	tooltip := a.tf(msgSettingsCurrentAppearance, a.appearanceModeLabel(a.appearance))
	button := ui.ButtonBase(c.Key("filter-rail-appearance")).
		Size(size, size).Shrink(0).Center().
		Radius(radius(theme, controlRadius)).Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).Label(label).Tooltip(tooltip).
		Role(ui.RolePopUpButton).Expanded(a.appearancePopoverOpen).
		Value(a.appearanceModeLabel(a.appearance)).Transition(motionColors)
	switch {
	case a.appearancePopoverOpen:
		button.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme))
	case button.Hovered():
		button.Background(hoverFill(theme))
	default:
		button.Background(ui.Transparent)
	}
	iconColor := secondaryText(theme)
	if a.appearancePopoverOpen {
		iconColor = accentColor(theme)
	}
	button.Children(func() {
		ui.Icon(c, appearanceIcon(a.appearance)).Size(space(theme, 4.5), space(theme, 4.5)).
			Shrink(0).TextColor(iconColor)
	})
	if button.Clicked() {
		a.appearancePopoverOpen = !a.appearancePopoverOpen
	}
	// 按钮贴着窗口下缘，气泡没有向下的空间，PopoverBase 会自动翻到按钮上方；
	// 右对齐到按钮右缘，气泡因此盖在侧栏上，而不是伸进右侧的内容区。
	ui.PopoverBase(c, button, &a.appearancePopoverOpen, func(panel ui.Element) {
		panel.AttachTo(button, ui.AnchorBottomEnd, ui.AnchorTopEnd).
			MinWidth(space(theme, 46)).Padding(space(theme, 1)).
			Radius(radius(theme, cardRadius)).
			Background(theme.Background).Border(1, borderColor(theme)).
			Shadow(0, space(theme, 1.5), space(theme, 5), 0, ui.RGBA(0, 0, 0, 0.18)).
			Margin(0, 0, space(theme, 1), 0).
			Transition(ui.ElementTransition{
				Duration: 140 * time.Millisecond,
				Ease:     ui.EaseOut,
				Enter:    &ui.Motion{Opacity: 0, Scale: 0.98, Y: 4},
			})
		a.appearancePopoverContent(c, theme)
	})
}

// appearancePopoverContent 是气泡里的模式列表：一个标题加三个单选项，当前
// 模式打勾。顺序与设置页「模式」分段控件一致。
func (a *application) appearancePopoverContent(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Gap(space(theme, 0.25)).Children(func() {
		ui.Text(c, a.t(msgSettingsAppearance)).FontSize(font(theme, fontMeta)).FontWeight(600).
			TextColor(secondaryText(theme)).Padding(space(theme, 0.5), space(theme, 1.5)).
			Role(ui.RoleHeading).SingleLine()
		for _, mode := range appearanceModes {
			a.appearancePopoverItem(c, theme, mode)
		}
	})
}

// appearancePopoverItem 是气泡里的一个模式：图标、名称，以及当前模式的勾选。
// 点选后立即应用并收起气泡。
func (a *application) appearancePopoverItem(c *ui.Context, theme *ui.Theme, mode int) {
	selected := a.appearance == mode
	label := a.appearanceModeLabel(mode)
	item := ui.ButtonBase(c.Key("filter-rail-appearance-"+appearanceModeName(mode))).
		FillWidth().Shrink(0).Height(controlHeightSize(c, theme, compactHeight)).
		Padding(0, space(theme, 1.5)).Gap(space(theme, 1.5)).
		Radius(radius(theme, controlRadius)).Label(label).
		Role(ui.RoleMenuItemRadio).Checked(selected).FocusRing(true)
	switch {
	case item.Pressed():
		item.Background(theme.SurfacePressed)
	case item.Hovered():
		item.Background(hoverFill(theme))
	default:
		item.Background(ui.Transparent)
	}
	iconColor := secondaryText(theme)
	if selected {
		iconColor = accentColor(theme)
	}
	item.Children(func() {
		ui.Icon(c, appearanceIcon(mode)).Size(space(theme, 4), space(theme, 4)).
			Shrink(0).TextColor(iconColor)
		ui.Text(c, label).Grow(1).MinWidth(0).FontSize(font(theme, fontControl)).
			TextColor(primaryText(theme)).SingleLine()
		// 勾选标记只出现在当前模式上，位置固定在最右侧；其余行不占位，
		// 文字一律左对齐，列表不会随选择左右跳动。
		if selected {
			ui.Icon(c, icons.check).Size(space(theme, 3.5), space(theme, 3.5)).
				Shrink(0).TextColor(accentColor(theme))
		}
	})
	if item.Clicked() {
		a.setAppearance(mode)
		a.appearancePopoverOpen = false
	}
}

// appearanceIcon 返回某个外观模式对应的图标。
func appearanceIcon(mode int) *ui.SVG {
	switch mode {
	case appearanceLight:
		return icons.sun
	case appearanceDark:
		return icons.moon
	default:
		return icons.auto
	}
}

// appearanceModeName 返回外观模式的稳定英文名，用作控件 key 的一部分。
func appearanceModeName(mode int) string {
	switch mode {
	case appearanceLight:
		return "light"
	case appearanceDark:
		return "dark"
	default:
		return "system"
	}
}

// appearanceModeLabel 返回某个外观模式在列表里的短名称：跟随系统 / 浅色 / 深色。
func (a *application) appearanceModeLabel(mode int) string {
	switch mode {
	case appearanceLight:
		return a.t(msgSettingsThemeLight)
	case appearanceDark:
		return a.t(msgSettingsThemeDark)
	default:
		return a.t(msgSettingsThemeSystem)
	}
}

// titleBarToggle 把筛选栏的展开/收起按钮放在标题栏左侧、窗口控制按钮的右边。
// 它跟随平台报出的控制按钮宽度，因此 macOS 的红绿灯、Windows/Linux 的窗口
// 按钮都不会遮住它。
func (a *application) titleBarToggle(c *ui.Context, theme *ui.Theme) {
	buttonHeight := titleBarButtonHeight(c, theme)
	top := (chromeHeight(c) - buttonHeight) / 2
	if top < 0 {
		top = 0
	}
	ui.Row(c).Absolute().Top(top).Left(titleBarToggleLeft(c, theme)).Children(func() {
		label := a.t(msgFilterRailCollapse)
		icon := icons.railCollapse
		if !a.filterRailOpen {
			label = a.t(msgFilterRailExpand)
			icon = icons.railExpand
		}
		button := ui.ButtonBase(c.Key("filter-rail-toggle")).
			Size(buttonHeight, buttonHeight).Shrink(0).Center().
			Radius(radius(theme, controlRadius)).TextColor(secondaryText(theme)).
			Label(label).Tooltip(label).Role(ui.RoleButton).Transition(motionColors)
		if button.Hovered() {
			button.Background(hoverFill(theme))
		} else {
			button.Background(ui.Transparent)
		}
		button.Children(func() {
			ui.Icon(c, icon).Size(space(theme, 4.25), space(theme, 4.25)).TextColor(secondaryText(theme))
		})
		if button.Clicked() {
			a.filterRailOpen = !a.filterRailOpen
		}
	})
}

// pageContent 渲染当前页面。容器按页面取 key，切页时旧页面淡出、新页面淡入，
// 内容不会硬切。
func (a *application) pageContent(c *ui.Context, theme *ui.Theme) {
	ui.Box(c.Key(a.page)).Fill().Transition(motionPage).Children(func() {
		switch a.page {
		case pageDevices:
			a.devicesPage(c, theme)
		case pageSettings:
			a.settingsPage(c, theme)
		default:
			a.historyPage(c, theme)
		}
	})
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
	maxWidth := windowWidth - a.filterRailSize(c, theme) - paneMinWidth(theme, minPreviewPaneWidth) - splitDividerWidth
	maxWidth = max(maxWidth, paneMinWidth(theme, minHistoryPaneWidth))
	a.historyPaneWidth = min(max(a.historyPaneWidth, paneMinWidth(theme, minHistoryPaneWidth)), maxWidth)
}

func (a *application) historyList(c *ui.Context, theme *ui.Theme, view history.View) {
	panel := ui.Column(c).Fill().
		MinWidth(paneMinWidth(theme, minHistoryPaneWidth)).
		Background(theme.Background).
		Padding(0, 0, space(theme, 2.5), spaceDIP(theme, paneSideInset))

	panel.Children(func() {
		// 顶部保留标题栏带，列表内容从带子下方统一的第一行开始。
		paneTop(c)
		a.historyHeader(c, theme, view)

		if len(view.Items) == 0 {
			a.emptyHistory(c, theme, view)
			return
		}

		a.historyRows(c, theme, view)
	})
}

func (a *application) historyHeader(c *ui.Context, theme *ui.Theme, view history.View) {
	countLabel := a.l10n().recordCount(len(view.Items), view.Total, a.query != "" || a.historyFiltered())
	// 标题在标题栏带下方起一行，与侧栏、预览、详情列共用同一条首行基线，
	// 因此侧栏收起时不需要任何横向或纵向位移。
	ui.Row(c.Key("history-header")).Shrink(0).FillWidth().AlignItems(ui.Start).DragWindow().
		Padding(space(theme, columnTopInset), spaceDIP(theme, paneSideInset), space(theme, 1.5), 0).
		Children(func() {
			ui.Column(c).Gap(space(theme, 0.25)).Children(func() {
				ui.Text(c, a.t(msgHistoryRecent)).FontSize(font(theme, fontPaneTitle)).FontWeight(600).SingleLine()
				ui.Text(c, countLabel).FontSize(font(theme, fontCaption)).
					TextColor(secondaryText(theme)).SingleLine()
			})
		})
}

// searchField 渲染筛选栏顶部的搜索框。它仍然是历史搜索：在设备/设置页输入时
// 会先回到历史页，让结果立刻可见。
func (a *application) searchField(c *ui.Context, theme *ui.Theme) {
	focused := c.Focused(a.historySearch)
	field := ui.Row(c.Key("history-search")).Height(controlHeightSize(c, theme, compactHeight)).FillWidth().Shrink(0).
		AlignItems(ui.Center).Gap(space(theme, 1.5)).Padding(0, space(theme, 2.5)).
		Radius(radius(theme, controlRadius)).Background(theme.Background).
		Border(1, borderColor(theme)).Transition(motionColors).
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
		if input.Changed() && a.page != pageHistory {
			a.setPage(pageHistory)
			c.Invalidate()
		}

		if a.query == "" {
			if !focused {
				searchShortcutHint(c, theme)
			}
			return
		}
		clearButton := ui.ButtonBase(c.Key("clear-history-search")).
			Size(space(theme, 5), space(theme, 5)).Shrink(0).Radius(radius(theme, controlRadius)).Center().
			TextColor(secondaryText(theme)).Role(ui.RoleButton).Transition(motionColors).
			Label(a.t(msgHistoryClearSearch)).Tooltip(a.t(msgHistoryClearSearch))
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
// 快捷键本身在 globalShortcuts 中注册（Cmd+K / Cmd+F）。
func searchShortcutHint(c *ui.Context, theme *ui.Theme) {
	ui.Text(c, searchShortcutLabel()).FontSize(font(theme, fontCaption)).FontWeight(500).
		TextColor(secondaryText(theme)).Padding(space(theme, 0.5), space(theme, 1)).
		Radius(radius(theme, controlRadius)).Background(subtleFill(theme)).SingleLine()
}

func (a *application) emptyHistory(c *ui.Context, theme *ui.Theme, view history.View) {
	firstUse := view.Total == 0 && a.query == "" && !a.historyFiltered()
	searchEmpty := a.query != ""
	filterEmpty := a.historyFiltered()
	title := a.t(msgHistoryEmptyTitle)
	message := a.t(msgHistoryEmptyMessage)
	icon := icons.filter
	switch {
	case firstUse:
		title = a.t(msgHistoryEmptyFirstTitle)
		message = a.t(msgHistoryEmptyFirstMessage)
		icon = icons.history
	case searchEmpty && filterEmpty:
		title = a.t(msgHistoryEmptySearchTitle)
		message = a.tf(msgHistoryEmptyFilterMessage, a.activeFilterLabel())
		icon = icons.search
	case searchEmpty:
		title = a.t(msgHistoryEmptySearchTitle)
		message = a.tf(msgHistoryEmptySearchMessage, truncateSingleLine(a.query, 24))
		icon = icons.search
	case filterEmpty:
		title = a.t(msgHistoryEmptyFilterTitle)
		message = a.tf(msgHistoryEmptyFilterMessage, a.activeFilterLabel())
		icon = icons.filter
	}
	ui.Box(c).Grow(1).Center().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(320).Shrink(0).Center().Gap(space(theme, 2)).
			Transition(motionAppear).Children(func() {
			ui.Box(c).Size(space(theme, 12), space(theme, 12)).Shrink(0).Radius(radius(theme, cardRadius)).
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
			if searchEmpty && !filterEmpty {
				resetLabel = a.t(msgHistoryResetSearch)
			} else if !searchEmpty && filterEmpty {
				resetLabel = a.t(msgHistoryShowAll)
			}
			reset := neutralButton(c, theme, "history-empty-reset", resetLabel)
			reset.Children(func() {
				ui.Text(c, resetLabel).FontSize(font(theme, fontControl)).SingleLine()
			})
			if reset.Clicked() {
				a.query = ""
				a.resetHistoryFilters()
			}
		})
	})
}

// historyQuery 构造当前历史视图的查询，并带上本地化的类型名称，
// 让搜索在任意界面语言下都能匹配内容类型。
func (a *application) historyQuery() history.Query {
	return history.Query{
		Text:         a.query,
		Kind:         a.filterKind,
		Tag:          a.filterTag,
		FavoriteOnly: a.filterFavorite,
		Source:       a.filterSource,
		Period:       a.filterPeriod,
		KindTerms:    a.kindSearchTerms,
		SourceName:   a.sourceDeviceName,
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

// historyRow 渲染一条历史记录：双行行的规格由 listRow 统一提供，与设备条目逐项
// 一致（行高、内边距、图标块、行距、选中态）。
func (a *application) historyRow(c *ui.Context, theme *ui.Theme, item history.Item, selected bool) ui.Element {
	// 历史列表是虚拟列表，没有列间距可用，因此由行自己提供 4 DIP 间距与右侧留白。
	row := listRow(c, theme, "history-row-"+item.ID, a.historyListText(item), selected,
		func() { a.historyRowTile(c, theme, item) },
		func() {
			a.historyRowTitle(c, theme, item)
			a.historyRowMeta(c, theme, item)
		})
	row.Margin(space(theme, listRowGap/2), spaceDIP(theme, paneSideInset), space(theme, listRowGap/2), 0)
	return row
}

// historyRowTile 渲染历史记录行左侧的图标块：外框尺寸取自 listRowTile，与设备
// 条目的图标块同一大小。
func (a *application) historyRowTile(c *ui.Context, theme *ui.Theme, item history.Item) {
	if item.Kind == kindImage && item.Image != nil {
		tile := listRowTile(c, theme, subtleFill(theme), func() {
			if bitmap := a.historyBitmap(item); bitmap != nil {
				size := space(theme, listRowTileSize)
				ui.Image(c, bitmap).Size(size-2, size-2).Fit(ui.Cover).Radius(radius(theme, controlRadius) - 1)
			}
		})
		tile.Border(1, borderColor(theme))
		return
	}
	// 列表行的图标按内容子类型着色（链接、代码各自不同），
	// 让一屏记录只靠图标就能区分，而不必逐行读文字。
	kindColor := colorForKind(theme, item.Kind)
	listRowTile(c, theme, kindColor.Alpha(0.14), func() {
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
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).Children(func() {
		ui.Text(c, a.kindLabel(item.Kind)).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(kindColor).SingleLine()
		// 代码条目在类型后接上识别出的语言（代码 · Go），让列表不读图标也能分辨语言；
		// 识别不出时只留类型。
		if language := a.codeLanguage(item); language != "" {
			ui.Text(c, "·").FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
			ui.Text(c, language).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).SingleLine()
		}
		ui.Spacer(c)
		ui.Text(c, a.timestampLabel(item.CreatedAt)).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).SingleLine()
	})
}

// 预览元信息行里各子部件的尺寸：多数是 space 的 4 DIP 格，描边是 DIP。绘制与
// 测量共用这一组常量：元信息行要先算宽度才能决定显示多少信息，两处各写一套
// 数字迟早会对不上。
const (
	metaKindIconSize float32 = 3.75 // 类型图标
	metaRowGap       float32 = 1.5  // 子项之间的间距
	metaTagPadX      float32 = 1.5  // 自动标签左右内边距
	metaTagPadY      float32 = 0.25 // 自动标签上下内边距
	metaTagBorder    float32 = 1    // 自动标签描边，单位 DIP
	metaSourceIcon   float32 = 3.25 // 来源图标
	metaSourceGap    float32 = 1.25 // 来源图标与名称之间
	metaSyncDot      float32 = 1.5  // 同步状态点
	metaSyncGap      float32 = 1.25 // 状态点与标签之间
	metaSyncPadX     float32 = 1    // 同步标记左右内边距
	metaSyncPadY     float32 = 0.5  // 同步标记上下内边距
	metaClusterGap   float32 = 0.5  // 来源与同步标记之间
)

// autoTag 渲染内容子类型的自动标签（链接、代码），没有标签时不占空间。
// 预览标题把它紧跟在类型标签之后，让子类型与类型同处首行，不再单独占位。
func (a *application) autoTag(c *ui.Context, theme *ui.Theme, item history.Item) {
	tag := a.codeTag(item)
	if tag == "" {
		return
	}
	tagColor := colorForKind(theme, item.Kind)
	ui.Text(c, tag).FontSize(font(theme, fontCaption)).TextColor(tagColor).
		Padding(space(theme, metaTagPadY), space(theme, metaTagPadX)).Radius(radius(theme, controlRadius)).
		Background(tagColor.Alpha(0.10)).
		Border(metaTagBorder, tagColor.Alpha(0.24)).SingleLine()
}

// autoTagWidth 是自动标签占用的宽度；没有标签时为 0，与 autoTag 不创建元素
// 的情形一致。
func (a *application) autoTagWidth(c *ui.Context, theme *ui.Theme, item history.Item) float32 {
	tag := a.codeTag(item)
	if tag == "" {
		return 0
	}
	return textWidth(c, tag, font(theme, fontCaption), 0) +
		2*space(theme, metaTagPadX) + 2*metaTagBorder
}

func (a *application) historyPreview(c *ui.Context, theme *ui.Theme, view history.View) {
	pane := ui.Column(c).Grow(1).FillHeight().
		MinWidth(paneMinWidth(theme, minPreviewPaneWidth)).Background(theme.Background)
	pane.Children(func() {
		// 与列表一致：预览区同样让出标题栏带，元信息行落在同一条首行基线上。
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
		ui.Column(c).Center().Gap(space(theme, 2)).Transition(motionAppear).Children(func() {
			ui.Box(c).Size(space(theme, 12), space(theme, 12)).Shrink(0).Radius(radius(theme, cardRadius)).
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
	ui.Column(c).Shrink(0).FillWidth().MaxWidth(detailMaxWidth(theme)).AlignSelf(ui.Center).DragWindow().
		Padding(space(theme, columnTopInset), space(theme, spacePageSide), space(theme, 3), space(theme, spacePageSide)).
		Gap(space(theme, 1.75)).Children(func() {
		// 元信息行预留固定高度，标记就不会撑高这一行，切换条目时下方的正文卡片
		// 也不再上下抖动。
		ui.Row(c).MinHeight(controlHeightSize(c, theme, 4.5)).AlignItems(ui.Center).
			Gap(space(theme, metaRowGap)).Children(func() {
			a.previewMetaRow(c, theme, item, a.previewContentWidth(c, theme))
		})
	})
}

// previewContentWidth 返回预览栏内容列的可用宽度：窗口宽度减去筛选栏、历史列表
// 和分割线，再受详情列最大宽度与左右留白的限制。元信息行按它决定显示多少信息。
func (a *application) previewContentWidth(c *ui.Context, theme *ui.Theme) float32 {
	windowWidth, _ := c.Size()
	previewWidth := windowWidth - a.filterRailSize(c, theme) - a.historyPaneWidth - splitDividerWidth
	// 窗口比最小宽度还窄时，两侧面板各自守住自己的最小宽度，所以预览栏不会比
	// minPreviewPaneWidth 更窄；这里取同一个下限，取舍判断才与实际布局一致。
	previewWidth = max(previewWidth, paneMinWidth(theme, minPreviewPaneWidth))
	previewWidth = min(previewWidth, detailMaxWidth(theme))
	return previewWidth - 2*space(theme, spacePageSide)
}

// previewMetaRow 渲染预览标题的元信息行：类型图标与类型名、自动标签、内容片段，
// 右端是来源、同步状态和时间。
//
// 宽度不够时只隐藏内容片段，而且从最不重要的开始：大小先消失，其次才是字符数，
// 所以图片的分辨率、文件的大小作为各自类型里最后的信息保留到最后一刻。类型、
// 来源、同步状态和时间不参与取舍——它们回答「这是什么、从哪来、什么时候」，
// 比内容体量更重要。
//
// 本机条目带同步状态标记，局域网条目没有。标记并进来源「本机」一组，让「本机
// 已同步」读作这条记录的来源与去向；它带弹出浮层，且正好落在复制/收藏按钮上方，
// 所以改成点击展开，指针滑过不会再误触发（见 syncIndicator）。
func (a *application) previewMetaRow(c *ui.Context, theme *ui.Theme, item history.Item, width float32) {
	baseKind := item.Kind.BaseKind()
	kindColor := colorForKind(theme, baseKind)
	metaSize := font(theme, fontMeta)
	captionSize := font(theme, fontCaption)
	secondary := secondaryText(theme)
	gap := space(theme, metaRowGap)

	// 每个内容片段由它前面的间隔符和文字两个子项组成，所以除了两者自身的宽度，
	// 还要算上它让行多出来的两个间距（见 fitMetaSegments）。
	segments := a.historyMetaSegments(item)
	segmentWidths := make([]float32, len(segments))
	for index, segment := range segments {
		segmentWidths[index] = textWidth(c, "·", metaSize, 0) + textWidth(c, segment, metaSize, 0)
	}
	visible := fitMetaSegments(width, a.previewMetaFixedWidth(c, theme, item, gap), gap, segmentWidths)

	kindIconSize := space(theme, metaKindIconSize)
	ui.Icon(c, iconForKind(baseKind)).Size(kindIconSize, kindIconSize).Shrink(0).TextColor(kindColor)
	ui.Text(c, a.kindLabel(baseKind)).FontSize(metaSize).FontWeight(600).
		TextColor(kindColor).SingleLine()
	a.autoTag(c, theme, item)
	for _, segment := range segments[:visible] {
		ui.Text(c, "·").FontSize(metaSize).TextColor(secondary)
		ui.Text(c, segment).FontSize(metaSize).TextColor(secondary).SingleLine()
	}
	ui.Spacer(c)
	// 来源与同步标记收进同一子行并收紧间距，让「本机 已同步」读作一组：
	// 先交代记录从哪来，再交代同步到了哪。
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, metaClusterGap)).Children(func() {
		a.historySource(c, theme, item)
		a.syncIndicator(c, theme, item)
	})
	ui.Text(c, "·").FontSize(captionSize).TextColor(secondary)
	ui.Text(c, a.timestampLabel(item.CreatedAt)).FontSize(captionSize).
		TextColor(secondary).SingleLine()
}

// previewMetaFixedWidth 返回元信息行里不可隐藏部分占用的宽度，包含子项之间的
// 间距：类型图标与类型名、自动标签、撑开左右的 Spacer、来源与同步状态、分隔符
// 和时间。宽度按各部分真实的字体、内边距和间距累加，所以取舍判断与实际布局
// 一致。
func (a *application) previewMetaFixedWidth(c *ui.Context, theme *ui.Theme, item history.Item, gap float32) float32 {
	captionSize := font(theme, fontCaption)
	widths := []float32{
		space(theme, metaKindIconSize),
		textWidth(c, a.kindLabel(item.Kind.BaseKind()), font(theme, fontMeta), 600),
	}
	if tag := a.autoTagWidth(c, theme, item); tag > 0 {
		widths = append(widths, tag)
	}
	widths = append(widths, 0) // Spacer：不占宽度，但和别的子项一样分到一个间距。
	source := a.historySourceWidth(c, theme, item)
	if sync := a.syncIndicatorWidth(c, theme, item); sync > 0 {
		source += space(theme, metaClusterGap) + sync
	}
	widths = append(widths,
		source,
		textWidth(c, "·", captionSize, 0),
		textWidth(c, a.timestampLabel(item.CreatedAt), captionSize, 0),
	)
	total := float32(0)
	for _, width := range widths {
		total += width
	}
	return total + float32(len(widths)-1)*gap
}

// fitMetaSegments 返回宽度放得下的内容片段数量。片段按重要程度排列，放不下的
// 从最后一个开始整段隐藏：宁可少显示，也不要挤到行里溢出。
//
// 每个片段占两个子项（分隔符与文字），因此它让行多出两个 gap；fixed 已经包含
// 不可隐藏部分自身的间距，这里再加上片段自己那一份。
func fitMetaSegments(width, fixed, gap float32, segments []float32) int {
	used := fixed
	visible := 0
	for _, segment := range segments {
		cost := segment + 2*gap
		if used+cost > width {
			break
		}
		used += cost
		visible++
	}
	return visible
}

// textWidth 返回一段文字在给定字号和字重下占用的宽度。它和 ui.Text 用同一套
// 字体度量（字重 0 即默认字重），元信息行因此能在渲染之前判断还剩多少位置。
func textWidth(c *ui.Context, value string, size float32, weight int) float32 {
	width, _ := c.MeasureText(0, ui.Span{Text: value, Size: size, Weight: weight})
	return width
}

func (a *application) previewContent(c *ui.Context, theme *ui.Theme, item history.Item) {
	content := ui.Scroll(c).Grow(1).MinHeight(0)
	content.Children(func() {
		ui.Column(c).FillWidth().MaxWidth(detailMaxWidth(theme)).AlignSelf(ui.Center).
			Padding(0, space(theme, spacePageSide), previewActionsClearance(c, theme)+space(theme, 2), space(theme, spacePageSide)).
			Gap(space(theme, 3)).Children(func() {
			switch item.Kind {
			case kindCode:
				a.codePreview(c, theme, item)
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
				ui.Box(c).MaxWidth(detailMaxWidth(theme)).Center().Padding(space(theme, 3)).Radius(radius(theme, cardRadius)).
					Background(panelFill(theme)).Border(1, borderColor(theme)).Children(func() {
					if bitmap := a.historyBitmap(item); bitmap != nil {
						ui.Image(c, bitmap).MaxWidth(detailMaxWidth(theme) - space(theme, 6)).MaxHeight(520).
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

// syncIndicatorWidth 是同步状态标记占用的宽度；条目没有标记时为 0，与
// syncIndicator 不创建元素的情形一致。
func (a *application) syncIndicatorWidth(c *ui.Context, theme *ui.Theme, item history.Item) float32 {
	if !item.IsLocal() || item.Sync == syncStateNone {
		return 0
	}
	label, _ := a.syncStateStyle(theme, item.Sync)
	return space(theme, metaSyncDot) + space(theme, metaSyncGap) +
		textWidth(c, label, font(theme, fontCaption), 600) + 2*space(theme, metaSyncPadX)
}

// syncIndicator renders the item's sync state as a compact chip in the preview
// meta row. The per-device detail and resend actions live in a popover the chip
// opens on hover, so the section costs no space while it is idle.
func (a *application) syncIndicator(c *ui.Context, theme *ui.Theme, item history.Item) {
	// Only items captured on this machine are delivered to peers, so only
	// they carry a sync status. A peer item already arrived here.
	if !item.IsLocal() || item.Sync == syncStateNone {
		return
	}
	label, color := a.syncStateStyle(theme, item.Sync)
	chip := ui.Row(c.Key("sync-chip-"+item.ID)).Shrink(0).AlignItems(ui.Center).
		Gap(space(theme, metaSyncGap)).Padding(space(theme, metaSyncPadY), space(theme, metaSyncPadX)).
		Radius(radius(theme, controlRadius))
	hovered := chip.Hovered()
	if hovered {
		chip.Background(hoverFill(theme))
	}
	chip.Children(func() {
		statusDot(c, color, space(theme, metaSyncDot))
		ui.Text(c, label).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(primaryText(theme)).SingleLine()
	})

	// 浮层由指针悬停展开：指针停在状态标记或其浮层上时保持展开，移开两者后收起，
	// 指针在标记与浮层之间移动时不会中途关闭，浮层里的重发按钮始终可点。展开状态
	// 按条目记忆，切换条目即收起。
	if a.syncPopoverItem != item.ID {
		a.syncPopoverItem = item.ID
		a.syncPopoverOpen = false
	}
	if hovered {
		a.syncPopoverOpen = true
	}
	open := a.syncPopoverOpen
	panelHovered := false
	ui.PopoverBase(c, chip, &open, func(panel ui.Element) {
		panel.MinWidth(space(theme, 60)).MaxWidth(space(theme, 96)).
			Padding(space(theme, 2)).
			Radius(radius(theme, cardRadius)).
			Background(theme.Background).Border(1, borderColor(theme)).
			Shadow(0, space(theme, 1.5), space(theme, 5), 0, ui.RGBA(0, 0, 0, 0.18)).
			Transition(ui.ElementTransition{
				Duration: 140 * time.Millisecond,
				Ease:     ui.EaseOut,
				Enter:    &ui.Motion{Opacity: 0, Scale: 0.98, Y: -4},
			}).
			Margin(0)
		a.syncPopoverContent(c, theme, item)
		panelHovered = panel.Hovered()
	})
	if !hovered && !panelHovered {
		a.syncPopoverOpen = false
	}
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

// historySourceName 返回记录来源的名称与图标：本机捕获的条目是「本机」，
// 局域网条目是它来自的配对设备名。设备按稳定 ID 解析，改名后仍然正确。
func (a *application) historySourceName(item history.Item) (*ui.SVG, string) {
	if item.IsLocal() {
		return icons.devices, a.t(msgHistorySourceThisDevice)
	}
	if device, ok := a.pairedDeviceByID(item.SourceDeviceID); ok {
		return icons.lan, a.tf(msgHistorySourceLanFormat, device.name)
	}
	return icons.lan, a.t(msgHistorySourceLanDevice)
}

// historySourceWidth 是来源标记占用的宽度，与 historySource 的布局共用同一组
// 尺寸常量。
func (a *application) historySourceWidth(c *ui.Context, theme *ui.Theme, item history.Item) float32 {
	_, name := a.historySourceName(item)
	return space(theme, metaSourceIcon) + space(theme, metaSourceGap) +
		textWidth(c, name, font(theme, fontCaption), 0)
}

// historySource 渲染记录的来源标记：图标加名称。
func (a *application) historySource(c *ui.Context, theme *ui.Theme, item history.Item) {
	icon, name := a.historySourceName(item)
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, metaSourceGap)).Children(func() {
		ui.Icon(c, icon).Size(space(theme, metaSourceIcon), space(theme, metaSourceIcon)).Shrink(0).
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
		ui.Row(c).FillWidth().MaxWidth(detailMaxWidth(theme)).Justify(ui.Center).
			Padding(0, space(theme, spacePageSide), space(theme, previewBarBottomInset), space(theme, spacePageSide)).
			Children(func() {
				ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).
					Padding(space(theme, previewBarPadding)).
					Radius(radius(theme, cardRadius)).
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
		TextColor(secondaryText(theme)).Label(label).Role(ui.RoleButton).Transition(motionColors)
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
		TextColor(theme.Danger).Label(label).Role(ui.RoleButton).Transition(motionColors)
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
	case kindRichText:
		return theme.Accent.Mix(theme.Success, 0.45)
	default:
		return theme.TextMuted
	}
}
