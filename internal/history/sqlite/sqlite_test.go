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
		FilePreview: &history.FilePreview{
			Image: &history.Image{PNG: []byte{0x89, 'P', 'N', 'G', 0x01}, Width: 64, Height: 48},
			Text:  "第一行摘录",
		},
		Deliveries: []history.DeviceDelivery{
			{DeviceID: "peer-a", State: history.DeliveryReceived},
			{DeviceID: "peer-b", State: history.DeliveryOffline},
			{DeviceID: "peer-c", State: history.DeliveryPending},
			{DeviceID: "peer-d", State: history.DeliveryTooLarge},
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

func TestRoundTripStoresRichTextMarkup(t *testing.T) {
	repository := openMemory(t)
	want := history.Item{
		ID:        "rich-1",
		Kind:      history.KindRichText,
		Text:      "bold word",
		HTML:      `<p><b>bold</b> word</p>`,
		CreatedAt: time.UnixMilli(1_700_000_000_000).UTC(),
	}
	mustPut(t, repository, want)

	got := mustLoad(t, repository)
	if len(got) != 1 {
		t.Fatalf("Load() returned %d items, want 1", len(got))
	}
	if !reflect.DeepEqual(normalized(got[0]), normalized(want)) {
		t.Errorf("round trip lost the markup:\n got %#v\nwant %#v", normalized(got[0]), normalized(want))
	}
}

func TestRoundTripStoresFilePreviews(t *testing.T) {
	base := time.UnixMilli(1_700_000_000_000).UTC()
	cases := []struct {
		name    string
		preview *history.FilePreview
		want    *history.FilePreview
	}{
		{
			name:    "图片缩略图",
			preview: &history.FilePreview{Image: &history.Image{PNG: []byte{0x89, 'P', 'N', 'G'}, Width: 640, Height: 480}},
			want:    &history.FilePreview{Image: &history.Image{PNG: []byte{0x89, 'P', 'N', 'G'}, Width: 640, Height: 480}},
		},
		{
			name:    "文本摘录",
			preview: &history.FilePreview{Text: "第一行摘录\n第二行"},
			want:    &history.FilePreview{Text: "第一行摘录\n第二行"},
		},
		{
			name:    "没有预览",
			preview: nil,
			want:    nil,
		},
		{
			name:    "空预览",
			preview: &history.FilePreview{},
			want:    nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repository := openMemory(t)
			want := history.Item{
				ID:          "file-1",
				Kind:        history.KindFile,
				Text:        "/tmp/notes.md",
				CreatedAt:   base,
				FilePreview: tc.preview,
			}
			mustPut(t, repository, want)

			got := mustLoad(t, repository)
			if len(got) != 1 {
				t.Fatalf("Load() returned %d items, want 1", len(got))
			}
			if !reflect.DeepEqual(got[0].FilePreview, tc.want) {
				t.Errorf("FilePreview = %#v, want %#v", got[0].FilePreview, tc.want)
			}
		})
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

func TestOpenMigratesVersionOneDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	ctx := context.Background()

	// A database as the first schema step left it: no html column, so the
	// markup a rich-text item carries has nowhere to go yet.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() = %v", err)
	}
	if _, err := raw.ExecContext(ctx, `
		CREATE TABLE history_items (
			id               TEXT    PRIMARY KEY,
			kind             INTEGER NOT NULL,
			content          TEXT    NOT NULL DEFAULT '',
			image_png        BLOB,
			image_width      INTEGER NOT NULL DEFAULT 0,
			image_height     INTEGER NOT NULL DEFAULT 0,
			file_size        INTEGER NOT NULL DEFAULT 0,
			source_device_id TEXT    NOT NULL DEFAULT '',
			created_at_ms    INTEGER NOT NULL,
			favorite         INTEGER NOT NULL DEFAULT 0,
			sync_state       INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE history_deliveries (
			item_id   TEXT    NOT NULL REFERENCES history_items (id) ON DELETE CASCADE,
			device_id TEXT    NOT NULL,
			state     INTEGER NOT NULL,
			position  INTEGER NOT NULL,
			PRIMARY KEY (item_id, device_id)
		);
		INSERT INTO history_items (id, kind, content, created_at_ms)
		VALUES ('legacy', 0, 'kept across the upgrade', 1700000000000);
		PRAGMA user_version = 1;
	`); err != nil {
		t.Fatalf("seed a version 1 database: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw Close() = %v", err)
	}

	repository, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() a version 1 database = %v", err)
	}
	t.Cleanup(func() {
		if err := repository.Close(); err != nil {
			t.Errorf("Close() = %v", err)
		}
	})

	got := mustLoad(t, repository)
	if len(got) != 1 || got[0].ID != "legacy" || got[0].Text != "kept across the upgrade" {
		t.Fatalf("Load() = %#v, want the row written before the upgrade", got)
	}
	if got[0].HTML != "" {
		t.Errorf("legacy HTML = %q, want empty", got[0].HTML)
	}

	// The upgraded column has to be usable right away.
	want := history.Item{
		ID:        "rich-1",
		Kind:      history.KindRichText,
		Text:      "bold word",
		HTML:      `<p><b>bold</b> word</p>`,
		CreatedAt: time.UnixMilli(1_700_000_001_000).UTC(),
	}
	mustPut(t, repository, want)
	got = mustLoad(t, repository)
	for _, item := range got {
		if item.ID == want.ID && item.HTML != want.HTML {
			t.Errorf("HTML after the upgrade = %q, want %q", item.HTML, want.HTML)
		}
	}

	// The same goes for the preview columns a later step adds: a row written
	// before they existed reads back without a preview, and a new one keeps
	// the preview it was given.
	for _, item := range got {
		if item.ID == "legacy" && item.FilePreview != nil {
			t.Errorf("legacy FilePreview = %#v, want nil", item.FilePreview)
		}
	}
	withPreview := history.Item{
		ID:          "file-1",
		Kind:        history.KindFile,
		Text:        "/tmp/notes.md",
		CreatedAt:   time.UnixMilli(1_700_000_002_000).UTC(),
		FilePreview: &history.FilePreview{Text: "第一行摘录"},
	}
	mustPut(t, repository, withPreview)
	for _, item := range mustLoad(t, repository) {
		if item.ID == withPreview.ID && (item.FilePreview == nil || item.FilePreview.Text != "第一行摘录") {
			t.Errorf("FilePreview after the upgrade = %#v, want the stored excerpt", item.FilePreview)
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
