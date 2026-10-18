package main

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestDefaultDeviceNameUsesTheHostname 检查默认设备名直接取系统主机名，只有读
// 不到主机名时才退回「平台名-设备 ID 前几位」。
func TestDefaultDeviceNameUsesTheHostname(t *testing.T) {
	cases := []struct {
		name         string
		hostname     string
		platformSlug string
		peerID       string
		want         string
	}{
		{"macOS 主机名原样", "ByteColts-Appleseed.local", "macos", "", "ByteColts-Appleseed.local"},
		{"Windows 主机名原样", "ByteColts-ASUS", "windows", "", "ByteColts-ASUS"},
		{"主机名两侧空白去掉", "  ByteColts-ASUS\n", "windows", "", "ByteColts-ASUS"},
		{"主机名只有空白", "   ", "linux", "", "linux"},
		{"无主机名时用平台名和设备 ID", "", "windows", "abcdef1234", "windows-abcdef"},
		{"设备 ID 不足六位", "", "macos", "abc", "macos-abc"},
		{"无主机名也无设备 ID", "", "macos", "", "macos"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultDeviceName(tc.hostname, tc.platformSlug, tc.peerID); got != tc.want {
				t.Errorf("defaultDeviceName(%q, %q, %q) = %q，期望 %q",
					tc.hostname, tc.platformSlug, tc.peerID, got, tc.want)
			}
		})
	}
}

// TestNewApplicationDefaultsToTheHostname 检查启动时真的用上了主机名，而不是只
// 有一个没人调用的兜底函数。
func TestNewApplicationDefaultsToTheHostname(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		t.Skipf("本机读不到主机名，跳过：%v", err)
	}
	app := newApplication()
	if app.deviceName != hostname {
		t.Errorf("默认设备名 = %q，期望主机名 %q", app.deviceName, hostname)
	}
	if app.deviceNameDefault != app.deviceName {
		t.Errorf("记下的默认名 = %q，期望与设备名 %q 一致", app.deviceNameDefault, app.deviceName)
	}
}

// TestDeviceDisplayNameStripsDotLocalOnlyForTheDefault 检查设备页只在名字还是
// 默认主机名时去掉 .local 后缀；用户自己填的名字原样显示。
func TestDeviceDisplayNameStripsDotLocalOnlyForTheDefault(t *testing.T) {
	cases := []struct {
		name        string
		deviceName  string
		defaultName string
		want        string
	}{
		{"默认主机名去掉后缀", "ByteColts-Appleseed.local", "ByteColts-Appleseed.local", "ByteColts-Appleseed"},
		{"默认名没有后缀", "ByteColts-ASUS", "ByteColts-ASUS", "ByteColts-ASUS"},
		{"兜底默认名", "windows", "windows", "windows"},
		{"自定义名带后缀原样", "studio.local", "ByteColts-Appleseed.local", "studio.local"},
		{"自定义名原样", "工作室 Mac", "ByteColts-Appleseed.local", "工作室 Mac"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := &application{deviceName: tc.deviceName, deviceNameDefault: tc.defaultName}
			if got := app.deviceDisplayName(); got != tc.want {
				t.Errorf("deviceDisplayName() = %q，期望 %q", got, tc.want)
			}
		})
	}
}

// TestDevicePageShowsTheNameWithoutDotLocal 检查设备页的侧栏条目和详情标题都用
// 去掉后缀的显示名。设置页的输入框绑的是 a.deviceName，保留完整主机名。
func TestDevicePageShowsTheNameWithoutDotLocal(t *testing.T) {
	app := previewApp("")
	app.deviceName = "ByteColts-Appleseed.local"
	app.deviceNameDefault = app.deviceName
	app.setPage(pageDevices)

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)
	tt.Frame()

	if !hasExactText(tt.Texts(), "ByteColts-Appleseed") {
		t.Errorf("设备页应显示去掉 .local 的本机名，实际显示：%q", tt.Texts())
	}
	if hasExactText(tt.Texts(), "ByteColts-Appleseed.local") {
		t.Errorf("设备页不应显示带 .local 的本机名，实际显示：%q", tt.Texts())
	}
	if app.deviceName != "ByteColts-Appleseed.local" {
		t.Errorf("设置页绑定的设备名 = %q，期望保留完整主机名", app.deviceName)
	}
}
