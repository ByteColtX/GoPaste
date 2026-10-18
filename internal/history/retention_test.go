package history

import (
	"slices"
	"testing"
	"time"
)

// orderedIDs returns the store's IDs newest first, so a retention test can
// assert both which records survived and the order the list keeps.
func orderedIDs(store *Store) []string {
	view := store.Snapshot(Query{})
	ids := make([]string, len(view.Items))
	for i, item := range view.Items {
		ids[i] = item.ID
	}
	return ids
}

func TestApplyRetentionKeepsNewestItems(t *testing.T) {
	now := time.Now()
	store := New([]Item{
		{ID: "a", CreatedAt: now},
		{ID: "b", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "c", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "d", CreatedAt: now.Add(-3 * time.Minute)},
	})

	removed := store.ApplyRetention(RetentionPolicy{MaxItems: 2})

	if want := []string{"c", "d"}; !slices.Equal(removed, want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	if want := []string{"a", "b"}; !slices.Equal(orderedIDs(store), want) {
		t.Fatalf("remaining = %v, want %v", orderedIDs(store), want)
	}
}

func TestApplyRetentionDropsOldItems(t *testing.T) {
	now := time.Now()
	store := New([]Item{
		{ID: "today", CreatedAt: now},
		{ID: "last week", CreatedAt: now.AddDate(0, 0, -6)},
		{ID: "two weeks", CreatedAt: now.AddDate(0, 0, -14)},
		{ID: "last year", CreatedAt: now.AddDate(-1, 0, 0)},
	})

	removed := store.ApplyRetention(RetentionPolicy{MaxDays: 7})

	if want := []string{"two weeks", "last year"}; !slices.Equal(removed, want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	if want := []string{"today", "last week"}; !slices.Equal(orderedIDs(store), want) {
		t.Fatalf("remaining = %v, want %v", orderedIDs(store), want)
	}
}

// TestApplyRetentionNeverDropsFavorites checks both halves of the favorite
// rule: a starred record survives even when it is ancient and past the item
// budget, and it does not consume that budget.
func TestApplyRetentionNeverDropsFavorites(t *testing.T) {
	now := time.Now()
	store := New([]Item{
		{ID: "newest", CreatedAt: now},
		{ID: "older", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "favorite old", CreatedAt: now.AddDate(-1, 0, 0), Favorite: true},
		{ID: "favorite new", CreatedAt: now.Add(-2 * time.Minute), Favorite: true},
	})

	removed := store.ApplyRetention(RetentionPolicy{MaxItems: 1, MaxDays: 30})

	if want := []string{"older"}; !slices.Equal(removed, want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	// Both favorites are still there, and the single item slot went to the
	// newest ordinary record rather than being spent on a favorite.
	want := []string{"newest", "favorite new", "favorite old"}
	if got := orderedIDs(store); !slices.Equal(got, want) {
		t.Fatalf("remaining = %v, want %v", got, want)
	}
}

// TestApplyRetentionAgeDoesNotSpendItemSlots pins the interaction the other way
// round: a record dropped for age must not also use up one of the item slots,
// or a history with one ancient entry would keep one record fewer than asked.
func TestApplyRetentionAgeDoesNotSpendItemSlots(t *testing.T) {
	now := time.Now()
	store := New([]Item{
		{ID: "a", CreatedAt: now},
		{ID: "b", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "c", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "ancient", CreatedAt: now.AddDate(0, 0, -90)},
	})

	removed := store.ApplyRetention(RetentionPolicy{MaxItems: 3, MaxDays: 30})

	if want := []string{"ancient"}; !slices.Equal(removed, want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(orderedIDs(store), want) {
		t.Fatalf("remaining = %v, want %v", orderedIDs(store), want)
	}
}

func TestApplyRetentionZeroPolicyKeepsEverything(t *testing.T) {
	now := time.Now()
	store := New([]Item{
		{ID: "old", CreatedAt: now.AddDate(-2, 0, 0)},
		{ID: "new", CreatedAt: now},
	})

	if removed := store.ApplyRetention(RetentionPolicy{}); removed != nil {
		t.Fatalf("removed = %v, want nil", removed)
	}
	if got := len(orderedIDs(store)); got != 2 {
		t.Fatalf("kept %d items, want 2", got)
	}

	for _, policy := range []RetentionPolicy{
		{},
		{MaxItems: -1},
		{MaxDays: -30},
		{MaxItems: 0, MaxDays: 0},
	} {
		if !policy.Unlimited() {
			t.Errorf("%+v should read as unlimited", policy)
		}
	}
	for _, policy := range []RetentionPolicy{{MaxItems: 1}, {MaxDays: 1}, {MaxItems: 5, MaxDays: 90}} {
		if policy.Unlimited() {
			t.Errorf("%+v should not read as unlimited", policy)
		}
	}
}

func TestApplyRetentionReportsNothingWhenNothingIsDropped(t *testing.T) {
	now := time.Now()
	store := New([]Item{{ID: "only", CreatedAt: now}})

	if removed := store.ApplyRetention(RetentionPolicy{MaxItems: 10, MaxDays: 30}); removed != nil {
		t.Fatalf("removed = %v, want nil", removed)
	}
	if want := []string{"only"}; !slices.Equal(orderedIDs(store), want) {
		t.Fatalf("remaining = %v, want %v", orderedIDs(store), want)
	}
}

func TestApplyRetentionMovesSelectionOffDroppedItem(t *testing.T) {
	now := time.Now()
	store := New([]Item{
		{ID: "newest", CreatedAt: now},
		{ID: "older", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "oldest", CreatedAt: now.Add(-2 * time.Minute)},
	})
	store.Select("oldest")

	store.ApplyRetention(RetentionPolicy{MaxItems: 2})

	if got := store.Snapshot(Query{}).SelectedID; got != "newest" {
		t.Fatalf("selected = %q, want newest", got)
	}
}

func TestApplyRetentionClearsSelectionWhenStoreEmpties(t *testing.T) {
	store := New([]Item{{ID: "ancient", CreatedAt: time.Now().AddDate(-1, 0, 0)}})

	removed := store.ApplyRetention(RetentionPolicy{MaxDays: 30})

	if want := []string{"ancient"}; !slices.Equal(removed, want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	if got := store.Snapshot(Query{}).SelectedID; got != "" {
		t.Fatalf("selected = %q, want empty", got)
	}
}
