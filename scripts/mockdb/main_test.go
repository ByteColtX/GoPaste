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
		history.KindText:     16,
		history.KindLink:     15,
		history.KindCode:     34,
		history.KindImage:    15,
		history.KindFile:     12,
		history.KindRichText: 8,
	}
	if len(entries) != 100 {
		t.Errorf("记录 = %d 条，期望 100 条", len(entries))
	}
	if !maps.Equal(kinds, wantKinds) {
		t.Errorf("各类型条数 = %v，期望 %v", kinds, wantKinds)
	}
	if favorites != 21 {
		t.Errorf("收藏 = %d 条，期望 21 条", favorites)
	}
	if local != 61 {
		t.Errorf("本机记录 = %d 条，期望 61 条", local)
	}
	if deliveries != 480 {
		t.Errorf("投递记录 = %d 条，期望 480 条", deliveries)
	}
}

// TestFolderCopyOutgrowsThePathRowLimit 检查「复制整个文件夹」那条记录确实超过
// 预览区一次列出的路径数，否则末尾的「还有 N 个」汇总行永远走不到。
//
// 上限是 view.go 里的 filePathRowsLimit；两个 main 包之间引不到这个常量，所以
// 这里写死并注明来源，改那边的时候记得一起改。
func TestFolderCopyOutgrowsThePathRowLimit(t *testing.T) {
	const pathRowLimit = 20

	paths := strings.Split(folderCopyPaths(), "\n")
	if len(paths) <= pathRowLimit {
		t.Errorf("文件夹复制的路径 = %d 条，必须超过预览上限 %d 条", len(paths), pathRowLimit)
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
