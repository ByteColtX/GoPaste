package main

import "testing"

// TestLanguagePreferenceResolvesAgainstTheLocale 检查语言偏好怎么解析成实际使用的
// 界面语言：固定某种语言时与系统无关，跟随系统时看 locale，非中文一律回退英文。
func TestLanguagePreferenceResolvesAgainstTheLocale(t *testing.T) {
	cases := []struct {
		name       string
		preference languagePreference
		locale     string
		want       language
	}{
		{"跟随系统·简中", languagePreferenceSystem, "zh-Hans-CN", languageSimplifiedChinese},
		{"跟随系统·繁中", languagePreferenceSystem, "zh-Hant-TW", languageSimplifiedChinese},
		{"跟随系统·英文", languagePreferenceSystem, "en-US", languageEnglish},
		{"跟随系统·其他", languagePreferenceSystem, "de-DE", languageEnglish},
		{"跟随系统·空 locale", languagePreferenceSystem, "", languageEnglish},
		{"固定简中·英文系统", languagePreferenceSimplifiedChinese, "en-US", languageSimplifiedChinese},
		{"固定简中·空 locale", languagePreferenceSimplifiedChinese, "", languageSimplifiedChinese},
		{"固定英文·中文系统", languagePreferenceEnglish, "zh-Hans-CN", languageEnglish},
		{"固定英文·空 locale", languagePreferenceEnglish, "", languageEnglish},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			locale := func() string { return tc.locale }
			if got := tc.preference.language(locale); got != tc.want {
				t.Errorf("偏好 %v 在 locale %q 下 = %v，期望 %v", tc.preference, tc.locale, got, tc.want)
			}
		})
	}
}

// TestPinnedLanguageNeverReadsTheLocale 检查固定语言时不会去读系统 locale。
// MyGo 在主线程上读 locale，测试里没有那个线程，读一下就会挂住。
func TestPinnedLanguageNeverReadsTheLocale(t *testing.T) {
	unread := func() string {
		t.Fatal("固定语言时不应读系统 locale")
		return ""
	}
	if got := languagePreferenceSimplifiedChinese.language(unread); got != languageSimplifiedChinese {
		t.Errorf("固定简中 = %v，期望简中", got)
	}
	if got := languagePreferenceEnglish.language(unread); got != languageEnglish {
		t.Errorf("固定英文 = %v，期望英文", got)
	}
}

// TestLanguageDefaultsToFollowingTheSystem 检查默认偏好是「跟随系统」，并且它是
// 选择项里的第一项——默认值排在列表最前。
func TestLanguageDefaultsToFollowingTheSystem(t *testing.T) {
	app := newApplication()
	if app.languagePreference != languagePreferenceSystem {
		t.Errorf("默认语言偏好 = %v，期望跟随系统", app.languagePreference)
	}
	choices := app.l10n().languageChoices()
	if len(choices) != 3 {
		t.Fatalf("语言选择项 = %d 个，期望 3 个", len(choices))
	}
	if choices[0].value != languagePreferenceSystem {
		t.Errorf("第一项 = %v，期望跟随系统", choices[0].value)
	}
}

// TestLanguageChoiceLabels 检查三种选择项的文案：语言名用它们自己的写法，不随界面
// 语言变化；「跟随系统」没有自己的写法，只能跟着界面语言翻译。
func TestLanguageChoiceLabels(t *testing.T) {
	cases := []struct {
		language language
		want     []string
	}{
		{languageSimplifiedChinese, []string{"跟随系统", "简体中文", "English"}},
		{languageEnglish, []string{"System Default", "简体中文", "English"}},
	}
	for _, tc := range cases {
		choices := newLocalizer(tc.language).languageChoices()
		if len(choices) != len(tc.want) {
			t.Fatalf("%v 下有 %d 个选择项，期望 %d 个", tc.language, len(choices), len(tc.want))
		}
		for index, want := range tc.want {
			if choices[index].label != want {
				t.Errorf("%v 下第 %d 项 = %q，期望 %q", tc.language, index, choices[index].label, want)
			}
		}
	}
}

// TestSupportedLanguagesCoverTheCatalogs 检查交给 MyGo 的语言标签和界面真正支持的
// 语言一一对应：每个标签都能解析回一种界面语言，两种界面语言也都有标签。第一种
// 是应用自己的语言，也是都匹配不上时的回退，所以它必须是英语。
func TestSupportedLanguagesCoverTheCatalogs(t *testing.T) {
	tags := supportedLanguages()
	if len(tags) == 0 {
		t.Fatal("没有声明任何语言")
	}
	if got := languageFromLocale(tags[0]); got != languageEnglish {
		t.Errorf("第一种语言 %q 解析成 %v，期望英语——它是应用的回退语言", tags[0], got)
	}
	seen := make(map[language]bool, len(tags))
	for _, tag := range tags {
		seen[languageFromLocale(tag)] = true
	}
	for _, want := range []language{languageSimplifiedChinese, languageEnglish} {
		if !seen[want] {
			t.Errorf("声明的语言 %v 覆盖不到 %v", tags, want)
		}
	}
	if len(seen) != len(tags) {
		t.Errorf("声明的语言 %v 有重复", tags)
	}
}

// TestApplyLanguageFollowsThePreference 检查 applyLanguage 按偏好改写界面语言，
// 固定语言时不看系统 locale。
func TestApplyLanguageFollowsThePreference(t *testing.T) {
	app := newApplication()
	app.languagePreference = languagePreferenceEnglish
	app.applyLanguage()
	if app.language != languageEnglish {
		t.Errorf("固定英文后界面语言 = %v，期望英文", app.language)
	}
	app.languagePreference = languagePreferenceSimplifiedChinese
	app.applyLanguage()
	if app.language != languageSimplifiedChinese {
		t.Errorf("固定简中后界面语言 = %v，期望简中", app.language)
	}
}
