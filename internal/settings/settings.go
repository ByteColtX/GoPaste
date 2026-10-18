// Package settings stores the few choices GoPaste keeps across runs in one
// small JSON file beside the history database. It depends on neither MyGo nor
// the network, so it can be tested on its own.
//
// The file is deliberately small: the history database owns clipboard data,
// and everything about the interface that is not listed here is still rebuilt
// on every launch.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// File is the whole of settings.json. Missing fields mean the default, so a
// file written by an older build still loads.
type File struct {
	Onboarding     Onboarding      `json:"onboarding"`
	DeviceName     string          `json:"deviceName,omitempty"`
	PendingPairing *PendingPairing `json:"pendingPairing,omitempty"`
}

// Onboarding records what the first-run wizard came to. Completed covers the
// skipped path too: once the user has dealt with the wizard, it does not come
// back on its own.
type Onboarding struct {
	Completed bool `json:"completed"`
}

// PendingPairing is a pairing code the user entered on a device that already
// runs GoPaste. Nothing completes it yet: the LAN pairing service is not
// implemented, so this is a request waiting for one, never a paired device.
type PendingPairing struct {
	// Code is the code as the user entered it, normalized to digits only. It
	// is kept verbatim because the pairing handshake will need the same value
	// the other device shows; the file is private to the user (0600).
	Code string `json:"code"`
	// CreatedAt is when the request was recorded, in UTC.
	CreatedAt time.Time `json:"createdAt"`
}

// PairingCodeLength is how many digits a pairing code has.
const PairingCodeLength = 6

// PairingCode normalizes a pairing code typed by the user: spaces and hyphens
// are dropped, because people copy codes as "482 913", and what remains must be
// exactly PairingCodeLength ASCII digits. It reports false for anything else.
func PairingCode(raw string) (string, bool) {
	digits := make([]rune, 0, PairingCodeLength)
	for _, r := range raw {
		switch {
		case r == ' ' || r == '\t' || r == '-' || r == '\u00a0':
			continue
		case r < '0' || r > '9':
			return "", false
		}
		digits = append(digits, r)
	}
	if len(digits) != PairingCodeLength {
		return "", false
	}
	return string(digits), true
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
