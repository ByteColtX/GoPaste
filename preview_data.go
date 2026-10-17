//go:build !preview

package main

// previewDevices 在常规构建中返回空设备列表：设备发现还没有接入，
// 已配对设备因此暂时只来自本地预览数据。
func previewDevices() []pairedDevice {
	return nil
}

// clipboardCaptureEnabled 报告本次构建是否监听系统剪贴板。常规构建监听；预览
// 构建不监听，理由见 preview_devices.go 里的同名函数。
func clipboardCaptureEnabled() bool {
	return true
}
