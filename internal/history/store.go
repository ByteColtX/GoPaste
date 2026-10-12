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
)

// BaseKind returns the broad category used by the history row icon.
func (kind Kind) BaseKind() Kind {
	switch kind {
	case KindLink, KindCode:
		return KindText
	default:
		return kind
	}
}

// Filter selects a subset of history items.
type Filter uint8

const (
	FilterAll Filter = iota
	FilterText
	FilterLinks
	FilterCode
	FilterImages
	FilterFiles
	FilterFavorites
)

// Image stores the portable image payload needed by history and clipboard
// writes. UI bitmap objects stay outside this package.
type Image struct {
	PNG    []byte
	Width  int
	Height int
}

// Item is the data owned by the history module.
type Item struct {
	ID        string
	Kind      Kind
	Text      string
	Image     *Image
	Source    string
	CreatedAt time.Time
	Favorite  bool
}

// Query describes the current history view requested by the UI.
type Query struct {
	Text   string
	Filter Filter
	// KindTerms returns extra searchable names for a kind, such as the
	// localized labels the UI shows. The history module owns no display
	// vocabulary of its own, so callers supply it. It may be nil.
	KindTerms func(Kind) []string
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
	for _, item := range s.items {
		if !matchesFilter(item, query.Filter) || !matchesQuery(item, query) {
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

func matchesFilter(item Item, filter Filter) bool {
	switch filter {
	case FilterText:
		return item.Kind == KindText
	case FilterLinks:
		return item.Kind == KindLink
	case FilterCode:
		return item.Kind == KindCode
	case FilterImages:
		return item.Kind == KindImage
	case FilterFiles:
		return item.Kind == KindFile
	case FilterFavorites:
		return item.Favorite
	default:
		return true
	}
}

func matchesQuery(item Item, query Query) bool {
	text := strings.ToLower(strings.TrimSpace(query.Text))
	if text == "" {
		return true
	}
	parts := []string{item.Text, item.Source}
	if query.KindTerms != nil {
		parts = append(parts, query.KindTerms(item.Kind)...)
	}
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), text)
}

func cloneItem(item Item) Item {
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
