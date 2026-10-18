// Package lansync connects GoPaste instances on the same local network: it
// finds the other devices, pairs with them, and delivers clipboard items to
// them directly.
//
// It talks only over the local network. There is no relay, no server outside
// the two devices, and no fallback connection of any kind: a pair of devices
// that cannot reach each other on the LAN simply does not sync. Clipboard
// payloads are sensitive, so the package keeps them out of logs and carries
// every byte over a mutually authenticated TLS 1.3 connection.
//
// The package depends on no UI toolkit, so it runs and is tested on its own.
// The application reaches it through Service alone, and publishes its results
// to the UI thread itself.
package lansync

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidIdentity is returned when stored key material cannot be used as an
// identity, as when it has the wrong length.
var ErrInvalidIdentity = errors.New("lansync: invalid identity key")

// peerIDEncoding spells a peer ID: base32 without padding, lowercased for
// reading aloud and for typing into a support message.
var peerIDEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// PeerID returns the stable ID of the device holding public: 16 lowercase
// base32 characters derived from the key's SHA-256. The ID is derived rather
// than stored, so it cannot drift from the key it names, and a device keeps it
// for life because it keeps the key.
func PeerID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return strings.ToLower(peerIDEncoding.EncodeToString(sum[:10]))
}

// Identity is this device's long-lived network identity: the key pair peers
// pin when they pair with it, and the ID they know it by.
//
// The private key never leaves the machine and is never logged. Pairing pins
// the public half, and every later connection proves possession of the private
// half, which is what makes a paired device recognisable without any
// certificate authority.
type Identity struct {
	private ed25519.PrivateKey
	// Public is the key peers pin. It is the identity's only public half.
	Public ed25519.PublicKey
	// ID is PeerID(Public).
	ID string
}

// NewIdentity generates a fresh identity from the system's random source.
func NewIdentity() (Identity, error) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, fmt.Errorf("lansync: generate identity: %w", err)
	}
	return identityFrom(private), nil
}

// IdentityFromPrivateKey rebuilds an identity from stored key material. The
// key must be a full ed25519 private key: its 32-byte seed followed by the
// public half.
func IdentityFromPrivateKey(key ed25519.PrivateKey) (Identity, error) {
	if len(key) != ed25519.PrivateKeySize {
		return Identity{}, fmt.Errorf("%w: %d bytes, want %d", ErrInvalidIdentity, len(key), ed25519.PrivateKeySize)
	}
	return identityFrom(key), nil
}

// identityFrom derives the identity from a private key already validated for
// length.
func identityFrom(private ed25519.PrivateKey) Identity {
	public := private.Public().(ed25519.PublicKey)
	return Identity{
		private: private,
		Public:  public,
		ID:      PeerID(public),
	}
}

// PrivateKey returns the key material to store. The result is the caller's:
// it belongs in the identity file and nowhere else.
func (i Identity) PrivateKey() ed25519.PrivateKey { return i.private }
