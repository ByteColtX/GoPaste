package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"gopaste/internal/history"
	"gopaste/internal/history/sqlite"

	_ "modernc.org/sqlite" // registers the "sqlite" driver for the schema test
)

func openMemory(t *testing.T) *sqlite.Repository {
	t.Helper()
	repository, err := sqlite.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("Open(:memory:) = %v", err)
	}
	t.Cleanup(func() {
		if err := repository.Close(); err != nil {
			t.Errorf("Close() = %v", err)
		}
	})
	return repository
}

// normalized rewrites a timestamp the way the repository stores it, so two
// items compare equal only when their instants match.
func normalized(item history.Item) history.Item {
	item.CreatedAt = time.UnixMilli(item.CreatedAt.UnixMilli()).UTC()
	return item
}

func mustPut(t *testing.T, repository history.Repository, items ...history.Item) {
	t.Helper()
	if err := repository.Put(context.Background(), items...); err != nil {
		t.Fatalf("Put(%d items) = %v", len(items), err)
	}
}

func mustLoad(t *testing.T, repository history.Repository) []history.Item {
	t.Helper()
	items, err := repository.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	return items
}

// fixture returns one item covering every stored field, so a round trip can
// lose nothing silently.
func fixture() history.Item {
	return history.Item{
		ID:   "item-1",
		Kind: history.KindCode,
		Text: "func main() {}",
		Image: &history.Image{
			PNG:    []byte{0x89, 'P', 'N', 'G', 0x00, 0xff},
			Width:  640,
			Height: 480,
		},
		FileSize:       2048,
		SourceDeviceID: "peer-a",
		CreatedAt:      time.Date(2026, time.March, 4, 5, 6, 7, 891_000_000, time.UTC),
		Favorite:       true,
		Sync:           history.SyncStatePartial,
		Deliveries: []history.DeviceDelivery{
			{DeviceID: "peer-a", State: history.DeliveryReceived},
			{DeviceID: "peer-b", State: history.DeliveryOffline},
			{DeviceID: "peer-c", State: history.DeliveryPending},
		},
	}
}

func TestRoundTripPreservesEveryField(t *testing.T) {
	repository := openMemory(t)
	want := fixture()
	mustPut(t, repository, want)

	got := mustLoad(t, repository)
	if len(got) != 1 {
		t.Fatalf("Load() returned %d items, want 1", len(got))
	}
	if !reflect.DeepEqual(normalized(got[0]), normalized(want)) {
		t.Errorf("round trip lost data:\n got %#v\nwant %#v", normalized(got[0]), normalized(want))
	}
}

func TestRoundTripKeepsDeliveryOrder(t *testing.T) {
	repository := openMemory(t)
	want := fixture()
	mustPut(t, repository, want)

	got := mustLoad(t, repository)
	if len(got) != 1 {
		t.Fatalf("Load() returned %d items, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0].Deliveries, want.Deliveries) {
		t.Errorf("deliveries = %v, want %v", got[0].Deliveries, want.Deliveries)
	}
}

func TestRoundTripStoresTextWithoutImage(t *testing.T) {
	repository := openMemory(t)
	want := history.Item{
		ID:        "text-1",
		Kind:      history.KindText,
		Text:      "plain text",
		CreatedAt: time.UnixMilli(1_700_000_000_000).UTC(),
	}
	mustPut(t, repository, want)

	got := mustLoad(t, repository)
	if len(got) != 1 {
		t.Fatalf("Load() returned %d items, want 1", len(got))
	}
	if got[0].Image != nil {
		t.Errorf("Image = %#v, want nil", got[0].Image)
	}
	if !reflect.DeepEqual(normalized(got[0]), normalized(want)) {
		t.Errorf("got %#v, want %#v", normalized(got[0]), normalized(want))
	}
}

func TestLoadOrdersNewestFirst(t *testing.T) {
	repository := openMemory(t)
	base := time.UnixMilli(1_700_000_000_000).UTC()
	mustPut(t, repository,
		history.Item{ID: "middle", CreatedAt: base.Add(time.Minute)},
		history.Item{ID: "oldest", CreatedAt: base},
		history.Item{ID: "newest", CreatedAt: base.Add(2 * time.Minute)},
	)

	got := mustLoad(t, repository)
	ids := make([]string, len(got))
	for i, item := range got {
		ids[i] = item.ID
	}
	want := []string{"newest", "middle", "oldest"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("order = %v, want %v", ids, want)
	}
}

func TestPutUpsertsExistingItem(t *testing.T) {
	repository := openMemory(t)
	original := fixture()
	mustPut(t, repository, original)

	updated := original
	updated.Text = "edited"
	updated.Favorite = false
	updated.Sync = history.SyncStateSynced
	updated.Deliveries = []history.DeviceDelivery{
		{DeviceID: "peer-b", State: history.DeliveryReceived},
	}
	mustPut(t, repository, updated)

	got := mustLoad(t, repository)
	if len(got) != 1 {
		t.Fatalf("Load() returned %d items, want 1 (Put must upsert, not append)", len(got))
	}
	if !reflect.DeepEqual(normalized(got[0]), normalized(updated)) {
		t.Errorf("got %#v, want %#v", normalized(got[0]), normalized(updated))
	}
}

func TestDeleteRemovesItemAndItsDeliveries(t *testing.T) {
	repository := openMemory(t)
	mustPut(t, repository,
		fixture(),
		history.Item{ID: "keep", CreatedAt: time.UnixMilli(1_700_000_000_000).UTC()},
	)

	if err := repository.Delete(context.Background(), "item-1", "missing"); err != nil {
		t.Fatalf("Delete() = %v, want nil for an unknown ID", err)
	}

	got := mustLoad(t, repository)
	if len(got) != 1 || got[0].ID != "keep" {
		t.Fatalf("Load() = %v, want only the kept item", got)
	}

	// Re-putting the deleted ID must not resurrect stale deliveries.
	mustPut(t, repository, history.Item{ID: "item-1", CreatedAt: time.UnixMilli(1_700_000_000_000).UTC()})
	got = mustLoad(t, repository)
	for _, item := range got {
		if item.ID == "item-1" && len(item.Deliveries) != 0 {
			t.Errorf("resurrected deliveries = %v, want none", item.Deliveries)
		}
	}
}

func TestPutRejectsEmptyIDAndStoresNothing(t *testing.T) {
	repository := openMemory(t)
	valid := history.Item{ID: "valid", CreatedAt: time.UnixMilli(1_700_000_000_000).UTC()}

	err := repository.Put(context.Background(), valid, history.Item{ID: "  "})
	if !errors.Is(err, history.ErrInvalidItem) {
		t.Fatalf("Put(empty ID) = %v, want ErrInvalidItem", err)
	}
	if got := mustLoad(t, repository); len(got) != 0 {
		t.Errorf("Load() = %v, want nothing stored after a rejected Put", got)
	}
}

func TestLoadOnEmptyDatabaseReturnsNoItems(t *testing.T) {
	repository := openMemory(t)
	got := mustLoad(t, repository)
	if len(got) != 0 {
		t.Errorf("Load() = %v, want no items", got)
	}
}

func TestDatabaseSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	ctx := context.Background()

	first, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	want := fixture()
	mustPut(t, first, want)
	if err := first.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	second, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen = %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	got := mustLoad(t, second)
	if len(got) != 1 || !reflect.DeepEqual(normalized(got[0]), normalized(want)) {
		t.Errorf("reopened data = %#v, want %#v", got, want)
	}
}

func TestOpenMigratesOnceAndReopensCleanly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	ctx := context.Background()

	for attempt := range 2 {
		repository, err := sqlite.Open(ctx, path)
		if err != nil {
			t.Fatalf("Open() attempt %d = %v", attempt+1, err)
		}
		if err := repository.Close(); err != nil {
			t.Fatalf("Close() attempt %d = %v", attempt+1, err)
		}
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	ctx := context.Background()

	repository, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if err := repository.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() = %v", err)
	}
	if _, err := raw.ExecContext(ctx, "PRAGMA user_version = 99"); err != nil {
		t.Fatalf("bump user_version: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw Close() = %v", err)
	}

	if _, err := sqlite.Open(ctx, path); err == nil {
		t.Fatal("Open() on a newer schema = nil, want an error")
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	for _, path := range []string{"", "   "} {
		if _, err := sqlite.Open(context.Background(), path); !errors.Is(err, sqlite.ErrEmptyPath) {
			t.Errorf("Open(%q) = %v, want ErrEmptyPath", path, err)
		}
	}
}

func TestOpenRejectsPathWithQuerySeparator(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db") + "?mode=ro"
	if _, err := sqlite.Open(context.Background(), path); !errors.Is(err, sqlite.ErrInvalidPath) {
		t.Errorf("Open(%q) = %v, want ErrInvalidPath", path, err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	repository := openMemory(t)
	base := time.UnixMilli(1_700_000_000_000).UTC()

	var group sync.WaitGroup
	for worker := range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			id := string(rune('a' + worker))
			for round := range 5 {
				item := history.Item{
					ID:        id,
					Text:      "round",
					CreatedAt: base.Add(time.Duration(round) * time.Second),
				}
				if err := repository.Put(context.Background(), item); err != nil {
					t.Errorf("Put() = %v", err)
					return
				}
				if _, err := repository.Load(context.Background()); err != nil {
					t.Errorf("Load() = %v", err)
					return
				}
			}
		}()
	}
	group.Wait()

	if got := mustLoad(t, repository); len(got) != 8 {
		t.Errorf("Load() returned %d items, want 8", len(got))
	}
}
