package main

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"gopaste/internal/history"
)

// TestFixtureKeepsItsDocumentedShape 检查手写预览记录的形状：自洽性检查，加上各
// 类型、收藏、本机记录和投递记录的条数。
//
// 这些条数是界面走查的前提：每个类型筛选、来源筛选和时间筛选都要有内容。改动
// fixture 时它们会跟着一起被检查。
func TestFixtureKeepsItsDocumentedShape(t *testing.T) {
	entries := buildFixture()
	if err := validateFixture(entries); err != nil {
		t.Fatalf("预览记录不自洽: %v", err)
	}

	kinds := map[history.Kind]int{}
	favorites, local, deliveries := 0, 0, 0
	for _, entry := range entries {
		kinds[entry.kind]++
		if entry.favorite {
			favorites++
		}
		if entry.source == "" {
			local++
		}
		deliveries += len(deliveriesFor(entry))
	}

	wantKinds := map[history.Kind]int{
		history.KindText:     8,
		history.KindLink:     7,
		history.KindCode:     16,
		history.KindImage:    10,
		history.KindFile:     6,
		history.KindRichText: 4,
	}
	if len(entries) != 51 {
		t.Errorf("记录 = %d 条，期望 51 条", len(entries))
	}
	if !maps.Equal(kinds, wantKinds) {
		t.Errorf("各类型条数 = %v，期望 %v", kinds, wantKinds)
	}
	if favorites != 13 {
		t.Errorf("收藏 = %d 条，期望 13 条", favorites)
	}
	if local != 28 {
		t.Errorf("本机记录 = %d 条，期望 28 条", local)
	}
	if deliveries != 264 {
		t.Errorf("投递记录 = %d 条，期望 264 条", deliveries)
	}
}

// TestDefaultOutputIsNotAResource 检查默认输出不在 resources/ 下。
//
// mygo dev 递归监视 resources/：应用运行时对数据库的每一次写入都会让它重建并
// 重启，而 resources/ 里的东西也会被打进应用包。默认输出因此必须待在外面。
func TestDefaultOutputIsNotAResource(t *testing.T) {
	clean := filepath.Clean(defaultOutputPath)
	if clean == "resources" || strings.HasPrefix(clean, "resources"+string(filepath.Separator)) {
		t.Errorf("默认输出 %q 位于 resources/ 下，mygo dev 会因此反复重建", defaultOutputPath)
	}
}
