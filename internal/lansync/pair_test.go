package lansync

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// pairingDevice is one device taking part in a pairing test: a store, the
// responding side wired to a real TLS server, and what the application would
// have been told.
type pairingDevice struct {
	store  *Store
	server *pairingServer
	addr   string

	mu       sync.Mutex
	requests []PairRequest
	paired   []Device
	failures []error
}

// newPairingDevice starts a device that answers pairing requests.
func newPairingDevice(t *testing.T, name, platform string) *pairingDevice {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "devices.json"), name, platform)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	device := &pairingDevice{store: store}
	device.server = &pairingServer{
		store: store,
		now:   time.Now,
		onRequest: func(request PairRequest) {
			device.mu.Lock()
			defer device.mu.Unlock()
			device.requests = append(device.requests, request)
		},
		onPaired: func(paired Device) {
			device.mu.Lock()
			defer device.mu.Unlock()
			device.paired = append(device.paired, paired)
		},
		onFailed: func(err error) {
			device.mu.Lock()
			defer device.mu.Unlock()
			device.failures = append(device.failures, err)
		},
	}
	mux := http.NewServeMux()
	device.server.register(mux)
	device.addr = startTestServer(t, store.Identity(), mux)
	return device
}

// record is how a peer would know this device from discovery.
func (d *pairingDevice) record() Device {
	return Device{
		ID:        d.store.Identity().ID,
		Name:      d.store.Name(),
		Platform:  d.store.Platform(),
		PublicKey: d.store.Identity().Public,
	}
}

// initiator builds the initiating side for a device with a fresh identity.
func initiator(t *testing.T, name string, target Device, addr string) *pairInitiator {
	t.Helper()
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	client := newHTTPClient(identity, func(candidate string) (Device, bool) {
		if candidate != addr {
			return Device{}, false
		}
		return target, true
	})
	// Pairing waits for a human, so the request deadline comes from the
	// context, not from a fixed client timeout.
	client.Timeout = 0
	return &pairInitiator{
		client:   client,
		baseURL:  "https://" + addr,
		identity: identity,
		name:     name,
		platform: "macOS",
		peer:     target,
		now:      time.Now,
	}
}

// pairingRun is one initiating pairing running in the background.
type pairingRun struct {
	codes   chan string
	result  chan Device
	failure chan error
	cancel  context.CancelFunc
}

// startPairing runs the initiating side and reports the code it shows.
func startPairing(t *testing.T, pair *pairInitiator) *pairingRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	run := &pairingRun{
		codes:   make(chan string, 1),
		result:  make(chan Device, 1),
		failure: make(chan error, 1),
		cancel:  cancel,
	}
	pair.onCode = func(code string) {
		select {
		case run.codes <- code:
		default:
		}
	}
	go func() {
		device, err := pair.run(ctx)
		if err != nil {
			run.failure <- err
			return
		}
		run.result <- device
	}()
	t.Cleanup(cancel)
	return run
}

// code waits for the code the initiating device shows.
func (r *pairingRun) code(t *testing.T) string {
	t.Helper()
	select {
	case code := <-r.codes:
		return code
	case err := <-r.failure:
		t.Fatalf("pairing failed before showing a code: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the pairing code")
	}
	return ""
}

// outcome waits for the pairing to finish.
func (r *pairingRun) outcome(t *testing.T) (Device, error) {
	t.Helper()
	select {
	case device := <-r.result:
		return device, nil
	case err := <-r.failure:
		return Device{}, err
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the pairing to finish")
	}
	return Device{}, nil
}

// TestPairingCompletesAndPinsBothSides 检查配对全流程：发起方显示 6 位码，响应方
// 输入同一个码，两边都把对方的公钥钉住。
func TestPairingCompletesAndPinsBothSides(t *testing.T) {
	responder := newPairingDevice(t, "工作室 PC", "Windows")
	pair := initiator(t, "MacBook Pro", responder.record(), responder.addr)
	run := startPairing(t, pair)

	code := run.code(t)
	if len(code) != pairCodeLength {
		t.Fatalf("code = %q, want %d digits", code, pairCodeLength)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("code = %q, want digits only", code)
		}
	}
	// 响应方的界面上应该出现这台请求配对的设备。
	waitFor(t, func() bool {
		responder.mu.Lock()
		defer responder.mu.Unlock()
		return len(responder.requests) == 1
	})
	responder.mu.Lock()
	request := responder.requests[0]
	responder.mu.Unlock()
	if request.Name != "MacBook Pro" || request.Platform != "macOS" {
		t.Errorf("pair request = %+v, want the initiating device", request)
	}

	if err := responder.server.confirm(code); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	device, err := run.outcome(t)
	if err != nil {
		t.Fatalf("pairing failed: %v", err)
	}
	if !device.PublicKey.Equal(responder.store.Identity().Public) {
		t.Error("the initiating side must pin the responding device's key")
	}
	if !device.SyncEnabled {
		t.Error("a freshly paired device must have sync on")
	}
	if device.PairedAt.IsZero() {
		t.Error("the pairing time must be recorded")
	}

	// 响应方把发起方钉住，界面也收到了配对完成。
	paired, ok := responder.store.Trusted(device.PublicKey)
	if ok {
		t.Fatalf("the responding side must not pin its own key; got %+v", paired)
	}
	responder.mu.Lock()
	recorded := append([]Device(nil), responder.paired...)
	responder.mu.Unlock()
	if len(recorded) != 1 {
		t.Fatalf("paired devices recorded = %d, want 1", len(recorded))
	}
	if recorded[0].ID != pair.identity.ID {
		t.Error("the responding side must pin the initiating device's key")
	}
	if _, ok := responder.store.Device(recorded[0].ID); !ok {
		t.Error("the paired device must be stored")
	}

	// 同一个码再确认一次不会再配一遍：会话已经结束。
	if err := responder.server.confirm(code); !errors.Is(err, ErrPairingTimeout) {
		t.Errorf("a second confirm = %v, want ErrPairingTimeout", err)
	}
	responder.mu.Lock()
	repeated := len(responder.paired)
	responder.mu.Unlock()
	if repeated != 1 {
		t.Errorf("pairing was reported %d times, want 1", repeated)
	}
}

// TestPairingRejectsWrongCode 检查码不对时配对失败，两边都不留下配对记录。
func TestPairingRejectsWrongCode(t *testing.T) {
	responder := newPairingDevice(t, "工作室 PC", "Windows")
	run := startPairing(t, initiator(t, "MacBook Pro", responder.record(), responder.addr))
	code := run.code(t)
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}

	if err := responder.server.confirm(wrong); !errors.Is(err, ErrPairingWrongCode) {
		t.Fatalf("confirm with a wrong code = %v, want ErrPairingWrongCode", err)
	}
	if _, err := run.outcome(t); !errors.Is(err, ErrPairingWrongCode) {
		t.Fatalf("pairing outcome = %v, want ErrPairingWrongCode", err)
	}
	if len(responder.store.Devices()) != 0 {
		t.Error("a failed pairing must not leave a device behind")
	}
}

// TestPairingTimesOut 检查没人输入配对码时，会话在超时后作废，发起方拿到超时错误。
func TestPairingTimesOut(t *testing.T) {
	responder := newPairingDevice(t, "工作室 PC", "Windows")
	responder.server.timeout = 100 * time.Millisecond
	run := startPairing(t, initiator(t, "MacBook Pro", responder.record(), responder.addr))
	run.code(t)

	if _, err := run.outcome(t); !errors.Is(err, ErrPairingTimeout) {
		t.Fatalf("pairing outcome = %v, want ErrPairingTimeout", err)
	}
	if len(responder.store.Devices()) != 0 {
		t.Error("a timed-out pairing must not leave a device behind")
	}
}

// TestPairingBusy 检查同时只接受一个配对请求：第二台设备被告知对方正在配对。
func TestPairingBusy(t *testing.T) {
	responder := newPairingDevice(t, "工作室 PC", "Windows")
	first := startPairing(t, initiator(t, "MacBook Pro", responder.record(), responder.addr))
	first.code(t)

	second := initiator(t, "第二台机器", responder.record(), responder.addr)
	second.onCode = func(string) {}
	if _, err := second.run(context.Background()); !errors.Is(err, ErrPairingBusy) {
		t.Fatalf("second pairing = %v, want ErrPairingBusy", err)
	}
	first.cancel()
}

// TestPairingDetectsManInTheMiddle 检查配对码真的能挡住中间人：中间人在两边各
// 开一段会话（各自算出不同的共享密钥），发起方显示的码在响应方那里必然对不上。
func TestPairingDetectsManInTheMiddle(t *testing.T) {
	target := newPairingDevice(t, "工作室 PC", "Windows")
	relay := newPairingRelay(t, target)

	// 发起方以为自己连的是「工作室 PC」，实际连的是中间人（相当于 mDNS 被冒充）。
	run := startPairing(t, initiator(t, "MacBook Pro", relay.record(), relay.addr))
	code := run.code(t)

	// 用户把发起方显示的码输进真正的响应方，响应方算出的是另一个码。
	if err := target.server.confirm(code); !errors.Is(err, ErrPairingWrongCode) {
		t.Fatalf("confirm through a man in the middle = %v, want ErrPairingWrongCode", err)
	}
	if _, err := run.outcome(t); !errors.Is(err, ErrPairingWrongCode) {
		t.Fatalf("pairing outcome = %v, want ErrPairingWrongCode", err)
	}
	if len(target.store.Devices()) != 0 {
		t.Error("the responding side must not pair through a man in the middle")
	}
}

// pairingRelay stands in the middle: it answers the initiating device with its
// own ephemeral key and opens a separate session with the real target, which is
// exactly what a spoofed advertisement would let an attacker do.
type pairingRelay struct {
	addr     string
	identity Identity
}

// newPairingRelay starts a relay in front of target.
func newPairingRelay(t *testing.T, target *pairingDevice) *pairingRelay {
	t.Helper()
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	record := target.record()
	client := newHTTPClient(identity, func(candidate string) (Device, bool) {
		if candidate != target.addr {
			return Device{}, false
		}
		return record, true
	})
	client.Timeout = 0
	relay := &pairingRelay{identity: identity}
	toTarget := &pairInitiator{
		client:   client,
		baseURL:  "https://" + target.addr,
		identity: identity,
		name:     "中间人",
		platform: "Linux",
		peer:     record,
		now:      time.Now,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+pathPairHello, func(w http.ResponseWriter, r *http.Request) {
		var request pairHello
		if err := readJSON(r, &request); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// 与真正的目标另开一段会话，用的是中间人自己的临时密钥。
		ownKey, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		var reply pairHelloReply
		status, err := toTarget.post(r.Context(), pathPairHello, pairHello{
			Version:   pairProtocolVersion,
			Ephemeral: ownKey.PublicKey().Bytes(),
			Name:      "中间人",
			Platform:  "Linux",
		}, &reply)
		if err != nil || status != http.StatusOK {
			http.Error(w, "relay failed", http.StatusBadGateway)
			return
		}
		// 给发起方的是中间人自己的另一把临时密钥，两边因此算出不同的共享密钥。
		presented, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, pairHelloReply{
			Version:   pairProtocolVersion,
			Ephemeral: presented.PublicKey().Bytes(),
			Name:      record.Name,
			Platform:  record.Platform,
		})
	})
	mux.HandleFunc("POST "+pathPairWait, func(w http.ResponseWriter, r *http.Request) {
		var outcome pairStatus
		if _, err := toTarget.post(r.Context(), pathPairWait, struct{}{}, &outcome); err != nil {
			http.Error(w, "relay failed", http.StatusBadGateway)
			return
		}
		writeJSON(w, http.StatusOK, outcome)
	})
	mux.HandleFunc("POST "+pathPairCancel, func(w http.ResponseWriter, r *http.Request) {
		_, _ = toTarget.post(r.Context(), pathPairCancel, struct{}{}, &pairStatus{})
		w.WriteHeader(http.StatusNoContent)
	})
	relay.addr = startTestServer(t, identity, mux)
	return relay
}

// record is how a spoofed advertisement would present the relay.
func (r *pairingRelay) record() Device {
	return Device{
		ID:        r.identity.ID,
		Name:      "工作室 PC",
		Platform:  "Windows",
		PublicKey: r.identity.Public,
	}
}

// TestPairingVersionMismatch 检查配对协议版本不同的设备拒绝配对，而不是半途而废。
func TestPairingVersionMismatch(t *testing.T) {
	responder := newPairingDevice(t, "工作室 PC", "Windows")
	pair := initiator(t, "MacBook Pro", responder.record(), responder.addr)

	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var reply pairHelloReply
	status, err := pair.post(context.Background(), pathPairHello, pairHello{
		Version:   "2",
		Ephemeral: key.PublicKey().Bytes(),
	}, &reply)
	if err != nil {
		t.Fatalf("hello: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if reply.Version != pairProtocolVersion {
		t.Errorf("reply version = %q, want %q", reply.Version, pairProtocolVersion)
	}
	if len(reply.Ephemeral) != 0 {
		t.Error("a version mismatch must not start a session")
	}
}

// TestPairingCancelClosesTheRequest 检查发起方取消后，响应方那边的等待结束，
// 不会一直挂着一个待确认的请求。
func TestPairingCancelClosesTheRequest(t *testing.T) {
	responder := newPairingDevice(t, "工作室 PC", "Windows")
	pair := initiator(t, "MacBook Pro", responder.record(), responder.addr)
	shown := make(chan string, 1)
	pair.onCode = func(code string) {
		select {
		case shown <- code:
		default:
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := pair.run(ctx)
		done <- err
	}()
	select {
	case <-shown:
	case <-time.After(5 * time.Second):
		t.Fatal("the pairing code was never shown")
	}

	if _, err := pair.post(context.Background(), pathPairCancel, struct{}{}, &pairStatus{}); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrPairingCanceled) {
			t.Fatalf("pairing outcome = %v, want ErrPairingCanceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the initiating side kept waiting after a cancel")
	}
	waitFor(t, func() bool {
		responder.mu.Lock()
		defer responder.mu.Unlock()
		return len(responder.failures) == 1
	})
	responder.mu.Lock()
	failure := responder.failures[0]
	responder.mu.Unlock()
	if !errors.Is(failure, ErrPairingCanceled) {
		t.Errorf("responding side saw %v, want ErrPairingCanceled", failure)
	}
	cancel()
}

// TestPairSecretIsDeterministic 检查同样的共享密钥和记录算出同样的码与证明，
// 而换了任一方密钥就都变。
func TestPairSecretIsDeterministic(t *testing.T) {
	first, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	second, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	shared := []byte("0123456789abcdef0123456789abcdef")
	firstEphemeral := make([]byte, pairKeySize)
	secondEphemeral := make([]byte, pairKeySize)
	secondEphemeral[0] = 1

	transcript := pairTranscript(first.Public, second.Public, firstEphemeral, secondEphemeral)
	code, proof, err := pairSecret(shared, transcript)
	if err != nil {
		t.Fatalf("pairSecret: %v", err)
	}
	againCode, againProof, err := pairSecret(shared, transcript)
	if err != nil {
		t.Fatalf("pairSecret: %v", err)
	}
	if code != againCode || !strings.EqualFold(string(proof), string(againProof)) {
		t.Error("the same input must derive the same code and proof")
	}
	if len(code) != pairCodeLength {
		t.Errorf("code = %q, want %d digits", code, pairCodeLength)
	}
	if len(proof) != 32 {
		t.Errorf("proof length = %d, want 32", len(proof))
	}

	// 换掉对方身份密钥、换掉临时密钥、换掉共享密钥，码和证明都必须变。
	for name, changed := range map[string]struct {
		shared     []byte
		transcript []byte
	}{
		"other static key": {
			shared:     shared,
			transcript: pairTranscript(first.Public, first.Public, firstEphemeral, secondEphemeral),
		},
		"other ephemeral": {
			shared:     shared,
			transcript: pairTranscript(first.Public, second.Public, secondEphemeral, firstEphemeral),
		},
		"other shared secret": {
			shared:     []byte("fedcba9876543210fedcba9876543210"),
			transcript: transcript,
		},
	} {
		otherCode, otherProof, err := pairSecret(changed.shared, changed.transcript)
		if err != nil {
			t.Fatalf("%s: pairSecret: %v", name, err)
		}
		if otherCode == code {
			t.Errorf("%s: the code must change", name)
		}
		if string(otherProof) == string(proof) {
			t.Errorf("%s: the proof must change", name)
		}
	}
}

// TestDisplayNameIsBounded 检查对方发来的名字被收拾干净：去掉换行、限制长度。
func TestDisplayNameIsBounded(t *testing.T) {
	if got := displayName("  工作室\n\tPC  "); got != "工作室 PC" {
		t.Errorf("displayName = %q, want %q", got, "工作室 PC")
	}
	long := strings.Repeat("名", pairNameLimit+10)
	got := displayName(long)
	if len([]rune(got)) != pairNameLimit {
		t.Errorf("displayName length = %d, want %d", len([]rune(got)), pairNameLimit)
	}
}

// waitFor polls until condition holds, failing the test after a few seconds.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not become true in time")
}
