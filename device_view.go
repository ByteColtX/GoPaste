package main

import (
	"fmt"

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
		Padding(space(theme, 6), space(theme, 4), space(theme, 5), space(theme, 4)).
		Gap(space(theme, spaceControl)).Background(sidebarFill(theme)).Children(func() {
		ui.Text(c, "本设备").FontSize(font(theme, fontSubheading)).FontWeight(600).
			TextColor(secondaryText(theme)).Padding(space(theme, 1), space(theme, 2.5), space(theme, 0.75), space(theme, 2.5)).SingleLine()
		a.localDeviceEntry(c, theme)

		ui.Row(c).Height(space(theme, controlHeight)).Shrink(0).AlignItems(ui.Center).
			Padding(space(theme, 3), space(theme, 2.5), 0, space(theme, 2.5)).Children(func() {
			ui.Text(c, "已配对设备").FontSize(font(theme, fontSubheading)).FontWeight(600).
				TextColor(secondaryText(theme)).SingleLine()
			ui.Spacer(c)
			refresh := ui.ButtonBase(c.Key("refresh-paired-devices")).
				Size(space(theme, controlHeight), space(theme, controlHeight)).Shrink(0).Center().
				Radius(radius(theme, controlRadius)).TextColor(secondaryText(theme)).
				Label("刷新设备列表").Tooltip("刷新设备列表")
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

		ui.Column(c).Shrink(0).Gap(space(theme, 2)).Children(func() {
			a.inviteDeviceButton(c, theme)
			ui.Row(c).Shrink(0).Padding(space(theme, 1), space(theme, 2.5)).Children(func() {
				deviceStatus(c, theme, fmt.Sprintf("%d 台在线", onlineCount), onlineColor)
			})
		})
	})
}

func (a *application) deviceEmptyState(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		ui.Icon(c, icons.devices).Size(space(theme, 5), space(theme, 5)).
			TextColor(secondaryText(theme))
		ui.Text(c, "暂无已配对设备").FontSize(font(theme, fontControl)).FontWeight(600).
			SingleLine()
		ui.Text(c, "通过局域网直连配对后，设备会显示在这里。\n剪贴板内容只在已配对设备之间同步。").
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
	})
}

func (a *application) deviceLoadingState(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		statusDot(c, theme.Accent, space(theme, 2))
		ui.Text(c, "正在加载设备").FontSize(font(theme, fontControl)).FontWeight(600).
			SingleLine()
		ui.Text(c, "正在检查本机局域网内的已配对设备。").
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
	})
}

func (a *application) deviceErrorState(c *ui.Context, theme *ui.Theme) {
	message := a.deviceDiscoveryMessage
	if message == "" {
		message = "暂时无法读取已配对设备。"
	}
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		statusDot(c, theme.Danger, space(theme, 2))
		ui.Text(c, "设备列表加载失败").FontSize(font(theme, fontControl)).FontWeight(600).
			TextColor(theme.Danger).SingleLine()
		ui.Text(c, message).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
		retry := neutralButton(c, theme, "retry-paired-devices", "重试")
		retry.Children(func() {
			ui.Text(c, "重试").FontSize(font(theme, fontControl)).SingleLine()
		})
		if retry.Clicked() {
			a.refreshDevices(c)
		}
	})
}

func (a *application) refreshDevices(c *ui.Context) {
	if a.onRefreshDevices == nil {
		c.Toast("设备发现服务尚未接入")
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
		deviceIcon(c, theme, icons.devices, selected)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 1)).Children(func() {
			ui.Text(c, a.deviceName).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			deviceSyncStatus(c, theme, a.deviceSyncEnabled, "同步已开启", "同步已关闭")
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
		deviceIcon(c, theme, icons.devices, selected)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.75)).Children(func() {
			ui.Text(c, device.name).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).Children(func() {
				deviceStatus(c, theme, connectionLabel(device.online), connectionColor(theme, device.online))
				ui.Text(c, "· 局域网直连").FontSize(font(theme, fontCaption)).
					TextColor(secondaryText(theme)).SingleLine()
			})
		})
	})
	if entry.Clicked() {
		a.selectedDeviceID = device.id
		a.deviceInfoExpanded = true
	}
}

func (a *application) deviceEntryButton(c *ui.Context, theme *ui.Theme, key string, selected bool) ui.Element {
	entry := ui.ButtonBase(c.Key(key)).Height(space(theme, 16)).FillWidth().Shrink(0).
		Padding(space(theme, 2.5), space(theme, 3)).Gap(space(theme, 2.5)).
		Radius(radius(theme, cardRadius)).AlignItems(ui.Center).
		TextColor(primaryText(theme)).Label("选择设备").
		Role(ui.RoleToggleButton).Selected(selected)
	if selected {
		entry.Background(selectedFill(theme))
	} else if entry.Hovered() {
		entry.Background(hoverFill(theme))
	} else {
		entry.Background(ui.Transparent)
	}
	return entry
}

func (a *application) inviteDeviceButton(c *ui.Context, theme *ui.Theme) {
	button := ui.ButtonBase(c.Key("invite-device")).Height(space(theme, controlHeight)).FillWidth().Shrink(0).
		Padding(space(theme, 1.5), space(theme, 3)).Gap(space(theme, spaceInline)).
		Radius(radius(theme, controlRadius)).Border(1, borderColor(theme)).
		TextColor(primaryText(theme)).Label("新增设备")
	if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icons.deviceAdd).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0)
		ui.Text(c, "新增设备").FontSize(font(theme, fontControl)).FontWeight(600).SingleLine()
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
			ui.Scroll(c).Grow(1).MinHeight(0).ScrollbarInsets(0, space(theme, 1.5), 0, 0).
				Children(func() {
					ui.Column(c).FillWidth().MaxWidth(pageDetailMaxWidth).AlignSelf(ui.Center).
						Padding(space(theme, spacePageTop), space(theme, spacePageSide), space(theme, spacePageBottom), space(theme, spacePageSide)).
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
				ui.Text(c, "设备当前离线").FontSize(font(theme, fontControl)).FontWeight(600).
					TextColor(theme.Warning).SingleLine()
				ui.Text(c, "重新回到同一局域网后，GoPaste 会恢复局域网直连；离线期间不会通过其他网络路径同步。").
					FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
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
		deviceIconLarge(c, theme, paired && !online)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 2)).Children(func() {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2.5)).Children(func() {
				ui.Text(c, name).FontSize(font(theme, fontPageTitle)).FontWeight(600).SingleLine()
				if paired {
					deviceBadge(c, theme, "已配对")
				} else {
					deviceBadge(c, theme, "本机")
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
				if paired {
					deviceConnectionStatus(c, theme, device)
					return
				}
				deviceSyncStatus(c, theme, syncEnabled, "同步已开启", "同步已关闭")
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
	button := dangerButton(c, theme, "unpair-selected-device", "取消配对").
		Gap(space(theme, spaceInline))
	button.Children(func() {
		ui.Icon(c, icons.clear).Size(space(theme, 3.5), space(theme, 3.5)).
			Shrink(0).TextColor(theme.Danger)
		ui.Text(c, "取消配对").FontSize(font(theme, fontControl)).FontWeight(600).
			TextColor(theme.Danger).SingleLine()
	})
	if button.Clicked() {
		if device, ok := a.selectedPairedDevice(); ok {
			a.requestDangerousAction(dangerousActionUnpairDevice, device.id)
		}
	}
}

func deviceConnectionStatus(c *ui.Context, theme *ui.Theme, device *pairedDevice) {
	deviceStatus(c, theme, connectionLabel(device.online), connectionColor(theme, device.online))
	ui.Text(c, "·").FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
	ui.Text(c, "局域网直连").FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).SingleLine()
	if device.platform != "" {
		ui.Text(c, "·").FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
		ui.Text(c, device.platform).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).SingleLine()
	}
}

func connectionLabel(online bool) string {
	if online {
		return "在线"
	}
	return "离线"
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
		a.deviceSwitchRow(c, theme, "启用同步", "允许设备之间同步内容", "local-sync", &a.deviceSyncEnabled, false)
		deviceDivider(c, theme)
		a.deviceSwitchRow(c, theme, "文件同步", "允许在设备间传输文件", "local-file-sync", &a.deviceFileSync, !a.deviceSyncEnabled)
	})
}

func (a *application) pairedSyncRows(c *ui.Context, theme *ui.Theme, device *pairedDevice) {
	a.deviceSwitchRow(c, theme, fmt.Sprintf("与%s同步", device.name), "双方复制的内容会自动同步。", device.id+"-sync", &device.syncEnabled, false)
	deviceDivider(c, theme)

	ui.Row(c).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 3.5), space(theme, 6)).
		AlignItems(ui.Center).Children(func() {
		ui.Text(c, "自定义同步内容").FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
		ui.Spacer(c)
		button := ui.ButtonBase(c.Key(device.id+"-all-bidirectional")).
			Height(space(theme, compactHeight)).Shrink(0).
			Padding(space(theme, 1), space(theme, 2)).Radius(radius(theme, controlRadius)).
			TextColor(secondaryText(theme)).Label("全部双向同步")
		if button.Hovered() {
			button.Background(hoverFill(theme))
		} else {
			button.Background(ui.Transparent)
		}
		button.Children(func() {
			ui.Text(c, "全部双向同步").FontSize(font(theme, fontControl)).SingleLine()
		})
		if button.Clicked() {
			a.setAllSyncDirections(device, defaultSyncDirection)
		}
	})
	deviceDivider(c, theme)

	ui.Row(c).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 2.75), space(theme, 6)).
		AlignItems(ui.Center).Children(func() {
		ui.Text(c, "分别设置每种内容的同步方向。").FontSize(font(theme, fontMeta)).
			TextColor(secondaryText(theme)).SingleLine()
		ui.Spacer(c)
		button := ui.ButtonBase(c.Key(device.id+"-restore-sync-defaults")).
			Height(space(theme, compactHeight)).Shrink(0).
			Padding(space(theme, 1), space(theme, 2)).Gap(space(theme, spaceInline)).
			Radius(radius(theme, controlRadius)).
			TextColor(secondaryText(theme)).Label("恢复默认")
		if button.Hovered() {
			button.Background(hoverFill(theme))
		} else {
			button.Background(ui.Transparent)
		}
		button.Children(func() {
			ui.Icon(c, icons.clear).Size(space(theme, 3.25), space(theme, 3.25)).Shrink(0)
			ui.Text(c, "恢复默认").FontSize(font(theme, fontControl)).SingleLine()
		})
		if button.Clicked() {
			a.restoreSyncDefaults(device)
		}
	})
	deviceDivider(c, theme)

	settings := []syncContentSetting{
		{key: "text", label: "文本", icon: icons.text, direction: &device.textSyncDirection},
		{key: "image", label: "图片", icon: icons.image, direction: &device.imageSyncDirection},
		{key: "file", label: "文件", icon: icons.file, direction: &device.fileSyncDirection},
		{key: "rich-text", label: "富文本", icon: icons.text, direction: &device.richTextSyncDirection},
	}
	directions := syncDirectionOptionsForPeer(device.name)
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
	direction *string
}

func (a *application) syncDirectionRow(
	c *ui.Context,
	theme *ui.Theme,
	device *pairedDevice,
	setting syncContentSetting,
	directions []string,
) {
	disabled := !device.syncEnabled || !device.online
	description := syncDirectionDescription(device, setting.label)
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
			ui.Select(c.Key(device.id+"-direction-"+setting.key), setting.direction, directions).
				FillWidth().Height(space(theme, controlHeight)).Shrink(0).Disabled(disabled).
				Label(setting.label + "同步方向")
		})
		return
	}

	ui.Row(c.Key(device.id+"-sync-content-"+setting.key)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 3), space(theme, 6)).AlignItems(ui.Center).Gap(space(theme, 3)).Children(func() {
		ui.Icon(c, setting.icon).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0).TextColor(secondaryText(theme))
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
			ui.Text(c, setting.label).FontSize(font(theme, fontBody)).FontWeight(600).SingleLine()
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		ui.Select(c.Key(device.id+"-direction-"+setting.key), setting.direction, directions).
			Width(260).Height(space(theme, controlHeight)).Shrink(0).Disabled(disabled).
			Label(setting.label + "同步方向")
	})
}

func syncDirectionDescription(device *pairedDevice, label string) string {
	if !device.syncEnabled {
		return "设备同步已关闭，开启后可设置" + label + "的方向。"
	}
	if !device.online {
		return "设备离线，重新建立局域网直连后可设置" + label + "的方向。"
	}
	return "仅影响此设备的" + label + "内容。"
}

func (a *application) setAllSyncDirections(device *pairedDevice, direction string) {
	device.textSyncDirection = direction
	device.imageSyncDirection = direction
	device.fileSyncDirection = direction
	device.richTextSyncDirection = direction
}

func (a *application) restoreSyncDefaults(device *pairedDevice) {
	// 演示版默认所有内容都允许双向同步，真实服务接入后可在这里读取持久化策略。
	a.setAllSyncDirections(device, defaultSyncDirection)
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
	if disabled && title == "文件同步" {
		description += " 请先开启设备同步。"
	}
	ui.Row(c.Key("device-setting-"+key)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 4), space(theme, 6)).AlignItems(ui.Center).Children(func() {
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
	title := "设备信息"
	if paired {
		title = "连接信息"
	}

	values := []deviceInfo{
		{label: "Peer ID", value: displayValue(a.devicePeerID)},
		{label: "平台", value: displayValue(a.devicePlatform)},
		{label: "连接方式", value: "本机"},
		{label: "应用版本", value: settingsVersion},
	}
	if paired {
		values = []deviceInfo{
			{label: "Peer ID", value: displayValue(device.peerID)},
			{label: "连接方式", value: "局域网直连"},
			{label: "地址", value: displayValue(device.address)},
		}
	}
	standardCard(c, theme, "device-info-card", func() {
		button := ui.ButtonBase(c.Key("device-info-disclosure")).FillWidth().Shrink(0).
			MinHeight(space(theme, rowHeight)).Padding(space(theme, 3), space(theme, 6)).
			Radius(radius(theme, cardRadius)).TextColor(primaryText(theme)).
			Label(deviceInfoDisclosureLabel(title, a.deviceInfoExpanded)).
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
			ui.Row(c.Key("device-info-row-"+info.label)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
				Padding(space(theme, 2.5), space(theme, 6)).AlignItems(ui.Center).Gap(space(theme, rowGap)).Children(func() {
				ui.Text(c, info.label).Width(space(theme, labelWidth/4)).Shrink(0).FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme)).SingleLine()
				ui.Text(c, info.value).Grow(1).MinWidth(0).FontSize(font(theme, fontControl)).
					TextColor(primaryText(theme)).Wrap()
			})
		}
	})
}

func deviceInfoDisclosureLabel(title string, expanded bool) string {
	if expanded {
		return "收起" + title
	}
	return "展开" + title
}

// addDeviceDialogView 保留配对入口的状态容器，不在协议接入前伪造成功结果。
func (a *application) addDeviceDialogView(c *ui.Context, theme *ui.Theme) {
	ui.Modal(c, &a.addDeviceDialog, func() {
		ui.Column(c).Width(440).Shrink(0).
			Padding(space(theme, 6), space(theme, 6)).Gap(space(theme, 2)).
			Radius(radius(theme, largeRadius)).Background(theme.Background).
			Border(1, borderColor(theme)).Role(ui.RoleDialog).Label("新增设备").Children(func() {
			ui.Text(c, "新增设备").FontSize(font(theme, fontSection)).FontWeight(600).
				Role(ui.RoleHeading).SingleLine()
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
				statusDot(c, theme.Accent, space(theme, 2))
				ui.Text(c, "仅支持局域网直连").FontSize(font(theme, fontControl)).
					TextColor(theme.Accent).FontWeight(600).SingleLine()
			})
			ui.Text(c, "设备配对功能正在接入中。完成协议接入后，可在这里发现同一局域网内的设备并完成配对。GoPaste 不使用云端、中继或其他网络回退连接。").
				FontSize(font(theme, fontControl)).TextColor(secondaryText(theme)).Wrap()
			if a.pairingError != "" {
				ui.Text(c, a.pairingError).FontSize(font(theme, fontMeta)).
					TextColor(theme.Danger).Wrap()
			}
			ui.Spacer(c)
			ui.Row(c).FillWidth().Shrink(0).Justify(ui.End).Children(func() {
				if a.pairingPending {
					ui.Text(c, "等待局域网配对服务…").FontSize(font(theme, fontCaption)).
						TextColor(secondaryText(theme)).SingleLine()
				}
				startLabel := "开始配对"
				if a.onStartPairing == nil {
					startLabel = "等待配对服务"
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

				button := neutralButton(c, theme, "close-invite-device", "关闭")
				button.Children(func() {
					ui.Text(c, "关闭").FontSize(font(theme, fontControl)).SingleLine()
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

func deviceIconLarge(c *ui.Context, theme *ui.Theme, muted bool) {
	color := theme.Text
	if muted {
		color = theme.TextMuted
	}
	ui.Box(c).Size(space(theme, 16.5), space(theme, 16.5)).Shrink(0).Radius(radius(theme, largeRadius)).
		Background(subtleFill(theme)).Border(1, borderColor(theme)).Center().Children(func() {
		ui.Icon(c, icons.devices).Size(space(theme, 8), space(theme, 8)).TextColor(color)
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
