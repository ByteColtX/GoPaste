package lansync

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"
)

const (
	// broadcastPort is the UDP port the LAN announcement uses. It is not 5353:
	// this is our own message on its own port, so it never collides with mDNS
	// or with the system's responder.
	broadcastPort = 47824
	// broadcastInterval is how often the announcement repeats.
	broadcastInterval = 3 * time.Second
	// announcementVersion is the broadcast message format. A device that
	// speaks another version is ignored rather than guessed at.
	announcementVersion = "1"
)

// announcement is the broadcast form of one device: everything a peer needs to
// list it and to pair with it later.
type announcement struct {
	Version   string `json:"v"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Platform  string `json:"platform"`
	App       string `json:"app"`
	PublicKey []byte `json:"pk"`
	// Port is the TCP port this device accepts connections on.
	Port int `json:"port"`
}

// peerFromAnnouncement turns one received announcement into a peer. from is the
// address the packet came from, which is where the device listens too.
//
// Like the mDNS answer, the ID must be the one the key derives: the pinned
// record and the device list have to agree about who this is.
func peerFromAnnouncement(payload []byte, from net.IP) (Peer, bool) {
	var message announcement
	if err := json.Unmarshal(payload, &message); err != nil {
		return Peer{}, false
	}
	if message.Version != announcementVersion {
		return Peer{}, false
	}
	if len(message.PublicKey) != ed25519.PublicKeySize || message.Port <= 0 || from == nil {
		return Peer{}, false
	}
	public := ed25519.PublicKey(message.PublicKey)
	id := message.ID
	if id == "" {
		id = PeerID(public)
	}
	if id != PeerID(public) {
		return Peer{}, false
	}
	return Peer{
		ID:        id,
		Name:      displayName(message.Name),
		Platform:  displayName(message.Platform),
		Version:   displayName(message.App),
		PublicKey: public,
		Addr:      from,
		Port:      message.Port,
	}, true
}

// broadcaster announces this device over UDP broadcast and listens for the same
// announcements from the other devices.
//
// It exists because mDNS is not always usable between two devices on one
// network: an access point that filters 224.0.0.0/24 between its wired and
// wireless sides silently drops every mDNS packet one way, while plain subnet
// broadcast still crosses. Both run; whichever hears a device first wins, and
// the peer list is the same either way.
type broadcaster struct {
	identity Identity
	name     func() string
	platform string
	version  string
	port     func() int
	onPeer   func(Peer)

	conn   *net.UDPConn
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// newBroadcaster builds a broadcaster. Call start to begin.
func newBroadcaster(identity Identity, name func() string, platform, version string, port func() int, onPeer func(Peer)) *broadcaster {
	return &broadcaster{
		identity: identity,
		name:     name,
		platform: platform,
		version:  version,
		port:     port,
		onPeer:   onPeer,
	}
}

// start binds the announcement port and begins sending and listening, until ctx
// ends or stop is called.
func (b *broadcaster) start(ctx context.Context) error {
	listenConfig := net.ListenConfig{
		Control: func(network, address string, connection syscall.RawConn) error {
			var err error
			if controlErr := connection.Control(func(fd uintptr) {
				// Several GoPaste instances on one machine (a development
				// build beside an installed one) each want this port.
				err = setSocketOptions(fd)
			}); controlErr != nil {
				return controlErr
			}
			return err
		},
	}
	packet, err := listenConfig.ListenPacket(ctx, "udp4", fmt.Sprintf(":%d", broadcastPort))
	if err != nil {
		return fmt.Errorf("lansync: listen for announcements: %w", err)
	}
	connection, ok := packet.(*net.UDPConn)
	if !ok {
		packet.Close()
		return errors.New("lansync: announcement listener is not a UDP connection")
	}

	runCtx, cancel := context.WithCancel(ctx)
	b.conn = connection
	b.cancel = cancel

	b.wg.Add(2)
	go func() {
		defer b.wg.Done()
		b.announceLoop(runCtx)
	}()
	go func() {
		defer b.wg.Done()
		b.listenLoop(runCtx)
	}()
	return nil
}

// stop ends the loops and closes the socket.
func (b *broadcaster) stop() {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	b.wg.Wait()
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
}

// announceLoop sends this device's announcement to every interface's broadcast
// address, first right away and then at every interval.
func (b *broadcaster) announceLoop(ctx context.Context) {
	ticker := time.NewTicker(broadcastInterval)
	defer ticker.Stop()
	for {
		b.announce()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// announce sends one announcement to each usable interface's broadcast address.
func (b *broadcaster) announce() {
	if b.conn == nil {
		return
	}
	payload, err := json.Marshal(b.message())
	if err != nil {
		return
	}
	interfaces, err := usableInterfaces()
	if err != nil {
		return
	}
	for _, iface := range interfaces {
		for _, network := range interfaceNetworks(iface) {
			// A failed send means this interface cannot broadcast (a tunnel,
			// an adapter without a gateway); the others still can.
			_, _ = b.conn.WriteToUDP(payload, &net.UDPAddr{IP: network.broadcast, Port: broadcastPort})
		}
	}
}

// message builds the announcement payload with the current name.
func (b *broadcaster) message() announcement {
	return announcement{
		Version:   announcementVersion,
		ID:        b.identity.ID,
		Name:      truncateTXT(b.name()),
		Platform:  truncateTXT(b.platform),
		App:       truncateTXT(b.version),
		PublicKey: b.identity.Public,
		Port:      b.port(),
	}
}

// listenLoop receives announcements and reports each one that belongs to a
// device this build can understand.
func (b *broadcaster) listenLoop(ctx context.Context) {
	buffer := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = b.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, from, err := b.conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			// A closed socket ends the loop; anything else is a bad packet
			// that the next read will move past.
			continue
		}
		peer, ok := peerFromAnnouncement(buffer[:n], from.IP)
		if !ok || b.onPeer == nil {
			continue
		}
		b.onPeer(peer)
	}
}
