package main

import (
	"strings"
	"testing"
	"time"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

// 预览元信息行的夹具：内容是 40 个「剪贴板」，所以字符数恒为 120 字符、大小
// 恒为 360 字节（每个汉字 3 字节）。时间取本地时区的固定日期，显示文本不随
// 机器时区变化。
const (
	previewCharLabel = "120 个字符"
	previewSizeLabel = "360 B"
	previewStamp     = "2020年3月4日 05:06"
)

func previewItem(source string) history.Item {
	return history.Item{
		ID:             "item",
		Kind:           kindText,
		Text:           strings.Repeat("剪贴板", 40),
		SourceDeviceID: source,
		CreatedAt:      time.Date(2020, 3, 4, 5, 6, 7, 0, time.Local),
		Sync:           syncStateSynced,
	}
}

func previewApp(source string) *application {
	app := newApplication()
	app.history = history.New([]history.Item{previewItem(source)})
	app.pairedDevices = nil
	return app
}

// hasExactText 报告一组文本里是否有一个恰好等于 want 的文本。
func hasExactText(texts []string, want string) bool {
	for _, text := range texts {
		if text == want {
			return true
		}
	}
	return false
}

// TestFitMetaSegments 检查片段取舍：放得下的留下，放不下的从最后一个开始丢，
// 而且每个片段除了自身宽度，还要算上它让行多出来的两个间距。
func TestFitMetaSegments(t *testing.T) {
	cases := []struct {
		name     string
		width    float32
		fixed    float32
		segments []float32
		want     int
	}{
		{"都放得下", 120, 40, []float32{20, 30}, 2},
		{"只放得下第一个", 70, 40, []float32{20, 30}, 1},
		{"第一个就放不下", 50, 40, []float32{20, 30}, 0},
		{"没有片段", 100, 40, nil, 0},
		{"宽度为 0", 0, 0, []float32{20}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fitMetaSegments(tc.width, tc.fixed, 5, tc.segments); got != tc.want {
				t.Errorf("fitMetaSegments(%v, %v, 5, %v) = %d，期望 %d",
					tc.width, tc.fixed, tc.segments, got, tc.want)
			}
		})
	}
}

// TestPreviewContentWidthFollowsThePanes 检查元信息行拿到的可用宽度：窗口宽度
// 减去筛选栏、历史列表和分割线，再减详情列的左右留白；筛选栏收起时把宽度让给
// 预览，窗口宽到一定程度后详情列不再变宽，窄于最小值时预览栏守住最小宽度。
func TestPreviewContentWidthFollowsThePanes(t *testing.T) {
	app := newApplication()
	app.historyPaneWidth = 320

	var theme *ui.Theme
	widths := make(map[float32]float32)
	view := func(c *ui.Context) {
		theme = c.Theme()
		window, _ := c.Size()
		widths[window] = app.previewContentWidth(c, theme)
		ui.Text(c, "x")
	}
	tt := ui.NewTester(view, defaultWindowWidth, 720)

	pageSide := 2 * space(theme, spacePageSide)
	rail := spaceDIP(theme, filterRailWidth)
	// 窗口宽度减去两侧面板与分割线，再减详情列的左右留白，最后受详情列最大宽度限制。
	wantContent := func(railWidth float32) float32 {
		return min(float32(defaultWindowWidth)-railWidth-320-splitDividerWidth, float32(pageDetailMaxWidth)) - pageSide
	}
	if got := widths[defaultWindowWidth]; got != wantContent(rail) {
		t.Errorf("默认窗口的可用宽度 = %v，期望 %v", got, wantContent(rail))
	}

	// 筛选栏收起：它让出的宽度全部给预览，直到详情列到达最大宽度。
	app.filterRailOpen = false
	tt.Frame()
	if got := widths[defaultWindowWidth]; got != wantContent(0) {
		t.Errorf("收起筛选栏后的可用宽度 = %v，期望 %v", got, wantContent(0))
	}

	// 窗口窄于最小值：预览栏不会小于自己的最小宽度。
	tt.SetSize(760, 720)
	if got, want := widths[760], float32(minPreviewPaneWidth)-pageSide; got != want {
		t.Errorf("窄窗口的可用宽度 = %v，期望 %v", got, want)
	}

	// 窗口很宽：详情列停在最大宽度上。
	tt.SetSize(defaultWindowWidth*3, 720)
	if got, want := widths[defaultWindowWidth*3], float32(pageDetailMaxWidth)-pageSide; got != want {
		t.Errorf("宽窗口的可用宽度 = %v，期望 %v", got, want)
	}
}

// TestPreviewMetaRowHidesSegmentsByWidth 检查元信息行的取舍：宽度只够一个片段时
// 留下字符数、丢掉大小，而类型、来源和时间始终保留。
func TestPreviewMetaRowHidesSegmentsByWidth(t *testing.T) {
	app := previewApp("")
	item := previewItem("")
	segments := app.historyMetaSegments(item)
	if len(segments) != 2 {
		t.Fatalf("文本记录应有两个内容片段，实际 %v", segments)
	}

	view := func(c *ui.Context) {
		theme := c.Theme()
		gap := space(theme, metaRowGap)
		segmentCost := func(segment string) float32 {
			return textWidth(c, "·", font(theme, fontMeta), 0) +
				textWidth(c, segment, font(theme, fontMeta), 0) + 2*gap
		}
		// 刚好放得下第一个片段，第二个差 0.5 DIP。
		width := app.previewMetaFixedWidth(c, theme, item, gap) + segmentCost(segments[0]) + 0.5
		app.previewMetaRow(c, theme, item, width)
	}
	tt := ui.NewTester(view, defaultWindowWidth, 720)
	texts := tt.Texts()

	if !hasExactText(texts, previewCharLabel) {
		t.Errorf("宽度够一个片段时应显示字符数，实际显示：%q", texts)
	}
	if hasExactText(texts, previewSizeLabel) {
		t.Errorf("放不下第二个片段时不应显示大小，实际显示：%q", texts)
	}
	for _, want := range []string{app.kindLabel(kindText), app.t(msgHistorySourceThisDevice), previewStamp} {
		if !hasExactText(texts, want) {
			t.Errorf("类型、来源和时间不应被隐藏，缺少 %q，实际显示：%q", want, texts)
		}
	}
}

// TestPreviewMetaHidesSegmentsBeforeTheTypeAndSource 从整个界面走一遍：窗口够宽
// 时字符数和大小都在；窗口收到最窄、来源又是一台名字很长的设备时，行里放不下
// 任何内容片段，但类型、来源和时间仍然显示。
func TestPreviewMetaHidesSegmentsBeforeTheTypeAndSource(t *testing.T) {
	app := previewApp("")
	tt := ui.NewTester(app.view, 1600, 720)
	for _, want := range []string{previewCharLabel, previewSizeLabel} {
		if !tt.HasText(want) {
			t.Errorf("宽窗口应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}

	device := pairedDevice{id: "peer-a", name: strings.Repeat("设计部", 12), platform: "macOS", online: true}
	app = previewApp(device.id)
	app.pairedDevices = []pairedDevice{device}
	tt = ui.NewTester(app.view, minimumWindowWidth, 720)

	if tt.HasText(previewCharLabel) || tt.HasText(previewSizeLabel) {
		t.Errorf("窄窗口不应显示字符数或大小，实际显示：%q", tt.Texts())
	}
	for _, want := range []string{app.tf(msgHistorySourceLanFormat, device.name), previewStamp} {
		if !tt.HasText(want) {
			t.Errorf("来源和时间不应被隐藏，缺少 %q", want)
		}
	}
}

// TestHistoryPageRendersTheFirstUseEmptyState 检查历史记录还没装载完（或一条都
// 没有）时，整个历史页渲染的是首次使用的空状态。装载在界面线程之外进行，
// 应用启动后的第一帧就是这个状态。
func TestHistoryPageRendersTheFirstUseEmptyState(t *testing.T) {
	app := newApplication()
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	if !tt.HasText(app.t(msgHistoryEmptyFirstTitle)) {
		t.Errorf("空历史应显示 %q，实际显示：%q", app.t(msgHistoryEmptyFirstTitle), tt.Texts())
	}
}
