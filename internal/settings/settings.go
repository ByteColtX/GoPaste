// Package settings stores the few choices GoPaste keeps across runs in one
// small JSON file beside the history database. It depends on neither MyGo nor
// the network, so it can be tested on its own.
//
// The file is deliberately small: the history database owns clipboard data, and
// the interface preferences it does keep are only the ones the settings page
// exposes.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// File is the whole of settings.json. Missing fields mean the default, so a
// file written by an older build still loads.
type File struct {
	Onboarding Onboarding `json:"onboarding"`
	DeviceName string     `json:"deviceName,omitempty"`
	// UI is nil while every interface preference is still the default: the
	// caller passes nil so a file that never saw a change stays small.
	UI *UI `json:"ui,omitempty"`
	// Retention is the history cleanup the user chose. A nil pointer means the
	// user never touched it, and the application applies its own defaults; a
	// present object records exactly what was chosen.
	Retention *Retention `json:"retention,omitempty"`
	// FileSyncMaxMB is the size limit for one file item delivered to a paired
	// device, in MiB. Absent or below MinFileSyncMaxMB means
	// DefaultFileSyncMaxMB: a limit of zero would mean "send nothing", which is
	// not a value this file expresses.
	FileSyncMaxMB int `json:"fileSyncMaxMB,omitempty"`
	// Sync is the sync page's settings. It is nil while every one of them is
	// still the default, so a file that never saw a change stays small.
	Sync *Sync `json:"sync,omitempty"`
}

// Sync is the sync page: the master switch, the automatic sending of new
// clipboard content, and the file sync block. Every switch defaults to on, so
// an absent field reads back as on; the pointers keep "never changed" apart
// from an explicit off.
type Sync struct {
	Enabled  *bool      `json:"enabled,omitempty"`
	AutoSend *bool      `json:"autoSend,omitempty"`
	Files    *SyncFiles `json:"files,omitempty"`
}

// SyncFiles is the file sync block: its own switch plus where received files
// land and how long the cache may grow.
type SyncFiles struct {
	// Enabled is file sync's own switch: off stops file content in both
	// directions while text, images and rich text keep flowing.
	Enabled *bool `json:"enabled,omitempty"`
	// Dir is the directory received files are stored in. Empty means the
	// application's default location beside the history database.
	Dir string `json:"dir,omitempty"`
	// QuotaMB bounds the received-file cache in MiB. Absent or below
	// MinFileQuotaMB means DefaultFileQuotaMB; there is no "unlimited" value,
	// because a quota of zero would mean "keep nothing".
	QuotaMB int `json:"quotaMB,omitempty"`
	// RetentionHours is how long a received file is kept. A nil pointer means
	// the user never chose, so the application's default applies; a present 0
	// means "keep until the history record goes" (no age limit). Both cases
	// must be distinguishable, exactly like Retention's two dimensions.
	RetentionHours *int `json:"retentionHours,omitempty"`
	// AutoClean is the switch that lets the retention limit delete expired
	// files on its own. Off leaves them alone until the user cleans manually.
	AutoClean *bool `json:"autoClean,omitempty"`
}

// UI is the interface preferences the settings page records. Every zero value
// is that preference's default, so the caller leaves defaults out and a missing
// field reads back as "never changed".
type UI struct {
	// Language is a pinned interface language ("zh-Hans" or "en"); empty
	// follows the system locale.
	Language string `json:"language,omitempty"`
	// Appearance is "light" or "dark"; empty follows the system appearance.
	Appearance string `json:"appearance,omitempty"`
	// ConfirmDelete is false only once the user turned the confirmation off.
	// The pointer keeps "never changed" (default: on) apart from an explicit
	// false.
	ConfirmDelete *bool `json:"confirmDelete,omitempty"`
	// Zoom is the interface scale, where 1 is 100%; zero is the default.
	Zoom float32 `json:"zoom,omitempty"`
}

// Retention is one saved history cleanup rule: how many records to keep and
// how old a record may become. Each limit is optional. A nil pointer means the
// user never chose that dimension, so the application keeps using its own
// default for it; a present 0 means the user chose "no limit". Both cases must
// be distinguishable, which is why these are pointers rather than plain ints:
// with ints an untouched file would read back as "unlimited" and silently
// switch the policy off for every existing user.
type Retention struct {
	MaxItems *int `json:"maxItems,omitempty"`
	MaxDays  *int `json:"maxDays,omitempty"`
}

// The file-sync size limit. The default is what one LAN transfer finishes
// within minutes and one disk absorbs, and it covers ordinary clipboard use.
const (
	DefaultFileSyncMaxMB = 2048
	MinFileSyncMaxMB     = 1
)

// The received-file cache: how much disk it may take, and how long one file
// stays before it counts as expired. The quota is in the same league as the
// per-file limit, so one maximum-size file fits inside it.
const (
	DefaultFileQuotaMB        = 2048
	MinFileQuotaMB            = 1
	DefaultFileRetentionHours = 24
)

// SyncEnabled reports whether sync is on. An absent object or field means on.
func (f File) SyncEnabled() bool { return onByDefault(f.sync().Enabled) }

// AutoSendEnabled reports whether new clipboard content is sent to other
// devices on its own. An absent field means on.
func (f File) AutoSendEnabled() bool { return onByDefault(f.sync().AutoSend) }

// FileSyncEnabled reports whether file content may travel. An absent field
// means on.
func (f File) FileSyncEnabled() bool { return onByDefault(f.files().Enabled) }

// FileAutoCleanEnabled reports whether expired files are deleted on their own.
// An absent field means on.
func (f File) FileAutoCleanEnabled() bool { return onByDefault(f.files().AutoClean) }

// FileSyncDir returns the configured directory for received files, or the empty
// string when the application's default location is used.
func (f File) FileSyncDir() string { return f.files().Dir }

// FileQuotaMB returns the effective cache quota in MiB.
func (f File) FileQuotaMB() int {
	megabytes := f.files().QuotaMB
	if megabytes < MinFileQuotaMB {
		return DefaultFileQuotaMB
	}
	return megabytes
}

// FileQuotaBytes returns the effective cache quota in bytes.
func (f File) FileQuotaBytes() int64 { return FileQuotaBytes(f.FileQuotaMB()) }

// FileRetentionHours returns how long a received file is kept, in hours; zero
// means no age limit. An absent value means DefaultFileRetentionHours.
func (f File) FileRetentionHours() int {
	if hours := f.files().RetentionHours; hours != nil {
		return *hours
	}
	return DefaultFileRetentionHours
}

// sync returns the sync object, or its zero value when the file never had one.
func (f File) sync() Sync {
	if f.Sync == nil {
		return Sync{}
	}
	return *f.Sync
}

// files returns the file sync object, or its zero value when absent.
func (f File) files() SyncFiles {
	if f.Sync == nil || f.Sync.Files == nil {
		return SyncFiles{}
	}
	return *f.Sync.Files
}

// onByDefault reads a switch whose absent value is on.
func onByDefault(value *bool) bool { return value == nil || *value }

// FileQuotaBytes converts a configured cache quota in MiB into bytes, applying
// the default when the number is not usable.
func FileQuotaBytes(megabytes int) int64 {
	if megabytes < MinFileQuotaMB {
		megabytes = DefaultFileQuotaMB
	}
	return megabytesToBytes(megabytes)
}

// FileSyncMaxBytes returns the effective limit in bytes: the configured number
// of MiB, or the default when the file does not name a usable one.
func (f File) FileSyncMaxBytes() int64 { return FileSyncMaxBytes(f.FileSyncMaxMB) }

// FileSyncMaxBytes converts a configured limit in MiB into bytes, applying the
// default when the number is not usable.
func FileSyncMaxBytes(megabytes int) int64 {
	if megabytes < MinFileSyncMaxMB {
		megabytes = DefaultFileSyncMaxMB
	}
	return megabytesToBytes(megabytes)
}

// megabytesToBytes converts MiB to bytes, clamping a number so large that the
// shift would wrap into a negative value.
func megabytesToBytes(megabytes int) int64 {
	if int64(megabytes) > math.MaxInt64>>20 {
		// A value this large is "no limit" in practice; clamping keeps the
		// conversion from wrapping into a negative limit.
		return math.MaxInt64
	}
	return int64(megabytes) << 20
}

// Onboarding records what the first-run wizard came to. Completed covers the
// skipped path too: once the user has dealt with the wizard, it does not come
// back on its own.
type Onboarding struct {
	Completed bool `json:"completed"`
}

// Load reads the settings at path. A file that does not exist is not an error:
// it means a first run, and Load returns the zero File so the caller can treat
// "no settings" and "default settings" the same way. A file that cannot be read
// or parsed is an error, and the caller decides what a damaged file means.
func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, nil
		}
		return File{}, fmt.Errorf("read settings: %w", err)
	}
	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("parse settings: %w", err)
	}
	return file, nil
}

// Save writes f to path, creating the directory if needed. The write goes to a
// temporary file in the same directory and is then renamed into place, so a
// crash mid-write cannot leave a half-written file that the next launch would
// read as a damaged one. The file is private to the current user.
func Save(path string, f File) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(dir, ".settings-*.json")
	if err != nil {
		return fmt.Errorf("create settings file: %w", err)
	}
	tempPath := temp.Name()
	// Any failure past this point must not leave the temporary file behind.
	defer func() {
		if tempPath != "" {
			os.Remove(tempPath)
		}
	}()

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write settings: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Chmod(tempPath, 0o600); err != nil {
		return fmt.Errorf("restrict settings file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace settings: %w", err)
	}
	tempPath = ""
	return nil
}
