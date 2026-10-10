package main

import (
	"github.com/egoist/mygo/ui"
)

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
			Background(dividerColor(theme))
		a.settingsDetail(c, theme)
	})

	if choice := ui.AlertDialog(
		c,
		&a.confirmClearHistory,
		"清空历史记录？",
		"当前保存的历史记录会被全部移除。",
		"取消",
		"清空",
	); choice == 1 {
		a.clearHistory()
		c.Toast("历史记录已清空")
	}
}

func (a *application) settingsSidebar(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Width(secondarySidebarWidth).FillHeight().Shrink(0).
		Padding(space(theme, 6), space(theme, 4), space(theme, 5), space(theme, 4)).
		Gap(space(theme, spaceControl)).Background(sidebarFill(theme)).Children(func() {
		ui.Text(c, "设置").FontSize(font(theme, fontSection)).FontWeight(600).
			Padding(space(theme, 0.5), space(theme, 1), space(theme, 0.5), space(theme, 1)).SingleLine()

		ui.Text(c, "应用").FontSize(font(theme, fontSubheading)).FontWeight(600).
			TextColor(secondaryText(theme)).Padding(space(theme, 2.25), space(theme, 2), space(theme, 0.25), space(theme, 2)).SingleLine()
		ui.Column(c).Gap(space(theme, 0.5)).Children(func() {
			for _, item := range settingsNavigationItems() {
				a.settingsNavButton(c, theme, item)
			}
		})

		ui.Spacer(c)
		ui.Textf(c, "GoPaste %s", settingsVersion).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).Padding(space(theme, 1), space(theme, 2)).SingleLine()
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
		Height(space(theme, controlHeight)).FillWidth().Shrink(0).
		Padding(space(theme, 1.25), space(theme, 2)).Radius(radius(theme, controlRadius)).
		TextColor(secondaryText(theme)).Label(item.title).Tooltip(item.title)
	if a.settingsArea == item.area {
		button.Background(selectedFill(theme)).TextColor(primaryText(theme))
	} else if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(space(theme, 2)).Children(func() {
			ui.Icon(c, item.icon).Size(space(theme, 4), space(theme, 4)).Shrink(0)
			ui.Text(c, item.title).FontSize(font(theme, fontControl)).SingleLine()
		})
	})
	if button.Clicked() {
		a.settingsArea = item.area
	}
}

func (a *application) settingsDetail(c *ui.Context, theme *ui.Theme) {
	// Row 默认会把未指定高度的子元素垂直居中。设置内容需要从顶部开始，
	// 外层列显式拉伸高度，避免滚动容器按内容高度参与行布局而被居中。
	ui.Column(c).Grow(1).FillHeight().AlignSelf(ui.Stretch).
		MinWidth(0).MinHeight(0).Children(func() {
		ui.Scroll(c).Fill().MinWidth(0).MinHeight(0).
			ScrollbarInsets(0, space(theme, 1), 0, 0).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(pageDetailMaxWidth).AlignSelf(ui.Center).
				Padding(space(theme, spacePageTop), space(theme, spacePageSide), space(theme, spacePageBottom), space(theme, spacePageSide)).
				Gap(space(theme, spaceSection)).Children(func() {
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
	pageHeader(c, theme, title, description)
}

func (a *application) generalSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "常规", "设备身份和 GoPaste 的基础使用偏好。", theme)
	a.settingsSubheading(c, "基本设置", theme)
	settingsGroup(c, theme, "settings-general-group", func() {
		a.settingsRow(c, theme, "设备名称", "用于在设备列表中识别本机", func() {
			ui.TextInput(c.Key("settings-device-name"), &a.deviceName).
				Width(220).Height(space(theme, controlHeight)).Shrink(0).
				Padding(0, space(theme, 2.5)).Radius(radius(theme, controlRadius)).
				Background(theme.Surface).Border(1, borderColor(theme)).
				Label("设备名称").Placeholder("输入设备名称")
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "开机自启", "登录系统后自动打开 GoPaste", func() {
			control := ui.Checkbox(c.Key("settings-launch-at-login"), &a.launchAtLogin, "").
				Shrink(0).Label("开机自启")
			if control.Changed() {
				c.Toast(a.updateLaunchAtLogin())
			}
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "语言", "选择应用界面语言", func() {
			ui.Select(c.Key("settings-language"), &a.language, languageOptions).
				Width(180).Height(space(theme, controlHeight)).Shrink(0).Label("语言")
		})
	})
}

func (a *application) appearanceSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "外观", "选择 GoPaste 使用的颜色主题。", theme)
	a.settingsSubheading(c, "视觉风格", theme)
	settingsGroup(c, theme, "settings-appearance-group", func() {
		a.settingsRow(c, theme, "模式", "跟随系统或固定使用浅色、深色主题", func() {
			control := ui.Segmented(
				c.Key("settings-appearance"),
				&a.appearance,
				"跟随系统",
				"浅色",
				"深色",
			).Width(260).Height(space(theme, controlHeight)).Shrink(0).Label("外观")
			if control.Changed() {
				a.applyAppearance()
			}
		})
	})
}

func (a *application) storageSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "存储", "查看和清理保存在本机的历史记录。", theme)
	a.settingsSubheading(c, "数据管理", theme)
	settingsGroup(c, theme, "settings-storage-group", func() {
		a.settingsRow(c, theme, "历史记录", "保存在本机的剪贴板记录", func() {
			ui.Column(c).Shrink(0).AlignItems(ui.End).Gap(space(theme, 0.5)).Children(func() {
				ui.Textf(c, "%d 条", len(a.items)).FontSize(font(theme, fontControl)).TextColor(primaryText(theme)).SingleLine()
				ui.Textf(c, "%d 张图片", a.imageItemCount()).FontSize(font(theme, fontCaption)).
					TextColor(secondaryText(theme)).SingleLine()
			})
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "清理历史记录", "删除当前保存的所有本地记录", func() {
			button := ui.ButtonBase(c.Key("settings-clear-history")).
				Height(space(theme, compactHeight)).Shrink(0).
				Padding(space(theme, 1), space(theme, 2.5)).Gap(space(theme, spaceInline)).
				Radius(radius(theme, controlRadius)).
				TextColor(secondaryText(theme)).Label("清空历史记录")
			if button.Hovered() {
				button.Background(hoverFill(theme))
			} else {
				button.Background(ui.Transparent)
			}
			button.Children(func() {
				ui.Icon(c, icons.clear).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0)
				ui.Text(c, "清空历史记录").FontSize(font(theme, fontControl)).SingleLine()
			})
			if button.Clicked() {
				a.confirmClearHistory = true
			}
		})
	})
}

func (a *application) aboutSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "关于", "GoPaste 剪贴板历史记录工具。", theme)
	a.settingsSubheading(c, "应用信息", theme)
	settingsGroup(c, theme, "settings-about-group", func() {
		a.settingsRow(c, theme, "版本", "当前应用版本", func() {
			ui.Text(c, settingsVersion).FontSize(font(theme, fontControl)).TextColor(secondaryText(theme)).SingleLine()
		})
	})
}

func (a *application) settingsSubheading(c *ui.Context, title string, theme *ui.Theme) {
	ui.Text(c, title).FontSize(font(theme, fontSubheading)).FontWeight(600).
		TextColor(secondaryText(theme)).Padding(0, 0, space(theme, 1), 0).SingleLine()
}

func settingsGroup(c *ui.Context, theme *ui.Theme, key string, body func()) {
	standardCard(c, theme, key, body)
}

func settingsRule(c *ui.Context, theme *ui.Theme) {
	standardDivider(c, theme)
}

func (a *application) settingsRow(c *ui.Context, theme *ui.Theme, title, description string, control func()) {
	ui.Row(c).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).Gap(space(theme, 4.5)).
		Padding(space(theme, 3), space(theme, 4)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.75)).Children(func() {
			ui.Text(c, title).FontSize(font(theme, fontBody)).SingleLine()
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		control()
	})
}
