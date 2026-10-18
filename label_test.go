package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// labelWidth 在一个只放固定文案的视图里量出这段文字占到的宽度。
func labelWidth(t *testing.T, view func(c *ui.Context)) float32 {
	t.Helper()
	var width float32
	tt := ui.NewTester(view, 200, 60)
	tt.Frame()
	if r, ok := tt.Find("收藏"); ok {
		width = r.W
	}
	if width == 0 {
		t.Fatal("视图里没有量到固定文案")
	}
	return width
}

// TestLabelTextKeepsItsNaturalWidth 检查固定文案放进更窄的盒子后仍占自身宽度。
// 它靠 NoWrap 排版：按内容定宽的盒子宽度恰好等于文字宽度，Windows 的 DirectWrite
// 在宽度差一个浮点最低位时就断行，中文每两个字之间都是断行点，「收藏」于是显示成
// 「收…」。压窄盒子后宽度不变，说明断行判断不会因为几个 ULP 的不足而触发。
func TestLabelTextKeepsItsNaturalWidth(t *testing.T) {
	const size = 13.3
	natural := labelWidth(t, func(c *ui.Context) {
		ui.Row(c).Children(func() { labelText(c, "收藏").FontSize(size) })
	})
	squeezed := labelWidth(t, func(c *ui.Context) {
		ui.Row(c).Width(10).Children(func() { labelText(c, "收藏").FontSize(size) })
	})
	if squeezed < natural {
		t.Errorf("固定文案被压窄：自然宽度 %.4f，盒子只有 10 DIP 时 %.4f", natural, squeezed)
	}
}
