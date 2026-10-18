package lansync

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// TestPeerFromAnnouncementRoundTrip 检查广播出去的内容能被对方解析回同一台设备，
// 地址取包从哪来——那正是对方接受连接的地方。
func TestPeerFromAnnouncementRoundTrip(t *testing.T) {
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	message := announcement{
		Version:   announcementVersion,
		ID:        identity.ID,
		Name:      "工作室的 MacBook",
		Platform:  "macOS",
		App:       "0.1.0",
		PublicKey: identity.Public,
		Port:      47823,
	}
	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	peer, ok := peerFromAnnouncement(payload, net.ParseIP("192.168.12.10"))
	if !ok {
		t.Fatal("a valid announcement must be accepted")
	}
	if peer.ID != identity.ID || !peer.PublicKey.Equal(identity.Public) {
		t.Error("the announcement must identify the device by its key")
	}
	if peer.Name != "工作室的 MacBook" || peer.Platform != "macOS" || peer.Version != "0.1.0" {
		t.Errorf("peer = %+v", peer)
	}
	if peer.Addr.String() != "192.168.12.10" || peer.Port != 47823 {
		t.Errorf("address = %v:%d, want the packet's source and the announced port", peer.Addr, peer.Port)
	}
}

// TestPeerFromAnnouncementRejectsUnusableMessages 检查版本不符、缺公钥、公钥与 ID
// 对不上、没有端口、没有来源地址的消息一律忽略。
func TestPeerFromAnnouncementRejectsUnusableMessages(t *testing.T) {
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	other, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	valid := announcement{
		Version:   announcementVersion,
		ID:        identity.ID,
		Name:      "本机",
		Platform:  "macOS",
		PublicKey: identity.Public,
		Port:      47823,
	}
	marshal := func(message announcement) []byte {
		payload, err := json.Marshal(message)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return payload
	}

	cases := map[string]struct {
		payload []byte
		from    net.IP
	}{
		"not json":      {payload: []byte("hello"), from: net.ParseIP("192.168.12.10")},
		"other version": {payload: marshal(withVersion(valid, "2")), from: net.ParseIP("192.168.12.10")},
		"no key":        {payload: marshal(withKey(valid, nil)), from: net.ParseIP("192.168.12.10")},
		"short key":     {payload: marshal(withKey(valid, []byte("short"))), from: net.ParseIP("192.168.12.10")},
		"key mismatch":  {payload: marshal(withID(valid, other.ID)), from: net.ParseIP("192.168.12.10")},
		"no port":       {payload: marshal(withPort(valid, 0)), from: net.ParseIP("192.168.12.10")},
		"no source":     {payload: marshal(valid), from: nil},
	}
	for name, tc := range cases {
		if peer, ok := peerFromAnnouncement(tc.payload, tc.from); ok {
			t.Errorf("%s: expected the message to be ignored, got %+v", name, peer)
		}
	}
}

// withVersion, withKey, withID and withPort are small builders for the rejection
// table above.
func withVersion(message announcement, version string) announcement {
	message.Version = version
	return message
}

func withKey(message announcement, key []byte) announcement {
	message.PublicKey = key
	return message
}

func withID(message announcement, id string) announcement {
	message.ID = id
	return message
}

func withPort(message announcement, port int) announcement {
	message.Port = port
	return message
}

// TestBroadcastAddress 检查子网广播地址的算法，包括 /32 这种没有广播地址的极端情况。
func TestBroadcastAddress(t *testing.T) {
	cases := []struct {
		address string
		bits    int
		want    string
	}{
		{"192.168.12.25", 24, "192.168.12.255"},
		{"10.1.2.3", 16, "10.1.255.255"},
		{"172.16.5.9", 12, "172.31.255.255"},
		{"192.168.12.25", 32, "192.168.12.25"},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.address).To4()
		mask := net.CIDRMask(tc.bits, 32)
		if got := broadcastAddress(ip, mask); got.String() != tc.want {
			t.Errorf("broadcastAddress(%s/%d) = %s, want %s", tc.address, tc.bits, got, tc.want)
		}
	}
}

// TestBroadcasterReceivesAnnouncement 检查接收路径：一条发到公告端口的消息会被解
// 析成 peer 交给上层。它用单播发到回环地址，因此不依赖网络是否转发广播。
func TestBroadcasterReceivesAnnouncement(t *testing.T) {
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	sender, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}

	// 用一个空闲端口，免得和本机正在运行的 GoPaste 抢公告端口。
	probe, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	port := probe.LocalAddr().(*net.UDPAddr).Port
	probe.Close()

	peers := make(chan Peer, 4)
	broadcast := newBroadcaster(identity, func() string { return "本机" }, "macOS", "0.1.0", func() int { return 47823 }, func(peer Peer) {
		select {
		case peers <- peer:
		default:
		}
	})
	broadcast.port = port
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := broadcast.start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer broadcast.stop()

	payload, err := json.Marshal(announcement{
		Version:   announcementVersion,
		ID:        sender.ID,
		Name:      "另一台",
		Platform:  "Windows",
		PublicKey: sender.Public,
		Port:      47823,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	// 广播每 3 秒一次，发几遍以避免与第一轮发送撞在一起。
	for attempt := 0; attempt < 3; attempt++ {
		if _, err := conn.Write(payload); err != nil {
			t.Fatalf("write: %v", err)
		}
		select {
		case peer := <-peers:
			if peer.ID != sender.ID {
				t.Fatalf("peer = %+v, want the sender", peer)
			}
			if peer.Name != "另一台" || peer.Port != 47823 {
				t.Errorf("peer = %+v", peer)
			}
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
	t.Fatal("the announcement never reached the listener")
}
