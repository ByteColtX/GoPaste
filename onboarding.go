package main

import (
	"log"
	"strings"
	"time"

	"gopaste/internal/settings"
)

// onboardingStep 是首次启动向导停在哪一步。onboardingStepNone 表示不显示向导。
type onboardingStep uint8

const (
	onboardingStepNone onboardingStep = iota
	// onboardingStepWelcome 是第一步：选一条路径。
	onboardingStepWelcome
	// onboardingStepDetail 是第二步：按路径填设备名或配对码。
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
	// onboardingJoinExisting 是「我已经在另一台设备上使用」：输入那台设备显示的配对码。
	onboardingJoinExisting
)

// pairingCodeTailLength 是启动提示里露出的配对码位数。
const pairingCodeTailLength = 3

// startOnboarding 从第一步开始向导，并用当前设备名预填名字草稿。预填的是设备页
// 显示的那个名字（默认名会去掉 .local），用户看到的就是以后会看到的样子。
func (a *application) startOnboarding() {
	a.onboarding = onboardingStepWelcome
	a.onboardingPath = onboardingFirstDevice
	a.onboardingName = a.deviceDisplayName()
	a.pairingCode = ""
	a.pairingCodeRejected = false
}

// onboardingVisible 报告向导是否正显示着。模态弹窗期间窗口快捷键不该再开别的
// 弹窗，所以它们先问这里。
func (a *application) onboardingVisible() bool {
	return a.onboarding != onboardingStepNone
}

// advanceOnboarding 走到下一步：第一步选完路径，第二步校验输入，第三步结束向导。
func (a *application) advanceOnboarding() {
	switch a.onboarding {
	case onboardingStepWelcome:
		a.onboarding = onboardingStepDetail
		// 换路径不该带着上一次的残留：回到第一步再选另一条路时，配对码清空，
		// 设备名草稿只在为空时重新预填，用户已经打进去的字不覆盖。
		a.pairingCode = ""
		a.pairingCodeRejected = false
		if a.onboardingName == "" {
			a.onboardingName = a.deviceDisplayName()
		}
	case onboardingStepDetail:
		if a.onboardingPath == onboardingJoinExisting {
			if _, ok := settings.PairingCode(a.pairingCode); !ok {
				// 「继续」在校验不过时是禁用的，能走到这里的只有回车提交。
				a.pairingCodeRejected = true
				return
			}
		}
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

// dismissOnboarding 处理弹窗被关掉：Escape、点弹窗外部和「跳过」都到这里。
// 第三步已经没有任何要放弃的输入，关掉等同于「开始使用」；前两步等同于跳过。
func (a *application) dismissOnboarding() {
	a.finishOnboarding(a.onboarding != onboardingStepDone)
}

// finishOnboarding 结束向导：应用草稿、记下待配对请求、标记完成并落盘，然后落到
// 设备页——不管走了哪条路径，设备页都是接下来要看的那个页面。
//
// skip 为真时是跳过：保留默认设备名，也不记任何待配对请求。跳过同样算完成，
// 所以向导不会在下次启动时再出现。
func (a *application) finishOnboarding(skip bool) {
	if !skip {
		if a.onboardingPath == onboardingJoinExisting {
			// 配对服务还没接：这里只记下请求，不假装配对成功，也不产生设备。
			if code, ok := settings.PairingCode(a.pairingCode); ok {
				a.pendingPairing = &settings.PendingPairing{Code: code, CreatedAt: time.Now().UTC()}
			}
		} else if name := strings.TrimSpace(a.onboardingName); name != "" && name != a.deviceDisplayName() {
			// 和预填的名字一样就什么都不做：默认名继续跟着系统主机名走。
			a.deviceName = name
		}
	}
	a.onboarding = onboardingStepNone
	a.onboardingCompleted = true
	a.pairingCode = ""
	a.pairingCodeRejected = false
	a.saveSettings()
	a.setPage(pageDevices)
}

// pairingCodeAccepted 报告第二步的配对码是否已经可以提交，用来决定「继续」的
// 可用状态。
func (a *application) pairingCodeAccepted() bool {
	_, ok := settings.PairingCode(a.pairingCode)
	return ok
}

// pairingCodeHint 是配对码字段下方的说明行：平时讲格式，回车提交过非法码之后
// 变成错误提示（onboarding_view 据此换成危险色）。
func (a *application) pairingCodeHint() (text string, invalid bool) {
	if a.pairingCodeRejected && !a.pairingCodeAccepted() {
		return a.tf(msgOnboardingPairingCodeBad, settings.PairingCodeLength), true
	}
	return a.tf(msgOnboardingPairingCodeHint, settings.PairingCodeLength), false
}

// applySettings 把读到的设置应用到界面状态，并决定是否显示首次启动向导。
func (a *application) applySettings(file settings.File) {
	if file.DeviceName != "" {
		a.deviceName = file.DeviceName
	}
	a.onboardingCompleted = file.Onboarding.Completed
	a.pendingPairing = file.PendingPairing
	if !a.onboardingCompleted {
		a.startOnboarding()
		return
	}
	// 待配对请求只在这里露面：设备页、侧栏、托盘都不显示它，配对服务接入前它
	// 只是一条本地记录。提示带尾号，全码不上屏。
	if a.pendingPairing != nil {
		a.pendingToast = a.tf(msgOnboardingPendingReminder, pairingCodeTail(a.pendingPairing.Code))
	}
}

// saveSettings 把要跨启动保留的项写进设置文件。写失败只记日志：本次运行内的状态
// 照旧，下次启动会重新显示向导，这是可接受的降级。
func (a *application) saveSettings() {
	if a.settingsPath == "" {
		return
	}
	file := settings.File{
		Onboarding:     settings.Onboarding{Completed: a.onboardingCompleted},
		PendingPairing: a.pendingPairing,
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

// pairingCodeTail 返回配对码的尾号，用在启动提示里：全码不必出现在屏幕上，尾号
// 够用户认出是哪一次请求。
func pairingCodeTail(code string) string {
	runes := []rune(code)
	if len(runes) <= pairingCodeTailLength {
		return code
	}
	return string(runes[len(runes)-pairingCodeTailLength:])
}
