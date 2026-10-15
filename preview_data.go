//go:build !preview

package main

// previewDevices 在常规构建中返回空设备列表：设备发现还没有接入，
// 已配对设备因此暂时只来自本地预览数据。
func previewDevices() []pairedDevice {
	return nil
}
