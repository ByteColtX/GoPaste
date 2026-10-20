package main

import (
	"strings"
	"time"

	"gopaste/internal/filescache"
	"gopaste/internal/history"
	"gopaste/internal/lansync"

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
type syncDirection = lansync.Direction

const (
	syncDirectionTwoWay      = lansync.DirectionTwoWay
	syncDirectionSendOnly    = lansync.DirectionSendOnly
	syncDirectionReceiveOnly = lansync.DirectionReceiveOnly
	syncDirectionOff         = lansync.DirectionOff
)

// settingsArea 表示设置页左侧导航中的当前主题。
type settingsArea uint8

const (
	settingsGeneral settingsArea = iota
	settingsSync
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
	syncStateTooLarge = history.SyncStateTooLarge
)

type deliveryState = history.DeliveryState

const (
	deliveryPending  = history.DeliveryPending
	deliveryReceived = history.DeliveryReceived
	deliveryOffline  = history.DeliveryOffline
	deliveryTooLarge = history.DeliveryTooLarge
)

// pairedDevice 是已配对设备在界面上的投影。真正的数据在 lansync 的
// devices.json 里，这里每个字段都由 applySyncDevices 从那边刷新，界面上的改动
// 则经 syncService 写回去，界面因此不持有唯一真相。
type pairedDevice struct {
	id       string
	name     string
	peerID   string
	platform string
	version  string
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

// deviceDiscoveryState describes what the discovery side of the sync service is
// doing, so the device page can show a spinner or an error instead of an empty
// list.
type deviceDiscoveryState uint8

const (
	deviceDiscoveryReady deviceDiscoveryState = iota
	deviceDiscoveryLoading
	deviceDiscoveryError
)

// pairingState is where the pairing dialog is in the pairing flow.
type pairingState uint8

const (
	// pairingIdle lists the devices discovered on the network.
	pairingIdle pairingState = iota
	// pairingWaiting shows the code this device generated for the device it is
	// pairing with.
	pairingWaiting
	// pairingAnswering asks for the code shown by the device that started the
	// pairing.
	pairingAnswering
)

// clipboardPort 是根包对系统剪贴板的全部需求。历史模块只要写文本和写图片；写文件
// 列表只在接收同步文件时用到，所以它是根包自己的接口，不进 history 的缝。
type clipboardPort interface {
	history.ClipboardWriter
	WriteFiles(paths ...string) error
}

// application 保存窗口在多次重绘之间需要延续的状态。UI 元素本身不放在这里，
// 因为 MyGo 的元素只在当前构建帧中有效。
type application struct {
	page page

	onSyncStateChanged func()
	onLanguageChanged  func()
	// onDevicesChanged 在已配对设备增删后触发，让托盘「设备同步」子菜单重建。
	onDevicesChanged func()
	// onFilterRailToggled 在侧栏展开/收起后触发，让 macOS 视图菜单里的
	// 「收起/展开侧栏」项跟上文案。
	onFilterRailToggled func()
	// onPageChanged 在分区切换后触发，让 macOS 菜单里只在设备页有意义的项
	// （刷新设备列表）跟着启用或置灰。
	onPageChanged func()
	// onMeasureStorageUsage 由运行时安装：在界面线程之外统计一遍磁盘占用，结果经
	// Window.Update 回填（见 applyStorageUsage）。为 nil 时（测试、没有窗口）不测，
	// 用量区显示上一次的结果。参数非空时先等它关闭再统计。
	onMeasureStorageUsage func(after <-chan struct{})
	// onCleanFileCache 由运行时安装：在界面线程之外测量收到的文件、按规则挑出该删
	// 的记录，结果经 Window.Update 回填（见 applyFileCacheCleanup）。为 nil 时
	// （测试、没有窗口）不清理。
	onCleanFileCache func(entries []filescache.Entry, policy filescache.Policy)
	// onChooseDirectory 由运行时安装：在界面线程之外打开系统目录选择框，选完经
	// Window.Update 回到 applyDirectoryChoice。为 nil 时（测试、没有窗口）按钮什么
	// 都不做。
	onChooseDirectory func(defaultPath string)
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
	clipboard      clipboardPort
	historySearch  ui.Handle
	listState      ui.ListState

	// persistence 是历史记录的落盘队列；为 nil 时只保存在内存里。
	// clipboardEcho 记录应用自己刚写到剪贴板的读数 ID，见 recordClipboardWrite。
	persistence   *historyPersistence
	clipboardEcho map[string]struct{}
	// retention 是当前的自动清理上限，见 pruneHistory。默认值由
	// defaultRetention 给出，用户改过才落盘。
	retention history.RetentionPolicy
	// historyReady 表示持久化的历史已经装载进 history。在此之前收到的同步内容先
	// 排进 pendingClips：装载结果会整份替换内存里的历史，直接写进去的条目会连同
	// 剪贴板里已有的内容一起丢掉。
	historyReady bool
	pendingClips []history.Item

	// syncService 是局域网同步服务。为 nil 时（测试、或服务启动失败）界面只显示
	// 本地状态，配对与同步入口按不可用处理。
	syncService *lansync.Service
	// syncPopoverItem 与 syncPopoverOpen 记录预览区同步状态浮层的展开状态：
	// 浮层由指针悬停状态标记展开，指针移开标记与浮层后收起。
	syncPopoverItem   string
	syncPopoverOpen   bool
	deviceName        string
	deviceNameDefault string // 启动时算出的默认名（主机名），设备页据此判断是否仍是默认值
	devicePeerID      string
	devicePlatform    string
	// syncEnabled 是同步页的总开关，autoSendEnabled 决定新内容是否自动投递，
	// fileSyncEnabled 是文件类内容自己的开关。三个都跨启动保留，设备页的本地卡与
	// 同步页绑定同一份状态。
	syncEnabled     bool
	autoSendEnabled bool
	fileSyncEnabled bool
	// soundMode 是同步页的提示音档位，sound 是它的出口（nil 表示不响：测试默认
	// 如此，不会在 CI 上出声）。
	soundMode soundMode
	sound     soundPlayer
	// fileSyncDir 是用户选的文件保存位置，空表示用 fileSyncDirDefault。
	// receivedFilesDir 是当前真正落盘的那个根目录：它是两者之一，同步服务、用量
	// 统计与清理都看它。
	fileSyncDir        string
	fileSyncDirDefault string
	// fileQuotaMB 是配额输入框里的文本，fileQuotaBytes 是它解析后的有效值，两者
	// 的关系与 fileSyncMaxMB/fileSyncLimitBytes 相同。
	fileQuotaMB    string
	fileQuotaBytes int64
	// fileRetentionHours 是收到的文件保留多少小时，0 表示不限；fileAutoClean 决定
	// 到期是否自动删除。
	fileRetentionHours int
	fileAutoClean      bool
	// fileCachePending 表示已经请运行时去测一遍文件缓存、结果还没回来：它挡住重复
	// 请求，也让「立即清理」按钮在跑的时候置灰。fileCacheManual 记住这一趟是不是
	// 手动触发的，决定没有可清理的东西时要不要提示。
	fileCachePending       bool
	fileCacheManual        bool
	pairedDevices          []pairedDevice
	deviceDiscovery        deviceDiscoveryState
	deviceDiscoveryMessage string
	deviceRefreshSpinUntil time.Time
	selectedDeviceID       string
	deviceInfoExpanded     bool
	addDeviceDialog        bool

	// discoveredPeers 是发现到但还没配对的设备，配对对话框据此列出可配对的设备。
	discoveredPeers []lansync.Peer
	// pairing、pairingCode、pairingPeerName 描述配对对话框当前所处的阶段。
	pairing         pairingState
	pairingCode     string
	pairingPeerID   string
	pairingPeerName string
	pairingError    string
	// pairCodeInput 是「对方发起配对」时用户输入的配对码。
	pairCodeInput string

	// 首次启动向导。onboarding 是当前步骤，onboardingStepNone 表示不显示；
	// onboardingCompleted 是已经落盘的那个事实，跳过也算完成，因此向导不会
	// 自己再回来。
	onboarding          onboardingStep
	onboardingCompleted bool
	onboardingPath      onboardingPath
	// onboardingName 是向导里的设备名草稿，进入向导时用当前设备名预填。
	onboardingName string
	// confirmSkipOnboarding 是「跳过首次设置？」确认框的展开状态。跳过会让向导
	// 不再出现，所以先问一句。
	confirmSkipOnboarding bool
	// settingsPath 是本次运行读写的设置文件；为空时不落盘（例如拿不到数据目录）。
	settingsPath string
	// fileSyncMaxMB 是设置页里「同步文件大小上限」的输入文本，fileSyncLimitBytes
	// 是它解析后的有效值：输入不合法时文本留在原处，上限仍是上一个有效值。
	fileSyncMaxMB      string
	fileSyncLimitBytes int64
	// receivedFilesDir 是收到的同步文件落盘的根目录（每条记录一个子目录）；为空
	// 时收不了文件，删除记录时也没有可清理的东西。
	receivedFilesDir string
	// storageUsage 是最近一次测得的磁盘占用，见 storage_usage.go。测量在界面线程
	// 之外跑，渲染只读这里的结果，因此页面每帧都不碰磁盘。storageUsagePending 表示
	// 已经请运行时去测、结果还没回来：它挡住重复请求，也让刷新图标转圈。
	storageUsage        storageUsage
	storageUsagePending bool

	launchAtLogin            bool
	languagePreference       languagePreference // 设置页选的语言，language 是它解析后的结果
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
