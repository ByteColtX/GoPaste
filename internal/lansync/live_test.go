package lansync

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopaste/internal/history"
)

// livePeerPrefix marks the device names the two-machine test uses, so it never
// pairs with a real GoPaste instance that happens to be running.
const livePeerPrefix = "gopaste-live-"

// TestLivePeerSync exercises the whole package between two machines on the same
// network: mDNS discovery across hosts, the pinned TLS connection, and clipboard
// delivery in both directions.
//
// It is skipped by default because it needs a second machine. To run it, start
// the same test binary on both machines at the same time with
// GOPASTE_LIVE_PEER=1 and a distinct GOPASTE_LIVE_NAME on each:
//
//	GOPASTE_LIVE_PEER=1 GOPASTE_LIVE_NAME=gopaste-live-mac go test -run TestLivePeerSync ./internal/lansync
//
// Pairing itself is covered by the loopback tests; this one pins the discovered
// peer directly, which is the state a completed pairing leaves behind, so the
// two sides can run unattended.
func TestLivePeerSync(t *testing.T) {
	if os.Getenv("GOPASTE_LIVE_PEER") == "" {
		t.Skip("set GOPASTE_LIVE_PEER=1 and run this test on two machines to test real LAN sync")
	}
	name := os.Getenv("GOPASTE_LIVE_NAME")
	if name == "" {
		host, err := os.Hostname()
		if err != nil {
			t.Fatalf("hostname: %v", err)
		}
		name = livePeerPrefix + host
	}
	if !strings.HasPrefix(name, livePeerPrefix) {
		t.Fatalf("GOPASTE_LIVE_NAME must start with %q, got %q", livePeerPrefix, name)
	}

	received := make(chan Clip, 8)
	delivered := make(chan []history.DeviceDelivery, 8)
	service, err := NewService(Options{
		StorePath: filepath.Join(t.TempDir(), "devices.json"),
		Name:      name,
		Platform:  runtime.GOOS,
		Version:   "live",
		FilesDir:  filepath.Join(t.TempDir(), "Files"),
		OnClip: func(clip Clip, _ Device) {
			select {
			case received <- clip:
			default:
			}
		},
		OnDelivery: func(_ string, deliveries []history.DeviceDelivery) {
			select {
			case delivered <- deliveries:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer service.Stop()
	t.Logf("%s: listening on %d, peer ID %s", name, service.portOf(), service.Identity().ID)

	// 1. Find the other machine through discovery.
	peer := waitForPeer(t, service, 90*time.Second)
	t.Logf("%s: found %q at %s:%d", name, peer.Name, peer.Addr, peer.Port)

	// 2. Pin it, which is what a completed pairing leaves behind.
	device := Device{
		ID:          peer.ID,
		Name:        peer.Name,
		Platform:    peer.Platform,
		PublicKey:   peer.PublicKey,
		SyncEnabled: true,
		PairedAt:    time.Now(),
	}
	if err := service.store.Put(device); err != nil {
		t.Fatalf("pin the peer: %v", err)
	}
	waitForOnline(t, service, peer.ID, 60*time.Second)

	// 3. Deliver one payload of every content type, retrying until the other
	// side has pinned this device too (until then it answers 403).
	payloads := []history.Item{
		liveText(name),
		liveRichText(name),
		liveImage(name),
		liveFile(t, name),
	}
	for _, payload := range payloads {
		waitForDelivery(t, service, delivered, payload, peer.ID, 90*time.Second)
	}

	// 4. The other machine delivers the same way; wait for every payload.
	for _, kind := range []history.Kind{
		history.KindText,
		history.KindRichText,
		history.KindImage,
		history.KindFile,
	} {
		waitForClip(t, received, kind, 90*time.Second)
	}
	t.Logf("%s: received all four payloads from %q", name, peer.Name)
}

// waitForPeer waits for the other machine to appear through discovery.
func waitForPeer(t *testing.T, service *Service, timeout time.Duration) Peer {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, peer := range service.Peers() {
			if strings.HasPrefix(peer.Name, livePeerPrefix) && peer.Name != service.Name() {
				return peer
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("no peer with the %q prefix appeared in %s; seen: %+v", livePeerPrefix, timeout, service.Peers())
	return Peer{}
}

// waitForOnline waits until the pinned device is reachable.
func waitForOnline(t *testing.T, service *Service, deviceID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, device := range service.Devices() {
			if device.ID == deviceID && device.Online {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("the pinned device never came online; devices: %+v", service.Devices())
}

// liveText is the text payload this side delivers: unique per device, so the
// other side can tell it apart from its own.
func liveText(name string) history.Item {
	text := "gopaste live from " + name + " at " + time.Now().UTC().Format(time.RFC3339)
	return history.Item{
		ID:        history.ContentID(history.Content{Kind: history.KindText, Text: text}),
		Kind:      history.KindText,
		Text:      text,
		CreatedAt: time.Now(),
	}
}

// liveRichText is the rich-text payload this side delivers: the markup carries
// the device name, so the other side can check both representations arrived.
func liveRichText(name string) history.Item {
	html := "<p><strong>gopaste live</strong> from " + name + "</p>"
	text := "gopaste live from " + name
	return history.Item{
		ID:        history.ContentID(history.Content{Kind: history.KindRichText, HTML: html, Text: text}),
		Kind:      history.KindRichText,
		HTML:      html,
		Text:      text,
		CreatedAt: time.Now(),
	}
}

// liveFile is the file payload this side delivers: a small directory with two
// files in it, so the other machine has to unpack a real archive. The paths are
// the sending machine's own, and its content travels with the delivery.
func liveFile(t *testing.T, name string) history.Item {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name+"-files")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the file payload: %v", err)
	}
	for _, file := range []string{"report.txt", "notes.md"} {
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, []byte("gopaste live file "+name+" "+file), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	paths := []string{dir}
	return history.Item{
		ID:        history.ContentID(history.Content{Kind: history.KindFile, Files: paths}),
		Kind:      history.KindFile,
		Text:      strings.Join(paths, "\n"),
		CreatedAt: time.Now(),
	}
}

// liveImage is the image payload this side delivers: a small PNG whose colour
// encodes the device name, so the other side can check it arrived intact.
func liveImage(name string) history.Item {
	sum := crc32.ChecksumIEEE([]byte(name))
	image := &history.Image{PNG: solidPNG(sum), Width: 8, Height: 8}
	return history.Item{
		ID:        history.ContentID(history.Content{Kind: history.KindImage, Image: image}),
		Kind:      history.KindImage,
		Image:     image,
		CreatedAt: time.Now(),
	}
}

// solidPNG encodes an 8×8 PNG whose pixel bytes carry seed.
func solidPNG(seed uint32) []byte {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], seed)
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: raw[0], G: raw[1], B: raw[2], A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}

// waitForDelivery delivers item until the peer reports it received it.
func waitForDelivery(t *testing.T, service *Service, delivered chan []history.DeviceDelivery, item history.Item, deviceID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		service.Send(item)
		select {
		case deliveries := <-delivered:
			for _, delivery := range deliveries {
				if delivery.DeviceID == deviceID && delivery.State == history.DeliveryReceived {
					return
				}
			}
		case <-time.After(3 * time.Second):
			continue
		}
	}
	t.Fatalf("the peer never reported receiving %s", item.ID)
}

// waitForClip waits for one payload of a kind to arrive from the other machine.
func waitForClip(t *testing.T, received chan Clip, kind history.Kind, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case clip := <-received:
			if clip.Kind != kind {
				continue
			}
			switch kind {
			case history.KindText:
				if !strings.HasPrefix(clip.Text, "gopaste live from ") {
					t.Errorf("unexpected text clip: %q", clip.Text)
				}
			case history.KindRichText:
				if !strings.Contains(clip.HTML, "gopaste live") {
					t.Errorf("the rich-text clip lost its markup: %q", clip.HTML)
				}
				if clip.Text == "" {
					t.Error("the rich-text clip lost its plain-text representation")
				}
			case history.KindImage:
				if clip.Image == nil || len(clip.Image.PNG) == 0 {
					t.Error("an image clip arrived without its payload")
				}
			case history.KindFile:
				if len(clip.LocalFiles) == 0 {
					t.Error("a file clip arrived without local paths")
					return
				}
				for _, path := range clip.LocalFiles {
					if _, err := os.Stat(path); err != nil {
						t.Errorf("a file clip points at %s, which is not there: %v", path, err)
					}
				}
			}
			return
		case <-deadline:
			t.Fatalf("no %v payload arrived in %s", kind, timeout)
		}
	}
}

// TestLivePairing drives a real pairing between two machines: one side shows the
// code, the other types it.
//
// It is skipped by default. To run it, start the responder on one machine and
// the initiator on the other:
//
//	GOPASTE_LIVE_PEER=1 GOPASTE_LIVE_NAME=gopaste-live-win GOPASTE_LIVE_ROLE=pair-responder \
//	    GOPASTE_LIVE_CODE_FILE=C:\Temp\gopaste-code.txt go test -run TestLivePairing ./internal/lansync
//	GOPASTE_LIVE_PEER=1 GOPASTE_LIVE_NAME=gopaste-live-mac GOPASTE_LIVE_ROLE=pair-initiator \
//	    go test -run TestLivePairing -v ./internal/lansync
//
// The initiator prints "CODE=<digits>"; write that code into the responder's code
// file and the pairing completes. The code travels outside the network on
// purpose: that is the whole point of the check.
func TestLivePairing(t *testing.T) {
	if os.Getenv("GOPASTE_LIVE_PEER") == "" {
		t.Skip("set GOPASTE_LIVE_PEER=1 and run this test on two machines to pair for real")
	}
	role := os.Getenv("GOPASTE_LIVE_ROLE")
	name := os.Getenv("GOPASTE_LIVE_NAME")
	if role != "pair-initiator" && role != "pair-responder" {
		t.Skipf("set GOPASTE_LIVE_ROLE to pair-initiator or pair-responder, got %q", role)
	}
	if name == "" {
		host, err := os.Hostname()
		if err != nil {
			t.Fatalf("hostname: %v", err)
		}
		name = livePeerPrefix + host
	}
	if !strings.HasPrefix(name, livePeerPrefix) {
		t.Fatalf("GOPASTE_LIVE_NAME must start with %q, got %q", livePeerPrefix, name)
	}

	codes := make(chan string, 4)
	service, err := NewService(Options{
		StorePath: filepath.Join(t.TempDir(), "devices.json"),
		Name:      name,
		Platform:  runtime.GOOS,
		Version:   "live",
		OnPairingCode: func(_ string, code string) {
			select {
			case codes <- code:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer service.Stop()
	t.Logf("%s: %s listening on %d, peer ID %s", name, role, service.portOf(), service.Identity().ID)

	peer := waitForPeer(t, service, 90*time.Second)
	t.Logf("%s: found %q at %s:%d", name, peer.Name, peer.Addr, peer.Port)

	switch role {
	case "pair-initiator":
		if err := service.Pair(peer.ID); err != nil {
			t.Fatalf("Pair: %v", err)
		}
		select {
		case code := <-codes:
			// 配对码是唯一需要人工（这里是我）搬运的东西：写到 stdout，并在
			// 设置了 GOPASTE_LIVE_CODE_OUT 时同时写进一个文件，方便编排脚本
			// 立刻取走——输出被管道缓冲时等它出现可能已经超过 60 秒窗口。
			fmt.Printf("CODE=%s\n", code)
			if path := os.Getenv("GOPASTE_LIVE_CODE_OUT"); path != "" {
				if err := os.WriteFile(path, []byte(code), 0o600); err != nil {
					t.Fatalf("write the pairing code: %v", err)
				}
			}
		case <-time.After(30 * time.Second):
			t.Fatal("no pairing code was shown")
		}
	case "pair-responder":
		codeFile := os.Getenv("GOPASTE_LIVE_CODE_FILE")
		if codeFile == "" {
			t.Fatal("GOPASTE_LIVE_CODE_FILE is required for the responder")
		}
		waitForPairRequest(t, service, 90*time.Second)
		code := waitForCodeFile(t, codeFile, 120*time.Second)
		if err := service.ConfirmPairing(code); err != nil {
			t.Fatalf("ConfirmPairing: %v", err)
		}
		t.Logf("%s: confirmed the code from %q", name, peer.Name)
	}

	waitForPaired(t, service, peer, 60*time.Second)
	t.Logf("%s: paired with %q", name, peer.Name)
}

// waitForPairRequest waits until the other machine asks to pair.
func waitForPairRequest(t *testing.T, service *Service, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if request, ok := service.PendingPairRequest(); ok {
			t.Logf("pairing request from %q (%s)", request.Name, request.Platform)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("no pairing request arrived")
}

// waitForCodeFile waits until the code the other machine showed is written to
// path, which is how it travels between the two machines in this test.
func waitForCodeFile(t *testing.T, path string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			code := strings.TrimSpace(string(data))
			if len(code) == pairCodeLength {
				return code
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("no pairing code appeared in %s", path)
	return ""
}

// waitForPaired waits until the paired device list contains the peer, pinned to
// the key it advertised.
func waitForPaired(t *testing.T, service *Service, peer Peer, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, device := range service.Devices() {
			if device.ID != peer.ID {
				continue
			}
			if !device.PublicKey.Equal(peer.PublicKey) {
				t.Fatalf("the paired key is not the advertised one")
			}
			if !device.SyncEnabled {
				t.Fatal("a freshly paired device must have sync on")
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("the pairing never completed; devices: %+v", service.Devices())
}
