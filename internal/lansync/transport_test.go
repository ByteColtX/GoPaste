package lansync

import (
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
)

// testPeer is one device in a transport test: its identity and the record a
// peer would have pinned for it.
type testPeer struct {
	identity Identity
	device   Device
}

// newTestPeer builds a peer with a fresh identity.
func newTestPeer(t *testing.T, name string) testPeer {
	t.Helper()
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	return testPeer{
		identity: identity,
		device:   Device{ID: identity.ID, Name: name, PublicKey: identity.Public},
	}
}

// startTestServer runs a TLS server on a free port with the production server
// configuration, and returns its address.
func startTestServer(t *testing.T, identity Identity, handler http.Handler) string {
	t.Helper()
	config, err := serverTLSConfig(identity)
	if err != nil {
		t.Fatalf("serverTLSConfig: %v", err)
	}
	listener, _, err := listen(0)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: handler, TLSConfig: config}
	go func() {
		_ = server.Serve(tls.NewListener(listener, config))
	}()
	t.Cleanup(func() {
		server.Close()
	})
	return listener.Addr().String()
}

// testClient builds the production client, resolving the server address to the
// device it pins.
func testClient(identity Identity, device Device, addr string) *http.Client {
	return newHTTPClient(identity, func(candidate string) (Device, bool) {
		if candidate != addr {
			return Device{}, false
		}
		return device, true
	})
}

// get performs one request against addr and returns the response.
func get(t *testing.T, client *http.Client, addr, path string) (*http.Response, error) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, "https://"+addr+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	return client.Do(request)
}

// TestPinnedPeersConnect 检查配对过的两台设备能建立双向认证的连接，服务端还能
// 从连接上认出发请求的是哪一台。
func TestPinnedPeersConnect(t *testing.T) {
	server := newTestPeer(t, "工作室 PC")
	client := newTestPeer(t, "MacBook Pro")

	var seen Device
	handler := requirePaired(serverTrust(server, client), func(w http.ResponseWriter, r *http.Request) {
		peer, ok := pairedPeer(r)
		if !ok {
			http.Error(w, "no peer", http.StatusInternalServerError)
			return
		}
		seen = peer
		w.WriteHeader(http.StatusOK)
	})
	addr := startTestServer(t, server.identity, handler)

	response, err := get(t, testClient(client.identity, server.device, addr), addr, "/")
	if err != nil {
		t.Fatalf("a paired client must connect: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, body = %s", response.StatusCode, body)
	}
	if seen.ID != client.identity.ID {
		t.Errorf("server saw peer %q, want %q", seen.ID, client.identity.ID)
	}
}

// serverTrust is the trust set of one test server: the devices it has paired
// with.
func serverTrust(_ testPeer, paired ...testPeer) Trust {
	return func(public ed25519.PublicKey) (Device, bool) {
		for _, peer := range paired {
			if peer.device.PublicKey.Equal(public) {
				return peer.device, true
			}
		}
		return Device{}, false
	}
}

// TestUnpairedClientIsRefused 检查没配对过的设备拿不到需要配对的端点：它的公钥
// 不在信任集合里，请求被拒。TLS 握手本身仍然允许它进来——配对就发生在同一条
// 通道上。
func TestUnpairedClientIsRefused(t *testing.T) {
	server := newTestPeer(t, "工作室 PC")
	stranger := newTestPeer(t, "陌生设备")

	handler := requirePaired(serverTrust(server), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	addr := startTestServer(t, server.identity, handler)

	response, err := get(t, testClient(stranger.identity, server.device, addr), addr, "/")
	if err != nil {
		t.Fatalf("the handshake itself must succeed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
}

// TestClientRefusesWrongServerKey 检查客户端只认钉住的那把公钥：对方换了密钥
// （或被冒充）就连不上，即使地址是对的。
func TestClientRefusesWrongServerKey(t *testing.T) {
	server := newTestPeer(t, "工作室 PC")
	impostor := newTestPeer(t, "冒充者")
	client := newTestPeer(t, "MacBook Pro")

	handler := requirePaired(serverTrust(server, client), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// 服务端用冒充者的身份跑，但客户端钉的是真设备。
	addr := startTestServer(t, impostor.identity, handler)

	response, err := get(t, testClient(client.identity, server.device, addr), addr, "/")
	if err == nil {
		response.Body.Close()
		t.Fatal("a server whose key is not the pinned one must be refused")
	}
}

// TestOldTLSIsRefused 检查 TLS 1.3 以下的版本连不上：协议只允许 1.3。
func TestOldTLSIsRefused(t *testing.T) {
	server := newTestPeer(t, "工作室 PC")
	client := newTestPeer(t, "MacBook Pro")
	addr := startTestServer(t, server.identity, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	config, err := clientTLSConfig(client.identity, server.device)
	if err != nil {
		t.Fatalf("clientTLSConfig: %v", err)
	}
	config.MaxVersion = tls.VersionTLS12
	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := (&tls.Dialer{NetDialer: dialer, Config: config}).Dial("tcp", addr)
	if err == nil {
		conn.Close()
		t.Fatal("a TLS 1.2 connection must be refused")
	}
}

// TestUnknownAddressIsRefused 检查客户端不会把请求发到没有钉住任何设备的地址：
// 地址解析不出来就不拨号。
func TestUnknownAddressIsRefused(t *testing.T) {
	client := newTestPeer(t, "MacBook Pro")
	httpClient := newHTTPClient(client.identity, func(string) (Device, bool) { return Device{}, false })
	if _, err := get(t, httpClient, "127.0.0.1:1", "/"); err == nil {
		t.Fatal("a request to an unknown address must fail")
	}
}

// TestListenFallsBackFromTakenPort 检查首选端口被占用时退到临时端口，并把真实
// 端口报给调用方（广播里带的就是它）。
func TestListenFallsBackFromTakenPort(t *testing.T) {
	first, firstPort, err := listen(0)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer first.Close()

	second, secondPort, err := listen(firstPort)
	if err != nil {
		t.Fatalf("listen on a taken port: %v", err)
	}
	defer second.Close()
	if secondPort == firstPort {
		t.Errorf("port = %d, want a fallback port different from the taken one", secondPort)
	}
	if secondPort == 0 {
		t.Error("the fallback must report the port it got")
	}
}

// TestCertificateCarriesTheIdentityKey 检查证书里带的就是身份公钥：钉住的公钥
// 和握手里证明的那把必须是同一把。
func TestCertificateCarriesTheIdentityKey(t *testing.T) {
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	certificate, err := identity.certificate()
	if err != nil {
		t.Fatalf("certificate: %v", err)
	}
	public, err := certificateKey(certificate.Certificate[0])
	if err != nil {
		t.Fatalf("certificateKey: %v", err)
	}
	if !public.Equal(identity.Public) {
		t.Error("the certificate must carry the identity key")
	}
	if _, err := certificateKey([]byte("not a certificate")); err == nil {
		t.Error("garbage must not parse as a certificate")
	}
	if _, err := certificateKey(certificate.Certificate[0][:10]); err == nil {
		t.Error("a truncated certificate must not parse")
	}
}

// TestPeerFromRequestWithoutTLS 检查没有客户端证书的请求被当成未配对设备，而不是
// 空设备。
func TestPeerFromRequestWithoutTLS(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://example.invalid/", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if _, err := peerFromRequest(request, func(ed25519.PublicKey) (Device, bool) { return Device{}, true }); !errors.Is(err, ErrUntrustedPeer) {
		t.Fatalf("err = %v, want ErrUntrustedPeer", err)
	}
}
