package lansync

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"gopaste/internal/history"
)

// clipProtocolVersion is the clipboard payload format this build speaks. A peer
// that speaks another version is refused rather than guessed at. File payloads
// travel through the file transfer endpoints (see files.go) and carry this same
// version in their offer.
const clipProtocolVersion = "2"

// maxClipBytes bounds one incoming payload. A clipboard image is the largest
// thing that travels, and 32 MiB of base64 is far more than a screen-sized PNG
// needs; the bound only exists so a broken peer cannot fill the disk.
const maxClipBytes = 32 << 20

// unreachableWindow is how long a device stays offline after a delivery attempt
// failed. Discovery sightings are older than the failure and would otherwise
// flip the device back online every few seconds, retrying forever.
const unreachableWindow = 20 * time.Second

// Clip outcomes, as they travel between devices.
const (
	clipStatusReceived = "received"
	clipStatusIgnored  = "ignored"

	clipReasonDirection = "direction"
	clipReasonVersion   = "version"
	clipReasonSize      = "size"
	// clipReasonKind refuses a payload whose kind does not fit the endpoint it
	// arrived on: a file item must come through the file transfer endpoints.
	clipReasonKind = "kind"
)

// ErrUnknownPeer reports a device that is neither paired nor currently on the
// network, so there is nothing to reach.
var ErrUnknownPeer = errors.New("lansync: the device is not on this network")

// ErrNoService reports a Service used before Start or after Stop.
var ErrNoService = errors.New("lansync: the service is not running")

// Clip is one clipboard payload: what travels between devices, and what one
// device received.
//
// It is the payload in the form item identity is derived from, so the receiving
// side computes the same ID the sending side has (history.ContentID) instead of
// trusting an ID off the wire. The same content copied on two machines therefore
// lands as one record rather than two.
type Clip struct {
	Kind history.Kind
	// Text is the plain-text representation, empty when absent.
	Text string
	// HTML is the markup of a rich-text payload, empty for every other kind.
	HTML string
	// Image is the image payload, nil for every other kind.
	Image *history.Image
	// Files are the paths of a file payload as the sending device copied them,
	// in clipboard order. They are the item's identity: a file item's content ID
	// is derived from them on both sides. The receiving device rewrites its own
	// copy of them into LocalFiles.
	Files []string
	// LocalFiles are the paths this device holds a file payload at, after a
	// file transfer landed. It is empty on the sending side, where Files is
	// already local, and never travels.
	LocalFiles []string
	// FileSize is the byte size of a file payload as it arrived, zero when the
	// item carries no files.
	FileSize int64
	// CreatedAt is when the content was copied on the sending device.
	CreatedAt time.Time
}

// ClipOf projects a history item onto the payload to deliver.
func ClipOf(item history.Item) Clip {
	clip := Clip{
		Kind:      item.Kind,
		Text:      item.Text,
		HTML:      item.HTML,
		Image:     item.Image,
		CreatedAt: item.CreatedAt,
	}
	if item.Kind == history.KindFile && item.Text != "" {
		// A file item keeps its paths as newline-joined text; identity needs
		// the list, so it is split back out. A local item's paths are both its
		// identity and where its content lives.
		clip.Files = strings.Split(item.Text, "\n")
		clip.LocalFiles = clip.Files
		clip.FileSize = item.FileSize
	}
	return clip
}

// ID returns the content-derived ID of the payload, the same one the sending
// device stored.
func (c Clip) ID() string {
	return history.ContentID(history.Content{
		Kind:  c.Kind,
		Text:  c.Text,
		HTML:  c.HTML,
		Image: c.Image,
		Files: c.Files,
	})
}

// HistoryItem turns a received payload into the history item to store. The
// arrival time is this device's own clock: the record belongs at the top of the
// list here, whatever the sending device's clock says.
//
// A file payload stores the paths this device holds it at, so the record's
// actions work on real files, while its ID still comes from the paths the
// sending device copied: the same content therefore keeps one ID across
// devices.
func (c Clip) HistoryItem(sourceDeviceID string, at time.Time) history.Item {
	item := history.Item{
		ID:             c.ID(),
		Kind:           c.Kind,
		Text:           c.Text,
		HTML:           c.HTML,
		Image:          c.Image,
		SourceDeviceID: sourceDeviceID,
		CreatedAt:      at,
	}
	if c.Kind == history.KindFile {
		local := c.LocalFiles
		if len(local) == 0 {
			local = c.Files
		}
		item.Text = strings.Join(local, "\n")
		item.FileSize = c.FileSize
	}
	return item
}

// clipPayload is the wire form of a Clip. Payload bytes ride as base64 in JSON,
// which keeps one endpoint and one decoder for every content type that fits in
// a request. File payloads are not among them: their bytes go through the file
// transfer endpoints instead (see files.go).
type clipPayload struct {
	Version   string    `json:"version"`
	Kind      int       `json:"kind"`
	Text      string    `json:"text,omitempty"`
	HTML      string    `json:"html,omitempty"`
	Image     []byte    `json:"image,omitempty"`
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// payloadOf projects a clip onto the wire form.
func payloadOf(clip Clip) clipPayload {
	payload := clipPayload{
		Version:   clipProtocolVersion,
		Kind:      int(clip.Kind),
		Text:      clip.Text,
		HTML:      clip.HTML,
		CreatedAt: clip.CreatedAt,
	}
	if clip.Image != nil {
		payload.Image = clip.Image.PNG
		payload.Width = clip.Image.Width
		payload.Height = clip.Image.Height
	}
	return payload
}

// clip decodes the wire form back into a payload.
func (p clipPayload) clip() (Clip, error) {
	if p.Version != clipProtocolVersion {
		return Clip{}, ErrPairingVersion
	}
	clip := Clip{
		Kind:      history.Kind(p.Kind),
		Text:      p.Text,
		HTML:      p.HTML,
		CreatedAt: p.CreatedAt,
	}
	if len(p.Image) > 0 {
		clip.Image = &history.Image{PNG: p.Image, Width: p.Width, Height: p.Height}
	}
	return clip, nil
}

// clipReply is how the receiving device answers a delivery.
type clipReply struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// Options configures a Service. Every callback runs on a service goroutine, so
// an application that owns UI state must hand results to its UI thread itself.
type Options struct {
	// StorePath is where the identity and the paired devices live. It is
	// required.
	StorePath string
	// Name is this device's name as peers see it.
	Name string
	// Platform is this device's platform name, such as "macOS".
	Platform string
	// Version is this application's version, shown in the device list.
	Version string
	// Now returns the current time. Tests set it; nil means time.Now.
	Now func() time.Time
	// FilesDir is where received file payloads are stored, one directory per
	// item. It is required for receiving files: without it a peer's file item is
	// refused instead of half-received.
	FilesDir string
	// FileLimitBytes bounds one file item, in bytes, on both sides: nothing
	// larger is sent, and nothing larger is accepted. Zero or negative means
	// DefaultFileLimitBytes.
	FileLimitBytes int64

	// OnPeersChanged reports the devices seen on the network.
	OnPeersChanged func([]Peer)
	// OnDevicesChanged reports a change to the paired devices: a pairing, an
	// unpairing, a settings change, or a device going online or offline.
	OnDevicesChanged func([]Device)
	// OnPairRequest reports a device asking to pair with this one.
	OnPairRequest func(PairRequest)
	// OnPairingCode reports the code to show while pairing with a device.
	OnPairingCode func(deviceName, code string)
	// OnPairingFailed reports a pairing that did not complete. The device name
	// is the one the pairing was with, and is empty when the failure came from
	// a request this device was answering.
	OnPairingFailed func(deviceName string, err error)
	// OnClip reports clipboard content that arrived from a paired device.
	OnClip func(clip Clip, from Device)
	// OnDelivery reports the outcome of delivering one item.
	OnDelivery func(itemID string, deliveries []history.DeviceDelivery)
}

// Service is the whole package as the application sees it: it advertises this
// device, finds the others, pairs with them, and delivers clipboard items to
// them.
//
// Every slow thing it does — discovery, pairing, delivery — runs on its own
// goroutine and reports through Options callbacks. Nothing in it blocks the
// caller beyond a local file write.
type Service struct {
	store    *Store
	identity Identity
	options  Options
	now      func() time.Time

	pairing *pairingServer
	client  *http.Client
	// fileClient is the client file uploads use: same pinning, no overall
	// timeout.
	fileClient *http.Client

	listener  net.Listener
	server    *http.Server
	discovery *Discovery

	ctx    context.Context
	cancel context.CancelFunc
	// wg tracks the goroutines that outlive a call: serving, discovery,
	// pairing attempts and deliveries.
	wg sync.WaitGroup

	mu   sync.RWMutex
	port int
	// peers is the latest discovery result, keyed by peer ID.
	peers map[string]Peer
	// unreachable records devices whose last delivery attempt failed, so a
	// stale sighting does not flip them back online.
	unreachable map[string]time.Time
	// pairAddress is where a pairing this device started is under way, and is
	// empty when none is. pairClient is pinned to that peer, which is not a
	// paired device yet, so it cannot go through the shared client.
	pairAddress string
	pairClient  *http.Client
	// noDiscovery turns off advertising and browsing. It exists for tests that
	// supply the peer list themselves, so they do not depend on multicast.
	noDiscovery bool

	// fileLimit bounds one file item in bytes, on both sides. SetFileLimitBytes
	// updates it while the service runs; it is guarded by mu like the rest of
	// the mutable state.
	fileLimit int64
	// files holds the file uploads this device has agreed to take, keyed by the
	// token it handed out. Each entry is removed when its upload starts or when
	// the staging sweep expires it.
	filesMu sync.Mutex
	files   map[string]*fileTransfer
}

// NewService opens the identity file and prepares a service. Call Start to
// begin advertising, discovering, and accepting connections.
func NewService(options Options) (*Service, error) {
	store, err := OpenStore(options.StorePath, options.Name, options.Platform)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	service := &Service{
		store:       store,
		identity:    store.Identity(),
		options:     options,
		now:         now,
		peers:       map[string]Peer{},
		unreachable: map[string]time.Time{},
		fileLimit:   options.FileLimitBytes,
		files:       map[string]*fileTransfer{},
	}
	service.client = newHTTPClient(store.Identity(), service.deviceAt)
	service.fileClient = newFileClient(store.Identity(), service.deviceAt)
	service.pairing = &pairingServer{
		store: store,
		now:   now,
		onRequest: func(request PairRequest) {
			if service.options.OnPairRequest != nil {
				service.options.OnPairRequest(request)
			}
		},
		onPaired: func(Device) { service.devicesChanged() },
		onFailed: func(err error) {
			if service.options.OnPairingFailed != nil {
				service.options.OnPairingFailed("", err)
			}
		},
	}
	return service, nil
}

// Start begins accepting connections, advertising this device, and browsing for
// peers. The service runs until Stop or until ctx ends.
//
// An error from discovery does not stop the listener: a device whose network
// cannot carry multicast can still be reached by a peer that knows its address,
// and the application shows the discovery error while delivery keeps working.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.ctx = runCtx
	s.cancel = cancel
	s.mu.Unlock()

	config, err := serverTLSConfig(s.identity)
	if err != nil {
		s.Stop()
		return err
	}
	listener, port, err := listen(defaultPort)
	if err != nil {
		s.Stop()
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+pathClip, requirePaired(s.store.Trusted, s.handleClip))
	mux.HandleFunc("POST "+pathFilesOffer, requirePaired(s.store.Trusted, s.handleFileOffer))
	mux.HandleFunc("POST "+pathFilesUpload+"/{token}", requirePaired(s.store.Trusted, s.handleFileUpload))
	s.pairing.register(mux)

	server := &http.Server{
		Handler:           mux,
		TLSConfig:         config,
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.mu.Lock()
	s.listener = listener
	s.port = port
	s.server = server
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Serve returns when the listener closes, which is how Stop ends it.
		_ = server.Serve(tls.NewListener(listener, config))
	}()

	if s.noDiscovery {
		return nil
	}
	s.discovery = NewDiscovery(DiscoveryOptions{
		Identity: s.identity,
		Name:     s.store.Name,
		Platform: s.store.Platform(),
		Version:  s.options.Version,
		Port:     s.portOf,
		OnUpdate: s.peersChanged,
		OnSight:  s.peerSighted,
	})
	if err := s.discovery.Start(runCtx); err != nil {
		return err
	}
	return nil
}

// Stop ends discovery and closes the listener. It waits for the goroutines it
// started, so a caller can treat it as "no more callbacks after this returns".
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	server := s.server
	listener := s.listener
	s.cancel = nil
	s.server = nil
	s.listener = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if s.discovery != nil {
		s.discovery.Stop()
	}
	if server != nil {
		_ = server.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
	s.wg.Wait()
	s.client.CloseIdleConnections()
}

// goRun starts fn on its own goroutine if the service is running, and reports
// whether it did. Taking the lock before counting is what makes Stop's wait
// see every goroutine it must wait for.
func (s *Service) goRun(fn func(ctx context.Context)) bool {
	s.mu.Lock()
	ctx := s.ctx
	if s.cancel == nil || ctx == nil {
		s.mu.Unlock()
		return false
	}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		fn(ctx)
	}()
	return true
}

// Identity returns this device's network identity.
func (s *Service) Identity() Identity { return s.identity }

// StorePath returns where the identity and pairing data live.
func (s *Service) StorePath() string { return s.store.Path() }

// Name returns this device's name as peers see it.
func (s *Service) Name() string { return s.store.Name() }

// SetName renames this device and re-announces it, so the other devices' lists
// pick the new name up.
func (s *Service) SetName(name string) error {
	if err := s.store.SetName(name); err != nil {
		return err
	}
	if s.discovery != nil {
		s.discovery.Refresh()
	}
	return nil
}

// Devices returns the paired devices with their current presence.
func (s *Service) Devices() []Device {
	return s.mergeDevices(s.store.Devices())
}

// Peers returns the devices seen on the network, paired or not.
func (s *Service) Peers() []Peer {
	if s.discovery == nil {
		return nil
	}
	return s.discovery.Peers()
}

// Refresh re-announces this device and browses again, as the device page's
// refresh button asks for.
func (s *Service) Refresh() {
	if s.discovery != nil {
		s.discovery.Refresh()
	}
}

// Pair starts pairing with one discovered device. It returns once the exchange
// is under way; the code to show arrives through OnPairingCode, and the outcome
// through OnDevicesChanged or OnPairingFailed.
func (s *Service) Pair(peerID string) error {
	s.mu.RLock()
	peer, ok := s.peers[peerID]
	s.mu.RUnlock()
	if !ok {
		return ErrUnknownPeer
	}
	target := Device{
		ID:        peer.ID,
		Name:      peer.Name,
		Platform:  peer.Platform,
		PublicKey: peer.PublicKey,
	}
	address := net.JoinHostPort(peer.Addr.String(), fmt.Sprint(peer.Port))
	client := s.clientFor(target, address)

	s.mu.Lock()
	if s.cancel == nil {
		s.mu.Unlock()
		return ErrNoService
	}
	if s.pairAddress != "" {
		s.mu.Unlock()
		return ErrPairingBusy
	}
	s.pairAddress = address
	s.pairClient = client
	s.mu.Unlock()

	started := s.goRun(func(ctx context.Context) {
		defer func() {
			s.mu.Lock()
			s.pairAddress = ""
			s.pairClient = nil
			s.mu.Unlock()
		}()
		initiator := &pairInitiator{
			client:   client,
			baseURL:  "https://" + address,
			identity: s.identity,
			name:     s.store.Name(),
			platform: s.store.Platform(),
			peer:     target,
			now:      s.now,
			onCode: func(code string) {
				if s.options.OnPairingCode != nil {
					s.options.OnPairingCode(peer.Name, code)
				}
			},
		}
		device, err := initiator.run(ctx)
		if err == nil {
			err = s.store.Put(device)
		}
		if err == nil {
			s.devicesChanged()
			return
		}
		if errors.Is(err, context.Canceled) {
			err = ErrPairingCanceled
		}
		if s.options.OnPairingFailed != nil {
			s.options.OnPairingFailed(peer.Name, err)
		}
	})
	if !started {
		s.mu.Lock()
		s.pairAddress = ""
		s.mu.Unlock()
		return ErrNoService
	}
	return nil
}

// PendingPairRequest returns the pairing request waiting for the user, if any.
func (s *Service) PendingPairRequest() (PairRequest, bool) {
	return s.pairing.pending()
}

// ConfirmPairing completes the pending pairing with the code the user typed on
// this device.
func (s *Service) ConfirmPairing(code string) error {
	return s.pairing.confirm(code)
}

// CancelPairing gives up the pairing in progress in either direction: a request
// this device was answering is dropped, and a pairing this device started is
// abandoned, which also tells the other device so its dialog closes.
func (s *Service) CancelPairing() {
	if session := s.pairing.unresolved(); session != nil {
		s.pairing.resolve(session, ErrPairingCanceled)
	}

	s.mu.RLock()
	address, client := s.pairAddress, s.pairClient
	s.mu.RUnlock()
	if address == "" || client == nil {
		return
	}
	// Telling the other device is what ends the waiting request there; the
	// initiating side then reads the outcome as a cancellation.
	s.goRun(func(ctx context.Context) {
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+address+pathPairCancel, nil)
		if err != nil {
			return
		}
		response, err := client.Do(request)
		if err != nil {
			return
		}
		response.Body.Close()
	})
}

// SetFileLimitBytes changes the size limit for one file item, as the settings
// page does when the user edits it. It applies to the next delivery and to the
// next offer this device accepts.
func (s *Service) SetFileLimitBytes(limit int64) {
	s.mu.Lock()
	s.fileLimit = limit
	s.mu.Unlock()
}

// fileLimitBytes returns the limit in force, never below one byte so a zero
// option cannot mean "refuse every file".
func (s *Service) fileLimitBytes() int64 {
	s.mu.RLock()
	limit := s.fileLimit
	s.mu.RUnlock()
	if limit <= 0 {
		return DefaultFileLimitBytes
	}
	return limit
}

// SetDeviceSync turns one device's sync switch on or off and persists it.
func (s *Service) SetDeviceSync(deviceID string, enabled bool) error {
	device, ok := s.store.Device(deviceID)
	if !ok {
		return ErrUnknownPeer
	}
	device.SyncEnabled = enabled
	if err := s.store.Put(device); err != nil {
		return err
	}
	s.devicesChanged()
	return nil
}

// SetDirection sets one content type's direction with one device and persists
// it.
func (s *Service) SetDirection(deviceID string, contentType ContentType, direction Direction) error {
	device, ok := s.store.Device(deviceID)
	if !ok {
		return ErrUnknownPeer
	}
	device.Directions[contentType] = direction
	if err := s.store.Put(device); err != nil {
		return err
	}
	s.devicesChanged()
	return nil
}

// RemoveDevice unpairs one device and returns the record it removed, so the
// caller can offer to undo.
func (s *Service) RemoveDevice(deviceID string) (Device, bool, error) {
	device, removed, err := s.store.Remove(deviceID)
	if err != nil || !removed {
		return device, removed, err
	}
	s.devicesChanged()
	return device, true, nil
}

// RestoreDevice puts a device removed moments ago back, for undo.
func (s *Service) RestoreDevice(device Device) error {
	if err := s.store.Put(device); err != nil {
		return err
	}
	s.devicesChanged()
	return nil
}

// Send delivers one locally captured item to every device that should get it.
// It returns immediately; the outcomes arrive through OnDelivery.
//
// An item with no target at all — no paired device wants this content type, or
// sync is off everywhere — reports nothing, which leaves the item with no sync
// record and keeps the preview's sync section hidden.
func (s *Service) Send(item history.Item) {
	if item.SourceDeviceID != "" {
		// A received item has already arrived somewhere; only local captures
		// travel on.
		return
	}
	clip := ClipOf(item)
	targets := s.targets(clip.Kind)
	if len(targets) == 0 {
		return
	}
	s.goRun(func(context.Context) { s.deliver(item.ID, clip, targets) })
}

// Resend delivers one item to one device again, as the preview's resend action
// asks for.
func (s *Service) Resend(item history.Item, deviceID string) {
	clip := ClipOf(item)
	device, ok := s.store.Device(deviceID)
	if !ok {
		return
	}
	devices := s.mergeDevices([]Device{device})
	if len(devices) == 0 || !devices[0].Sends(ContentTypeOf(clip.Kind)) {
		return
	}
	s.goRun(func(context.Context) { s.deliver(item.ID, clip, devices) })
}

// targets returns the devices an item of one kind should be delivered to.
func (s *Service) targets(kind history.Kind) []Device {
	contentType := ContentTypeOf(kind)
	devices := s.mergeDevices(s.store.Devices())
	targets := make([]Device, 0, len(devices))
	for _, device := range devices {
		if device.Sends(contentType) {
			targets = append(targets, device)
		}
	}
	return targets
}

// deliver sends one clip to every target and reports the outcomes together.
func (s *Service) deliver(itemID string, clip Clip, targets []Device) {
	deliveries := s.deliveriesOf(clip, targets)
	if s.options.OnDelivery != nil {
		s.options.OnDelivery(itemID, deliveries)
	}
}

// deliveriesOf sends one clip to every target and returns one outcome per
// target, in the same order.
//
// A file payload takes a different road from text and images: its bytes are
// streamed through the file transfer endpoints, and a payload that is too large
// is refused here without touching the network at all.
func (s *Service) deliveriesOf(clip Clip, targets []Device) []history.DeviceDelivery {
	if clip.Kind == history.KindFile {
		return s.deliverFiles(clip, targets)
	}
	deliveries := make([]history.DeviceDelivery, len(targets))
	var wg sync.WaitGroup
	for index, device := range targets {
		index, device := index, device
		wg.Add(1)
		go func() {
			defer wg.Done()
			deliveries[index] = history.DeviceDelivery{
				DeviceID: device.ID,
				State:    s.deliverTo(clip, device),
			}
		}()
	}
	wg.Wait()
	return deliveries
}

// deliverTo sends one clip to one device and returns the outcome.
func (s *Service) deliverTo(clip Clip, device Device) history.DeliveryState {
	if !device.Online || device.Addr == nil {
		return history.DeliveryOffline
	}
	address := net.JoinHostPort(device.Addr.String(), fmt.Sprint(device.Port))
	payload, err := json.Marshal(payloadOf(clip))
	if err != nil {
		return history.DeliveryOffline
	}
	s.mu.RLock()
	serviceCtx := s.ctx
	s.mu.RUnlock()
	if serviceCtx == nil {
		return history.DeliveryOffline
	}
	ctx, cancel := context.WithTimeout(serviceCtx, requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+address+pathClip, bytes.NewReader(payload))
	if err != nil {
		return history.DeliveryOffline
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		// The device is not answering where discovery saw it: treat it as
		// gone until it is seen again.
		s.markUnreachable(device.ID)
		return history.DeliveryOffline
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return history.DeliveryOffline
	}
	var reply clipReply
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<16)).Decode(&reply); err != nil {
		return history.DeliveryOffline
	}
	if reply.Status != clipStatusReceived {
		// The other device refused this content type: nothing was delivered,
		// and nothing will be until one side's settings change.
		return history.DeliveryPending
	}
	s.markReachable(device.ID)
	return history.DeliveryReceived
}

// handleClip accepts one payload from a paired device.
func (s *Service) handleClip(w http.ResponseWriter, r *http.Request) {
	device, ok := pairedPeer(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	body := http.MaxBytesReader(w, r.Body, maxClipBytes)
	var payload clipPayload
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, clipReply{Status: clipStatusIgnored, Reason: clipReasonSize})
			return
		}
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	clip, err := payload.clip()
	if err != nil {
		writeJSON(w, http.StatusOK, clipReply{Status: clipStatusIgnored, Reason: clipReasonVersion})
		return
	}
	if clip.Kind == history.KindFile {
		// File payloads carry bytes, so they have their own endpoints; a file
		// item that arrived here would have no content to store.
		writeJSON(w, http.StatusOK, clipReply{Status: clipStatusIgnored, Reason: clipReasonKind})
		return
	}
	// The receiving side's own settings decide: a device set to send only does
	// not accept content, whatever the sender believes.
	if !device.Receives(ContentTypeOf(clip.Kind)) {
		writeJSON(w, http.StatusOK, clipReply{Status: clipStatusIgnored, Reason: clipReasonDirection})
		return
	}
	if s.options.OnClip != nil {
		s.options.OnClip(clip, device)
	}
	s.markReachable(device.ID)
	writeJSON(w, http.StatusOK, clipReply{Status: clipStatusReceived})
}

// clientFor returns a client whose connections are pinned to device. Pairing
// needs it: the peer is discovered but not paired yet, so the shared client —
// which only resolves paired devices — would refuse to dial it.
func (s *Service) clientFor(device Device, address string) *http.Client {
	client := newHTTPClient(s.identity, func(candidate string) (Device, bool) {
		if candidate != address {
			return Device{}, false
		}
		return device, true
	})
	// Pairing waits for a person on the other device, so its deadline comes
	// from the request context rather than a fixed client timeout.
	client.Timeout = 0
	return client
}

// deviceAt resolves a dialled address back to a paired device, so the client
// only ever connects to a pinned key.
func (s *Service) deviceAt(address string) (Device, bool) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return Device{}, false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return Device{}, false
	}
	for _, device := range s.mergeDevices(s.store.Devices()) {
		if device.Addr == nil || !device.Addr.Equal(ip) || fmt.Sprint(device.Port) != port {
			continue
		}
		return device, true
	}
	return Device{}, false
}

// peerSighted records one sighting. Presence decays on a clock, so the device
// list must see every sighting rather than only the ones that change the
// published peer list; otherwise a device that keeps announcing looks offline
// as soon as the last published list is older than peerExpiry.
func (s *Service) peerSighted(peer Peer) {
	s.mu.Lock()
	s.peers[peer.ID] = peer
	s.mu.Unlock()
}

// peersChanged handles a discovery update: presence is merged into the paired
// devices and the application is told what changed.
func (s *Service) peersChanged(peers []Peer) {
	updated := make(map[string]Peer, len(peers))
	for _, peer := range peers {
		updated[peer.ID] = peer
	}
	s.mu.Lock()
	s.peers = updated
	s.mu.Unlock()

	if s.options.OnPeersChanged != nil {
		s.options.OnPeersChanged(peers)
	}
	s.devicesChanged()
}

// markUnreachable records that a device did not answer where discovery saw it.
func (s *Service) markUnreachable(deviceID string) {
	s.mu.Lock()
	s.unreachable[deviceID] = s.now()
	s.mu.Unlock()
	s.devicesChanged()
}

// markReachable clears a previous failure, so the device counts as online
// between discovery rounds.
func (s *Service) markReachable(deviceID string) {
	s.mu.Lock()
	delete(s.unreachable, deviceID)
	if peer, ok := s.peers[deviceID]; ok {
		peer.Seen = s.now()
		s.peers[deviceID] = peer
	}
	s.mu.Unlock()
}

// mergeDevices fills in where each paired device was last seen, and whether it
// is reachable right now.
func (s *Service) mergeDevices(devices []Device) []Device {
	s.mu.RLock()
	peers := make(map[string]Peer, len(s.peers))
	for id, peer := range s.peers {
		peers[id] = peer
	}
	unreachable := make(map[string]time.Time, len(s.unreachable))
	for id, at := range s.unreachable {
		unreachable[id] = at
	}
	s.mu.RUnlock()

	now := s.now()
	merged := make([]Device, len(devices))
	for index, device := range devices {
		device.Addr = nil
		device.Port = 0
		device.Online = false
		if peer, ok := peers[device.ID]; ok {
			device.Addr = peer.Addr
			device.Port = peer.Port
			device.LastSeen = peer.Seen
			device.Online = peer.Online(now)
			if peer.Name != "" {
				// The device reports its own name, so a rename on that side
				// reaches this list without pairing again.
				device.Name = peer.Name
			}
			if peer.Platform != "" {
				device.Platform = peer.Platform
			}
		}
		if at, ok := unreachable[device.ID]; ok && now.Sub(at) < unreachableWindow {
			device.Online = false
		}
		merged[index] = device
	}
	return merged
}

// portOf returns the port the listener is on, for the announcement.
func (s *Service) portOf() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}

// devicesChanged reports the current device list.
func (s *Service) devicesChanged() {
	if s.options.OnDevicesChanged == nil {
		return
	}
	s.options.OnDevicesChanged(s.Devices())
}
