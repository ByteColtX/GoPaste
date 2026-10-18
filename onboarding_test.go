package main

import (
	"path/filepath"
	"strings"
	"testing"

	"gopaste/internal/settings"

	"github.com/egoist/mygo/ui"
)

// onboardingApp 返回一个接在临时设置文件上的应用，并走真实启动的同一条
// applySettings 路径把文件读进状态。返回的路径可以用来检查落盘结果。
func onboardingApp(t *testing.T, file settings.File) (*application, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "GoPaste", "settings.json")
	if err := settings.Save(path, file); err != nil {
		t.Fatalf("写入测试设置出错: %v", err)
	}
	app := newApplication()
	app.settingsPath = path
	loaded, err := settings.Load(path)
	if err != nil {
		t.Fatalf("读取测试设置出错: %v", err)
	}
	app.applySettings(loaded)
	return app, path
}

// readSettings 读回测试写入的设置文件。
func readSettings(t *testing.T, path string) settings.File {
	t.Helper()
	file, err := settings.Load(path)
	if err != nil {
		t.Fatalf("读回设置出错: %v", err)
	}
	return file
}

// onboardingTester 渲染应用并跑一帧，返回 Tester。
func onboardingTester(app *application) *ui.Tester {
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()
	return tt
}

// TestFirstRunShowsTheWizard 检查没有任何设置时向导出现在第一帧，并且两条路径都
// 摆在第一步上。
func TestFirstRunShowsTheWizard(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	if !tt.HasText(app.t(msgOnboardingWelcomeTitle)) {
		t.Fatalf("首次启动应显示向导，实际显示：%q", tt.Texts())
	}
	for _, label := range []string{
		app.t(msgOnboardingPathFirstTitle),
		app.t(msgOnboardingPathJoinTitle),
		app.t(msgOnboardingContinue),
		app.t(msgOnboardingSkip),
	} {
		if !tt.HasText(label) {
			t.Errorf("第一步应显示 %q，实际显示：%q", label, tt.Texts())
		}
	}
	// 默认选中第一台设备：大多数人只有一台机器，默认值应该落在它上面。
	if app.onboardingPath != onboardingFirstDevice {
		t.Errorf("默认路径 = %v，期望「这是我的第一台设备」", app.onboardingPath)
	}
}

// TestCompletedOnboardingStaysClosed 检查完成过的设置不会再弹向导。
func TestCompletedOnboardingStaysClosed(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{Onboarding: settings.Onboarding{Completed: true}})
	tt := onboardingTester(app)

	if app.onboardingVisible() {
		t.Error("已完成时向导不应显示")
	}
	if tt.HasText(app.t(msgOnboardingWelcomeTitle)) {
		t.Errorf("已完成时窗口里不该有向导，实际显示：%q", tt.Texts())
	}
}

// TestFirstDevicePathRenamesTheDevice 走完第一台设备这条路径：预填默认名、改成
// 自己的名字、完成后落到设备页，名字写进设置文件。
func TestFirstDevicePathRenamesTheDevice(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	// 第二步的设备名输入框预填当前默认名。
	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if app.onboarding != onboardingStepDetail {
		t.Fatalf("点「继续」后步骤 = %v，期望第二步", app.onboarding)
	}
	if app.onboardingName != app.deviceDisplayName() {
		t.Errorf("设备名预填 = %q，期望设备页显示的名字 %q", app.onboardingName, app.deviceDisplayName())
	}

	tt.Command("selectAll")
	tt.Type("Studio Mac")
	tt.Frame()
	if app.onboardingName != "Studio Mac" {
		t.Fatalf("输入后设备名 = %q，期望 Studio Mac", app.onboardingName)
	}

	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if app.onboarding != onboardingStepDone {
		t.Fatalf("提交后步骤 = %v，期望第三步", app.onboarding)
	}
	if !tt.HasText(app.tf(msgOnboardingDoneFirst, "Studio Mac")) {
		t.Errorf("完成页应复述新名字，实际显示：%q", tt.Texts())
	}

	if err := tt.Click(app.t(msgOnboardingStart)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingStart), err)
	}
	tt.Frame()

	if app.onboardingVisible() {
		t.Error("点「开始使用」后向导应关闭")
	}
	if app.deviceName != "Studio Mac" {
		t.Errorf("设备名 = %q，期望 Studio Mac", app.deviceName)
	}
	if app.page != pageDevices {
		t.Errorf("完成后页面 = %v，期望设备页", app.page)
	}
	file := readSettings(t, path)
	if !file.Onboarding.Completed {
		t.Error("完成后应把完成标记写进设置")
	}
	if file.DeviceName != "Studio Mac" {
		t.Errorf("设置里的设备名 = %q，期望 Studio Mac", file.DeviceName)
	}
}

// TestEmptyNameKeepsTheDefault 检查设备名留空就是回到默认名：默认名跟着系统主机名
// 走，所以它不该被写进设置文件冻住。
func TestEmptyNameKeepsTheDefault(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	app.onboarding = onboardingStepDetail
	app.onboardingName = "   "
	tt := onboardingTester(app)

	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingStart)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingStart), err)
	}
	tt.Frame()

	if app.deviceName != app.deviceNameDefault {
		t.Errorf("设备名 = %q，期望保持默认名 %q", app.deviceName, app.deviceNameDefault)
	}
	if file := readSettings(t, path); file.DeviceName != "" {
		t.Errorf("默认名不该写进设置，实际写入 %q", file.DeviceName)
	}
}

// TestUnchangedPrefilledNameIsNotPersisted 检查预填的名字没改就完成时，设置文件里
// 不出现设备名：默认名要继续跟着系统主机名走，不能被向导顺手冻住。
func TestUnchangedPrefilledNameIsNotPersisted(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	app.onboarding = onboardingStepDetail
	app.onboardingName = app.deviceDisplayName()
	tt := onboardingTester(app)

	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingStart)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingStart), err)
	}
	tt.Frame()

	if app.deviceName != app.deviceNameDefault {
		t.Errorf("设备名 = %q，期望仍是默认名 %q", app.deviceName, app.deviceNameDefault)
	}
	if file := readSettings(t, path); file.DeviceName != "" {
		t.Errorf("没改过的名字不该写进设置，实际写入 %q", file.DeviceName)
	}
}

// TestJoinPathRecordsAPendingRequestWithoutADevice 走完「已在另一台设备上使用」：
// 配对码落盘，但不会凭空出现一台已配对设备——设备列表、托盘、来源筛选都不该有它。
func TestJoinPathRecordsAPendingRequestWithoutADevice(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	if err := tt.Click(app.t(msgOnboardingPathJoinTitle)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingPathJoinTitle), err)
	}
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()

	// 配对服务还没接，这件事要在填之前就说清楚。
	if !tt.HasText(app.t(msgOnboardingPairingPending)) {
		t.Errorf("配对码这一步应说明配对服务尚未接入，实际显示：%q", tt.Texts())
	}

	tt.Type("482913")
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if !tt.HasText(app.tf(msgOnboardingDoneJoin, "482 913")) {
		t.Errorf("完成页应显示分组后的配对码，实际显示：%q", tt.Texts())
	}
	if err := tt.Click(app.t(msgOnboardingStart)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingStart), err)
	}
	tt.Frame()

	if app.pendingPairing == nil {
		t.Fatal("完成后应记下待配对请求")
	}
	if app.pendingPairing.Code != "482913" {
		t.Errorf("配对码 = %q，期望 482913", app.pendingPairing.Code)
	}
	if len(app.pairedDevices) != 0 {
		t.Errorf("待配对请求不该变成已配对设备，实际 %d 台", len(app.pairedDevices))
	}
	if devices := app.trayDevices(); len(devices) != 0 {
		t.Errorf("托盘里不该出现待配对设备，实际 %v", devices)
	}
	file := readSettings(t, path)
	if file.PendingPairing == nil || file.PendingPairing.Code != "482913" {
		t.Errorf("设置里的待配对请求 = %+v，期望配对码 482913", file.PendingPairing)
	}
	// 设备页不该给这条请求留任何痕迹：它还没成为一台设备。
	if tt.HasText(app.t(msgOnboardingPendingReminder)) {
		t.Errorf("设备页不该显示待配对占位，实际显示：%q", tt.Texts())
	}
}

// TestIncompletePairingCodeKeepsContinueDisabled 检查配对码没填够时「继续」不可用，
// 点它也不会走到下一步。
func TestIncompletePairingCodeKeepsContinueDisabled(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	app.onboarding = onboardingStepDetail
	app.onboardingPath = onboardingJoinExisting
	app.pairingCode = "482"
	tt := onboardingTester(app)

	if app.pairingCodeAccepted() {
		t.Fatal("3 位配对码不该通过校验")
	}
	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if app.onboarding != onboardingStepDetail {
		t.Errorf("配对码不完整时步骤 = %v，期望仍停在第二步", app.onboarding)
	}
}

// TestRejectedPairingCodeExplainsItself 检查回车提交非法配对码后，字段下方的说明
// 变成错误提示，而不是让用户对着一个没反应的按钮发呆。
func TestRejectedPairingCodeExplainsItself(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	app.onboarding = onboardingStepDetail
	app.onboardingPath = onboardingJoinExisting
	app.pairingCode = "48291a"
	tt := onboardingTester(app)

	if _, invalid := app.pairingCodeHint(); invalid {
		t.Fatal("还没提交过，说明行不该是错误")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()

	if app.onboarding != onboardingStepDetail {
		t.Fatalf("非法配对码不该进入下一步，步骤 = %v", app.onboarding)
	}
	hint, invalid := app.pairingCodeHint()
	if !invalid {
		t.Fatalf("提交过非法配对码后说明行应报错，实际 %q", hint)
	}
	if !tt.HasText(hint) {
		t.Errorf("错误提示应显示出来，实际显示：%q", tt.Texts())
	}
}

// TestSkipCompletesWithoutRecordingAnything 检查「跳过」：向导不再出现，但既不
// 改设备名，也不记待配对请求。
func TestSkipCompletesWithoutRecordingAnything(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	if err := tt.Click(app.t(msgOnboardingSkip)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingSkip), err)
	}
	tt.Frame()

	if app.onboardingVisible() {
		t.Error("跳过后向导应关闭")
	}
	if app.deviceName != app.deviceNameDefault {
		t.Errorf("跳过后设备名 = %q，期望默认名", app.deviceName)
	}
	if app.pendingPairing != nil {
		t.Errorf("跳过后不该记待配对请求，实际 %+v", app.pendingPairing)
	}
	file := readSettings(t, path)
	if !file.Onboarding.Completed {
		t.Error("跳过也算完成，应写入完成标记")
	}
	if file.DeviceName != "" || file.PendingPairing != nil {
		t.Errorf("跳过不该写入别的内容，实际 %+v", file)
	}
}

// TestEscapeSkipsTheWizard 检查 Escape 与「跳过」走同一条路径：弹窗自己的 Escape
// 处理会关掉它，向导据此结束，不留一个状态不明的弹窗。
func TestEscapeSkipsTheWizard(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	if app.onboardingVisible() {
		t.Error("Escape 后向导应关闭")
	}
	if !readSettings(t, path).Onboarding.Completed {
		t.Error("Escape 也算完成，应写入完成标记")
	}
}

// TestPendingPairingRemindsAtStartup 检查待配对请求在下次启动时提示一次，只露尾号。
func TestPendingPairingRemindsAtStartup(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{
		Onboarding:     settings.Onboarding{Completed: true},
		PendingPairing: &settings.PendingPairing{Code: "482913"},
	})

	want := app.tf(msgOnboardingPendingReminder, "913")
	if app.pendingToast != want {
		t.Fatalf("启动提示 = %q，期望 %q", app.pendingToast, want)
	}
	if strings.Contains(app.pendingToast, "482913") {
		t.Errorf("启动提示不该露出完整配对码: %q", app.pendingToast)
	}

	tt := onboardingTester(app)
	if app.pendingToast != "" {
		t.Error("提示应在第一帧发出后被清空")
	}
	if !tt.HasText(want) {
		t.Errorf("第一帧应显示待配对提示，实际显示：%q", tt.Texts())
	}
}

// TestPairingCodeTail 检查提示里只露尾号，短码原样返回。
func TestPairingCodeTail(t *testing.T) {
	cases := []struct{ code, want string }{
		{"482913", "913"},
		{"913", "913"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := pairingCodeTail(tc.code); got != tc.want {
			t.Errorf("pairingCodeTail(%q) = %q，期望 %q", tc.code, got, tc.want)
		}
	}
}

// TestGroupPairingCode 检查完成页把 6 位码按 3+3 分组显示，长度不对时原样返回。
func TestGroupPairingCode(t *testing.T) {
	cases := []struct{ code, want string }{
		{"482913", "482 913"},
		{"482", "482"},
		{"4829134", "4829134"},
	}
	for _, tc := range cases {
		if got := groupPairingCode(tc.code); got != tc.want {
			t.Errorf("groupPairingCode(%q) = %q，期望 %q", tc.code, got, tc.want)
		}
	}
}

// TestWizardLayoutHoldsTogether 检查第一步的排版：两张选项卡上下排列、说明文字在
// 自己的标题下方、整块内容没有压到按钮行。换行或字体变化把说明挤出弹窗，是这种
// 多行选项卡最容易出的事。
func TestWizardLayoutHoldsTogether(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	firstTitle, ok := tt.Find(app.t(msgOnboardingPathFirstTitle))
	if !ok {
		t.Fatalf("第一步应显示第一张选项卡，实际显示：%q", tt.Texts())
	}
	firstDesc, ok := tt.Find(app.t(msgOnboardingPathFirstDesc))
	if !ok {
		t.Fatalf("第一张选项卡应带说明，实际显示：%q", tt.Texts())
	}
	joinTitle, ok := tt.Find(app.t(msgOnboardingPathJoinTitle))
	if !ok {
		t.Fatalf("第一步应显示第二张选项卡，实际显示：%q", tt.Texts())
	}
	joinDesc, ok := tt.Find(app.t(msgOnboardingPathJoinDesc))
	if !ok {
		t.Fatalf("第二张选项卡应带说明，实际显示：%q", tt.Texts())
	}
	skip, ok := tt.Find(app.t(msgOnboardingSkip))
	if !ok {
		t.Fatalf("第一步应显示「%s」，实际显示：%q", app.t(msgOnboardingSkip), tt.Texts())
	}
	primary, ok := tt.Find(app.t(msgOnboardingContinue))
	if !ok {
		t.Fatalf("第一步应显示「%s」，实际显示：%q", app.t(msgOnboardingContinue), tt.Texts())
	}

	if firstTitle.X != joinTitle.X {
		t.Errorf("两张选项卡左缘应对齐：%.1f 与 %.1f", firstTitle.X, joinTitle.X)
	}
	if firstDesc.Y <= firstTitle.Y {
		t.Errorf("说明应在自己的标题下方：标题 %.1f，说明 %.1f", firstTitle.Y, firstDesc.Y)
	}
	if joinTitle.Y <= firstDesc.Y+firstDesc.H {
		t.Errorf("第二张选项卡压住了第一张的说明：说明底边 %.1f，第二张顶边 %.1f",
			firstDesc.Y+firstDesc.H, joinTitle.Y)
	}
	if joinDesc.Y+joinDesc.H > skip.Y {
		t.Errorf("内容压到了按钮行：内容底边 %.1f，按钮顶边 %.1f", joinDesc.Y+joinDesc.H, skip.Y)
	}
	// 与其它弹窗一致：跳过在左、主操作在右，同一行同一高度。
	if skip.X+skip.W > primary.X {
		t.Errorf("「%s」应在「%s」之前", app.t(msgOnboardingSkip), app.t(msgOnboardingContinue))
	}
	if skip.Y != primary.Y || skip.H != primary.H {
		t.Errorf("两个按钮应在同一行同一高度：%v 与 %v", skip, primary)
	}
}

// TestWizardFollowsTheInterfaceLanguage 检查向导跟着界面语言走：英文界面下没有
// 中文残留。
func TestWizardFollowsTheInterfaceLanguage(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	app.languagePreference = languagePreferenceEnglish
	app.applyLanguage()
	tt := onboardingTester(app)

	if !tt.HasText(app.t(msgOnboardingWelcomeTitle)) {
		t.Fatalf("英文界面应显示英文向导，实际显示：%q", tt.Texts())
	}
	for _, text := range tt.Texts() {
		for _, r := range text {
			if r >= 0x4e00 && r <= 0x9fff {
				t.Errorf("英文向导里出现了中文：%q", text)
				break
			}
		}
	}
}
