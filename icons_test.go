package main

import (
	"bytes"
	"image/png"
	"testing"
)

// TestAppIconCarriesTransparency 检查应用图标带透明区域。
//
// 它同时是托盘图标，也是 Windows 图标资源的来源：那两种用法都只认 32bpp 的 PNG
// 负载，全不透明的图会被打包成 24bpp，托盘图标就创建不出来。
func TestAppIconCarriesTransparency(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(embeddedTrayIcon))
	if err != nil {
		t.Fatalf("解码应用图标失败: %v", err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
				return
			}
		}
	}
	t.Errorf("应用图标没有透明像素，Windows 的托盘图标会创建失败")
}
