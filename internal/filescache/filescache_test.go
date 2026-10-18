package filescache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// now 是这些测试里的固定「当前时间」，所有 ReceivedAt 都以它为基准。
var now = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// entry 造一条 entry：hours 是它多久之前到达的。
func entry(id string, bytes int64, hours int) Entry {
	return Entry{ID: id, Bytes: bytes, ReceivedAt: now.Add(-time.Duration(hours) * time.Hour)}
}

func TestPlanDropsExpiredEntries(t *testing.T) {
	policy := Policy{MaxAge: 24 * time.Hour}
	entries := []Entry{entry("old", 10, 25), entry("fresh", 10, 1)}
	got := Plan(entries, policy, now)
	if len(got) != 1 || got[0] != "old" {
		t.Errorf("要删的 = %v，期望 [old]", got)
	}
}

func TestPlanKeepsAnEntryRightAtTheLimit(t *testing.T) {
	policy := Policy{MaxAge: 24 * time.Hour}
	if got := Plan([]Entry{entry("edge", 10, 24)}, policy, now); len(got) != 0 {
		t.Errorf("刚好到期的不该删，实际删了 %v", got)
	}
}

func TestPlanKeepsEverythingWithinTheQuota(t *testing.T) {
	policy := Policy{QuotaBytes: 100}
	entries := []Entry{entry("a", 40, 3), entry("b", 40, 2)}
	if got := Plan(entries, policy, now); len(got) != 0 {
		t.Errorf("没超配额不该删，实际删了 %v", got)
	}
}

func TestPlanDropsOldestUntilWithinTheQuota(t *testing.T) {
	policy := Policy{QuotaBytes: 100}
	entries := []Entry{entry("newest", 50, 1), entry("oldest", 50, 9), entry("middle", 50, 5)}
	got := Plan(entries, policy, now)
	if len(got) != 1 || got[0] != "oldest" {
		t.Fatalf("要删的 = %v，期望 [oldest]", got)
	}

	// 收紧配额：只删最旧的那条还不够，必须接着删下一条。
	policy.QuotaBytes = 60
	got = Plan(entries, policy, now)
	if len(got) != 2 || got[0] != "oldest" || got[1] != "middle" {
		t.Errorf("要删的 = %v，期望 [oldest middle]", got)
	}
}

func TestPlanDropsExpiredBeforeWeighingTheQuota(t *testing.T) {
	// 过期的那条体积很大：如果把它算进总量，配额会连带把还年轻的记录也删掉。
	policy := Policy{QuotaBytes: 100, MaxAge: 24 * time.Hour}
	entries := []Entry{entry("expired", 100, 30), entry("fresh", 100, 1)}
	got := Plan(entries, policy, now)
	if len(got) != 1 || got[0] != "expired" {
		t.Errorf("要删的 = %v，期望只删过期的 expired", got)
	}
}

func TestPlanWithoutLimitsDropsNothing(t *testing.T) {
	entries := []Entry{entry("a", 1<<30, 24*400)}
	if got := Plan(entries, Policy{}, now); len(got) != 0 {
		t.Errorf("没有期限与配额时不该删，实际删了 %v", got)
	}
}

func TestPlanOrdersTiesByID(t *testing.T) {
	policy := Policy{QuotaBytes: 10}
	entries := []Entry{entry("b", 10, 2), entry("a", 10, 2)}
	got := Plan(entries, policy, now)
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("要删的 = %v，期望同一时刻按 ID 取 a", got)
	}
}

func TestMeasureAddsUpTheDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatalf("建目录出错: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), make([]byte, 100), 0o600); err != nil {
		t.Fatalf("写文件出错: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "b.txt"), make([]byte, 24), 0o600); err != nil {
		t.Fatalf("写文件出错: %v", err)
	}
	measured := Measure([]Entry{{ID: "a", Dir: dir}})
	if len(measured) != 1 || measured[0].Bytes != 124 {
		t.Errorf("测得 = %+v，期望 124 字节", measured)
	}
}

func TestMeasureCountsAMissingDirectoryAsZero(t *testing.T) {
	measured := Measure([]Entry{{ID: "gone", Dir: filepath.Join(t.TempDir(), "gone")}})
	if len(measured) != 1 || measured[0].Bytes != 0 {
		t.Errorf("测得 = %+v，期望 0 字节", measured)
	}
}
