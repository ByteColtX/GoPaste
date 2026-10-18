// Package filescache decides which received files the cache may keep. Two rules
// work together: an age limit, and a quota measured on disk. The package touches
// the filesystem only to measure a directory; it never deletes anything itself,
// so the caller stays in charge of the history records that own those files.
//
// It depends on nothing but the standard library, so the rules can be tested on
// their own.
package filescache

import (
	"io/fs"
	"path/filepath"
	"sort"
	"time"
)

// Entry is one received file record: where its files live, how much disk they
// take, and when it arrived. Bytes is filled in by Measure; a caller may leave
// it zero when it already knows the size.
type Entry struct {
	// ID is the history record's content ID, which is also the name of the
	// directory holding the files.
	ID string
	// Dir is the record's directory. An empty or missing directory counts as
	// zero bytes, so a half-cleaned cache does not turn into an error.
	Dir string
	// Bytes is the directory's size on disk.
	Bytes int64
	// ReceivedAt is when the record arrived on this device.
	ReceivedAt time.Time
}

// Policy is what the cache may keep. Either limit may be off: MaxAge at or
// below zero means files never expire, QuotaBytes at or below zero means the
// cache has no size limit.
type Policy struct {
	QuotaBytes int64
	MaxAge     time.Duration
}

// Measure returns the entries with Bytes filled in from their directories.
func Measure(entries []Entry) []Entry {
	measured := make([]Entry, len(entries))
	for index, entry := range entries {
		entry.Bytes = DirectoryBytes(entry.Dir)
		measured[index] = entry
	}
	return measured
}

// Plan returns the IDs to delete, oldest first: the entries past the age limit,
// then — while the total is over the quota — the oldest of what is left.
//
// Expired entries go before the quota is even considered: they are gone by the
// user's own rule, and counting their bytes would delete extra records that are
// still inside the age limit.
func Plan(entries []Entry, policy Policy, now time.Time) []string {
	expired := make([]Entry, 0, len(entries))
	kept := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if policy.MaxAge > 0 && now.Sub(entry.ReceivedAt) > policy.MaxAge {
			expired = append(expired, entry)
			continue
		}
		kept = append(kept, entry)
	}
	sortOldestFirst(expired)
	sortOldestFirst(kept)

	removed := make([]string, 0, len(entries))
	for _, entry := range expired {
		removed = append(removed, entry.ID)
	}
	var total int64
	for _, entry := range kept {
		total += entry.Bytes
	}
	for _, entry := range kept {
		if policy.QuotaBytes <= 0 || total <= policy.QuotaBytes {
			break
		}
		total -= entry.Bytes
		removed = append(removed, entry.ID)
	}
	return removed
}

// DirectoryBytes returns the sum of every regular file under dir. It does not
// follow symbolic links: what a link points at is not inside this directory, so
// counting it would make the number disagree with the directory's real size.
// An unreadable directory counts as zero: this is a display and cleanup figure,
// not an error state.
func DirectoryBytes(dir string) int64 {
	if dir == "" {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Skip a subtree we cannot read and keep counting the rest.
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// sortOldestFirst orders entries by arrival, with the ID breaking ties so the
// same cache always yields the same plan.
func sortOldestFirst(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].ReceivedAt.Equal(entries[j].ReceivedAt) {
			return entries[i].ReceivedAt.Before(entries[j].ReceivedAt)
		}
		return entries[i].ID < entries[j].ID
	})
}
