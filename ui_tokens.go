package main

import "github.com/egoist/mygo/ui"

// GoPaste 的原生界面使用一组共享的尺寸和颜色角色。页面可以有不同的
// 信息密度，但同一层级的标题、说明、控件和卡片都从这里取值。
const (
	fontPageTitle  float32 = 1.75
	fontSection    float32 = 1.125
	fontSubheading float32 = 0.9375
	fontBody       float32 = 1
	fontControl    float32 = 0.875
	fontMeta       float32 = 0.8125
	fontCaption    float32 = 0.75
	fontCode       float32 = 0.9375

	spacePageTop    float32 = 10
	spacePageSide   float32 = 8
	spacePageBottom float32 = 12
	spaceSection    float32 = 6
	spaceControl    float32 = 3
	spaceInline     float32 = 2

	controlHeight float32 = 8.5
	compactHeight float32 = 7.5
	rowHeight     float32 = 14
	cardRadius    float32 = 3
	controlRadius float32 = 2
	largeRadius   float32 = 4
)

const (
	// secondarySidebarWidth is shared by the device and settings pages so their
	// detail panes begin at the same x position.
	secondarySidebarWidth         = 224
	pageDetailMaxWidth            = 720
	historyTrailingInset  float32 = 4
	historyControlGap     float32 = 1
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
	return theme.Surface.Alpha(0.68)
}

func panelFill(theme *ui.Theme) ui.Color {
	return theme.Surface.Alpha(0.88)
}

func subtleFill(theme *ui.Theme) ui.Color {
	return theme.Surface.Alpha(0.56)
}

func selectedFill(theme *ui.Theme) ui.Color {
	return theme.Accent.Alpha(0.14)
}

func hoverFill(theme *ui.Theme) ui.Color {
	return theme.SurfaceHover.Alpha(0.72)
}

func borderColor(theme *ui.Theme) ui.Color {
	return theme.Border.Alpha(0.72)
}

func dividerColor(theme *ui.Theme) ui.Color {
	return theme.Border.Alpha(0.52)
}

func primaryText(theme *ui.Theme) ui.Color {
	return theme.Text
}

func secondaryText(theme *ui.Theme) ui.Color {
	return theme.TextMuted
}

// pageHeader keeps page title and description aligned across settings and
// device detail panes.
func pageHeader(c *ui.Context, theme *ui.Theme, title, description string) {
	ui.Column(c).Shrink(0).Gap(space(theme, spaceInline)).Children(func() {
		ui.Text(c, title).FontSize(font(theme, fontPageTitle)).FontWeight(600).SingleLine()
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

func neutralButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(space(theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Radius(radius(theme, controlRadius)).
		TextColor(primaryText(theme)).Label(label)
	if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	return button
}

// dangerButton 用于会删除数据或解除关联的操作，保持文字和悬停反馈为危险色。
func dangerButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(space(theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Radius(radius(theme, controlRadius)).
		TextColor(theme.Danger).Label(label)
	if button.Hovered() {
		button.Background(theme.Danger.Alpha(0.12))
	} else {
		button.Background(ui.Transparent)
	}
	return button
}

// dangerConfirmButton 是危险确认弹窗中的主操作按钮。
func dangerConfirmButton(c *ui.Context, theme *ui.Theme, key, label string) ui.Element {
	button := ui.ButtonBase(c.Key(key)).Height(space(theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Radius(radius(theme, controlRadius)).
		Background(theme.Danger).TextColor(theme.AccentText).Label(label)
	if button.Hovered() {
		button.Background(theme.Danger.Mix(theme.Text, 0.12))
	}
	return button
}

// dangerConfirmDialog 提供统一的危险操作二次确认界面。
func dangerConfirmDialog(
	c *ui.Context,
	theme *ui.Theme,
	open *bool,
	title string,
	message string,
	confirmLabel string,
) bool {
	if !*open {
		return false
	}

	confirmed := false
	ui.Modal(c, open, func() {
		ui.Column(c).Width(440).Shrink(0).
			Padding(space(theme, 5), space(theme, 5)).Gap(space(theme, 3)).
			Radius(radius(theme, largeRadius)).Background(theme.Background).
			Border(1, borderColor(theme)).Label(title).Children(func() {
			ui.Text(c, title).FontSize(font(theme, fontSection)).FontWeight(600).SingleLine()
			ui.Text(c, message).FontSize(font(theme, fontBody)).
				TextColor(secondaryText(theme)).Wrap()
			ui.Row(c).FillWidth().Shrink(0).Justify(ui.End).Gap(space(theme, 2)).Children(func() {
				cancel := neutralButton(c, theme, "danger-confirm-cancel", "取消")
				cancel.Children(func() {
					ui.Text(c, "取消").FontSize(font(theme, fontControl)).SingleLine()
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
