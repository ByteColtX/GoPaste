package main

import (
	"path/filepath"
	"testing"

	"gopaste/internal/lansync"

	"github.com/egoist/mygo/ui"
)

// dialogFixture 渲染一个只有弹窗的视图，返回面板矩形、这一帧的 Tester 和主题。
// 弹窗用应用自己的主题（applyTheme），量到的几何因此和真实窗口里一致。
func dialogFixture(t *testing.T, build func(c *ui.Context, theme *ui.Theme) dialogSpec) (ui.Rect, *ui.Tester, *ui.Theme) {
	t.Helper()
	app := newApplication()
	open := true
	var panel ui.Rect
	var frameTheme *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) {
		theme := app.applyTheme(c)
		frameTheme = theme
		panel = dialog(c, theme, &open, build(c, theme)).Bounds()
	}, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	if panel.W == 0 {
		t.Fatal("弹窗面板应有宽度")
	}
	return panel, tt, frameTheme
}

// TestDialogsShareOneSurface 检查二次确认与任务型弹窗是同一个表面：同宽、同样
// 居中、四周同样的内边距。这些数值只写在 dialog 里，两条路径不会各走各的。
func TestDialogsShareOneSurface(t *testing.T) {
	const confirmTitle = "删除这条记录？"
	const confirmMessage = "删除后可以立即撤销。"
	confirmPanel, confirmTT, confirmTheme := dialogFixture(t, func(c *ui.Context, theme *ui.Theme) dialogSpec {
		return dialogSpec{
			title:   confirmTitle,
			role:    ui.RoleAlertDialog,
			message: confirmMessage,
			actions: func() {
				dialogDismissButton(c, theme, "fixture-cancel", "取消")
				dialogDangerButton(c, theme, "fixture-confirm", "删除", "D")
			},
		}
	})
	taskPanel, taskTT, _ := dialogFixture(t, func(c *ui.Context, theme *ui.Theme) dialogSpec {
		return dialogSpec{
			title:      "添加设备",
			role:       ui.RoleDialog,
			message:    "配对只在这台设备和对方之间建立直连。",
			badge:      "仅支持直连",
			badgeColor: accentColor(theme),
			actions: func() {
				dialogDismissButton(c, theme, "fixture-close", "关闭")
				dialogPrimaryButton(c, theme, "fixture-start", "开始配对")
			},
		}
	})

	wantWidth := spaceDIP(confirmTheme, dialogWidth)
	center := float32(defaultWindowWidth) / 2
	for _, tc := range []struct {
		name  string
		panel ui.Rect
	}{
		{"二次确认", confirmPanel},
		{"任务型弹窗", taskPanel},
	} {
		if tc.panel.W != wantWidth {
			t.Errorf("%s的面板宽度应为 %v DIP，实际 %v", tc.name, wantWidth, tc.panel.W)
		}
		if got := tc.panel.X + tc.panel.W/2; got != center {
			t.Errorf("%s的面板应水平居中，中心应为 %v，实际 %v", tc.name, center, got)
		}
	}
	if confirmPanel.X != taskPanel.X || confirmPanel.W != taskPanel.W {
		t.Errorf("两个弹窗的面板应左右重合：二次确认 %v，任务型弹窗 %v", confirmPanel, taskPanel)
	}

	// 面板以标题作为可访问名称，所以 Find 先拿到的就是面板本身。
	for _, tc := range []struct {
		name  string
		title string
		tt    *ui.Tester
		panel ui.Rect
	}{
		{"二次确认", confirmTitle, confirmTT, confirmPanel},
		{"任务型弹窗", "添加设备", taskTT, taskPanel},
	} {
		if got, ok := tc.tt.Find(tc.title); !ok || got != tc.panel {
			t.Errorf("%s的面板应以「%s」为可访问名称，实际拿到 %v（ok=%v）", tc.name, tc.title, got, ok)
		}
	}

	// 正文的左缘是面板的左缘加 1px 描边和内边距，两个弹窗都一样。
	wantInset := 1 + space(confirmTheme, dialogPadding)
	message, ok := confirmTT.Find(confirmMessage)
	if !ok {
		t.Fatalf("确认框应显示说明，实际显示：%q", confirmTT.Texts())
	}
	if got := message.X - confirmPanel.X; got != wantInset {
		t.Errorf("说明距面板左缘应为 %v DIP，实际 %v", wantInset, got)
	}
}

// TestDialogWidthFollowsTheZoom 检查弹窗宽度跟着界面缩放走。宽度写成 DIP，必须经
// spaceDIP 换算：直接写 440 的话，放大界面后弹窗还是 440 DIP，里面的正文反而比
// 窗口里的控件更挤。
func TestDialogWidthFollowsTheZoom(t *testing.T) {
	app := newApplication()
	app.setZoom(1.25)

	open := true
	var panel ui.Rect
	var frameTheme *ui.Theme
	tt := ui.NewTester(func(c *ui.Context) {
		theme := app.applyTheme(c)
		frameTheme = theme
		panel = dialog(c, theme, &open, dialogSpec{
			title:   "删除这条记录？",
			role:    ui.RoleAlertDialog,
			message: "删除后可以立即撤销。",
			actions: func() {
				dialogDismissButton(c, theme, "fixture-cancel", "取消")
			},
		}).Bounds()
	}, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	wantWidth := spaceDIP(frameTheme, dialogWidth)
	if wantWidth <= dialogWidth {
		t.Fatalf("缩放 1.25 后宽度应大于 %v DIP，实际 %v", dialogWidth, wantWidth)
	}
	if panel.W != wantWidth {
		t.Errorf("缩放 1.25 后弹窗宽度应为 %v DIP，实际 %v", wantWidth, panel.W)
	}
}

// TestEveryDialogKeepsTheSameDismissPath 检查每个弹窗的退出路径一致：关闭按钮画
// 一个 Esc 键帽，弹窗打开时焦点也落在它上面。焦点在关闭按钮上，Return 因此永远
// 不会触发不可撤销的操作。
func TestEveryDialogKeepsTheSameDismissPath(t *testing.T) {
	t.Run("二次确认", func(t *testing.T) {
		app := previewApp("")
		tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

		tt.Key(0, ui.KeyD)
		if !app.confirmDangerousAction {
			t.Fatalf("按 D 后应打开删除确认框，实际显示：%q", tt.Texts())
		}
		if got := countText(tt.Texts(), "Esc"); got != 1 {
			t.Errorf("确认框应画一个 Esc 键帽，实际 %d 个", got)
		}
		if !tt.Focused(app.t(msgDangerCancel)) {
			t.Errorf("确认框打开时焦点应在「%s」上", app.t(msgDangerCancel))
		}
	})

	t.Run("配对对话框", func(t *testing.T) {
		app := previewApp("")
		app.setPage(pageDevices)
		tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

		tt.Key(ui.Cmd, ui.KeyN)
		if !app.addDeviceDialog {
			t.Fatalf("Cmd+N 后应打开配对对话框，实际显示：%q", tt.Texts())
		}
		if got := countText(tt.Texts(), "Esc"); got != 1 {
			t.Errorf("配对对话框应画一个 Esc 键帽，实际 %d 个", got)
		}
		if !tt.Focused(app.t(msgDevicesPairClose)) {
			t.Errorf("配对对话框打开时焦点应在「%s」上", app.t(msgDevicesPairClose))
		}
	})
}

// TestDialogActionsKeepTheHIGOrder 检查两个弹窗的按钮行同构：取消/关闭在左，主操作
// 或确认在右，两者在同一行上。Apple 的提醒框把最可能选中的按钮放在尾侧、取消放在
// 前侧，两个弹窗都照此排列。
func TestDialogActionsKeepTheHIGOrder(t *testing.T) {
	assertOrder := func(t *testing.T, leading, trailing ui.Rect, leadingLabel, trailingLabel string) {
		t.Helper()
		if leading.X+leading.W > trailing.X {
			t.Errorf("「%s」（右缘 %.1f）应在「%s」（左缘 %.1f）之前",
				leadingLabel, leading.X+leading.W, trailingLabel, trailing.X)
		}
		if leading.Y != trailing.Y || leading.H != trailing.H {
			t.Errorf("「%s」%v 与「%s」%v 应在同一行同一高度",
				leadingLabel, leading, trailingLabel, trailing)
		}
	}

	t.Run("二次确认", func(t *testing.T) {
		app := previewApp("")
		tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
		app.requestDangerousAction(dangerousActionClearHistory, "")
		tt.Frame()

		cancel, ok := tt.Find(app.t(msgDangerCancel))
		if !ok {
			t.Fatalf("确认框应显示「%s」，实际显示：%q", app.t(msgDangerCancel), tt.Texts())
		}
		confirm, ok := tt.Find(app.t(msgDangerClearConfirm))
		if !ok {
			t.Fatalf("确认框应显示「%s」，实际显示：%q", app.t(msgDangerClearConfirm), tt.Texts())
		}
		assertOrder(t, cancel, confirm, app.t(msgDangerCancel), app.t(msgDangerClearConfirm))
	})

	t.Run("配对对话框", func(t *testing.T) {
		app := previewApp("")
		app.setPage(pageDevices)
		// 没有发现到任何设备时，对话框仍然有退出与刷新两个入口。
		service, err := lansync.NewService(lansync.Options{
			StorePath: filepath.Join(t.TempDir(), "devices.json"),
			Name:      app.deviceName,
			Platform:  app.devicePlatform,
		})
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		app.syncService = service
		app.openAddDeviceDialog()
		tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
		tt.Frame()

		closeButton, ok := tt.Find(app.t(msgDevicesPairClose))
		if !ok {
			t.Fatalf("配对对话框应显示「%s」，实际显示：%q", app.t(msgDevicesPairClose), tt.Texts())
		}
		refresh, ok := tt.Find(app.t(msgDevicesPairRefresh))
		if !ok {
			t.Fatalf("配对对话框应显示「%s」，实际显示：%q", app.t(msgDevicesPairRefresh), tt.Texts())
		}
		assertOrder(t, closeButton, refresh, app.t(msgDevicesPairClose), app.t(msgDevicesPairRefresh))
		if !hasExactText(tt.Texts(), app.t(msgDevicesPairNoneFound)) {
			t.Errorf("对话框应说明没有可配对的设备，实际显示：%q", tt.Texts())
		}
	})
}
