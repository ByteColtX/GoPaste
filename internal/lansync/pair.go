package lansync

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// pairProtocolVersion is the pairing protocol this build speaks. Two devices
// that disagree refuse to pair instead of half-working.
const pairProtocolVersion = "1"

const (
	// pairTimeout is how long a pairing request waits for the user to type the
	// code on the other device.
	pairTimeout = 60 * time.Second
	// pairKeySize is an X25519 public key's length.
	pairKeySize = 32
	// pairCodeLength is how many digits the pairing code has. Six digits are
	// short enough to retype and give a one-in-a-million chance of a wrong
	// code being accepted, on a single attempt per session.
	pairCodeLength = 6
	// pairNameLimit bounds a device name that arrives over the wire, so a peer
	// cannot push an unbounded string into the device list.
	pairNameLimit = 64
)

// HTTP paths of the pairing exchange and of clipboard delivery.
const (
	pathPairHello  = "/v1/pair/hello"
	pathPairWait   = "/v1/pair/wait"
	pathPairCancel = "/v1/pair/cancel"
	pathClip       = "/v1/clip"
)

// Pairing outcomes. They travel between the two devices, so they are part of
// the wire contract.
const (
	pairStatusConfirmed = "confirmed"
	pairStatusAborted   = "aborted"

	pairReasonWrongCode = "wrong-code"
	pairReasonTimeout   = "timeout"
	pairReasonCanceled  = "canceled"
	pairReasonVersion   = "version"
	pairReasonBusy      = "busy"
	pairReasonInternal  = "internal"
)

// Pairing errors, as the application sees them.
var (
	// ErrPairingWrongCode means the code typed on the other device did not
	// match the one this device showed.
	ErrPairingWrongCode = errors.New("lansync: the pairing code did not match")
	// ErrPairingTimeout means nobody typed the code in time.
	ErrPairingTimeout = errors.New("lansync: pairing timed out")
	// ErrPairingCanceled means the pairing was given up.
	ErrPairingCanceled = errors.New("lansync: pairing was canceled")
	// ErrPairingVersion means the two devices speak different pairing
	// protocols, as when one of them is a different version.
	ErrPairingVersion = errors.New("lansync: the other device speaks a different pairing protocol")
	// ErrPairingBusy means the other device is already pairing with someone.
	ErrPairingBusy = errors.New("lansync: the other device is already pairing")
	// ErrPairingProof means the other device could not prove it holds the
	// shared secret, so the session is not trustworthy.
	ErrPairingProof = errors.New("lansync: the other device failed to prove the pairing")
)

// pairTranscriptTag domain-separates the pairing transcript from any other use
// of the same key material.
const pairTranscriptTag = "gopaste/pair/v1\x00"

// PairRequest is a device asking to pair with this one. It is what the
// application shows in the pairing dialog, and what the user's typed code is
// checked against.
type PairRequest struct {
	// PeerID identifies the requesting device.
	PeerID string
	// Name is the name the request carries. It is display only.
	Name string
	// Platform is the requesting device's platform name.
	Platform string
}

// pairHello is the first message of a pairing, sent by the device that shows
// the code. Its static key is not in the body: it travels in the TLS
// certificate, which TLS has already proved the device holds.
type pairHello struct {
	Version   string `json:"version"`
	Ephemeral []byte `json:"ephemeral"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
}

// pairHelloReply answers a hello with the responder's ephemeral key.
type pairHelloReply struct {
	Version   string `json:"version"`
	Ephemeral []byte `json:"ephemeral"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
}

// pairStatus is how a pairing session ended, and the proof that lets the
// initiating device trust the outcome.
type pairStatus struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Proof  []byte `json:"proof,omitempty"`
}

// pairTranscript binds both static keys and both ephemeral keys into one byte
// string. Every derived value depends on all four, so a man in the middle who
// terminates the connection on each side ends up with a different transcript —
// and therefore a different code — than the devices themselves.
func pairTranscript(initiator, responder ed25519.PublicKey, initiatorEphemeral, responderEphemeral []byte) []byte {
	transcript := make([]byte, 0, len(pairTranscriptTag)+2*ed25519.PublicKeySize+2*pairKeySize)
	transcript = append(transcript, pairTranscriptTag...)
	transcript = append(transcript, initiator...)
	transcript = append(transcript, responder...)
	transcript = append(transcript, initiatorEphemeral...)
	transcript = append(transcript, responderEphemeral...)
	return transcript
}

// pairSecret derives the code the user compares and the proof the responder
// sends back, both from the X25519 shared secret and the transcript.
//
// The code is the value that authenticates the session: it never crosses the
// network, so only a device that computed the same shared secret shows or
// accepts the same digits.
func pairSecret(shared, transcript []byte) (code string, proof []byte, err error) {
	codeBytes, err := hkdf.Key(sha256.New, shared, transcript, "gopaste/pair/v1 sas", 4)
	if err != nil {
		return "", nil, fmt.Errorf("lansync: derive pairing code: %w", err)
	}
	digits := binary.BigEndian.Uint32(codeBytes) % 1_000_000
	code = fmt.Sprintf("%0*d", pairCodeLength, digits)

	proofKey, err := hkdf.Key(sha256.New, shared, transcript, "gopaste/pair/v1 confirm", 32)
	if err != nil {
		return "", nil, fmt.Errorf("lansync: derive pairing proof: %w", err)
	}
	mac := hmac.New(sha256.New, proofKey)
	mac.Write(transcript)
	return code, mac.Sum(nil), nil
}

// pairError maps a wire reason back to the error the application reports.
func pairError(reason string) error {
	switch reason {
	case pairReasonWrongCode:
		return ErrPairingWrongCode
	case pairReasonTimeout:
		return ErrPairingTimeout
	case pairReasonCanceled:
		return ErrPairingCanceled
	case pairReasonVersion:
		return ErrPairingVersion
	case pairReasonBusy:
		return ErrPairingBusy
	default:
		return ErrPairingCanceled
	}
}

// pairReason maps an error onto its wire reason.
func pairReason(err error) string {
	switch {
	case errors.Is(err, ErrPairingWrongCode):
		return pairReasonWrongCode
	case errors.Is(err, ErrPairingTimeout):
		return pairReasonTimeout
	case errors.Is(err, ErrPairingVersion):
		return pairReasonVersion
	case errors.Is(err, ErrPairingBusy):
		return pairReasonBusy
	case errors.Is(err, ErrPairingCanceled):
		return pairReasonCanceled
	default:
		return pairReasonInternal
	}
}

// pairSession is one pairing in progress on the responding device.
type pairSession struct {
	// peer is the requesting device as its TLS certificate identified it. The
	// name and platform are unverified: they are what the other device says
	// about itself, and only the user seeing them can judge them.
	peer     Device
	code     string
	proof    []byte
	deadline time.Time
	done     chan struct{}
	// resolved is set when the session ends, so a second confirmation cannot
	// pair the same device twice. It is guarded by pairingServer.mu.
	resolved bool
	// err is set before done closes, so a reader that saw done closed reads a
	// final value.
	err error
}

// pairingServer is the responding side: it accepts pairing requests and holds
// at most one at a time, so two people cannot race a code check on one device.
type pairingServer struct {
	store *Store
	now   func() time.Time

	// onRequest tells the application to show the pairing dialog. It runs on
	// the HTTP goroutine.
	onRequest func(PairRequest)
	// onPaired reports a completed pairing. It runs on the HTTP goroutine.
	onPaired func(Device)
	// onFailed reports a pairing that did not complete. It runs on the HTTP
	// goroutine.
	onFailed func(error)

	// timeout is how long a request waits for the user; zero means
	// pairTimeout. Tests shorten it.
	timeout time.Duration

	mu      sync.Mutex
	session *pairSession
}

// pairTimeout returns how long a request waits for the user.
func (p *pairingServer) waitTimeout() time.Duration {
	if p.timeout > 0 {
		return p.timeout
	}
	return pairTimeout
}

// register installs the pairing endpoints. They are deliberately not wrapped in
// requirePaired: pairing is how a device becomes paired. Confirming is not an
// endpoint: the code is typed on this device, so it is checked here.
func (p *pairingServer) register(mux *http.ServeMux) {
	mux.HandleFunc("POST "+pathPairHello, p.hello)
	mux.HandleFunc("POST "+pathPairWait, p.wait)
	mux.HandleFunc("POST "+pathPairCancel, p.cancel)
}

// pending reports the request waiting for the user, if any.
func (p *pairingServer) pending() (PairRequest, bool) {
	session := p.unresolved()
	if session == nil {
		return PairRequest{}, false
	}
	return requestOf(session), true
}

// unresolved returns the session still waiting for an answer, or nil. A session
// that ended — confirmed, refused, canceled, or out of time — is not waiting.
func (p *pairingServer) unresolved() *pairSession {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session == nil || p.session.resolved || p.now().After(p.session.deadline) {
		return nil
	}
	return p.session
}

// requestOf projects a session onto what the dialog shows.
func requestOf(session *pairSession) PairRequest {
	return PairRequest{
		PeerID:   session.peer.ID,
		Name:     session.peer.Name,
		Platform: session.peer.Platform,
	}
}

// hello starts a pairing session.
func (p *pairingServer) hello(w http.ResponseWriter, r *http.Request) {
	public, err := presentedKey(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	var request pairHello
	if err := readJSON(r, &request); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if request.Version != pairProtocolVersion {
		// Answer with this device's own version so the caller can tell a
		// version mismatch from a broken exchange.
		writeJSON(w, http.StatusOK, pairHelloReply{Version: pairProtocolVersion})
		return
	}
	if len(request.Ephemeral) != pairKeySize {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	initiatorEphemeral, err := ecdh.X25519().NewPublicKey(request.Ephemeral)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	identity := p.store.Identity()
	responderEphemeral, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	shared, err := responderEphemeral.ECDH(initiatorEphemeral)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	transcript := pairTranscript(public, identity.Public, request.Ephemeral, responderEphemeral.PublicKey().Bytes())
	code, proof, err := pairSecret(shared, transcript)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	session := &pairSession{
		peer: Device{
			ID:        PeerID(public),
			Name:      displayName(request.Name),
			Platform:  displayName(request.Platform),
			PublicKey: public,
		},
		code:     code,
		proof:    proof,
		deadline: p.now().Add(p.waitTimeout()),
		done:     make(chan struct{}),
	}

	p.mu.Lock()
	previous := p.session
	if previous != nil && !previous.resolved && p.now().Before(previous.deadline) {
		p.mu.Unlock()
		writeJSON(w, http.StatusConflict, pairStatus{Status: pairStatusAborted, Reason: pairReasonBusy})
		return
	}
	p.session = session
	p.mu.Unlock()
	// A session that ran out its clock but has not been resolved yet would
	// otherwise keep a waiting request blocked forever.
	if previous != nil && !previous.resolved {
		p.resolve(previous, ErrPairingTimeout)
	}

	go p.expire(session, session.deadline)
	if p.onRequest != nil {
		p.onRequest(requestOf(session))
	}
	writeJSON(w, http.StatusOK, pairHelloReply{
		Version:   pairProtocolVersion,
		Ephemeral: responderEphemeral.PublicKey().Bytes(),
		Name:      p.store.Name(),
		Platform:  p.store.Platform(),
	})
}

// wait holds the requesting device's request open until the session resolves,
// so the initiating side learns the outcome without polling.
func (p *pairingServer) wait(w http.ResponseWriter, r *http.Request) {
	session, ok := p.current(r)
	if !ok {
		writeJSON(w, http.StatusOK, pairStatus{Status: pairStatusAborted, Reason: pairReasonCanceled})
		return
	}
	select {
	case <-session.done:
	case <-r.Context().Done():
		return
	}
	if session.err != nil {
		writeJSON(w, http.StatusOK, pairStatus{Status: pairStatusAborted, Reason: pairReason(session.err)})
		return
	}
	writeJSON(w, http.StatusOK, pairStatus{Status: pairStatusConfirmed, Proof: session.proof})
}

// confirm completes the pending request with the code the user typed on this
// device. It returns ErrPairingWrongCode when the code does not match, and
// ErrPairingTimeout when there is nothing waiting to confirm.
func (p *pairingServer) confirm(code string) error {
	session := p.unresolved()
	if session == nil {
		return ErrPairingTimeout
	}
	if subtle.ConstantTimeCompare([]byte(code), []byte(session.code)) != 1 {
		p.resolve(session, ErrPairingWrongCode)
		return ErrPairingWrongCode
	}

	device := session.peer
	device.SyncEnabled = true
	device.PairedAt = p.now()
	if err := p.store.Put(device); err != nil {
		p.resolve(session, err)
		return err
	}
	p.resolve(session, nil)
	if p.onPaired != nil {
		p.onPaired(device)
	}
	return nil
}

// cancel abandons the session, as when either user closes the dialog.
func (p *pairingServer) cancel(w http.ResponseWriter, r *http.Request) {
	if session, ok := p.current(r); ok {
		p.resolve(session, ErrPairingCanceled)
	}
	w.WriteHeader(http.StatusNoContent)
}

// current returns the session belonging to the requesting device, so one device
// cannot answer for another's pairing.
func (p *pairingServer) current(r *http.Request) (*pairSession, bool) {
	public, err := presentedKey(r)
	if err != nil {
		return nil, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session == nil || !p.session.peer.PublicKey.Equal(public) {
		return nil, false
	}
	return p.session, true
}

// expire resolves a session that nobody confirmed in time. deadline is passed in
// rather than read off the session, which another goroutine may be resolving at
// the same time.
func (p *pairingServer) expire(session *pairSession, deadline time.Time) {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-session.done:
	case <-timer.C:
		p.resolve(session, ErrPairingTimeout)
	}
}

// resolve finishes a session once. A resolved session keeps its record so a
// waiting request still learns the outcome, but stops blocking new requests.
func (p *pairingServer) resolve(session *pairSession, err error) {
	p.mu.Lock()
	if session.resolved {
		p.mu.Unlock()
		return
	}
	session.resolved = true
	session.err = err
	close(session.done)
	p.mu.Unlock()

	if err != nil && p.onFailed != nil {
		p.onFailed(err)
	}
}

// pairInitiator is the side that shows the code: it opens the exchange, derives
// the code, and waits for the other device to confirm it.
type pairInitiator struct {
	client  *http.Client
	baseURL string
	// identity is this device's identity; name and platform are what it tells
	// the other device about itself.
	identity Identity
	name     string
	platform string
	// peer is the device being paired with: the pinned key the connection must
	// present, and the record to complete once pairing succeeds.
	peer Device
	now  func() time.Time
	// onCode reports the code to show the user. It runs on the pairing
	// goroutine.
	onCode func(code string)
}

// run performs the pairing and returns the device to record. The device's
// PublicKey is the pinned one: TLS already proved the peer holds the matching
// private key.
func (i *pairInitiator) run(ctx context.Context) (Device, error) {
	ephemeral, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Device{}, fmt.Errorf("lansync: generate pairing key: %w", err)
	}
	var reply pairHelloReply
	status, err := i.post(ctx, pathPairHello, pairHello{
		Version:   pairProtocolVersion,
		Ephemeral: ephemeral.PublicKey().Bytes(),
		Name:      i.name,
		Platform:  i.platform,
	}, &reply)
	if err != nil {
		return Device{}, err
	}
	if status == http.StatusConflict {
		return Device{}, ErrPairingBusy
	}
	if status != http.StatusOK {
		return Device{}, fmt.Errorf("lansync: pairing request refused (%d)", status)
	}
	if reply.Version != pairProtocolVersion {
		return Device{}, ErrPairingVersion
	}
	if len(reply.Ephemeral) != pairKeySize {
		return Device{}, ErrPairingProof
	}
	responderEphemeral, err := ecdh.X25519().NewPublicKey(reply.Ephemeral)
	if err != nil {
		return Device{}, ErrPairingProof
	}
	shared, err := ephemeral.ECDH(responderEphemeral)
	if err != nil {
		return Device{}, ErrPairingProof
	}
	transcript := pairTranscript(i.identity.Public, i.peer.PublicKey, ephemeral.PublicKey().Bytes(), reply.Ephemeral)
	code, proof, err := pairSecret(shared, transcript)
	if err != nil {
		return Device{}, err
	}
	if i.onCode != nil {
		i.onCode(code)
	}

	// Wait for the user on the other device. The deadline covers the pairing
	// window plus a little slack for the final round trip.
	waitCtx, cancel := context.WithTimeout(ctx, pairTimeout+5*time.Second)
	defer cancel()
	var outcome pairStatus
	status, err = i.post(waitCtx, pathPairWait, struct{}{}, &outcome)
	if err != nil {
		return Device{}, err
	}
	if status != http.StatusOK {
		return Device{}, fmt.Errorf("lansync: pairing did not complete (%d)", status)
	}
	if outcome.Status != pairStatusConfirmed {
		return Device{}, pairError(outcome.Reason)
	}
	if !hmac.Equal(outcome.Proof, proof) {
		return Device{}, ErrPairingProof
	}
	device := i.peer
	device.PairedAt = i.now()
	device.SyncEnabled = true
	return device, nil
}

// post sends one JSON request and decodes the reply into out. It returns the
// HTTP status so the caller can tell a refused request from a failed one.
func (i *pairInitiator) post(ctx context.Context, path string, body, out any) (int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("lansync: encode %s: %w", path, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, i.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("lansync: build %s request: %w", path, err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := i.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("lansync: %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return response.StatusCode, nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(out); err != nil {
		return response.StatusCode, fmt.Errorf("lansync: decode %s: %w", path, err)
	}
	return response.StatusCode, nil
}

// displayName cleans a name that arrived over the wire: trimmed, single line,
// and bounded, so it fits the device list whatever the other device sends.
func displayName(name string) string {
	fields := bytes.Fields([]byte(name))
	joined := string(bytes.Join(fields, []byte(" ")))
	runes := []rune(joined)
	if len(runes) > pairNameLimit {
		runes = append(runes[:pairNameLimit-1], '…')
	}
	return string(runes)
}

// readJSON decodes a request body with a size bound. Pairing messages are tiny;
// the bound only exists so a broken peer cannot stream forever.
func readJSON(r *http.Request, value any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return decoder.Decode(value)
}

// writeJSON writes one JSON reply.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
