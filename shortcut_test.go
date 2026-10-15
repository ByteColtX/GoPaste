package main

import (
	"testing"

	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

// historyIDs 返回当前历史视图里记录的 ID，顺序与列表一致。
func historyIDs(app *application) []string {
	items := app.history.Snapshot(app.historyQuery()).Items
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// countText 报告一组文本里恰好等于 want 的个数。
func countText(texts []string, want string) int {
	count := 0
	for _, text := range texts {
		if text == want {
			count++
		}
	}
	return count
}

// TestShortcutKeyLabel 检查键帽文案：字母与数字取键面字符，Escape 取自己的名字。
func TestShortcutKeyLabel(t *testing.T) {
	cases := []struct {
		key  ui.Key
		want string
	}{
		{ui.KeyD, "D"},
		{ui.KeyY, "Y"},
		{ui.KeyZ, "Z"},
		{ui.Key0, "0"},
		{ui.Key9, "9"},
		{ui.KeyEscape, "Esc"},
		{ui.KeyEnter, ""},
	}
	for _, tc := range cases {
		if got := shortcutKeyLabel(tc.key); got != tc.want {
			t.Errorf("shortcutKeyLabel(%v) = %q，期望 %q", tc.key, got, tc.want)
		}
	}
}

// TestDeleteConfirmationShowsShortcutHints 检查二次确认框把可用按键画在按钮上：
// 确认按钮显示 D 与 Y，取消按钮显示 Esc。预览操作条上本来就有个 D 键帽，所以
// D 在打开对话框后应从一个变成两个。
func TestDeleteConfirmationShowsShortcutHints(t *testing.T) {
	app := previewApp("")
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	if got := countText(tt.Texts(), "D"); got != 1 {
		t.Fatalf("预览操作条应有一个 D 键帽，实际 %d 个", got)
	}

	tt.Key(0, ui.KeyD)
	if !app.confirmDangerousAction {
		t.Fatalf("按 D 后应打开删除确认框，实际显示：%q", tt.Texts())
	}

	texts := tt.Texts()
	if got := countText(texts, "D"); got != 2 {
		t.Errorf("确认框应再显示一个 D 键帽，实际共 %d 个", got)
	}
	if got := countText(texts, "Y"); got != 1 {
		t.Errorf("确认框应显示一个 Y 键帽，实际 %d 个", got)
	}
	if got := countText(texts, "Esc"); got != 1 {
		t.Errorf("确认框应显示一个 Esc 键帽，实际 %d 个", got)
	}
}

// TestDeleteConfirmationAcceptsConfirmKeys 检查删除二次确认框接受 D 与 Y：两者
// 都应执行删除并关闭对话框，而不是只让确认按钮可点。
func TestDeleteConfirmationAcceptsConfirmKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  ui.Key
	}{
		{"D 确认", ui.KeyD},
		{"Y 确认", ui.KeyY},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := previewApp("")
			tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

			// 预览操作条上的 D 先打开二次确认框。
			tt.Key(0, ui.KeyD)
			if !app.confirmDangerousAction {
				t.Fatalf("按 D 后应打开删除确认框，实际显示：%q", tt.Texts())
			}

			tt.Key(0, tc.key)
			if app.confirmDangerousAction {
				t.Error("确认后确认框应关闭")
			}
			if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 0 {
				t.Errorf("确认后历史应为空，实际 %d 条", got)
			}
		})
	}
}

// TestDeleteConfirmationEscapeCancels 检查 Escape 只关闭确认框，不删除记录。
func TestDeleteConfirmationEscapeCancels(t *testing.T) {
	app := previewApp("")
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	tt.Key(0, ui.KeyD)
	if !app.confirmDangerousAction {
		t.Fatal("按 D 后应打开删除确认框")
	}

	tt.Key(0, ui.KeyEscape)
	if app.confirmDangerousAction {
		t.Error("Escape 应关闭确认框")
	}
	if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 1 {
		t.Errorf("取消后应保留 1 条记录，实际 %d 条", got)
	}
}

// TestUndoShortcutRestoresDeletedItem 检查 Cmd+Z 与提示里的「撤销」等价：恢复
// 刚删除的记录，而且机会只有一次，再按不会重复恢复。
func TestUndoShortcutRestoresDeletedItem(t *testing.T) {
	app := previewApp("")
	// 模拟用户已经勾选「不再询问」，直接删除，省去确认框。
	app.suppressDeleteConfirmation = true

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Key(0, ui.KeyD)
	if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 0 {
		t.Fatalf("按 D 后应删除记录，实际 %d 条", got)
	}

	tt.Key(ui.Cmd, ui.KeyZ)
	if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 1 {
		t.Fatalf("Cmd+Z 应恢复记录，实际 %d 条", got)
	}

	tt.Key(ui.Cmd, ui.KeyZ)
	if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 1 {
		t.Errorf("第二次 Cmd+Z 不应再改历史，实际 %d 条", got)
	}
	if len(app.pendingUndos) != 0 {
		t.Errorf("撤销后栈应为空，实际 %d 条", len(app.pendingUndos))
	}
}

// TestUndoShortcutUndoesTheMostRecentDeleteFirst 检查撤销栈是后进先出：连续删除
// 两条记录后，Cmd+Z 先恢复最近删除的那条。
func TestUndoShortcutUndoesTheMostRecentDeleteFirst(t *testing.T) {
	older := previewItem("")
	older.ID = "older"
	newer := previewItem("")
	newer.ID = "newer"

	app := newApplication()
	app.history = history.New([]history.Item{older, newer})
	app.pairedDevices = nil
	app.suppressDeleteConfirmation = true

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	app.history.Select("newer")
	tt.Frame()
	tt.Key(0, ui.KeyD)

	app.history.Select("older")
	tt.Frame()
	tt.Key(0, ui.KeyD)

	if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 0 {
		t.Fatalf("两次删除后历史应为空，实际 %d 条", got)
	}

	tt.Key(ui.Cmd, ui.KeyZ)
	if ids := historyIDs(app); len(ids) != 1 || ids[0] != "older" {
		t.Fatalf("第一次 Cmd+Z 应恢复最近删除的 older，实际 %v", ids)
	}

	tt.Key(ui.Cmd, ui.KeyZ)
	if ids := historyIDs(app); len(ids) != 2 {
		t.Errorf("第二次 Cmd+Z 应恢复 newer，实际 %v", ids)
	}
}

// TestUndoMenuRequestRestoresDeletedItem 检查 macOS 菜单项走的排队路径：菜单回调
// 拿不到构建帧的 ui.Context，只能排队，由下一帧消费后恢复记录。
func TestUndoMenuRequestRestoresDeletedItem(t *testing.T) {
	app := previewApp("")
	app.suppressDeleteConfirmation = true

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Key(0, ui.KeyD)
	if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 0 {
		t.Fatalf("按 D 后应删除记录，实际 %d 条", got)
	}

	app.requestUndo()
	tt.Frame()
	if got := len(app.history.Snapshot(app.historyQuery()).Items); got != 1 {
		t.Errorf("菜单请求撤销后应恢复记录，实际 %d 条", got)
	}
}

// TestSidebarToggleShortcut 检查 Cmd+Ctrl+S 折叠与展开筛选栏。
func TestSidebarToggleShortcut(t *testing.T) {
	app := previewApp("")
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	if !app.filterRailOpen {
		t.Fatal("默认应展开侧栏")
	}
	tt.Key(ui.Cmd|ui.Ctrl, ui.KeyS)
	if app.filterRailOpen {
		t.Error("Cmd+Ctrl+S 应收起侧栏")
	}
	tt.Key(ui.Cmd|ui.Ctrl, ui.KeyS)
	if !app.filterRailOpen {
		t.Error("再按一次应展开侧栏")
	}
}

// TestDeviceShortcuts 检查设备页的 Cmd+N 新增设备与 Cmd+R 刷新。
func TestDeviceShortcuts(t *testing.T) {
	app := previewApp("")
	app.setPage(pageDevices)
	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	tt.Key(ui.Cmd, ui.KeyN)
	if !app.addDeviceDialog {
		t.Fatalf("Cmd+N 应打开新增设备对话框，实际显示：%q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	if app.addDeviceDialog {
		t.Error("Escape 应关闭新增设备对话框")
	}

	tt.Key(ui.Cmd, ui.KeyR)
	if !app.deviceRefreshSpinning() {
		t.Error("Cmd+R 应开始刷新设备列表")
	}
}
