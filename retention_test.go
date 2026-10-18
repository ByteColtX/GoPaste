package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"gopaste/internal/history"
	"gopaste/internal/settings"

	"github.com/egoist/mygo/ui"
)

// retentionItems returns a history spanning a few minutes, newest first once
// the store has ordered it.
func retentionItems(now time.Time) []history.Item {
	return []history.Item{
		{ID: "newest", CreatedAt: now},
		{ID: "middle", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "oldest", CreatedAt: now.Add(-2 * time.Minute)},
	}
}

// TestInstallHistoryAppliesRetention 检查装载完成后立刻裁一次：升级后第一次
// 启动，超过上限的旧记录在这一刻消失，删除也排进了写队列。
func TestInstallHistoryAppliesRetention(t *testing.T) {
	repository := &recordingRepository{}
	persistence := newHistoryPersistence(repository)
	app := newApplication()
	app.retention = history.RetentionPolicy{MaxItems: 2}

	app.installHistory(retentionItems(time.Now()), persistence)
	persistence.Close()

	if got, want := app.history.Snapshot(history.Query{}).Total, 2; got != want {
		t.Errorf("装载后条数 = %d，期望 %d", got, want)
	}
	if _, ok := app.history.Item("oldest"); ok {
		t.Error("超过上限的记录装载后应当已经删除")
	}
	want := []string{"delete:oldest", "close"}
	if got := repository.recorded(); !slices.Equal(got, want) {
		t.Errorf("落盘操作 = %v，期望 %v", got, want)
	}
}

// TestAppendCapturedPrunesOldRecords 检查每来一条新内容就裁一次：条数上限靠它
// 维持，被挤掉的记录同时排进删除队列。
func TestAppendCapturedPrunesOldRecords(t *testing.T) {
	repository := &recordingRepository{}
	persistence := newHistoryPersistence(repository)
	app := newApplication()
	app.retention = history.RetentionPolicy{MaxItems: 2}
	app.installHistory(retentionItems(time.Now()), persistence)

	app.appendCaptured(history.Item{ID: "fresh", CreatedAt: time.Now().Add(time.Minute)})
	persistence.Close()

	view := app.history.Snapshot(history.Query{})
	ids := make([]string, len(view.Items))
	for i, item := range view.Items {
		ids[i] = item.ID
	}
	if want := []string{"fresh", "newest"}; !slices.Equal(ids, want) {
		t.Fatalf("剩余记录 = %v，期望 %v", ids, want)
	}
	want := []string{"delete:oldest", "put:fresh", "delete:middle", "close"}
	if got := repository.recorded(); !slices.Equal(got, want) {
		t.Errorf("落盘操作 = %v，期望 %v", got, want)
	}
}

// TestRetentionSettingsOnlySavedWhenChanged 检查默认值不写进设置文件，改过之后
// 只写改过的那一维：另一维的默认值因此可以跟着程序演进，而不是被冻住。
func TestRetentionSettingsOnlySavedWhenChanged(t *testing.T) {
	app := newApplication()
	if got := app.retentionSettings(); got != nil {
		t.Fatalf("没改过的保留策略 = %+v，期望 nil", got)
	}

	app.applyRetentionSettings(&settings.Retention{MaxItems: retentionValue(500)})
	if got, want := app.retention, (history.RetentionPolicy{MaxItems: 500, MaxDays: defaultRetentionDays}); got != want {
		t.Fatalf("读回的保留策略 = %+v，期望 %+v", got, want)
	}
	saved := app.retentionSettings()
	if saved == nil || saved.MaxItems == nil || *saved.MaxItems != 500 {
		t.Fatalf("要写进文件的条数上限 = %+v，期望 500", saved)
	}
	if saved.MaxDays != nil {
		t.Fatalf("没改过的天数上限不应写进文件: %d", *saved.MaxDays)
	}
}

// TestApplyRetentionSettingsKeepsDefaultsWhenAbsent 检查旧版本写的设置文件（没有
// retention 字段）读进来后仍然是默认上限，而不是被当成「不限」。
func TestApplyRetentionSettingsKeepsDefaultsWhenAbsent(t *testing.T) {
	app := newApplication()
	app.applyRetentionSettings(nil)

	if got, want := app.retention, defaultRetention(); got != want {
		t.Fatalf("缺字段时的保留策略 = %+v，期望默认 %+v", got, want)
	}
	if got := defaultRetention(); got.MaxItems != 10000 || got.MaxDays != 90 {
		t.Fatalf("默认保留策略 = %+v，期望一万条 / 九十天", got)
	}
}

// TestStorageSettingsShowsRetentionLimits 检查「存储」页真的画出两个上限下拉与
// 「立即删除且不可撤销」的说明。测试器在 key 重复时会 panic，所以这也是一次
// 新控件 key 的冒烟检查。
func TestStorageSettingsShowsRetentionLimits(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.settingsArea = settingsStorage

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	for _, want := range []string{
		app.t(msgSettingsRetentionItemsRow),
		app.t(msgSettingsRetentionDaysRow),
		app.l10n().retentionItemLabel(defaultRetentionItems),
		app.l10n().retentionDayLabel(defaultRetentionDays),
		app.t(msgSettingsRetentionNote),
	} {
		if !tt.HasText(want) {
			t.Errorf("存储页应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}
}

// retentionRecords builds n records a second apart, so the oldest is a known one.
func retentionRecords(n int, now time.Time) []history.Item {
	items := make([]history.Item, n)
	for i := range items {
		items[i] = history.Item{
			ID:        fmt.Sprintf("item-%03d", i),
			CreatedAt: now.Add(-time.Duration(i) * time.Second),
		}
	}
	return items
}

// TestLoweringTheRetentionLimitPrunesImmediately 检查在设置页把上限调低的那一刻
// 就真的删掉超额记录，并把选择写进设置文件——不弹二次确认，也不等到下次复制。
func TestLoweringTheRetentionLimitPrunesImmediately(t *testing.T) {
	repository := &recordingRepository{}
	persistence := newHistoryPersistence(repository)
	app := newApplication()
	app.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	app.installHistory(retentionRecords(501, time.Now()), persistence)
	app.setPage(pageSettings)
	app.settingsArea = settingsStorage

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	if err := tt.Click(app.l10n().retentionItemLabel(defaultRetentionItems)); err != nil {
		t.Fatalf("点开「最多保留」下拉失败：%v", err)
	}
	tt.Frame()
	if err := tt.Click(app.l10n().retentionItemLabel(500)); err != nil {
		t.Fatalf("选择 500 条失败：%v", err)
	}
	tt.Frame()
	if tt.HasText(app.l10n().storageRecord(501)) {
		t.Errorf("调低上限后存储页仍显示旧条数，实际显示：%q", tt.Texts())
	}
	persistence.Close()

	if got := app.retention.MaxItems; got != 500 {
		t.Fatalf("保留条数 = %d，期望 500", got)
	}
	if got := app.history.Snapshot(history.Query{}).Total; got != 500 {
		t.Errorf("调低上限后剩余 %d 条，期望立刻裁到 500 条", got)
	}
	if _, ok := app.history.Item("item-500"); ok {
		t.Error("最旧的一条应当已经被删掉")
	}
	if got, want := repository.recorded(), []string{"delete:item-500", "close"}; !slices.Equal(got, want) {
		t.Errorf("落盘操作 = %v，期望 %v", got, want)
	}

	saved, err := settings.Load(app.settingsPath)
	if err != nil {
		t.Fatalf("读取设置文件失败：%v", err)
	}
	if saved.Retention == nil || saved.Retention.MaxItems == nil || *saved.Retention.MaxItems != 500 {
		t.Errorf("设置文件里的条数上限 = %+v，期望 500", saved.Retention)
	}
	if saved.Retention.MaxDays != nil {
		t.Errorf("没改过的天数上限不应写进文件: %d", *saved.Retention.MaxDays)
	}
}

// TestChoosingNoLimitStopsPruning 检查「不限」真的是关掉该维度：选完之后新内容
// 进来不再删旧的。
func TestChoosingNoLimitStopsPruning(t *testing.T) {
	app := newApplication()
	app.retention.MaxItems = 0
	persistence := newHistoryPersistence(&recordingRepository{})
	app.installHistory(retentionRecords(3, time.Now()), persistence)

	app.appendCaptured(history.Item{ID: "fresh", CreatedAt: time.Now().Add(time.Minute)})
	persistence.Close()

	if got := app.history.Snapshot(history.Query{}).Total; got != 4 {
		t.Fatalf("「不限」下剩余 %d 条，期望 4 条全部保留", got)
	}
}
