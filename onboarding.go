package main

import (
	"log"
	"strings"

	"gopaste/internal/settings"
)

// onboardingStep 是首次启动向导停在哪一步。onboardingStepNone 表示不显示向导。
type onboardingStep uint8

const (
	onboardingStepNone onboardingStep = iota
	// onboardingStepWelcome 是第一步：选一条路径。
	onboardingStepWelcome
	// onboardingStepDetail 是第二步：按路径起名字，或者说明去哪里配对。
	onboardingStepDetail
	// onboardingStepDone 是第三步：复述结果，然后结束。
	onboardingStepDone
)

// onboardingStepCount 是向导的步骤数，用于「第 1 步，共 3 步」。
const onboardingStepCount = 3

// onboardingPath 是用户在第一步选的路径。
type onboardingPath uint8

const (
	// onboardingFirstDevice 是「这是我的第一台设备」：起个名字就完事。
	onboardingFirstDevice onboardingPath = iota
	// onboardingJoinExisting 是「我已经在另一台设备上使用」：向导只说明配对在哪里
	// 完成，真正的配对交给设备页——配对码由一方显示、另一方输入，那一来一回需要
	// 设备发现与网络，放在弹窗里做等于把设备页的流程抄一遍。
	onboardingJoinExisting
)

// startOnboarding 从第一步开始向导，并用当前设备名预填名字草稿。预填的是设备页
// 显示的那个名字（默认名会去掉 .local），用户看到的就是以后会看到的样子。
func (a *application) startOnboarding() {
	a.onboarding = onboardingStepWelcome
	a.onboardingPath = onboardingFirstDevice
	a.onboardingName = a.deviceDisplayName()
	a.confirmSkipOnboarding = false
}

// onboardingVisible 报告向导是否正显示着。向导占满内容区期间窗口快捷键不该再开
// 别的弹窗，所以它们先问这里。
func (a *application) onboardingVisible() bool {
	return a.onboarding != onboardingStepNone
}

// advanceOnboarding 走到下一步：第一步选完路径，第二步进入复述，第三步结束向导。
func (a *application) advanceOnboarding() {
	switch a.onboarding {
	case onboardingStepWelcome:
		a.onboarding = onboardingStepDetail
		// 换路径不该带着上一次的残留：设备名草稿只在为空时重新预填，用户已经
		// 打进去的字不覆盖。
		if a.onboardingName == "" {
			a.onboardingName = a.deviceDisplayName()
		}
	case onboardingStepDetail:
		a.onboarding = onboardingStepDone
	case onboardingStepDone:
		a.finishOnboarding(false)
	}
}

// backOnboarding 从第二步退回第一步，让选错路径的人不必跳过整个向导。
func (a *application) backOnboarding() {
	if a.onboarding == onboardingStepDetail {
		a.onboarding = onboardingStepWelcome
	}
}

// requestSkipOnboarding 处理「跳过」与 Escape。跳过之后向导不会再自动出现，所以前
// 两步先弹一句确认；第三步已经没有任何可跳过的输入，关掉它等同于「开始使用」。
func (a *application) requestSkipOnboarding() {
	if a.onboarding == onboardingStepDone {
		a.finishOnboarding(false)
		return
	}
	a.confirmSkipOnboarding = true
}

// dismissOnboarding 真正结束向导。它只由确认框里的「仍然跳过」调用：Escape 和
// 「跳过」现在都先走 requestSkipOnboarding，不会直接跳过。
func (a *application) dismissOnboarding() {
	a.confirmSkipOnboarding = false
	a.finishOnboarding(a.onboarding != onboardingStepDone)
}

// finishOnboarding 结束向导：应用设备名、标记完成并落盘，然后落到设备页——不管走
// 了哪条路径，设备页都是接下来要看的那个页面（「已在另一台设备用过」的人正是在
// 那里点「添加设备」）。
//
// skip 为真时是跳过：保留默认设备名。跳过同样算完成，所以向导不会在下次启动时
// 再出现。
func (a *application) finishOnboarding(skip bool) {
	if !skip && a.onboardingPath == onboardingFirstDevice {
		if name := strings.TrimSpace(a.onboardingName); name != "" && name != a.deviceDisplayName() {
			// 和预填的名字一样就什么都不做：默认名继续跟着系统主机名走。
			a.deviceName = name
			a.applyDeviceName()
		}
	}
	a.onboarding = onboardingStepNone
	a.onboardingCompleted = true
	a.confirmSkipOnboarding = false
	a.saveSettings()
	a.setPage(pageDevices)
}

// applySettings 把读到的设置应用到界面状态，并决定是否显示首次启动向导。
//
// 它只写内存状态：语言、外观与缩放要等运行时在主线程上应用（见 runtime.ready），
// 所以测试可以脱离 MyGo 直接走这条路径。
func (a *application) applySettings(file settings.File) {
	if file.DeviceName != "" {
		a.deviceName = file.DeviceName
	}
	a.applyPreferences(file.UI)
	a.onboardingCompleted = file.Onboarding.Completed
	if !a.onboardingCompleted {
		a.startOnboarding()
	}
}

// saveSettings 把要跨启动保留的项写进设置文件：向导的完成标记、改过的设备名，以及
// 界面偏好。每次都是整份重写，所以向导结束或任一项偏好改变都会带上其它字段。写失败
// 只记日志：本次运行内的状态照旧，下次启动会重新显示向导、偏好回到默认，这是可接受
// 的降级。
func (a *application) saveSettings() {
	if a.settingsPath == "" {
		return
	}
	file := settings.File{
		Onboarding: settings.Onboarding{Completed: a.onboardingCompleted},
		UI:         a.preferences(),
	}
	// 默认名不落盘：它跟着系统主机名走，写进文件会把这一刻的主机名冻住，用户
	// 之后改主机名就不再生效。只有用户真的改过名字才记下来。
	if a.deviceName != a.deviceNameDefault {
		file.DeviceName = a.deviceName
	}
	if err := settings.Save(a.settingsPath, file); err != nil {
		log.Printf("无法保存设置: %v", err)
	}
}
