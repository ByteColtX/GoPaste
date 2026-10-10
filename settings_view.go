package main

import (
	"github.com/egoist/mygo/ui"
)

// settingsSidebarWidth 保持设置导航紧凑，让内容区域成为设置页的视觉重点。
// 设置页使用三段式结构：应用导航、设置导航、设置内容。
const settingsSidebarWidth = 192

type settingsNavItem struct {
	area  settingsArea
	icon  *ui.SVG
	title string
}

// settingsPage 使用参考图中的二级导航布局：左侧定位设置主题，右侧展示当前内容。
func (a *application) settingsPage(c *ui.Context, theme *ui.Theme) {
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		a.settingsSidebar(c, theme)
		ui.Box(c).Width(1).FillHeight().Shrink(0).
			Background(theme.Border.Alpha(0.42))
		a.settingsDetail(c, theme)
	})

	if choice := ui.AlertDialog(
		c,
		&a.confirmClearHistory,
		"清空历史记录？",
		"当前演示记录会被全部移除。",
		"取消",
		"清空",
	); choice == 1 {
		a.clearHistory()
	}
}

func (a *application) settingsSidebar(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Width(settingsSidebarWidth).FillHeight().Shrink(0).
		Padding(24, 16, 20, 16).Gap(10).
		Background(theme.Surface.Alpha(0.22)).Children(func() {
		ui.Text(c, "设置").FontSize(17).FontWeight(600).
			Padding(2, 4, 2, 4).SingleLine()

		ui.Text(c, "应用").FontSize(11).FontWeight(600).
			TextColor(theme.TextMuted).Padding(9, 8, 1, 8).SingleLine()
		ui.Column(c).Gap(2).Children(func() {
			for _, item := range settingsNavigationItems() {
				a.settingsNavButton(c, theme, item)
			}
		})

		ui.Spacer(c)
		ui.Textf(c, "GoPaste %s", settingsVersion).FontSize(11).
			TextColor(theme.TextMuted).Padding(4, 8).SingleLine()
	})
}

func settingsNavigationItems() []settingsNavItem {
	return []settingsNavItem{
		{area: settingsGeneral, icon: icons.devices, title: "常规"},
		{area: settingsAppearance, icon: icons.settings, title: "外观"},
		{area: settingsStorage, icon: icons.file, title: "存储"},
		{area: settingsAbout, icon: icons.history, title: "关于"},
	}
}

func (a *application) settingsNavButton(c *ui.Context, theme *ui.Theme, item settingsNavItem) {
	button := ui.ButtonBase(c.Key("settings-nav-"+item.title)).
		Height(32).FillWidth().Shrink(0).Padding(5, 8).Radius(8).
		TextColor(theme.TextMuted).Label(item.title).Tooltip(item.title)
	if a.settingsArea == item.area {
		button.Background(theme.Text.Alpha(0.09)).TextColor(theme.Text)
	} else if button.Hovered() {
		button.Background(theme.Text.Alpha(0.06))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Icon(c, item.icon).Size(15, 15).Shrink(0)
			ui.Text(c, item.title).FontSize(12).SingleLine()
		})
	})
	if button.Clicked() {
		a.settingsArea = item.area
		a.settingsMessage = ""
	}
}

func (a *application) settingsDetail(c *ui.Context, theme *ui.Theme) {
	// Row 默认会把未指定高度的子元素垂直居中。设置内容需要从顶部开始，
	// 外层列显式拉伸高度，避免滚动容器按内容高度参与行布局而被居中。
	ui.Column(c).Grow(1).FillHeight().AlignSelf(ui.Stretch).
		MinWidth(0).MinHeight(0).Children(func() {
		ui.Scroll(c).Fill().MinWidth(0).MinHeight(0).
			ScrollbarInsets(0, 4, 0, 0).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(600).AlignSelf(ui.Center).
				Padding(46, 48, 56, 48).Gap(10).Children(func() {
				switch a.settingsArea {
				case settingsAppearance:
					a.appearanceSettings(c, theme)
				case settingsStorage:
					a.storageSettings(c, theme)
				case settingsAbout:
					a.aboutSettings(c, theme)
				default:
					a.generalSettings(c, theme)
				}
			})
		})
	})
}

func (a *application) settingsHeader(c *ui.Context, title, description string, theme *ui.Theme) {
	ui.Column(c).Shrink(0).Gap(4).Padding(0, 0, 12, 0).Children(func() {
		ui.Text(c, title).FontSize(23).FontWeight(600).SingleLine()
		ui.Text(c, description).FontSize(13).TextColor(theme.TextMuted).Wrap()
	})
}

func (a *application) generalSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "常规", "设备身份和 GoPaste 的基础使用偏好。", theme)
	a.settingsSubheading(c, "基本设置", theme)
	settingsGroup(c, theme, func() {
		a.settingsRow(c, theme, "设备名称", "用于在设备列表中识别本机", func() {
			ui.TextInput(c.Key("settings-device-name"), &a.deviceName).
				Width(220).Height(32).Shrink(0).Padding(0, 10).Radius(7).
				Background(theme.Surface).Border(1, theme.Border).
				Label("设备名称").Placeholder("输入设备名称")
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "开机自启", "登录系统后自动打开 GoPaste", func() {
			control := ui.Checkbox(c.Key("settings-launch-at-login"), &a.launchAtLogin, "").
				Shrink(0).Label("开机自启")
			if control.Changed() {
				a.updateLaunchAtLogin()
			}
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "语言", "选择应用界面语言", func() {
			ui.Select(c.Key("settings-language"), &a.language, languageOptions).
				Width(180).Shrink(0).Label("语言")
		})
	})
	a.settingsMessageView(c, theme)
}

func (a *application) appearanceSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "外观", "选择 GoPaste 使用的颜色主题。", theme)
	a.settingsSubheading(c, "视觉风格", theme)
	settingsGroup(c, theme, func() {
		a.settingsRow(c, theme, "模式", "跟随系统或固定使用浅色、深色主题", func() {
			control := ui.Segmented(
				c.Key("settings-appearance"),
				&a.appearance,
				"跟随系统",
				"浅色",
				"深色",
			).Width(260).Shrink(0).Label("外观")
			if control.Changed() {
				a.applyAppearance()
			}
		})
	})
	a.settingsMessageView(c, theme)
}

func (a *application) storageSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "存储", "查看和清理保存在本机的历史记录。", theme)
	a.settingsSubheading(c, "数据管理", theme)
	settingsGroup(c, theme, func() {
		a.settingsRow(c, theme, "历史记录", "保存在本机的剪贴板记录", func() {
			ui.Column(c).Shrink(0).AlignItems(ui.End).Gap(2).Children(func() {
				ui.Textf(c, "%d 条", len(a.items)).FontSize(13).TextColor(theme.Text).SingleLine()
				ui.Textf(c, "%d 张图片", a.imageItemCount()).FontSize(11).
					TextColor(theme.TextMuted).SingleLine()
			})
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "清理历史记录", "删除当前保存的所有本地记录", func() {
			button := ui.ButtonBase(c.Key("settings-clear-history")).
				Height(30).Shrink(0).Padding(4, 10).Radius(7).
				TextColor(theme.TextMuted).Label("清空历史记录")
			if button.Hovered() {
				button.Background(theme.Text.Alpha(0.07))
			} else {
				button.Background(ui.Transparent)
			}
			button.Children(func() {
				ui.Icon(c, icons.clear).Size(14, 14).Shrink(0)
				ui.Text(c, "清空历史记录").FontSize(12).SingleLine()
			})
			if button.Clicked() {
				a.confirmClearHistory = true
			}
		})
	})
	a.settingsMessageView(c, theme)
}

func (a *application) aboutSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "关于", "GoPaste 剪贴板历史记录工具。", theme)
	a.settingsSubheading(c, "应用信息", theme)
	settingsGroup(c, theme, func() {
		a.settingsRow(c, theme, "版本", "当前应用版本", func() {
			ui.Text(c, settingsVersion).FontSize(13).TextColor(theme.TextMuted).SingleLine()
		})
	})
}

func (a *application) settingsSubheading(c *ui.Context, title string, theme *ui.Theme) {
	ui.Text(c, title).FontSize(12).FontWeight(600).
		TextColor(theme.TextMuted).Padding(10, 0, 4, 0).SingleLine()
}

func (a *application) settingsMessageView(c *ui.Context, theme *ui.Theme) {
	if a.settingsMessage == "" {
		return
	}
	ui.Text(c, a.settingsMessage).FontSize(12).TextColor(theme.Accent).
		Padding(4, 2).Wrap()
}

func settingsGroup(c *ui.Context, theme *ui.Theme, body func()) {
	ui.Column(c).FillWidth().Gap(0).Radius(12).
		Background(theme.Surface.Alpha(0.18)).
		Border(1, theme.Border.Alpha(0.56)).Children(body)
}

func settingsRule(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).Height(1).FillWidth().Shrink(0).
		Background(theme.Border.Alpha(0.38))
}

func (a *application) settingsRow(c *ui.Context, theme *ui.Theme, title, description string, control func()) {
	ui.Row(c).FillWidth().MinHeight(58).Shrink(0).Gap(18).
		Padding(11, 16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, title).FontSize(13).SingleLine()
			ui.Text(c, description).FontSize(11).TextColor(theme.TextMuted).Wrap()
		})
		control()
	})
}
