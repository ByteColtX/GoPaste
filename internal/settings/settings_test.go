package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadMissingFileIsAFirstRun 检查没有 settings.json 时不报错：第一次启动就是
// 这个状态，调用方据此显示向导，而不是把缺失当成故障。
func TestLoadMissingFileIsAFirstRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GoPaste", "settings.json")
	file, err := Load(path)
	if err != nil {
		t.Fatalf("文件不存在时 Load 出错: %v", err)
	}
	if file.Onboarding.Completed || file.DeviceName != "" {
		t.Errorf("文件不存在时应返回零值，实际 %+v", file)
	}
}

// TestSaveLoadRoundTrip 检查写入再读回来是同一份设置：完成标记与改过的设备名
// 都要原样回来。
func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := File{
		Onboarding: Onboarding{Completed: true},
		DeviceName: "bytecolt-mbp",
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save 出错: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if !got.Onboarding.Completed {
		t.Error("完成标记没有读回来")
	}
	if got.DeviceName != want.DeviceName {
		t.Errorf("设备名 = %q，期望 %q", got.DeviceName, want.DeviceName)
	}
}

// TestSaveOmitsEmptyFields 检查只完成向导、什么都没改时文件里不出现空字段：
// 默认设备名不落盘，配对请求也不该以 null 的形式留下。
func TestSaveOmitsEmptyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, File{Onboarding: Onboarding{Completed: true}}); err != nil {
		t.Fatalf("Save 出错: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取写入的文件出错: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("写入的文件不是合法 JSON: %v", err)
	}
	if _, ok := fields["deviceName"]; ok {
		t.Errorf("没改过的设备名不应写进文件: %s", data)
	}
	// 没动过的保留策略不落盘：默认值将来变了，没选过的人应当跟着变。
	if _, ok := fields["retention"]; ok {
		t.Errorf("没改过的保留策略不应写进文件: %s", data)
	}
}

// TestLoadLeavesRetentionUnset 检查旧版本写的文件读回来是 nil：调用方据此套用
// 自己的默认上限，而不是把「缺字段」读成「不限」。
func TestLoadLeavesRetentionUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"onboarding":{"completed":true}}`), 0o600); err != nil {
		t.Fatalf("写入旧版本设置出错: %v", err)
	}
	file, err := Load(path)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if file.Retention != nil {
		t.Errorf("缺 retention 字段时应读成 nil，实际 %+v", file.Retention)
	}
}

// TestRetentionRoundTrip 检查两个上限都能写进文件再原样读回来。
func TestRetentionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := &Retention{MaxItems: retentionInt(500), MaxDays: retentionInt(30)}
	if err := Save(path, File{Onboarding: Onboarding{Completed: true}, Retention: want}); err != nil {
		t.Fatalf("Save 出错: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if got.Retention == nil {
		t.Fatal("保存过的保留策略读回来是 nil")
	}
	if got.Retention.MaxItems == nil || *got.Retention.MaxItems != 500 {
		t.Errorf("条数上限 = %v，期望 500", got.Retention.MaxItems)
	}
	if got.Retention.MaxDays == nil || *got.Retention.MaxDays != 30 {
		t.Errorf("天数上限 = %v，期望 30", got.Retention.MaxDays)
	}
}

// TestRetentionSavesOnlyTheChosenDimension 检查只改过一项时另一项不被写进文件：
// 没选过的维度应当继续跟着程序的默认值走，而不是被冻在这一刻。
func TestRetentionSavesOnlyTheChosenDimension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, File{Retention: &Retention{MaxItems: retentionInt(500)}}); err != nil {
		t.Fatalf("Save 出错: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if got.Retention == nil || got.Retention.MaxItems == nil || *got.Retention.MaxItems != 500 {
		t.Fatalf("条数上限 = %+v，期望 500", got.Retention)
	}
	if got.Retention.MaxDays != nil {
		t.Errorf("没改过的天数上限不应写进文件: %v", *got.Retention.MaxDays)
	}
}

// TestRetentionZeroSurvivesTheRoundTrip 检查「不限」是明确的 0，而不是被省略掉的
// 空值：这是 Retention 用指针而不是两个 int 的全部理由。
func TestRetentionZeroSurvivesTheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(path, File{Retention: &Retention{MaxItems: retentionInt(0)}}); err != nil {
		t.Fatalf("Save 出错: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取写入的文件出错: %v", err)
	}
	if !strings.Contains(string(data), `"maxItems": 0`) {
		t.Fatalf("选过「不限」时 maxItems 不应被省略: %s", data)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if got.Retention == nil || got.Retention.MaxItems == nil {
		t.Fatal("选过「不限」的条数上限读回来是 nil，会被当成没设过")
	}
	if *got.Retention.MaxItems != 0 {
		t.Errorf("条数上限 = %d，期望 0（不限）", *got.Retention.MaxItems)
	}
}

// retentionInt 返回 v 的地址，用来构造设置文件里的「设过的」上限。
func retentionInt(v int) *int { return &v }

// TestSaveCreatesAPrivateFile 检查目录和文件的权限：设置文件里有配对码，只有当前
// 用户能读。同时确认写入用过的临时文件没有留在目录里。
func TestSaveCreatesAPrivateFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "GoPaste")
	path := filepath.Join(dir, "settings.json")
	if err := Save(path, File{Onboarding: Onboarding{Completed: true}}); err != nil {
		t.Fatalf("Save 出错: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("settings.json 不存在: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("文件权限 = %o，期望 600", perm)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("设置目录不存在: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("目录权限 = %o，期望 700", perm)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取设置目录出错: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != "settings.json" {
			t.Errorf("目录里留下了临时文件 %q", entry.Name())
		}
	}
}

// TestSaveReplacesAnExistingFile 检查第二次保存会整体替换旧内容：向导只写一次，
// 但设置文件的写入必须是替换而不是追加。
func TestSaveReplacesAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	first := File{
		Onboarding: Onboarding{Completed: true},
		DeviceName: "old-name",
	}
	if err := Save(path, first); err != nil {
		t.Fatalf("第一次 Save 出错: %v", err)
	}
	second := File{Onboarding: Onboarding{Completed: true}, DeviceName: "renamed"}
	if err := Save(path, second); err != nil {
		t.Fatalf("第二次 Save 出错: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load 出错: %v", err)
	}
	if got.DeviceName != "renamed" {
		t.Errorf("设备名 = %q，期望 renamed", got.DeviceName)
	}
}

// TestLoadRejectsADamagedFile 检查坏文件会报错而不是悄悄当成首次运行：调用方要
// 能区分「没有设置」和「设置有损坏」。
func TestLoadRejectsADamagedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatalf("写入坏文件出错: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Error("坏文件应当返回错误")
	}
}
