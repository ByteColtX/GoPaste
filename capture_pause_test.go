package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"gopaste/internal/settings"

	"github.com/egoist/mygo/ui"
)

// TestCapturePausedNoticeAppearsAndResumes 检查暂停提示条：不暂停时列表顶部没有
// 它，暂停后出现，点里面的「恢复捕获」就恢复捕获并把它收起来。
func TestCapturePausedNoticeAppearsAndResumes(t *testing.T) {
	app := previewApp("")
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	title := app.t(msgCapturePausedTitle)
	if tt.HasText(title) {
		t.Fatalf("未暂停时不该显示暂停提示，实际显示：%q", tt.Texts())
	}

	app.setCapturePaused(true)
	tt.Frame()
	if !tt.HasText(title) {
		t.Fatalf("暂停后应显示暂停提示，实际显示：%q", tt.Texts())
	}
	if !tt.HasText(app.t(msgCaptureResume)) {
		t.Errorf("暂停提示里应有「恢复捕获」按钮，实际显示：%q", tt.Texts())
	}

	if err := tt.Click(app.t(msgCaptureResume)); err != nil {
		t.Fatalf("点击恢复捕获失败：%v", err)
	}
	tt.Frame()
	if app.capturePaused {
		t.Fatalf("点击「恢复捕获」后应恢复捕获")
	}
	if tt.HasText(title) {
		t.Errorf("恢复捕获后不该再显示暂停提示，实际显示：%q", tt.Texts())
	}
}

// pixelRun 是一段连续像素的起止坐标。
type pixelRun struct{ min, max int }

// verticalRuns 返回某一列上 from..to 之间连续非底色的色段，用来找卡片卡在哪里。
func verticalRuns(img *image.RGBA, x, from, to int, background color.RGBA) []pixelRun {
	var runs []pixelRun
	start := -1
	for y := from; y <= to; y++ {
		ink := y < to && img.RGBAAt(x, y) != background
		switch {
		case ink && start < 0:
			start = y
		case !ink && start >= 0:
			runs = append(runs, pixelRun{start, y - 1})
			start = -1
		}
	}
	return runs
}

// horizontalRun 返回某一行上 from..to 之间非底色像素的范围。
func horizontalRun(img *image.RGBA, y, from, to int, background color.RGBA) pixelRun {
	run := pixelRun{min: -1}
	for x := from; x < to; x++ {
		if img.RGBAAt(x, y) == background {
			continue
		}
		if run.min < 0 {
			run.min = x
		}
		run.max = x
	}
	return run
}

// TestCapturePausedNoticeAlignsWithHistoryRows 检查暂停提示与历史记录行左右对齐。
// 两者是同一个列表里的卡片，边不齐看起来就像贴错了一层；只改内边距或外边距
// 很容易把右边推出去 4 DIP，所以用像素量一遍。
func TestCapturePausedNoticeAlignsWithHistoryRows(t *testing.T) {
	app := previewApp("")
	app.setCapturePaused(true)
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	img := tt.Image()
	// 列表空白处的底色：用它区分卡片与页面。
	background := img.RGBAAt(250, 400)
	// 从提示标题往上找提示卡片的上边，再往下就是第一行。
	title, ok := tt.Find(app.t(msgCapturePausedTitle))
	if !ok {
		t.Fatalf("暂停提示里应显示标题，实际显示：%q", tt.Texts())
	}
	bands := verticalRuns(img, 236, int(title.Y)-10, 260, background)
	if len(bands) < 2 {
		t.Fatalf("应能找到暂停提示与历史记录行两段卡片，实际 %d 段", len(bands))
	}
	// 量卡片竖直方向的中点：圆角只在上下两端收窄，中间才是卡片的完整宽度。
	// 横向扫到分隔线之前（分隔线在 538），超出列表列的东西不算卡片。
	notice := horizontalRun(img, (bands[0].min+bands[0].max)/2, 220, 536, background)
	row := horizontalRun(img, (bands[1].min+bands[1].max)/2, 220, 536, background)
	if notice != row {
		t.Errorf("暂停提示与历史记录行左右未对齐：提示 %v，行 %v", notice, row)
	}
}

// TestSetCapturePausedNotifiesOnlyOnChange 检查暂停开关只在状态真的变化时通知运行时：
// 重复设置同一个值不该再停一次监听。
func TestSetCapturePausedNotifiesOnlyOnChange(t *testing.T) {
	app := previewApp("")
	notified := 0
	app.onCapturePausedChanged = func(bool) { notified++ }

	app.setCapturePaused(true)
	app.setCapturePaused(true)
	if notified != 1 {
		t.Fatalf("暂停两次只应通知一次，实际 %d 次", notified)
	}
	app.setCapturePaused(false)
	if notified != 2 {
		t.Fatalf("恢复捕获应再通知一次，实际 %d 次", notified)
	}
}

// TestPausingCaptureStopsTheListener 检查暂停真的取消捕获上下文：监听 goroutine 与
// 平台侧的监听一起结束，而不是继续读剪贴板再把结果丢掉。
func TestPausingCaptureStopsTheListener(t *testing.T) {
	rt := newApplicationRuntime("")
	ctx, cancel := context.WithCancel(context.Background())
	rt.captureCancel = cancel
	rt.app.onCapturePausedChanged = rt.capturePausedChanged

	rt.app.setCapturePaused(true)

	if rt.captureCancel != nil {
		t.Errorf("暂停后不该还留着捕获上下文")
	}
	if !rt.captureSuspended {
		t.Errorf("暂停时应记下监听本来在跑，恢复时才知道要不要重启")
	}
	if ctx.Err() == nil {
		t.Errorf("暂停后捕获上下文应已取消")
	}
}

// TestCapturePauseIsNotPersisted 检查暂停状态不写进设置文件：它是临时开关，重启后
// 回到正常捕获。
func TestCapturePauseIsNotPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(path, settings.File{Onboarding: settings.Onboarding{Completed: true}}); err != nil {
		t.Fatalf("写设置文件失败：%v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读设置文件失败：%v", err)
	}

	app := previewApp("")
	app.settingsPath = path
	app.setCapturePaused(true)

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读设置文件失败：%v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("暂停捕获改动了设置文件：%s → %s", before, after)
	}
}
