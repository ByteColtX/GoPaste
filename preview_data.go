//go:build !preview

package main

import "gopaste/internal/history"

// previewData 在常规构建中返回真实数据源接入前的空状态。
func previewData() ([]history.Item, []pairedDevice) {
	return nil, nil
}
