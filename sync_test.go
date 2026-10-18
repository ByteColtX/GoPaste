package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net"
	"testing"
	"time"

	"gopaste/internal/history"
	"gopaste/internal/lansync"
)

// syncApp builds an app with an empty history and a recording clipboard, ready
// for the sync paths that touch both.
func syncApp() (*application, *recordingClipboard) {
	app := newApplication()
	app.pairedDevices = nil
	clipboard := &recordingClipboard{}
	app.clipboard = clipboard
	// 与生产一致：历史装载完成之后才开始收同步内容。
	app.installHistory(nil, nil)
	return app, clipboard
}

// remoteDevice builds a paired-device record as the sync service would report it.
func remoteDevice(id, name string) lansync.Device {
	return lansync.Device{
		ID:          id,
		Name:        name,
		Platform:    "Windows",
		PublicKey:   ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)),
		SyncEnabled: true,
		Addr:        net.ParseIP("192.168.1.42"),
		Port:        47823,
		Online:      true,
		LastSeen:    time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
	}
}

// TestSummarizeSync 检查一条记录的同步状态怎么从各设备的结果汇总出来。
func TestSummarizeSync(t *testing.T) {
	cases := []struct {
		name       string
		deliveries []history.DeviceDelivery
		want       syncState
	}{
		{
			name: "no target device",
			want: syncStateNone,
		},
		{
			name: "every device has it",
			deliveries: []history.DeviceDelivery{
				{DeviceID: "a", State: deliveryReceived},
				{DeviceID: "b", State: deliveryReceived},
			},
			want: syncStateSynced,
		},
		{
			name: "one of two",
			deliveries: []history.DeviceDelivery{
				{DeviceID: "a", State: deliveryReceived},
				{DeviceID: "b", State: deliveryOffline},
			},
			want: syncStatePartial,
		},
		{
			name: "none of them",
			deliveries: []history.DeviceDelivery{
				{DeviceID: "a", State: deliveryOffline},
				{DeviceID: "b", State: deliveryOffline},
			},
			want: syncStateOffline,
		},
		{
			// 对方拒收（方向设置不允许）也还是没送到，仍在等待投递。
			name: "refused by the other device",
			deliveries: []history.DeviceDelivery{
				{DeviceID: "a", State: deliveryPending},
			},
			want: syncStateOffline,
		},
		{
			// 文件超过上限不是「等对方上线」就能变好的事，所以汇总读「文件过大」。
			name: "refused as too large",
			deliveries: []history.DeviceDelivery{
				{DeviceID: "a", State: deliveryTooLarge},
			},
			want: syncStateTooLarge,
		},
		{
			// 一台设备收到、另一台因为体积没收到：还是部分同步。
			name: "one received, one too large",
			deliveries: []history.DeviceDelivery{
				{DeviceID: "a", State: deliveryReceived},
				{DeviceID: "b", State: deliveryTooLarge},
			},
			want: syncStatePartial,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := summarizeSync(tc.deliveries); got != tc.want {
				t.Errorf("summarizeSync = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPairedDeviceOfKeepsWhatTheDevicePageShows 检查服务里的设备记录被投影成界面
// 需要的那一份：地址、在线状态、开关与四种方向都不丢。
func TestPairedDeviceOfKeepsWhatTheDevicePageShows(t *testing.T) {
	device := remoteDevice("peer-a", "工作室 PC")
	device.Directions[lansync.ContentImage] = lansync.DirectionReceiveOnly
	device.Directions[lansync.ContentFile] = lansync.DirectionOff
	device.Directions[lansync.ContentRichText] = lansync.DirectionSendOnly

	projection := pairedDeviceOf(device)
	if projection.id != device.ID || projection.peerID != device.ID {
		t.Errorf("id/peerID = %q/%q, want %q", projection.id, projection.peerID, device.ID)
	}
	if projection.name != "工作室 PC" || projection.platform != "Windows" {
		t.Errorf("name/platform = %q/%q", projection.name, projection.platform)
	}
	if projection.address != "192.168.1.42:47823" {
		t.Errorf("address = %q, want the discovery address", projection.address)
	}
	if !projection.online {
		t.Error("the projection must keep the online state")
	}
	if !projection.syncEnabled {
		t.Error("the projection must keep the sync switch")
	}
	if got := projection.textSyncDirection; got != syncDirectionTwoWay {
		t.Errorf("text direction = %v, want two-way", got)
	}
	if got := projection.imageSyncDirection; got != syncDirectionReceiveOnly {
		t.Errorf("image direction = %v, want receive-only", got)
	}
	if got := projection.fileSyncDirection; got != syncDirectionOff {
		t.Errorf("file direction = %v, want off", got)
	}
	if got := projection.richTextSyncDirection; got != syncDirectionSendOnly {
		t.Errorf("rich text direction = %v, want send-only", got)
	}

	// 没有地址的设备不编一个假地址出来。
	offline := remoteDevice("peer-b", "笔记本")
	offline.Addr = nil
	offline.Port = 0
	if got := pairedDeviceOf(offline).address; got != "" {
		t.Errorf("address = %q, want empty for a device never seen", got)
	}
}

// TestApplySyncDevicesKeepsSelection 检查设备列表刷新后选中状态仍然指向同一台设备，
// 那台设备消失时回到「本机」。
func TestApplySyncDevicesKeepsSelection(t *testing.T) {
	app, _ := syncApp()
	app.selectedDeviceID = "peer-a"
	app.deviceInfoExpanded = true

	app.applySyncDevices([]lansync.Device{remoteDevice("peer-a", "工作室 PC")})
	if app.selectedDeviceID != "peer-a" {
		t.Errorf("selection = %q, want peer-a", app.selectedDeviceID)
	}

	app.applySyncDevices(nil)
	if app.selectedDeviceID != "" || app.deviceInfoExpanded {
		t.Error("selection must fall back to this device when the device is gone")
	}
}

// TestApplySyncDevicesClosesThePairingWait 检查配对成功后等待中的配对码收起：设备
// 列表里出现了正在配对的那台，对话框该回到设备列表。
func TestApplySyncDevicesClosesThePairingWait(t *testing.T) {
	app, _ := syncApp()
	app.pairing = pairingWaiting
	app.pairingCode = "123456"
	app.pairingPeerID = "peer-a"
	app.pairingPeerName = "工作室 PC"

	app.applySyncDevices([]lansync.Device{remoteDevice("peer-a", "工作室 PC")})
	if app.pairing != pairingIdle {
		t.Errorf("pairing state = %v, want idle", app.pairing)
	}
	if app.pairingCode != "" {
		t.Error("the code must be cleared once pairing succeeded")
	}
}

// TestReceiveClipStoresAndCopies 检查收到的内容既进历史（带来源设备）也写进本机
// 剪贴板，让用户可以直接粘贴。
func TestReceiveClipStoresAndCopies(t *testing.T) {
	app, clipboard := syncApp()
	from := remoteDevice("peer-a", "工作室 PC")
	clip := lansync.Clip{Kind: history.KindText, Text: "从另一台机器来的"}

	app.receiveClip(clip, from)

	items := app.history.Snapshot(history.Query{}).Items
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	item := items[0]
	if item.Text != "从另一台机器来的" {
		t.Errorf("text = %q", item.Text)
	}
	if item.SourceDeviceID != from.ID {
		t.Errorf("source = %q, want %q", item.SourceDeviceID, from.ID)
	}
	if item.IsLocal() {
		t.Error("a received item is not local")
	}
	if len(clipboard.texts) != 1 || clipboard.texts[0] != "从另一台机器来的" {
		t.Errorf("clipboard = %q, want the received text", clipboard.texts)
	}
	// 这次写入不该被捕获路径当成一次新的复制。
	if !app.consumeClipboardEcho(item.ID) {
		t.Error("receiving must record the clipboard echo")
	}
}

// TestReceiveClipKeepsOneRecordPerContent 检查同样的内容收到两次只留一条记录，但
// 每次都会写进剪贴板：用户确实又复制了一次。
func TestReceiveClipKeepsOneRecordPerContent(t *testing.T) {
	app, clipboard := syncApp()
	from := remoteDevice("peer-a", "工作室 PC")
	clip := lansync.Clip{Kind: history.KindText, Text: "同一段内容"}

	app.receiveClip(clip, from)
	app.receiveClip(clip, from)

	if items := app.history.Snapshot(history.Query{}).Items; len(items) != 1 {
		t.Errorf("items = %d, want 1", len(items))
	}
	if len(clipboard.texts) != 2 {
		t.Errorf("clipboard writes = %d, want 2", len(clipboard.texts))
	}
}

// TestReceiveClipBeforeTheHistoryLoads 检查装载完成之前收到的内容不会被丢掉：那一刻
// 内存里的历史还是空的，装载结果稍后会整份替换它。剪贴板照样先写，条目在装载完成后
// 补进历史，否则用户只会看到剪贴板变了、列表里什么都没有。
func TestReceiveClipBeforeTheHistoryLoads(t *testing.T) {
	app := newApplication()
	app.pairedDevices = nil
	clipboard := &recordingClipboard{}
	app.clipboard = clipboard
	if app.historyReady {
		t.Fatal("a fresh application must not claim the history is loaded")
	}

	from := remoteDevice("peer-a", "工作室 PC")
	app.receiveClip(lansync.Clip{Kind: history.KindText, Text: "早到的内容"}, from)

	if len(clipboard.texts) != 1 || clipboard.texts[0] != "早到的内容" {
		t.Fatalf("clipboard = %q, want the received text", clipboard.texts)
	}
	if items := app.history.Snapshot(history.Query{}).Items; len(items) != 0 {
		t.Fatalf("items before the load = %d, want the clip queued", len(items))
	}

	app.installHistory(nil, nil)

	items := app.history.Snapshot(history.Query{}).Items
	if len(items) != 1 || items[0].Text != "早到的内容" {
		t.Fatalf("items = %+v, want the clip received before the load", items)
	}
	if items[0].SourceDeviceID != from.ID {
		t.Errorf("source = %q, want %q", items[0].SourceDeviceID, from.ID)
	}
	if len(app.pendingClips) != 0 {
		t.Error("the queue must be empty once it has been flushed")
	}
}

// TestApplyDeliveryUpdatesTheItem 检查投递结果写回记录：预览区的同步状态与每台
// 设备的结果都来自这里。
func TestApplyDeliveryUpdatesTheItem(t *testing.T) {
	app, _ := syncApp()
	item := history.Item{ID: "item-1", Kind: kindText, Text: "hi", CreatedAt: time.Now()}
	if err := app.history.Append(item); err != nil {
		t.Fatalf("Append: %v", err)
	}

	deliveries := []history.DeviceDelivery{{DeviceID: "peer-a", State: deliveryReceived}}
	app.applyDelivery("item-1", deliveries)

	stored, ok := app.history.Item("item-1")
	if !ok {
		t.Fatal("the item must still exist")
	}
	if stored.Sync != syncStateSynced {
		t.Errorf("sync state = %v, want synced", stored.Sync)
	}
	if len(stored.Deliveries) != 1 || stored.Deliveries[0].DeviceID != "peer-a" {
		t.Errorf("deliveries = %+v", stored.Deliveries)
	}

	// 记录已被删除时投递结果不该把应用弄崩，也不该复活记录。
	app.applyDelivery("gone", deliveries)
	if _, ok := app.history.Item("gone"); ok {
		t.Error("a delivery for a deleted item must not recreate it")
	}
}

// TestMarkSuperseded 检查新的复制把一条都没送到的旧记录标成「已被替代」，而已经
// 送到过的、以及收到的记录都不受影响。
func TestMarkSuperseded(t *testing.T) {
	app, _ := syncApp()
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	undelivered := history.Item{ID: "old", Kind: kindText, Text: "old", CreatedAt: base, Sync: syncStateOffline,
		Deliveries: []history.DeviceDelivery{{DeviceID: "peer-a", State: deliveryOffline}}}
	delivered := history.Item{ID: "delivered", Kind: kindText, Text: "delivered", CreatedAt: base.Add(time.Second), Sync: syncStateSynced,
		Deliveries: []history.DeviceDelivery{{DeviceID: "peer-a", State: deliveryReceived}}}
	received := history.Item{ID: "received", Kind: kindText, Text: "received", SourceDeviceID: "peer-a",
		CreatedAt: base.Add(2 * time.Second), Sync: syncStateOffline}
	newest := history.Item{ID: "new", Kind: kindText, Text: "new", CreatedAt: base.Add(3 * time.Second)}
	for _, item := range []history.Item{undelivered, delivered, received, newest} {
		if err := app.history.Append(item); err != nil {
			t.Fatalf("Append %s: %v", item.ID, err)
		}
	}

	app.markSuperseded("new")

	if item, _ := app.history.Item("old"); item.Sync != syncStateReplaced {
		t.Errorf("an undelivered item = %v, want replaced", item.Sync)
	}
	if item, _ := app.history.Item("delivered"); item.Sync != syncStateSynced {
		t.Errorf("a delivered item = %v, want untouched", item.Sync)
	}
	if item, _ := app.history.Item("received"); item.Sync != syncStateOffline {
		t.Errorf("a received item = %v, want untouched", item.Sync)
	}
	if item, _ := app.history.Item("new"); item.Sync != syncStateNone {
		t.Errorf("the new item = %v, want untouched", item.Sync)
	}
}

// TestPendingForDevice 检查设备重新上线时要补发的记录：只有本机复制的、还没送到
// 那台设备的、且没被替代的。
func TestPendingForDevice(t *testing.T) {
	app, _ := syncApp()
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	items := []history.Item{
		{ID: "pending", Kind: kindText, Text: "pending", CreatedAt: base, Sync: syncStateOffline,
			Deliveries: []history.DeviceDelivery{{DeviceID: "peer-a", State: deliveryOffline}}},
		{ID: "received-by-a", Kind: kindText, Text: "ok", CreatedAt: base.Add(time.Second), Sync: syncStatePartial,
			Deliveries: []history.DeviceDelivery{{DeviceID: "peer-a", State: deliveryReceived}}},
		{ID: "replaced", Kind: kindText, Text: "replaced", CreatedAt: base.Add(2 * time.Second), Sync: syncStateReplaced,
			Deliveries: []history.DeviceDelivery{{DeviceID: "peer-a", State: deliveryOffline}}},
		{ID: "from-peer", Kind: kindText, Text: "from peer", SourceDeviceID: "peer-b",
			CreatedAt: base.Add(3 * time.Second)},
	}
	for _, item := range items {
		if err := app.history.Append(item); err != nil {
			t.Fatalf("Append %s: %v", item.ID, err)
		}
	}

	pending := app.pendingForDevice("peer-a")
	if len(pending) != 1 || pending[0].ID != "pending" {
		t.Fatalf("pending = %+v, want only the undelivered local item", pending)
	}
}

// TestPairablePeersSkipsPairedDevices 检查配对对话框只列出还没配对的设备。
func TestPairablePeersSkipsPairedDevices(t *testing.T) {
	app, _ := syncApp()
	app.pairedDevices = []pairedDevice{{id: "peer-a", name: "工作室 PC"}}
	app.discoveredPeers = []lansync.Peer{
		{ID: "peer-a", Name: "工作室 PC"},
		{ID: "peer-b", Name: "笔记本"},
	}

	peers := app.pairablePeers()
	if len(peers) != 1 || peers[0].ID != "peer-b" {
		t.Fatalf("pairable = %+v, want only the unpaired device", peers)
	}
}

// TestPairingMessagesNameTheFailure 检查每一种配对失败都有对应的界面文案，不会
// 落到空字符串。
func TestPairingMessagesNameTheFailure(t *testing.T) {
	app, _ := syncApp()
	for _, err := range []error{
		lansync.ErrPairingWrongCode,
		lansync.ErrPairingTimeout,
		lansync.ErrPairingCanceled,
		lansync.ErrPairingBusy,
		lansync.ErrPairingVersion,
		errors.New("something else"),
	} {
		if message := app.pairingMessage(err); message == "" {
			t.Errorf("%v has no message", err)
		}
	}
}

// TestSyncActionsNeedTheService 检查服务没启动时同步入口不做任何事，而不是崩。
func TestSyncActionsNeedTheService(t *testing.T) {
	app, _ := syncApp()
	app.startPairing("peer-a")
	app.cancelPairing()
	app.confirmPairing("123456")
	app.declinePairRequest()
	app.resendPending("peer-a")
	app.setDeviceDirection("peer-a", lansync.ContentText, syncDirectionOff)
	if app.pairingError != "" {
		t.Errorf("pairing error = %q, want nothing without a service", app.pairingError)
	}
}

// TestOpenSyncServiceRetriesWhileTheFileIsBusy 检查身份文件被短暂占用时会重试，而不是
// 让整次运行没有同步：重启、热重载都可能撞上这个锁。
func TestOpenSyncServiceRetriesWhileTheFileIsBusy(t *testing.T) {
	busy := errors.New("The process cannot access the file because it is being used by another process")
	attempts := 0
	service, err := openSyncService(context.Background(), func() (*lansync.Service, error) {
		attempts++
		if attempts < 3 {
			return nil, busy
		}
		return &lansync.Service{}, nil
	})
	if err != nil {
		t.Fatalf("openSyncService: %v", err)
	}
	if service == nil {
		t.Fatal("a service must come back once the file is free")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

// TestOpenSyncServiceGivesUpOnUnreadableData 检查内容本身读不出来时不重试：等一等
// 不会让它变好，只会让用户多等三秒。
func TestOpenSyncServiceGivesUpOnUnreadableData(t *testing.T) {
	attempts := 0
	_, err := openSyncService(context.Background(), func() (*lansync.Service, error) {
		attempts++
		return nil, lansync.ErrStoreUnreadable
	})
	if !errors.Is(err, lansync.ErrStoreUnreadable) {
		t.Fatalf("err = %v, want ErrStoreUnreadable", err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

// TestOpenSyncServiceStopsWhenTheAppQuits 检查退出时重试会停下，不会拖住关闭流程。
func TestOpenSyncServiceStopsWhenTheAppQuits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	_, err := openSyncService(ctx, func() (*lansync.Service, error) {
		attempts++
		return nil, errors.New("busy")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if attempts > 1 {
		t.Errorf("attempts = %d, want the retry to stop immediately", attempts)
	}
}
