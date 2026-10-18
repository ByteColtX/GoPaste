package lansync

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	// serviceType is the mDNS service every GoPaste instance advertises. It is
	// part of the wire contract between two devices, so it never changes.
	serviceType = "_gopaste._tcp"
	// serviceDomain is the mDNS domain.
	serviceDomain = "local"
	// queryInterval is how often each interface is asked for peers. Peers
	// announce themselves only when asked, so browsing is a repeating query;
	// three seconds keeps the device list responsive without being noisy.
	queryInterval = 3 * time.Second
	// queryTimeout bounds one round of browsing on one interface.
	queryTimeout = 2 * time.Second
	// peerExpiry is how long a peer stays online after its last sighting. It
	// is several query intervals, so one lost packet does not flip a device
	// offline.
	peerExpiry = 20 * time.Second
)

// TXT record keys. They are part of the wire contract.
const (
	txtID       = "id"
	txtName     = "name"
	txtPlatform = "platform"
	txtVersion  = "version"
	txtKey      = "pk"
)

// maxTXTValueLength bounds one TXT value: mDNS carries each value as a single
// length-prefixed string of at most 255 bytes.
const maxTXTValueLength = 255

// Peer is one device seen on the local network, whether or not this device has
// paired with it. Presence comes from discovery alone: without a sighting there
// is no address to send to, so a peer that mDNS cannot reach cannot sync.
type Peer struct {
	// ID is the peer ID derived from PublicKey.
	ID string
	// Name is the name the device advertises.
	Name string
	// Platform is the platform name it advertises.
	Platform string
	// Version is the application version it advertises.
	Version string
	// PublicKey is the key it advertises. Pairing pins this key, and the
	// pairing code is what proves the key really belongs to the device the
	// user is looking at.
	PublicKey ed25519.PublicKey
	// Addr and Port are where it accepts connections.
	Addr net.IP
	Port int
	// Seen is when discovery last heard from it.
	Seen time.Time
}

// Online reports whether the peer was heard from recently enough to try
// connecting to it.
func (p Peer) Online(now time.Time) bool {
	return !p.Seen.IsZero() && now.Sub(p.Seen) <= peerExpiry
}

// Discovery advertises this device over mDNS and browses for the others.
//
// One responder and one browse loop run per usable interface rather than one
// for the whole machine: the system's default multicast route is often a VPN
// tunnel that cannot reach the LAN at all, while the LAN interface itself
// works. Binding per interface also makes the advertised address the one that
// interface actually has.
type Discovery struct {
	identity Identity
	version  string
	name     func() string
	platform string
	port     func() int
	onUpdate func([]Peer)

	mu    sync.Mutex
	peers map[string]Peer
	// fingerprint is what the last published peer list looked like, so an
	// unchanged list does not wake the UI every query round.
	fingerprint string
	ctx         context.Context
	cancel      context.CancelFunc
	servers     []*mdns.Server
	broadcaster *broadcaster
	wg          sync.WaitGroup
	started     bool
}

// DiscoveryOptions configures a Discovery.
type DiscoveryOptions struct {
	// Identity is this device's identity; it supplies the advertised peer ID
	// and public key.
	Identity Identity
	// Name returns this device's current name. It is read at every (re)start,
	// so a renamed device re-advertises by calling Refresh.
	Name func() string
	// Platform is this device's platform name, such as "macOS".
	Platform string
	// Version is this application's version, shown in the device list.
	Version string
	// Port returns the port this device accepts connections on. It is read at
	// every (re)start, so the port can be chosen before discovery starts.
	Port func() int
	// OnUpdate is called with the full peer list whenever it changes. It runs
	// on a discovery goroutine: a caller that owns UI state must hand the
	// result to the UI thread itself.
	OnUpdate func([]Peer)
}

// NewDiscovery builds a Discovery. Call Start to begin advertising and
// browsing.
func NewDiscovery(options DiscoveryOptions) *Discovery {
	return &Discovery{
		identity: options.Identity,
		version:  options.Version,
		name:     options.Name,
		platform: options.Platform,
		port:     options.Port,
		onUpdate: options.OnUpdate,
		peers:    map[string]Peer{},
	}
}

// ErrNoInterfaces reports that no interface could carry mDNS: the machine is
// offline, or only has a loopback interface.
var ErrNoInterfaces = errors.New("lansync: no local network interface to advertise on")

// Start advertises this device and begins browsing, until ctx ends. Starting
// twice is a no-op. An error means discovery is not running at all; an
// interface that fails on its own is skipped and reported through OnUpdate.
func (d *Discovery) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return nil
	}
	interfaces, err := usableInterfaces()
	if err != nil {
		d.mu.Unlock()
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	d.ctx = ctx
	d.cancel = cancel
	d.started = true
	d.mu.Unlock()

	d.announce(runCtx, interfaces)
	// Broadcast runs beside mDNS. It is not fatal when it cannot bind: mDNS may
	// still reach the other devices, and vice versa.
	d.startBroadcast(runCtx)
	// Publish once right away, even when nothing was found yet: the caller then
	// knows discovery is running instead of waiting for the first sighting.
	d.publish(true)
	return nil
}

// startBroadcast begins the UDP broadcast announcement and listen loop.
func (d *Discovery) startBroadcast(ctx context.Context) {
	broadcast := newBroadcaster(d.identity, d.name, d.platform, d.version, d.port, d.sighted)
	if err := broadcast.start(ctx); err != nil {
		return
	}
	d.mu.Lock()
	d.broadcaster = broadcast
	d.mu.Unlock()
}

// Stop stops advertising and browsing, and forgets every peer.
func (d *Discovery) Stop() {
	d.mu.Lock()
	cancel := d.cancel
	servers := d.servers
	broadcast := d.broadcaster
	d.cancel = nil
	d.servers = nil
	d.broadcaster = nil
	d.started = false
	d.peers = map[string]Peer{}
	d.fingerprint = ""
	d.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if broadcast != nil {
		broadcast.stop()
	}
	for _, server := range servers {
		server.Shutdown()
	}
	d.wg.Wait()
}

// Refresh re-advertises this device: it restarts discovery so a renamed device
// is announced under its new name and reaches the other devices' lists.
func (d *Discovery) Refresh() {
	d.mu.Lock()
	ctx, started := d.ctx, d.started
	d.mu.Unlock()
	if !started || ctx == nil || ctx.Err() != nil {
		return
	}
	d.Stop()
	_ = d.Start(ctx)
}

// Peers returns the current peer list, newest sighting first.
func (d *Discovery) Peers() []Peer {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.snapshotLocked()
}

// announce starts one responder and one browse loop per interface.
func (d *Discovery) announce(ctx context.Context, interfaces []net.Interface) {
	for _, iface := range interfaces {
		iface := iface
		addresses := interfaceIPv4s(iface)
		if len(addresses) == 0 {
			continue
		}
		port := d.port()
		if port <= 0 {
			continue
		}
		zone, err := mdns.NewMDNSService(
			d.identity.ID,
			serviceType,
			serviceDomain,
			"",
			port,
			addresses,
			d.txtRecords(),
		)
		if err != nil {
			continue
		}
		server, err := mdns.NewServer(&mdns.Config{Zone: zone, Iface: &iface})
		if err != nil {
			// One unusable interface (a tunnel, an unconfigured adapter) must
			// not stop the others from advertising.
			continue
		}
		d.mu.Lock()
		d.servers = append(d.servers, server)
		d.mu.Unlock()

		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.browse(ctx, iface)
		}()
	}
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.expire(ctx)
	}()
}

// browse asks one interface for peers every queryInterval.
func (d *Discovery) browse(ctx context.Context, iface net.Interface) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		entries := make(chan *mdns.ServiceEntry, 16)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case entry := <-entries:
					if peer, ok := peerFromEntry(entry); ok {
						d.sighted(peer)
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
		params := mdns.DefaultParams(serviceType)
		params.Domain = serviceDomain
		params.Entries = entries
		params.Timeout = queryTimeout
		params.Interface = &iface
		// A query error means this round found nothing (the interface went
		// down, or the network blocks multicast); the next round tries again.
		_ = mdns.QueryContext(queryCtx, params)
		cancel()
		<-done

		select {
		case <-ctx.Done():
			return
		case <-time.After(queryInterval):
		}
	}
}

// expire marks peers offline once they have not been heard from for peerExpiry,
// and drops the ones that have been gone long enough to be forgotten.
func (d *Discovery) expire(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			d.mu.Lock()
			for id, peer := range d.peers {
				if now.Sub(peer.Seen) > 4*peerExpiry {
					delete(d.peers, id)
				}
			}
			d.mu.Unlock()
			d.publish(false)
		}
	}
}

// sighted records one sighting and publishes the peer list if it changed.
func (d *Discovery) sighted(peer Peer) {
	if peer.ID == d.identity.ID {
		return
	}
	peer.Seen = time.Now()
	d.mu.Lock()
	d.peers[peer.ID] = peer
	d.mu.Unlock()
	d.publish(false)
}

// publish hands the current list to OnUpdate when it differs from the last one.
// force publishes even an unchanged list, which is how a fresh start reports that
// discovery is running.
func (d *Discovery) publish(force bool) {
	d.mu.Lock()
	peers := d.snapshotLocked()
	fingerprint := peerFingerprint(peers)
	if fingerprint == d.fingerprint && !force {
		d.mu.Unlock()
		return
	}
	d.fingerprint = fingerprint
	onUpdate := d.onUpdate
	d.mu.Unlock()
	if onUpdate != nil {
		onUpdate(peers)
	}
}

// snapshotLocked copies the peer map into a sorted slice. Callers hold the
// lock. Peers that have not been heard from recently stay in the list: the UI
// shows them as offline rather than dropping them, and a device that comes back
// keeps its record.
func (d *Discovery) snapshotLocked() []Peer {
	peers := make([]Peer, 0, len(d.peers))
	for _, peer := range d.peers {
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Name != peers[j].Name {
			return peers[i].Name < peers[j].Name
		}
		return peers[i].ID < peers[j].ID
	})
	return peers
}

// peerFingerprint reduces a peer list to what the UI shows, so a list that only
// differs in timestamps does not count as a change.
func peerFingerprint(peers []Peer) string {
	var builder strings.Builder
	now := time.Now()
	for _, peer := range peers {
		fmt.Fprintf(&builder, "%s|%s|%s|%s|%s|%t;", peer.ID, peer.Name, peer.Platform, peer.Addr, strconv.Itoa(peer.Port), peer.Online(now))
	}
	return builder.String()
}

// txtRecords is the announcement's TXT payload.
func (d *Discovery) txtRecords() []string {
	return []string{
		txtID + "=" + d.identity.ID,
		txtName + "=" + truncateTXT(d.name()),
		txtPlatform + "=" + truncateTXT(d.platform),
		txtVersion + "=" + truncateTXT(d.version),
		txtKey + "=" + base64.StdEncoding.EncodeToString(d.identity.Public),
	}
}

// truncateTXT cuts a value to one TXT string without splitting a rune.
func truncateTXT(value string) string {
	if len(value) <= maxTXTValueLength {
		return value
	}
	cut := maxTXTValueLength
	for cut > 0 && !utf8Start(value[cut]) {
		cut--
	}
	return value[:cut]
}

// utf8Start reports whether b starts a UTF-8 sequence.
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// peerFromEntry turns one mDNS answer into a peer. An answer without a usable
// ID and key is ignored: without them the device can be neither displayed
// meaningfully nor paired with.
func peerFromEntry(entry *mdns.ServiceEntry) (Peer, bool) {
	if entry == nil || entry.AddrV4 == nil || entry.Port <= 0 {
		return Peer{}, false
	}
	fields := map[string]string{}
	for _, info := range entry.InfoFields {
		key, value, ok := strings.Cut(info, "=")
		if !ok {
			continue
		}
		fields[key] = value
	}
	key, err := base64.StdEncoding.DecodeString(fields[txtKey])
	if err != nil || len(key) != ed25519.PublicKeySize {
		return Peer{}, false
	}
	public := ed25519.PublicKey(key)
	id := fields[txtID]
	if id == "" {
		id = PeerID(public)
	}
	if id != PeerID(public) {
		// The advertised ID must be the one the key derives, or the pinned
		// record and the device list would disagree about who this is.
		return Peer{}, false
	}
	return Peer{
		ID:        id,
		Name:      fields[txtName],
		Platform:  fields[txtPlatform],
		Version:   fields[txtVersion],
		PublicKey: public,
		Addr:      entry.AddrV4,
		Port:      entry.Port,
	}, true
}

// usableInterfaces lists the interfaces mDNS can run on: up, multicast-capable,
// not loopback, and holding at least one private IPv4 address.
func usableInterfaces() ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("lansync: list interfaces: %w", err)
	}
	usable := make([]net.Interface, 0, len(all))
	for _, iface := range all {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(interfaceIPv4s(iface)) == 0 {
			continue
		}
		usable = append(usable, iface)
	}
	if len(usable) == 0 {
		return nil, ErrNoInterfaces
	}
	return usable, nil
}

// ipv4Network is one private IPv4 address of an interface together with the
// broadcast address of its subnet.
type ipv4Network struct {
	ip        net.IP
	broadcast net.IP
}

// interfaceNetworks lists the interface's private IPv4 networks: the ones a peer
// on the same local network can reach.
func interfaceNetworks(iface net.Interface) []ipv4Network {
	addresses, err := iface.Addrs()
	if err != nil {
		return nil
	}
	usable := make([]ipv4Network, 0, len(addresses))
	for _, address := range addresses {
		ip, network, err := net.ParseCIDR(address.String())
		if err != nil {
			continue
		}
		ip4 := ip.To4()
		if ip4 == nil || !ip4.IsPrivate() {
			continue
		}
		usable = append(usable, ipv4Network{ip: ip4, broadcast: broadcastAddress(ip4, network.Mask)})
	}
	return usable
}

// interfaceIPv4s returns just the addresses of interfaceNetworks.
func interfaceIPv4s(iface net.Interface) []net.IP {
	networks := interfaceNetworks(iface)
	usable := make([]net.IP, 0, len(networks))
	for _, network := range networks {
		usable = append(usable, network.ip)
	}
	return usable
}

// broadcastAddress returns the subnet's broadcast address, or the address itself
// when the mask is unusable.
func broadcastAddress(ip net.IP, mask net.IPMask) net.IP {
	ip4 := ip.To4()
	if ip4 == nil || len(mask) != net.IPv4len {
		return ip
	}
	broadcast := make(net.IP, net.IPv4len)
	for index := 0; index < net.IPv4len; index++ {
		broadcast[index] = ip4[index] | ^mask[index]
	}
	return broadcast
}
