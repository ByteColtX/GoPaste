package lansync

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"time"
)

const (
	// defaultPort is the port this device prefers to accept connections on. It
	// is fixed so a firewall rule stays valid across runs; when it is taken the
	// device falls back to a free one and advertises that instead.
	defaultPort = 47823
	// certificateLifetime is how long the self-signed certificate claims to be
	// valid. Nothing verifies it against a clock or a chain — peers pin the key
	// instead — so a long lifetime only avoids a pointless renewal.
	certificateLifetime = 100 * 365 * 24 * time.Hour
	// dialTimeout bounds one connection attempt. A paired device that is
	// unreachable fails fast instead of holding a delivery open.
	dialTimeout = 5 * time.Second
	// requestTimeout bounds one whole exchange, including a large image.
	requestTimeout = 30 * time.Second
)

// ErrUntrustedPeer reports a connection from a key that is not pinned to any
// paired device.
var ErrUntrustedPeer = errors.New("lansync: peer key is not paired with this device")

// certificate is this device's TLS certificate: a self-signed certificate over
// the identity key. Nothing signs it and no authority is consulted; peers pin
// the key it carries, which is what makes it trustworthy between two paired
// devices and useless to anyone else.
func (i Identity) certificate() (tls.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("lansync: certificate serial: %w", err)
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: i.ID},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(certificateLifetime),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{i.ID},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, i.Public, i.private)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("lansync: create certificate: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  i.private,
	}, nil
}

// certificateKey returns the ed25519 public key a presented certificate carries.
// Any other key type is refused: an identity is an ed25519 key, so a peer
// presenting something else is not a GoPaste device.
func certificateKey(der []byte) (ed25519.PublicKey, error) {
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("lansync: parse peer certificate: %w", err)
	}
	public, ok := certificate.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: certificate key is %T, not ed25519", ErrUntrustedPeer, certificate.PublicKey)
	}
	return public, nil
}

// Trust resolves a pinned key to the device that owns it. The server consults it
// for every connection and the client consults it before dialling, so a device
// removed from the pairing list stops being reachable in both directions.
type Trust func(public ed25519.PublicKey) (Device, bool)

// serverTLSConfig is the configuration for accepting connections: TLS 1.3 only
// and this device's certificate. A client certificate is required, and its key
// must be an ed25519 key — an identity is one, so anything else is not a GoPaste
// device.
//
// RequireAnyClientCert is what lets a peer be checked without a certificate
// authority: the standard chain verification is skipped, so a self-signed
// certificate is not rejected for lacking an issuer. Requiring the certificate
// here also makes TLS itself prove the peer holds the matching private key;
// whether that key belongs to a *paired* device is decided per request by
// requirePaired, which is what keeps pairing possible while everything else
// stays pinned.
func serverTLSConfig(identity Identity) (*tls.Config, error) {
	certificate, err := identity.certificate()
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAnyClientCert,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return ErrUntrustedPeer
			}
			_, err := certificateKey(rawCerts[0])
			return err
		},
	}, nil
}

// clientTLSConfig is the configuration for connecting to one paired device: TLS
// 1.3 only, this device's certificate, and the peer's pinned key required. The
// standard verification is replaced rather than supplemented, because there is
// no certificate authority to verify against — the pin is the whole check.
func clientTLSConfig(identity Identity, device Device) (*tls.Config, error) {
	if len(device.PublicKey) != ed25519.PublicKeySize {
		return nil, ErrUntrustedPeer
	}
	certificate, err := identity.certificate()
	if err != nil {
		return nil, err
	}
	expected := device.PublicKey
	return &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return ErrUntrustedPeer
			}
			public, err := certificateKey(rawCerts[0])
			if err != nil {
				return err
			}
			if !public.Equal(expected) {
				return fmt.Errorf("%w: presented key is not the pinned one", ErrUntrustedPeer)
			}
			return nil
		},
	}, nil
}

// peerFromRequest returns the paired device that made an authenticated request.
// The TLS layer has already established that the peer holds the key its
// certificate carries; this resolves the key back to the device record the rest
// of the package works with.
func peerFromRequest(r *http.Request, trust Trust) (Device, error) {
	public, err := presentedKey(r)
	if err != nil {
		return Device{}, err
	}
	device, ok := trust(public)
	if !ok {
		return Device{}, ErrUntrustedPeer
	}
	return device, nil
}

// presentedKey returns the identity key the peer's certificate carries. TLS has
// already proved the peer holds the matching private key.
func presentedKey(r *http.Request) (ed25519.PublicKey, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return nil, ErrUntrustedPeer
	}
	return certificateKey(r.TLS.PeerCertificates[0].Raw)
}

// peerKey is the context key carrying the paired device behind a request.
type peerKey struct{}

// requirePaired wraps a handler so it only runs for a paired device. Every
// handler except pairing goes through it, so a forgotten check cannot leave a
// clipboard endpoint reachable from an unpaired peer.
func requirePaired(trust Trust, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		device, err := peerFromRequest(r, trust)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), peerKey{}, device)))
	}
}

// pairedPeer returns the device requirePaired resolved for this request.
func pairedPeer(r *http.Request) (Device, bool) {
	device, ok := r.Context().Value(peerKey{}).(Device)
	return device, ok
}

// newHTTPClient builds the client used to reach paired devices. It is
// configured once and reused, so connections to a device stay open between
// deliveries.
func newHTTPClient(identity Identity, resolve func(addr string) (Device, bool)) *http.Client {
	return &http.Client{
		Timeout:       requestTimeout,
		Transport:     newPinnedTransport(identity, resolve),
		CheckRedirect: noRedirect,
	}
}

// newFileClient builds the client a file upload uses. It shares the pinning and
// connection pooling with newHTTPClient but carries no overall timeout: a large
// file legitimately takes minutes, and the upload sets its own deadline.
func newFileClient(identity Identity, resolve func(addr string) (Device, bool)) *http.Client {
	return &http.Client{
		Transport:     newPinnedTransport(identity, resolve),
		CheckRedirect: noRedirect,
	}
}

// noRedirect refuses to follow a redirect. A paired device never redirects;
// following one would move the connection somewhere the pin was not checked for.
func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// newPinnedTransport is the shared transport behind both clients.
//
// Dialling is where the pin is applied: the dialled address is resolved back to
// a paired device, and the connection's TLS configuration then requires exactly
// that device's key. One shared transport therefore pools connections per peer
// while every connection stays pinned to the peer it was opened for.
func newPinnedTransport(identity Identity, resolve func(addr string) (Device, bool)) *http.Transport {
	dialer := &net.Dialer{Timeout: dialTimeout}
	return &http.Transport{
		DialContext:         dialer.DialContext,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     90 * time.Second,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			device, ok := resolve(addr)
			if !ok {
				return nil, fmt.Errorf("%w: %s is not a paired device address", ErrUntrustedPeer, addr)
			}
			config, err := clientTLSConfig(identity, device)
			if err != nil {
				return nil, err
			}
			tlsDialer := &tls.Dialer{NetDialer: dialer, Config: config}
			return tlsDialer.DialContext(ctx, network, addr)
		},
	}
}

// listen opens the listener this device accepts connections on. port is the
// preferred one; 0 asks for a free port, and a taken port falls back to a free
// one. It returns the port it actually got: the announcement carries it, so
// peers find the device either way.
func listen(port int) (net.Listener, int, error) {
	if port < 0 {
		port = 0
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("", fmt.Sprint(port)))
	if err == nil {
		return listener, listener.Addr().(*net.TCPAddr).Port, nil
	}
	if port == 0 {
		return nil, 0, fmt.Errorf("lansync: listen: %w", err)
	}
	listener, err = net.Listen("tcp", ":0")
	if err != nil {
		return nil, 0, fmt.Errorf("lansync: listen: %w", err)
	}
	return listener, listener.Addr().(*net.TCPAddr).Port, nil
}
