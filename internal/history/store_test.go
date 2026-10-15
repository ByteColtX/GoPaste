package history

import (
	"testing"
	"time"
)

// filterFixture builds a store with one item of every kind, a favorite, a
// received item, and items at known ages, so each facet can be exercised.
func filterFixture(t *testing.T) *Store {
	t.Helper()
	now := time.Now()
	return New([]Item{
		{ID: "text", Kind: KindText, Text: "plain", CreatedAt: now},
		{ID: "rich", Kind: KindRichText, Text: "<p>rich</p>", CreatedAt: now},
		{ID: "link", Kind: KindLink, Text: "https://example.com", CreatedAt: now},
		{ID: "code", Kind: KindCode, Text: "func main() {}", CreatedAt: now},
		{ID: "image", Kind: KindImage, Image: &Image{PNG: []byte{1}}, CreatedAt: now},
		{ID: "file", Kind: KindFile, Text: "notes.md", FileSize: 12, CreatedAt: now},
		{ID: "favorite", Kind: KindText, Text: "keep me", CreatedAt: now, Favorite: true},
		{ID: "received", Kind: KindText, Text: "from peer", CreatedAt: now, SourceDeviceID: "peer-a"},
		{ID: "yesterday", Kind: KindText, Text: "yesterday", CreatedAt: now.AddDate(0, 0, -1)},
		{ID: "old", Kind: KindText, Text: "last month", CreatedAt: now.AddDate(0, 0, -40)},
	})
}

// idsOf returns the IDs of a view in order, so a test can compare a filtered
// result as a set without depending on snapshot ordering.
func idsOf(view View) map[string]bool {
	ids := make(map[string]bool, len(view.Items))
	for _, item := range view.Items {
		ids[item.ID] = true
	}
	return ids
}

func TestKindFilterMatchesBaseKinds(t *testing.T) {
	store := filterFixture(t)

	cases := []struct {
		name   string
		filter KindFilter
		want   []string
	}{
		{
			name:   "text covers links and code",
			filter: KindFilterText,
			want:   []string{"text", "link", "code", "favorite", "received", "yesterday", "old"},
		},
		{name: "rich text keeps its own format", filter: KindFilterRichText, want: []string{"rich"}},
		{name: "image", filter: KindFilterImage, want: []string{"image"}},
		{name: "file", filter: KindFilterFile, want: []string{"file"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := idsOf(store.Snapshot(Query{Kind: tc.filter}))
			if len(got) != len(tc.want) {
				t.Fatalf("got %d items, want %d: %v", len(got), len(tc.want), got)
			}
			for _, id := range tc.want {
				if !got[id] {
					t.Errorf("missing %q in %v", id, got)
				}
			}
		})
	}
}

func TestTagFilterMatchesSubtypes(t *testing.T) {
	store := filterFixture(t)

	cases := []struct {
		filter TagFilter
		want   string
	}{
		{TagFilterLink, "link"},
		{TagFilterCode, "code"},
		{TagFilterImage, "image"},
	}
	for _, tc := range cases {
		view := store.Snapshot(Query{Tag: tc.filter})
		if len(view.Items) != 1 || view.Items[0].ID != tc.want {
			t.Errorf("tag %v: got %v, want only %q", tc.filter, idsOf(view), tc.want)
		}
	}
}

func TestFacetsCombine(t *testing.T) {
	store := filterFixture(t)

	view := store.Snapshot(Query{Kind: KindFilterText, Tag: TagFilterLink})
	if len(view.Items) != 1 || view.Items[0].ID != "link" {
		t.Fatalf("text + link: got %v, want only link", idsOf(view))
	}

	view = store.Snapshot(Query{Kind: KindFilterText, FavoriteOnly: true})
	if len(view.Items) != 1 || view.Items[0].ID != "favorite" {
		t.Fatalf("text + favorite: got %v, want only favorite", idsOf(view))
	}

	view = store.Snapshot(Query{Source: DeviceSource("peer-a")})
	if len(view.Items) != 1 || view.Items[0].ID != "received" {
		t.Fatalf("source: got %v, want only received", idsOf(view))
	}
}

func TestPeriodFilterWindows(t *testing.T) {
	store := filterFixture(t)

	cases := []struct {
		name   string
		filter PeriodFilter
		want   []string
	}{
		{"today", PeriodFilterToday, []string{"text", "rich", "link", "code", "image", "file", "favorite", "received"}},
		{"yesterday", PeriodFilterYesterday, []string{"yesterday"}},
		{"last 7 days", PeriodFilterLast7Days, []string{"yesterday"}},
		{"last 30 days", PeriodFilterLast30Days, []string{"yesterday"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := idsOf(store.Snapshot(Query{Period: tc.filter}))
			for _, id := range tc.want {
				if !got[id] {
					t.Errorf("missing %q in %v", id, got)
				}
			}
			if got["old"] {
				t.Errorf("old item should be outside the window: %v", got)
			}
			if tc.filter == PeriodFilterToday && got["yesterday"] {
				t.Errorf("today must not include yesterday's item: %v", got)
			}
		})
	}
}

func TestQueryFiltered(t *testing.T) {
	if (Query{}).Filtered() {
		t.Fatal("the zero query must not read as filtered")
	}
	if !(Query{FavoriteOnly: true}).Filtered() {
		t.Fatal("a favorite filter must read as filtered")
	}
	if !(Query{Source: DeviceSource("peer-a")}).Filtered() {
		t.Fatal("a source filter must read as filtered")
	}
	if !(Query{Kind: KindFilterText}).Filtered() {
		t.Fatal("a kind filter must read as filtered")
	}
	if !(Query{Tag: TagFilterLink}).Filtered() {
		t.Fatal("a tag filter must read as filtered")
	}
	if !(Query{Period: PeriodFilterToday}).Filtered() {
		t.Fatal("a period filter must read as filtered")
	}
}

func TestSourceFilterKeepsLocalItemsApart(t *testing.T) {
	store := filterFixture(t)

	view := store.Snapshot(Query{Source: LocalSource()})
	got := idsOf(view)
	if got["received"] {
		t.Errorf("the local source must not include a received item: %v", got)
	}
	if !got["text"] {
		t.Errorf("the local source must include items captured here: %v", got)
	}

	// The empty source is the zero value, so an unset filter shows both the
	// local items and the ones received from a peer.
	all := idsOf(store.Snapshot(Query{}))
	if !all["received"] || !all["text"] {
		t.Fatalf("the empty source must show every source: %v", all)
	}
	if !LocalSource().IsSet() || (SourceFilter{}).IsSet() {
		t.Fatal("only a source that names one origin may read as set")
	}
	if !LocalSource().IsLocal() || DeviceSource("peer-a").IsLocal() {
		t.Fatal("IsLocal must report the local source only")
	}
	if got := DeviceSource("peer-a").DeviceID(); got != "peer-a" {
		t.Errorf("device source ID = %q, want peer-a", got)
	}
	if DeviceSource("").IsSet() {
		t.Error("an empty device ID must return the empty source")
	}
}

func TestRichTextBaseKind(t *testing.T) {
	if got := KindRichText.BaseKind(); got != KindRichText {
		t.Fatalf("rich text base kind = %v, want itself", got)
	}
	if got := KindLink.BaseKind(); got != KindText {
		t.Fatalf("link base kind = %v, want text", got)
	}
}

// TestStoreOwnsFilePreviews checks the store deep-copies a file preview: the
// caller that supplied the payload can mutate its slice, struct and image
// afterwards without changing what the store holds.
func TestStoreOwnsFilePreviews(t *testing.T) {
	payload := []byte{1, 2, 3}
	preview := &FilePreview{
		Image: &Image{PNG: payload, Width: 2, Height: 3},
		Text:  "excerpt",
	}
	store := New([]Item{{
		ID:          "file",
		Kind:        KindFile,
		Text:        "photo.png",
		FilePreview: preview,
		CreatedAt:   time.Now(),
	}})

	payload[0] = 9
	preview.Text = "changed"
	preview.Image.Width = 99

	stored, ok := store.Item("file")
	if !ok {
		t.Fatal("stored item not found")
	}
	if got := stored.FilePreview.Image.PNG[0]; got != 1 {
		t.Errorf("存储的预览负载 = %d，期望 1（store 应深拷贝）", got)
	}
	if got := stored.FilePreview.Text; got != "excerpt" {
		t.Errorf("存储的预览文本 = %q，期望 %q", got, "excerpt")
	}
	if got := stored.FilePreview.Image.Width; got != 2 {
		t.Errorf("存储的预览宽度 = %d，期望 2", got)
	}
}
