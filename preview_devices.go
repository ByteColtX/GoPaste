//go:build preview

package main

import "time"

// 本文件只在 -tags preview 的构建里编译，提供界面走查用的已配对设备列表。
// 设备发现还没接入，常规构建的对应实现见 preview_data.go，返回空列表。
//
// 这里只有假设备名和 192.168.x 地址，没有任何真实数据，所以它进仓库：预览构建
// 因此在干净克隆里也能编译，走查不再依赖本机的未跟踪文件。
//
// 历史记录不在这里：它们是应用数据库（history.db）里的普通记录，由
// scripts/mockdb 生成到 .preview/history.db，再让应用指向那个文件。

// 预览数据里的同步方向下标，与 syncDirection 的取值一一对应。
const (
	previewDirBoth = iota
	previewDirSend
	previewDirReceive
	previewDirOff
)

// previewDirections 指定一台设备各内容类型的同步方向。
type previewDirections struct {
	text     int
	image    int
	file     int
	richText int
}

// previewDevices 在预览构建里返回走查用的已配对设备列表；常规构建的对应实现
// 见 preview_data.go，它返回空列表。
func previewDevices() []pairedDevice {
	return previewPairedDevices()
}

// previewPairedDevices 构造覆盖 macOS、Windows、Linux、Ubuntu 四种平台，
// 以及在线/离线、同步开关、各内容方向都不同的设备列表。
// 离线设备携带各自的同步设置和上次在线时间戳，
// 用于走查"设备离线时仍显示正确设置"的场景。
func previewPairedDevices() []pairedDevice {
	return []pairedDevice{
		previewDevice("preview-mac", "MacBook Pro", "macOS", "192.168.1.24", true, true, time.Time{},
			previewDirections{}),
		previewDevice("preview-windows", "工作室 PC", "Windows", "192.168.1.31", false, true, mockTimestamp(1, 9, 12),
			previewDirections{image: previewDirSend, file: previewDirOff}),
		previewDevice("preview-imac-design", "设计部 iMac", "macOS", "192.168.1.42", true, true, time.Time{},
			previewDirections{image: previewDirSend}),
		previewDevice("preview-surface", "Surface Laptop", "Windows", "192.168.1.55", true, true, time.Time{},
			previewDirections{file: previewDirOff}),
		previewDevice("preview-thinkpad", "ThinkPad X1", "Linux", "192.168.1.63", true, true, time.Time{},
			previewDirections{richText: previewDirReceive}),
		previewDevice("preview-ubuntu-workstation", "Ubuntu 工作站", "Ubuntu", "192.168.1.71", false, true, mockTimestamp(1, 21, 40),
			previewDirections{text: previewDirReceive, image: previewDirSend, file: previewDirOff}),
		previewDevice("preview-mac-mini", "会议室 Mac mini", "macOS", "192.168.1.88", true, true, time.Time{},
			previewDirections{}),
		previewDevice("preview-windows-finance", "财务 PC", "Windows", "192.168.1.95", false, false, mockTimestamp(3, 17, 30),
			previewDirections{image: previewDirSend, richText: previewDirOff}),
		previewDevice("preview-fedora", "开发机 Fedora", "Linux", "192.168.1.102", true, true, time.Time{},
			previewDirections{image: previewDirOff, file: previewDirSend}),
		previewDevice("preview-macbook-air", "MacBook Air", "macOS", "192.168.1.117", true, true, time.Time{},
			previewDirections{}),
		previewDevice("preview-windows-dev", "开发 Windows", "Windows", "192.168.1.126", false, true, mockTimestamp(2, 8, 55),
			previewDirections{image: previewDirSend, file: previewDirReceive, richText: previewDirOff}),
		previewDevice("preview-ubuntu-laptop", "备用 Ubuntu 笔记本", "Ubuntu", "192.168.1.140", false, true, mockTimestamp(4, 14, 20),
			previewDirections{image: previewDirReceive, file: previewDirOff}),
	}
}

// previewDevice 生成一台已配对设备；lastSeenAt 为零值表示没有上次在线记录，
// 未指定的内容类型默认双向同步。
func previewDevice(id, name, platform, address string, online, syncEnabled bool, lastSeenAt time.Time, dirs previewDirections) pairedDevice {
	return pairedDevice{
		id: id, name: name, peerID: id,
		platform: platform, version: settingsVersion, channel: "局域网",
		address: address, online: online, syncEnabled: syncEnabled,
		lastSeenAt:            lastSeenAt,
		textSyncDirection:     syncDirection(dirs.text),
		imageSyncDirection:    syncDirection(dirs.image),
		fileSyncDirection:     syncDirection(dirs.file),
		richTextSyncDirection: syncDirection(dirs.richText),
	}
}

// mockTimestamp 返回相对现在往前 daysAgo 天、指定时刻的演示时间戳，
// 用于构造离线设备的「上次在线」时间。
func mockTimestamp(daysAgo, hour, minute int) time.Time {
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	return day.AddDate(0, 0, -daysAgo)
}
