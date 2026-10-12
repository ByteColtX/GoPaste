package main

import (
	"fmt"

	"gopaste/internal/history"

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
}

func (a *application) settingsSidebar(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Width(secondarySidebarSize(c, theme)).FillHeight().Shrink(0).
		Padding(space(theme, columnTopInset), space(theme, 4), space(theme, 5), space(theme, 4)).
		Gap(space(theme, spaceControl)).Background(sidebarFill(theme)).Children(func() {
		ui.Text(c, "设置").FontSize(font(theme, fontSection)).FontWeight(600).
			Padding(space(theme, 0.5), space(theme, 1), space(theme, 0.5), space(theme, 1)).SingleLine()

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
		{area: settingsGeneral, icon: icons.settings, title: "通用"},
		{area: settingsAppearance, icon: icons.appearance, title: "外观"},
		{area: settingsStorage, icon: icons.storage, title: "存储"},
		{area: settingsAbout, icon: icons.about, title: "关于"},
	}
}

func (a *application) settingsNavButton(c *ui.Context, theme *ui.Theme, item settingsNavItem) {
	button := ui.ButtonBase(c.Key("settings-nav-"+item.title)).
		Height(controlHeightSize(c, theme, controlHeight)).FillWidth().Shrink(0).
		Padding(space(theme, 1.25), space(theme, 2)).Radius(radius(theme, controlRadius)).
		Border(1, ui.Transparent).
		TextColor(secondaryText(theme)).Label(item.title).Tooltip(item.title).
		Role(ui.RoleToggleButton).Selected(a.settingsArea == item.area)
	if a.settingsArea == item.area {
		button.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme)).TextColor(primaryText(theme))
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
				Padding(space(theme, columnTopInset), space(theme, spacePageSide), space(theme, spacePageBottom), space(theme, spacePageSide)).
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
	a.settingsHeader(c, "通用", "设备身份和 GoPaste 的基础使用偏好。", theme)
	a.settingsSubheading(c, "基本设置", theme)
	settingsGroup(c, theme, "settings-general-group", func() {
		a.settingsRow(c, theme, "设备名称", "用于在设备列表中识别本机", func() {
			input := ui.TextInput(c.Key("settings-device-name"), &a.deviceName).
				Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).
				Padding(0, space(theme, 2.5)).Radius(radius(theme, controlRadius)).
				Background(theme.Surface).Border(1, borderColor(theme)).
				Label("设备名称").Placeholder("输入设备名称")
			if compactDetailLayout(c, theme) {
				input.FillWidth()
			} else {
				input.Width(220)
			}
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
			languageControl := ui.Select(c.Key("settings-language"), &a.language, languageOptions).
				Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).Label("语言")
			if compactDetailLayout(c, theme) {
				languageControl.FillWidth()
			} else {
				languageControl.Width(180)
			}
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
			).Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).Label("外观")
			if compactDetailLayout(c, theme) {
				control.FillWidth()
			} else {
				control.Width(260)
			}
			if control.Changed() {
				a.applyAppearance()
			}
		})
	})
	ui.Text(c, "当前使用："+appearanceLabel(a.appearance)).FontSize(font(theme, fontMeta)).
		TextColor(secondaryText(theme)).Padding(space(theme, 1), 0, 0, 0).Wrap()
}

func appearanceLabel(appearance int) string {
	switch appearance {
	case appearanceLight:
		return "浅色主题"
	case appearanceDark:
		return "深色主题"
	default:
		return "跟随系统"
	}
}

func (a *application) storageSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "存储", "查看和清理保存在本机的历史记录。", theme)
	a.settingsSubheading(c, "数据管理", theme)
	stats := a.history.Snapshot(history.Query{})
	settingsGroup(c, theme, "settings-storage-group", func() {
		a.settingsRow(c, theme, "历史记录", "保存在本机的剪贴板记录", func() {
			ui.Column(c).Shrink(0).AlignItems(ui.End).Gap(space(theme, 0.5)).Children(func() {
				ui.Text(c, storageRecordSummary(stats.Total)).FontSize(font(theme, fontControl)).
					TextColor(primaryText(theme)).SingleLine()
				ui.Textf(c, "%d 张图片", stats.ImageCount).FontSize(font(theme, fontCaption)).
					TextColor(secondaryText(theme)).SingleLine()
			})
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "清理历史记录", "删除当前保存的所有本地记录", func() {
			button := dangerButton(c, theme, "settings-clear-history", "清空历史记录").
				Gap(space(theme, spaceInline)).Disabled(stats.Total == 0)
			button.Children(func() {
				ui.Icon(c, icons.clear).Size(space(theme, 3.5), space(theme, 3.5)).
					Shrink(0).TextColor(theme.Danger)
				ui.Text(c, storageClearLabel(stats.Total)).FontSize(font(theme, fontControl)).SingleLine()
			})
			if button.Clicked() && stats.Total > 0 {
				a.requestDangerousAction(dangerousActionClearHistory, "")
			}
		})
	})
	if stats.Total == 0 {
		ui.Text(c, "暂无本机历史记录。复制内容后，记录会自动出现在历史页面。").
			FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).
			Padding(space(theme, 1), 0, 0, 0).Wrap()
	}
}

func storageRecordSummary(total int) string {
	if total == 0 {
		return "暂无记录"
	}
	return fmt.Sprintf("%d 条", total)
}

func storageClearLabel(total int) string {
	if total == 0 {
		return "没有可清理的记录"
	}
	return "清空历史记录"
}

func (a *application) aboutSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, "关于", "GoPaste 剪贴板历史记录工具。", theme)
	a.settingsSubheading(c, "应用信息", theme)
	settingsGroup(c, theme, "settings-about-group", func() {
		a.settingsRow(c, theme, "版本", "当前应用版本", func() {
			ui.Text(c, settingsVersion).FontSize(font(theme, fontControl)).TextColor(secondaryText(theme)).SingleLine()
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "同步范围", "只在已配对设备之间建立局域网直连", func() {
			ui.Text(c, "局域网直连").FontSize(font(theme, fontControl)).
				TextColor(theme.Success).SingleLine()
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, "隐私", "剪贴板内容保存在本机，不写入普通日志或第三方服务", func() {
			ui.Text(c, "本机保存").FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme)).SingleLine()
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
	if compactDetailLayout(c, theme) {
		ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 2)).
			Padding(space(theme, 3), space(theme, 4)).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(space(theme, 0.75)).Children(func() {
				ui.Text(c, title).FontSize(font(theme, fontBody)).SingleLine()
				ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
			})
			control()
		})
		return
	}
	ui.Row(c).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).Gap(space(theme, 4.5)).
		Padding(space(theme, 3), space(theme, 4)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.75)).Children(func() {
			ui.Text(c, title).FontSize(font(theme, fontBody)).SingleLine()
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		control()
	})
}
