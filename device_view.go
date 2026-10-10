package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

const (
	devicesSidebarWidth = 230
	devicesDetailWidth  = 1080
)

// devicesPage 使用侧栏选择设备，右侧展示当前 Peer 的同步设置。
func (a *application) devicesPage(c *ui.Context, theme *ui.Theme) {
	ui.Row(c).Fill().MinWidth(0).Children(func() {
		a.deviceSidebar(c, theme)
		ui.Box(c).Width(1).FillHeight().Shrink(0).
			Background(theme.Border.Alpha(0.38))
		a.deviceDetail(c, theme)
	})

	if choice := ui.AlertDialog(
		c,
		&a.confirmUnpairDevice,
		"取消配对设备？",
		"取消配对后，该设备将不再与本机同步。",
		"取消",
		"取消配对",
	); choice == 1 {
		a.unpairDevice(a.unpairDeviceID)
	}

	a.addDeviceDialogView(c, theme)
}

// deviceSidebar 是设备页的主从导航。侧栏保持固定宽度，避免设备名称变化时右侧内容跳动。
func (a *application) deviceSidebar(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Width(devicesSidebarWidth).FillHeight().Shrink(0).
		Padding(30, 16, 14, 16).Gap(10).
		Background(theme.Surface.Alpha(0.16)).Children(func() {
		ui.Text(c, "本设备").FontSize(15).FontWeight(600).
			TextColor(theme.TextMuted).Padding(4, 10, 3, 10).SingleLine()
		a.localDeviceEntry(c, theme)

		ui.Row(c).Height(32).Shrink(0).AlignItems(ui.Center).
			Padding(12, 10, 0, 10).Children(func() {
			ui.Text(c, "已配对设备").FontSize(14).FontWeight(600).
				TextColor(theme.TextMuted).SingleLine()
			ui.Spacer(c)
			refresh := ui.ButtonBase(c.Key("refresh-paired-devices")).
				Size(28, 28).Shrink(0).Center().Radius(7).
				TextColor(theme.TextMuted).Label("刷新设备列表").Tooltip("刷新设备列表")
			if refresh.Hovered() {
				refresh.Background(theme.Text.Alpha(0.07))
			}
			refresh.Children(func() {
				ui.Text(c, "↻").FontSize(19).SingleLine()
			})
			if refresh.Clicked() {
				a.devicesMessage = "设备列表已刷新"
			}
		})

		ui.Scroll(c).Grow(1).MinHeight(0).ScrollbarInsets(0, 2, 0, 0).
			Children(func() {
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					for index := range a.pairedDevices {
						a.pairedDeviceEntry(c, theme, index)
					}
				})
			})

		ui.Column(c).Shrink(0).Gap(8).Children(func() {
			a.inviteDeviceButton(c, theme)
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Padding(4, 10).
				Children(func() {
					ui.Box(c).Size(7, 7).Shrink(0).Radius(4).Background(theme.Success)
					ui.Textf(c, "%d 台在线", a.onlineDeviceCount()).FontSize(12).
						TextColor(theme.TextMuted).SingleLine()
				})
			if a.devicesMessage != "" && !a.addDeviceDialog {
				ui.Text(c, a.devicesMessage).FontSize(11).TextColor(theme.TextMuted).
					Padding(10, 0, 2, 10).Wrap()
			}
		})
	})
}

func (a *application) localDeviceEntry(c *ui.Context, theme *ui.Theme) {
	selected := a.selectedDeviceID == ""
	entry := a.deviceEntryButton(c, theme, "local-device", selected)
	entry.Children(func() {
		deviceIcon(c, theme, icons.devices, selected)
		ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
			ui.Text(c, a.deviceName).FontSize(14).FontWeight(600).SingleLine()
			deviceSyncStatus(c, theme, a.deviceSyncEnabled, "同步已开启", "同步已关闭")
		})
	})
	if entry.Clicked() {
		a.selectedDeviceID = ""
		a.deviceInfoExpanded = false
		a.devicesMessage = ""
	}
}

func (a *application) pairedDeviceEntry(c *ui.Context, theme *ui.Theme, index int) {
	device := &a.pairedDevices[index]
	selected := a.selectedDeviceID == device.id
	entry := a.deviceEntryButton(c, theme, "paired-device-"+device.id, selected)
	entry.Children(func() {
		deviceIcon(c, theme, icons.devices, selected)
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, device.name).FontSize(13).FontWeight(600).SingleLine()
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(5).Children(func() {
				ui.Text(c, device.platform).FontSize(11).
					TextColor(theme.TextMuted).SingleLine()
				if device.online {
					ui.Box(c).Size(5, 5).Shrink(0).Radius(3).Background(theme.Success)
				}
			})
		})
	})
	if entry.Clicked() {
		a.selectedDeviceID = device.id
		a.deviceInfoExpanded = true
		a.devicesMessage = ""
	}
}

func (a *application) deviceEntryButton(c *ui.Context, theme *ui.Theme, key string, selected bool) ui.Element {
	entry := ui.ButtonBase(c.Key(key)).Height(72).FillWidth().Shrink(0).
		Padding(10, 12).Gap(10).Radius(12).AlignItems(ui.Center).
		TextColor(theme.Text).Label("选择设备")
	if selected {
		entry.Background(theme.Text.Alpha(0.11))
	} else if entry.Hovered() {
		entry.Background(theme.Text.Alpha(0.06))
	} else {
		entry.Background(ui.Transparent)
	}
	return entry
}

func (a *application) inviteDeviceButton(c *ui.Context, theme *ui.Theme) {
	button := ui.ButtonBase(c.Key("invite-device")).Height(42).FillWidth().Shrink(0).
		Padding(6, 12).Radius(10).Border(1, theme.Border.Alpha(0.65)).
		TextColor(theme.Text).Label("邀请桌面设备")
	if button.Hovered() {
		button.Background(theme.Text.Alpha(0.06))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Text(c, "+").FontSize(24).FontWeight(300).SingleLine()
		ui.Text(c, "邀请设备").FontSize(13).FontWeight(600).SingleLine()
	})
	if button.Clicked() {
		a.devicesMessage = ""
		a.addDeviceDialog = true
	}
}

// deviceDetail 展示侧栏当前选中设备的详情。
func (a *application) deviceDetail(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Grow(1).FillHeight().MinWidth(0).Background(theme.Background).
		Children(func() {
			ui.Scroll(c).Grow(1).MinHeight(0).ScrollbarInsets(0, 6, 0, 0).
				Children(func() {
					ui.Column(c).FillWidth().MaxWidth(devicesDetailWidth).AlignSelf(ui.Center).
						Padding(58, 34, 64, 34).Gap(26).Children(func() {
						a.deviceDetailHeader(c, theme)
						a.deviceSyncSettings(c, theme)
						a.deviceInfoDisclosure(c, theme)
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

	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(18).Children(func() {
		deviceIconLarge(c, theme, paired && !online)
		ui.Column(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, name).FontSize(30).FontWeight(700).SingleLine()
				if paired {
					deviceBadge(c, theme, "已配对")
				} else {
					deviceBadge(c, theme, "本机")
				}
			})
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(7).Children(func() {
				if paired {
					deviceConnectionStatus(c, theme, device)
					return
				}
				deviceSyncStatus(c, theme, syncEnabled, "同步已开启", "同步已关闭")
				ui.Text(c, "·").FontSize(13).TextColor(theme.TextMuted)
				ui.Text(c, platform).FontSize(12).TextColor(theme.TextMuted).SingleLine()
			})
		})
		if paired {
			a.unpairButton(c, theme)
		}
	})
}

func (a *application) unpairButton(c *ui.Context, theme *ui.Theme) {
	button := ui.ButtonBase(c.Key("unpair-selected-device")).
		Height(34).Shrink(0).Padding(5, 10).Radius(9).
		TextColor(theme.Danger).Label("取消配对")
	if button.Hovered() {
		button.Background(theme.Danger.Alpha(0.10))
	} else {
		button.Background(ui.Transparent)
	}
	button.Children(func() {
		ui.Icon(c, icons.clear).Size(14, 14).Shrink(0)
		ui.Text(c, "取消配对").FontSize(13).FontWeight(600).SingleLine()
	})
	if button.Clicked() {
		if device, ok := a.selectedPairedDevice(); ok {
			a.unpairDeviceID = device.id
			a.confirmUnpairDevice = true
		}
	}
}

func deviceConnectionStatus(c *ui.Context, theme *ui.Theme, device *pairedDevice) {
	statusColor := theme.TextMuted
	status := "离线"
	if device.online {
		statusColor = theme.Success
		status = "在线"
	}
	ui.Box(c).Size(8, 8).Shrink(0).Radius(5).Background(statusColor)
	ui.Text(c, status).FontSize(14).FontWeight(600).
		TextColor(statusColor).SingleLine()
	if device.channel != "" {
		ui.Text(c, "·").FontSize(13).TextColor(theme.TextMuted)
		ui.Text(c, device.channel).FontSize(12).TextColor(theme.TextMuted).SingleLine()
	}
}

func (a *application) deviceSyncSettings(c *ui.Context, theme *ui.Theme) {
	device, paired := a.selectedPairedDevice()
	ui.Column(c).FillWidth().Shrink(0).Radius(18).
		Background(theme.Surface.Alpha(0.20)).
		Border(1, theme.Border.Alpha(0.56)).Children(func() {
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

	ui.Row(c).FillWidth().MinHeight(62).Shrink(0).Padding(14, 24).
		AlignItems(ui.Center).Children(func() {
		ui.Text(c, "自定义同步内容").FontSize(16).FontWeight(600).SingleLine()
		ui.Spacer(c)
		button := ui.ButtonBase(c.Key(device.id+"-all-bidirectional")).
			Height(30).Shrink(0).Padding(4, 8).Radius(8).
			TextColor(theme.TextMuted).Label("全部双向同步")
		if button.Hovered() {
			button.Background(theme.Text.Alpha(0.07))
		}
		button.Children(func() {
			ui.Text(c, "全部双向同步").FontSize(12).SingleLine()
		})
		if button.Clicked() {
			a.setAllSyncDirections(device, defaultSyncDirection)
		}
	})
	deviceDivider(c, theme)

	ui.Row(c).FillWidth().MinHeight(54).Shrink(0).Padding(11, 24).
		AlignItems(ui.Center).Children(func() {
		ui.Text(c, "分别设置每种内容的同步方向。").FontSize(12).
			TextColor(theme.TextMuted).SingleLine()
		ui.Spacer(c)
		button := ui.ButtonBase(c.Key(device.id+"-restore-sync-defaults")).
			Height(30).Shrink(0).Padding(4, 8).Radius(8).
			TextColor(theme.TextMuted).Label("恢复默认")
		if button.Hovered() {
			button.Background(theme.Text.Alpha(0.07))
		}
		button.Children(func() {
			ui.Icon(c, icons.clear).Size(13, 13).Shrink(0)
			ui.Text(c, "恢复默认").FontSize(12).SingleLine()
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
		{key: "link", label: "链接", icon: icons.link, direction: &device.linkSyncDirection},
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
	ui.Row(c.Key(device.id+"-sync-content-"+setting.key)).FillWidth().MinHeight(64).Shrink(0).
		Padding(12, 24).AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Icon(c, setting.icon).Size(18, 18).Shrink(0).TextColor(theme.TextMuted)
		ui.Text(c, setting.label).FontSize(15).FontWeight(600).SingleLine()
		ui.Spacer(c)
		ui.Select(c.Key(device.id+"-direction-"+setting.key), setting.direction, directions).
			Width(260).Height(34).Shrink(0).Disabled(!device.syncEnabled).
			Label(setting.label + "同步方向")
	})
}

func (a *application) setAllSyncDirections(device *pairedDevice, direction string) {
	device.textSyncDirection = direction
	device.imageSyncDirection = direction
	device.fileSyncDirection = direction
	device.linkSyncDirection = direction
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
	ui.Row(c.Key("device-setting-"+key)).FillWidth().MinHeight(72).Shrink(0).
		Padding(16, 24).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
			ui.Text(c, title).FontSize(16).FontWeight(600).SingleLine()
			ui.Text(c, description).FontSize(13).TextColor(theme.TextMuted).Wrap()
		})
		ui.Switch(c.Key("switch"), value).Shrink(0).Disabled(disabled).Label(title)
	})
}

func (a *application) deviceInfoDisclosure(c *ui.Context, theme *ui.Theme) {
	device, paired := a.selectedPairedDevice()
	title := "设备信息"
	if paired {
		title = "连接信息"
	}

	values := []deviceInfo{
		{label: "Peer ID", value: a.devicePeerID},
		{label: "平台", value: a.devicePlatform},
		{label: "应用版本", value: settingsVersion},
	}
	if paired {
		values = []deviceInfo{
			{label: "Peer ID", value: device.peerID},
			{label: "通道", value: device.channel},
			{label: "地址", value: device.address},
		}
	}
	ui.Column(c.Key("device-info-card")).FillWidth().Shrink(0).Radius(18).
		Background(theme.Surface.Alpha(0.20)).
		Border(1, theme.Border.Alpha(0.56)).Children(func() {
		button := ui.ButtonBase(c.Key("device-info-disclosure")).FillWidth().Shrink(0).
			MinHeight(54).Padding(12, 24).Radius(18).
			TextColor(theme.Text).Label(deviceInfoDisclosureLabel(title, a.deviceInfoExpanded))
		if button.Hovered() {
			button.Background(theme.Text.Alpha(0.04))
		} else {
			button.Background(ui.Transparent)
		}
		button.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, title).FontSize(16).FontWeight(600).SingleLine()
				ui.Spacer(c)
				rotation := float32(-90)
				if a.deviceInfoExpanded {
					rotation = 0
				}
				ui.Icon(c, icons.chevron).Size(18, 18).Rotate(rotation).
					TextColor(theme.TextMuted)
			})
		})
		if button.Clicked() {
			a.deviceInfoExpanded = !a.deviceInfoExpanded
		}
		if !a.deviceInfoExpanded {
			return
		}
		for index, info := range values {
			if index > 0 {
				deviceDivider(c, theme)
			}
			ui.Row(c.Key("device-info-row-"+info.label)).FillWidth().MinHeight(48).Shrink(0).
				Padding(10, 24).AlignItems(ui.Center).Gap(18).Children(func() {
				ui.Text(c, info.label).Width(120).Shrink(0).FontSize(12).
					TextColor(theme.TextMuted).SingleLine()
				ui.Text(c, info.value).Grow(1).MinWidth(0).FontSize(13).
					TextColor(theme.Text.Alpha(0.9)).SingleLine()
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

// addDeviceDialogView 暂留邀请设备弹窗的视觉容器，配对表单等待后续协议接入。
func (a *application) addDeviceDialogView(c *ui.Context, theme *ui.Theme) {
	ui.Modal(c, &a.addDeviceDialog, func() {
		ui.Box(c).Width(440).Height(240).Shrink(0).Radius(16).
			Background(theme.Background).
			Border(1, theme.Border.Alpha(0.72))
	})
}

func (a *application) unpairDevice(id string) {
	for index, device := range a.pairedDevices {
		if device.id != id {
			continue
		}
		a.pairedDevices = append(a.pairedDevices[:index], a.pairedDevices[index+1:]...)
		break
	}
	a.selectedDeviceID = ""
	a.deviceInfoExpanded = false
	a.unpairDeviceID = ""
	a.confirmUnpairDevice = false
	a.devicesMessage = "已取消配对"
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
	color := theme.TextMuted
	background := theme.Text.Alpha(0.05)
	if selected {
		color = theme.Text
		background = theme.SurfaceHover
	}
	ui.Box(c).Size(36, 36).Shrink(0).Radius(9).
		Background(background).Center().Children(func() {
		ui.Icon(c, icon).Size(18, 18).TextColor(color)
	})
}

func deviceIconLarge(c *ui.Context, theme *ui.Theme, muted bool) {
	color := theme.Text
	if muted {
		color = theme.TextMuted
	}
	ui.Box(c).Size(66, 66).Shrink(0).Radius(16).
		Background(theme.Surface.Alpha(0.32)).
		Border(1, theme.Border.Alpha(0.45)).Center().Children(func() {
		ui.Icon(c, icons.devices).Size(32, 32).TextColor(color)
	})
}

func deviceSyncStatus(c *ui.Context, theme *ui.Theme, enabled bool, onLabel, offLabel string) {
	color := theme.TextMuted
	label := offLabel
	if enabled {
		color = theme.Success
		label = onLabel
	}
	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Box(c).Size(7, 7).Shrink(0).Radius(4).Background(color)
		ui.Text(c, label).FontSize(11).TextColor(color).SingleLine()
	})
}

func deviceBadge(c *ui.Context, theme *ui.Theme, label string) {
	ui.Text(c, label).FontSize(11).FontWeight(600).
		TextColor(theme.TextMuted).Padding(4, 9).Radius(10).
		Background(theme.Text.Alpha(0.08)).SingleLine()
}

func deviceDivider(c *ui.Context, theme *ui.Theme) {
	ui.Box(c).Height(1).FillWidth().Shrink(0).
		Background(theme.Border.Alpha(0.36))
}
