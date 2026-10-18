package history

import "time"

// RetentionPolicy is the automatic cleanup rule for history: how many records
// to keep and how old a record may become before it is dropped. A field left at
// zero means that dimension is unlimited, so the zero policy keeps everything
// and is what a caller with no opinion should pass.
//
// Favorites sit outside the policy in both dimensions: a starred record is
// never dropped automatically, and it does not count toward MaxItems, so a
// history full of favorites still keeps MaxItems ordinary records instead of
// deleting each new capture the moment it arrives.
type RetentionPolicy struct {
	// MaxItems caps how many non-favorite records are kept, newest first.
	MaxItems int
	// MaxDays caps how old a non-favorite record may be.
	MaxDays int
}

// Unlimited reports whether the policy drops nothing at all.
func (p RetentionPolicy) Unlimited() bool { return p.MaxItems <= 0 && p.MaxDays <= 0 }

// ApplyRetention drops the records the policy no longer keeps and returns their
// IDs, so the caller can persist exactly the deletions that happened. It
// returns nil when the policy keeps everything.
//
// The age limit is applied first: a record past MaxDays goes even when the item
// budget still has room, so a history that went quiet keeps nothing ancient.
// The item limit then keeps the newest MaxItems ordinary records and drops the
// older ones. Favorites are copied through untouched and are not counted.
//
// When the dropped records include the selected one, selection moves to the
// newest record that remains, or to nothing when the store ends up empty: a
// prune is not a user action on one record, so it has no "next to it" to land
// on the way Delete does.
func (s *Store) ApplyRetention(policy RetentionPolicy) []string {
	if policy.Unlimited() || len(s.items) == 0 {
		return nil
	}

	var cutoff time.Time
	if policy.MaxDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -policy.MaxDays)
	}

	// The store keeps its items newest first, so one pass both counts the
	// ordinary records that stay and decides which of them are too old.
	kept := make([]Item, 0, len(s.items))
	var removed []string
	ordinary := 0
	for _, item := range s.items {
		switch {
		case item.Favorite:
			kept = append(kept, item)
		case policy.MaxDays > 0 && item.CreatedAt.Before(cutoff):
			removed = append(removed, item.ID)
		case policy.MaxItems > 0 && ordinary >= policy.MaxItems:
			removed = append(removed, item.ID)
		default:
			ordinary++
			kept = append(kept, item)
		}
	}
	if len(removed) == 0 {
		return nil
	}

	s.items = kept
	s.selectFirstRemaining()
	return removed
}

// selectFirstRemaining keeps selection on an item that still exists, moving it
// to the newest record when the selected one was dropped.
func (s *Store) selectFirstRemaining() {
	if len(s.items) == 0 {
		s.selectedID = ""
		return
	}
	for _, item := range s.items {
		if item.ID == s.selectedID {
			return
		}
	}
	s.selectedID = s.items[0].ID
}
