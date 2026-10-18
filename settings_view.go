package main

import (
	"os"

	"gopaste/internal/history"

	"github.com/egoist/mygo"
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
		{area: settingsSync, icon: icons.lan, key: "sync", title: a.t(msgSettingsSync)},
		{area: settingsAppearance, icon: icons.appearance, key: "appearance", title: a.t(msgSettingsAppearance)},
		{area: settingsStorage, icon: icons.storage, key: "storage", title: a.t(msgSettingsStorage)},
		{area: settingsAbout, icon: icons.about, key: "about", title: a.t(msgSettingsAbout)},
	}
}

// settingsNavButton 渲染设置分栏的一个入口：单行条目的规格由 sidebarItemButton
// 统一提供，与历史筛选项逐项一致（高度、内边距、图标、间距、选中态）。
func (a *application) settingsNavButton(c *ui.Context, theme *ui.Theme, item settingsNavItem) {
	sidebarItemButton(c, theme, "settings-nav-"+item.key, item.icon, item.title,
		a.settingsArea == item.area, func() { a.setSettingsArea(item.area) })
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
				case settingsSync:
					a.syncSettings(c, theme)
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
			// 改完名字就把广播也换掉：否则局域网里还是旧名字，直到下次重启。名字同样
			// 要落盘，否则重启后会退回默认的主机名。
			if input.Changed() {
				a.deviceNameChanged()
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
				a.saveSettings()
			}
		})
		settingsRule(c, theme)
		// 「不再询问」的回头路：弹窗里勾选后删除不再确认，这个开关把它打开。
		// 它只管单条记录的删除，清空历史与取消配对始终确认。
		a.settingsRow(c, theme, a.t(msgSettingsConfirmDelete), a.t(msgSettingsConfirmDeleteDesc), func() {
			control := ui.Switch(c.Key("settings-confirm-delete"), &a.confirmBeforeDelete).
				Shrink(0).Label(a.t(msgSettingsConfirmDelete))
			if control.Changed() {
				a.saveSettings()
			}
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
				a.saveSettings()
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

// syncSettings 是设置 › 同步：总开关、自动发送，以及文件同步的整块参数。设备页
// 的本地卡与这里绑定同一份状态，两处都能改。
func (a *application) syncSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, a.t(msgSettingsSync), a.t(msgSettingsSyncDesc), theme)
	a.settingsSubheading(c, a.t(msgSettingsSyncSwitches), theme)
	settingsGroup(c, theme, "settings-sync-switches", func() {
		a.settingsRow(c, theme, a.t(msgSettingsSyncEnabled), a.t(msgSettingsSyncEnabledDesc), func() {
			control := ui.Switch(c.Key("settings-sync-enabled"), &a.syncEnabled).
				Shrink(0).Label(a.t(msgSettingsSyncEnabled))
			if control.Changed() {
				a.setSyncEnabled(a.syncEnabled)
			}
		})
		settingsRule(c, theme)
		// 自动发送只在同步开着时有意义：总开关关着时这一行置灰，而不是让它看起来
		// 还能改。
		a.settingsRow(c, theme, a.t(msgSettingsAutoSend), a.t(msgSettingsAutoSendDesc), func() {
			control := ui.Switch(c.Key("settings-auto-send"), &a.autoSendEnabled).
				Shrink(0).Disabled(!a.syncEnabled).Label(a.t(msgSettingsAutoSend))
			if control.Changed() {
				a.setAutoSendEnabled(a.autoSendEnabled)
			}
		})
	})

	a.settingsSubheading(c, a.t(msgSettingsSyncFiles), theme)
	settingsGroup(c, theme, "settings-file-sync-group", func() {
		fileSyncDescription := a.t(msgSettingsFileSyncEnabledDesc)
		if !a.syncEnabled {
			fileSyncDescription = a.tf(msgDevicesFileSyncDisabled, fileSyncDescription)
		}
		a.settingsRow(c, theme, a.t(msgSettingsFileSyncEnabled), fileSyncDescription, func() {
			control := ui.Switch(c.Key("settings-file-sync-enabled"), &a.fileSyncEnabled).
				Shrink(0).Disabled(!a.syncEnabled).Label(a.t(msgSettingsFileSyncEnabled))
			if control.Changed() {
				a.setFileSyncEnabled(a.fileSyncEnabled)
			}
		})
		settingsRule(c, theme)
		a.fileSyncDirRow(c, theme)
		settingsRule(c, theme)
		a.fileSyncLimitRow(c, theme)
		settingsRule(c, theme)
		a.fileQuotaRow(c, theme)
		settingsRule(c, theme)
		a.fileRetentionRow(c, theme)
		settingsRule(c, theme)
		a.fileAutoCleanRow(c, theme)
		settingsRule(c, theme)
		a.fileCacheUsageRow(c, theme)
	})
}

// fileSyncParamsDisabled 报告文件同步的那些参数是否该置灰：总开关或文件同步任一
// 关掉，改它们都不会生效。
func (a *application) fileSyncParamsDisabled() bool {
	return !a.syncEnabled || !a.fileSyncEnabled
}

// fileSyncDirRow 是「保存位置」一行：路径文本 + 选择目录 + 恢复默认。路径可能很长，
// 所以它按可用宽度截断；只有用户自己的路径才用 SingleLine。
func (a *application) fileSyncDirRow(c *ui.Context, theme *ui.Theme) {
	disabled := a.fileSyncParamsDisabled()
	path := a.receivedFilesDir
	if path == "" {
		path = a.t(msgSettingsFileSyncDirUnset)
	}
	a.settingsRow(c, theme, a.t(msgSettingsFileSyncDir), a.t(msgSettingsFileSyncDirDesc), func() {
		ui.Column(c).Shrink(0).Gap(space(theme, 1)).Children(func() {
			pathText := ui.Text(c, path).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme)).SingleLine()
			if a.compactDetailLayout(c, theme) {
				pathText.FillWidth()
			} else {
				pathText.Width(240)
			}
			ui.Row(c).Shrink(0).Gap(space(theme, 1)).Children(func() {
				chooseLabel := a.t(msgSettingsFileSyncDirChoose)
				choose := neutralButton(c, theme, "settings-file-sync-dir-choose", chooseLabel).Disabled(disabled)
				choose.Children(func() {
					labelText(c, chooseLabel).FontSize(font(theme, fontControl))
				})
				if choose.Clicked() {
					a.chooseFileSyncDir(c)
				}
				resetLabel := a.t(msgSettingsFileSyncDirReset)
				reset := neutralButton(c, theme, "settings-file-sync-dir-reset", resetLabel).
					Disabled(disabled || a.fileSyncDir == "")
				reset.Children(func() {
					labelText(c, resetLabel).FontSize(font(theme, fontControl))
				})
				if reset.Clicked() {
					a.resetFileSyncDir()
				}
			})
		})
	})
}

// chooseFileSyncDir 弹出系统目录选择框，把选中的目录变成保存位置。目录先建出来
// 并确认能写：选一个只读的位置只会让之后收到的文件全部「未送达」。
func (a *application) chooseFileSyncDir(c *ui.Context) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:             a.t(msgSettingsFileSyncDir),
		DefaultPath:       a.receivedFilesDir,
		ButtonLabel:       a.t(msgSettingsFileSyncDirChoose),
		Directory:         true,
		CreateDirectories: true,
	})
	if err != nil || len(paths) == 0 {
		return
	}
	dir := paths[0]
	if err := os.MkdirAll(dir, 0o700); err != nil {
		c.Toast(a.t(msgToastFileDirUnusable))
		return
	}
	a.setFileSyncDir(dir)
}

// fileSyncLimitRow 是单个文件项的体积上限，原来在储存空间页，现在跟其它文件同步
// 参数放在一起。
func (a *application) fileSyncLimitRow(c *ui.Context, theme *ui.Theme) {
	a.syncNumberRow(c, theme, "settings-file-sync-limit",
		a.t(msgSettingsFileSyncLimit), a.t(msgSettingsFileSyncLimitDesc),
		&a.fileSyncMaxMB, a.t(msgSettingsFileSyncLimitPlaceholder), a.t(msgSettingsFileSyncLimitUnit),
		a.fileSyncParamsDisabled(), func() { a.editFileSyncLimit(a.fileSyncMaxMB) })
}

// fileQuotaRow 是收到的文件最多占多少磁盘。
func (a *application) fileQuotaRow(c *ui.Context, theme *ui.Theme) {
	a.syncNumberRow(c, theme, "settings-file-quota",
		a.t(msgSettingsFileQuota), a.t(msgSettingsFileQuotaDesc),
		&a.fileQuotaMB, a.t(msgSettingsFileQuotaPlaceholder), a.t(msgSettingsFileQuotaUnit),
		a.fileSyncParamsDisabled(), func() { a.editFileQuota(a.fileQuotaMB) })
}

// syncNumberRow 渲染一行「单位是 MB 的数字输入」：上限与配额长得一样，只是绑的
// 字段不同。输入不合法时 apply 不改值，输入框里的字留着让用户接着改。
func (a *application) syncNumberRow(
	c *ui.Context,
	theme *ui.Theme,
	key, title, description string,
	value *string,
	placeholder, unit string,
	disabled bool,
	apply func(),
) {
	a.settingsRow(c, theme, title, description, func() {
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).Children(func() {
			input := ui.TextInput(c.Key(key), value).
				Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).
				Padding(0, space(theme, 2.5)).Radius(radius(theme, controlRadius)).
				Background(theme.Background).Border(1, borderColor(theme)).
				Label(title).Placeholder(placeholder).Disabled(disabled)
			if a.compactDetailLayout(c, theme) {
				input.FillWidth()
			} else {
				input.Width(110)
			}
			labelText(c, unit).FontSize(font(theme, fontControl)).TextColor(secondaryText(theme))
			// 改完立刻生效：正在运行的同步服务拿到新值，下一次投递或清理就按它判。
			if input.Changed() {
				apply()
			}
		})
	})
}

// fileRetentionRow 是收到的文件留多久；0 是「不限」。
func (a *application) fileRetentionRow(c *ui.Context, theme *ui.Theme) {
	a.settingsRow(c, theme, a.t(msgSettingsFileRetention), a.t(msgSettingsFileRetentionDesc), func() {
		control := selectField(c, theme, "settings-file-retention", &a.fileRetentionHours,
			a.t(msgSettingsFileRetention), a.l10n().fileRetentionChoices()).
			Disabled(a.fileSyncParamsDisabled())
		if a.compactDetailLayout(c, theme) {
			control.FillWidth()
		} else {
			control.Width(180)
		}
		if control.Changed() {
			a.editFileRetention(a.fileRetentionHours)
		}
	})
}

// fileCacheUsageRow 显示收到的文件当前占了多少盘，并提供「立即清理」：按当前的
// 期限与配额立刻跑一遍，不受自动清理开关影响。数字来自最近一次统计，渲染本身不碰
// 磁盘。
func (a *application) fileCacheUsageRow(c *ui.Context, theme *ui.Theme) {
	a.settingsRow(c, theme, a.t(msgSettingsFileCacheUsage), a.t(msgSettingsFileCacheUsageDesc), func() {
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2)).Children(func() {
			labelText(c, formatBytes(a.storageUsage.files)).FontSize(font(theme, fontControl)).
				TextColor(secondaryText(theme))
			cleanLabel := a.t(msgSettingsFileCacheClean)
			button := neutralButton(c, theme, "settings-file-cache-clean", cleanLabel).
				Disabled(a.fileCachePending)
			button.Children(func() {
				labelText(c, cleanLabel).FontSize(font(theme, fontControl))
			})
			if button.Clicked() {
				a.requestFileCacheCleanup(true)
			}
		})
	})
}

// fileAutoCleanRow 是自动清理：与保留期限绑定，期限为「不限」时没有可清理的东西，
// 开关因此置灰。
func (a *application) fileAutoCleanRow(c *ui.Context, theme *ui.Theme) {
	disabled := a.fileSyncParamsDisabled() || a.fileRetentionHours <= 0
	a.settingsRow(c, theme, a.t(msgSettingsFileAutoClean), a.t(msgSettingsFileAutoCleanDesc), func() {
		control := ui.Switch(c.Key("settings-file-auto-clean"), &a.fileAutoClean).
			Shrink(0).Disabled(disabled).Label(a.t(msgSettingsFileAutoClean))
		if control.Changed() {
			a.editFileAutoClean(a.fileAutoClean)
		}
	})
}

func (a *application) storageSettings(c *ui.Context, theme *ui.Theme) {
	a.settingsHeader(c, a.t(msgSettingsStorage), a.t(msgSettingsStorageDesc), theme)
	stats := a.history.Snapshot(history.Query{})
	// 用量一节只陈述现状：占了多少盘、存了多少条。它不套卡片，也不和设置行
	// 挤在一起——这里没有可以改的东西。
	a.settingsSubheading(c, a.t(msgSettingsStorageUsage), theme)
	a.storageUsageSection(c, theme, stats)
	// 设置行按用途分两张卡：保留策略，以及单独一张的清除历史。文件同步的参数
	// 都在设置 › 同步里，这里只管历史记录。破坏性操作不跟普通设置同卡，也不在
	// 页面中段。
	a.settingsSubheading(c, a.t(msgSettingsData), theme)
	// 保留策略与它的说明放在同一个列里：说明是那张卡的脚注，不该和下一张卡
	// 等距，否则读起来分不清它属于谁。
	ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 1)).Children(func() {
		settingsGroup(c, theme, "settings-retention-group", func() {
			// 保留策略两行：条数与天数各自可单独关掉（不限），改完立即生效，
			// 见 applyRetention。
			a.settingsRow(c, theme, a.t(msgSettingsRetentionItemsRow), a.t(msgSettingsRetentionItemsRowDesc), func() {
				control := selectField(c, theme, "settings-retention-items", &a.retention.MaxItems,
					a.t(msgSettingsRetentionItemsRow), a.l10n().retentionItemChoices())
				if a.compactDetailLayout(c, theme) {
					control.FillWidth()
				} else {
					control.Width(180)
				}
				if control.Changed() {
					a.applyRetention()
				}
			})
			settingsRule(c, theme)
			a.settingsRow(c, theme, a.t(msgSettingsRetentionDaysRow), a.t(msgSettingsRetentionDaysRowDesc), func() {
				control := selectField(c, theme, "settings-retention-days", &a.retention.MaxDays,
					a.t(msgSettingsRetentionDaysRow), a.l10n().retentionDayChoices())
				if a.compactDetailLayout(c, theme) {
					control.FillWidth()
				} else {
					control.Width(180)
				}
				if control.Changed() {
					a.applyRetention()
				}
			})
		})
		ui.Text(c, a.t(msgSettingsRetentionNote)).
			FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
	})

	settingsGroup(c, theme, "settings-clear-history-group", func() {
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
}

// storageUsageCategory 是占比条与图例里的一类占用。
type storageUsageCategory struct {
	key   string
	icon  *ui.SVG
	name  string
	color ui.Color
	bytes int64
}

// minStorageSegmentShare 是一类占用在占比条上的最小占比。数据库通常比另一类大
// 几个数量级，按真实比例画的话收到的文件只剩零点几个像素，看起来像没有东西。
const minStorageSegmentShare = 0.015

// storageUsageCategories 按固定顺序列出两类占用。颜色都在强调色上取不同浓淡：
// 两者是同一件事（应用占的盘）的两个部分，不是两个不同的语义。设置与配对文件
// 只有几 KB，不值得单列一行，也不进总计。
func (a *application) storageUsageCategories(theme *ui.Theme) []storageUsageCategory {
	return []storageUsageCategory{
		{key: "database", icon: icons.storage, name: a.t(msgSettingsStorageDatabase),
			color: theme.Accent, bytes: a.storageUsage.database},
		{key: "files", icon: icons.file, name: a.t(msgSettingsStorageFiles),
			color: theme.Accent.Alpha(0.55), bytes: a.storageUsage.files},
	}
}

// storageUsageSection 画「用量」一节的用量区：总计、分段占比条、图例，以及记录条数。
// 数字来自最近一次统计（见 requestStorageUsage），渲染本身不碰磁盘；记录条数来自
// 内存里的历史，与用量同属「现在存了什么」，因此排在一起。
func (a *application) storageUsageSection(c *ui.Context, theme *ui.Theme, stats history.View) {
	categories := a.storageUsageCategories(theme)
	total := a.storageUsage.total()
	ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 2.5)).Children(func() {
		ui.Row(c).FillWidth().Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
			labelText(c, formatBytes(total)).FontSize(font(theme, fontPageTitle)).FontWeight(600)
			labelText(c, a.t(msgSettingsStorageTotal)).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme))
			ui.Spacer(c)
			a.iconActionButton(c, theme, "settings-storage-refresh", icons.refresh,
				a.t(msgSettingsStorageRefresh), a.storageUsagePending, a.refreshStorageUsage)
		})
		a.storageUsageBar(c, theme, categories, total)
		a.storageUsageLegend(c, theme, categories)
		labelText(c, a.l10n().storageRecord(stats.Total)+" · "+a.l10n().imageCount(stats.ImageCount)).
			FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme))
		if stats.Total == 0 {
			ui.Text(c, a.t(msgSettingsNoHistoryNote)).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme)).Wrap()
		}
	})
}

// storageUsageBar 画分段占比条：一段一类，段间留一道缝，每段自己带圆角。总计为 0
// 时画一条浅色空底，而不是什么都不画。
func (a *application) storageUsageBar(c *ui.Context, theme *ui.Theme, categories []storageUsageCategory, total int64) {
	height := space(theme, 2.5)
	if total <= 0 {
		ui.Box(c.Key("settings-storage-usage-track")).FillWidth().Height(height).Shrink(0).
			Radius(height / 2).Background(borderColor(theme))
		return
	}
	ui.Row(c.Key("settings-storage-usage-bar")).FillWidth().Height(height).Shrink(0).
		Gap(space(theme, 0.5)).Children(func() {
		for _, category := range categories {
			if category.bytes <= 0 {
				continue
			}
			share := float32(category.bytes) / float32(total)
			ui.Box(c.Key("settings-storage-usage-segment-" + category.key)).Height(height).Shrink(0).
				Grow(max(share, minStorageSegmentShare)).Radius(height / 2).
				Background(category.color)
		}
	})
}

// storageUsageLegend 把两类占用排成两列（窄布局下一列），每项是色点 + 图标 + 名称，
// 大小靠右对齐。
func (a *application) storageUsageLegend(c *ui.Context, theme *ui.Theme, categories []storageUsageCategory) {
	columns := 2
	if a.compactDetailLayout(c, theme) {
		columns = 1
	}
	ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 1.5)).Children(func() {
		for start := 0; start < len(categories); start += columns {
			group := categories[start:min(start+columns, len(categories))]
			ui.Row(c).FillWidth().Shrink(0).Gap(space(theme, 3)).Children(func() {
				for _, category := range group {
					a.storageUsageLegendItem(c, theme, category)
				}
				// 末行不满时用空格子占位，最后一列的大小仍与前几行对齐。
				for missing := columns - len(group); missing > 0; missing-- {
					ui.Box(c).Grow(1)
				}
			})
		}
	})
}

func (a *application) storageUsageLegendItem(c *ui.Context, theme *ui.Theme, category storageUsageCategory) {
	ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(space(theme, 1)).Children(func() {
		dot := space(theme, 1.25)
		ui.Box(c).Size(dot, dot).Shrink(0).Radius(dot / 2).Background(category.color)
		ui.Icon(c, category.icon).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0).
			TextColor(secondaryText(theme))
		labelText(c, category.name).FontSize(font(theme, fontControl)).TextColor(primaryText(theme))
		ui.Spacer(c)
		labelText(c, formatBytes(category.bytes)).FontSize(font(theme, fontControl)).
			TextColor(secondaryText(theme))
	})
}

// storageClearLabel 根据是否有记录返回清理按钮的文案。
func (a *application) storageClearLabel(total int) string {
	if total == 0 {
		return a.t(msgSettingsNothingToClear)
	}
	return a.t(msgSettingsClearAll)
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
