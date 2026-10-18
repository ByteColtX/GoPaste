package main

import (
	"path/filepath"
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

// TestJoinPathPointsAtTheDevicePage 检查「已在另一台设备用过」这条路径：向导只说明
// 配对在设备页完成，不收集配对码、不伪造设备，完成后落到设备页。
func TestJoinPathPointsAtTheDevicePage(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	if err := tt.Click(app.t(msgOnboardingPathJoinTitle)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingPathJoinTitle), err)
	}
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()

	if app.onboarding != onboardingStepDetail {
		t.Fatalf("步骤 = %v，期望第二步", app.onboarding)
	}
	// 这一步没有输入框：配对码由一方显示、另一方输入，那一来一回在设备页做。
	if !tt.HasText(app.t(msgOnboardingJoinWhere)) {
		t.Errorf("应说明去哪里配对，实际显示：%q", tt.Texts())
	}

	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if !tt.HasText(app.t(msgOnboardingDoneJoin)) {
		t.Errorf("完成页应说明配对在设备页完成，实际显示：%q", tt.Texts())
	}

	if err := tt.Click(app.t(msgOnboardingStart)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingStart), err)
	}
	tt.Frame()

	if app.page != pageDevices {
		t.Errorf("完成后页面 = %v，期望设备页", app.page)
	}
	if len(app.pairedDevices) != 0 {
		t.Errorf("向导不该凭空产生设备，实际 %d 台", len(app.pairedDevices))
	}
}

// skipThroughConfirmation 点「跳过」，并在确认框里选「仍然跳过」。
func skipThroughConfirmation(t *testing.T, app *application, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(app.t(msgOnboardingSkip)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingSkip), err)
	}
	tt.Frame()
	if !app.confirmSkipOnboarding {
		t.Fatal("点「跳过」后应先弹出确认框")
	}
	if err := tt.Click(app.t(msgOnboardingSkipConfirm)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingSkipConfirm), err)
	}
	tt.Frame()
}

// TestSkipAsksForConfirmation 检查「跳过」不直接结束：先弹一句确认，并把「稍后
// 去哪里做」说清楚。选「继续设置」回到向导，什么都没写进设置。
func TestSkipAsksForConfirmation(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	if err := tt.Click(app.t(msgOnboardingSkip)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingSkip), err)
	}
	tt.Frame()

	if !app.confirmSkipOnboarding {
		t.Fatal("点「跳过」后应弹出确认框")
	}
	if !tt.HasText(app.t(msgOnboardingSkipTitle)) {
		t.Errorf("确认框应显示「%s」，实际显示：%q", app.t(msgOnboardingSkipTitle), tt.Texts())
	}
	if !tt.HasText(app.t(msgOnboardingSkipMessage)) {
		t.Errorf("确认框应说明稍后可以去哪里做，实际显示：%q", tt.Texts())
	}
	if !app.onboardingVisible() {
		t.Error("确认框展开时向导不该结束")
	}
	// Return 不能把人送出向导：焦点落在「继续设置」上。
	if !tt.Focused(app.t(msgOnboardingSkipStay)) {
		t.Errorf("确认框打开时焦点应在「%s」上", app.t(msgOnboardingSkipStay))
	}

	if err := tt.Click(app.t(msgOnboardingSkipStay)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingSkipStay), err)
	}
	tt.Frame()

	if app.confirmSkipOnboarding {
		t.Error("选「继续设置」后确认框应关闭")
	}
	if app.onboarding != onboardingStepWelcome {
		t.Errorf("选「继续设置」后应回到向导，实际步骤 %v", app.onboarding)
	}
	if readSettings(t, path).Onboarding.Completed {
		t.Error("还没跳过，不该写入完成标记")
	}
}

// TestSkipCompletesWithoutRecordingAnything 检查「跳过」：向导不再出现，但既不
// 改设备名，也不记待配对请求。
func TestSkipCompletesWithoutRecordingAnything(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	skipThroughConfirmation(t, app, tt)

	if app.onboardingVisible() {
		t.Error("跳过后向导应关闭")
	}
	if app.deviceName != app.deviceNameDefault {
		t.Errorf("跳过后设备名 = %q，期望默认名", app.deviceName)
	}
	file := readSettings(t, path)
	if !file.Onboarding.Completed {
		t.Error("跳过也算完成，应写入完成标记")
	}
	if file.DeviceName != "" {
		t.Errorf("跳过不该写入设备名，实际 %+v", file)
	}
}

// TestEscapeSkipsTheWizardThroughConfirmation 检查 Escape 与「跳过」走同一条路径：
// 先弹出确认框；再按 Escape 只关掉确认框、回到向导；确认后才算跳过。
func TestEscapeSkipsTheWizardThroughConfirmation(t *testing.T) {
	app, path := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !app.confirmSkipOnboarding {
		t.Fatal("Escape 应先弹出确认框")
	}
	if !app.onboardingVisible() {
		t.Error("第一次 Escape 不该直接结束向导")
	}

	// 确认框自己处理 Escape：关掉的是这一层。
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if app.confirmSkipOnboarding {
		t.Error("第二次 Escape 应关闭确认框")
	}
	if app.onboarding != onboardingStepWelcome {
		t.Errorf("关掉确认框后应回到向导，实际步骤 %v", app.onboarding)
	}
	if readSettings(t, path).Onboarding.Completed {
		t.Error("还没确认跳过，不该写入完成标记")
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingSkipConfirm)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingSkipConfirm), err)
	}
	tt.Frame()
	if app.onboardingVisible() {
		t.Error("确认跳过后向导应关闭")
	}
	if !readSettings(t, path).Onboarding.Completed {
		t.Error("跳过也算完成，应写入完成标记")
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

// TestBackButtonLetsTheUserChangeTheirMind 检查第二步能退回第一步换一条路径。向导
// 没有重开入口，选错路径必须能当场改；换路径后也不该带着上一次的输入。
func TestBackButtonLetsTheUserChangeTheirMind(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	if err := tt.Click(app.t(msgOnboardingPathJoinTitle)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingPathJoinTitle), err)
	}
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()
	if app.onboarding != onboardingStepDetail || app.onboardingPath != onboardingJoinExisting {
		t.Fatalf("应停在配对码那一步，实际步骤 %v、路径 %v", app.onboarding, app.onboardingPath)
	}

	if err := tt.Click(app.t(msgOnboardingBack)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingBack), err)
	}
	tt.Frame()
	if app.onboarding != onboardingStepWelcome {
		t.Fatalf("退回后步骤 = %v，期望第一步", app.onboarding)
	}

	if err := tt.Click(app.t(msgOnboardingPathFirstTitle)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingPathFirstTitle), err)
	}
	tt.Frame()
	if err := tt.Click(app.t(msgOnboardingContinue)); err != nil {
		t.Fatalf("点击「%s」出错: %v", app.t(msgOnboardingContinue), err)
	}
	tt.Frame()

	if app.onboardingPath != onboardingFirstDevice {
		t.Errorf("改选后路径 = %v，期望「这是我的第一台设备」", app.onboardingPath)
	}
}

// TestWizardBlocksTheAddDeviceShortcut 检查向导开着时 Cmd+N 不会再叠一个配对
// 弹窗：模态已经挡住了指针，快捷键不该绕过它。
func TestWizardBlocksTheAddDeviceShortcut(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	tt.Key(ui.Cmd, ui.KeyN)
	if app.addDeviceDialog {
		t.Error("向导开着时 Cmd+N 不应打开配对弹窗")
	}
	if !app.onboardingVisible() {
		t.Error("向导不应被快捷键关掉")
	}
}

// TestWizardReplacesTheWholeWindow 检查向导期间整个内容区都是向导：侧栏与历史列表
// 不在，跳过后三栏界面回来。这是「整页」与「弹窗盖在上面」的区别所在。
func TestWizardReplacesTheWholeWindow(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	if !tt.HasText(app.t(msgOnboardingWelcomeTitle)) {
		t.Fatalf("首次启动应显示向导，实际显示：%q", tt.Texts())
	}
	for _, label := range []string{app.t(msgHistoryRecent), app.t(msgFilterAll)} {
		if tt.HasText(label) {
			t.Errorf("向导期间不该画出三栏界面的 %q，实际显示：%q", label, tt.Texts())
		}
	}

	skipThroughConfirmation(t, app, tt)

	if !tt.HasText(app.t(msgHistoryRecent)) {
		t.Errorf("跳过后应回到三栏界面，实际显示：%q", tt.Texts())
	}
}

// TestWizardContentIsCentered 检查向导内容在窗口里纵向居中：整页形态下如果贴着
// 顶边，上面会空出一大片，读起来像没加载完。
func TestWizardContentIsCentered(t *testing.T) {
	app, _ := onboardingApp(t, settings.File{})
	tt := onboardingTester(app)

	title, ok := tt.Find(app.t(msgOnboardingWelcomeTitle))
	if !ok {
		t.Fatalf("首次启动应显示向导，实际显示：%q", tt.Texts())
	}
	primary, ok := tt.Find(app.t(msgOnboardingContinue))
	if !ok {
		t.Fatalf("第一步应显示「%s」，实际显示：%q", app.t(msgOnboardingContinue), tt.Texts())
	}

	contentCenter := (title.Y + primary.Y + primary.H) / 2
	windowCenter := float32(defaultWindowHeight) / 2
	if diff := contentCenter - windowCenter; diff > 40 || diff < -40 {
		t.Errorf("内容纵向中心 %.1f 偏离窗口中心 %.1f 过多（内容 %v～%v）",
			contentCenter, windowCenter, title, primary)
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
