package main

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"gopaste/internal/filetype"
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
		history.KindFile:     46,
		history.KindRichText: 8,
	}
	if len(entries) != 134 {
		t.Errorf("记录 = %d 条，期望 134 条", len(entries))
	}
	if !maps.Equal(kinds, wantKinds) {
		t.Errorf("各类型条数 = %v，期望 %v", kinds, wantKinds)
	}
	if favorites != 21 {
		t.Errorf("收藏 = %d 条，期望 21 条", favorites)
	}
	if local != 82 {
		t.Errorf("本机记录 = %d 条，期望 82 条", local)
	}
	if deliveries != 624 {
		t.Errorf("投递记录 = %d 条，期望 624 条", deliveries)
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

// fileIconNames 是每个「有独立图标的文件格式」各挑一个代表文件名。
//
// 清单与 file_preview_test.go 的 TestFormatsHaveDistinctIcons 一一对应：那边保证
// 每个格式有自己的图标，这边保证预览数据里每个格式都有一条文件记录，否则图标表
// 里总有几枚在界面里永远看不到。改那边的时候记得一起改。
var fileIconNames = []string{
	"notes.txt",         // 文本
	"notes.md",          // Markdown
	"contract.docx",     // 文档
	"budget.numbers",    // 表格
	"report.xlsx",       // Excel
	"history.csv",       // CSV
	"roadmap.key",       // 演示文稿
	"manual.pdf",        // PDF
	"animation.gif",     // 图片
	"shot.png",          // PNG
	"photo.jpg",         // JPEG
	"legacy.bmp",        // BMP
	"icons.svg",         // SVG
	"recording.mov",     // 视频
	"memo.m4a",          // 音频
	"export.zip",        // 压缩包
	"Device.swift",      // 源码
	"store.go",          // Go
	"merge.py",          // Python
	"history.js",        // JavaScript
	"api.ts",            // TypeScript
	"HistoryList.vue",   // Vue
	"DeviceCard.svelte", // Svelte
	"main.rs",           // Rust
	"index.php",         // PHP
	"preview.sh",        // Shell
	"002-html.sql",      // SQL
	"index.html",        // HTML
	"app.css",           // CSS
	"theme.scss",        // Sass
	"info.plist",        // XML
	"tokens.json",       // JSON
	"build.yaml",        // YAML
	"mygo.toml",         // 配置
	"Dockerfile",        // Dockerfile
	"Inter.ttf",         // 字体
	"history.db",        // 数据库
	"icons.sketch",      // 设计稿
	"GoPaste.fig",       // Figma
	"model.blend",       // Blender
	"setup.exe",         // 可执行文件
}

// TestFixtureCoversEveryFileIcon 检查预览数据里的文件条目覆盖了每一个有独立图标的
// 格式。
func TestFixtureCoversEveryFileIcon(t *testing.T) {
	covered := map[filetype.Format]bool{}
	for _, entry := range buildFixture() {
		if entry.kind != history.KindFile {
			continue
		}
		first, _, _ := strings.Cut(entry.text, "\n")
		covered[filetype.Detect(first)] = true
	}
	for _, name := range fileIconNames {
		if !covered[filetype.Detect(name)] {
			t.Errorf("预览数据里没有 %s 这类格式的文件", name)
		}
	}
}
