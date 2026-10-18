package lansync

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopaste/internal/history"
)

// ContentType is the coarse payload category the per-device sync directions are
// kept for. It follows history.Kind.BaseKind, so a link or a code snippet
// travels as text: the four settings the device page shows are exactly these
// four values.
type ContentType uint8

const (
	ContentText ContentType = iota
	ContentImage
	ContentFile
	ContentRichText

	contentTypeCount = 4
)

// contentTypeNames are the stored names of the four types. They are part of the
// identity file, so they never change: the settings file outlives a release.
var contentTypeNames = [contentTypeCount]string{"text", "image", "file", "richText"}

// ContentTypeOf returns the setting a payload kind is gated by.
func ContentTypeOf(kind history.Kind) ContentType {
	switch kind.BaseKind() {
	case history.KindImage:
		return ContentImage
	case history.KindFile:
		return ContentFile
	case history.KindRichText:
		return ContentRichText
	default:
		return ContentText
	}
}

// String returns the stored name of the content type.
func (t ContentType) String() string {
	if int(t) >= len(contentTypeNames) {
		return contentTypeNames[ContentText]
	}
	return contentTypeNames[t]
}

// contentTypes lists every content type in settings order.
func contentTypes() []ContentType {
	return []ContentType{ContentText, ContentImage, ContentFile, ContentRichText}
}

// contentTypesByName resolves stored names back to their type. An unknown name
// is skipped rather than rejected: a file written by a later version must not
// stop this one from pairing.
func contentTypeByName(name string) (ContentType, bool) {
	for _, t := range contentTypes() {
		if contentTypeNames[t] == name {
			return t, true
		}
	}
	return 0, false
}

// Direction is one content type's sync direction with one paired device, as
// the device page shows it. Its zero value, DirectionTwoWay, is what a freshly
// paired device gets.
type Direction uint8

const (
	DirectionTwoWay Direction = iota
	DirectionSendOnly
	DirectionReceiveOnly
	DirectionOff
)

// AllowsSend reports whether the direction lets items leave this device.
func (d Direction) AllowsSend() bool { return d == DirectionTwoWay || d == DirectionSendOnly }

// AllowsReceive reports whether the direction lets items arrive from the peer.
func (d Direction) AllowsReceive() bool {
	return d == DirectionTwoWay || d == DirectionReceiveOnly
}

// Device is one paired device: the durable record peers pin, plus where it was
// last seen. The durable part is what the identity file holds; the presence
// fields are refreshed by discovery and by every exchange, and are never
// written to disk.
type Device struct {
	// ID is the peer ID, derived from PublicKey.
	ID string
	// Name is the device's own name, as it reported it. It is display only:
	// nothing is keyed by it, so a rename is just a new name.
	Name string
	// Platform is the device's platform name, such as "macOS".
	Platform string
	// PublicKey is the pinned key. A connection from this device is only
	// accepted when it proves possession of the matching private key.
	PublicKey ed25519.PublicKey
	// SyncEnabled is the device page's master switch for this device.
	SyncEnabled bool
	// Directions is indexed by ContentType.
	Directions [contentTypeCount]Direction
	// PairedAt is when the pairing completed.
	PairedAt time.Time

	// Addr and Port are where discovery last found the device. They are
	// meaningless while Online is false.
	Addr net.IP
	Port int
	// Online reports whether discovery saw the device recently.
	Online bool
	// LastSeen is the last discovery sighting, or the last successful
	// exchange, whichever is later.
	LastSeen time.Time
}

// Direction returns the direction for one content type.
func (d Device) Direction(t ContentType) Direction { return d.Directions[t] }

// Sends reports whether an item of type t should be delivered to this device:
// the device switch is on and the type's direction lets items leave.
func (d Device) Sends(t ContentType) bool {
	return d.SyncEnabled && d.Direction(t).AllowsSend()
}

// Receives reports whether an item of type t arriving from this device is
// accepted: the device switch is on and the type's direction lets items in.
func (d Device) Receives(t ContentType) bool {
	return d.SyncEnabled && d.Direction(t).AllowsReceive()
}

// ErrStoreUnreadable reports an identity file that exists but cannot be parsed.
// It is never overwritten: the pairing data in it is not reproducible.
var ErrStoreUnreadable = errors.New("lansync: identity file is unreadable")

// storeVersion is the identity file's format version. It is written into the
// file so a later format can be recognised instead of misread.
const storeVersion = 1

// Store is the durable side of the package: this device's identity and the
// devices it has paired with, in one file the user owns.
//
// The file holds a private key and the pinned keys of paired devices, so it is
// created 0600 and replaced atomically: a crash mid-write leaves the previous
// contents rather than half a file. It is safe for concurrent use.
type Store struct {
	mu       sync.RWMutex
	path     string
	name     string
	platform string
	identity Identity
	devices  []Device
}

// OpenStore opens the identity file at path, creating it with a fresh identity
// when it does not exist yet. name and platform describe this device to peers;
// a non-empty name replaces what the file held, because the application owns
// device naming, while an empty one keeps the stored value.
//
// A file that exists but cannot be read returns an error wrapping
// ErrStoreUnreadable and is left untouched.
func OpenStore(path, name, platform string) (*Store, error) {
	if path == "" {
		return nil, errors.New("lansync: identity file path is empty")
	}
	store := &Store{path: path, name: name, platform: platform}

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		identity, err := NewIdentity()
		if err != nil {
			return nil, err
		}
		store.identity = identity
		if err := store.save(); err != nil {
			return nil, err
		}
		return store, nil
	case err != nil:
		return nil, fmt.Errorf("lansync: read identity file: %w", err)
	}

	var file storeFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrStoreUnreadable, path, err)
	}
	if file.Version > storeVersion {
		return nil, fmt.Errorf("%w: %s: version %d is newer than this build supports (%d)",
			ErrStoreUnreadable, path, file.Version, storeVersion)
	}
	identity, err := IdentityFromPrivateKey(ed25519.PrivateKey(file.Identity.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrStoreUnreadable, path, err)
	}
	store.identity = identity
	store.devices = file.devices()
	if store.name == "" {
		store.name = file.Identity.Name
	}
	if store.platform == "" {
		store.platform = file.Identity.Platform
	}
	// Rewrite unconditionally: the caller's name and platform win, and a file
	// that predates a field gains it now rather than on the next change.
	if err := store.save(); err != nil {
		return nil, err
	}
	return store, nil
}

// Identity returns this device's identity.
func (s *Store) Identity() Identity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity
}

// Path returns the identity file's location.
func (s *Store) Path() string { return s.path }

// Name returns this device's name as peers see it.
func (s *Store) Name() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name
}

// Platform returns this device's platform name as peers see it.
func (s *Store) Platform() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.platform
}

// SetName renames this device and persists it, so the next announcement carries
// the new name.
func (s *Store) SetName(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.name == name {
		return nil
	}
	s.name = name
	return s.saveLocked()
}

// Devices returns a copy of the paired devices.
func (s *Store) Devices() []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Device(nil), s.devices...)
}

// Device returns one paired device by peer ID.
func (s *Store) Device(id string) (Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, device := range s.devices {
		if device.ID == id {
			return device, true
		}
	}
	return Device{}, false
}

// Trusted reports whether public belongs to a paired device, and which one. It
// is the pin lookup every incoming connection is checked against.
func (s *Store) Trusted(public ed25519.PublicKey) (Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, device := range s.devices {
		if device.PublicKey != nil && device.PublicKey.Equal(public) {
			return device, true
		}
	}
	return Device{}, false
}

// Put inserts or replaces one paired device and persists the result. Presence
// fields are kept as given: callers pass back what Devices returned.
func (s *Store) Put(device Device) error {
	if device.ID == "" {
		return errors.New("lansync: device ID is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := range s.devices {
		if s.devices[index].ID == device.ID {
			s.devices[index] = device
			return s.saveLocked()
		}
	}
	s.devices = append(s.devices, device)
	return s.saveLocked()
}

// Remove unpairs a device and persists the result. It returns the record it
// removed so a caller can offer to undo.
func (s *Store) Remove(id string) (Device, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, device := range s.devices {
		if device.ID != id {
			continue
		}
		s.devices = append(s.devices[:index], s.devices[index+1:]...)
		return device, true, s.saveLocked()
	}
	return Device{}, false, nil
}

// save persists the current contents. The caller holds no lock.
func (s *Store) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.write(s.snapshot())
}

// saveLocked persists the current contents. The caller holds the write lock,
// so it snapshots without taking it again.
func (s *Store) saveLocked() error { return s.write(s.snapshot()) }

// snapshot projects the current contents onto the file's shape. Callers hold
// at least the read lock.
func (s *Store) snapshot() storeFile {
	file := storeFile{
		Version: storeVersion,
		Identity: storeIdentity{
			PrivateKey: s.identity.PrivateKey(),
			Name:       s.name,
			Platform:   s.platform,
		},
		Devices: make([]storeDevice, 0, len(s.devices)),
	}
	for _, device := range s.devices {
		file.Devices = append(file.Devices, newStoreDevice(device))
	}
	return file
}

// write writes the identity file atomically: a temporary file in the same
// directory, then a rename, so a reader never sees a partial file.
func (s *Store) write(file storeFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("lansync: encode identity file: %w", err)
	}
	directory := filepath.Dir(s.path)
	// The directory holds a private key: only this user may look inside.
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("lansync: create identity directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".devices-*.json")
	if err != nil {
		return fmt.Errorf("lansync: write identity file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("lansync: write identity file: %w", err)
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return fmt.Errorf("lansync: write identity file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("lansync: write identity file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("lansync: write identity file: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("lansync: replace identity file: %w", err)
	}
	return nil
}

// storeFile is the identity file's on-disk shape.
type storeFile struct {
	Version  int           `json:"version"`
	Identity storeIdentity `json:"identity"`
	Devices  []storeDevice `json:"devices"`
}

// storeIdentity is this device's own record.
type storeIdentity struct {
	PrivateKey []byte `json:"privateKey"`
	Name       string `json:"name"`
	Platform   string `json:"platform"`
}

// storeDevice is one paired device's durable record. Presence is deliberately
// absent: where a device was last seen is a fact about this run, not a fact
// worth keeping across one.
type storeDevice struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Platform    string               `json:"platform"`
	PublicKey   []byte               `json:"publicKey"`
	SyncEnabled bool                 `json:"syncEnabled"`
	Directions  map[string]Direction `json:"directions"`
	PairedAt    time.Time            `json:"pairedAt"`
}

// newStoreDevice projects a device onto its durable form.
func newStoreDevice(device Device) storeDevice {
	record := storeDevice{
		ID:          device.ID,
		Name:        device.Name,
		Platform:    device.Platform,
		PublicKey:   device.PublicKey,
		SyncEnabled: device.SyncEnabled,
		Directions:  make(map[string]Direction, contentTypeCount),
		PairedAt:    device.PairedAt,
	}
	for _, t := range contentTypes() {
		record.Directions[t.String()] = device.Directions[t]
	}
	return record
}

// devices turns the file's records back into devices. A record with an
// unusable key is dropped: it could never authenticate anything.
func (f storeFile) devices() []Device {
	devices := make([]Device, 0, len(f.Devices))
	for _, record := range f.Devices {
		if len(record.PublicKey) != ed25519.PublicKeySize {
			continue
		}
		device := Device{
			ID:          record.ID,
			Name:        record.Name,
			Platform:    record.Platform,
			PublicKey:   ed25519.PublicKey(record.PublicKey),
			SyncEnabled: record.SyncEnabled,
			PairedAt:    record.PairedAt,
		}
		if device.ID == "" {
			device.ID = PeerID(device.PublicKey)
		}
		for name, direction := range record.Directions {
			if t, ok := contentTypeByName(name); ok {
				device.Directions[t] = direction
			}
		}
		devices = append(devices, device)
	}
	return devices
}
