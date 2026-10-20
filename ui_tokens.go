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

	// motionLayoutSpringAnimation 是布局变化的弹簧，元素过渡与数值动画共用，
	// 保证侧栏宽度和它让出的留白同步移动。
	motionLayoutSpringAnimation = ui.Animation{Spring: ui.SpringOptions{Duration: motionLayoutSpring}}

	// motionLayout 用于展开/收起这类尺寸变化：内容被裁剪，所以用零回弹的弹簧。
	motionLayout = ui.ElementTransition{
		Size:      true,
		Animation: &motionLayoutSpringAnimation,
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

	// motionSpinPeriod 是刷新按钮转一圈的时间，也是点击后的最短转圈时间。
	// 取整圈而不是更短的固定值：图标停下来时正好回到静止角度，不会从半圈
	// 突然跳回去；反过来，最短时间取整圈也保证后端再快，点击都看得见反馈。
	motionSpinPeriod = 900 * time.Millisecond
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
	// listRowHeight 是双行行（历史记录、设备条目）的最小高度；listRowTileSize 是
	// 它左侧图标块的边长（内部图标 4.5，即 18 DIP）；listRowGap 是行与行之间的
	// 垂直间距，由所在容器提供（历史列表用行的上下 margin，设备列表用列间距），
	// 因此两种列表的第一行都正好落在所在栏的首行基线上。
	listRowHeight   float32 = 16
	listRowTileSize float32 = 9
	listRowGap      float32 = 1
	// 全应用只用两级圆角：控件、行、导航项和标签用 controlRadius；卡片、浮层、
	// 对话框和媒体容器用 cardRadius。不再有临时值，嵌套圆角按「外层半径 − 内边距」
	// 推导，因此下拉面板（cardRadius）里的选项正好是 controlRadius。
	cardRadius    float32 = 3
	controlRadius float32 = 2
)

// 弹窗（对话框与二次确认）的共享几何。宽度按 DIP 走缩放网格，其余走 theme.Space
// 网格，因此界面缩放时弹窗和窗口里的控件同步放大。
const (
	// dialogWidth 是弹窗宽度，单位 DIP：去掉两侧 20 DIP 内边距后正文约 400 DIP，
	// 落在 Apple 提醒框 420 pt 的正文宽度附近，也在 Material 3 对话框
	// 280–560 dp 的区间内。
	dialogWidth float32 = 440
	// dialogPadding 是弹窗四周的留白，20 DIP。
	dialogPadding float32 = 5
	// dialogGap 是弹窗里相邻内容的间距，8 DIP：正文块内部与按钮之间都用它。
	dialogGap float32 = 2
	// dialogActionGap 是正文与按钮行之间额外让出的留白，12 DIP；与 dialogGap 相加
	// 是 20 DIP，按钮行因此从正文里明确分出来，而不是和说明连成一段。
	dialogActionGap float32 = 3
)

const (
	// filterRailWidth is the sidebar's width in DIPs. The sidebar holds the
	// history filters, the device list, or the settings panes, so it has to
	// fit "all sources" and long paired-device names.
	// filterRailWidth 是侧栏宽度，单位 DIP。底栏左侧是图标组（含一个暂时留空的
	// 第 5 个位置），右侧是单独右对齐的外观按钮：4 个 32 DIP 图标位 + 外观 + 4 个
	// navButtonGap 间隙 = 218 − 30，正好铺满，所以间隙是整数 7 DIP、外观贴右留白。
	filterRailWidth = 218
	// navButtonGap 是底栏图标之间的固定间隙，单位 DIP。用固定值而不是均分，任何
	// 宽度下都不会算出小数间隙；余量由外观按钮前的 Spacer 吸收。
	navButtonGap       = 7
	pageDetailMaxWidth = 720
	// paneSideInset 是侧栏与历史列表左右两侧的统一留白，单位 DIP：整数，两侧取
	// 同一个值，选中的条目/行才会在所在栏里居中；右侧同时让开覆盖式滚动条
	// （滚动条占右侧 3–9 DIP），高亮框因此不会被它压住。
	paneSideInset = 14

	// columnTopInset 是各面板内容列首行距标题栏带底边的留白。每个面板都先让出
	// 标题栏带（窗口控件与侧栏开关在带内居中），再从这条基线开始内容，因此
	// 侧栏、列表、预览、设备与设置详情的第一行落在同一条线上。
	//
	// 依据 HIG 派生布局参考的内容边距：「Top margin（无标签视图）14 pt，从标题栏/
	// 工具栏底边到第一个控件」；取同网格上的 12 DIP。
	columnTopInset float32 = 3
)

func font(theme *ui.Theme, scale float32) float32 {
	return theme.Rem(scale)
}

// spaceDIP 把以 DIP 计的尺寸换算成 theme.Space 的单位。必须用浮点除法：写成
// dip/4 时两个无类型整数常量会做整数除法（14/4 = 3），留白会静默少掉 2 DIP。
func spaceDIP(theme *ui.Theme, dip float32) float32 {
	return theme.Space(dip / 4)
}

// detailMaxWidth 与 paneMinWidth 把用 DIP 写死的布局下限换算到当前界面缩放：
// 放大界面时这些下限同步放大，内容列不会反而被 DIP 上限压窄。
func detailMaxWidth(theme *ui.Theme) float32 { return spaceDIP(theme, pageDetailMaxWidth) }
func paneMinWidth(theme *ui.Theme, dip float32) float32 {
	return spaceDIP(theme, dip)
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

// labelText 画一段固定文案：单行、按内容定宽，不折行也不省略。
//
// 按内容定宽的盒子宽度恰好等于文字自身的宽度，而 Windows 的 DirectWrite 拿到的
// 宽度比它自己量出来的少一个浮点最低位，就按这个宽度断行；中文每两个字之间都是
// 断行点，「收藏」于是显示成「收…」。NoWrap 让文字按宽度 0 断行，盒子再窄也不会
// 断，省略号无从发生。可能显示用户长内容（历史正文、文件路径、设备名）的文字仍用
// SingleLine，按可用宽度截断。
func labelText(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).SingleLine().NoWrap()
}

// filterRailSize 返回筛选栏当前占用的宽度：收起时为 0，内容区因此占满窗口。
func (a *application) filterRailSize(c *ui.Context, theme *ui.Theme) float32 {
	if !a.filterRailOpen {
		return 0
	}
	return spaceDIP(theme, filterRailWidth)
}

// filterRailContentWidth 是筛选栏内容列的固定宽度：外层收窄时内容宽度不变，
// 由外层的 ClipX 裁掉，收起读起来是「滑走」而不是「挤扁」。它取外层内容盒的
// 宽度（去掉两侧 1 DIP 描边），滚动条因此仍贴在外层右缘。
func filterRailContentWidth(theme *ui.Theme) float32 {
	return spaceDIP(theme, filterRailWidth) - 2
}

func (a *application) compactDetailLayout(c *ui.Context, theme *ui.Theme) bool {
	// 筛选栏常驻在窗口左侧（收起时为 0），设备页与设置页的详情宽度要把它一起扣掉。
	available := contentWidth(c) - a.filterRailSize(c, theme) - 1
	// 缩放会同时放大文字和 4 DIP 网格，所以紧凑布局的门槛也跟着放大，
	// 否则放大后明明装不下却还按非紧凑布局排版。
	return available < spaceDIP(theme, 680)*textScale(c)
}

// pageHeader keeps page title and description aligned across settings and
// device detail panes. The header doubles as the drag region for the pane,
// which no longer keeps a title-bar band of its own.
func pageHeader(c *ui.Context, theme *ui.Theme, title, description string) {
	ui.Column(c).Shrink(0).Gap(space(theme, 1)).DragWindow().Children(func() {
		labelText(c, title).FontSize(font(theme, fontPageTitle)).FontWeight(600).
			Role(ui.RoleHeading)
		ui.Text(c, description).FontSize(font(theme, fontBody)).TextColor(secondaryText(theme)).Wrap()
	})
}

func standardCard(c *ui.Context, theme *ui.Theme, key string, body func()) {
	ui.Column(c.Key(key)).FillWidth().Shrink(0).Radius(radius(theme, cardRadius)).
		Background(panelFill(theme)).Border(1, borderColor(theme)).Children(body)
}

func standardDivider(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).Height(1).FillWidth().Shrink(0).Background(dividerColor(theme))
}

// floatingShadow 是浮层共用的投影颜色：淡到只把浮层从画布上托起来，符合设计系统
// 「不要重投影」的要求。
var floatingShadow = ui.RGBA(0, 0, 0, 0.18)

// dialogScrim 是弹窗背后压暗窗口的那层黑色，与 MyGo 的 Modal、AlertDialog 取同一个
// 深度。它不分明暗外观：Material 3 的对话框遮罩也不随主题变化。
var dialogScrim = ui.RGBA(0, 0, 0, 0.4)

// floatingSurface 给浮层套上统一外观：卡片圆角、不透明表面、1px 语义描边和同一档
// 投影。对话框、气泡、下拉面板和悬浮操作条都走它，各自的几何（宽度、内边距、锚点）
// 由调用方给出。
func floatingSurface(theme *ui.Theme, panel ui.Element) ui.Element {
	return panel.Radius(radius(theme, cardRadius)).Background(theme.Background).
		Border(1, borderColor(theme)).
		Shadow(0, space(theme, 1.5), space(theme, 5), 0, floatingShadow)
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

// dialogSpec 是一个弹窗的内容。弹窗的外观只在 dialog 里定义一次：一个表面（不再
// 套第二层卡片）、固定的宽度与内边距、标题与说明的排版、右对齐的按钮行，以及打开
// 时落在关闭按钮上的焦点。调用方只填内容，并用下面的按钮助手构建按钮行。
type dialogSpec struct {
	// title 是弹窗标题，也是辅助技术读到的弹窗名字。
	title string
	// role 取 RoleAlertDialog（二次确认）或 RoleDialog（任务型弹窗）。
	role ui.Role
	// message 是标题下的说明，可留空。
	message string
	// badge 非空时在标题正下方画一行状态：一个圆点和标签，用 badgeColor。
	// 它陈述弹窗所涉对象的当前状态，因此排在说明之前。
	badge      string
	badgeColor ui.Color
	// content 是说明与按钮行之间的额外内容（错误、进度、复选项），可为 nil。
	content func()
	// actions 构建按钮行，至少一个按钮。
	actions func()
}

// dialog 渲染一个弹窗并返回它的面板，供调用方登记覆盖层快捷键。
//
// 用 DialogBase 而不是 Modal：Modal 会给面板套上自己的内边距、表面和投影，我们再
// 套一层卡片时那条描边就画在投影边缘往里 20 DIP 的地方。DialogBase 只提供遮罩、
// 焦点陷阱和 Escape，表面完全由这里决定，弹窗因此只有一个表面。
func dialog(c *ui.Context, theme *ui.Theme, open *bool, spec dialogSpec) ui.Element {
	width, height := c.Size()
	return ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(dialogScrim)
		floatingSurface(theme, panel).
			Width(spaceDIP(theme, dialogWidth)).
			MaxWidth(width - space(theme, 10)).
			MaxHeight(height - space(theme, 10)).
			Padding(space(theme, dialogPadding)).
			Gap(space(theme, dialogGap)).
			Role(spec.role).Label(spec.title).Description(spec.message)

		ui.Text(c, spec.title).FontSize(font(theme, fontSection)).FontWeight(600).
			Role(ui.RoleHeading).MaxLines(2).Wrap()
		if spec.badge != "" {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
				statusDot(c, spec.badgeColor, space(theme, 2))
				labelText(c, spec.badge).FontSize(font(theme, fontControl)).
					TextColor(spec.badgeColor).FontWeight(600)
			})
		}
		if spec.message != "" {
			ui.Text(c, spec.message).FontSize(font(theme, fontBody)).
				TextColor(secondaryText(theme)).Wrap()
		}
		if spec.content != nil {
			spec.content()
		}
		ui.Row(c).FillWidth().Shrink(0).Justify(ui.End).Gap(space(theme, dialogGap)).
			Margin(space(theme, dialogActionGap), 0, 0, 0).Children(spec.actions)
	})
}

// dialogDismissButton 是弹窗里的取消/关闭按钮：安静的字形、Escape 键帽，并在弹窗
// 打开时拿到焦点。所有弹窗的退出路径因此一致——Escape、这个按钮、点击弹窗之外，
// 三者都只是取消。焦点落在它而不是确认按钮上，是为了让 Return 永远不会触发不可撤销
// 的操作；Apple 的提醒框同样建议不要让用户习惯性地按 Return 打发掉提醒。
func dialogDismissButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	button := neutralButton(c, theme, key, label).Gap(space(theme, 1)).AutoFocus()
	button.Children(func() {
		labelText(c, label).FontSize(font(theme, fontControl))
		keycap(c, theme, shortcutKeyLabel(ui.KeyEscape), primaryText(theme))
	})
	return button
}

// dialogPrimaryButton 是任务型弹窗里的主操作按钮：强调色填充、文字反色。
func dialogPrimaryButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	return primaryButton(c, theme, key, nil, label, "")
}

// dialogDangerButton 是二次确认里的确认按钮：危险色填充，文字反色。shortcut 非空
// 时在文字后画出键帽——只画一个，其余可用按键照常生效但不显示，免得按钮上挤满键帽。
func dialogDangerButton(c *ui.Context, theme *ui.Theme, key, label, shortcut string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Radius(radius(theme, controlRadius)).
		Background(theme.Danger).TextColor(theme.AccentText).Label(label).
		Role(ui.RoleButton).Transition(motionColors)
	if button.Hovered() {
		button.Background(theme.Danger.Mix(theme.Text, 0.12))
	}
	button.Children(func() {
		labelText(c, label).FontSize(font(theme, fontControl)).FontWeight(600)
		if shortcut != "" {
			keycap(c, theme, shortcut, theme.AccentText)
		}
	})
	return button
}

// dangerDialogKeys 是二次确认框的按键配置：confirm 与 cancel 是要注册的覆盖层
// 快捷键，confirmShown 是确认按钮上唯一画成键帽的那个。多出来的按键照常生效，
// 只是不显示，免得按钮上挤满键帽。Escape 由弹窗自己处理，不放进 cancel。
type dangerDialogKeys struct {
	confirm      []ui.Key
	confirmShown ui.Key
	cancel       []ui.Key
}

// dangerConfirmDialog 渲染危险操作的二次确认框，并在打开期间为 keys 里的按键
// 注册覆盖层快捷键。Escape 由弹窗自己处理：关闭即取消。
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
	keys dangerDialogKeys,
) bool {
	if !*open {
		return false
	}

	confirmed := false
	panel := dialog(c, theme, open, dialogSpec{
		title:   title,
		role:    ui.RoleAlertDialog,
		message: message,
		content: func() {
			// 只有删除需要「不再询问」：勾选后本次确认即记住偏好。
			if dontAskLabel != "" {
				ui.Checkbox(c.Key("danger-confirm-dont-ask"), dontAsk, dontAskLabel).
					Shrink(0).Label(dontAskLabel)
			}
		},
		actions: func() {
			cancelLabel := l.text(msgDangerCancel)
			cancel := dialogDismissButton(c, theme, "danger-confirm-cancel", cancelLabel)
			if cancel.Clicked() {
				*open = false
			}

			confirm := dialogDangerButton(c, theme, "danger-confirm-submit", confirmLabel,
				shortcutKeyLabel(keys.confirmShown))
			if confirm.Clicked() {
				confirmed = true
				*open = false
			}
		},
	})

	// 覆盖层快捷键在对话框子元素建完后登记：焦点落在「不再询问」之类的控件上时，
	// 这些按键仍然生效。确认与点击确认按钮、取消与点击取消按钮各走同一条路径。
	for _, key := range keys.confirm {
		if panel.OverlayShortcut(0, key) {
			confirmed = true
			*open = false
		}
	}
	for _, key := range keys.cancel {
		if panel.OverlayShortcut(0, key) {
			*open = false
		}
	}

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

// optionRowGapUnits 是一行选项与下一行之间的间距，单位同 space()。选项行的底色
// 只画在行自己的内边距盒里，行距为 0 时上下两行的底色会贴成一块，看起来像同一
// 块高亮，所以下拉和气泡的选项都按这个间距排版。
const optionRowGapUnits float32 = 0.5

// optionRow 把一行选项的几何统一到单行条目的规格：固定高度、左右内边距、圆角，
// 内容垂直居中。下拉里的选项与侧栏外观气泡里的选项都走这里，两处的数字不会再
// 各自漂移。
//
// 垂直的余量交给 AlignItems 而不是上下内边距：行高是固定的，Padding 会从高度里
// 扣掉它，内容盒因此比行矮，日后往行里放一个等高控件时会居中溢出。
func optionRow(c *ui.Context, theme *ui.Theme, item ui.Element) ui.Element {
	return item.Height(controlHeightSize(c, theme, compactHeight)).
		Padding(0, space(theme, 2)).
		Radius(radius(theme, controlRadius)).
		AlignItems(ui.Center)
}

// optionRowFill 返回一行选项的底色与描边，取值与列表行一致：当前值用共享的选中
// 底色，指针或方向键所在的项用悬停底色。两者因此不会看起来是同一块高亮。
func optionRowFill(c *ui.Context, theme *ui.Theme, selected, highlighted, pressed bool) (ui.Color, ui.Color) {
	switch {
	case selected:
		return selectedFill(theme), selectionBorder(c, theme)
	case pressed:
		return theme.SurfacePressed, ui.Transparent
	case highlighted:
		return hoverFill(theme), ui.Transparent
	default:
		return ui.Transparent, ui.Transparent
	}
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
		labelText(c, labelFor(*value, options)).FontSize(font(theme, fontControl))
		ui.Icon(c, icons.chevron).Size(space(theme, 3.5), space(theme, 3.5)).
			Shrink(0).TextColor(secondaryText(theme))
	})
	sel.Popup(func(panel ui.Element) {
		floatingSurface(theme, panel).Gap(space(theme, optionRowGapUnits)).
			Padding(space(theme, 1))
		for _, opt := range options {
			item := optionRow(c, theme, sel.Item(opt.value)).
				Border(1, ui.Transparent).TextColor(primaryText(theme)).Transition(motionColors)
			background, border := optionRowFill(c, theme, opt.value == *value, item.Highlighted(), item.Pressed())
			item.Background(background).BorderColor(border)
			item.Children(func() {
				labelText(c, opt.label).FontSize(font(theme, fontControl))
			})
		}
	})
	return trigger
}

// segmentedField 渲染分段控件（模式切换），并让选中的那一段使用与行、筛选项
// 相同的选中样式：淡强调色底 + 淡描边，而不是一块自带的凸起白片。
//
// 每一段等宽（`Grow(1)`，与 CSS 的 flex: 1 一样从零尺寸起步），不按文字长短定宽：
// 选中的那块因此不会随选项名字的长短变宽变窄，整条控件的宽度只由调用方给。
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
			segment := parts.Segment(index).Shrink(0).Grow(1).Center().
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
				labelText(c, text).FontSize(font(theme, fontControl)).FontWeight(500)
			})
		}
	})
	return track
}

// sidebarItemButton 是侧栏里单行条目（历史筛选项、设置分栏）的唯一实现：图标 +
// 文字，紧凑高度，选中态与列表行共用 selectedFill / selectionBorder。两处调用
// 同一份规格，不会再各自漂移。
func sidebarItemButton(
	c *ui.Context,
	theme *ui.Theme,
	key string,
	icon *ui.SVG,
	label string,
	selected bool,
	onSelect func(),
) ui.Element {
	// 不加 Tooltip：这一行本来就显示自己的文字，悬停再弹一个同名气泡只是噪音，
	// 而且提示在键盘焦点落到它上面时也会弹。提示留给不显示文字的图标按钮。
	item := ui.ButtonBase(c.Key(key)).
		Height(controlHeightSize(c, theme, compactHeight)).FillWidth().Shrink(0).
		Padding(space(theme, 1.5), space(theme, 2)).Gap(space(theme, 2)).
		Radius(radius(theme, controlRadius)).AlignItems(ui.Center).Justify(ui.Start).
		Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).Label(label).
		Role(ui.RoleToggleButton).Selected(selected).Transition(motionColors)
	switch {
	case selected:
		item.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).TextColor(primaryText(theme))
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
		if icon != nil {
			ui.Icon(c, icon).Size(space(theme, 4.25), space(theme, 4.25)).Shrink(0).TextColor(iconColor)
		}
		labelText(c, label).FontSize(font(theme, fontControl))
	})
	if item.Clicked() {
		onSelect()
	}
	return item
}

// listRow 是双行行（历史记录、设备条目）的唯一实现：左侧一个统一尺寸的图标块，
// 右侧两行文字。行高、内边距、图标块、行距与选中态只有这一份定义，两个列表因此
// 逐项一致；图标块外框尺寸统一后，它也与其所在栏里单行条目的图标、分区标题落在
// 同一条竖线上。
func listRow(
	c *ui.Context,
	theme *ui.Theme,
	key string,
	label string,
	selected bool,
	tile func(),
	lines func(),
) ui.Element {
	row := ui.ButtonBase(c.Key(key)).
		MinHeight(controlHeightSize(c, theme, listRowHeight)).AlignSelf(ui.Stretch).Shrink(0).
		Padding(space(theme, 2), space(theme, 2)).Gap(space(theme, 3)).
		Radius(radius(theme, controlRadius)).Border(1, ui.Transparent).Row().
		AlignItems(ui.Center).TextColor(primaryText(theme)).Label(label).
		Role(ui.RoleButton).Selected(selected).Transition(motionColors)
	// 注意：MyGo 的 Selected(true) 会顺手把底色设成 Accent、文字设成 AccentText，
	// 所以选中分支必须把两者都改回来——只改底色的话，行内文字会保持「强调色上的
	// 文字」（浅色外观下接近白色），贴在淡色高亮底上几乎读不出来。
	switch {
	case selected:
		row.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).TextColor(primaryText(theme))
	case row.Hovered():
		row.Background(hoverFill(theme)).BorderColor(ui.Transparent).TextColor(primaryText(theme))
	default:
		row.Background(ui.Transparent).BorderColor(ui.Transparent).TextColor(primaryText(theme))
	}
	row.Children(func() {
		tile()
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 1)).Children(lines)
	})
	return row
}

// listRowTile 是双行行左侧图标块的统一外框：固定边长、controlRadius、居中。
// 填充内容（类型色图标、缩略图、平台图标）由调用方给出。
func listRowTile(c *ui.Context, theme *ui.Theme, background ui.Color, body func()) ui.Element {
	return ui.Box(c).Size(space(theme, listRowTileSize), space(theme, listRowTileSize)).Shrink(0).
		Radius(radius(theme, controlRadius)).Background(background).Center().Children(body)
}

// shortcutKeyLabel 返回单键快捷键在键帽里的显示文本：字母与数字取键面字符，
// Escape 这类按键取它自己的名字。返回空串表示这个键没有合适的键帽。
func shortcutKeyLabel(key ui.Key) string {
	switch {
	case key >= ui.KeyA && key <= ui.KeyZ:
		return string(rune('A' + (key - ui.KeyA)))
	case key >= ui.Key0 && key <= ui.Key9:
		return string(rune('0' + (key - ui.Key0)))
	case key == ui.KeyEscape:
		return "Esc"
	}
	return ""
}

// keycap 渲染单键快捷键提示：小圆角方块内一个字母，跟在按钮文字之后。fg 取按钮
// 前景色，让提示在强调色按钮和普通按钮上都与文字保持同一色调。
func keycap(c *ui.Context, theme *ui.Theme, key string, fg ui.Color) {
	ui.Box(c).Shrink(0).Center().
		MinWidth(space(theme, 4)).Height(space(theme, 4)).
		Padding(0, space(theme, 0.75)).Radius(radius(theme, controlRadius)).
		Background(fg.Alpha(0.16)).Border(1, fg.Alpha(0.28)).Children(func() {
		labelText(c, key).FontSize(font(theme, fontCaption)).FontWeight(600).
			TextColor(fg)
	})
}

// primaryButton 是面板中的主操作按钮：使用系统强调色填充，文字与图标反色。
// 复制是历史预览的首要动作，因此它承担这一角色。shortcut 非空时在文字后显示快捷键。
// icon 为 nil 时不画图标，弹窗的按钮行只用文字。
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
		if icon != nil {
			ui.Icon(c, icon).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0)
		}
		labelText(c, label).FontSize(font(theme, fontControl)).FontWeight(600)
		if shortcut != "" {
			keycap(c, theme, shortcut, theme.AccentText)
		}
	})
	return button
}
