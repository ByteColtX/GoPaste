package main

import "gopaste/internal/settings"

// 界面偏好跨启动保留：内存里用的是界面直接消费的枚举与数值，设置文件里存的是稳定、
// 可读的名字。两边在这里互相转换，启动恢复与保存都走同一对函数。
//
// 每个偏好都有默认值（跟随系统语言、跟随系统外观、删除前确认开、100%），默认值不
// 落盘：缺字段读回来就是默认，所以「没设过」与「设回默认」是同一件事。

// languagePreferenceName 返回语言偏好在设置文件里的写法。跟随系统是默认值，没有写法。
func languagePreferenceName(preference languagePreference) string {
	switch preference {
	case languagePreferenceSimplifiedChinese:
		return "zh-Hans"
	case languagePreferenceEnglish:
		return "en"
	default:
		return ""
	}
}

// languagePreferenceFromName 把设置文件里的语言写法解析回偏好。认不出的取值
// （旧版本、手改坏的文件）回退到默认的跟随系统。
func languagePreferenceFromName(name string) languagePreference {
	switch name {
	case "zh-Hans":
		return languagePreferenceSimplifiedChinese
	case "en":
		return languagePreferenceEnglish
	default:
		return languagePreferenceSystem
	}
}

// appearancePreferenceName 返回外观模式在设置文件里的写法。跟随系统是默认值，没有写法。
func appearancePreferenceName(mode int) string {
	switch mode {
	case appearanceLight:
		return "light"
	case appearanceDark:
		return "dark"
	default:
		return ""
	}
}

// appearanceFromName 把设置文件里的外观写法解析回模式，认不出就回退到跟随系统。
func appearanceFromName(name string) int {
	switch name {
	case "light":
		return appearanceLight
	case "dark":
		return appearanceDark
	default:
		return appearanceSystem
	}
}

// preferences 把当前界面偏好投影成设置文件里的 ui 对象。全是默认值时返回 nil：
// 那样文件里就不会多出一个空的 ui 对象。
func (a *application) preferences() *settings.UI {
	ui := settings.UI{
		Language:   languagePreferenceName(a.languagePreference),
		Appearance: appearancePreferenceName(a.appearance),
	}
	if !a.confirmBeforeDelete {
		off := false
		ui.ConfirmDelete = &off
	}
	if a.zoom != 1 {
		ui.Zoom = a.zoom
	}
	if ui.Language == "" && ui.Appearance == "" && ui.ConfirmDelete == nil && ui.Zoom == 0 {
		return nil
	}
	return &ui
}

// applyPreferences 把设置文件里的界面偏好写回内存状态。它只赋值、不碰 MyGo：语言与
// 外观要等运行时在主线程上解析并应用（见 runtime.ready），缩放则要等窗口有了所在
// 屏幕之后才能按屏幕可用区域再夹一次。
func (a *application) applyPreferences(ui *settings.UI) {
	if ui == nil {
		return
	}
	a.languagePreference = languagePreferenceFromName(ui.Language)
	a.appearance = appearanceFromName(ui.Appearance)
	if ui.ConfirmDelete != nil {
		a.confirmBeforeDelete = *ui.ConfirmDelete
	}
	if ui.Zoom > 0 {
		// 先按内置范围夹一次；屏幕上限要等窗口打开后才知道。
		a.zoom = min(max(ui.Zoom, zoomMinimum), zoomMaximum)
	}
}
