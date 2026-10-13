package main

import (
	"strings"
	"time"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

const (
	// 窗口和分栏的最小尺寸，保证窄窗口下内容仍然可读。
	navigationRailWidth   = navigationRailCompactWidth
	minHistoryPaneWidth   = 300
	minPreviewPaneWidth   = 460
	splitDividerWidth     = 1
	windowChromeMinHeight = 36
	windowTitleBarHeight  = 52

	defaultHistoryPaneWidth = 320
	historyItemHeight       = 82
	historyTextLimit        = 88
)

const settingsVersion = "0.1.0"

const (
	appearanceSystem int = iota
	appearanceLight
	appearanceDark
)

// desktopPlatforms 限定当前同步协议支持的桌面端平台。
var desktopPlatforms = []string{"macOS", "Windows", "Linux"}

// syncDirection is the language-independent direction stored for each content
// type. Labels are derived from the active language at render time, so
// switching languages never rewrites the stored value.
type syncDirection uint8

const (
	syncDirectionTwoWay syncDirection = iota
	syncDirectionSendOnly
	syncDirectionReceiveOnly
	syncDirectionOff
)

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
	dangerousActionDeleteItem
)

type page uint8

const (
	pageHistory page = iota
	pageDevices
	pageSettings
)

type historyFilter = history.Filter

const (
	filterAll       = history.FilterAll
	filterText      = history.FilterText
	filterLinks     = history.FilterLinks
	filterCode      = history.FilterCode
	filterImages    = history.FilterImages
	filterFiles     = history.FilterFiles
	filterFavorites = history.FilterFavorites
)

type contentKind = history.Kind

const (
	kindText  = history.KindText
	kindLink  = history.KindLink
	kindCode  = history.KindCode
	kindImage = history.KindImage
	kindFile  = history.KindFile
)

type syncState = history.SyncState

const (
	syncStateNone     = history.SyncStateNone
	syncStateSynced   = history.SyncStateSynced
	syncStatePartial  = history.SyncStatePartial
	syncStateOffline  = history.SyncStateOffline
	syncStateReplaced = history.SyncStateReplaced
)

type deliveryState = history.DeliveryState

const (
	deliveryPending  = history.DeliveryPending
	deliveryReceived = history.DeliveryReceived
	deliveryOffline  = history.DeliveryOffline
)

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
	// lastSeenAt 记录设备最近一次在线的时间，零值表示未知。
	lastSeenAt time.Time

	syncEnabled           bool
	textSyncDirection     syncDirection
	imageSyncDirection    syncDirection
	fileSyncDirection     syncDirection
	richTextSyncDirection syncDirection
}

// deviceDiscoveryState describes the device provider state exposed by the UI.
// The current preview has no discovery backend, so it remains ready by default;
// a future LAN adapter can publish loading and error states without changing
// the page structure.
type deviceDiscoveryState uint8

const (
	deviceDiscoveryReady deviceDiscoveryState = iota
	deviceDiscoveryLoading
	deviceDiscoveryError
)

// application 保存窗口在多次重绘之间需要延续的状态。UI 元素本身不放在这里，
// 因为 MyGo 的元素只在当前构建帧中有效。
type application struct {
	page page

	onSyncStateChanged func()
	onLanguageChanged  func()

	historyPaneWidth float32
	filter           historyFilter
	query            string
	history          *history.Store
	historyBitmaps   map[string]*ui.Bitmap
	clipboard        history.ClipboardWriter
	historySearch    ui.Handle
	listState        ui.ListState

	onResendItem func(itemID, deviceID string)
	// syncPopoverItem 与 syncPopoverOpen 记录预览区同步状态浮层的展开状态：
	// 浮层由点击状态标记展开，点击别处或按 Esc 收起，指针滑过不会触发。
	syncPopoverItem          string
	syncPopoverOpen          bool
	deviceName               string
	devicePeerID             string
	devicePlatform           string
	deviceSyncEnabled        bool
	deviceFileSync           bool
	pairedDevices            []pairedDevice
	deviceDiscovery          deviceDiscoveryState
	deviceDiscoveryMessage   string
	onRefreshDevices         func()
	selectedDeviceID         string
	deviceInfoExpanded       bool
	addDeviceDialog          bool
	pairingPending           bool
	pairingError             string
	onStartPairing           func()
	launchAtLogin            bool
	language                 language
	appearance               int
	settingsArea             settingsArea
	confirmDangerousAction   bool
	pendingDangerousAction   dangerousAction
	pendingDangerousTargetID string

	// theme 是收敛后的当前帧主题，跨帧复用同一份存储以避免每帧分配。
	theme ui.Theme
}

// l10n returns the localizer for the current UI language.
func (a *application) l10n() localizer { return newLocalizer(a.language) }

// t returns the message for key in the current UI language.
func (a *application) t(key messageKey) string { return a.language.catalog().text(key) }

// tf returns the message for key with its verbs filled in.
func (a *application) tf(key messageKey, args ...any) string {
	return a.l10n().format(key, args...)
}

// kindLabel names a content kind in the current UI language.
func (a *application) kindLabel(kind contentKind) string { return a.l10n().kindLabel(kind) }

// kindSearchTerms returns every name a content kind can be searched by, across
// all languages, so search keeps working whichever language is displayed.
func (a *application) kindSearchTerms(kind contentKind) []string {
	return []string{
		newLocalizer(languageSimplifiedChinese).kindLabel(kind),
		newLocalizer(languageEnglish).kindLabel(kind),
	}
}

func displayValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	return value
}
