package main

import (
	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

type settingsNavItem struct {
	area  settingsArea
	icon  *ui.SVG
	key   string
	title string
}

// settingsPage 只渲染详情。设置分栏是侧栏的一部分，见 filterRail 的 sidebarSettings。
func (a *application) settingsPage(c *ui.Context, theme *ui.Theme) {
	a.settingsDetail(c, theme)
}

func (a *application) settingsNavigationItems() []settingsNavItem {
	return []settingsNavItem{
		{area: settingsGeneral, icon: icons.settings, key: "general", title: a.t(msgSettingsGeneral)},
		{area: settingsAppearance, icon: icons.appearance, key: "appearance", title: a.t(msgSettingsAppearance)},
		{area: settingsStorage, icon: icons.storage, key: "storage", title: a.t(msgSettingsStorage)},
		{area: settingsAbout, icon: icons.about, key: "about", title: a.t(msgSettingsAbout)},
	}
}

// settingsNavButton 渲染设置分栏的一个入口：单行条目的规格由 sidebarItemButton
// 统一提供，与历史筛选项逐项一致（高度、内边距、图标、间距、选中态）。
func (a *application) settingsNavButton(c *ui.Context, theme *ui.Theme, item settingsNavItem) {
	sidebarItemButton(c, theme, "settings-nav-"+item.key, item.icon, item.title,
		a.settingsArea == item.area, func() { a.settingsArea = item.area })
}

func (a *application) settingsDetail(c *ui.Context, theme *ui.Theme) {
	// Row 默认会把未指定高度的子元素垂直居中。设置内容需要从顶部开始，
	// 外层列显式拉伸高度，避免滚动容器按内容高度参与行布局而被居中。
	ui.Column(c).Grow(1).FillHeight().AlignSelf(ui.Stretch).
		MinWidth(0).MinHeight(0).Children(func() {
		paneTop(c)
		ui.Scroll(c).Fill().MinWidth(0).MinHeight(0).
			ScrollbarInsets(0, space(theme, 1), 0, 0).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(detailMaxWidth(theme)).AlignSelf(ui.Center).
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
	a.settingsHeader(c, a.t(msgSettingsGeneral), a.t(msgSettingsGeneralDesc), theme)
	a.settingsSubheading(c, a.t(msgSettingsBasics), theme)
	settingsGroup(c, theme, "settings-general-group", func() {
		a.settingsRow(c, theme, a.t(msgSettingsDeviceName), a.t(msgSettingsDeviceNameDesc), func() {
			input := ui.TextInput(c.Key("settings-device-name"), &a.deviceName).
				Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).
				Padding(0, space(theme, 2.5)).Radius(radius(theme, controlRadius)).
				Background(theme.Background).Border(1, borderColor(theme)).
				Label(a.t(msgSettingsDeviceName)).Placeholder(a.t(msgSettingsDeviceNamePlaceholder))
			if a.compactDetailLayout(c, theme) {
				input.FillWidth()
			} else {
				input.Width(220)
			}
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, a.t(msgSettingsLaunchAtLogin), a.t(msgSettingsLaunchAtLoginDesc), func() {
			control := ui.Checkbox(c.Key("settings-launch-at-login"), &a.launchAtLogin, "").
				Shrink(0).Label(a.t(msgSettingsLaunchAtLogin))
			if control.Changed() {
				c.Toast(a.updateLaunchAtLogin())
			}
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, a.t(msgSettingsLanguage), a.t(msgSettingsLanguageDesc), func() {
			languageControl := selectField(c, theme, "settings-language", &a.languagePreference, a.t(msgSettingsLanguage), a.l10n().languageChoices())
			if a.compactDetailLayout(c, theme) {
				languageControl.FillWidth()
			} else {
				languageControl.Width(180)
			}
			if languageControl.Changed() {
				a.applyLanguage()
			}
		})
		settingsRule(c, theme)
		// 「不再询问」的回头路：弹窗里勾选后删除不再确认，这个开关把它打开。
		// 它只管单条记录的删除，清空历史与取消配对始终确认。
		a.settingsRow(c, theme, a.t(msgSettingsConfirmDelete), a.t(msgSettingsConfirmDeleteDesc), func() {
			ui.Switch(c.Key("settings-confirm-delete"), &a.confirmBeforeDelete).
				Shrink(0).Label(a.t(msgSettingsConfirmDelete))
		})
	})
}

func (a *application) appearanceSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, a.t(msgSettingsAppearance), a.t(msgSettingsAppearanceDesc), theme)
	a.settingsSubheading(c, a.t(msgSettingsVisualStyle), theme)
	settingsGroup(c, theme, "settings-appearance-group", func() {
		a.settingsRow(c, theme, a.t(msgSettingsMode), a.t(msgSettingsModeDesc), func() {
			control := segmentedField(c, theme, "settings-appearance", &a.appearance, a.t(msgSettingsAppearance), []string{
				a.appearanceModeLabel(appearanceSystem),
				a.appearanceModeLabel(appearanceLight),
				a.appearanceModeLabel(appearanceDark),
			}).Height(controlHeightSize(c, theme, controlHeight))
			if a.compactDetailLayout(c, theme) {
				control.FillWidth()
			} else {
				control.Width(260)
			}
			if control.Changed() {
				a.applyAppearance()
			}
		})
	})
	ui.Text(c, a.tf(msgSettingsCurrentAppearance, a.appearanceLabel(a.appearance))).FontSize(font(theme, fontMeta)).
		TextColor(secondaryText(theme)).Padding(space(theme, 1), 0, 0, 0).Wrap()

	a.settingsSubheading(c, a.t(msgSettingsInterface), theme)
	settingsGroup(c, theme, "settings-zoom-group", func() {
		a.settingsRow(c, theme, a.t(msgSettingsZoom), a.t(msgSettingsZoomDesc), func() {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).Children(func() {
				out := neutralButton(c, theme, "settings-zoom-out", a.t(msgSettingsZoomOut))
				out.Children(func() {
					labelText(c, a.t(msgSettingsZoomOut)).FontSize(font(theme, fontControl))
				})
				if out.Clicked() {
					a.zoomOut()
				}
				labelText(c, zoomLabel(a.zoom)).FontSize(font(theme, fontControl)).FontWeight(600).
					TextColor(primaryText(theme)).MinWidth(space(theme, 9)).Center()
				in := neutralButton(c, theme, "settings-zoom-in", a.t(msgSettingsZoomIn))
				in.Children(func() {
					labelText(c, a.t(msgSettingsZoomIn)).FontSize(font(theme, fontControl))
				})
				if in.Clicked() {
					a.zoomIn()
				}
				// 已经在 100% 时没有可重置的东西，按钮置灰。
				reset := neutralButton(c, theme, "settings-zoom-reset", a.t(msgSettingsZoomReset)).
					Disabled(a.zoom == 1)
				reset.Children(func() {
					labelText(c, a.t(msgSettingsZoomReset)).FontSize(font(theme, fontControl))
				})
				if reset.Clicked() {
					a.resetZoom()
				}
			})
		})
	})
}

func (a *application) appearanceLabel(appearance int) string {
	switch appearance {
	case appearanceLight:
		return a.t(msgSettingsThemeLightName)
	case appearanceDark:
		return a.t(msgSettingsThemeDarkName)
	default:
		return a.t(msgSettingsThemeSystemName)
	}
}

func (a *application) storageSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, a.t(msgSettingsStorage), a.t(msgSettingsStorageDesc), theme)
	a.settingsSubheading(c, a.t(msgSettingsData), theme)
	stats := a.history.Snapshot(history.Query{})
	settingsGroup(c, theme, "settings-storage-group", func() {
		a.settingsRow(c, theme, a.t(msgSettingsHistoryRow), a.t(msgSettingsHistoryRowDesc), func() {
			ui.Column(c).Shrink(0).AlignItems(ui.End).Gap(space(theme, 0.5)).Children(func() {
				labelText(c, a.l10n().storageRecord(stats.Total)).FontSize(font(theme, fontControl)).
					TextColor(primaryText(theme))
				labelText(c, a.l10n().imageCount(stats.ImageCount)).FontSize(font(theme, fontCaption)).
					TextColor(secondaryText(theme))
			})
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, a.t(msgSettingsClearHistoryRow), a.t(msgSettingsClearHistoryRowDesc), func() {
			clearLabel := a.storageClearLabel(stats.Total)
			button := dangerButton(c, theme, "settings-clear-history", clearLabel).
				Gap(space(theme, spaceInline)).Disabled(stats.Total == 0)
			button.Children(func() {
				ui.Icon(c, icons.clear).Size(space(theme, 3.5), space(theme, 3.5)).
					Shrink(0).TextColor(theme.Danger)
				labelText(c, clearLabel).FontSize(font(theme, fontControl))
			})
			if button.Clicked() && stats.Total > 0 {
				a.requestDangerousAction(dangerousActionClearHistory, "")
			}
		})
	})
	if stats.Total == 0 {
		ui.Text(c, a.t(msgSettingsNoHistoryNote)).
			FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).
			Padding(space(theme, 1), 0, 0, 0).Wrap()
	}
}

// storageClearLabel 根据是否有记录返回清理按钮的文案。
func (a *application) storageClearLabel(total int) string {
	if total == 0 {
		return a.t(msgSettingsNothingToClear)
	}
	return a.t(msgSettingsClearHistory)
}

func (a *application) aboutSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, a.t(msgSettingsAbout), a.t(msgSettingsAboutDesc), theme)
	a.settingsSubheading(c, a.t(msgSettingsAppInfo), theme)
	settingsGroup(c, theme, "settings-about-group", func() {
		a.settingsRow(c, theme, a.t(msgSettingsVersion), a.t(msgSettingsVersionDesc), func() {
			labelText(c, settingsVersion).FontSize(font(theme, fontControl)).TextColor(secondaryText(theme))
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, a.t(msgSettingsSyncScope), a.t(msgSettingsSyncScopeDesc), func() {
			labelText(c, a.t(msgDevicesDirect)).FontSize(font(theme, fontControl)).
				TextColor(theme.Success)
		})
		settingsRule(c, theme)
		a.settingsRow(c, theme, a.t(msgSettingsPrivacy), a.t(msgSettingsPrivacyDesc), func() {
			labelText(c, a.t(msgSettingsStoredLocally)).FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme))
		})
	})
}

func (a *application) settingsSubheading(c *ui.Context, title string, theme *ui.Theme) {
	labelText(c, title).FontSize(font(theme, fontSubheading)).FontWeight(600).
		TextColor(secondaryText(theme)).Padding(0, 0, space(theme, 1), 0)
}

func settingsGroup(c *ui.Context, theme *ui.Theme, key string, body func()) {
	standardCard(c, theme, key, body)
}

func settingsRule(c *ui.Context, theme *ui.Theme) {
	standardDivider(c, theme)
}

func (a *application) settingsRow(c *ui.Context, theme *ui.Theme, title, description string, control func()) {
	if a.compactDetailLayout(c, theme) {
		ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 2)).
			Padding(space(theme, 3), space(theme, 4)).Children(func() {
			ui.Column(c).FillWidth().MinWidth(0).Gap(space(theme, 0.75)).Children(func() {
				labelText(c, title).FontSize(font(theme, fontBody)).FontWeight(500)
				ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
			})
			control()
		})
		return
	}
	ui.Row(c).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).Gap(space(theme, 4.5)).
		Padding(space(theme, 3), space(theme, 4)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.75)).Children(func() {
			labelText(c, title).FontSize(font(theme, fontBody)).FontWeight(500)
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		control()
	})
}
