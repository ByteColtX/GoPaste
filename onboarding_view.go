package main

import (
	"strings"

	"gopaste/internal/settings"

	"github.com/egoist/mygo/ui"
)

// onboardingPage 是首次启动向导的整页形态：向导期间不画侧栏、历史列表和预览，
// 整个窗口内容区都属于它。窗口尺寸与标题栏不变——不进系统全屏，也不新开窗口，
// 因此完成或跳过后回到三栏界面时，窗口不会变形、也不会掉出原来的桌面空间。
//
// Escape 等于跳过：以前由弹窗自己处理，整页之后由这里注册。向导期间没有别的
// 快捷键，所以不存在和输入框争抢 Escape 的情况。
func (a *application) onboardingPage(c *ui.Context, theme *ui.Theme) {
	c.OnShortcut(0, ui.KeyEscape, a.dismissOnboarding)
	// 内容至少和可视区一样高，中间那层才能把向导纵向居中；内容比可视区高时
	// MinHeight 被自然高度顶开，滚动容器接手，不会裁掉按钮行。
	_, windowHeight := c.Size()
	viewportHeight := windowHeight - chromeHeight(c)
	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		paneTop(c)
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().MinHeight(viewportHeight).Justify(ui.Center).Children(func() {
				ui.Column(c).FillWidth().MaxWidth(onboardingMaxWidth(theme)).AlignSelf(ui.Center).
					Padding(space(theme, onboardingTopInset), space(theme, spacePageSide),
						space(theme, onboardingTopInset), space(theme, spacePageSide)).
					Gap(space(theme, spaceSection)).Children(func() {
					a.onboardingHeader(c, theme)
					switch a.onboarding {
					case onboardingStepWelcome:
						a.onboardingPathChoices(c, theme)
					case onboardingStepDetail:
						a.onboardingDetailFields(c, theme)
					case onboardingStepDone:
						a.onboardingSummary(c, theme)
					}
					// 按钮行与弹窗时一样靠右：主操作在尾侧，跳过在它前面。
					ui.Row(c).FillWidth().Shrink(0).Justify(ui.End).Gap(space(theme, dialogGap)).
						Children(func() { a.onboardingActions(c, theme) })
				})
			})
		})
	})
}

// onboardingMaxWidth 是向导内容栏的宽度上限。向导只有一条纵向阅读线，太宽反而难读；
// 三栏界面的内容宽度是给列表和预览用的，不适用于这里。
func onboardingMaxWidth(theme *ui.Theme) float32 {
	return spaceDIP(theme, onboardingColumnWidth)
}

// onboardingTopInset 是向导内容距拖拽条的下方留白（单位：4 DIP 网格）。
const onboardingTopInset = 10

// onboardingColumnWidth 是向导内容栏的宽度（DIP）。
const onboardingColumnWidth = 560

// onboardingHeader 是向导的页头：标题、状态、说明与步骤。整块都是窗口拖动区，
// 因为整页形态没有标题栏带可拖。
func (a *application) onboardingHeader(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).Shrink(0).Gap(space(theme, 1.5)).DragWindow().Children(func() {
		ui.Text(c, a.onboardingTitle()).FontSize(font(theme, fontPageTitle)).FontWeight(600).
			Role(ui.RoleHeading).SingleLine()
		// 第一步就把这条约束说清楚：这个应用只连同一局域网里的已配对设备。
		if badge := a.onboardingBadge(); badge != "" {
			ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(space(theme, 1.5)).Children(func() {
				statusDot(c, accentColor(theme), space(theme, 2))
				ui.Text(c, badge).FontSize(font(theme, fontControl)).FontWeight(600).
					TextColor(accentColor(theme)).SingleLine()
			})
		}
		if message := a.onboardingMessage(); message != "" {
			ui.Text(c, message).FontSize(font(theme, fontBody)).
				TextColor(secondaryText(theme)).Wrap()
		}
		ui.Text(c, a.tf(msgOnboardingStep, int(a.onboarding), onboardingStepCount)).
			FontSize(font(theme, fontMeta)).TextColor(secondaryText(theme)).SingleLine()
	})
}

func (a *application) onboardingTitle() string {
	switch a.onboarding {
	case onboardingStepDetail:
		if a.onboardingPath == onboardingJoinExisting {
			return a.t(msgOnboardingJoinTitle)
		}
		return a.t(msgOnboardingNameTitle)
	case onboardingStepDone:
		return a.t(msgOnboardingDoneTitle)
	default:
		return a.t(msgOnboardingWelcomeTitle)
	}
}

func (a *application) onboardingMessage() string {
	switch a.onboarding {
	case onboardingStepDetail:
		if a.onboardingPath == onboardingJoinExisting {
			return a.t(msgOnboardingJoinMessage)
		}
		return a.t(msgOnboardingNameMessage)
	case onboardingStepDone:
		// 结果复述属于内容，不是说明：它在下面单独排。
		return ""
	default:
		return a.t(msgOnboardingWelcomeMessage)
	}
}

// onboardingBadge 只在第一步给出「仅支持直连」这条状态，与配对对话框用的是同一句
// 话：两处说的是同一件事，不该各写一遍。
func (a *application) onboardingBadge() string {
	if a.onboarding == onboardingStepWelcome {
		return a.t(msgDevicesPairDirectOnly)
	}
	return ""
}

// onboardingStepIndicator 不再单独绘制：步骤文字现在属于页头，见 onboardingHeader。

// onboardingPathChoices 是第一步的两条路径。每一项整行可点（RadioBase），
// 说明文字属于这个选择本身，而不是悬在旁边的注解。
func (a *application) onboardingPathChoices(c *ui.Context, theme *ui.Theme) {
	ui.RadioGroup(c, func() {
		a.onboardingPathOption(c, theme, onboardingFirstDevice,
			a.t(msgOnboardingPathFirstTitle), a.t(msgOnboardingPathFirstDesc))
		a.onboardingPathOption(c, theme, onboardingJoinExisting,
			a.t(msgOnboardingPathJoinTitle), a.t(msgOnboardingPathJoinDesc))
	}).Label(a.t(msgOnboardingPathLabel)).FillWidth()
}

func (a *application) onboardingPathOption(
	c *ui.Context,
	theme *ui.Theme,
	path onboardingPath,
	title string,
	description string,
) {
	selected := a.onboardingPath == path
	key := "onboarding-path-first"
	if path == onboardingJoinExisting {
		key = "onboarding-path-join"
	}
	row := ui.RadioBase(c.Key(key), &a.onboardingPath, path).
		FillWidth().Shrink(0).AlignItems(ui.Start).Gap(space(theme, 2)).
		Padding(space(theme, 2), space(theme, 2.5)).
		Radius(radius(theme, cardRadius)).Transition(motionColors).
		Label(title).Description(description)
	switch {
	case selected:
		row.Background(selectedFill(theme)).Border(1, selectionBorder(c, theme))
	case row.Hovered():
		row.Background(hoverFill(theme)).Border(1, borderColor(theme))
	default:
		row.Background(theme.Background).Border(1, borderColor(theme))
	}
	row.Children(func() {
		onboardingRadioDot(c, theme, selected)
		ui.Column(c).Grow(1).MinWidth(0).Gap(space(theme, 0.5)).Children(func() {
			ui.Text(c, title).FontSize(font(theme, fontBody)).FontWeight(500).SingleLine()
			ui.Text(c, description).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme)).Wrap()
		})
	})
}

// onboardingRadioDot 画出 RadioBase 缺省不画的那颗圆点。外观与 ui.Radio 保持一致，
// 但整行都是点击区域——选项带说明文字时，让只有圆圈能点会很难命中。
func onboardingRadioDot(c *ui.Context, theme *ui.Theme, on bool) {
	dot := ui.Box(c).Size(space(theme, 4), space(theme, 4)).Shrink(0).Center().
		Radius(space(theme, 2))
	if on {
		dot.Background(theme.Accent)
	} else {
		dot.Background(theme.Background).Border(1, theme.Border.Mix(theme.Text, 0.25))
	}
	dot.Children(func() {
		if !on {
			return
		}
		ui.Box(c).Size(space(theme, 1.5), space(theme, 1.5)).
			Radius(space(theme, 0.75)).Background(theme.AccentText)
	})
}

// onboardingDetailFields 是第二步的输入：第一台设备起名字，另一台设备填配对码。
// 输入框与它下面的一两句说明自成一组（组内小间距），与外层的段间距分开：
// 「6 位数字」和「配对服务还没接入」是同一段话的两行，不能排成两个段。
func (a *application) onboardingDetailFields(c *ui.Context, theme *ui.Theme) {
	ui.Column(c).FillWidth().Shrink(0).Gap(space(theme, 1.5)).Children(func() {
		if a.onboardingPath == onboardingJoinExisting {
			ui.TextInput(c.Key("onboarding-pairing-code"), &a.pairingCode).
				Height(controlHeightSize(c, theme, controlHeight)).Width(onboardingCodeFieldWidth).
				AlignSelf(ui.Start).
				Padding(0, space(theme, 2.5)).Radius(radius(theme, controlRadius)).
				Background(theme.Background).Border(1, borderColor(theme)).
				Label(a.t(msgOnboardingPairingCodeLabel)).Placeholder("000000").
				AutoFocus().OnSubmit(a.advanceOnboarding)
			hint, invalid := a.pairingCodeHint()
			hintColor := secondaryText(theme)
			if invalid {
				hintColor = theme.Danger
			}
			ui.Text(c, hint).FontSize(font(theme, fontMeta)).TextColor(hintColor).Wrap()
			// 配对服务还没接，先把这件事说在填之前，而不是让用户填完才发现。
			ui.Text(c, a.t(msgOnboardingPairingPending)).FontSize(font(theme, fontMeta)).
				TextColor(secondaryText(theme)).Wrap()
			return
		}
		ui.TextInput(c.Key("onboarding-device-name"), &a.onboardingName).
			Height(controlHeightSize(c, theme, controlHeight)).FillWidth().
			Padding(0, space(theme, 2.5)).Radius(radius(theme, controlRadius)).
			Background(theme.Background).Border(1, borderColor(theme)).
			Label(a.t(msgOnboardingNameLabel)).Placeholder(a.deviceDisplayName()).
			AutoFocus().OnSubmit(a.advanceOnboarding)
	})
}

// onboardingCodeFieldWidth 是配对码输入框的宽度（DIP）：6 位数字不需要整栏那么宽。
const onboardingCodeFieldWidth = 320

// onboardingSummary 复述向导的结果。配对码在这里按分组显示，方便和另一台设备
// 屏幕上的一致对得上；启动提示里则只露尾号。
func (a *application) onboardingSummary(c *ui.Context, theme *ui.Theme) {
	message := ""
	if a.onboardingPath == onboardingJoinExisting {
		code, ok := settings.PairingCode(a.pairingCode)
		if !ok {
			code = a.pairingCode
		}
		message = a.tf(msgOnboardingDoneJoin, groupPairingCode(code))
	} else {
		message = a.tf(msgOnboardingDoneFirst, a.onboardingDeviceName())
	}
	ui.Text(c, message).FontSize(font(theme, fontBody)).TextColor(primaryText(theme)).Wrap()
}

// onboardingDeviceName 返回向导最终会用的设备名：草稿为空就用当前的设备名。
func (a *application) onboardingDeviceName() string {
	if name := strings.TrimSpace(a.onboardingName); name != "" {
		return name
	}
	return a.deviceDisplayName()
}

// onboardingActions 是按钮行：上一步（第二步才有）、跳过（第三步没有，因为那时
// 已经没什么可放弃的）、继续/开始使用。
func (a *application) onboardingActions(c *ui.Context, theme *ui.Theme) {
	if a.onboarding == onboardingStepDetail {
		backLabel := a.t(msgOnboardingBack)
		back := neutralButton(c, theme, "onboarding-back", backLabel)
		back.Children(func() {
			ui.Text(c, backLabel).FontSize(font(theme, fontControl)).SingleLine()
		})
		if back.Clicked() {
			a.backOnboarding()
		}
	}

	if a.onboarding != onboardingStepDone {
		skipLabel := a.t(msgOnboardingSkip)
		skip := neutralButton(c, theme, "onboarding-skip", skipLabel).Gap(space(theme, 1))
		skip.Children(func() {
			ui.Text(c, skipLabel).FontSize(font(theme, fontControl)).SingleLine()
			// Escape 由弹窗自己处理，跳过按钮上标出它，用户不必先发现。
			keycap(c, theme, shortcutKeyLabel(ui.KeyEscape), primaryText(theme))
		})
		if skip.Clicked() {
			a.dismissOnboarding()
		}
	}

	primaryLabel := a.t(msgOnboardingContinue)
	if a.onboarding == onboardingStepDone {
		primaryLabel = a.t(msgOnboardingStart)
	}
	primary := dialogPrimaryButton(c, theme, "onboarding-primary", primaryLabel)
	if a.onboarding == onboardingStepDetail && a.onboardingPath == onboardingJoinExisting {
		// 配对码格式不对时没有可提交的东西；回车提交会走 advanceOnboarding 报错。
		primary.Disabled(!a.pairingCodeAccepted())
	}
	if a.onboarding == onboardingStepDone {
		// 最后一步只有一件事可做，Return 就交给它。
		primary.AutoFocus()
	}
	if primary.Clicked() {
		a.advanceOnboarding()
	}
}

// groupPairingCode 把 6 位配对码按 3+3 分组，和另一台设备屏幕上显示的样子一致。
// 长度不是 6 位时原样返回：这里只负责排版，不负责校验。
func groupPairingCode(code string) string {
	runes := []rune(code)
	if len(runes) != 2*pairingCodeTailLength {
		return code
	}
	return string(runes[:pairingCodeTailLength]) + " " + string(runes[pairingCodeTailLength:])
}
