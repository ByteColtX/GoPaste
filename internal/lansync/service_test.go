package lansync

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gopaste/internal/history"
)

// testService is one running Service with the callbacks it fired recorded.
type testService struct {
	service *Service
	port    int

	// codes and pairFailures carry the pairing callbacks, so a test can drive
	// a real pairing between two services.
	codes        chan string
	pairFailures chan error

	mu         sync.Mutex
	clips      []Clip
	from       []Device
	deliveries map[string][]history.DeviceDelivery
	devices    []Device
}

// newTestService starts a service on a free port without discovery: the tests
// hand each service the peer list, so they do not depend on multicast reaching
// anywhere.
func newTestService(t *testing.T, name string) *testService {
	t.Helper()
	recorder := &testService{
		deliveries:   map[string][]history.DeviceDelivery{},
		codes:        make(chan string, 4),
		pairFailures: make(chan error, 4),
	}
	service, err := NewService(Options{
		StorePath: filepath.Join(t.TempDir(), "devices.json"),
		Name:      name,
		Platform:  "macOS",
		Version:   "0.1.0",
		OnClip: func(clip Clip, from Device) {
			recorder.mu.Lock()
			defer recorder.mu.Unlock()
			recorder.clips = append(recorder.clips, clip)
			recorder.from = append(recorder.from, from)
		},
		OnDelivery: func(itemID string, deliveries []history.DeviceDelivery) {
			recorder.mu.Lock()
			defer recorder.mu.Unlock()
			recorder.deliveries[itemID] = deliveries
		},
		OnDevicesChanged: func(devices []Device) {
			recorder.mu.Lock()
			defer recorder.mu.Unlock()
			recorder.devices = devices
		},
		OnPairingCode: func(_ string, code string) {
			select {
			case recorder.codes <- code:
			default:
			}
		},
		OnPairingFailed: func(_ string, err error) {
			select {
			case recorder.pairFailures <- err:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	service.noDiscovery = true
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(service.Stop)
	recorder.service = service
	recorder.port = service.portOf()
	return recorder
}

// peer is how the other device's discovery would see this one.
func (s *testService) peer() Peer {
	return Peer{
		ID:        s.service.Identity().ID,
		Name:      s.service.Name(),
		Platform:  "macOS",
		PublicKey: s.service.Identity().Public,
		Addr:      net.ParseIP("127.0.0.1"),
		Port:      s.port,
		Seen:      time.Now(),
	}
}

// record is the device record the other side would have pinned.
func (s *testService) record() Device {
	return Device{
		ID:          s.service.Identity().ID,
		Name:        s.service.Name(),
		Platform:    "macOS",
		PublicKey:   s.service.Identity().Public,
		SyncEnabled: true,
	}
}

// pairWith pins each service's key in the other's store and lets both see each
// other, which is the state a completed pairing leaves behind.
func pairWith(t *testing.T, a, b *testService) {
	t.Helper()
	if err := a.service.store.Put(b.record()); err != nil {
		t.Fatalf("pin %s in %s: %v", b.service.Name(), a.service.Name(), err)
	}
	if err := b.service.store.Put(a.record()); err != nil {
		t.Fatalf("pin %s in %s: %v", a.service.Name(), b.service.Name(), err)
	}
	a.service.peersChanged([]Peer{b.peer()})
	b.service.peersChanged([]Peer{a.peer()})
}

// textItem builds a local text item with its content-derived ID.
func textItem(t *testing.T, text string) history.Item {
	t.Helper()
	content := history.Content{Kind: history.KindText, Text: text}
	return history.Item{
		ID:        history.ContentID(content),
		Kind:      history.KindText,
		Text:      text,
		CreatedAt: time.Now(),
	}
}

// deliveryFor returns the outcome recorded for one item and device.
func (s *testService) deliveryFor(t *testing.T, itemID, deviceID string) history.DeliveryState {
	t.Helper()
	waitFor(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.deliveries[itemID]) > 0
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, delivery := range s.deliveries[itemID] {
		if delivery.DeviceID == deviceID {
			return delivery.State
		}
	}
	t.Fatalf("no delivery recorded for device %s", deviceID)
	return history.DeliveryPending
}

// received returns the clips this service received.
func (s *testService) received() []Clip {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Clip(nil), s.clips...)
}

// TestSendDeliversClip 检查一条本地记录能送到已配对设备：对方收到同样的内容，
// 且用内容算出的 ID 与本机一致。
func TestSendDeliversClip(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	item := textItem(t, "hello from the LAN")
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryReceived; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	waitFor(t, func() bool { return len(receiver.received()) == 1 })
	clips := receiver.received()
	if clips[0].Text != "hello from the LAN" {
		t.Errorf("received text = %q", clips[0].Text)
	}
	if got := clips[0].ID(); got != item.ID {
		t.Errorf("received ID = %q, want the sender's %q", got, item.ID)
	}

	receiver.mu.Lock()
	from := receiver.from[0]
	receiver.mu.Unlock()
	if from.ID != sender.service.Identity().ID {
		t.Errorf("clip came from %q, want %q", from.ID, sender.service.Identity().ID)
	}

	// 收到的条目带上来源设备，ID 仍是同一个。
	stored := clips[0].HistoryItem(from.ID, time.Now())
	if stored.SourceDeviceID != sender.service.Identity().ID {
		t.Error("a received item must carry the sending device")
	}
	if stored.ID != item.ID {
		t.Error("a received item must keep the content-derived ID")
	}
	if stored.IsLocal() {
		t.Error("a received item is not local")
	}
}

// TestImageTravelsIntact 检查图片载荷原样送到，尺寸信息也在。
func TestImageTravelsIntact(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	image := &history.Image{PNG: []byte("\x89PNG\r\n\x1a\nfake"), Width: 12, Height: 8}
	content := history.Content{Kind: history.KindImage, Image: image}
	item := history.Item{ID: history.ContentID(content), Kind: history.KindImage, Image: image, CreatedAt: time.Now()}
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryReceived; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	waitFor(t, func() bool { return len(receiver.received()) == 1 })
	clip := receiver.received()[0]
	if clip.Image == nil || string(clip.Image.PNG) != string(image.PNG) {
		t.Fatal("the image payload must arrive unchanged")
	}
	if clip.Image.Width != 12 || clip.Image.Height != 8 {
		t.Errorf("image size = %dx%d, want 12x8", clip.Image.Width, clip.Image.Height)
	}
}

// TestFileItemTravelsAsPaths 检查文件项只同步路径，不传文件内容，接收端存成
// 文件条目、不读盘。
func TestFileItemTravelsAsPaths(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	paths := []string{"/Users/bytecolt/report.pdf", "/Users/bytecolt/notes.md"}
	content := history.Content{Kind: history.KindFile, Files: paths}
	item := history.Item{
		ID:        history.ContentID(content),
		Kind:      history.KindFile,
		Text:      "/Users/bytecolt/report.pdf\n/Users/bytecolt/notes.md",
		CreatedAt: time.Now(),
	}
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryReceived; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	waitFor(t, func() bool { return len(receiver.received()) == 1 })
	clip := receiver.received()[0]
	if len(clip.Files) != 2 || clip.Files[0] != paths[0] {
		t.Errorf("files = %v, want %v", clip.Files, paths)
	}
	stored := clip.HistoryItem(sender.service.Identity().ID, time.Now())
	if stored.FilePreview != nil {
		t.Error("a received file item must not carry a preview: the file is elsewhere")
	}
	if stored.ID != item.ID {
		t.Error("the file item must keep its content-derived ID")
	}
}

// TestReceiverDirectionRefusesContent 检查接收端自己的方向设置说了算：设成「仅
// 发送」就不收，发送方看到的是未送达而不是已送达。
func TestReceiverDirectionRefusesContent(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	// 接收端把「来自发送方的文本」设成仅发送，等于拒收。
	record, ok := receiver.service.store.Device(sender.service.Identity().ID)
	if !ok {
		t.Fatal("the receiver must have pinned the sender")
	}
	record.Directions[ContentText] = DirectionSendOnly
	if err := receiver.service.store.Put(record); err != nil {
		t.Fatalf("Put: %v", err)
	}

	item := textItem(t, "not for you")
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryPending; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	if clips := receiver.received(); len(clips) != 0 {
		t.Errorf("the receiver must not accept refused content, got %d clips", len(clips))
	}
}

// TestSenderDirectionSkipsContent 检查发送端方向设成「关闭」时根本不发，也不留
// 投递记录。
func TestSenderDirectionSkipsContent(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	record, ok := sender.service.store.Device(receiver.service.Identity().ID)
	if !ok {
		t.Fatal("the sender must have pinned the receiver")
	}
	record.Directions[ContentText] = DirectionOff
	if err := sender.service.store.Put(record); err != nil {
		t.Fatalf("Put: %v", err)
	}

	item := textItem(t, "stays here")
	sender.service.Send(item)

	// 没有目标设备，就不该有任何投递回调。
	time.Sleep(100 * time.Millisecond)
	sender.mu.Lock()
	deliveries := len(sender.deliveries[item.ID])
	sender.mu.Unlock()
	if deliveries != 0 {
		t.Error("an item with no target device must not report a delivery")
	}
	if clips := receiver.received(); len(clips) != 0 {
		t.Error("nothing must reach the receiver")
	}
}

// TestOfflineDeviceIsReportedOffline 检查设备不在线时不尝试连接，直接记为离线。
func TestOfflineDeviceIsReportedOffline(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	// 只配对，不让发送方看到对方在线。
	if err := sender.service.store.Put(receiver.record()); err != nil {
		t.Fatalf("pin: %v", err)
	}

	item := textItem(t, "wait for me")
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryOffline; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
}

// TestUnpairedPeerIsRefused 检查只有一边记录了配对时内容送不过去：接收端按信任
// 集合拒收，发送方记为未送达。
func TestUnpairedPeerIsRefused(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	// 发送方钉住了对方，对方却不认识它。
	if err := sender.service.store.Put(receiver.record()); err != nil {
		t.Fatalf("pin: %v", err)
	}
	sender.service.peersChanged([]Peer{receiver.peer()})

	item := textItem(t, "stranger")
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryOffline; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	if clips := receiver.received(); len(clips) != 0 {
		t.Error("an unpaired sender must not deliver anything")
	}
}

// TestResendDeliversAgain 检查重发把同一条记录再送一次，用于设备重新上线后补发。
func TestResendDeliversAgain(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	item := textItem(t, "send me twice")
	sender.service.Resend(item, receiver.service.Identity().ID)
	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryReceived; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	waitFor(t, func() bool { return len(receiver.received()) == 1 })

	sender.service.Resend(item, receiver.service.Identity().ID)
	waitFor(t, func() bool { return len(receiver.received()) == 2 })
}

// TestReceivedItemsAreNotForwarded 检查从别的设备收到的内容不会再被转发出去：
// 只有本机复制的内容才同步。
func TestReceivedItemsAreNotForwarded(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	item := textItem(t, "forwarded?")
	item.SourceDeviceID = "another-device"
	sender.service.Send(item)

	time.Sleep(100 * time.Millisecond)
	sender.mu.Lock()
	deliveries := len(sender.deliveries[item.ID])
	sender.mu.Unlock()
	if deliveries != 0 {
		t.Error("a received item must not be delivered onwards")
	}
}

// TestDevicesMergePresence 检查已配对设备带上在线状态与地址，界面据此显示
// 在线/离线与连接信息。
func TestDevicesMergePresence(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	devices := sender.service.Devices()
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(devices))
	}
	device := devices[0]
	if !device.Online {
		t.Error("a device discovery just saw must be online")
	}
	if device.Addr == nil || device.Port != receiver.port {
		t.Errorf("address = %v:%d, want 127.0.0.1:%d", device.Addr, device.Port, receiver.port)
	}
	if device.LastSeen.IsZero() {
		t.Error("the last sighting must be recorded")
	}

	// 设备消失后转为离线。
	sender.service.peersChanged(nil)
	if devices := sender.service.Devices(); devices[0].Online {
		t.Error("a device discovery no longer sees must be offline")
	}
}

// fakeClock is a controllable clock for the presence tests.
type fakeClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *fakeClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(by)
}

// TestPresenceStaysFreshWhileThePeerKeepsAnnouncing 检查在线状态既不早退也不滞留：
// 设备一直在公告就一直在线上（即使已经超过 peerExpiry），停止公告 peerExpiry 之后
// 才转为离线。只按「列表变化」刷新的话，设备会在最后一次发布之后 20 秒莫名其妙变成
// 离线，同步于是静悄悄地停掉。
func TestPresenceStaysFreshWhileThePeerKeepsAnnouncing(t *testing.T) {
	clock := &fakeClock{at: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	service, err := NewService(Options{
		StorePath: filepath.Join(t.TempDir(), "devices.json"),
		Name:      "MacBook Pro",
		Platform:  "macOS",
		Version:   "test",
		Now:       clock.now,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	service.noDiscovery = true
	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer service.Stop()

	device := Device{
		ID:          identity.ID,
		Name:        "工作室 PC",
		Platform:    "Windows",
		PublicKey:   identity.Public,
		SyncEnabled: true,
	}
	if err := service.store.Put(device); err != nil {
		t.Fatalf("Put: %v", err)
	}
	peer := Peer{
		ID:        identity.ID,
		Name:      "工作室 PC",
		Platform:  "Windows",
		PublicKey: identity.Public,
		Addr:      net.ParseIP("192.168.12.10"),
		Port:      47823,
		Seen:      clock.now(),
	}

	service.peerSighted(peer)
	clock.advance(15 * time.Second)
	peer.Seen = clock.now()
	service.peerSighted(peer)

	// 距第一次 30 秒，但距最近一次只有 15 秒：仍然在线。
	clock.advance(15 * time.Second)
	devices := service.Devices()
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(devices))
	}
	if !devices[0].Online {
		t.Error("a device that keeps announcing must stay online past peerExpiry")
	}

	// 停止公告之后再过 peerExpiry，才算离线。
	clock.advance(peerExpiry)
	if devices := service.Devices(); devices[0].Online {
		t.Error("a device that stopped announcing must go offline")
	}
}

// TestUnpairAndUndo 检查取消配对返回被移除的记录，撤销后设备回到列表。
func TestUnpairAndUndo(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	removed, ok, err := sender.service.RemoveDevice(receiver.service.Identity().ID)
	if err != nil || !ok {
		t.Fatalf("RemoveDevice = %v, %v", ok, err)
	}
	if len(sender.service.store.Devices()) != 0 {
		t.Error("the device must be gone")
	}
	if err := sender.service.RestoreDevice(removed); err != nil {
		t.Fatalf("RestoreDevice: %v", err)
	}
	if len(sender.service.store.Devices()) != 1 {
		t.Error("the undo must put the device back")
	}
}

// TestServicePairsTwoDevices 检查服务级的配对：一台发起、显示配对码，另一台用这个码
// 确认，双方各自钉住对方。它覆盖的正是「对端还没配对时也要能连上」这条路径——共享
// 客户端只认已配对设备，配对必须走自己钉住对端的连接。
func TestServicePairsTwoDevices(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	// 互相发现，但还没有配对记录。
	sender.service.peersChanged([]Peer{receiver.peer()})
	receiver.service.peersChanged([]Peer{sender.peer()})

	if err := sender.service.Pair(receiver.service.Identity().ID); err != nil {
		t.Fatalf("Pair: %v", err)
	}
	var code string
	select {
	case code = <-sender.codes:
	case err := <-sender.pairFailures:
		t.Fatalf("pairing failed before showing a code: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("no pairing code was shown")
	}
	waitFor(t, func() bool {
		_, ok := receiver.service.PendingPairRequest()
		return ok
	})

	if err := receiver.service.ConfirmPairing(code); err != nil {
		t.Fatalf("ConfirmPairing: %v", err)
	}

	waitFor(t, func() bool {
		_, paired := sender.service.store.Device(receiver.service.Identity().ID)
		return paired
	})
	device, paired := receiver.service.store.Device(sender.service.Identity().ID)
	if !paired {
		t.Fatal("the responding side must pin the initiating device")
	}
	if !device.SyncEnabled {
		t.Error("a freshly paired device must have sync on")
	}
	if device.PublicKey == nil || !device.PublicKey.Equal(sender.service.Identity().Public) {
		t.Error("the pinned key must be the initiating device's key")
	}
}

// TestSettingsPersist 检查设备开关与方向设置落盘：重启后仍然是用户选的那套。
func TestSettingsPersist(t *testing.T) {
	service := newTestService(t, "MacBook Pro")
	peer := newTestService(t, "工作室 PC")
	if err := service.service.store.Put(peer.record()); err != nil {
		t.Fatalf("pin: %v", err)
	}
	deviceID := peer.service.Identity().ID

	if err := service.service.SetDirection(deviceID, ContentImage, DirectionReceiveOnly); err != nil {
		t.Fatalf("SetDirection: %v", err)
	}
	if err := service.service.SetDeviceSync(deviceID, false); err != nil {
		t.Fatalf("SetDeviceSync: %v", err)
	}

	reopened, err := OpenStore(service.service.StorePath(), "MacBook Pro", "macOS")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	device, ok := reopened.Device(deviceID)
	if !ok {
		t.Fatal("the device must survive a reopen")
	}
	if got, want := device.Direction(ContentImage), DirectionReceiveOnly; got != want {
		t.Errorf("image direction = %v, want %v", got, want)
	}
	if device.SyncEnabled {
		t.Error("the sync switch must stay off")
	}
}

// TestServiceStartsRealDiscovery 检查真实环境下的启动路径：监听端口、绑定网卡、
// 起 mDNS responder 都不报错，停止时也干净地收回来。它默认跳过，因为组播依赖
// 机器所在的网络；GOPASTE_LIVE_MDNS=1 时才跑。
func TestServiceStartsRealDiscovery(t *testing.T) {
	if os.Getenv("GOPASTE_LIVE_MDNS") == "" {
		t.Skip("set GOPASTE_LIVE_MDNS=1 to run the live discovery test")
	}
	service, err := NewService(Options{
		StorePath: filepath.Join(t.TempDir(), "devices.json"),
		Name:      "测试设备",
		Platform:  "macOS",
		Version:   "0.1.0",
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if port := service.portOf(); port <= 0 {
		t.Errorf("port = %d, want the listening port", port)
	}
	// 发现至少跑起来了：列表可以是空的（同网段没有第二台设备）。
	time.Sleep(500 * time.Millisecond)
	service.Refresh()
	service.Stop()
}
