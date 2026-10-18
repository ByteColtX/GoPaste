package lansync

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopaste/internal/history"
)

// openTestStore opens a store in a temporary directory.
func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := OpenStore(path, "本机", "macOS")
	if err != nil {
		t.Fatalf("OpenStore(%s): %v", path, err)
	}
	return store
}

// pairedDevice builds a device record as pairing would.
func pairedDevice(t *testing.T, name string) Device {
	t.Helper()
	identity, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	return Device{
		ID:          identity.ID,
		Name:        name,
		Platform:    "Windows",
		PublicKey:   identity.Public,
		SyncEnabled: true,
		PairedAt:    time.UnixMilli(1_700_000_000_000).UTC(),
	}
}

// TestOpenStoreCreatesPrivateIdentityFile 检查第一次打开就落下一个只有本用户
// 可读的身份文件：里面是私钥和钉住的公钥，权限必须是 0600。
func TestOpenStoreCreatesPrivateIdentityFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	store := openTestStore(t, path)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat identity file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("identity file mode = %o, want 600", got)
	}
	if store.Identity().ID == "" {
		t.Error("a fresh store must have an identity")
	}
	if got := store.Name(); got != "本机" {
		t.Errorf("name = %q, want %q", got, "本机")
	}
}

// TestStoreKeepsIdentityAndDevicesAcrossReopen 检查重新打开后身份、设备、方向
// 都还在：配对是一次性的，重启不该让人重配。
func TestStoreKeepsIdentityAndDevicesAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	first := openTestStore(t, path)

	device := pairedDevice(t, "工作室 PC")
	device.Directions[ContentImage] = DirectionReceiveOnly
	device.Directions[ContentFile] = DirectionOff
	device.Directions[ContentRichText] = DirectionSendOnly
	if err := first.Put(device); err != nil {
		t.Fatalf("Put: %v", err)
	}
	identity := first.Identity().ID

	second := openTestStore(t, path)
	if got := second.Identity().ID; got != identity {
		t.Errorf("identity changed across reopen: %q then %q", identity, got)
	}
	devices := second.Devices()
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(devices))
	}
	restored := devices[0]
	if restored.ID != device.ID || restored.Name != device.Name || restored.Platform != device.Platform {
		t.Errorf("restored device = %+v, want %+v", restored, device)
	}
	if !restored.PublicKey.Equal(device.PublicKey) {
		t.Error("the pinned key must survive a reopen")
	}
	if !restored.SyncEnabled {
		t.Error("the sync switch must survive a reopen")
	}
	if got, want := restored.Direction(ContentText), DirectionTwoWay; got != want {
		t.Errorf("text direction = %v, want %v", got, want)
	}
	if got, want := restored.Direction(ContentImage), DirectionReceiveOnly; got != want {
		t.Errorf("image direction = %v, want %v", got, want)
	}
	if got, want := restored.Direction(ContentFile), DirectionOff; got != want {
		t.Errorf("file direction = %v, want %v", got, want)
	}
	if got, want := restored.Direction(ContentRichText), DirectionSendOnly; got != want {
		t.Errorf("rich text direction = %v, want %v", got, want)
	}
	// 上次在线位置不该落盘：它是这一次运行的事实，不是配置。
	if restored.Online || restored.Addr != nil || !restored.LastSeen.IsZero() {
		t.Errorf("presence must not be persisted: %+v", restored)
	}
}

// TestStoreTrustsOnlyPinnedKeys 检查信任集合只认已配对设备钉住的那把公钥。
func TestStoreTrustsOnlyPinnedKeys(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "devices.json"))
	device := pairedDevice(t, "MacBook Pro")
	if err := store.Put(device); err != nil {
		t.Fatalf("Put: %v", err)
	}

	trusted, ok := store.Trusted(device.PublicKey)
	if !ok {
		t.Fatal("a pinned key must be trusted")
	}
	if trusted.ID != device.ID {
		t.Errorf("trusted device = %q, want %q", trusted.ID, device.ID)
	}

	other, err := NewIdentity()
	if err != nil {
		t.Fatalf("NewIdentity: %v", err)
	}
	if _, ok := store.Trusted(other.Public); ok {
		t.Error("an unknown key must not be trusted")
	}
}

// TestStoreRemoveReturnsTheRecord 检查取消配对返回被移除的记录，撤销才有东西
// 可以放回去。
func TestStoreRemoveReturnsTheRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	store := openTestStore(t, path)
	device := pairedDevice(t, "工作室 PC")
	if err := store.Put(device); err != nil {
		t.Fatalf("Put: %v", err)
	}

	removed, ok, err := store.Remove(device.ID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !ok || removed.ID != device.ID {
		t.Fatalf("Remove = %+v, %v; want the device", removed, ok)
	}
	if len(store.Devices()) != 0 {
		t.Error("the device must be gone")
	}
	if _, ok, err := store.Remove(device.ID); ok || err != nil {
		t.Errorf("removing an unknown device = %v, %v; want false, nil", ok, err)
	}
	// 撤销：把记录放回去，重新打开后依然在。
	if err := store.Put(removed); err != nil {
		t.Fatalf("Put after remove: %v", err)
	}
	if len(openTestStore(t, path).Devices()) != 1 {
		t.Error("the restored device must persist")
	}
}

// TestStoreRejectsUnreadableFile 检查损坏的身份文件报错且原文件不被覆盖：配对
// 数据不可重建，宁可这一次不工作也不能把它冲掉。
func TestStoreRejectsUnreadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	broken := []byte("{not json")
	if err := os.WriteFile(path, broken, 0o600); err != nil {
		t.Fatalf("write broken file: %v", err)
	}

	_, err := OpenStore(path, "本机", "macOS")
	if !errors.Is(err, ErrStoreUnreadable) {
		t.Fatalf("err = %v, want ErrStoreUnreadable", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(after) != string(broken) {
		t.Error("an unreadable identity file must be left untouched")
	}
}

// TestStoreIgnoresUnknownDirectionNames 检查后一个版本写下的未知内容类型不会
// 让这一次的读取失败：配对数据要能跨版本读。
func TestStoreIgnoresUnknownDirectionNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	store := openTestStore(t, path)
	device := pairedDevice(t, "工作室 PC")
	device.Directions[ContentImage] = DirectionOff
	if err := store.Put(device); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// 手工塞进一个本版本不认识的类型名，再读回来。
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	patched := []byte(string(data))
	patched = replaceOnce(patched, `"text":`, `"video": 3, "text":`)
	if err := os.WriteFile(path, patched, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	reopened := openTestStore(t, path)
	devices := reopened.Devices()
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(devices))
	}
	if got, want := devices[0].Direction(ContentImage), DirectionOff; got != want {
		t.Errorf("image direction = %v, want %v", got, want)
	}
}

// TestStoreSetNamePersists 检查改名落盘：改名后对方看到的名字要跟着变。
func TestStoreSetNamePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.json")
	store := openTestStore(t, path)
	if err := store.SetName("Bytecolt 的 MacBook"); err != nil {
		t.Fatalf("SetName: %v", err)
	}
	// 应用每次都把当前设备名传给 OpenStore；传空时才用文件里记下的那个，
	// 这里靠它确认改名真的落盘了。
	reopened, err := OpenStore(path, "", "")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if got := reopened.Name(); got != "Bytecolt 的 MacBook" {
		t.Errorf("stored name = %q, want %q", got, "Bytecolt 的 MacBook")
	}
}

// TestContentTypeOfMapsKinds 检查同步方向按界面上的四行分组：链接和代码算
// 文本，富文本、图片、文件各归各的。
func TestContentTypeOfMapsKinds(t *testing.T) {
	cases := map[history.Kind]ContentType{
		history.KindText:     ContentText,
		history.KindLink:     ContentText,
		history.KindCode:     ContentText,
		history.KindRichText: ContentRichText,
		history.KindImage:    ContentImage,
		history.KindFile:     ContentFile,
	}
	for kind, want := range cases {
		if got := ContentTypeOf(kind); got != want {
			t.Errorf("ContentTypeOf(%v) = %v, want %v", kind, got, want)
		}
	}
}

// TestDirectionGates 检查四种方向对收和发的放行规则，以及设备总开关会同时关掉
// 两个方向。
func TestDirectionGates(t *testing.T) {
	cases := []struct {
		direction Direction
		send      bool
		receive   bool
	}{
		{DirectionTwoWay, true, true},
		{DirectionSendOnly, true, false},
		{DirectionReceiveOnly, false, true},
		{DirectionOff, false, false},
	}
	for _, tc := range cases {
		if got := tc.direction.AllowsSend(); got != tc.send {
			t.Errorf("%v.AllowsSend() = %v, want %v", tc.direction, got, tc.send)
		}
		if got := tc.direction.AllowsReceive(); got != tc.receive {
			t.Errorf("%v.AllowsReceive() = %v, want %v", tc.direction, got, tc.receive)
		}
	}

	device := Device{SyncEnabled: true}
	device.Directions[ContentText] = DirectionTwoWay
	if !device.Sends(ContentText) || !device.Receives(ContentText) {
		t.Fatal("a two-way device with sync on must both send and receive")
	}
	device.SyncEnabled = false
	if device.Sends(ContentText) || device.Receives(ContentText) {
		t.Error("the device switch must gate both directions")
	}
}

// replaceOnce replaces the first occurrence of old in b with new.
func replaceOnce(b []byte, old, new string) []byte {
	text := string(b)
	for index := 0; index+len(old) <= len(text); index++ {
		if text[index:index+len(old)] == old {
			return []byte(text[:index] + new + text[index+len(old):])
		}
	}
	return b
}
