//go:build preview

package main

import "testing"

// TestPreviewBuildNeverCaptures 守住预览构建的约定：设备 fixture 在，剪贴板监听
// 关。走查用的历史只来自 scripts/mockdb 生成的数据库，任何真实复制都不该被写进
// 那个文件。
func TestPreviewBuildNeverCaptures(t *testing.T) {
	if clipboardCaptureEnabled() {
		t.Error("预览构建不应监听剪贴板")
	}
	if devices := previewDevices(); len(devices) == 0 {
		t.Error("预览构建应带出设备 fixture，否则设备页是空的")
	}
}
