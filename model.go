package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	// 窗口和分栏的最小尺寸，保证窄窗口下内容仍然可读。
	navigationRailWidth   = 64
	minHistoryPaneWidth   = 300
	minPreviewPaneWidth   = 460
	splitDividerWidth     = 1
	windowChromeMinHeight = 36
	windowTitleBarHeight  = 52

	defaultHistoryPaneWidth = 320
	historyItemHeight       = 82
	historyTextLimit        = 88
)

const (
	copiedStatus          = "已复制到系统剪贴板"
	favoriteAddedStatus   = "已加入收藏"
	favoriteRemovedStatus = "已取消收藏"
	settingsVersion       = "0.1.0"
)

const (
	appearanceSystem int = iota
	appearanceLight
	appearanceDark
)

var languageOptions = []string{"简体中文", "English"}

// desktopPlatforms 限定当前同步协议支持的桌面端平台。
var desktopPlatforms = []string{"macOS", "Windows", "Linux"}

const defaultSyncDirection = "双向同步"

// syncDirectionOptionsForPeer 根据当前 Peer 名称生成同步方向选项，
// 让发送和接收方向清楚表达内容的去向。
func syncDirectionOptionsForPeer(peerName string) []string {
	peerName = strings.TrimSpace(peerName)
	if peerName == "" {
		peerName = "该设备"
	}
	return []string{
		defaultSyncDirection,
		fmt.Sprintf("仅发往 %s", peerName),
		fmt.Sprintf("仅从 %s 接收", peerName),
		"不同步",
	}
}

// settingsArea 表示设置页左侧导航中的当前主题。
type settingsArea uint8

const (
	settingsGeneral settingsArea = iota
	settingsAppearance
	settingsStorage
	settingsAbout
)

// dangerousAction 标记需要用户再次确认的高影响操作。
type dangerousAction uint8

const (
	dangerousActionNone dangerousAction = iota
	dangerousActionClearHistory
	dangerousActionUnpairDevice
)

type page uint8

const (
	pageHistory page = iota
	pageDevices
	pageSettings
)

type historyFilter uint8

const (
	filterAll historyFilter = iota
	filterText
	filterLinks
	filterCode
	filterImages
	filterFiles
	filterFavorites
)

type contentKind uint8

const (
	kindText contentKind = iota
	kindLink
	kindCode
	kindImage
	kindFile
)

// displayName 返回面向用户显示的内容类型名称。
func (kind contentKind) displayName() string {
	switch kind {
	case kindLink:
		return "链接"
	case kindCode:
		return "代码"
	case kindImage:
		return "图片"
	case kindFile:
		return "文件"
	default:
		return "文本"
	}
}

type clipboardItem struct {
	id        string
	kind      contentKind
	content   string
	image     *clipboardImage
	source    string
	timestamp string
	favorite  bool
}

// clipboardImage 保存剪贴板图片的原始 PNG 和已经解码的位图。
// 原始数据用于复制，位图用于列表缩略图和详情预览。
type clipboardImage struct {
	png    []byte
	bitmap *ui.Bitmap
	width  int
	height int
}

// pairedDevice 保存一个已配对设备的运行时状态。
// 这些字段先保存在内存中，后续接入持久化同步服务时可直接替换数据来源。
type pairedDevice struct {
	id       string
	name     string
	peerID   string
	platform string
	version  string
	channel  string
	address  string
	online   bool

	syncEnabled           bool
	textSyncDirection     string
	imageSyncDirection    string
	fileSyncDirection     string
	linkSyncDirection     string
	richTextSyncDirection string
}

// application 保存窗口在多次重绘之间需要延续的状态。UI 元素本身不放在这里，
// 因为 MyGo 的元素只在当前构建帧中有效。
type application struct {
	page page

	historyPaneWidth float32
	filter           historyFilter
	query            string

	items         []clipboardItem
	selectedID    string
	selectedIndex int
	listState     ui.ListState

	deviceName               string
	devicePeerID             string
	devicePlatform           string
	deviceSyncEnabled        bool
	deviceFileSync           bool
	pairedDevices            []pairedDevice
	selectedDeviceID         string
	deviceInfoExpanded       bool
	addDeviceDialog          bool
	launchAtLogin            bool
	language                 string
	appearance               int
	settingsArea             settingsArea
	confirmDangerousAction   bool
	pendingDangerousAction   dangerousAction
	pendingDangerousDeviceID string
}

// baseKind 返回记录的基础类型。链接和代码属于文本，只作为智能标签显示。
func (item clipboardItem) baseKind() contentKind {
	switch item.kind {
	case kindLink, kindCode:
		return kindText
	default:
		return item.kind
	}
}

// smartTag 返回自动识别出的标签。收藏状态由用户手动维护，不在这里返回。
func (item clipboardItem) smartTag() string {
	switch item.kind {
	case kindLink:
		return "链接"
	case kindCode:
		return "代码"
	default:
		return ""
	}
}

// listText 返回历史列表中的单行正文。
func (item clipboardItem) listText() string {
	if item.kind == kindImage {
		return item.displayLabel()
	}
	content := strings.TrimSpace(item.content)
	if content == "" {
		return item.kind.displayName()
	}
	return truncateSingleLine(content, historyTextLimit)
}

// displayLabel 返回图片条目的类型和分辨率。
func (item clipboardItem) displayLabel() string {
	if item.kind == kindImage && item.image != nil {
		return fmt.Sprintf("图片（%dx%d）", item.image.width, item.image.height)
	}
	return item.baseKind().displayName()
}

// resolutionLabel 返回图片在历史列表中显示的分辨率。
func (item clipboardItem) resolutionLabel() string {
	if item.image == nil {
		return "图片"
	}
	return fmt.Sprintf("%dx%d", item.image.width, item.image.height)
}

func displayValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	return value
}
