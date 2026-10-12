package main

import (
	"github.com/egoist/mygo/ui"
)

// devicesPage 使用侧栏选择设备，右侧展示当前 Peer 的同步设置。
func (a *application) devicesPage(c *ui.Context, theme *ui.Theme) {
	ui.Row(c).Fill().MinWidth(0).Children(func() {
		a.deviceSidebar(c, theme)
		ui.Box(c).Width(1).FillHeight().Shrink(0).
			Background(dividerColor(theme))
		a.deviceDetail(c, theme)
	})

	a.addDeviceDialogView(c, theme)
}

// deviceSidebar 是设备页的主从导航。侧栏保持固定宽度，避免设备名称变化时右侧内容跳动。
func (a *application) deviceSidebar(c *ui.Context, theme *ui.Theme) {
	onlineCount := a.onlineDeviceCount()
	onlineColor := secondaryText(theme)
	if onlineCount > 0 {
		onlineColor = theme.Success
	}

	ui.Column(c).Width(secondarySidebarSize(c, theme)).FillHeight().Shrink(0).
		Padding(space(theme, columnTopInset), 0, space(theme, 5), space(theme, 4)).
		Gap(space(theme, spaceControl)).Background(sidebarFill(theme)).Children(func() {
		secondarySidebarTitle(c, theme, a.t(msgDevicesTitle))
		a.localDeviceEntry(c, theme)

		ui.Row(c).Height(controlHeightSize(c, theme, controlHeight)).Shrink(0).AlignItems(ui.Center).
			Margin(0, space(theme, sidebarTrailingInset), 0, 0).
			Padding(space(theme, 3), 0, 0, 0).Children(func() {
			ui.Text(c, a.t(msgDevicesPaired)).FontSize(font(theme, fontSubheading)).FontWeight(600).
				TextColor(secondaryText(theme)).SingleLine()
			ui.Spacer(c)
			refresh := ui.ButtonBase(c.Key("refresh-paired-devices")).
				Size(controlHeightSize(c, theme, controlHeight), controlHeightSize(c, theme, controlHeight)).Shrink(0).Center().
				Radius(radius(theme, controlRadius)).TextColor(secondaryText(theme)).
				Label(a.t(msgDevicesRefresh)).Tooltip(a.t(msgDevicesRefresh))
			if refresh.Hovered() {
				refresh.Background(hoverFill(theme))
			}
			refresh.Children(func() {
				ui.Icon(c, icons.refresh).Size(space(theme, 4), space(theme, 4))
			})
			if refresh.Clicked() {
				a.refreshDevices(c)
			}
		})

		ui.Scroll(c).Grow(1).MinHeight(0).ScrollbarInsets(0, space(theme, 0.5), 0, 0).
			Children(func() {
				ui.Column(c).FillWidth().Gap(space(theme, 1)).Children(func() {
					switch a.deviceDiscovery {
					case deviceDiscoveryLoading:
						a.deviceLoadingState(c, theme)
					case deviceDiscoveryError:
						a.deviceErrorState(c, theme)
					default:
						if len(a.pairedDevices) == 0 {
							a.deviceEmptyState(c, theme)
						}
						for index := range a.pairedDevices {
							a.pairedDeviceEntry(c, theme, index)
						}
					}
				})
			})

		ui.Column(c).Shrink(0).Gap(space(theme, 2)).
			Margin(0, space(theme, sidebarTrailingInset), 0, 0).Children(func() {
			a.inviteDeviceButton(c, theme)
			ui.Row(c).Shrink(0).Padding(space(theme, 1), space(theme, 2.5)).Children(func() {
				deviceStatus(c, theme, a.l10n().onlineCount(onlineCount), onlineColor)
			})
		})
	})
}

func (a *application) deviceEmptyState(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		ui.Icon(c, icons.devices).Size(space(theme, 5), space(theme, 5)).
			TextColor(secondaryText(theme))
		ui.Text(c, a.t(msgDevicesEmptyTitle)).FontSize(font(theme, fontControl)).FontWeight(600).
			SingleLine()
		ui.Text(c, a.t(msgDevicesEmptyMessage)).
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
	})
}

func (a *application) deviceLoadingState(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		statusDot(c, theme.Accent, space(theme, 2))
		ui.Text(c, a.t(msgDevicesLoadingTitle)).FontSize(font(theme, fontControl)).FontWeight(600).
			SingleLine()
		ui.Text(c, a.t(msgDevicesLoadingMessage)).
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
	})
}

func (a *application) deviceErrorState(c *ui.Context, theme *ui.Theme) {
	message := a.deviceDiscoveryMessage
	if message == "" {
		message = a.t(msgDevicesErrorMessage)
	}
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		statusDot(c, theme.Danger, space(theme, 2))
		ui.Text(c, a.t(msgDevicesErrorTitle)).FontSize(font(theme, fontControl)).FontWeight(600).
			TextColor(theme.Danger).SingleLine()
		ui.Text(c, message).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
		retryLabel := a.t(msgDevicesRetry)
		retry := neutralButton(c, theme, "retry-paired-devices", retryLabel)
		retry.Children(func() {
			ui.Text(c, retryLabel).FontSize(font(theme, fontControl)).SingleLine()
		})
		if retry.Clicked() {
			a.refreshDevices(c)
		}
	})
}

func (a *application) refreshDevices(c *ui.Context) {
	if a.onRefreshDevices == nil {
		c.Toast(a.t(msgToastDeviceDiscoveryUnavailable))
		return
	}
	a.deviceDiscovery = deviceDiscoveryLoading
	a.deviceDiscoveryMessage = ""
	a.onRefreshDevices()
}

func (a *application) localDeviceEntry(c *ui.Context, theme *ui.Theme) {
	selected := a.selectedDeviceID == ""
	entry := a.deviceEntryButton(c, theme, "local-device", selected)
	entry.Children(func() {
		deviceIcon(c, theme, iconForPlatform(a.devicePlatform), selected)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 1)).Children(func() {
			ui.Text(c, a.deviceName).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			deviceSyncStatus(c, theme, a.deviceSyncEnabled, a.t(msgDevicesSyncOn), a.t(msgDevicesSyncOff))
		})
	})
	if entry.Clicked() {
		a.selectedDeviceID = ""
		a.deviceInfoExpanded = false
	}
}

func (a *application) pairedDeviceEntry(c *ui.Context, theme *ui.Theme, index int) {
	device := &a.pairedDevices[index]
	selected := a.selectedDeviceID == device.id
	entry := a.deviceEntryButton(c, theme, "paired-device-"+device.id, selected)
	entry.Children(func() {
		deviceIcon(c, theme, iconForPlatform(device.platform), selected)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.75)).Children(func() {
			ui.Text(c, device.name).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).Children(func() {
				deviceStatus(c, theme, a.connectionLabel(device.online), connectionColor(theme, device.online))
			})
		})
	})
	if entry.Clicked() {
		a.selectedDeviceID = device.id
		a.deviceInfoExpanded = true
	}
}

func (a *application) deviceEntryButton(c *ui.Context, theme *ui.Theme, key string, selected bool) ui.Element {
	entry := ui.ButtonBase(c.Key(key)).MinHeight(controlHeightSize(c, theme, 16)).Shrink(0).
		AlignSelf(ui.Stretch).
		Margin(0, space(theme, sidebarTrailingInset), 0, 0).
		Padding(space(theme, 2.5), space(theme, 3)).Gap(space(theme, 2.5)).
		Radius(radius(theme, cardRadius)).AlignItems(ui.Center).Border(1, ui.Transparent).
		TextColor(primaryText(theme)).Label(a.t(msgDevicesSelect)).
		Role(ui.RoleToggleButton).Selected(selected)
	if selected {
		entry.Background(selectedFill(theme)).BorderColor(selectionBorder(c, theme))
	} else if entry.Hovered() {
		entry.Background(hoverFill(theme))
	} else {
		entry.Background(ui.Transparent)
	}
	return entry
}

func (a *application) inviteDeviceButton(c *ui.Context, theme *ui.Theme) {
	button := ui.ButtonBase(c.Key("invite-device")).Height(controlHeightSize(c, theme, controlHeight)).FillWidth().Shrink(0).
		Padding(space(theme, 1.5), space(theme, 3)).Gap(space(theme, spaceInline)).
		Radius(radius(theme, controlRadius)).Border(1, borderColor(theme)).
		TextColor(primaryText(theme)).Label(a.t(msgDevicesAdd))
	if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icons.deviceAdd).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0)
		ui.Text(c, a.t(msgDevicesAdd)).FontSize(font(theme, fontControl)).FontWeight(600).SingleLine()
	})
	if button.Clicked() {
		a.addDeviceDialog = true
		a.pairingPending = false
		a.pairingError = ""
	}
}

// deviceDetail 展示侧栏当前选中设备的详情。
func (a *application) deviceDetail(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Grow(1).FillHeight().MinWidth(0).Background(theme.Background).
		Children(func() {
			ui.Scroll(c).Grow(1).MinHeight(0).ScrollbarInsets(0, space(theme, 1), 0, 0).
				Children(func() {
					ui.Column(c).FillWidth().MaxWidth(pageDetailMaxWidth).AlignSelf(ui.Center).
						Padding(space(theme, columnTopInset), space(theme, spacePageSide), space(theme, spacePageBottom), space(theme, spacePageSide)).
						Gap(space(theme, spaceSection)).Children(func() {
						a.deviceDetailHeader(c, theme)
						a.deviceConnectionNotice(c, theme)
						a.deviceSyncSettings(c, theme)
						a.deviceInfoDisclosure(c, theme)
					})
				})
		})
}

func (a *application) deviceConnectionNotice(c *ui.Context, theme *ui.Theme) {
	device, paired := a.selectedPairedDevice()
	if !paired || device.online {
		return
	}
	standardCard(c, theme, "device-offline-notice", func() {
		ui.Row(c).FillWidth().Shrink(0).Padding(space(theme, 3), space(theme, 4)).
			AlignItems(ui.Center).Gap(space(theme, 2)).Children(func() {
			statusDot(c, theme.Warning, space(theme, 2))
			ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
				ui.Text(c, a.t(msgDevicesOfflineTitle)).FontSize(font(theme, fontControl)).FontWeight(600).
					TextColor(theme.Warning).SingleLine()
				ui.Text(c, a.t(msgDevicesOfflineMessage)).
					FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
				if !device.lastSeenAt.IsZero() {
					ui.Text(c, a.tf(msgDevicesLastSeen, a.timestampLabel(device.lastSeenAt))).
						FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).SingleLine()
				}
			})
		})
	})
}

func (a *application) deviceDetailHeader(c *ui.Context, theme *ui.Theme) {
	device, paired := a.selectedPairedDevice()
	name := a.deviceName
	platform := a.devicePlatform
	online := true
	syncEnabled := a.deviceSyncEnabled
	if paired {
		name = device.name
		platform = device.platform
		online = device.online
		syncEnabled = device.syncEnabled
	}

	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 4.5)).Children(func() {
		deviceIconLarge(c, theme, platform, paired && !online)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 2)).Children(func() {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2.5)).Children(func() {
				ui.Text(c, name).FontSize(font(theme, fontPageTitle)).FontWeight(600).SingleLine()
				if paired {
					deviceBadge(c, theme, a.t(msgDevicesBadgePaired))
				} else {
					deviceBadge(c, theme, a.t(msgDevicesBadgeLocal))
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
				if paired {
					a.deviceConnectionStatus(c, theme, device)
					return
				}
				deviceSyncStatus(c, theme, syncEnabled, a.t(msgDevicesSyncOn), a.t(msgDevicesSyncOff))
				ui.Text(c, "·").FontSize(font(theme, fontControl)).TextColor(secondaryText(theme))
				ui.Text(c, platform).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).SingleLine()
			})
		})
		if paired {
			a.unpairButton(c, theme)
		}
	})
}

func (a *application) unpairButton(c *ui.Context, theme *ui.Theme) {
	unpairLabel := a.t(msgDevicesUnpair)
	button := dangerButton(c, theme, "unpair-selected-device", unpairLabel).
		Gap(space(theme, spaceInline))
	button.Children(func() {
		ui.Icon(c, icons.unlink).Size(space(theme, 3.5), space(theme, 3.5)).
			Shrink(0).TextColor(theme.Danger)
		ui.Text(c, unpairLabel).FontSize(font(theme, fontControl)).FontWeight(600).
			TextColor(theme.Danger).SingleLine()
	})
	if button.Clicked() {
		if device, ok := a.selectedPairedDevice(); ok {
			a.requestDangerousAction(dangerousActionUnpairDevice, device.id)
		}
	}
}

func (a *application) deviceConnectionStatus(c *ui.Context, theme *ui.Theme, device *pairedDevice) {
	deviceStatus(c, theme, a.connectionLabel(device.online), connectionColor(theme, device.online))
	ui.Text(c, "·").FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
	ui.Text(c, a.t(msgDevicesDirect)).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).SingleLine()
	if device.platform != "" {
		ui.Text(c, "·").FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
		ui.Text(c, device.platform).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).SingleLine()
	}
}

func (a *application) connectionLabel(online bool) string {
	if online {
		return a.t(msgDevicesOnline)
	}
	return a.t(msgDevicesOffline)
}

func connectionColor(theme *ui.Theme, online bool) ui.Color {
	if online {
		return theme.Success
	}
	return secondaryText(theme)
}

func (a *application) deviceSyncSettings(c *ui.Context, theme *ui.Theme) {
	device, paired := a.selectedPairedDevice()
	standardCard(c, theme, "device-sync-settings", func() {
		if paired {
			a.pairedSyncRows(c, theme, device)
			return
		}
		a.deviceSwitchRow(c, theme, a.t(msgDevicesEnableSync), a.t(msgDevicesEnableSyncDesc), "local-sync", &a.deviceSyncEnabled, false)
		deviceDivider(c, theme)
		fileSyncDescription := a.t(msgDevicesFileSyncDesc)
		if !a.deviceSyncEnabled {
			fileSyncDescription = a.tf(msgDevicesFileSyncDisabled, fileSyncDescription)
		}
		a.deviceSwitchRow(c, theme, a.t(msgDevicesFileSync), fileSyncDescription, "local-file-sync", &a.deviceFileSync, !a.deviceSyncEnabled)
	})
}

func (a *application) pairedSyncRows(c *ui.Context, theme *ui.Theme, device *pairedDevice) {
	a.deviceSwitchRow(c, theme, a.tf(msgDevicesSyncWith, device.name), a.t(msgDevicesSyncWithDesc), device.id+"-sync", &device.syncEnabled, false)
	deviceDivider(c, theme)

	ui.Row(c.Key(device.id+"-sync-content-mode")).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 3.5), space(theme, 4)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 1)).Children(func() {
			ui.Text(c, a.t(msgDevicesSyncContent)).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			ui.Text(c, a.t(msgDevicesSyncContentDesc)).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme)).Wrap()
		})
		deviceBadge(c, theme, a.syncContentModeLabel(device))
	})
	deviceDivider(c, theme)

	settings := []syncContentSetting{
		{key: "text", label: a.t(msgKindText), icon: icons.text, direction: &device.textSyncDirection},
		{key: "image", label: a.t(msgKindImage), icon: icons.image, direction: &device.imageSyncDirection},
		{key: "file", label: a.t(msgKindFile), icon: icons.file, direction: &device.fileSyncDirection},
		{key: "rich-text", label: a.t(msgKindRichText), icon: icons.text, direction: &device.richTextSyncDirection},
	}
	directions := a.l10n().syncDirectionOptions(device.name)
	for index, setting := range settings {
		a.syncDirectionRow(c, theme, device, setting, directions)
		if index < len(settings)-1 {
			deviceDivider(c, theme)
		}
	}
}

type syncContentSetting struct {
	key       string
	label     string
	icon      *ui.SVG
	direction *syncDirection
}

func (a *application) syncDirectionRow(
	c *ui.Context,
	theme *ui.Theme,
	device *pairedDevice,
	setting syncContentSetting,
	directions []option[syncDirection],
) {
	disabled := !device.syncEnabled || !device.online
	description := a.syncDirectionDescription(device, setting.label)
	selectLabel := a.tf(msgSyncDirectionLabel, setting.label)
	if compactDetailLayout(c, theme) {
		ui.Column(c.Key(device.id+"-sync-content-"+setting.key)).FillWidth().Shrink(0).
			Padding(space(theme, 3), space(theme, 4)).Gap(space(theme, 1.5)).Children(func() {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2)).Children(func() {
				ui.Icon(c, setting.icon).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0).
					TextColor(secondaryText(theme))
				ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
					ui.Text(c, setting.label).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
					ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
				})
			})
			selectField(c, theme, device.id+"-direction-"+setting.key, setting.direction, selectLabel, directions).
				FillWidth().Disabled(disabled)
		})
		return
	}

	ui.Row(c.Key(device.id+"-sync-content-"+setting.key)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 3), space(theme, 4)).AlignItems(ui.Center).Gap(space(theme, 3)).Children(func() {
		ui.Icon(c, setting.icon).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0).TextColor(secondaryText(theme))
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
			ui.Text(c, setting.label).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		selectField(c, theme, device.id+"-direction-"+setting.key, setting.direction, selectLabel, directions).
			Width(260).Disabled(disabled)
	})
}

func (a *application) syncDirectionDescription(device *pairedDevice, label string) string {
	if !device.syncEnabled {
		return a.tf(msgSyncDescSyncOff, label)
	}
	if !device.online {
		return a.tf(msgSyncDescOffline, label)
	}
	return a.tf(msgSyncDescNormal, label)
}

// syncContentModeLabel 用状态而不是按钮来表达同步内容的当前模式：
// 四项内容都是双向同步时显示“全部双向同步”，否则视为已自定义。
func (a *application) syncContentModeLabel(device *pairedDevice) string {
	if device.textSyncDirection == syncDirectionTwoWay &&
		device.imageSyncDirection == syncDirectionTwoWay &&
		device.fileSyncDirection == syncDirectionTwoWay &&
		device.richTextSyncDirection == syncDirectionTwoWay {
		return a.t(msgDevicesAllTwoWay)
	}
	return a.t(msgDevicesCustomSync)
}

func (a *application) deviceSwitchRow(
	c *ui.Context,
	theme *ui.Theme,
	title string,
	description string,
	key string,
	value *bool,
	disabled bool,
) {
	ui.Row(c.Key("device-setting-"+key)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 4), space(theme, 4)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 1)).Children(func() {
			ui.Text(c, title).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		switchControl := ui.Switch(c.Key("device-switch-"+key), value).
			Shrink(0).Disabled(disabled).Label(title)
		if key == "local-sync" && switchControl.Changed() {
			// 页面开关和托盘开关共用同一条状态更新路径，避免设备状态不一致。
			a.setSyncEnabled(a.deviceSyncEnabled)
		}
	})
}

func (a *application) deviceInfoDisclosure(c *ui.Context, theme *ui.Theme) {
	device, paired := a.selectedPairedDevice()
	title := a.t(msgDevicesInfoTitle)
	if paired {
		title = a.t(msgDevicesConnectionTitle)
	}

	values := []deviceInfo{
		{key: "peer-id", label: a.t(msgDevicesInfoPeerID), value: displayValue(a.devicePeerID)},
		{key: "platform", label: a.t(msgDevicesInfoPlatform), value: displayValue(a.devicePlatform)},
		{key: "connection", label: a.t(msgDevicesInfoConnection), value: a.t(msgDevicesInfoLocalValue)},
		{key: "app-version", label: a.t(msgDevicesInfoAppVersion), value: settingsVersion},
	}
	if paired {
		values = []deviceInfo{
			{key: "peer-id", label: a.t(msgDevicesInfoPeerID), value: displayValue(device.peerID)},
			{key: "connection", label: a.t(msgDevicesInfoConnection), value: a.t(msgDevicesInfoDirectValue)},
			{key: "address", label: a.t(msgDevicesInfoAddress), value: displayValue(device.address)},
		}
	}
	standardCard(c, theme, "device-info-card", func() {
		button := ui.ButtonBase(c.Key("device-info-disclosure")).FillWidth().Shrink(0).
			MinHeight(space(theme, rowHeight)).Padding(space(theme, 3), space(theme, 4)).
			Radius(radius(theme, cardRadius)).TextColor(primaryText(theme)).
			Label(a.deviceInfoDisclosureLabel(title, a.deviceInfoExpanded)).
			Role(ui.RoleDisclosure).Expanded(a.deviceInfoExpanded)
		if button.Hovered() {
			button.Background(hoverFill(theme))
		} else {
			button.Background(ui.Transparent)
		}
		button.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
				ui.Spacer(c)
				rotation := float32(-90)
				if a.deviceInfoExpanded {
					rotation = 0
				}
				ui.Icon(c, icons.chevron).Size(18, 18).Rotate(rotation).
					TextColor(secondaryText(theme))
			})
		})
		if button.Clicked() {
			a.deviceInfoExpanded = !a.deviceInfoExpanded
		}
		if !a.deviceInfoExpanded {
			return
		}
		labelWidth := float32(120)
		rowGap := float32(4.5)
		if compactDetailLayout(c, theme) {
			labelWidth = 96
			rowGap = 2.5
		}
		for index, info := range values {
			if index > 0 {
				deviceDivider(c, theme)
			}
			ui.Row(c.Key("device-info-row-"+info.key)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
				Padding(space(theme, 2.5), space(theme, 4)).AlignItems(ui.Center).Gap(space(theme, rowGap)).Children(func() {
				ui.Text(c, info.label).Width(space(theme, labelWidth/4)).Shrink(0).FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme)).SingleLine()
				ui.Text(c, info.value).Grow(1).MinWidth(0).FontSize(font(theme, fontControl)).
					TextColor(primaryText(theme)).Wrap()
			})
		}
	})
}

func (a *application) deviceInfoDisclosureLabel(title string, expanded bool) string {
	if expanded {
		return a.tf(msgDevicesCollapse, title)
	}
	return a.tf(msgDevicesExpand, title)
}

// addDeviceDialogView 保留配对入口的状态容器，不在协议接入前伪造成功结果。
func (a *application) addDeviceDialogView(c *ui.Context, theme *ui.Theme) {
	ui.Modal(c, &a.addDeviceDialog, func() {
		ui.Column(c).Width(440).Shrink(0).
			Padding(space(theme, 6), space(theme, 6)).Gap(space(theme, 2)).
			Radius(radius(theme, largeRadius)).Background(theme.Background).
			Border(1, borderColor(theme)).Role(ui.RoleDialog).Label(a.t(msgDevicesAdd)).Children(func() {
			ui.Text(c, a.t(msgDevicesAdd)).FontSize(font(theme, fontSection)).FontWeight(600).
				Role(ui.RoleHeading).SingleLine()
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
				statusDot(c, theme.Accent, space(theme, 2))
				ui.Text(c, a.t(msgDevicesPairDirectOnly)).FontSize(font(theme, fontControl)).
					TextColor(theme.Accent).FontWeight(600).SingleLine()
			})
			ui.Text(c, a.t(msgDevicesPairDescription)).
				FontSize(font(theme, fontControl)).TextColor(secondaryText(theme)).Wrap()
			if a.pairingError != "" {
				ui.Text(c, a.pairingError).FontSize(font(theme, fontMeta)).
					TextColor(theme.Danger).Wrap()
			}
			ui.Spacer(c)
			ui.Row(c).FillWidth().Shrink(0).Justify(ui.End).Children(func() {
				if a.pairingPending {
					ui.Text(c, a.t(msgDevicesPairWaiting)).FontSize(font(theme, fontCaption)).
						TextColor(secondaryText(theme)).SingleLine()
				}
				startLabel := a.t(msgDevicesPairStart)
				if a.onStartPairing == nil {
					startLabel = a.t(msgDevicesPairWaitingSvc)
				}
				start := neutralButton(c, theme, "start-pairing", startLabel).
					Disabled(a.onStartPairing == nil || a.pairingPending)
				start.Children(func() {
					ui.Text(c, startLabel).FontSize(font(theme, fontControl)).SingleLine()
				})
				if start.Clicked() && a.onStartPairing != nil {
					a.pairingPending = true
					a.pairingError = ""
					a.onStartPairing()
				}

				closeLabel := a.t(msgDevicesPairClose)
				button := neutralButton(c, theme, "close-invite-device", closeLabel)
				button.Children(func() {
					ui.Text(c, closeLabel).FontSize(font(theme, fontControl)).SingleLine()
				})
				if button.Clicked() {
					a.addDeviceDialog = false
					a.pairingPending = false
				}
			})
		})
	})
}

func (a *application) unpairDevice(id string) bool {
	for index, device := range a.pairedDevices {
		if device.id != id {
			continue
		}
		a.pairedDevices = append(a.pairedDevices[:index], a.pairedDevices[index+1:]...)
		a.selectedDeviceID = ""
		a.deviceInfoExpanded = false
		return true
	}
	return false
}

func (a *application) onlineDeviceCount() int {
	count := 0
	for _, device := range a.pairedDevices {
		if device.online {
			count++
		}
	}
	return count
}

func (a *application) selectedPairedDevice() (*pairedDevice, bool) {
	if a.selectedDeviceID == "" {
		return nil, false
	}
	for index := range a.pairedDevices {
		if a.pairedDevices[index].id == a.selectedDeviceID {
			return &a.pairedDevices[index], true
		}
	}
	return nil, false
}

type deviceInfo struct {
	key   string
	label string
	value string
}

func deviceIcon(c *ui.Context, theme *ui.Theme, icon *ui.SVG, selected bool) {
	color := secondaryText(theme)
	background := subtleFill(theme)
	if selected {
		color = primaryText(theme)
		background = hoverFill(theme)
	}
	ui.Box(c).Size(space(theme, 9), space(theme, 9)).Shrink(0).Radius(radius(theme, controlRadius)).
		Background(background).Center().Children(func() {
		ui.Icon(c, icon).Size(space(theme, 4.5), space(theme, 4.5)).TextColor(color)
	})
}

func deviceIconLarge(c *ui.Context, theme *ui.Theme, platform string, muted bool) {
	color := theme.Text
	if muted {
		color = theme.TextMuted
	}
	ui.Box(c).Size(space(theme, 16.5), space(theme, 16.5)).Shrink(0).Radius(radius(theme, largeRadius)).
		Background(subtleFill(theme)).Border(1, borderColor(theme)).Center().Children(func() {
		ui.Icon(c, iconForPlatform(platform)).Size(space(theme, 8), space(theme, 8)).TextColor(color)
	})
}

func deviceSyncStatus(c *ui.Context, theme *ui.Theme, enabled bool, onLabel, offLabel string) {
	color := theme.TextMuted
	label := offLabel
	if enabled {
		color = theme.Success
		label = onLabel
	}
	deviceStatus(c, theme, label, color)
}

func deviceStatus(c *ui.Context, theme *ui.Theme, label string, color ui.Color) {
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		statusDot(c, color, space(theme, 1.5))
		ui.Text(c, label).FontSize(font(theme, fontCaption)).
			TextColor(color).SingleLine()
	})
}

func deviceBadge(c *ui.Context, theme *ui.Theme, label string) {
	ui.Text(c, label).FontSize(font(theme, fontCaption)).FontWeight(600).
		TextColor(secondaryText(theme)).Padding(space(theme, 1), space(theme, 2.25)).Radius(radius(theme, cardRadius)).
		Background(subtleFill(theme)).SingleLine()
}

func deviceDivider(c *ui.Context, theme *ui.Theme) {
	standardDivider(c, theme)
}
