package main

import (
	"strings"
	"time"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

const (
	// 窗口和分栏的最小尺寸，保证窄窗口下内容仍然可读。
	minHistoryPaneWidth = 300
	minPreviewPaneWidth = 460
	splitDividerWidth   = 1
	// windowControlsInset 是隐藏标题栏窗口左上角窗口控制按钮（macOS 红绿灯）
	// 距窗口左边缘的水平间距，与 runtime 里的 TrafficLightPosition.X 一致。
	windowControlsInset = 18
	// windowControlsTopInset 是红绿灯距窗口顶边的垂直间距。macOS 用它和
	// 红绿灯直径（14）算出标题栏高度：14 + 2×9 = 32，与系统原生高度一致。
	windowControlsTopInset = 9

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

type historyKindFilter = history.KindFilter
type historyTagFilter = history.TagFilter
type historyPeriodFilter = history.PeriodFilter

const (
	kindFilterAny      = history.KindFilterAny
	kindFilterText     = history.KindFilterText
	kindFilterRichText = history.KindFilterRichText
	kindFilterImage    = history.KindFilterImage
	kindFilterFile     = history.KindFilterFile
)

const (
	tagFilterAny   = history.TagFilterAny
	tagFilterLink  = history.TagFilterLink
	tagFilterCode  = history.TagFilterCode
	tagFilterImage = history.TagFilterImage
)

const (
	periodFilterAny        = history.PeriodFilterAny
	periodFilterToday      = history.PeriodFilterToday
	periodFilterYesterday  = history.PeriodFilterYesterday
	periodFilterLast7Days  = history.PeriodFilterLast7Days
	periodFilterLast30Days = history.PeriodFilterLast30Days
)

type contentKind = history.Kind

const (
	kindText     = history.KindText
	kindLink     = history.KindLink
	kindCode     = history.KindCode
	kindImage    = history.KindImage
	kindFile     = history.KindFile
	kindRichText = history.KindRichText
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
	// onDevicesChanged 在已配对设备增删后触发，让托盘「设备同步」子菜单重建。
	onDevicesChanged func()
	// onZoomChanged 在界面缩放变化后让运行时调整窗口的最小尺寸与大小；
	// onMaxZoom 由运行时按窗口所在屏幕的可用区域回答当前能放到多大。
	onZoomChanged func(zoom float32)
	onMaxZoom     func() float32

	// zoom 是界面缩放比例，1 为 100%；platformFontSize/platformSpacing 记住
	// 平台主题的基准值（含系统文字大小设置），zoomedFontSize/zoomedSpacing 是
	// 上一次写回主题的值，用来分辨「读到的主题是自己写的」还是平台改了。
	zoom             float32
	platformFontSize float32
	platformSpacing  float32
	zoomedFontSize   float32
	zoomedSpacing    float32

	historyPaneWidth float32
	// filterRailOpen 记录筛选栏是否展开：标题栏的展开/收起按钮切换它。
	// 收起时筛选栏不占宽度，内容区占满窗口。
	filterRailOpen bool
	// The history filter rail keeps one field per facet. Their zero values
	// mean "no filter", so the whole history shows until the user picks one.
	filterKind     historyKindFilter
	filterTag      historyTagFilter
	filterFavorite bool
	filterSource   history.SourceFilter
	filterPeriod   historyPeriodFilter
	query          string
	history        *history.Store
	historyBitmaps map[string]*ui.Bitmap
	codeHighlights codeHighlightCache
	clipboard      history.ClipboardWriter
	historySearch  ui.Handle
	listState      ui.ListState

	onResendItem func(itemID, deviceID string)
	// syncPopoverItem 与 syncPopoverOpen 记录预览区同步状态浮层的展开状态：
	// 浮层由指针悬停状态标记展开，指针移开标记与浮层后收起。
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
	deviceRefreshSpinUntil   time.Time
	selectedDeviceID         string
	deviceInfoExpanded       bool
	addDeviceDialog          bool
	pairingPending           bool
	pairingError             string
	onStartPairing           func()
	launchAtLogin            bool
	language                 language
	appearance               int
	appearancePopoverOpen    bool // 侧栏底部外观气泡是否展开：按钮点击切换，点外部或 Esc 收起
	settingsArea             settingsArea
	confirmDangerousAction   bool
	pendingDangerousAction   dangerousAction
	pendingDangerousTargetID string
	// dangerDontAskAgain 是当前危险操作弹窗里“不再询问”的勾选状态；
	// confirmBeforeDelete 是删除单条记录前是否先弹二次确认，默认开启。勾选
	// “不再询问”会把它关掉，设置页的「删除前确认」开关可以再打开。
	dangerDontAskAgain  bool
	confirmBeforeDelete bool
	// pendingToast 暂存需要在下一帧展示的提示，用于撤销删除等回调场景。
	pendingToast string
	// pendingUndos 保存仍可撤销的破坏性操作，栈顶是最近一次；Cmd+Z 撤销栈顶。
	pendingUndos []pendingUndo
	// undoRequested 是 macOS 菜单排队的一次撤销请求，由下一帧消费；菜单回调
	// 不在构建帧里，拿不到 ui.Context，无法直接关掉提示。
	undoRequested bool
	// onUndoAvailabilityChanged 在撤销机会由无到有或相反时触发，让 macOS 菜单项
	// 同步启用状态。
	onUndoAvailabilityChanged func()

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
