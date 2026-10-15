package main

import (
	"bytes"
	"image/png"
	"testing"
)

// hasTransparency 报告 PNG 里是否有半透明或全透明的像素。
func hasTransparency(t *testing.T, name string, data []byte) bool {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码 %s 失败: %v", name, err)
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
				return true
			}
		}
	}
	return false
}

// TestIconAssetsCarryTransparency 检查两张图标都带透明区域。
//
// 应用图标用圆角卡片做透明边角：Windows 的图标资源只认 32bpp 的 PNG 负载，
// 全不透明的图会被打包成 24bpp，托盘图标就创建不出来。模板托盘图标同理，
// macOS 只取它的 alpha 通道，全不透明会被涂成一整块实心方块。
func TestIconAssetsCarryTransparency(t *testing.T) {
	for _, asset := range []struct {
		name string
		data []byte
	}{
		{"应用图标 resources/icon.png", embeddedTrayIcon},
		{"模板托盘图标 resources/tray-icon.png", embeddedTrayTemplateIcon},
	} {
		if !hasTransparency(t, asset.name, asset.data) {
			t.Errorf("%s 没有透明像素", asset.name)
		}
	}
}
