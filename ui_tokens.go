package main

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// GoPaste 的动效参考 Apple 的 WWDC23《Animate with springs》与 HIG Motion：
// 位移和尺寸一律用弹簧，可中断、可改目标，中途反向不会硬停；被裁剪的展开/收起
// 用零回弹的弹簧，否则回弹会被裁掉；只换位置而不传达信息时用淡入淡出。
// 时长参考 NN/g 的建议：常规动画 100–500 ms，取能看清的最短时间，所以悬停与
// 选中用 120 ms、页面切换用 140 ms、布局变化用 300 ms。MyGo 在系统开启
// 「减弱动态效果」时会自动直接到目标，这里不必再判断。
const (
	motionColorDuration = 120 * time.Millisecond
	motionPageDuration  = 140 * time.Millisecond
	motionLayoutSpring  = 300 * time.Millisecond
)

var (
	// motionColors 用于悬停、选中、聚焦这类只改颜色、不动布局的反馈。
	motionColors = ui.ElementTransition{Colors: true, Duration: motionColorDuration}

	// motionLayout 用于展开/收起这类尺寸变化：内容被裁剪，所以用零回弹的弹簧。
	motionLayout = ui.ElementTransition{
		Size:      true,
		Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: motionLayoutSpring}},
	}

	// motionSlide 用于被布局推动的相邻元素，让它与变化的元素同步移动。
	motionSlide = ui.ElementTransition{
		Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: motionLayoutSpring}},
	}

	// motionAppear 用于进入视野的内容：轻微上移并淡入。
	motionAppear = ui.ElementTransition{
		Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: motionLayoutSpring, Bounce: 0.1}},
		Enter:     &ui.Motion{Y: 8},
	}

	// motionPage 用于页面切换：新旧页面短暂交叉淡入淡出。
	motionPage = ui.ElementTransition{
		Duration: motionPageDuration,
		Ease:     ui.EaseOut,
		Enter:    &ui.Motion{Opacity: 0},
		Exit:     &ui.Motion{Opacity: 0},
	}
)

// GoPaste 的原生界面使用一组共享的尺寸和颜色角色。页面可以有不同的
// 信息密度，但同一层级的标题、说明、控件和卡片都从这里取值。
//
// 设计语言：安静的画布、克制的强调色、以排版和留白建立层级。
const (
	fontPageTitle  float32 = 1.55
	fontPaneTitle  float32 = 1.23
	fontSection    float32 = 1.08
	fontSubheading float32 = 0.92
	fontBody       float32 = 1
	fontControl    float32 = 0.95
	fontMeta       float32 = 0.88
	fontCaption    float32 = 0.85
	fontCode       float32 = 0.92

	spacePageSide   float32 = 6
	spacePageBottom float32 = 7
	spaceSection    float32 = 6
	spaceControl    float32 = 2
	spaceInline     float32 = 2

	controlHeight float32 = 9
	compactHeight float32 = 8
	rowHeight     float32 = 14
	// 全应用只用两级圆角：控件、行、导航项和标签用 controlRadius；卡片、浮层、
	// 对话框和媒体容器用 cardRadius。不再有临时值，嵌套圆角按「外层半径 − 内边距」
	// 推导，因此下拉面板（cardRadius）里的选项正好是 controlRadius。
	cardRadius    float32 = 3
	controlRadius float32 = 2
)

const (
	// secondarySidebarWidth is shared by the device and settings pages so their
	// detail panes begin at the same x position.
	secondarySidebarWidth        = 236
	secondarySidebarCompactWidth = 208
	// filterRailWidth is the history filter rail's width in DIPs. It has to
	// hold "all sources" and long paired-device names.
	filterRailWidth        = 208
	responsiveCompactWidth = 1024
	pageDetailMaxWidth     = 720
	// sidebarTrailingInset 是历史、设备侧栏右侧的统一留白：既对齐各页内容，
	// 也为滚动条预留出空间，避免滚动条遮挡选中卡片的边框。
	sidebarTrailingInset float32 = 4

	// columnTopInset 统一筛选栏与历史、设备、设置三页内容列的首行起始内边距，
	// 让各页栏位从同一高度开始。
	columnTopInset float32 = 3
)

func font(theme *ui.Theme, scale float32) float32 {
	return theme.Rem(scale)
}

func space(theme *ui.Theme, units float32) float32 {
	return theme.Space(units)
}

func radius(theme *ui.Theme, units float32) float32 {
	return theme.Space(units)
}

func sidebarFill(theme *ui.Theme) ui.Color {
	return theme.Surface
}

func panelFill(theme *ui.Theme) ui.Color {
	return theme.Surface
}

func subtleFill(theme *ui.Theme) ui.Color {
	return theme.SurfaceHover.Alpha(0.5)
}

// selectedFill 是选中行、筛选项和导航项背后的底色。它只是一层很淡的强调色
// 淡洗，让选中读作一种安静的状态，而不是一块亮色；强调留给文字和图标。
func selectedFill(theme *ui.Theme) ui.Color {
	return theme.Accent.Alpha(0.09)
}

func hoverFill(theme *ui.Theme) ui.Color {
	return theme.SurfaceHover
}

func borderColor(theme *ui.Theme) ui.Color {
	return theme.Border
}

func dividerColor(theme *ui.Theme) ui.Color {
	return theme.Border
}

func primaryText(theme *ui.Theme) ui.Color {
	return theme.Text
}

func secondaryText(theme *ui.Theme) ui.Color {
	return theme.TextMuted
}

func favoriteColor(theme *ui.Theme) ui.Color {
	return theme.Warning
}

func accentColor(theme *ui.Theme) ui.Color {
	return theme.Accent
}

func successFill(theme *ui.Theme) ui.Color {
	return theme.Success.Alpha(0.14)
}

func warningFill(theme *ui.Theme) ui.Color {
	return theme.Warning.Alpha(0.14)
}

func dangerFill(theme *ui.Theme) ui.Color {
	return theme.Danger.Alpha(0.14)
}

// selectionBorder 是选中项的 1px 描边。普通外观下它保持透明：选中状态完全由
// 那层淡强调色底色表达，不再描一圈亮框；宽度恒定，未选中时用透明色占位，切换
// 状态时内容不会跳动。高对比度外观下才恢复为实强调色描边。
func selectionBorder(c *ui.Context, theme *ui.Theme) ui.Color {
	if c.Preferences().HighContrast {
		return theme.Accent
	}
	return ui.Transparent
}

func contentWidth(c *ui.Context) float32 {
	width, _ := c.Size()
	return width
}

func textScale(c *ui.Context) float32 {
	scale := c.Preferences().TextScale
	if scale < 1 {
		return 1
	}
	return scale
}

func controlHeightSize(c *ui.Context, theme *ui.Theme, units float32) float32 {
	return space(theme, units*textScale(c))
}

func secondarySidebarSize(c *ui.Context, theme *ui.Theme) float32 {
	if contentWidth(c) < responsiveCompactWidth {
		return space(theme, secondarySidebarCompactWidth/4)
	}
	return secondarySidebarWidth
}

// filterRailSize 返回筛选栏当前占用的宽度：收起时为 0，内容区因此占满窗口。
func (a *application) filterRailSize(c *ui.Context, theme *ui.Theme) float32 {
	if !a.filterRailOpen {
		return 0
	}
	return space(theme, filterRailWidth/4)
}

// filterRailContentWidth 是筛选栏内容列的固定宽度：外层收窄时内容宽度不变，
// 由外层的 ClipX 裁掉，收起读起来是「滑走」而不是「挤扁」。它取外层内容盒的
// 宽度（去掉两侧 1 DIP 描边），滚动条因此仍贴在外层右缘。
func filterRailContentWidth(theme *ui.Theme) float32 {
	return space(theme, filterRailWidth/4) - 2
}

func (a *application) compactDetailLayout(c *ui.Context, theme *ui.Theme) bool {
	// 筛选栏常驻在窗口左侧（收起时为 0），设备页与设置页的详情宽度要把它一起扣掉。
	available := contentWidth(c) - a.filterRailSize(c, theme) - secondarySidebarSize(c, theme) - 1
	return available < 680*textScale(c)
}

// pageHeader keeps page title and description aligned across settings and
// device detail panes. The header doubles as the drag region for the pane,
// which no longer keeps a title-bar band of its own.
func pageHeader(c *ui.Context, theme *ui.Theme, title, description string) {
	ui.Column(c).Shrink(0).Gap(space(theme, 1)).DragWindow().Children(func() {
		ui.Text(c, title).FontSize(font(theme, fontPageTitle)).FontWeight(600).
			Role(ui.RoleHeading).SingleLine()
		ui.Text(c, description).FontSize(font(theme, fontBody)).TextColor(secondaryText(theme)).Wrap()
	})
}

// secondarySidebarTitle 是设备页与设置页侧栏顶部的首个标题。两栏共用同一份
// 定义，保证字号、字重、颜色和左边距一致，不会再各自漂移。侧栏不再保留标题栏
// 留白，因此标题行同时承担拖动窗口的职责。
func secondarySidebarTitle(c *ui.Context, theme *ui.Theme, label string) ui.Element {
	return ui.Text(c, label).FontSize(font(theme, fontPaneTitle)).FontWeight(600).
		Padding(space(theme, 0.5), 0, space(theme, 0.5), 0).SingleLine().DragWindow()
}

func standardCard(c *ui.Context, theme *ui.Theme, key string, body func()) {
	ui.Column(c.Key(key)).FillWidth().Shrink(0).Radius(radius(theme, cardRadius)).
		Background(panelFill(theme)).Border(1, borderColor(theme)).Children(body)
}

func standardDivider(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).Height(1).FillWidth().Shrink(0).Background(dividerColor(theme))
}

func neutralButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Radius(radius(theme, controlRadius)).
		TextColor(primaryText(theme)).Label(label).Role(ui.RoleButton).Transition(motionColors)
	if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	return button
}

// dangerButton 用于会删除数据或解除关联的操作，保持文字和悬停反馈为危险色。
func dangerButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Radius(radius(theme, controlRadius)).
		TextColor(theme.Danger).Label(label).Role(ui.RoleButton).Transition(motionColors)
	if button.Hovered() {
		button.Background(theme.Danger.Alpha(0.12))
	} else {
		button.Background(ui.Transparent)
	}
	return button
}

// dangerConfirmButton 是危险确认弹窗中的主操作按钮。
func dangerConfirmButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Radius(radius(theme, controlRadius)).
		Background(theme.Danger).TextColor(theme.AccentText).Label(label).
		Role(ui.RoleButton).Transition(motionColors)
	if button.Hovered() {
		button.Background(theme.Danger.Mix(theme.Text, 0.12))
	}
	return button
}

// dangerConfirmDialog 提供统一的危险操作二次确认界面。
func dangerConfirmDialog(
	c *ui.Context,
	theme *ui.Theme,
	l localizer,
	open *bool,
	title string,
	message string,
	confirmLabel string,
	dontAskLabel string,
	dontAsk *bool,
) bool {
	if !*open {
		return false
	}

	confirmed := false
	ui.Modal(c, open, func() {
		ui.Column(c).Width(440).Shrink(0).
			Padding(space(theme, 5), space(theme, 5)).Gap(space(theme, 3)).
			Radius(radius(theme, cardRadius)).Background(theme.Background).
			Border(1, borderColor(theme)).Role(ui.RoleAlertDialog).Label(title).Children(func() {
			ui.Text(c, title).FontSize(font(theme, fontSection)).FontWeight(600).
				Role(ui.RoleHeading).SingleLine()
			ui.Text(c, message).FontSize(font(theme, fontBody)).
				TextColor(secondaryText(theme)).Wrap()
			if dontAskLabel != "" {
				ui.Checkbox(c.Key("danger-confirm-dont-ask"), dontAsk, dontAskLabel).
					Shrink(0).Label(dontAskLabel)
			}
			ui.Row(c).FillWidth().Shrink(0).Justify(ui.End).Gap(space(theme, 2)).Children(func() {
				cancelLabel := l.text(msgDangerCancel)
				cancel := neutralButton(c, theme, "danger-confirm-cancel", cancelLabel)
				cancel.Children(func() {
					ui.Text(c, cancelLabel).FontSize(font(theme, fontControl)).SingleLine()
				})
				if cancel.Clicked() {
					*open = false
				}

				confirm := dangerConfirmButton(c, theme, "danger-confirm-submit", confirmLabel)
				confirm.Children(func() {
					ui.Text(c, confirmLabel).FontSize(font(theme, fontControl)).FontWeight(600).SingleLine()
				})
				if confirm.Clicked() {
					confirmed = true
					*open = false
				}
			})
		})
	})

	return confirmed
}

func statusDot(c *ui.Context, color ui.Color, size float32) {
	ui.Box(c).Size(size, size).Shrink(0).Radius(size / 2).Background(color)
}

// option pairs a stable value with the user-facing label shown for it, so a
// select can bind to a value that does not change with the UI language while
// the visible text is derived each frame.
type option[T comparable] struct {
	value T
	label string
}

// labelFor returns the label for value, or "" when value is not an option.
func labelFor[T comparable](value T, options []option[T]) string {
	for _, opt := range options {
		if opt.value == value {
			return opt.label
		}
	}
	return ""
}

// selectField renders a themed drop-down that stores a stable value while
// showing a localized label. It wraps MyGo's SelectBase, so keyboard, focus and
// popup behavior stay native while the bound value stays language-independent.
// Apply a width to the returned element, or call FillWidth on it.
func selectField[T comparable](
	c *ui.Context,
	theme *ui.Theme,
	key string,
	value *T,
	label string,
	options []option[T],
) ui.Element {
	sel := ui.SelectBase(c.Key(key), value)
	trigger := sel.Trigger
	trigger.Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).
		Padding(space(theme, 1.5), space(theme, 3)).Gap(space(theme, 1.5)).
		Radius(radius(theme, controlRadius)).Border(1, borderColor(theme)).
		Background(theme.Surface).TextColor(primaryText(theme)).
		Justify(ui.SpaceBetween).Label(label).Transition(motionColors)
	trigger.Children(func() {
		ui.Text(c, labelFor(*value, options)).FontSize(font(theme, fontControl)).SingleLine()
		ui.Icon(c, icons.chevron).Size(space(theme, 3.5), space(theme, 3.5)).
			Shrink(0).TextColor(secondaryText(theme))
	})
	sel.Popup(func(panel ui.Element) {
		panel.Padding(space(theme, 1)).Radius(radius(theme, cardRadius)).
			Background(theme.Background).Border(1, borderColor(theme)).
			Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.18))
		for _, opt := range options {
			item := sel.Item(opt.value).Padding(space(theme, 1.5), space(theme, 2.5)).
				Radius(radius(theme, controlRadius)).Border(1, ui.Transparent).
				TextColor(primaryText(theme)).Transition(motionColors)
			// 当前值与指针/方向键所在的项用同一套选中样式，
			// 而不是一块实心强调色：下拉里的选中和列表里的选中看起来一样。
			switch {
			case item.Highlighted():
				item.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme))
			case opt.value == *value:
				item.Background(selectedFill(theme))
			}
			item.Children(func() {
				ui.Text(c, opt.label).FontSize(font(theme, fontControl)).SingleLine()
			})
		}
	})
	return trigger
}

// segmentedField 渲染分段控件（模式切换），并让选中的那一段使用与行、筛选项
// 相同的选中样式：淡强调色底 + 淡描边，而不是一块自带的凸起白片。
func segmentedField(
	c *ui.Context,
	theme *ui.Theme,
	key string,
	selected *int,
	label string,
	labels []string,
) ui.Element {
	parts := ui.SegmentedBase(c.Key(key), selected, len(labels))
	track := parts.Track
	track.Shrink(0).AlignItems(ui.Stretch).
		Padding(space(theme, 0.5)).Gap(space(theme, 0.5)).
		Radius(radius(theme, controlRadius)).Background(subtleFill(theme)).
		Border(1, borderColor(theme)).Label(label)
	track.Children(func() {
		for index, text := range labels {
			segment := parts.Segment(index).Shrink(0).Center().
				Padding(space(theme, 1), space(theme, 2.5)).
				Radius(radius(theme, controlRadius)).Border(1, ui.Transparent).
				TextColor(secondaryText(theme)).Transition(motionColors)
			switch {
			case index == *selected:
				segment.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).
					TextColor(primaryText(theme))
			case segment.Hovered():
				segment.Background(hoverFill(theme))
			}
			segment.Children(func() {
				ui.Text(c, text).FontSize(font(theme, fontControl)).FontWeight(500).SingleLine()
			})
		}
	})
	return track
}

// keycap 渲染单键快捷键提示：小圆角方块内一个字母，跟在按钮文字之后。fg 取按钮
// 前景色，让提示在强调色按钮和普通按钮上都与文字保持同一色调。
func keycap(c *ui.Context, theme *ui.Theme, key string, fg ui.Color) {
	ui.Box(c).Shrink(0).Center().
		MinWidth(space(theme, 4)).Height(space(theme, 4)).
		Padding(0, space(theme, 0.75)).Radius(radius(theme, controlRadius)).
		Background(fg.Alpha(0.16)).Border(1, fg.Alpha(0.28)).Children(func() {
		ui.Text(c, key).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(fg).SingleLine()
	})
}

// primaryButton 是面板中的主操作按钮：使用系统强调色填充，文字与图标反色。
// 复制是历史预览的首要动作，因此它承担这一角色。shortcut 非空时在文字后显示快捷键。
func primaryButton(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, shortcut string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Gap(space(theme, 1)).
		Radius(radius(theme, controlRadius)).
		Background(theme.Accent).TextColor(theme.AccentText).Label(label).
		Role(ui.RoleButton).Transition(motionColors)
	if button.Hovered() {
		button.Background(theme.AccentHover)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0)
		ui.Text(c, label).FontSize(font(theme, fontControl)).FontWeight(600).SingleLine()
		if shortcut != "" {
			keycap(c, theme, shortcut, theme.AccentText)
		}
	})
	return button
}
