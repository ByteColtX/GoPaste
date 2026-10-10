//go:build !preview

package main

// previewData 在常规构建中返回真实数据源接入前的空状态。
func previewData() ([]clipboardItem, []pairedDevice) {
	return nil, nil
}
