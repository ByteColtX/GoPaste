package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 界面缩放（页面缩放）：像浏览器一样按 Cmd +/-/0 放大、缩小、回到 100%。
//
// 缩放不改变任何布局代码，而是缩放主题的两个基准值：FontSize（字体）与
// Spacing（4 DIP 网格）。界面里所有文字和间距都走 font()/space()，因此整屏
// 一起等比缩放；用 DIP 写死的布局下限（各栏最小宽度、详情列上限）也按同一
// 比例放大，窗口最小尺寸跟着变，窗口不够大时长到刚好放得下（不超过屏幕可用
// 区域），所以放大后不会把内容挤到裁切。
const (
	zoomMinimum = 0.8
	zoomMaximum = 1.5
)

// zoomSteps 是缩放的档位，按顺序递增。用固定档位而不是任意比例：缩放后每个
// 数字仍是整齐的百分比，也不会把 4 DIP 网格变成小数。
var zoomSteps = [...]float32{0.8, 0.9, 1, 1.1, 1.25, 1.5}

// zoomStep 返回从 current 向 direction（-1 缩小、+1 放大）走一档的结果。
// 已经在两端时返回 current。
func zoomStep(current float32, direction int) float32 {
	nearest := 0
	best := float32(math.MaxFloat32)
	for index, step := range zoomSteps {
		if gap := abs32(step - current); gap < best {
			best, nearest = gap, index
		}
	}
	next := nearest + direction
	if next < 0 {
		next = 0
	}
	if next >= len(zoomSteps) {
		next = len(zoomSteps) - 1
	}
	return zoomSteps[next]
}

// zoomLabel 把缩放比例写成百分比标签，例如 125%。
func zoomLabel(zoom float32) string {
	return fmt.Sprintf("%.0f%%", zoom*100)
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

// setZoom 应用新的缩放比例并通知运行时调整窗口。比例夹在允许范围内，上限
// 还受屏幕可用区域限制：界面最小宽度都放不下的比例不给用（见 zoomCeiling）。
// 屏幕上限可能落在两档之间，此时缩放就停在上限本身；档位顺序仍然单调，继续
// 按放大不会有效果，按缩小仍回到下一档。
func (a *application) setZoom(zoom float32) {
	zoom = min(max(zoom, zoomMinimum), a.zoomCeiling())
	a.pendingToast = a.tf(msgToastZoom, zoomLabel(zoom))
	if zoom == a.zoom {
		return
	}
	a.zoom = zoom
	if a.onZoomChanged != nil {
		a.onZoomChanged(zoom)
	}
}

// zoomCeiling 返回当前屏幕允许的最大缩放。运行时按窗口所在屏幕的可用区域
// 算出上限；没有窗口时（测试、无界面）只用内置上限。
func (a *application) zoomCeiling() float32 {
	ceiling := float32(zoomMaximum)
	if a.onMaxZoom != nil {
		if screen := a.onMaxZoom(); screen > 0 {
			ceiling = min(ceiling, screen)
		}
	}
	return max(ceiling, zoomMinimum)
}

// zoomIn、zoomOut、resetZoom 是 Cmd +/-/0 与视图菜单的动作。
func (a *application) zoomIn()  { a.setZoom(zoomStep(a.zoom, 1)) }
func (a *application) zoomOut() { a.setZoom(zoomStep(a.zoom, -1)) }
func (a *application) resetZoom() {
	a.setZoom(1)
}

// applyZoom 把缩放比例写进主题。MyGo 的主题里 FontSize 已经包含系统文字大小
// 设置（Preferences.TextScale），所以这里在它之上再乘一个比例，两层各自独立。
//
// 平台主题的基准值只在它变化时重新读取：读到的字号等于我们上次写回的值，
// 说明这一帧的主题就是自己写的，不该再乘一次，否则每帧都会复利放大。
func (a *application) applyZoom(t *ui.Theme) {
	if t.FontSize != a.zoomedFontSize {
		a.platformFontSize = t.FontSize
	}
	if t.Spacing != a.zoomedSpacing {
		a.platformSpacing = t.Spacing
	}
	if a.platformFontSize <= 0 {
		a.platformFontSize = t.Rem(1)
	}
	if a.platformSpacing <= 0 {
		a.platformSpacing = t.Space(1)
	}
	a.zoomedFontSize = a.platformFontSize * a.zoom
	a.zoomedSpacing = a.platformSpacing * a.zoom
	t.FontSize = a.zoomedFontSize
	t.Spacing = a.zoomedSpacing
}

// zoomedWindowMinimum 返回缩放后的窗口最小尺寸，供窗口与布局共用同一套下限。
func zoomedWindowMinimum(zoom float32) (int, int) {
	width := int(math.Round(float64(float32(minimumWindowWidth) * zoom)))
	height := int(math.Round(float64(float32(minimumWindowHeight) * zoom)))
	return width, height
}

// zoomCeilingForArea 返回屏幕可用区域放得下的最大缩放：界面最小尺寸按比例
// 放大后仍要装得进去。结果向下取到整百分比，上限因此总是一个读得出来的数。
func zoomCeilingForArea(area mygo.Rectangle) float32 {
	if area.Width <= 0 || area.Height <= 0 {
		return 0
	}
	byWidth := float64(area.Width) / float64(minimumWindowWidth)
	byHeight := float64(area.Height) / float64(minimumWindowHeight)
	// 在 float64 里算并加一点容差：1280/1000 在 float32 里是 1.2799999…，
	// 直接向下取整会白白少 1%。
	return float32(math.Floor(min(byWidth, byHeight)*100+1e-9) / 100)
}

// windowSizeForZoom 返回缩放变化后窗口应该采用的尺寸：某一维比新的最小尺寸小
// 就长到刚好放得下，已经更大的一维不动（缩放不缩窗）；长出来的部分不超过屏幕
// 可用区域。
func windowSizeForZoom(width, height int, zoom float32, area mygo.Rectangle) (int, int) {
	minWidth, minHeight := zoomedWindowMinimum(zoom)
	nextWidth, nextHeight := width, height
	if nextWidth < minWidth {
		nextWidth = minWidth
		if area.Width > 0 {
			nextWidth = min(nextWidth, area.Width)
		}
	}
	if nextHeight < minHeight {
		nextHeight = minHeight
		if area.Height > 0 {
			nextHeight = min(nextHeight, area.Height)
		}
	}
	return nextWidth, nextHeight
}
