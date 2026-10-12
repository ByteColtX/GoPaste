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

// languageChoices lists the language picker options. Each label is the
// language's own name, so it stays legible whatever the current language is.
var languageChoices = []option[language]{
	{value: languageSimplifiedChinese, label: "简体中文"},
	{value: languageEnglish, label: "English"},
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
	// Navigation.
	msgNavHistory  messageKey = "nav.history"
	msgNavDevices  messageKey = "nav.devices"
	msgNavSettings messageKey = "nav.settings"

	// History filters.
	msgFilterAll       messageKey = "filter.all"
	msgFilterText      messageKey = "filter.text"
	msgFilterLinks     messageKey = "filter.links"
	msgFilterCode      messageKey = "filter.code"
	msgFilterImages    messageKey = "filter.images"
	msgFilterFiles     messageKey = "filter.files"
	msgFilterFavorites messageKey = "filter.favorites"
	msgFilterCurrent   messageKey = "filter.current"

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
	msgHistoryCopy               messageKey = "history.copy"
	msgHistoryCopyFailed         messageKey = "history.copyFailed"

	// Content kind names.
	msgKindText     messageKey = "kind.text"
	msgKindLink     messageKey = "kind.link"
	msgKindCode     messageKey = "kind.code"
	msgKindImage    messageKey = "kind.image"
	msgKindFile     messageKey = "kind.file"
	msgKindRichText messageKey = "kind.richText"

	// Favorites.
	msgFavorite      messageKey = "favorite.add"
	msgFavoriteAdded messageKey = "favorite.added"

	// Toasts.
	msgToastCopied                     messageKey = "toast.copied"
	msgToastFavoriteAdded              messageKey = "toast.favoriteAdded"
	msgToastFavoriteRemoved            messageKey = "toast.favoriteRemoved"
	msgToastHistoryCleared             messageKey = "toast.historyCleared"
	msgToastUnpaired                   messageKey = "toast.unpaired"
	msgToastLaunchAtLoginUnsupported   messageKey = "toast.launchAtLoginUnsupported"
	msgToastLaunchAtLoginOn            messageKey = "toast.launchAtLoginOn"
	msgToastLaunchAtLoginOff           messageKey = "toast.launchAtLoginOff"
	msgToastDeviceDiscoveryUnavailable messageKey = "toast.deviceDiscoveryUnavailable"

	// Devices.
	msgDevicesTitle            messageKey = "devices.title"
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
	msgDevicesSelect           messageKey = "devices.select"
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
	msgSettingsLanguageDesc          messageKey = "settings.languageDesc"
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
	msgDangerClearTitle    messageKey = "danger.clear.title"
	msgDangerClearMessage  messageKey = "danger.clear.message"
	msgDangerClearConfirm  messageKey = "danger.clear.confirm"
	msgDangerUnpairTitle   messageKey = "danger.unpair.title"
	msgDangerUnpairMessage messageKey = "danger.unpair.message"
	msgDangerUnpairConfirm messageKey = "danger.unpair.confirm"
	msgDangerCancel        messageKey = "danger.cancel"

	// Tray menu.
	msgTrayTurnSyncOff messageKey = "tray.turnSyncOff"
	msgTrayTurnSyncOn  messageKey = "tray.turnSyncOn"
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

	msgFilterAll:       "全部",
	msgFilterText:      "文本",
	msgFilterLinks:     "链接",
	msgFilterCode:      "代码",
	msgFilterImages:    "图片",
	msgFilterFiles:     "文件",
	msgFilterFavorites: "收藏",
	msgFilterCurrent:   "当前",

	msgHistoryRecent:             "最近复制",
	msgHistoryCount:              "%d 条记录",
	msgHistoryCountOne:           "%d 条记录",
	msgHistoryCountFiltered:      "%d / %d 条记录",
	msgHistorySearchLabel:        "搜索历史记录",
	msgHistorySearchPlaceholder:  "搜索",
	msgHistoryClearSearch:        "清除搜索",
	msgHistoryEmptyTitle:         "没有匹配记录",
	msgHistoryEmptyMessage:       "换个关键词或筛选条件试试",
	msgHistoryEmptyFirstTitle:    "这里还没有记录",
	msgHistoryEmptyFirstMessage:  "复制文本或图片后，历史记录会显示在这里。记录保存在本机。",
	msgHistoryEmptySearchTitle:   "没有找到匹配记录",
	msgHistoryEmptySearchMessage: "没有包含“%s”的记录。可以清除搜索或换个关键词。",
	msgHistoryEmptyFilterTitle:   "这个筛选条件暂无记录",
	msgHistoryEmptyFilterMessage: "当前没有“%s”记录。可以显示全部记录。",
	msgHistoryResetSearchAndFilt: "清除搜索和筛选",
	msgHistoryResetSearch:        "清除搜索",
	msgHistoryShowAll:            "显示全部记录",
	msgHistorySelectRecord:       "选择一条记录",
	msgHistoryPreviewHint:        "预览会显示在这里",
	msgHistoryCharCount:          "%d 字符",
	msgHistoryCharCountOne:       "%d 字符",
	msgHistoryImageUnavailable:   "图片预览不可用",
	msgHistoryImageLabel:         "图片（%dx%d）",
	msgHistoryImageShort:         "图片",
	msgHistoryCopy:               "复制",
	msgHistoryCopyFailed:         "复制失败",

	msgKindText:     "文本",
	msgKindLink:     "链接",
	msgKindCode:     "代码",
	msgKindImage:    "图片",
	msgKindFile:     "文件",
	msgKindRichText: "富文本",

	msgFavorite:      "收藏",
	msgFavoriteAdded: "已收藏",

	msgToastCopied:                     "已复制到系统剪贴板",
	msgToastFavoriteAdded:              "已加入收藏",
	msgToastFavoriteRemoved:            "已取消收藏",
	msgToastHistoryCleared:             "历史记录已清空",
	msgToastUnpaired:                   "已取消配对",
	msgToastLaunchAtLoginUnsupported:   "当前运行环境暂不支持开机自启",
	msgToastLaunchAtLoginOn:            "已开启开机自启",
	msgToastLaunchAtLoginOff:           "已关闭开机自启",
	msgToastDeviceDiscoveryUnavailable: "设备发现服务尚未接入",

	msgDevicesTitle:            "本设备",
	msgDevicesPaired:           "已配对设备",
	msgDevicesRefresh:          "刷新设备列表",
	msgDevicesOnlineCount:      "%d 台在线",
	msgDevicesEmptyTitle:       "暂无已配对设备",
	msgDevicesEmptyMessage:     "通过直连配对后，设备会显示在这里。\n剪贴板内容只在已配对设备之间同步。",
	msgDevicesLoadingTitle:     "正在加载设备",
	msgDevicesLoadingMessage:   "正在检查本机已配对的设备。",
	msgDevicesErrorTitle:       "设备列表加载失败",
	msgDevicesErrorMessage:     "暂时无法读取已配对设备。",
	msgDevicesRetry:            "重试",
	msgDevicesSelect:           "选择设备",
	msgDevicesAdd:              "新增设备",
	msgDevicesOfflineTitle:     "设备已离线",
	msgDevicesOfflineMessage:   "该设备当前不在线，重新连接后自动恢复同步。",
	msgDevicesLastSeen:         "上次在线：%s",
	msgDevicesBadgePaired:      "已配对",
	msgDevicesBadgeLocal:       "本机",
	msgDevicesUnpair:           "取消配对",
	msgDevicesDirect:           "直连",
	msgDevicesOnline:           "在线",
	msgDevicesOffline:          "离线",
	msgDevicesSyncOn:           "同步已开启",
	msgDevicesSyncOff:          "同步已关闭",
	msgDevicesEnableSync:       "启用同步",
	msgDevicesEnableSyncDesc:   "允许设备之间同步内容",
	msgDevicesFileSync:         "文件同步",
	msgDevicesFileSyncDesc:     "允许在设备间传输文件",
	msgDevicesFileSyncDisabled: "%s 请先开启设备同步。",
	msgDevicesSyncWith:         "与%s同步",
	msgDevicesSyncWithDesc:     "双方复制的内容会自动同步。",
	msgDevicesSyncContent:      "同步内容",
	msgDevicesSyncContentDesc:  "分别设置每种内容的同步方向。",
	msgDevicesAllTwoWay:        "全部双向同步",
	msgDevicesCustomSync:       "自定义同步内容",
	msgDevicesInfoTitle:        "设备信息",
	msgDevicesConnectionTitle:  "连接信息",
	msgDevicesInfoPlatform:     "平台",
	msgDevicesInfoPeerID:       "Peer ID",
	msgDevicesInfoConnection:   "连接方式",
	msgDevicesInfoAppVersion:   "应用版本",
	msgDevicesInfoAddress:      "地址",
	msgDevicesInfoLocalValue:   "本机",
	msgDevicesInfoDirectValue:  "直连",
	msgDevicesExpand:           "展开%s",
	msgDevicesCollapse:         "收起%s",
	msgDevicesPairDirectOnly:   "仅支持直连",
	msgDevicesPairDescription:  "设备配对功能正在接入中。完成协议接入后，可在这里发现设备并完成配对。GoPaste 不使用云端、中继或其他网络回退连接。",
	msgDevicesPairWaiting:      "等待配对服务…",
	msgDevicesPairStart:        "开始配对",
	msgDevicesPairWaitingSvc:   "等待配对服务",
	msgDevicesPairClose:        "关闭",

	msgSyncBoth:           "双向同步",
	msgSyncSend:           "仅发往 %s",
	msgSyncReceive:        "仅从 %s 接收",
	msgSyncOff:            "不同步",
	msgSyncDirectionLabel: "%s 同步方向",
	msgSyncDescSyncOff:    "设备同步已关闭，开启后可设置%s的方向。",
	msgSyncDescOffline:    "设备离线，重新建立直连后可设置%s的方向。",
	msgSyncDescNormal:     "仅影响此设备的%s内容。",
	msgSyncThisDevice:     "该设备",

	msgSettingsGeneral:               "通用",
	msgSettingsGeneralDesc:           "设备身份和 GoPaste 的基础使用偏好。",
	msgSettingsBasics:                "基本设置",
	msgSettingsDeviceName:            "设备名称",
	msgSettingsDeviceNameDesc:        "用于在设备列表中识别本机",
	msgSettingsDeviceNamePlaceholder: "输入设备名称",
	msgSettingsLaunchAtLogin:         "开机自启",
	msgSettingsLaunchAtLoginDesc:     "登录系统后自动打开 GoPaste",
	msgSettingsLanguage:              "语言",
	msgSettingsLanguageDesc:          "选择应用界面语言",
	msgSettingsAppearance:            "外观",
	msgSettingsAppearanceDesc:        "选择 GoPaste 使用的颜色主题。",
	msgSettingsVisualStyle:           "视觉风格",
	msgSettingsMode:                  "模式",
	msgSettingsModeDesc:              "跟随系统或固定使用浅色、深色主题",
	msgSettingsThemeSystem:           "跟随系统",
	msgSettingsThemeLight:            "浅色",
	msgSettingsThemeDark:             "深色",
	msgSettingsCurrentAppearance:     "当前使用：%s",
	msgSettingsThemeLightName:        "浅色主题",
	msgSettingsThemeDarkName:         "深色主题",
	msgSettingsThemeSystemName:       "跟随系统",
	msgSettingsStorage:               "存储",
	msgSettingsStorageDesc:           "查看和清理保存在本机的历史记录。",
	msgSettingsData:                  "数据管理",
	msgSettingsHistoryRow:            "历史记录",
	msgSettingsHistoryRowDesc:        "保存在本机的剪贴板记录",
	msgSettingsNoRecords:             "暂无记录",
	msgSettingsRecordCount:           "%d 条",
	msgSettingsRecordCountOne:        "%d 条",
	msgSettingsImageCount:            "%d 张图片",
	msgSettingsImageCountOne:         "%d 张图片",
	msgSettingsClearHistoryRow:       "清理历史记录",
	msgSettingsClearHistoryRowDesc:   "删除当前保存的所有本地记录",
	msgSettingsClearHistory:          "清空历史记录",
	msgSettingsNothingToClear:        "没有可清理的记录",
	msgSettingsNoHistoryNote:         "暂无本机历史记录。复制内容后，记录会自动出现在历史页面。",
	msgSettingsAbout:                 "关于",
	msgSettingsAboutDesc:             "GoPaste 剪贴板历史记录工具。",
	msgSettingsAppInfo:               "应用信息",
	msgSettingsVersion:               "版本",
	msgSettingsVersionDesc:           "当前应用版本",
	msgSettingsSyncScope:             "同步范围",
	msgSettingsSyncScopeDesc:         "只在已配对设备之间建立直连",
	msgSettingsPrivacy:               "隐私",
	msgSettingsPrivacyDesc:           "剪贴板内容保存在本机，不写入普通日志或第三方服务",
	msgSettingsStoredLocally:         "本机保存",

	msgDangerClearTitle:    "清空历史记录？",
	msgDangerClearMessage:  "GoPaste 会删除本机保存的全部剪贴板历史记录。此操作不可恢复。",
	msgDangerClearConfirm:  "清空",
	msgDangerUnpairTitle:   "取消配对设备？",
	msgDangerUnpairMessage: "取消配对“%s”后，该设备将不再与本机同步。",
	msgDangerUnpairConfirm: "取消配对",
	msgDangerCancel:        "取消",

	msgTrayTurnSyncOff: "关闭同步",
	msgTrayTurnSyncOn:  "打开同步",
	msgTrayOpen:        "打开",
	msgTraySettings:    "设置",
	msgTrayRelaunch:    "重启",
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
	msgFilterText:      "Text",
	msgFilterLinks:     "Links",
	msgFilterCode:      "Code",
	msgFilterImages:    "Images",
	msgFilterFiles:     "Files",
	msgFilterFavorites: "Favorites",
	msgFilterCurrent:   "Current",

	msgHistoryRecent:             "Recent copies",
	msgHistoryCount:              "%d records",
	msgHistoryCountOne:           "%d record",
	msgHistoryCountFiltered:      "%d / %d records",
	msgHistorySearchLabel:        "Search history",
	msgHistorySearchPlaceholder:  "Search",
	msgHistoryClearSearch:        "Clear search",
	msgHistoryEmptyTitle:         "No matching records",
	msgHistoryEmptyMessage:       "Try a different keyword or filter.",
	msgHistoryEmptyFirstTitle:    "No records yet",
	msgHistoryEmptyFirstMessage:  "Copied text or images show up here. Records stay on this device.",
	msgHistoryEmptySearchTitle:   "No results found",
	msgHistoryEmptySearchMessage: "No records contain “%s”. Clear the search or try another keyword.",
	msgHistoryEmptyFilterTitle:   "No records for this filter",
	msgHistoryEmptyFilterMessage: "There are no “%s” records. Show all records.",
	msgHistoryResetSearchAndFilt: "Clear search and filters",
	msgHistoryResetSearch:        "Clear search",
	msgHistoryShowAll:            "Show all records",
	msgHistorySelectRecord:       "Select a record",
	msgHistoryPreviewHint:        "The preview shows up here",
	msgHistoryCharCount:          "%d characters",
	msgHistoryCharCountOne:       "%d character",
	msgHistoryImageUnavailable:   "Image preview unavailable",
	msgHistoryImageLabel:         "Image (%d×%d)",
	msgHistoryImageShort:         "Image",
	msgHistoryCopy:               "Copy",
	msgHistoryCopyFailed:         "Copy failed",

	msgKindText:     "Text",
	msgKindLink:     "Link",
	msgKindCode:     "Code",
	msgKindImage:    "Image",
	msgKindFile:     "File",
	msgKindRichText: "Rich text",

	msgFavorite:      "Favorite",
	msgFavoriteAdded: "Favorited",

	msgToastCopied:                     "Copied to the system clipboard",
	msgToastFavoriteAdded:              "Added to favorites",
	msgToastFavoriteRemoved:            "Removed from favorites",
	msgToastHistoryCleared:             "History cleared",
	msgToastUnpaired:                   "Unpaired",
	msgToastLaunchAtLoginUnsupported:   "Launch at login isn't supported here",
	msgToastLaunchAtLoginOn:            "Launch at login turned on",
	msgToastLaunchAtLoginOff:           "Launch at login turned off",
	msgToastDeviceDiscoveryUnavailable: "Device discovery isn't available yet",

	msgDevicesTitle:            "This device",
	msgDevicesPaired:           "Paired devices",
	msgDevicesRefresh:          "Refresh device list",
	msgDevicesOnlineCount:      "%d online",
	msgDevicesEmptyTitle:       "No paired devices",
	msgDevicesEmptyMessage:     "Devices show up here after you pair them over a direct connection.\nClipboard content syncs only between paired devices.",
	msgDevicesLoadingTitle:     "Loading devices",
	msgDevicesLoadingMessage:   "Checking this device's paired devices.",
	msgDevicesErrorTitle:       "Couldn't load devices",
	msgDevicesErrorMessage:     "Paired devices are temporarily unavailable.",
	msgDevicesRetry:            "Retry",
	msgDevicesSelect:           "Select device",
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
	msgDevicesSyncOn:           "Sync on",
	msgDevicesSyncOff:          "Sync off",
	msgDevicesEnableSync:       "Enable sync",
	msgDevicesEnableSyncDesc:   "Let paired devices sync content",
	msgDevicesFileSync:         "File sync",
	msgDevicesFileSyncDesc:     "Let files transfer between devices",
	msgDevicesFileSyncDisabled: "%s Turn on device sync first.",
	msgDevicesSyncWith:         "Sync with %s",
	msgDevicesSyncWithDesc:     "Content copied on either device syncs automatically.",
	msgDevicesSyncContent:      "Synced content",
	msgDevicesSyncContentDesc:  "Set a sync direction for each content type.",
	msgDevicesAllTwoWay:        "All two-way",
	msgDevicesCustomSync:       "Custom sync",
	msgDevicesInfoTitle:        "Device info",
	msgDevicesConnectionTitle:  "Connection info",
	msgDevicesInfoPlatform:     "Platform",
	msgDevicesInfoPeerID:       "Peer ID",
	msgDevicesInfoConnection:   "Connection",
	msgDevicesInfoAppVersion:   "App version",
	msgDevicesInfoAddress:      "Address",
	msgDevicesInfoLocalValue:   "This device",
	msgDevicesInfoDirectValue:  "Direct",
	msgDevicesExpand:           "Show %s",
	msgDevicesCollapse:         "Hide %s",
	msgDevicesPairDirectOnly:   "Direct connections only",
	msgDevicesPairDescription:  "Pairing isn't available yet. Once it lands, you can discover and pair devices here. GoPaste never uses the cloud, relays, or other network fallbacks.",
	msgDevicesPairWaiting:      "Waiting for pairing…",
	msgDevicesPairStart:        "Start pairing",
	msgDevicesPairWaitingSvc:   "Waiting for pairing service",
	msgDevicesPairClose:        "Close",

	msgSyncBoth:           "Two-way",
	msgSyncSend:           "Send only to %s",
	msgSyncReceive:        "Receive only from %s",
	msgSyncOff:            "Don't sync",
	msgSyncDirectionLabel: "%s sync direction",
	msgSyncDescSyncOff:    "Device sync is off. Turn it on to set the %s direction.",
	msgSyncDescOffline:    "The device is offline. Reconnect over a direct link to set the %s direction.",
	msgSyncDescNormal:     "Affects only this device's %s content.",
	msgSyncThisDevice:     "this device",

	msgSettingsGeneral:               "General",
	msgSettingsGeneralDesc:           "Device identity and basic GoPaste preferences.",
	msgSettingsBasics:                "Basics",
	msgSettingsDeviceName:            "Device name",
	msgSettingsDeviceNameDesc:        "Identifies this device in the device list",
	msgSettingsDeviceNamePlaceholder: "Enter a device name",
	msgSettingsLaunchAtLogin:         "Launch at login",
	msgSettingsLaunchAtLoginDesc:     "Open GoPaste automatically after you sign in",
	msgSettingsLanguage:              "Language",
	msgSettingsLanguageDesc:          "Choose the interface language",
	msgSettingsAppearance:            "Appearance",
	msgSettingsAppearanceDesc:        "Choose the color theme GoPaste uses.",
	msgSettingsVisualStyle:           "Visual style",
	msgSettingsMode:                  "Mode",
	msgSettingsModeDesc:              "Follow the system, or always use light or dark",
	msgSettingsThemeSystem:           "System",
	msgSettingsThemeLight:            "Light",
	msgSettingsThemeDark:             "Dark",
	msgSettingsCurrentAppearance:     "Currently using %s",
	msgSettingsThemeLightName:        "Light theme",
	msgSettingsThemeDarkName:         "Dark theme",
	msgSettingsThemeSystemName:       "System",
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
	msgSettingsNoHistoryNote:         "No history on this device yet. Copied content appears on the History page automatically.",
	msgSettingsAbout:                 "About",
	msgSettingsAboutDesc:             "A clipboard history tool by GoPaste.",
	msgSettingsAppInfo:               "App info",
	msgSettingsVersion:               "Version",
	msgSettingsVersionDesc:           "Current app version",
	msgSettingsSyncScope:             "Sync scope",
	msgSettingsSyncScopeDesc:         "Direct connections only between paired devices",
	msgSettingsPrivacy:               "Privacy",
	msgSettingsPrivacyDesc:           "Clipboard content stays on this device and is never written to logs or third-party services",
	msgSettingsStoredLocally:         "Stored locally",

	msgDangerClearTitle:    "Clear history?",
	msgDangerClearMessage:  "GoPaste will delete all clipboard history saved on this device. This can't be undone.",
	msgDangerClearConfirm:  "Clear",
	msgDangerUnpairTitle:   "Unpair device?",
	msgDangerUnpairMessage: "After you unpair “%s”, it will no longer sync with this device.",
	msgDangerUnpairConfirm: "Unpair",
	msgDangerCancel:        "Cancel",

	msgTrayTurnSyncOff: "Turn sync off",
	msgTrayTurnSyncOn:  "Turn sync on",
	msgTrayOpen:        "Open",
	msgTraySettings:    "Settings",
	msgTrayRelaunch:    "Relaunch",
	msgTrayQuit:        "Quit",

	msgDateToday:     "Today %s",
	msgDateYesterday: "Yesterday %s",
	msgDateThisYear:  "Jan 2, 3:04 PM",
	msgDateOtherYear: "Jan 2, 2006, 3:04 PM",
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
