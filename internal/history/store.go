// Package history owns clipboard history data and its user-facing operations.
// It deliberately has no dependency on MyGo or any UI type.
package history

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// Kind identifies the payload stored in a history item.
type Kind uint8

const (
	KindText Kind = iota
	KindLink
	KindCode
	KindImage
	KindFile
	KindRichText
)

// BaseKind returns the broad category a subtype belongs to, such as the
// preview's primary type label for a link or code item. Links and code are
// text payloads with a detected subtype, so their base kind is text; rich
// text keeps its own format.
func (kind Kind) BaseKind() Kind {
	switch kind {
	case KindLink, KindCode:
		return KindText
	default:
		return kind
	}
}

// KindFilter selects a payload format for the history list. The text filter
// covers every text payload, including links and code, which the tag filter
// narrows further.
type KindFilter uint8

const (
	KindFilterAny KindFilter = iota
	KindFilterText
	KindFilterRichText
	KindFilterImage
	KindFilterFile
)

// TagFilter selects an automatically detected content tag.
type TagFilter uint8

const (
	TagFilterAny TagFilter = iota
	TagFilterLink
	TagFilterCode
	TagFilterImage
)

// PeriodFilter selects a window of time relative to now.
type PeriodFilter uint8

const (
	PeriodFilterAny PeriodFilter = iota
	PeriodFilterToday
	PeriodFilterYesterday
	PeriodFilterLast7Days
	PeriodFilterLast30Days
)

// SourceFilter selects where history items came from. Its zero value, the
// empty source, shows items from every source, so it follows the other
// facets' zero-value rule.
//
// An item captured on this machine carries no source device ID, so "this
// machine" cannot be written as a device ID: it is its own case, which is
// why this filter is a value with constructors rather than a plain enum.
type SourceFilter struct {
	local    bool
	deviceID string
}

// LocalSource returns the filter for items captured on this machine.
func LocalSource() SourceFilter { return SourceFilter{local: true} }

// DeviceSource returns the filter for items received from one paired device.
// An empty device ID returns the empty filter, which shows every source.
func DeviceSource(deviceID string) SourceFilter {
	if deviceID == "" {
		return SourceFilter{}
	}
	return SourceFilter{deviceID: deviceID}
}

// IsSet reports whether the filter narrows the history to one source.
func (s SourceFilter) IsSet() bool { return s != SourceFilter{} }

// IsLocal reports whether the filter is this machine.
func (s SourceFilter) IsLocal() bool { return s.local }

// DeviceID returns the paired device the filter names. It is empty for this
// machine and for the empty filter.
func (s SourceFilter) DeviceID() string { return s.deviceID }

// Image stores the portable image payload needed by history and clipboard
// writes. UI bitmap objects stay outside this package.
type Image struct {
	PNG    []byte
	Width  int
	Height int
}

// Item is the data owned by the history module. Image carries an inline PNG
// payload, while FileSize carries the byte size of a file payload whose
// content stays on disk, so rendering never touches the filesystem.
type Item struct {
	ID       string
	Kind     Kind
	Text     string
	Image    *Image
	FileSize int64
	// SourceDeviceID is the stable ID of the paired device the item arrived
	// from, the same ID the device list uses. It is empty when the item was
	// captured on this machine. The device name is never stored: the preview
	// resolves it from the device list, so a renamed device still maps to the
	// same history record.
	SourceDeviceID string
	CreatedAt      time.Time
	Favorite       bool
	// Sync is the item's delivery summary across paired devices. Its zero
	// value, SyncStateNone, means the item has no sync record and the
	// preview hides the sync section.
	Sync SyncState
	// Deliveries records each target device's delivery outcome, keyed by the
	// stable paired-device ID. The preview resolves them against the current
	// device list, so it stays correct as devices are paired or removed.
	Deliveries []DeviceDelivery
}

// IsLocal reports whether the item was captured on this machine. Only local
// items are delivered to paired devices, so only they carry a sync status; an
// item received from a peer already arrived here.
func (i Item) IsLocal() bool { return i.SourceDeviceID == "" }

// Query describes the current history view requested by the UI. Each facet is
// independent: the zero value of every field shows the whole history, so a
// caller sets only the filters it wants.
type Query struct {
	Text string
	// Kind limits the payload format; KindFilterAny shows every format.
	Kind KindFilter
	// Tag limits the detected content tag; TagFilterAny shows every tag.
	Tag TagFilter
	// FavoriteOnly keeps only favorited items.
	FavoriteOnly bool
	// Source limits items to one origin, which is this machine or a paired
	// device. Its zero value shows every source.
	Source SourceFilter
	// Period limits items to a window of time; PeriodFilterAny shows every
	// record regardless of age.
	Period PeriodFilter
	// KindTerms returns extra searchable names for a kind, such as the
	// localized labels the UI shows. The history module owns no display
	// vocabulary of its own, so callers supply it. It may be nil.
	KindTerms func(Kind) []string
	// SourceName resolves an item's SourceDeviceID to a searchable device
	// name. The history module holds no device table, so callers supply it;
	// it returns "" for an unknown device. It may be nil.
	SourceName func(deviceID string) string
}

// Filtered reports whether any facet narrows the list beyond the search text.
func (q Query) Filtered() bool {
	return q.Kind != KindFilterAny || q.Tag != TagFilterAny || q.FavoriteOnly ||
		q.Source.IsSet() || q.Period != PeriodFilterAny
}

// View is a stable snapshot used by both the list and preview panes.
type View struct {
	Items         []Item
	SelectedID    string
	SelectedIndex int
	Total         int
	ImageCount    int
}

// ClipboardWriter is the seam for the operating-system clipboard. MyGo's
// current WriteText method cannot return an error, while WriteImage can.
type ClipboardWriter interface {
	WriteText(string)
	WriteImage([]byte) error
}

var (
	ErrItemNotFound = errors.New("history item not found")
	ErrDuplicateID  = errors.New("history item ID already exists")
	ErrInvalidItem  = errors.New("history item ID is empty")
	ErrNilWriter    = errors.New("clipboard writer is nil")
	ErrNoImage      = errors.New("history item has no image payload")
)

// Store is the deep history module. Its selection is represented by a stable
// item ID; list indexes are derived from each view and never persisted here.
// A Store is owned by the UI state owner; asynchronous clipboard or LAN
// adapters must publish mutations onto that owner's UI thread first.
type Store struct {
	items      []Item
	selectedID string
}

// New creates a store from an initial snapshot. Items are ordered newest
// first; the input slice is copied so callers cannot mutate the store by
// retaining the slice they passed in.
func New(items []Item) *Store {
	store := &Store{items: make([]Item, 0, len(items))}
	for _, item := range items {
		store.items = append(store.items, cloneItem(item))
	}
	sortItemsNewestFirst(store.items)
	if len(store.items) > 0 {
		store.selectedID = store.items[0].ID
	}
	return store
}

// Snapshot filters and orders history for a frame. If the selected item is no
// longer visible, selection is normalized to the first visible item. This is
// an in-memory state correction and performs no I/O.
func (s *Store) Snapshot(query Query) View {
	visible := make([]Item, 0, len(s.items))
	now := time.Now()
	for _, item := range s.items {
		if !matchesQuery(item, query) || !matchesFacets(item, query, now) {
			continue
		}
		visible = append(visible, item)
	}

	selectedIndex := -1
	if len(visible) == 0 {
		s.selectedID = ""
	} else {
		for index, item := range visible {
			if item.ID == s.selectedID {
				selectedIndex = index
				break
			}
		}
		if selectedIndex < 0 {
			selectedIndex = 0
			s.selectedID = visible[0].ID
		}
	}

	imageCount := 0
	for _, item := range s.items {
		if item.Kind == KindImage {
			imageCount++
		}
	}

	return View{
		Items:         visible,
		SelectedID:    s.selectedID,
		SelectedIndex: selectedIndex,
		Total:         len(s.items),
		ImageCount:    imageCount,
	}
}

// Select changes the selected item when the ID belongs to this store.
func (s *Store) Select(id string) {
	for _, item := range s.items {
		if item.ID == id {
			s.selectedID = id
			return
		}
	}
}

// Move moves selection inside the current query result and clamps it to the
// visible range. It returns whether the selected ID changed.
func (s *Store) Move(query Query, delta int) bool {
	view := s.Snapshot(query)
	if len(view.Items) == 0 {
		return false
	}

	index := view.SelectedIndex
	if index < 0 {
		index = 0
	} else {
		index += delta
	}
	index = max(0, min(index, len(view.Items)-1))
	selectedID := view.Items[index].ID
	changed := selectedID != s.selectedID
	s.selectedID = selectedID
	return changed
}

// Append inserts one item, keeping the history ordered newest first. IDs are
// stable and unique.
func (s *Store) Append(item Item) error {
	if strings.TrimSpace(item.ID) == "" {
		return ErrInvalidItem
	}
	for _, existing := range s.items {
		if existing.ID == item.ID {
			return ErrDuplicateID
		}
	}
	index := sort.Search(len(s.items), func(i int) bool {
		return !s.items[i].CreatedAt.After(item.CreatedAt)
	})
	s.items = append(s.items, Item{})
	copy(s.items[index+1:], s.items[index:])
	s.items[index] = cloneItem(item)
	if s.selectedID == "" {
		s.selectedID = item.ID
	}
	return nil
}

// ToggleFavorite changes one item's favorite state and returns its new value.
func (s *Store) ToggleFavorite(id string) (bool, error) {
	for index := range s.items {
		if s.items[index].ID != id {
			continue
		}
		s.items[index].Favorite = !s.items[index].Favorite
		return s.items[index].Favorite, nil
	}
	return false, ErrItemNotFound
}

// Item returns one stored item by ID and whether it exists. The caller only
// reads the result, so the item is returned by value like a Snapshot entry.
func (s *Store) Item(id string) (Item, bool) {
	for _, item := range s.items {
		if item.ID == id {
			return item, true
		}
	}
	return Item{}, false
}

// Delete removes one item and returns ErrItemNotFound when the ID is unknown.
// When the removed item was selected, selection moves to the item that takes
// its place, or to the previous item when the last one was removed, so the
// preview keeps its position instead of jumping back to the top of the list.
func (s *Store) Delete(id string) error {
	for index := range s.items {
		if s.items[index].ID != id {
			continue
		}
		s.items = append(s.items[:index], s.items[index+1:]...)
		if s.selectedID == id {
			if len(s.items) == 0 {
				s.selectedID = ""
			} else {
				s.selectedID = s.items[min(index, len(s.items)-1)].ID
			}
		}
		return nil
	}
	return ErrItemNotFound
}

// Restore puts a deleted item back in newest-first order and selects it, so
// the preview returns to the record the user just restored. It backs the undo
// offered for a short while after Delete; the ID is free again by then.
func (s *Store) Restore(item Item) error {
	if err := s.Append(item); err != nil {
		return err
	}
	s.selectedID = item.ID
	return nil
}

// Copy writes the exact stored payload to the supplied clipboard adapter.
func (s *Store) Copy(id string, writer ClipboardWriter) error {
	if writer == nil {
		return ErrNilWriter
	}
	for _, item := range s.items {
		if item.ID != id {
			continue
		}
		if item.Kind == KindImage {
			if item.Image == nil || len(item.Image.PNG) == 0 {
				return ErrNoImage
			}
			return writer.WriteImage(item.Image.PNG)
		}
		writer.WriteText(item.Text)
		return nil
	}
	return ErrItemNotFound
}

// Clear removes every item and resets selection.
func (s *Store) Clear() {
	s.items = nil
	s.selectedID = ""
}

// matchesFacets applies every facet of the query to one item. A facet left at
// its zero value matches every item.
func matchesFacets(item Item, query Query, now time.Time) bool {
	return matchesKind(item, query.Kind) &&
		matchesTag(item, query.Tag) &&
		matchesPeriod(item, query.Period, now) &&
		matchesSource(item, query) &&
		(!query.FavoriteOnly || item.Favorite)
}

// matchesKind applies the payload-format facet. Text matches links and code
// too, because their payload is text; the tag facet narrows them.
func matchesKind(item Item, filter KindFilter) bool {
	switch filter {
	case KindFilterText:
		return item.Kind.BaseKind() == KindText
	case KindFilterRichText:
		return item.Kind == KindRichText
	case KindFilterImage:
		return item.Kind == KindImage
	case KindFilterFile:
		return item.Kind == KindFile
	default:
		return true
	}
}

// matchesTag applies the detected-tag facet.
func matchesTag(item Item, filter TagFilter) bool {
	switch filter {
	case TagFilterLink:
		return item.Kind == KindLink
	case TagFilterCode:
		return item.Kind == KindCode
	case TagFilterImage:
		return item.Kind == KindImage
	default:
		return true
	}
}

// matchesSource applies the source facet. The empty source matches every
// item, including the ones captured here.
func matchesSource(item Item, query Query) bool {
	switch {
	case query.Source.IsLocal():
		return item.IsLocal()
	case query.Source.deviceID != "":
		return item.SourceDeviceID == query.Source.deviceID
	default:
		return true
	}
}

// matchesPeriod applies the time-window facet against now.
func matchesPeriod(item Item, filter PeriodFilter, now time.Time) bool {
	if filter == PeriodFilterAny {
		return true
	}
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch filter {
	case PeriodFilterToday:
		return !item.CreatedAt.Before(startOfToday)
	case PeriodFilterYesterday:
		start := startOfToday.AddDate(0, 0, -1)
		return !item.CreatedAt.Before(start) && item.CreatedAt.Before(startOfToday)
	case PeriodFilterLast7Days:
		return !item.CreatedAt.Before(startOfToday.AddDate(0, 0, -6))
	case PeriodFilterLast30Days:
		return !item.CreatedAt.Before(startOfToday.AddDate(0, 0, -29))
	default:
		return true
	}
}

func matchesQuery(item Item, query Query) bool {
	text := strings.ToLower(strings.TrimSpace(query.Text))
	if text == "" {
		return true
	}
	parts := []string{item.Text}
	if query.SourceName != nil && item.SourceDeviceID != "" {
		if name := query.SourceName(item.SourceDeviceID); name != "" {
			parts = append(parts, name)
		}
	}
	if query.KindTerms != nil {
		parts = append(parts, query.KindTerms(item.Kind)...)
	}
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), text)
}

func cloneItem(item Item) Item {
	if len(item.Deliveries) > 0 {
		item.Deliveries = append([]DeviceDelivery(nil), item.Deliveries...)
	}
	if item.Image == nil {
		return item
	}
	image := *item.Image
	image.PNG = append([]byte(nil), item.Image.PNG...)
	item.Image = &image
	return item
}

// sortItemsNewestFirst orders items from newest to oldest, keeping the
// original order for items that share a timestamp.
func sortItemsNewestFirst(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}
