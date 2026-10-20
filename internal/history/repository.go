package history

import "context"

// Repository is the persistence seam for clipboard history. It stores the
// Item values this package owns, delivery records included, so one adapter
// covers both keeping history across runs and recording what each paired
// device has received. Filtering, ordering for display, and selection stay in
// the in-memory Store; a Repository only reads and writes records.
//
// The contract every implementation must honour:
//
//   - Put is an upsert keyed by Item.ID. An item with an empty ID is rejected
//     with ErrInvalidItem and stores nothing. When one call repeats an ID, the
//     last item for that ID wins.
//   - Put does not retain the caller's payloads: the Item, its Image.PNG and
//     its Deliveries may be mutated after the call returns.
//   - Delete is idempotent; removing an unknown ID is not an error. Deleting
//     an item also removes its delivery records.
//   - Load returns every stored item, newest first. Items that share a
//     CreatedAt keep a deterministic order across runs.
//   - An item with no image and an item whose Image holds an empty PNG both
//     load back with a nil Image, and an item with no deliveries loads back
//     with nil Deliveries.
//   - Text and HTML are stored verbatim and load back byte for byte, so a
//     rich-text item keeps the markup beside its plain-text representation.
//   - CreatedAt is stored with millisecond precision in UTC, so a loaded item
//     compares equal in time but not necessarily in location or sub-millisecond
//     offset. Kind, Sync and DeliveryState are stored as integers and are
//     returned as-is, so a record written by a newer build does not fail to
//     load.
//
// Implementations must be safe for concurrent use by multiple goroutines.
type Repository interface {
	// Load returns every stored item, newest first.
	Load(ctx context.Context) ([]Item, error)
	// Put inserts or replaces items and their delivery records.
	Put(ctx context.Context, items ...Item) error
	// Delete removes items and their delivery records by ID.
	Delete(ctx context.Context, ids ...string) error
	// Close releases the resources the repository holds.
	Close() error
}
