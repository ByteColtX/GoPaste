package lansync

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
)

// TestPeerIDIsStableAndDerived 检查 peer ID 由公钥算出、长度固定、全是小写：
// 界面上「设备 ID」显示的、配对时钉住的、历史记录里记的都是这一个值。
func TestPeerIDIsStableAndDerived(t *testing.T) {
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	id := PeerID(identity.Public)
	if id != identity.ID {
		t.Errorf("PeerID(public) = %q, identity.ID = %q", id, identity.ID)
	}
	if len(id) != 16 {
		t.Errorf("peer ID length = %d, want 16", len(id))
	}
	if id != strings.ToLower(id) {
		t.Errorf("peer ID %q is not lowercase", id)
	}
	// 同一个公钥每次算出的 ID 都一样，重启后设备仍被认作同一台。
	if again := PeerID(identity.Public); again != id {
		t.Errorf("PeerID is not stable: %q then %q", id, again)
	}
}

// TestIdentityFromPrivateKeyRoundTrip 检查身份可以按密钥材料重建：设备重启后
// peer ID 和签名能力都不变，配对过的设备因此继续认得它。
func TestIdentityFromPrivateKeyRoundTrip(t *testing.T) {
	original, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	restored, err := IdentityFromPrivateKey(original.PrivateKey())
	if err != nil {
		t.Fatalf("IdentityFromPrivateKey: %v", err)
	}
	if restored.ID != original.ID {
		t.Errorf("restored ID = %q, want %q", restored.ID, original.ID)
	}
	if !restored.Public.Equal(original.Public) {
		t.Error("the restored identity must carry the same public key")
	}
	// 配对过的设备钉住的是这把公钥，所以重建后必须还能用它证明身份。
	message := []byte("hello")
	signature := ed25519.Sign(original.PrivateKey(), message)
	if !ed25519.Verify(restored.Public, message, signature) {
		t.Error("the restored key must still prove possession")
	}
}

// TestIdentityRejectsShortKey 检查长度不对的密钥材料被当成错误，而不是造出一个
// 半个身份。
func TestIdentityRejectsShortKey(t *testing.T) {
	_, err := IdentityFromPrivateKey(ed25519.PrivateKey([]byte("too short")))
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("err = %v, want ErrInvalidIdentity", err)
	}
}

// TestIdentitiesDiffer 检查两台设备各自生成的密钥不同，peer ID 因此也不同。
func TestIdentitiesDiffer(t *testing.T) {
	first, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	second, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if first.Public.Equal(second.Public) {
		t.Fatal("two devices must not share a key")
	}
	if first.ID == second.ID {
		t.Fatal("two devices must not share a peer ID")
	}
}
