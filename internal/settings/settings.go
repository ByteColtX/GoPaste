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
