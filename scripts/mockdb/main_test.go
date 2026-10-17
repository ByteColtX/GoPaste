package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopaste/internal/filetype"
	"gopaste/internal/history"
)

// TestFixtureFillsEveryFilter 检查预览数据填满了界面上的每个筛选维度：类型、标签、
// 来源（本机与每台配对设备）、时间窗口、收藏，以及每种同步状态。
//
// 这里不钉条数：fixture 是随手改的东西，钉死总数只会让每次调整都要改测试。要守住
// 的是「点开任何一个筛选都不会空」——那个状态才有东西可走查。
func TestFixtureFillsEveryFilter(t *testing.T) {
	entries := buildFixture()
	if err := validateFixture(entries); err != nil {
		t.Fatalf("预览记录不自洽: %v", err)
	}

	// 用真实的 history.Store 过滤，而不是在这里重算时间窗口：窗口语义归
	// internal/history 管，测试只关心结果非空。
	now := time.Now()
	items := make([]history.Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, history.Item{
			ID:             entry.id,
			Kind:           entry.kind,
			Text:           entry.text,
			SourceDeviceID: entry.source,
			CreatedAt:      resolveCreated(entry.created, now),
			Favorite:       entry.favorite,
			Sync:           entry.syncState,
		})
	}
	store := history.New(items)

	type facet struct {
		name  string
		query history.Query
	}
	facets := []facet{
		{"类型：文本", history.Query{Kind: history.KindFilterText}},
		{"类型：富文本", history.Query{Kind: history.KindFilterRichText}},
		{"类型：图片", history.Query{Kind: history.KindFilterImage}},
		{"类型：文件", history.Query{Kind: history.KindFilterFile}},
		{"标签：链接", history.Query{Tag: history.TagFilterLink}},
		{"标签：代码", history.Query{Tag: history.TagFilterCode}},
		{"标签：图片", history.Query{Tag: history.TagFilterImage}},
		{"来源：本机", history.Query{Source: history.LocalSource()}},
		{"时间：今天", history.Query{Period: history.PeriodFilterToday}},
		{"时间：昨天", history.Query{Period: history.PeriodFilterYesterday}},
		{"时间：最近 7 天", history.Query{Period: history.PeriodFilterLast7Days}},
		{"时间：最近 30 天", history.Query{Period: history.PeriodFilterLast30Days}},
		{"收藏", history.Query{FavoriteOnly: true}},
	}
	// 设备页的「只看这台设备」对每台配对设备都得有内容，否则那个筛选点了是空的。
	for _, deviceID := range previewDeviceIDs {
		facets = append(facets, facet{"来源：" + deviceID, history.Query{Source: history.DeviceSource(deviceID)}})
	}

	for _, f := range facets {
		if view := store.Snapshot(f.query); len(view.Items) == 0 {
			t.Errorf("%s 筛选是空的", f.name)
		}
	}

	// 同步状态不是筛选，但每种都要有记录，否则对应的徽标和投递说明看不到。
	states := map[history.SyncState]bool{}
	for _, entry := range entries {
		states[entry.syncState] = true
	}
	for _, want := range []history.SyncState{
		history.SyncStateNone,
		history.SyncStateSynced,
		history.SyncStatePartial,
		history.SyncStateOffline,
		history.SyncStateReplaced,
	} {
		if !states[want] {
			t.Errorf("预览数据里没有同步状态 %d 的记录", want)
		}
	}

	// 有同步状态的本机记录，投递记录必须覆盖每一台目标设备：界面按这个列表
	// 算「已同步 / 部分送达 / 待发送」，缺一条就会显示成没送达。
	for _, entry := range entries {
		if entry.source != "" || entry.syncState == history.SyncStateNone || entry.syncState == history.SyncStateReplaced {
			continue
		}
		if got := len(deliveriesFor(entry)); got != len(previewDeviceIDs) {
			t.Errorf("%s 的投递记录 = %d 条，期望 %d 条", entry.id, got, len(previewDeviceIDs))
		}
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
