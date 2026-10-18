package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestFormatBytes 检查用量区显示的大小文案：1024 进制，10 以下留一位小数，以上取整，
// 进位到 1024 时升到下一个单位。
func TestFormatBytes(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{bytes: 0, want: "0 B"},
		{bytes: 1023, want: "1023 B"},
		{bytes: 1024, want: "1 KB"},
		{bytes: 5734, want: "5.6 KB"},
		{bytes: 181_248, want: "177 KB"},
		{bytes: 10 * 1024, want: "10 KB"},
		// 1023.96 KB 四舍五入到 1024 KB，应当升成 1 MB，而不是显示 1024 KB。
		{bytes: 1024*1024 - 1, want: "1 MB"},
		{bytes: 75_000_000, want: "72 MB"},
		{bytes: 3 * 1024 * 1024 * 1024, want: "3 GB"},
	}
	for _, testCase := range cases {
		if got := formatBytes(testCase.bytes); got != testCase.want {
			t.Errorf("formatBytes(%d) = %q，期望 %q", testCase.bytes, got, testCase.want)
		}
	}
}

// TestMeasureStorageUsageCountsEveryLocation 检查三类占用各自算到该算的文件：数据库
// 连 -wal/-shm 一起算，收到的文件整棵目录树都算，设置与配对是两个文件的和。
func TestMeasureStorageUsageCountsEveryLocation(t *testing.T) {
	dir := t.TempDir()
	database := filepath.Join(dir, "history.db")
	writeSizedFile(t, database, 1000)
	writeSizedFile(t, database+"-wal", 200)
	writeSizedFile(t, database+"-shm", 30)
	files := filepath.Join(dir, "Files")
	writeSizedFile(t, filepath.Join(files, "item-1", "报表.txt"), 500)
	writeSizedFile(t, filepath.Join(files, "item-2", "nested", "photo.png"), 70)
	settings := filepath.Join(dir, "settings.json")
	writeSizedFile(t, settings, 40)
	devices := filepath.Join(dir, "devices.json")
	writeSizedFile(t, devices, 60)

	usage := measureStorageUsage(storageUsagePaths{
		database: database,
		files:    files,
		settings: settings,
		devices:  devices,
	})

	want := storageUsage{database: 1230, files: 570, settings: 100}
	if usage != want {
		t.Fatalf("用量 = %+v，期望 %+v", usage, want)
	}
	if got := usage.total(); got != 1900 {
		t.Errorf("总计 = %d，期望 1900", got)
	}
}

// TestMeasureStorageUsageIgnoresMissingLocations 检查空路径与不存在的文件都算 0：
// 没有数据目录、同步不可用、或者文件刚好被删掉，都不该让用量区变成错误态。
func TestMeasureStorageUsageIgnoresMissingLocations(t *testing.T) {
	usage := measureStorageUsage(storageUsagePaths{})
	if usage != (storageUsage{}) {
		t.Fatalf("空路径的用量 = %+v，期望全 0", usage)
	}

	dir := t.TempDir()
	usage = measureStorageUsage(storageUsagePaths{
		database: filepath.Join(dir, "missing.db"),
		files:    filepath.Join(dir, "missing-files"),
		settings: filepath.Join(dir, "missing.json"),
		devices:  filepath.Join(dir, "missing-devices.json"),
	})
	if usage != (storageUsage{}) {
		t.Fatalf("不存在的文件的用量 = %+v，期望全 0", usage)
	}
}

// TestStorageSettingsShowsTheUsageBreakdown 检查储存空间页画出总计、三类名称与各自
// 的大小。测试器在 key 重复时会 panic，所以这也是一次新控件 key 的冒烟检查。
func TestStorageSettingsShowsTheUsageBreakdown(t *testing.T) {
	app := previewApp("")
	app.storageUsage = storageUsage{database: 75_000_000, files: 181_248, settings: 5734}
	app.setPage(pageSettings)
	app.setSettingsArea(settingsStorage)

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	for _, want := range []string{
		"72 MB",
		app.t(msgSettingsStorageTotal),
		app.t(msgSettingsStorageDatabase),
		app.t(msgSettingsStorageFiles),
		app.t(msgSettingsStorageSettings),
		"177 KB",
		"5.6 KB",
	} {
		if !tt.HasText(want) {
			t.Errorf("储存空间页应显示 %q，实际显示：%q", want, tt.Texts())
		}
	}
}

// TestStorageSettingsShowsAnEmptyUsage 检查没有任何数据时总计与三类都写 0 B：空状态
// 也是一个数字，不是一个空白区。
func TestStorageSettingsShowsAnEmptyUsage(t *testing.T) {
	app := previewApp("")
	app.setPage(pageSettings)
	app.setSettingsArea(settingsStorage)

	tt := ui.NewTester(app.view, defaultWindowWidth, defaultWindowHeight)

	if !tt.HasText("0 B") {
		t.Errorf("没有数据时应显示 0 B，实际显示：%q", tt.Texts())
	}
}

// TestEnteringStorageMeasuresTheUsage 检查测量时机：进入储存空间页测一次，测量期间
// 重复请求被忽略，结果回来后再点刷新才测第二次。
func TestEnteringStorageMeasuresTheUsage(t *testing.T) {
	app := newApplication()
	measured := 0
	app.onMeasureStorageUsage = func(<-chan struct{}) { measured++ }

	app.setPage(pageSettings)
	if measured != 0 {
		t.Fatalf("进入通用分栏就测了 %d 次，期望一次都不测", measured)
	}
	app.setSettingsArea(settingsStorage)
	if measured != 1 {
		t.Fatalf("进入储存空间分栏后测了 %d 次，期望 1 次", measured)
	}
	if !app.storageUsagePending {
		t.Fatal("结果还没回来时应当标记为测量中")
	}

	// 测量还在飞的时候再点刷新：忽略，不排队。
	app.refreshStorageUsage()
	if measured != 1 {
		t.Fatalf("测量中重复请求后测了 %d 次，期望仍是 1 次", measured)
	}

	app.applyStorageUsage(storageUsage{database: 1024})
	if app.storageUsagePending {
		t.Fatal("结果回来后应当清掉测量中标记")
	}
	app.refreshStorageUsage()
	if measured != 2 {
		t.Fatalf("结果回来后再点刷新测了 %d 次，期望 2 次", measured)
	}
}

// TestReturningToStorageMeasuresAgain 检查离开设置页再回来会重测：这期间记录可能已经
// 变了，重新进来看到的应当是当下的数字。
func TestReturningToStorageMeasuresAgain(t *testing.T) {
	app := newApplication()
	measured := 0
	app.onMeasureStorageUsage = func(<-chan struct{}) { measured++ }

	app.setPage(pageSettings)
	app.setSettingsArea(settingsStorage)
	app.applyStorageUsage(storageUsage{})
	app.setPage(pageHistory)
	app.setPage(pageSettings)

	if measured != 2 {
		t.Fatalf("回到储存空间分栏后共测了 %d 次，期望 2 次", measured)
	}
}

// TestClearingHistoryMeasuresAfterTheFilesAreGone 检查清除历史后重测，而且测量要等
// 收到的文件目录删完：否则刚清掉的那一份还会被算进用量。
func TestClearingHistoryMeasuresAfterTheFilesAreGone(t *testing.T) {
	app := newApplication()
	app.receivedFilesDir = t.TempDir()
	writeSizedFile(t, filepath.Join(app.receivedFilesDir, "item-1", "报表.txt"), 500)
	var after <-chan struct{}
	app.onMeasureStorageUsage = func(channel <-chan struct{}) { after = channel }

	app.clearHistory()

	if after == nil {
		t.Fatal("清除历史后应当带着删除完成的通道去测量")
	}
	<-after
	if _, err := os.Stat(app.receivedFilesDir); !os.IsNotExist(err) {
		t.Fatalf("删除完成后目录应当已经不存在，Stat 返回 %v", err)
	}
}

// writeSizedFile 写一个恰好 size 字节的文件，用来断言统计出来的字节数。
func writeSizedFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("创建目录失败：%v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("写文件 %s 失败：%v", path, err)
	}
}
