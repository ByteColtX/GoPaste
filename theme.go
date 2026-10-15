package main

import "github.com/egoist/mygo/ui"

// GoPaste 的视觉语言在 MyGo 默认主题之上做了一层收敛：中性色更安静、边框更细、
// 圆角略大。它始终以当前帧的主题为基础，因此深色模式、系统强调色、高对比度和
// 文字缩放都继续生效，只调整中性色角色和几何尺寸。
//
// 说明：这里覆盖的是“画布/表面/边框/文字”这类中性角色；强调色、成功、警告、
// 危险色仍沿用系统主题，保证与平台一致。

// applyTheme 把当前帧主题收敛成 GoPaste 的主题，并写回帧上下文。
// 返回收敛后的主题，供调用方在同一帧内继续使用。
func (a *application) applyTheme(c *ui.Context) *ui.Theme {
	base := c.Theme()
	if base == nil {
		return base
	}

	t := *base
	if t.Dark {
		t.Background = ui.Hex("#1c1c1e")
		t.Surface = ui.Hex("#242426")
		t.SurfaceHover = ui.Hex("#2e2e31")
		t.SurfacePressed = ui.Hex("#38383c")
		t.Border = ui.Hex("#34343a")
		t.Text = ui.Hex("#f2f2f4")
		t.TextMuted = ui.Hex("#98989e")
	} else {
		t.Background = ui.Hex("#ffffff")
		t.Surface = ui.Hex("#f5f5f7")
		t.SurfaceHover = ui.Hex("#ececef")
		t.SurfacePressed = ui.Hex("#e1e1e5")
		t.Border = ui.Hex("#e5e5e9")
		t.Text = ui.Hex("#1d1d1f")
		t.TextMuted = ui.Hex("#86868b")
	}

	// 高对比度偏好：加粗边框、加深次级文字，并把焦点环设为不透明。
	if c.Preferences().HighContrast {
		t.Border = t.Border.Mix(t.Text, 0.45)
		t.TextMuted = t.TextMuted.Mix(t.Text, 0.4)
		t.Focus.A = 255
	}

	// 界面缩放作用在字号与 4 DIP 网格上，所有 font()/space() 取值一起等比变化。
	a.applyZoom(&t)

	t.Radius = 8
	a.theme = t
	c.SetTheme(&a.theme)
	return &a.theme
}
