package main

import (
	"testing"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

// filterApp 构造一个含各类内容的历史库：文本、链接、代码、富文本、图像、文件，
// 每一种都能被筛选栏单独选中，用来验证类型与标签不会互相打架。
func filterApp() *application {
	app := newApplication()
	app.history = history.New([]history.Item{
		{ID: "text", Kind: kindText, Text: "普通文本"},
		{ID: "link", Kind: kindLink, Text: "https://example.com"},
		{ID: "code", Kind: kindCode, Text: "package main"},
		{ID: "rich", Kind: kindRichText, Text: "富文本"},
		{ID: "image", Kind: kindImage, Text: "一张图"},
		{ID: "file", Kind: kindFile, Text: "~/Documents/report.pdf"},
	})
	app.pairedDevices = nil
	return app
}

// filteredItemIDs 返回当前筛选结果的 ID，按列表顺序。
func filteredItemIDs(app *application) []string {
	view := app.history.Snapshot(app.historyQuery())
	ids := make([]string, 0, len(view.Items))
	for _, item := range view.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

// TestTagFacetKeepsTheKindConsistent 复现筛选栏的 0 结果问题：先选中图像，再点
// 标签里的链接。链接的载荷是文本，两个条件同时生效时列表一条都不剩。
func TestTagFacetKeepsTheKindConsistent(t *testing.T) {
	app := filterApp()
	tt := ui.NewTester(app.view, defaultWindowWidth, 900)

	if err := tt.Click(app.t(msgKindImage)); err != nil {
		t.Fatalf("点击「%s」失败：%v", app.t(msgKindImage), err)
	}
	if app.filterKind != kindFilterImage {
		t.Fatalf("点「%s」后类型 = %v，期望 %v", app.t(msgKindImage), app.filterKind, kindFilterImage)
	}

	if err := tt.Click(app.t(msgKindLink)); err != nil {
		t.Fatalf("点击「%s」失败：%v", app.t(msgKindLink), err)
	}
	if app.filterTag != tagFilterLink {
		t.Fatalf("点「%s」后标签 = %v，期望 %v", app.t(msgKindLink), app.filterTag, tagFilterLink)
	}
	if app.filterKind != kindFilterText {
		t.Errorf("选链接后类型 = %v，期望收窄到 %v", app.filterKind, kindFilterText)
	}
	if ids := filteredItemIDs(app); len(ids) != 1 || ids[0] != "link" {
		t.Errorf("图像后选链接应只剩链接，实际 %v", ids)
	}
}

// TestKindFacetClearsAContradictingTag 检查反过来的一半：选中链接后再选图像，
// 标签要让位，而不是把列表筛成空。
func TestKindFacetClearsAContradictingTag(t *testing.T) {
	app := filterApp()
	tt := ui.NewTester(app.view, defaultWindowWidth, 900)

	if err := tt.Click(app.t(msgKindLink)); err != nil {
		t.Fatalf("点击「%s」失败：%v", app.t(msgKindLink), err)
	}
	if app.filterTag != tagFilterLink || app.filterKind != kindFilterText {
		t.Fatalf("点「%s」后应选中链接与文本，实际标签 %v 类型 %v", app.t(msgKindLink), app.filterTag, app.filterKind)
	}

	if err := tt.Click(app.t(msgKindImage)); err != nil {
		t.Fatalf("点击「%s」失败：%v", app.t(msgKindImage), err)
	}
	if app.filterTag != tagFilterAny {
		t.Errorf("选图像后标签 = %v，期望清空", app.filterTag)
	}
	if ids := filteredItemIDs(app); len(ids) != 1 || ids[0] != "image" {
		t.Errorf("链接后选图像应只剩图像，实际 %v", ids)
	}
}

// TestKindAndTagFiltersNeverContradict 遍历类型与标签的每一种组合，确认状态永远
// 落在同一种载荷上，列表不会因为两个条件互相排斥而变空。
func TestKindAndTagFiltersNeverContradict(t *testing.T) {
	kinds := []historyKindFilter{kindFilterAny, kindFilterText, kindFilterRichText, kindFilterImage, kindFilterFile}
	tags := []historyTagFilter{tagFilterAny, tagFilterLink, tagFilterCode}
	for _, kind := range kinds {
		for _, tag := range tags {
			app := filterApp()
			app.setKindFilter(kind)
			app.setTagFilter(tag)

			if want := tagKind(app.filterTag); want != kindFilterAny && want != app.filterKind {
				t.Errorf("类型 %v + 标签 %v 互相矛盾：标签属于 %v，类型却是 %v", kind, tag, want, app.filterKind)
			}
			if ids := filteredItemIDs(app); len(ids) == 0 {
				t.Errorf("类型 %v + 标签 %v 筛出了 0 条", kind, tag)
			}
		}
	}
}
