package main

import (
	"fmt"
	"strings"
	"time"
)

// GoPaste ships with two UI languages. Every string the user reads comes from
// the catalogs below: a language selects a catalog, and the localizer turns a
// typed message key into text. Keys are typed and both catalogs live in one
// file so a missing or empty translation fails at startup instead of showing a
// blank label.

// language identifies one of the supported UI languages.
type language uint8

const (
	languageSimplifiedChinese language = iota
	languageEnglish
)

// languagePreference is the interface-language choice the settings page stores:
// follow the system, or pin one language. It stays separate from language so the
// stored choice never depends on what the system happens to be today: the
// preference is what the user picked, language is what is on screen.
type languagePreference uint8

const (
	// languagePreferenceSystem follows the system locale, and is the default.
	languagePreferenceSystem languagePreference = iota
	languagePreferenceSimplifiedChinese
	languagePreferenceEnglish
)

// language resolves the preference to the language to display. The locale is
// read only when following the system, so a pinned language never touches it —
// MyGo reads the locale on the main thread, which a test has none of.
func (p languagePreference) language(locale func() string) language {
	switch p {
	case languagePreferenceSimplifiedChinese:
		return languageSimplifiedChinese
	case languagePreferenceEnglish:
		return languageEnglish
	default:
		return languageFromLocale(locale())
	}
}

// languageChoices pairs each preference with its label for the current
// language. Following the system has no name of its own, so it is the one label
// that gets translated; the languages keep their own names, so they stay legible
// whatever the interface is showing.
func (l localizer) languageChoices() []option[languagePreference] {
	return []option[languagePreference]{
		{value: languagePreferenceSystem, label: l.text(msgSettingsLanguageSystem)},
		{value: languagePreferenceSimplifiedChinese, label: "简体中文"},
		{value: languagePreferenceEnglish, label: "English"},
	}
}

// supportedLanguages lists the languages GoPaste is written in, for
// mygo.App.SetLanguages. By MyGo's convention the first is the app's own
// language, and the one shown when none of the user's languages matches: it is
// English here, which is the same fallback languageFromLocale takes for
// anything that is not Chinese.
//
// Declaring these is what makes following the system work at all: MyGo picks
// the language to show by matching the user's preferred languages against this
// list, and without it the app is taken to be written in English alone, so
// Locale reports "en" on every machine.
func supportedLanguages() []string {
	return []string{"en", "zh-Hans"}
}

// languageFromLocale maps a system locale such as "en-US" or "zh-Hans-CN" to a
// supported language. Anything that is not Chinese falls back to English.
func languageFromLocale(locale string) language {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "zh") {
		return languageSimplifiedChinese
	}
	return languageEnglish
}

// messageKey names one user-facing string in a catalog.
type messageKey string

const (
	// Navigation. These also title the contextual sidebar for each section.
	msgNavHistory  messageKey = "nav.history"
	msgNavDevices  messageKey = "nav.devices"
	msgNavSettings messageKey = "nav.settings"

	// History filters.
	msgFilterAll       messageKey = "filter.all"
	msgFilterFavorites messageKey = "filter.favorites"

	// Filter rail sections and their facet choices.
	msgFilterSectionType   messageKey = "filter.section.type"
	msgFilterSectionTag    messageKey = "filter.section.tag"
	msgFilterSectionSource messageKey = "filter.section.source"
	msgFilterSectionTime   messageKey = "filter.section.time"
	msgFilterSourceAll     messageKey = "filter.source.all"
	msgFilterTimeAll       messageKey = "filter.time.all"
	msgFilterTimeToday     messageKey = "filter.time.today"
	msgFilterTimeYesterday messageKey = "filter.time.yesterday"
	msgFilterTimeLast7     messageKey = "filter.time.last7"
	msgFilterTimeLast30    messageKey = "filter.time.last30"
	msgFilterRailCollapse  messageKey = "filter.rail.collapse"
	msgFilterRailExpand    messageKey = "filter.rail.expand"
	msgAppMenuSettings     messageKey = "app.menu.settings"

	// History list, search, empty states, and preview.
	msgHistoryRecent             messageKey = "history.recent"
	msgHistoryCount              messageKey = "history.count"
	msgHistoryCountOne           messageKey = "history.countOne"
	msgHistoryCountFiltered      messageKey = "history.countFiltered"
	msgHistorySearchLabel        messageKey = "history.searchLabel"
	msgHistorySearchPlaceholder  messageKey = "history.searchPlaceholder"
	msgHistoryClearSearch        messageKey = "history.clearSearch"
	msgHistoryEmptyTitle         messageKey = "history.empty.title"
	msgHistoryEmptyMessage       messageKey = "history.empty.message"
	msgHistoryEmptyFirstTitle    messageKey = "history.emptyFirst.title"
	msgHistoryEmptyFirstMessage  messageKey = "history.emptyFirst.message"
	msgHistoryEmptySearchTitle   messageKey = "history.emptySearch.title"
	msgHistoryEmptySearchMessage messageKey = "history.emptySearch.message"
	msgHistoryEmptyFilterTitle   messageKey = "history.emptyFilter.title"
	msgHistoryEmptyFilterMessage messageKey = "history.emptyFilter.message"
	msgHistoryResetSearchAndFilt messageKey = "history.resetSearchAndFilter"
	msgHistoryResetSearch        messageKey = "history.resetSearch"
	msgHistoryShowAll            messageKey = "history.showAll"
	msgHistorySelectRecord       messageKey = "history.selectRecord"
	msgHistoryPreviewHint        messageKey = "history.previewHint"
	msgHistoryCharCount          messageKey = "history.charCount"
	msgHistoryCharCountOne       messageKey = "history.charCountOne"
	msgHistoryImageUnavailable   messageKey = "history.imageUnavailable"
	msgHistoryImageLabel         messageKey = "history.imageLabel"
	msgHistoryImageShort         messageKey = "history.imageShort"
	msgHistoryFilePreviewGone    messageKey = "history.filePreviewUnavailable"
	msgHistoryFileMoreFormat     messageKey = "history.fileMoreFormat"
	msgHistoryFileMoreFormatOne  messageKey = "history.fileMoreFormatOne"
	msgHistoryCopy               messageKey = "history.copy"
	msgHistoryCopyFailed         messageKey = "history.copyFailed"
	msgHistoryDelete             messageKey = "history.delete"

	// Content kind names.
	msgKindText     messageKey = "kind.text"
	msgKindLink     messageKey = "kind.link"
	msgKindCode     messageKey = "kind.code"
	msgKindImage    messageKey = "kind.image"
	msgKindFile     messageKey = "kind.file"
	msgKindRichText messageKey = "kind.richText"

	// A file item's format is named by filetype.Format.Label and the file's own
	// extension, not by a catalog entry: a format tag reads the same in every UI
	// language, so it is never translated.

	// Favorites.
	msgFavorite      messageKey = "favorite.add"
	msgFavoriteAdded messageKey = "favorite.added"

	// Toasts.
	msgToastCopied                     messageKey = "toast.copied"
	msgToastFavoriteAdded              messageKey = "toast.favoriteAdded"
	msgToastFavoriteRemoved            messageKey = "toast.favoriteRemoved"
	msgToastHistoryCleared             messageKey = "toast.historyCleared"
	msgToastDeleted                    messageKey = "toast.deleted"
	msgToastUndo                       messageKey = "toast.undo"
	msgToastRestored                   messageKey = "toast.restored"
	msgToastUnpaired                   messageKey = "toast.unpaired"
	msgToastLaunchAtLoginUnsupported   messageKey = "toast.launchAtLoginUnsupported"
	msgToastLaunchAtLoginOn            messageKey = "toast.launchAtLoginOn"
	msgToastLaunchAtLoginOff           messageKey = "toast.launchAtLoginOff"
	msgToastDeviceDiscoveryUnavailable messageKey = "toast.deviceDiscoveryUnavailable"

	// Devices.
	msgDevicesPaired           messageKey = "devices.paired"
	msgDevicesRefresh          messageKey = "devices.refresh"
	msgDevicesOnlineCount      messageKey = "devices.onlineCount"
	msgDevicesEmptyTitle       messageKey = "devices.empty.title"
	msgDevicesEmptyMessage     messageKey = "devices.empty.message"
	msgDevicesLoadingTitle     messageKey = "devices.loading.title"
	msgDevicesLoadingMessage   messageKey = "devices.loading.message"
	msgDevicesErrorTitle       messageKey = "devices.error.title"
	msgDevicesErrorMessage     messageKey = "devices.error.message"
	msgDevicesRetry            messageKey = "devices.retry"
	msgDevicesAdd              messageKey = "devices.add"
	msgDevicesOfflineTitle     messageKey = "devices.offline.title"
	msgDevicesOfflineMessage   messageKey = "devices.offline.message"
	msgDevicesLastSeen         messageKey = "devices.lastSeen"
	msgDevicesBadgePaired      messageKey = "devices.badge.paired"
	msgDevicesBadgeLocal       messageKey = "devices.badge.local"
	msgDevicesUnpair           messageKey = "devices.unpair"
	msgDevicesDirect           messageKey = "devices.direct"
	msgDevicesOnline           messageKey = "devices.online"
	msgDevicesOffline          messageKey = "devices.offline"
	msgDevicesSyncOn           messageKey = "devices.syncOn"
	msgDevicesSyncOff          messageKey = "devices.syncOff"
	msgDevicesEnableSync       messageKey = "devices.enableSync"
	msgDevicesEnableSyncDesc   messageKey = "devices.enableSyncDesc"
	msgDevicesFileSync         messageKey = "devices.fileSync"
	msgDevicesFileSyncDesc     messageKey = "devices.fileSyncDesc"
	msgDevicesFileSyncDisabled messageKey = "devices.fileSyncDisabled"
	msgDevicesSyncWith         messageKey = "devices.syncWith"
	msgDevicesSyncWithDesc     messageKey = "devices.syncWithDesc"
	msgDevicesSyncContent      messageKey = "devices.syncContent"
	msgDevicesSyncContentDesc  messageKey = "devices.syncContentDesc"
	msgDevicesAllTwoWay        messageKey = "devices.allTwoWay"
	msgDevicesCustomSync       messageKey = "devices.customSync"
	msgDevicesInfoTitle        messageKey = "devices.info.title"
	msgDevicesConnectionTitle  messageKey = "devices.info.connection"
	msgDevicesInfoPlatform     messageKey = "devices.info.platform"
	msgDevicesInfoPeerID       messageKey = "devices.info.peerID"
	msgDevicesInfoConnection   messageKey = "devices.info.connectionMethod"
	msgDevicesInfoAppVersion   messageKey = "devices.info.appVersion"
	msgDevicesInfoAddress      messageKey = "devices.info.address"
	msgDevicesInfoLocalValue   messageKey = "devices.info.localValue"
	msgDevicesInfoDirectValue  messageKey = "devices.info.directValue"
	msgDevicesExpand           messageKey = "devices.expand"
	msgDevicesCollapse         messageKey = "devices.collapse"
	msgDevicesPairDirectOnly   messageKey = "devices.pair.directOnly"
	msgDevicesPairDescription  messageKey = "devices.pair.description"
	msgDevicesPairWaiting      messageKey = "devices.pair.waiting"
	msgDevicesPairStart        messageKey = "devices.pair.start"
	msgDevicesPairWaitingSvc   messageKey = "devices.pair.waitingService"
	msgDevicesPairClose        messageKey = "devices.pair.close"

	// Sync direction.
	msgSyncBoth           messageKey = "sync.both"
	msgSyncSend           messageKey = "sync.send"
	msgSyncReceive        messageKey = "sync.receive"
	msgSyncOff            messageKey = "sync.off"
	msgSyncDirectionLabel messageKey = "sync.directionLabel"
	msgSyncDescSyncOff    messageKey = "sync.desc.syncOff"
	msgSyncDescOffline    messageKey = "sync.desc.offline"
	msgSyncDescNormal     messageKey = "sync.desc.normal"
	msgSyncThisDevice     messageKey = "sync.thisDevice"

	// Sync status.
	msgSyncStatusTitle         messageKey = "sync.status.title"
	msgSyncStateSynced         messageKey = "sync.status.synced"
	msgSyncStatePartial        messageKey = "sync.status.partial"
	msgSyncStateOffline        messageKey = "sync.status.offline"
	msgSyncStateReplaced       messageKey = "sync.status.replaced"
	msgSyncStateNone           messageKey = "sync.status.none"
	msgSyncDeliveryReceived    messageKey = "sync.delivery.received"
	msgSyncDeliveryOffline     messageKey = "sync.delivery.offline"
	msgSyncDeliveryPending     messageKey = "sync.delivery.pending"
	msgSyncDeliveryNotSent     messageKey = "sync.delivery.notSent"
	msgSyncResendAll           messageKey = "sync.resend.all"
	msgSyncResendAllAccess     messageKey = "sync.resend.allAccess"
	msgSyncResend              messageKey = "sync.resend.one"
	msgSyncResendAccess        messageKey = "sync.resend.oneAccess"
	msgSyncNoDevices           messageKey = "sync.noDevices"
	msgSyncReplacedNote        messageKey = "sync.replacedNote"
	msgSyncServiceUnavailable  messageKey = "sync.serviceUnavailable"
	msgHistorySourceThisDevice messageKey = "history.source.thisDevice"
	msgHistorySourceLanDevice  messageKey = "history.source.lanDevice"
	msgHistorySourceLanFormat  messageKey = "history.source.lanFormat"

	// Settings.
	msgSettingsGeneral               messageKey = "settings.general"
	msgSettingsGeneralDesc           messageKey = "settings.generalDesc"
	msgSettingsBasics                messageKey = "settings.basics"
	msgSettingsDeviceName            messageKey = "settings.deviceName"
	msgSettingsDeviceNameDesc        messageKey = "settings.deviceNameDesc"
	msgSettingsDeviceNamePlaceholder messageKey = "settings.deviceNamePlaceholder"
	msgSettingsLaunchAtLogin         messageKey = "settings.launchAtLogin"
	msgSettingsLaunchAtLoginDesc     messageKey = "settings.launchAtLoginDesc"
	msgSettingsLanguage              messageKey = "settings.language"
	msgSettingsLanguageSystem        messageKey = "settings.language.system"
	msgSettingsLanguageDesc          messageKey = "settings.languageDesc"
	msgSettingsConfirmDelete         messageKey = "settings.confirmDelete"
	msgSettingsConfirmDeleteDesc     messageKey = "settings.confirmDeleteDesc"
	msgSettingsAppearance            messageKey = "settings.appearance"
	msgSettingsAppearanceDesc        messageKey = "settings.appearanceDesc"
	msgSettingsVisualStyle           messageKey = "settings.visualStyle"
	msgSettingsMode                  messageKey = "settings.mode"
	msgSettingsModeDesc              messageKey = "settings.modeDesc"
	msgSettingsThemeSystem           messageKey = "settings.theme.system"
	msgSettingsThemeLight            messageKey = "settings.theme.light"
	msgSettingsThemeDark             messageKey = "settings.theme.dark"
	msgSettingsCurrentAppearance     messageKey = "settings.currentAppearance"
	msgSettingsThemeLightName        messageKey = "settings.theme.lightName"
	msgSettingsThemeDarkName         messageKey = "settings.theme.darkName"
	msgSettingsThemeSystemName       messageKey = "settings.theme.systemName"
	msgSettingsInterface             messageKey = "settings.interface"
	msgSettingsZoom                  messageKey = "settings.zoom"
	msgSettingsZoomDesc              messageKey = "settings.zoomDesc"
	msgSettingsZoomOut               messageKey = "settings.zoomOut"
	msgSettingsZoomIn                messageKey = "settings.zoomIn"
	msgSettingsZoomReset             messageKey = "settings.zoomReset"
	msgMenuView                      messageKey = "menu.view"
	msgMenuUndo                      messageKey = "menu.undo"
	msgMenuZoomIn                    messageKey = "menu.zoomIn"
	msgMenuZoomOut                   messageKey = "menu.zoomOut"
	msgMenuZoomReset                 messageKey = "menu.zoomReset"
	msgToastZoom                     messageKey = "toast.zoom"
	msgSettingsStorage               messageKey = "settings.storage"
	msgSettingsStorageDesc           messageKey = "settings.storageDesc"
	msgSettingsData                  messageKey = "settings.data"
	msgSettingsHistoryRow            messageKey = "settings.historyRow"
	msgSettingsHistoryRowDesc        messageKey = "settings.historyRowDesc"
	msgSettingsNoRecords             messageKey = "settings.noRecords"
	msgSettingsRecordCount           messageKey = "settings.recordCount"
	msgSettingsRecordCountOne        messageKey = "settings.recordCountOne"
	msgSettingsImageCount            messageKey = "settings.imageCount"
	msgSettingsImageCountOne         messageKey = "settings.imageCountOne"
	msgSettingsClearHistoryRow       messageKey = "settings.clearHistoryRow"
	msgSettingsClearHistoryRowDesc   messageKey = "settings.clearHistoryRowDesc"
	msgSettingsClearHistory          messageKey = "settings.clearHistory"
	msgSettingsNothingToClear        messageKey = "settings.nothingToClear"
	msgSettingsNoHistoryNote         messageKey = "settings.noHistoryNote"
	msgSettingsAbout                 messageKey = "settings.about"
	msgSettingsAboutDesc             messageKey = "settings.aboutDesc"
	msgSettingsAppInfo               messageKey = "settings.appInfo"
	msgSettingsVersion               messageKey = "settings.version"
	msgSettingsVersionDesc           messageKey = "settings.versionDesc"
	msgSettingsSyncScope             messageKey = "settings.syncScope"
	msgSettingsSyncScopeDesc         messageKey = "settings.syncScopeDesc"
	msgSettingsPrivacy               messageKey = "settings.privacy"
	msgSettingsPrivacyDesc           messageKey = "settings.privacyDesc"
	msgSettingsStoredLocally         messageKey = "settings.storedLocally"

	// Destructive-action dialogs.
	msgDangerClearTitle          messageKey = "danger.clear.title"
	msgDangerClearMessage        messageKey = "danger.clear.message"
	msgDangerClearConfirm        messageKey = "danger.clear.confirm"
	msgDangerDeleteTitle         messageKey = "danger.delete.title"
	msgDangerDeleteMessage       messageKey = "danger.delete.message"
	msgDangerDeleteMessageFormat messageKey = "danger.delete.messageFormat"
	msgDangerDeleteConfirm       messageKey = "danger.delete.confirm"
	msgDangerDontAskAgain        messageKey = "danger.dontAskAgain"
	msgDangerUnpairTitle         messageKey = "danger.unpair.title"
	msgDangerUnpairMessage       messageKey = "danger.unpair.message"
	msgDangerUnpairConfirm       messageKey = "danger.unpair.confirm"
	msgDangerCancel              messageKey = "danger.cancel"

	// Tray menu.
	msgTrayTurnSyncOff messageKey = "tray.turnSyncOff"
	msgTrayTurnSyncOn  messageKey = "tray.turnSyncOn"
	msgTrayDeviceSync  messageKey = "tray.deviceSync"
	msgTrayNoDevices   messageKey = "tray.noDevices"
	msgTrayOpen        messageKey = "tray.open"
	msgTraySettings    messageKey = "tray.settings"
	msgTrayRelaunch    messageKey = "tray.relaunch"
	msgTrayQuit        messageKey = "tray.quit"

	// Timestamps.
	msgDateToday     messageKey = "date.today"
	msgDateYesterday messageKey = "date.yesterday"
	msgDateThisYear  messageKey = "date.thisYear"
	msgDateOtherYear messageKey = "date.otherYear"
	msgTimeLayout    messageKey = "date.timeLayout"
)

// catalog holds one language's messages.
type catalog map[messageKey]string

func (c catalog) text(key messageKey) string { return c[key] }

var simplifiedChineseCatalog = catalog{
	msgNavHistory:  "历史记录",
	msgNavDevices:  "设备",
	msgNavSettings: "设置",

	msgFilterAll:       "所有",
	msgFilterFavorites: "收藏",

	msgFilterSectionType:   "类型",
	msgFilterSectionTag:    "标签",
	msgFilterSectionSource: "来源",
	msgFilterSectionTime:   "时间",
	msgFilterSourceAll:     "所有来源",
	msgFilterTimeAll:       "所有时间",
	msgFilterTimeToday:     "今天",
	msgFilterTimeYesterday: "昨天",
	msgFilterTimeLast7:     "近 7 天",
	msgFilterTimeLast30:    "近 30 天",
	msgFilterRailCollapse:  "隐藏边栏",
	msgFilterRailExpand:    "显示边栏",
	msgAppMenuSettings:     "设置…",

	msgHistoryRecent:             "历史记录",
	msgHistoryCount:              "%d 条记录",
	msgHistoryCountOne:           "%d 条记录",
	msgHistoryCountFiltered:      "%d / %d 条记录",
	msgHistorySearchLabel:        "搜索历史记录",
	msgHistorySearchPlaceholder:  "搜索",
	msgHistoryClearSearch:        "清除搜索",
	msgHistoryEmptyTitle:         "没有匹配的记录",
	msgHistoryEmptyMessage:       "试试其他关键词或筛选条件。",
	msgHistoryEmptyFirstTitle:    "还没有任何记录",
	msgHistoryEmptyFirstMessage:  "拷贝文本或图像后，记录会显示在这里。记录仅储存在此设备。",
	msgHistoryEmptySearchTitle:   "未找到结果",
	msgHistoryEmptySearchMessage: "没有包含“%s”的记录。请清除搜索或尝试其他关键词。",
	msgHistoryEmptyFilterTitle:   "没有符合筛选条件的记录",
	msgHistoryEmptyFilterMessage: "没有符合“%s”的记录。",
	msgHistoryResetSearchAndFilt: "清除搜索和筛选",
	msgHistoryResetSearch:        "清除搜索",
	msgHistoryShowAll:            "显示全部记录",
	msgHistorySelectRecord:       "选择一条记录",
	msgHistoryPreviewHint:        "预览会显示在这里",
	msgHistoryCharCount:          "%d 个字符",
	msgHistoryCharCountOne:       "%d 个字符",
	msgHistoryImageUnavailable:   "无法预览图像",
	msgHistoryImageLabel:         "图像（%d×%d）",
	msgHistoryImageShort:         "图像",
	msgHistoryFilePreviewGone:    "无法预览此文件",
	msgHistoryFileMoreFormat:     "另有 %d 个文件",
	msgHistoryFileMoreFormatOne:  "另有 %d 个文件",
	msgHistoryCopy:               "拷贝",
	msgHistoryCopyFailed:         "拷贝失败",
	msgHistoryDelete:             "删除",

	msgKindText:     "文本",
	msgKindLink:     "链接",
	msgKindCode:     "代码",
	msgKindImage:    "图像",
	msgKindFile:     "文件",
	msgKindRichText: "富文本",

	msgFavorite:      "收藏",
	msgFavoriteAdded: "已收藏",

	msgToastCopied:                     "已拷贝到剪贴板",
	msgToastFavoriteAdded:              "已添加到收藏",
	msgToastFavoriteRemoved:            "已从收藏中移除",
	msgToastHistoryCleared:             "历史记录已清除",
	msgToastDeleted:                    "已删除",
	msgToastUndo:                       "撤销",
	msgToastRestored:                   "已恢复",
	msgToastUnpaired:                   "已取消配对",
	msgToastLaunchAtLoginUnsupported:   "此系统不支持登录时打开",
	msgToastLaunchAtLoginOn:            "GoPaste 将在登录时打开",
	msgToastLaunchAtLoginOff:           "GoPaste 将不再在登录时打开",
	msgToastDeviceDiscoveryUnavailable: "设备发现功能尚未开放",

	msgDevicesPaired:           "已配对的设备",
	msgDevicesRefresh:          "刷新设备列表",
	msgDevicesOnlineCount:      "%d 台在线",
	msgDevicesEmptyTitle:       "暂无已配对的设备",
	msgDevicesEmptyMessage:     "完成直连配对后，设备会显示在这里。\n剪贴板内容仅在已配对的设备之间同步。",
	msgDevicesLoadingTitle:     "正在加载设备",
	msgDevicesLoadingMessage:   "正在检查已配对的设备。",
	msgDevicesErrorTitle:       "无法加载设备",
	msgDevicesErrorMessage:     "暂时无法读取已配对的设备。",
	msgDevicesRetry:            "重试",
	msgDevicesAdd:              "添加设备",
	msgDevicesOfflineTitle:     "设备已离线",
	msgDevicesOfflineMessage:   "此设备当前离线，重新连接后会自动恢复同步。",
	msgDevicesLastSeen:         "上次在线：%s",
	msgDevicesBadgePaired:      "已配对",
	msgDevicesBadgeLocal:       "此设备",
	msgDevicesUnpair:           "取消配对",
	msgDevicesDirect:           "直连",
	msgDevicesOnline:           "在线",
	msgDevicesOffline:          "离线",
	msgDevicesSyncOn:           "同步已打开",
	msgDevicesSyncOff:          "同步已关闭",
	msgDevicesEnableSync:       "启用同步",
	msgDevicesEnableSyncDesc:   "允许已配对的设备之间同步内容",
	msgDevicesFileSync:         "文件同步",
	msgDevicesFileSyncDesc:     "允许在设备之间传输文件",
	msgDevicesFileSyncDisabled: "%s。请先打开设备同步。",
	msgDevicesSyncWith:         "与 %s 同步",
	msgDevicesSyncWithDesc:     "在任一设备上拷贝的内容都会自动同步。",
	msgDevicesSyncContent:      "同步内容",
	msgDevicesSyncContentDesc:  "分别设置每种内容的同步方向。",
	msgDevicesAllTwoWay:        "全部双向同步",
	msgDevicesCustomSync:       "自定",
	msgDevicesInfoTitle:        "设备信息",
	msgDevicesConnectionTitle:  "连接信息",
	msgDevicesInfoPlatform:     "平台",
	msgDevicesInfoPeerID:       "设备 ID",
	msgDevicesInfoConnection:   "连接方式",
	msgDevicesInfoAppVersion:   "应用版本",
	msgDevicesInfoAddress:      "地址",
	msgDevicesInfoLocalValue:   "本机",
	msgDevicesInfoDirectValue:  "直连",
	msgDevicesExpand:           "展开%s",
	msgDevicesCollapse:         "收起%s",
	msgDevicesPairDirectOnly:   "仅支持直连",
	msgDevicesPairDescription:  "设备配对功能尚未开放。开放后，你可以在这里发现并配对设备。GoPaste 不会使用云端、中继或其他网络回退连接。",
	msgDevicesPairWaiting:      "正在等待配对…",
	msgDevicesPairStart:        "开始配对",
	msgDevicesPairWaitingSvc:   "正在等待配对服务…",
	msgDevicesPairClose:        "关闭",

	msgSyncBoth:           "双向同步",
	msgSyncSend:           "仅发往 %s",
	msgSyncReceive:        "仅从 %s 接收",
	msgSyncOff:            "不同步",
	msgSyncDirectionLabel: "%s的同步方向",
	msgSyncDescSyncOff:    "设备同步已关闭。打开后可设置%s的同步方向。",
	msgSyncDescOffline:    "设备已离线。重新建立直连后可设置%s的同步方向。",
	msgSyncDescNormal:     "仅影响此设备上的%s内容。",
	msgSyncThisDevice:     "该设备",

	msgSyncStatusTitle:         "同步状态",
	msgSyncStateSynced:         "已同步",
	msgSyncStatePartial:        "部分同步",
	msgSyncStateOffline:        "对方设备离线",
	msgSyncStateReplaced:       "已被替代",
	msgSyncStateNone:           "未同步",
	msgSyncDeliveryReceived:    "已接收",
	msgSyncDeliveryOffline:     "对方设备离线",
	msgSyncDeliveryPending:     "待发送",
	msgSyncDeliveryNotSent:     "未送达",
	msgSyncResendAll:           "全部重发",
	msgSyncResendAllAccess:     "重发到全部设备",
	msgSyncResend:              "重发",
	msgSyncResendAccess:        "重发到 %s",
	msgSyncNoDevices:           "暂无已配对的设备",
	msgSyncReplacedNote:        "从未同步到任何设备，已被更新的记录替代",
	msgSyncServiceUnavailable:  "同步功能尚未开放",
	msgHistorySourceThisDevice: "此设备",
	msgHistorySourceLanDevice:  "局域网设备",
	msgHistorySourceLanFormat:  "来自 %s",

	msgSettingsGeneral:               "通用",
	msgSettingsGeneralDesc:           "设备名称和 GoPaste 的基本偏好设置。",
	msgSettingsBasics:                "基本设置",
	msgSettingsDeviceName:            "设备名称",
	msgSettingsDeviceNameDesc:        "用于在设备列表中识别此设备",
	msgSettingsDeviceNamePlaceholder: "输入设备名称",
	msgSettingsLaunchAtLogin:         "登录时打开",
	msgSettingsLaunchAtLoginDesc:     "登录时自动打开 GoPaste",
	msgSettingsLanguage:              "语言",
	msgSettingsLanguageSystem:        "跟随系统",
	msgSettingsLanguageDesc:          "选择界面语言",
	msgSettingsConfirmDelete:         "删除前确认",
	msgSettingsConfirmDeleteDesc:     "删除单条记录前先询问。关闭后删除将立即执行，清除历史记录和取消配对仍会先确认。",
	msgSettingsAppearance:            "外观",
	msgSettingsAppearanceDesc:        "选择 GoPaste 的颜色主题。",
	msgSettingsVisualStyle:           "视觉风格",
	msgSettingsMode:                  "模式",
	msgSettingsModeDesc:              "自动跟随系统，或始终使用浅色或深色",
	msgSettingsThemeSystem:           "自动",
	msgSettingsThemeLight:            "浅色",
	msgSettingsThemeDark:             "深色",
	msgSettingsCurrentAppearance:     "当前使用：%s",
	msgSettingsThemeLightName:        "浅色主题",
	msgSettingsThemeDarkName:         "深色主题",
	msgSettingsThemeSystemName:       "自动",
	msgSettingsInterface:             "界面",
	msgSettingsZoom:                  "界面缩放",
	msgSettingsZoomDesc:              "放大或缩小整个界面。快捷键：⌘=、⌘- 和 ⌘0",
	msgSettingsZoomOut:               "缩小",
	msgSettingsZoomIn:                "放大",
	msgSettingsZoomReset:             "实际大小",
	msgMenuView:                      "视图",
	msgMenuUndo:                      "撤销",
	msgMenuZoomIn:                    "放大",
	msgMenuZoomOut:                   "缩小",
	msgMenuZoomReset:                 "实际大小",
	msgToastZoom:                     "界面缩放：%s",
	msgSettingsStorage:               "储存空间",
	msgSettingsStorageDesc:           "查看并清除储存在此设备上的历史记录。",
	msgSettingsData:                  "数据",
	msgSettingsHistoryRow:            "历史记录",
	msgSettingsHistoryRowDesc:        "此设备上保存的剪贴板记录",
	msgSettingsNoRecords:             "暂无记录",
	msgSettingsRecordCount:           "%d 条",
	msgSettingsRecordCountOne:        "%d 条",
	msgSettingsImageCount:            "%d 张图像",
	msgSettingsImageCountOne:         "%d 张图像",
	msgSettingsClearHistoryRow:       "清除历史记录",
	msgSettingsClearHistoryRowDesc:   "删除此设备上保存的所有记录",
	msgSettingsClearHistory:          "清除历史记录",
	msgSettingsNothingToClear:        "没有可清除的记录",
	msgSettingsNoHistoryNote:         "此设备上暂无历史记录。拷贝内容后，记录会自动出现在“历史记录”页面。",
	msgSettingsAbout:                 "关于",
	msgSettingsAboutDesc:             "剪贴板历史记录与局域网同步工具。",
	msgSettingsAppInfo:               "应用信息",
	msgSettingsVersion:               "版本",
	msgSettingsVersionDesc:           "当前应用版本",
	msgSettingsSyncScope:             "同步范围",
	msgSettingsSyncScopeDesc:         "仅在已配对的设备之间建立直连",
	msgSettingsPrivacy:               "隐私",
	msgSettingsPrivacyDesc:           "剪贴板内容仅储存在此设备，不会写入日志或发送给第三方服务",
	msgSettingsStoredLocally:         "储存在此设备",

	msgDangerClearTitle:          "清除历史记录？",
	msgDangerClearMessage:        "此设备上保存的全部剪贴板历史记录都将被删除。此操作无法撤销。",
	msgDangerClearConfirm:        "清除",
	msgDangerDeleteTitle:         "删除这条记录？",
	msgDangerDeleteMessage:       "将从此设备的历史记录中删除这条记录。此操作无法撤销。",
	msgDangerDeleteMessageFormat: "将从此设备的历史记录中删除“%s”。此操作无法撤销。",
	msgDangerDeleteConfirm:       "删除",
	msgDangerDontAskAgain:        "不再询问",
	msgDangerUnpairTitle:         "取消配对设备？",
	msgDangerUnpairMessage:       "取消配对“%s”后，该设备将不再与此设备同步。",
	msgDangerUnpairConfirm:       "取消配对",
	msgDangerCancel:              "取消",

	msgTrayTurnSyncOff: "关闭同步",
	msgTrayTurnSyncOn:  "打开同步",
	msgTrayDeviceSync:  "设备同步",
	msgTrayNoDevices:   "暂无已配对的设备",
	msgTrayOpen:        "打开",
	msgTraySettings:    "设置",
	msgTrayRelaunch:    "重新启动",
	msgTrayQuit:        "退出",

	msgDateToday:     "今天 %s",
	msgDateYesterday: "昨天 %s",
	msgDateThisYear:  "1月2日 15:04",
	msgDateOtherYear: "2006年1月2日 15:04",
	msgTimeLayout:    "15:04",
}

var englishCatalog = catalog{
	msgNavHistory:  "History",
	msgNavDevices:  "Devices",
	msgNavSettings: "Settings",

	msgFilterAll:       "All",
	msgFilterFavorites: "Favorites",

	msgFilterSectionType:   "Type",
	msgFilterSectionTag:    "Tag",
	msgFilterSectionSource: "Source",
	msgFilterSectionTime:   "Time",
	msgFilterSourceAll:     "All sources",
	msgFilterTimeAll:       "All time",
	msgFilterTimeToday:     "Today",
	msgFilterTimeYesterday: "Yesterday",
	msgFilterTimeLast7:     "Last 7 days",
	msgFilterTimeLast30:    "Last 30 days",
	msgFilterRailCollapse:  "Hide sidebar",
	msgFilterRailExpand:    "Show sidebar",
	msgAppMenuSettings:     "Settings…",

	msgHistoryRecent:             "History",
	msgHistoryCount:              "%d records",
	msgHistoryCountOne:           "%d record",
	msgHistoryCountFiltered:      "%d / %d records",
	msgHistorySearchLabel:        "Search history",
	msgHistorySearchPlaceholder:  "Search",
	msgHistoryClearSearch:        "Clear search",
	msgHistoryEmptyTitle:         "No matching records",
	msgHistoryEmptyMessage:       "Try another keyword or filter.",
	msgHistoryEmptyFirstTitle:    "No records yet",
	msgHistoryEmptyFirstMessage:  "Text and images you copy show up here. Records stay on this device.",
	msgHistoryEmptySearchTitle:   "No results",
	msgHistoryEmptySearchMessage: "No records contain “%s”. Clear the search or try another keyword.",
	msgHistoryEmptyFilterTitle:   "No records match this filter",
	msgHistoryEmptyFilterMessage: "No records match “%s”.",
	msgHistoryResetSearchAndFilt: "Clear search and filters",
	msgHistoryResetSearch:        "Clear search",
	msgHistoryShowAll:            "Show all records",
	msgHistorySelectRecord:       "Select a record",
	msgHistoryPreviewHint:        "The preview appears here",
	msgHistoryCharCount:          "%d characters",
	msgHistoryCharCountOne:       "%d character",
	msgHistoryImageUnavailable:   "Can't preview this image",
	msgHistoryImageLabel:         "Image (%d×%d)",
	msgHistoryImageShort:         "Image",
	msgHistoryFilePreviewGone:    "Can't preview this file",
	msgHistoryFileMoreFormat:     "%d more files",
	msgHistoryFileMoreFormatOne:  "%d more file",
	msgHistoryCopy:               "Copy",
	msgHistoryCopyFailed:         "Couldn't copy",
	msgHistoryDelete:             "Delete",

	msgKindText:     "Text",
	msgKindLink:     "Link",
	msgKindCode:     "Code",
	msgKindImage:    "Image",
	msgKindFile:     "File",
	msgKindRichText: "Rich text",

	msgFavorite:      "Favorite",
	msgFavoriteAdded: "Favorited",

	msgToastCopied:                     "Copied to clipboard",
	msgToastFavoriteAdded:              "Added to favorites",
	msgToastFavoriteRemoved:            "Removed from favorites",
	msgToastHistoryCleared:             "History cleared",
	msgToastDeleted:                    "Deleted",
	msgToastUndo:                       "Undo",
	msgToastRestored:                   "Restored",
	msgToastUnpaired:                   "Unpaired",
	msgToastLaunchAtLoginUnsupported:   "Open at Login isn't available on this system",
	msgToastLaunchAtLoginOn:            "GoPaste will open at login",
	msgToastLaunchAtLoginOff:           "GoPaste will no longer open at login",
	msgToastDeviceDiscoveryUnavailable: "Device discovery isn't available yet",

	msgDevicesPaired:           "Paired devices",
	msgDevicesRefresh:          "Refresh device list",
	msgDevicesOnlineCount:      "%d online",
	msgDevicesEmptyTitle:       "No paired devices",
	msgDevicesEmptyMessage:     "Devices show up here after you pair them over a direct connection.\nClipboard content syncs only between paired devices.",
	msgDevicesLoadingTitle:     "Loading devices",
	msgDevicesLoadingMessage:   "Checking for paired devices.",
	msgDevicesErrorTitle:       "Couldn't load devices",
	msgDevicesErrorMessage:     "Paired devices are temporarily unavailable.",
	msgDevicesRetry:            "Retry",
	msgDevicesAdd:              "Add device",
	msgDevicesOfflineTitle:     "Device offline",
	msgDevicesOfflineMessage:   "This device is offline. Sync resumes when it reconnects.",
	msgDevicesLastSeen:         "Last online: %s",
	msgDevicesBadgePaired:      "Paired",
	msgDevicesBadgeLocal:       "This device",
	msgDevicesUnpair:           "Unpair",
	msgDevicesDirect:           "Direct",
	msgDevicesOnline:           "Online",
	msgDevicesOffline:          "Offline",
	msgDevicesSyncOn:           "Sync is on",
	msgDevicesSyncOff:          "Sync is off",
	msgDevicesEnableSync:       "Enable sync",
	msgDevicesEnableSyncDesc:   "Let paired devices sync content",
	msgDevicesFileSync:         "File sync",
	msgDevicesFileSyncDesc:     "Transfer files between paired devices",
	msgDevicesFileSyncDisabled: "%s. Turn on device sync first.",
	msgDevicesSyncWith:         "Sync with %s",
	msgDevicesSyncWithDesc:     "Content copied on either device syncs automatically.",
	msgDevicesSyncContent:      "Content to sync",
	msgDevicesSyncContentDesc:  "Set a sync direction for each content type.",
	msgDevicesAllTwoWay:        "All two-way",
	msgDevicesCustomSync:       "Custom",
	msgDevicesInfoTitle:        "Device info",
	msgDevicesConnectionTitle:  "Connection info",
	msgDevicesInfoPlatform:     "Platform",
	msgDevicesInfoPeerID:       "Device ID",
	msgDevicesInfoConnection:   "Connection type",
	msgDevicesInfoAppVersion:   "App version",
	msgDevicesInfoAddress:      "Address",
	msgDevicesInfoLocalValue:   "Local",
	msgDevicesInfoDirectValue:  "Direct",
	msgDevicesExpand:           "Show %s",
	msgDevicesCollapse:         "Hide %s",
	msgDevicesPairDirectOnly:   "Direct connections only",
	msgDevicesPairDescription:  "Pairing isn't available yet. Once it is, you can find and pair devices here. GoPaste never uses the cloud, relays, or other network fallbacks.",
	msgDevicesPairWaiting:      "Waiting for pairing…",
	msgDevicesPairStart:        "Start pairing",
	msgDevicesPairWaitingSvc:   "Waiting for pairing service…",
	msgDevicesPairClose:        "Close",

	msgSyncBoth:           "Two-way",
	msgSyncSend:           "Send only to %s",
	msgSyncReceive:        "Receive only from %s",
	msgSyncOff:            "Don't sync",
	msgSyncDirectionLabel: "Sync direction for %s",
	msgSyncDescSyncOff:    "Device sync is off. Turn it on to set the sync direction for %s.",
	msgSyncDescOffline:    "The device is offline. Reconnect over a direct link to set the sync direction for %s.",
	msgSyncDescNormal:     "Affects only %s content on this device.",
	msgSyncThisDevice:     "the device",

	msgSyncStatusTitle:         "Sync status",
	msgSyncStateSynced:         "Synced",
	msgSyncStatePartial:        "Partially synced",
	msgSyncStateOffline:        "Other device offline",
	msgSyncStateReplaced:       "Replaced",
	msgSyncStateNone:           "Not synced",
	msgSyncDeliveryReceived:    "Received",
	msgSyncDeliveryOffline:     "Other device offline",
	msgSyncDeliveryPending:     "Waiting to send",
	msgSyncDeliveryNotSent:     "Not delivered",
	msgSyncResendAll:           "Resend all",
	msgSyncResendAllAccess:     "Resend to all devices",
	msgSyncResend:              "Resend",
	msgSyncResendAccess:        "Resend to %s",
	msgSyncNoDevices:           "No paired devices yet",
	msgSyncReplacedNote:        "Never synced to any device. Replaced by a newer record.",
	msgSyncServiceUnavailable:  "Sync isn't available yet",
	msgHistorySourceThisDevice: "This device",
	msgHistorySourceLanDevice:  "LAN device",
	msgHistorySourceLanFormat:  "From %s",

	msgSettingsGeneral:               "General",
	msgSettingsGeneralDesc:           "Your device name and basic GoPaste preferences.",
	msgSettingsBasics:                "Basics",
	msgSettingsDeviceName:            "Device name",
	msgSettingsDeviceNameDesc:        "Identifies this device in the device list",
	msgSettingsDeviceNamePlaceholder: "Enter a device name",
	msgSettingsLaunchAtLogin:         "Open at Login",
	msgSettingsLaunchAtLoginDesc:     "Open GoPaste automatically when you log in",
	msgSettingsLanguage:              "Language",
	msgSettingsLanguageSystem:        "System Default",
	msgSettingsLanguageDesc:          "Choose the interface language",
	msgSettingsConfirmDelete:         "Confirm before deleting",
	msgSettingsConfirmDeleteDesc:     "Ask before deleting a single record. When off, records are deleted right away; clearing history and unpairing still ask first.",
	msgSettingsAppearance:            "Appearance",
	msgSettingsAppearanceDesc:        "Choose the color theme for GoPaste.",
	msgSettingsVisualStyle:           "Visual style",
	msgSettingsMode:                  "Mode",
	msgSettingsModeDesc:              "Follow the system, or always use light or dark",
	msgSettingsThemeSystem:           "Auto",
	msgSettingsThemeLight:            "Light",
	msgSettingsThemeDark:             "Dark",
	msgSettingsCurrentAppearance:     "Currently using %s",
	msgSettingsThemeLightName:        "Light theme",
	msgSettingsThemeDarkName:         "Dark theme",
	msgSettingsThemeSystemName:       "Auto",
	msgSettingsInterface:             "Interface",
	msgSettingsZoom:                  "Interface zoom",
	msgSettingsZoomDesc:              "Zoom the whole interface with ⌘=, ⌘- and ⌘0",
	msgSettingsZoomOut:               "Zoom out",
	msgSettingsZoomIn:                "Zoom in",
	msgSettingsZoomReset:             "Actual size",
	msgMenuView:                      "View",
	msgMenuUndo:                      "Undo",
	msgMenuZoomIn:                    "Zoom in",
	msgMenuZoomOut:                   "Zoom out",
	msgMenuZoomReset:                 "Actual size",
	msgToastZoom:                     "Interface zoom: %s",
	msgSettingsStorage:               "Storage",
	msgSettingsStorageDesc:           "View and clear history saved on this device.",
	msgSettingsData:                  "Data",
	msgSettingsHistoryRow:            "History",
	msgSettingsHistoryRowDesc:        "Clipboard records saved on this device",
	msgSettingsNoRecords:             "No records",
	msgSettingsRecordCount:           "%d records",
	msgSettingsRecordCountOne:        "%d record",
	msgSettingsImageCount:            "%d images",
	msgSettingsImageCountOne:         "%d image",
	msgSettingsClearHistoryRow:       "Clear history",
	msgSettingsClearHistoryRowDesc:   "Delete all records saved on this device",
	msgSettingsClearHistory:          "Clear history",
	msgSettingsNothingToClear:        "Nothing to clear",
	msgSettingsNoHistoryNote:         "No history on this device yet. Content you copy appears on the History page automatically.",
	msgSettingsAbout:                 "About",
	msgSettingsAboutDesc:             "A clipboard history and LAN sync tool.",
	msgSettingsAppInfo:               "App info",
	msgSettingsVersion:               "Version",
	msgSettingsVersionDesc:           "Current app version",
	msgSettingsSyncScope:             "Sync scope",
	msgSettingsSyncScopeDesc:         "Direct connections only between paired devices",
	msgSettingsPrivacy:               "Privacy",
	msgSettingsPrivacyDesc:           "Clipboard content stays on this device and is never written to logs or sent to third-party services",
	msgSettingsStoredLocally:         "Stored on this device",

	msgDangerClearTitle:          "Clear history?",
	msgDangerClearMessage:        "All clipboard history saved on this device will be deleted. This action can't be undone.",
	msgDangerClearConfirm:        "Clear",
	msgDangerDeleteTitle:         "Delete this record?",
	msgDangerDeleteMessage:       "This record will be removed from this device's history. This action can't be undone.",
	msgDangerDeleteMessageFormat: "“%s” will be removed from this device's history. This action can't be undone.",
	msgDangerDeleteConfirm:       "Delete",
	msgDangerDontAskAgain:        "Don't ask again",
	msgDangerUnpairTitle:         "Unpair device?",
	msgDangerUnpairMessage:       "After you unpair “%s”, it will no longer sync with this device.",
	msgDangerUnpairConfirm:       "Unpair",
	msgDangerCancel:              "Cancel",

	msgTrayTurnSyncOff: "Turn sync off",
	msgTrayTurnSyncOn:  "Turn sync on",
	msgTrayDeviceSync:  "Device sync",
	msgTrayNoDevices:   "No paired devices",
	msgTrayOpen:        "Open",
	msgTraySettings:    "Settings",
	msgTrayRelaunch:    "Relaunch",
	msgTrayQuit:        "Quit",

	msgDateToday:     "Today at %s",
	msgDateYesterday: "Yesterday at %s",
	msgDateThisYear:  "Jan 2 at 3:04 PM",
	msgDateOtherYear: "Jan 2, 2006 at 3:04 PM",
	msgTimeLayout:    "3:04 PM",
}

func (l language) catalog() catalog {
	if l == languageEnglish {
		return englishCatalog
	}
	return simplifiedChineseCatalog
}

// init fails fast when the catalogs drift: every key must exist in both
// languages and every message must be non-empty.
func init() {
	for _, l := range []language{languageSimplifiedChinese, languageEnglish} {
		for key, value := range l.catalog() {
			if value == "" {
				panic("i18n: empty message for " + string(key))
			}
		}
	}
	for key := range simplifiedChineseCatalog {
		if _, ok := englishCatalog[key]; !ok {
			panic("i18n: missing English message for " + string(key))
		}
	}
	for key := range englishCatalog {
		if _, ok := simplifiedChineseCatalog[key]; !ok {
			panic("i18n: missing Chinese message for " + string(key))
		}
	}
}

// localizer resolves message keys for one language.
type localizer struct {
	language language
	catalog  catalog
}

func newLocalizer(l language) localizer {
	return localizer{language: l, catalog: l.catalog()}
}

// text returns the message for key.
func (l localizer) text(key messageKey) string { return l.catalog.text(key) }

// format returns the message for key with its verbs filled in.
func (l localizer) format(key messageKey, args ...any) string {
	return fmt.Sprintf(l.text(key), args...)
}

// plural picks the singular message when n is one in English, and the plural
// message otherwise. Chinese has no plural form, so both entries are equal.
func (l localizer) plural(n int, one, many messageKey) string {
	if l.language == languageEnglish && n == 1 {
		return l.text(one)
	}
	return l.text(many)
}

// pluralFormat fills the plural form of a counted message.
func (l localizer) pluralFormat(n int, one, many messageKey, args ...any) string {
	return fmt.Sprintf(l.plural(n, one, many), args...)
}

// recordCount describes how many history records are shown, optionally out of a
// filtered total.
func (l localizer) recordCount(count, total int, filtered bool) string {
	if filtered {
		return l.format(msgHistoryCountFiltered, count, total)
	}
	return l.pluralFormat(count, msgHistoryCountOne, msgHistoryCount, count)
}

// charCount describes the length of a text record.
func (l localizer) charCount(n int) string {
	return l.pluralFormat(n, msgHistoryCharCountOne, msgHistoryCharCount, n)
}

// storageRecord describes the number of stored records.
func (l localizer) storageRecord(n int) string {
	if n == 0 {
		return l.text(msgSettingsNoRecords)
	}
	return l.pluralFormat(n, msgSettingsRecordCountOne, msgSettingsRecordCount, n)
}

// imageCount describes the number of stored images.
func (l localizer) imageCount(n int) string {
	return l.pluralFormat(n, msgSettingsImageCountOne, msgSettingsImageCount, n)
}

// onlineCount describes how many paired devices are online.
func (l localizer) onlineCount(n int) string {
	return l.format(msgDevicesOnlineCount, n)
}

// kindLabel names a history content kind for display.
func (l localizer) kindLabel(kind contentKind) string {
	switch kind {
	case kindLink:
		return l.text(msgKindLink)
	case kindCode:
		return l.text(msgKindCode)
	case kindImage:
		return l.text(msgKindImage)
	case kindFile:
		return l.text(msgKindFile)
	case kindRichText:
		return l.text(msgKindRichText)
	default:
		return l.text(msgKindText)
	}
}

// smartTag returns the automatic tag shown for a text subtype, or "".
func (l localizer) smartTag(kind contentKind) string {
	switch kind {
	case kindLink:
		return l.text(msgKindLink)
	case kindCode:
		return l.text(msgKindCode)
	default:
		return ""
	}
}

// syncDirectionLabel names a sync direction, naming the peer for the
// directional choices.
func (l localizer) syncDirectionLabel(direction syncDirection, peerName string) string {
	switch direction {
	case syncDirectionSendOnly:
		return l.format(msgSyncSend, peerName)
	case syncDirectionReceiveOnly:
		return l.format(msgSyncReceive, peerName)
	case syncDirectionOff:
		return l.text(msgSyncOff)
	default:
		return l.text(msgSyncBoth)
	}
}

// syncDirectionOptions pairs each direction with its label for the current
// language, so a select can bind to the stable direction value.
func (l localizer) syncDirectionOptions(peerName string) []option[syncDirection] {
	return []option[syncDirection]{
		{value: syncDirectionTwoWay, label: l.syncDirectionLabel(syncDirectionTwoWay, peerName)},
		{value: syncDirectionSendOnly, label: l.syncDirectionLabel(syncDirectionSendOnly, peerName)},
		{value: syncDirectionReceiveOnly, label: l.syncDirectionLabel(syncDirectionReceiveOnly, peerName)},
		{value: syncDirectionOff, label: l.syncDirectionLabel(syncDirectionOff, peerName)},
	}
}

// dateLabel renders a timestamp with a relative day for today and yesterday and
// an explicit date otherwise. The dash marks an unknown time.
func (l localizer) dateLabel(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	now := time.Now()
	switch daysAgo := calendarDaysAgo(value, now); {
	case daysAgo <= 0:
		return l.format(msgDateToday, value.Format(l.text(msgTimeLayout)))
	case daysAgo == 1:
		return l.format(msgDateYesterday, value.Format(l.text(msgTimeLayout)))
	case value.Year() == now.Year():
		return value.Format(l.text(msgDateThisYear))
	default:
		return value.Format(l.text(msgDateOtherYear))
	}
}
