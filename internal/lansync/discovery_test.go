package lansync

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

// announcingDiscovery builds a Discovery that is not started, for the
// announcement helpers that need no network.
func announcingDiscovery(t *testing.T, name string) *Discovery {
	t.Helper()
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	return NewDiscovery(DiscoveryOptions{
		Identity: identity,
		Name:     func() string { return name },
		Platform: "macOS",
		Version:  "0.1.0",
		Port:     func() int { return 47823 },
	})
}

// TestAnnouncementRoundTrip 检查广播出去的 TXT 能被对方解析回同一台设备：
// 这是「发现」的全部约定，配对对话框里的设备信息就来自它。
func TestAnnouncementRoundTrip(t *testing.T) {
	discovery := announcingDiscovery(t, "工作室的 MacBook")
	entry := &mdns.ServiceEntry{
		Name:       discovery.identity.ID + "." + serviceType + "." + serviceDomain + ".",
		AddrV4:     net.ParseIP("192.168.1.42"),
		Port:       47823,
		InfoFields: discovery.txtRecords(),
	}

	peer, ok := peerFromEntry(entry)
	if !ok {
		t.Fatalf("peerFromEntry rejected a valid announcement: %q", entry.InfoFields)
	}
	if peer.ID != discovery.identity.ID {
		t.Errorf("peer ID = %q, want %q", peer.ID, discovery.identity.ID)
	}
	if !peer.PublicKey.Equal(discovery.identity.Public) {
		t.Error("the advertised key must come back unchanged")
	}
	if peer.Name != "工作室的 MacBook" {
		t.Errorf("name = %q, want %q", peer.Name, "工作室的 MacBook")
	}
	if peer.Platform != "macOS" || peer.Version != "0.1.0" {
		t.Errorf("platform/version = %q/%q", peer.Platform, peer.Version)
	}
	if got := peer.Addr.String(); got != "192.168.1.42" {
		t.Errorf("address = %q", got)
	}
	if peer.Port != 47823 {
		t.Errorf("port = %d", peer.Port)
	}
}

// TestPeerFromEntryRejectsUnusableAnswers 检查缺地址、缺公钥、或公钥与 ID 对不上
// 的应答一律忽略：这种设备既显示不出来，也没法配对。
func TestPeerFromEntryRejectsUnusableAnswers(t *testing.T) {
	discovery := announcingDiscovery(t, "本机")
	records := discovery.txtRecords()
	other, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	mismatched := make([]string, 0, len(records))
	for _, record := range records {
		if strings.HasPrefix(record, txtID+"=") {
			record = txtID + "=" + other.ID
		}
		mismatched = append(mismatched, record)
	}
	withoutKey := make([]string, 0, len(records))
	for _, record := range records {
		if !strings.HasPrefix(record, txtKey+"=") {
			withoutKey = append(withoutKey, record)
		}
	}

	cases := map[string]*mdns.ServiceEntry{
		"no address":    {AddrV4: nil, Port: 47823, InfoFields: records},
		"no port":       {AddrV4: net.ParseIP("192.168.1.42"), InfoFields: records},
		"no key":        {AddrV4: net.ParseIP("192.168.1.42"), Port: 47823, InfoFields: withoutKey},
		"id/key differ": {AddrV4: net.ParseIP("192.168.1.42"), Port: 47823, InfoFields: mismatched},
		"garbage key": {AddrV4: net.ParseIP("192.168.1.42"), Port: 47823,
			InfoFields: []string{txtID + "=" + discovery.identity.ID, txtKey + "=not base64"}},
	}
	for name, entry := range cases {
		if peer, ok := peerFromEntry(entry); ok {
			t.Errorf("%s: expected the answer to be ignored, got %+v", name, peer)
		}
	}
	if _, ok := peerFromEntry(nil); ok {
		t.Error("a nil entry must be ignored")
	}
}

// TestTruncateTXT 检查超长设备名被截到一条 TXT 能放下的长度，而且不切坏一个
// UTF-8 字符。
func TestTruncateTXT(t *testing.T) {
	if got := truncateTXT("短名字"); got != "短名字" {
		t.Errorf("short name changed: %q", got)
	}
	long := strings.Repeat("名", 200)
	got := truncateTXT(long)
	if len(got) > maxTXTValueLength {
		t.Errorf("truncated length = %d, want <= %d", len(got), maxTXTValueLength)
	}
	if !strings.HasPrefix(long, got) {
		t.Error("truncation must keep a prefix of the name")
	}
	for _, r := range got {
		if r == '\uFFFD' {
			t.Fatal("truncation split a rune")
		}
	}
}

// TestPeerFingerprintTracksWhatTheUIshows 检查指纹只在界面看得见的东西变化时才
// 变：每次查询都会刷新 Seen，那不该让界面重画。
func TestPeerFingerprintTracksWhatTheUIShows(t *testing.T) {
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	base := Peer{
		ID:        identity.ID,
		Name:      "MacBook Pro",
		Platform:  "macOS",
		PublicKey: identity.Public,
		Addr:      net.ParseIP("192.168.1.42"),
		Port:      47823,
		Seen:      time.Now(),
	}
	first := peerFingerprint([]Peer{base})

	newer := base
	newer.Seen = time.Now().Add(time.Second)
	if got := peerFingerprint([]Peer{newer}); got != first {
		t.Error("a newer sighting alone must not count as a change")
	}

	renamed := base
	renamed.Name = "工作室 PC"
	if peerFingerprint([]Peer{renamed}) == first {
		t.Error("a rename must count as a change")
	}

	moved := base
	moved.Addr = net.ParseIP("192.168.1.43")
	if peerFingerprint([]Peer{moved}) == first {
		t.Error("a new address must count as a change")
	}

	stale := base
	stale.Seen = time.Now().Add(-time.Hour)
	if peerFingerprint([]Peer{stale}) == first {
		t.Error("going offline must count as a change")
	}
}

// TestDiscoveryReportsEverySighting 检查每一次「看到设备」都上报，而列表只在界面上
// 看得见的东西变了才发布：在线状态按时间衰减，读列表的一方必须能刷新时间戳。
func TestDiscoveryReportsEverySighting(t *testing.T) {
	discovery := announcingDiscovery(t, "本机")
	other, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	sights, updates := 0, 0
	discovery.onSight = func(Peer) { sights++ }
	discovery.onUpdate = func([]Peer) { updates++ }
	peer := Peer{
		ID:        other.ID,
		Name:      "另一台",
		Platform:  "Windows",
		PublicKey: other.Public,
		Addr:      net.ParseIP("192.168.12.10"),
		Port:      47823,
	}

	discovery.sighted(peer)
	discovery.sighted(peer)

	if sights != 2 {
		t.Errorf("sightings reported = %d, want 2", sights)
	}
	if updates != 1 {
		t.Errorf("list updates = %d, want 1: the list did not change", updates)
	}
	// 自己的公告不当作设备。
	discovery.sighted(Peer{ID: discovery.identity.ID, Addr: net.ParseIP("192.168.12.25"), Port: 47823})
	if sights != 2 {
		t.Errorf("sightings reported after our own announcement = %d, want 2", sights)
	}
}

// TestUsableInterfacesFindsTheLAN 检查这台机器上至少能找到一张可用于组播的网卡；
// 一张都没有时发现层要报 ErrNoInterfaces，界面据此显示错误状态而不是空列表。
func TestUsableInterfacesFindsTheLAN(t *testing.T) {
	interfaces, err := usableInterfaces()
	if errors.Is(err, ErrNoInterfaces) {
		t.Skip("这台机器没有可用的局域网网卡")
	}
	if err != nil {
		t.Fatalf("usableInterfaces: %v", err)
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			t.Errorf("interface %s is loopback and must not be used", iface.Name)
		}
		if len(interfaceIPv4s(iface)) == 0 {
			t.Errorf("interface %s has no private IPv4 address", iface.Name)
		}
	}
}

// TestPeerOnlineWindow 检查在线窗口：刚看到过算在线，超过 peerExpiry 就算离线。
func TestPeerOnlineWindow(t *testing.T) {
	now := time.Now()
	peer := Peer{Seen: now}
	if !peer.Online(now) {
		t.Error("a peer seen now must be online")
	}
	if peer.Online(now.Add(peerExpiry + time.Second)) {
		t.Error("a peer unseen for longer than peerExpiry must be offline")
	}
	if (Peer{}).Online(now) {
		t.Error("a peer never seen must be offline")
	}
}

// TestPublicKeyIsAdvertisedVerbatim 检查广播的公钥和身份公钥逐字节一致：配对时
// 钉住的就是它。
func TestPublicKeyIsAdvertisedVerbatim(t *testing.T) {
	discovery := announcingDiscovery(t, "本机")
	var advertised string
	for _, record := range discovery.txtRecords() {
		if key, value, ok := strings.Cut(record, "="); ok && key == txtKey {
			advertised = value
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(advertised)
	if err != nil {
		t.Fatalf("advertised key is not base64: %v", err)
	}
	if !ed25519.PublicKey(decoded).Equal(discovery.identity.Public) {
		t.Error("the advertised key differs from the identity key")
	}
}
