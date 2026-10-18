package main

import (
	"strings"
	"time"

	"gopaste/internal/lansync"

	"github.com/egoist/mygo/ui"
)

// devicesPage 只渲染详情。设备列表是侧栏的一部分，见 filterRail 的 sidebarDevices。
func (a *application) devicesPage(c *ui.Context, theme *ui.Theme) {
	a.deviceDetail(c, theme)
	a.addDeviceDialogView(c, theme)
}

func (a *application) deviceEmptyState(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		ui.Icon(c, icons.devices).Size(space(theme, 5), space(theme, 5)).
			TextColor(secondaryText(theme))
		labelText(c, a.t(msgDevicesEmptyTitle)).FontSize(font(theme, fontControl)).FontWeight(600)
		ui.Text(c, a.t(msgDevicesEmptyMessage)).
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
	})
}

func (a *application) deviceLoadingState(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Padding(space(theme, 5), space(theme, 3)).
		AlignItems(ui.Center).Gap(space(theme, 1.25)).Children(func() {
		statusDot(c, theme.Accent, space(theme, 2))
		labelText(c, a.t(msgDevicesLoadingTitle)).FontSize(font(theme, fontControl)).FontWeight(600)
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
		labelText(c, a.t(msgDevicesErrorTitle)).FontSize(font(theme, fontControl)).FontWeight(600).
			TextColor(theme.Danger)
		ui.Text(c, message).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
		retryLabel := a.t(msgDevicesRetry)
		retry := neutralButton(c, theme, "retry-paired-devices", retryLabel)
		retry.Children(func() {
			labelText(c, retryLabel).FontSize(font(theme, fontControl))
		})
		if retry.Clicked() {
			a.refreshDevices(c)
		}
	})
}

// startDeviceRefresh 开始刷新设备列表，返回需要提示给用户的文案（空表示无需提示）。
// 设备发现服务未接入时返回不可用提示。窗口按钮、快捷键与 macOS 菜单项共用它。
//
// now 是调用方所在时间线的当前时刻：构建帧里的调用方传 c.Now()，最短转圈时间
// 因此与图标动画（Loop）走同一条时钟，测试里也能用 Tester 的时钟推进而不用
// 真的等待。菜单回调没有构建帧，只能传系统时钟 time.Now()。
func (a *application) startDeviceRefresh(now time.Time) string {
	// 无论后端是否可用都先起一段最短转圈：点击必须有回应，否则按钮看起来
	// 像没接上。转圈本身不代表结果，结果仍由下面的状态与提示给出。
	a.deviceRefreshSpinUntil = now.Add(motionSpinPeriod)
	if a.syncService == nil {
		return a.t(msgToastDeviceDiscoveryUnavailable)
	}
	// 转圈只表示「点击有回应」：刷新结果的到达时间由发现层决定，界面上不再挂一个
	// 自己不会结束的加载态。
	a.syncService.Refresh()
	return ""
}

// refreshDevices 是窗口内的刷新入口：开始刷新并直接提示。
func (a *application) refreshDevices(c *ui.Context) {
	if message := a.startDeviceRefresh(c.Now()); message != "" {
		c.Toast(message)
	}
}

// requestDeviceRefresh 由 macOS 菜单项调用：菜单回调没有构建帧的 ui.Context，
// 无法直接提示，所以把文案排队到下一帧，与撤销的排队同一套做法。时间取系统
// 时钟，与窗口帧里的 c.Now() 在生产中同源。
func (a *application) requestDeviceRefresh() {
	if message := a.startDeviceRefresh(time.Now()); message != "" {
		a.pendingToast = message
	}
}

// deviceRefreshSpinning 报告刷新按钮是否该转圈：发现仍在进行，或者上一次点击的
// 最短转圈时间还没走完。后者让后端返回得再快，点击也看得见反馈。now 与
// startDeviceRefresh 同源：帧里传 c.Now()。
func (a *application) deviceRefreshSpinning(now time.Time) bool {
	return a.deviceDiscovery == deviceDiscoveryLoading || now.Before(a.deviceRefreshSpinUntil)
}

// deviceDisplayName 返回设备页显示的本机名称。默认名就是系统主机名，macOS 上
// 带 .local 后缀，设备页显示时去掉它；设置页的输入框仍显示完整主机名。用户改
// 过的名字原样显示，不受后缀影响。
func (a *application) deviceDisplayName() string {
	if a.deviceName == a.deviceNameDefault {
		return strings.TrimSuffix(a.deviceName, ".local")
	}
	return a.deviceName
}

func (a *application) localDeviceEntry(c *ui.Context, theme *ui.Theme) {
	name := a.deviceDisplayName()
	selected := a.selectedDeviceID == ""
	entry := listRow(c, theme, "local-device", name, selected,
		func() { deviceRowTile(c, theme, iconForPlatform(a.devicePlatform), selected) },
		func() {
			ui.Text(c, name).FontSize(font(theme, fontBody)).FontWeight(500).SingleLine()
			deviceSyncStatus(c, theme, a.deviceSyncEnabled, a.t(msgDevicesSyncOn), a.t(msgDevicesSyncOff))
		})
	if entry.Clicked() {
		a.selectedDeviceID = ""
		a.deviceInfoExpanded = false
	}
}

func (a *application) pairedDeviceEntry(c *ui.Context, theme *ui.Theme, index int) {
	device := &a.pairedDevices[index]
	selected := a.selectedDeviceID == device.id
	entry := listRow(c, theme, "paired-device-"+device.id, device.name, selected,
		func() { deviceRowTile(c, theme, iconForPlatform(device.platform), selected) },
		func() {
			ui.Text(c, device.name).FontSize(font(theme, fontBody)).FontWeight(500).SingleLine()
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1)).Children(func() {
				deviceStatus(c, theme, a.connectionLabel(device.online), connectionColor(theme, device.online))
			})
		})
	if entry.Clicked() {
		a.selectedDeviceID = device.id
		a.deviceInfoExpanded = true
	}
}

// openAddDeviceDialog 打开配对对话框，并清掉上一次尝试留下的状态。侧栏底栏的
// 「+」、文件菜单的「新增设备」与 Cmd+N 都走这里。
//
// 正在等待配对码或正在输入对方的配对码时不清状态：重新打开对话框不该打断一次
// 正在进行的配对。
func (a *application) openAddDeviceDialog() {
	a.addDeviceDialog = true
	a.pairingError = ""
	a.pairCodeInput = ""
	if a.pairing == pairingWaiting || a.pairing == pairingAnswering {
		return
	}
	a.pairing = pairingIdle
	a.pairingCode = ""
	a.pairingPeerName = ""
	a.discoveredPeers = a.pairablePeers()
}

// sidebarIconAction 是侧栏列表分组标题行里的小图标动作按钮（刷新）：与分区切换器
// 同一套尺寸、圆角与悬停反馈，但没有选中态。文字放在提示与辅助技术标签里。
//
// spinning 让图标连续转圈，用于「正在进行」的动作（刷新）。
func (a *application) sidebarIconAction(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, spinning bool, onClick func()) {
	size := controlHeightSize(c, theme, controlHeight)
	button := ui.ButtonBase(c.Key(key)).Size(size, size).Shrink(0).Center().
		Radius(radius(theme, controlRadius)).TextColor(secondaryText(theme)).
		Label(label).Tooltip(label).Transition(motionColors)
	if button.Hovered() {
		button.Background(hoverFill(theme))
	}
	button.Children(func() {
		glyph := ui.Icon(c, icon).Size(space(theme, 4), space(theme, 4))
		if spinning {
			// Loop 每帧重算进度并请求下一帧，图标因此连续转圈。key 让这段动画
			// 只属于这个按钮：转圈期间重新构建视图不会把它从头开始。
			glyph.Rotate(button.Loop("refresh-spin", motionSpinPeriod, ui.Linear) * 360)
		}
	})
	if button.Clicked() {
		onClick()
	}
}

// sidebarWideButton 是侧栏底栏里的宽按钮：图标加文字，宽度由内容决定。高度和
// 圆角与分区切换器一致，但没有选中态；表面色加描边沿用搜索框的写法，因此它读
// 起来是一个可点的控件，而不是一条列表项。
//
// 与 sidebarIconAction 的分工：图标按钮留给列表作用域的次要动作（刷新），
// 宽按钮留给页面主操作，文字可见，不必悬停才发现。
func (a *application) sidebarWideButton(c *ui.Context, theme *ui.Theme, key string, icon *ui.SVG, label string, onClick func()) {
	button := ui.ButtonBase(c.Key(key)).Height(controlHeightSize(c, theme, compactHeight)).Shrink(0).
		Padding(space(theme, 1), space(theme, 2.5)).Gap(space(theme, 1.5)).
		Radius(radius(theme, controlRadius)).Border(1, borderColor(theme)).
		TextColor(primaryText(theme)).Label(label).Role(ui.RoleButton).Transition(motionColors)
	if button.Hovered() {
		button.Background(hoverFill(theme))
	} else {
		button.Background(theme.Background)
	}
	button.Children(func() {
		ui.Icon(c, icon).Size(space(theme, 3.5), space(theme, 3.5)).Shrink(0)
		labelText(c, label).FontSize(font(theme, fontControl)).FontWeight(600)
	})
	if button.Clicked() {
		onClick()
	}
}

// deviceDetail 展示侧栏当前选中设备的详情。
func (a *application) deviceDetail(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Grow(1).FillHeight().MinWidth(0).Background(theme.Background).
		Children(func() {
			paneTop(c)
			ui.Scroll(c).Grow(1).MinHeight(0).ScrollbarInsets(0, space(theme, 1), 0, 0).
				Children(func() {
					ui.Column(c).FillWidth().MaxWidth(detailMaxWidth(theme)).AlignSelf(ui.Center).
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
				labelText(c, a.t(msgDevicesOfflineTitle)).FontSize(font(theme, fontControl)).FontWeight(600).
					TextColor(theme.Warning)
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
	name := a.deviceDisplayName()
	platform := a.devicePlatform
	online := true
	syncEnabled := a.deviceSyncEnabled
	if paired {
		name = device.name
		platform = device.platform
		online = device.online
		syncEnabled = device.syncEnabled
	}

	ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 4.5)).DragWindow().Children(func() {
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
				labelText(c, platform).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme))
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
		labelText(c, unpairLabel).FontSize(font(theme, fontControl)).FontWeight(600).
			TextColor(theme.Danger)
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
	labelText(c, a.t(msgDevicesDirect)).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
	if device.platform != "" {
		ui.Text(c, "·").FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
		labelText(c, device.platform).FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme))
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
			labelText(c, a.t(msgDevicesSyncContent)).FontSize(font(theme, fontBody)).FontWeight(500)
			ui.Text(c, a.t(msgDevicesSyncContentDesc)).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme)).Wrap()
		})
		deviceBadge(c, theme, a.syncContentModeLabel(device))
	})
	deviceDivider(c, theme)

	settings := []syncContentSetting{
		{key: "text", label: a.t(msgKindText), icon: icons.text, direction: &device.textSyncDirection, contentType: lansync.ContentText},
		{key: "image", label: a.t(msgKindImage), icon: icons.image, direction: &device.imageSyncDirection, contentType: lansync.ContentImage},
		{key: "file", label: a.t(msgKindFile), icon: icons.file, direction: &device.fileSyncDirection, contentType: lansync.ContentFile},
		{key: "rich-text", label: a.t(msgKindRichText), icon: icons.text, direction: &device.richTextSyncDirection, contentType: lansync.ContentRichText},
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
	key         string
	label       string
	icon        *ui.SVG
	direction   *syncDirection
	contentType lansync.ContentType
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
	if a.compactDetailLayout(c, theme) {
		ui.Column(c.Key(device.id+"-sync-content-"+setting.key)).FillWidth().Shrink(0).
			Padding(space(theme, 3), space(theme, 4)).Gap(space(theme, 1.5)).Children(func() {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 2)).Children(func() {
				ui.Icon(c, setting.icon).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0).
					TextColor(secondaryText(theme))
				ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
					labelText(c, setting.label).FontSize(font(theme, fontBody)).FontWeight(500)
					ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
				})
			})
			field := selectField(c, theme, device.id+"-direction-"+setting.key, setting.direction, selectLabel, directions).
				FillWidth().Disabled(disabled)
			if field.Changed() {
				a.setDeviceDirection(device.id, setting.contentType, *setting.direction)
			}
		})
		return
	}

	ui.Row(c.Key(device.id+"-sync-content-"+setting.key)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
		Padding(space(theme, 3), space(theme, 4)).AlignItems(ui.Center).Gap(space(theme, 3)).Children(func() {
		ui.Icon(c, setting.icon).Size(space(theme, 4.5), space(theme, 4.5)).Shrink(0).TextColor(secondaryText(theme))
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
			labelText(c, setting.label).FontSize(font(theme, fontBody)).FontWeight(500)
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		field := selectField(c, theme, device.id+"-direction-"+setting.key, setting.direction, selectLabel, directions).
			Width(260).Disabled(disabled)
		if field.Changed() {
			a.setDeviceDirection(device.id, setting.contentType, *setting.direction)
		}
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
			labelText(c, title).FontSize(font(theme, fontBody)).FontWeight(500)
			ui.Text(c, description).FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).Wrap()
		})
		switchControl := ui.Switch(c.Key("device-switch-"+key), value).
			Shrink(0).Disabled(disabled).Label(title)
		if switchControl.Changed() {
			switch key {
			case "local-sync":
				// 页面开关和托盘开关共用同一条状态更新路径，避免设备状态不一致。
				a.setSyncEnabled(a.deviceSyncEnabled)
			case "local-file-sync":
				// 本机的文件同步开关只影响本机，没有要写回的服务状态。
			default:
				// 单台设备的开关：写回同步服务并落盘。
				a.toggleDeviceSync(strings.TrimSuffix(key, "-sync"))
			}
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
				labelText(c, title).FontSize(font(theme, fontBody)).FontWeight(600)
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
		if a.compactDetailLayout(c, theme) {
			labelWidth = 96
			rowGap = 2.5
		}
		for index, info := range values {
			if index > 0 {
				deviceDivider(c, theme)
			}
			ui.Row(c.Key("device-info-row-"+info.key)).FillWidth().MinHeight(space(theme, rowHeight)).Shrink(0).
				Padding(space(theme, 2.5), space(theme, 4)).AlignItems(ui.Center).Gap(space(theme, rowGap)).Children(func() {
				labelText(c, info.label).Width(space(theme, labelWidth/4)).Shrink(0).FontSize(font(theme, fontMeta)).
					TextColor(secondaryText(theme))
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

// addDeviceDialogView 是配对对话框：列出发现到、还没配对的设备，等待对方输入配对码
// 时显示本机生成的 6 位码，对方发起配对时要求输入对方的码。外观走 dialog 的共享
// 外壳：与二次确认同一个宽度、内边距、表面和按钮行，主操作按钮在右。
func (a *application) addDeviceDialogView(c *ui.Context, theme *ui.Theme) {
	dialog(c, theme, &a.addDeviceDialog, dialogSpec{
		title:      a.t(msgDevicesAdd),
		role:       ui.RoleDialog,
		message:    a.t(msgDevicesPairDescription),
		badge:      a.t(msgDevicesPairDirectOnly),
		badgeColor: accentColor(theme),
		content: func() {
			if a.pairingError != "" {
				ui.Text(c, a.pairingError).FontSize(font(theme, fontMeta)).
					TextColor(theme.Danger).Wrap()
			}
			switch a.pairing {
			case pairingWaiting:
				a.pairingCodeContent(c, theme)
			case pairingAnswering:
				a.pairingAnswerContent(c, theme)
			default:
				a.pairingDeviceList(c, theme)
			}
		},
		actions: func() {
			switch a.pairing {
			case pairingWaiting:
				close := dialogDismissButton(c, theme, "cancel-pairing", a.t(msgDevicesPairClose))
				if close.Clicked() {
					a.cancelPairing()
				}
			case pairingAnswering:
				decline := dialogDismissButton(c, theme, "decline-pairing", a.t(msgDevicesPairDecline))
				if decline.Clicked() {
					a.declinePairRequest()
				}
				confirmLabel := a.t(msgDevicesPairConfirm)
				confirm := dialogPrimaryButton(c, theme, "confirm-pairing", confirmLabel)
				if confirm.Clicked() {
					a.confirmPairing(a.pairCodeInput)
				}
			default:
				close := dialogDismissButton(c, theme, "close-invite-device", a.t(msgDevicesPairClose))
				if close.Clicked() {
					a.addDeviceDialog = false
				}
				if a.syncService == nil {
					return
				}
				refreshLabel := a.t(msgDevicesPairRefresh)
				refresh := neutralButton(c, theme, "refresh-pairable-devices", refreshLabel)
				refresh.Children(func() {
					ui.Text(c, refreshLabel).FontSize(font(theme, fontControl)).SingleLine()
				})
				if refresh.Clicked() {
					a.refreshDevices(c)
				}
			}
		},
	})
}

// pairingDeviceList 列出发现到、还没配对的设备；一台都没有时说明原因，并留一个
// 刷新入口。
func (a *application) pairingDeviceList(c *ui.Context, theme *ui.Theme) {
	if a.syncService == nil {
		ui.Text(c, a.t(msgDevicesPairWaitingSvc)).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).Wrap()
		return
	}
	peers := a.pairablePeers()
	if len(peers) == 0 {
		message := a.t(msgDevicesPairNoneFound)
		if a.deviceDiscovery == deviceDiscoveryError && a.deviceDiscoveryMessage != "" {
			message = a.deviceDiscoveryMessage
		}
		ui.Text(c, message).FontSize(font(theme, fontCaption)).
			TextColor(secondaryText(theme)).Wrap()
		return
	}
	ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 1)).Children(func() {
		for _, peer := range peers {
			a.pairablePeerRow(c, theme, peer)
		}
	})
}

// pairablePeerRow 是一台可配对的设备：名字、平台，和右边的配对按钮。
func (a *application) pairablePeerRow(c *ui.Context, theme *ui.Theme, peer lansync.Peer) {
	ui.Row(c.Key("pairable-"+peer.ID)).FillWidth().Shrink(0).
		MinHeight(space(theme, rowHeight)).Padding(space(theme, 2), space(theme, 2)).
		AlignItems(ui.Center).Gap(space(theme, 2)).Children(func() {
		deviceRowTile(c, theme, iconForPlatform(peer.Platform), false)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
			name := peer.Name
			if name == "" {
				name = peer.ID
			}
			ui.Text(c, name).FontSize(font(theme, fontBody)).FontWeight(500).SingleLine()
			ui.Text(c, displayValue(peer.Platform)).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme)).SingleLine()
		})
		label := a.t(msgDevicesPairStart)
		pair := neutralButton(c, theme, "pair-with-"+peer.ID, label)
		pair.Children(func() {
			ui.Text(c, label).FontSize(font(theme, fontControl)).SingleLine()
		})
		if pair.Clicked() {
			a.startPairing(peer.ID)
		}
	})
}

// pairingCodeContent 显示本机生成、等待对方输入的 6 位配对码。
func (a *application) pairingCodeContent(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Shrink(0).AlignItems(ui.Center).
		Gap(space(theme, 1.5)).Padding(space(theme, 1), 0, space(theme, 1), 0).Children(func() {
		ui.Text(c, a.tf(msgDevicesPairCodeFor, displayValue(a.pairingPeerName))).
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
		code := a.pairingCode
		if code == "" {
			code = "······"
		}
		ui.Text(c, code).FontSize(font(theme, fontPageTitle)).FontWeight(700).
			LetterSpacing(space(theme, 1)).SingleLine()
		ui.Text(c, a.t(msgDevicesPairCodeHint)).FontSize(font(theme, fontMeta)).
			TextColor(secondaryText(theme)).Wrap()
	})
}

// pairingAnswerContent 要求输入对方显示的配对码。
func (a *application) pairingAnswerContent(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 1.5)).Children(func() {
		ui.Text(c, a.tf(msgDevicesPairRequestFrom, displayValue(a.pairingPeerName))).
			FontSize(font(theme, fontCaption)).TextColor(secondaryText(theme)).Wrap()
		label := a.t(msgDevicesPairCodeLabel)
		field := ui.Row(c.Key("pair-code-field")).Height(controlHeightSize(c, theme, controlHeight)).
			FillWidth().Shrink(0).AlignItems(ui.Center).Padding(0, space(theme, 2.5)).
			Radius(radius(theme, controlRadius)).Background(theme.Background).
			Border(1, borderColor(theme)).Transition(motionColors).Label(label)
		field.Children(func() {
			input := ui.TextInput(c.Key("pair-code-input"), &a.pairCodeInput).
				Grow(1).MinWidth(0).Padding(0).Radius(0).
				Background(ui.Transparent).Border(0, ui.Transparent).
				Label(label).Placeholder(a.t(msgDevicesPairCodePlaceholder))
			if input.Submitted() {
				a.confirmPairing(a.pairCodeInput)
			}
		})
		ui.Text(c, a.t(msgDevicesPairCodeHint)).FontSize(font(theme, fontMeta)).
			TextColor(secondaryText(theme)).Wrap()
	})
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

// deviceRowTile 渲染设备条目左侧的图标块：外框尺寸取自 listRowTile，与历史记录
// 行的图标块同一大小。
func deviceRowTile(c *ui.Context, theme *ui.Theme, icon *ui.SVG, selected bool) {
	color := secondaryText(theme)
	background := subtleFill(theme)
	if selected {
		color = primaryText(theme)
		background = hoverFill(theme)
	}
	listRowTile(c, theme, background, func() {
		ui.Icon(c, icon).Size(space(theme, 4.5), space(theme, 4.5)).TextColor(color)
	})
}

func deviceIconLarge(c *ui.Context, theme *ui.Theme, platform string, muted bool) {
	color := theme.Text
	if muted {
		color = theme.TextMuted
	}
	ui.Box(c).Size(space(theme, 16.5), space(theme, 16.5)).Shrink(0).Radius(radius(theme, cardRadius)).
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
		labelText(c, label).FontSize(font(theme, fontCaption)).
			TextColor(color)
	})
}

func deviceBadge(c *ui.Context, theme *ui.Theme, label string) {
	labelText(c, label).FontSize(font(theme, fontCaption)).FontWeight(600).
		TextColor(secondaryText(theme)).Padding(space(theme, 1), space(theme, 2.25)).Radius(radius(theme, cardRadius)).
		Background(subtleFill(theme))
}

func deviceDivider(c *ui.Context, theme *ui.Theme) {
	standardDivider(c, theme)
}
