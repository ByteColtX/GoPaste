package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestLoadMissingFileIsAFirstRun 检查没有 settings.json 时不报错：第一次启动就是
// 这个状态，调用方据此显示向导，而不是把缺失当成故障。
func TestLoadMissingFileIsAFirstRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GoPaste", "settings.json")
	file, err := Load(path)
	if err != nil {
		t.Fatalf("文件不存在时 Load 出错: %v", err)
	}
	if file.Onboarding.Completed || file.DeviceName != "" || file.PendingPairing != nil {
		t.Errorf("文件不存在时应返回零值，实际 %+v", file)
	}
}

// TestSaveLoadRoundTrip 检查写入再读回来是同一份设置：完成标记、改过的设备名和
// 待配对请求都要原样回来，待配对请求的时间按 UTC 相等比较。
func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	created := time.Date(2026, 2, 11, 10, 0, 0, 0, time.UTC)
	want := File{
		Onboarding:     Onboarding{Completed: true},
		DeviceName:     "bytecolt-mbp",
		PendingPairing: &PendingPairing{Code: "482913", CreatedAt: created},
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
	if got.PendingPairing == nil {
		t.Fatal("待配对请求没有读回来")
	}
	if got.PendingPairing.Code != want.PendingPairing.Code {
		t.Errorf("配对码 = %q，期望 %q", got.PendingPairing.Code, want.PendingPairing.Code)
	}
	if !got.PendingPairing.CreatedAt.Equal(created) {
		t.Errorf("记录时间 = %v，期望 %v", got.PendingPairing.CreatedAt, created)
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
	if _, ok := fields["pendingPairing"]; ok {
		t.Errorf("没有待配对请求时不应写这个字段: %s", data)
	}
}

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
		Onboarding:     Onboarding{Completed: true},
		PendingPairing: &PendingPairing{Code: "482913", CreatedAt: time.Now().UTC()},
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
	if got.PendingPairing != nil {
		t.Errorf("旧内容没有被替换掉: %+v", got)
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

// TestPairingCode 检查配对码的规范化与校验：空格和连字符是粘贴时带进来的，
// 其余字符和长度不对都要拒绝。
func TestPairingCode(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{"六位数字", "482913", "482913", true},
		{"分组输入", "482 913", "482913", true},
		{"带连字符", "482-913", "482913", true},
		{"前后空格", "  482913  ", "482913", true},
		{"太短", "48291", "", false},
		{"太长", "4829134", "", false},
		{"含字母", "48291a", "", false},
		{"全角数字", "４８２９１３", "", false},
		{"空串", "", "", false},
		{"只有空格", "      ", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := PairingCode(tc.raw)
			if ok != tc.ok {
				t.Fatalf("PairingCode(%q) ok = %v，期望 %v", tc.raw, ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("PairingCode(%q) = %q，期望 %q", tc.raw, got, tc.want)
			}
		})
	}
}
