package main

import "github.com/egoist/mygo/ui"

// GoPaste 的原生界面使用一组共享的尺寸和颜色角色。页面可以有不同的
// 信息密度，但同一层级的标题、说明、控件和卡片都从这里取值。
const (
	fontPageTitle float32 = 1.75
	fontSection   float32 = 1.125
	fontBody      float32 = 1
	fontControl   float32 = 0.875
	fontMeta      float32 = 0.8125
	fontCaption   float32 = 0.75
	fontCode      float32 = 0.9375

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
	secondarySidebarWidth = 224
	pageDetailMaxWidth    = 720
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

func statusDot(c *ui.Context, color ui.Color, size float32) {
	ui.Box(c).Size(size, size).Shrink(0).Radius(size / 2).Background(color)
}
