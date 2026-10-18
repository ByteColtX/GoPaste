package lansync

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gopaste/internal/history"
)

// fileItem builds a local file item over paths, the way the capture path does.
func fileItem(paths ...string) history.Item {
	item := history.Item{
		ID:        history.ContentID(history.Content{Kind: history.KindFile, Files: paths}),
		Kind:      history.KindFile,
		Text:      strings.Join(paths, "\n"),
		CreatedAt: time.Now(),
	}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			item.FileSize += info.Size()
		}
	}
	return item
}

// writeFile creates one file with content and returns its path.
func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestFileTransferCarriesADirectoryTree 检查一条含目录、空目录、符号链接与同名
// 文件的文件项能完整过去：接收端还原同样的结构，空目录还在，链接还是链接。
func TestFileTransferCarriesADirectoryTree(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	root := t.TempDir()
	docs := filepath.Join(root, "Docs")
	writeFile(t, filepath.Join(docs, "a.txt"), "A")
	writeFile(t, filepath.Join(docs, "sub", "b.txt"), "B")
	if err := os.MkdirAll(filepath.Join(docs, "empty"), 0o700); err != nil {
		t.Fatalf("create the empty directory: %v", err)
	}
	if err := os.Symlink("a.txt", filepath.Join(docs, "link")); err != nil {
		t.Fatalf("create the symlink: %v", err)
	}
	one := writeFile(t, filepath.Join(root, "one", "a.txt"), "from one")
	two := writeFile(t, filepath.Join(root, "two", "a.txt"), "from two")

	item := fileItem(docs, one, two)
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryReceived; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	waitFor(t, func() bool { return len(receiver.received()) == 1 })
	clip := receiver.received()[0]
	if len(clip.LocalFiles) != 3 {
		t.Fatalf("local paths = %v, want the three copied paths", clip.LocalFiles)
	}
	itemDir := filepath.Join(receiver.service.options.FilesDir, clip.ID())
	want := []string{
		filepath.Join(itemDir, "Docs"),
		filepath.Join(itemDir, "a.txt"),
		filepath.Join(itemDir, "a (2).txt"),
	}
	for index, path := range clip.LocalFiles {
		if path != want[index] {
			t.Errorf("local path %d = %q, want %q", index, path, want[index])
		}
	}
	contents := map[string]string{
		filepath.Join(itemDir, "Docs", "a.txt"):        "A",
		filepath.Join(itemDir, "Docs", "sub", "b.txt"): "B",
		filepath.Join(itemDir, "a.txt"):                "from one",
		filepath.Join(itemDir, "a (2).txt"):            "from two",
	}
	for path, expected := range contents {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if string(data) != expected {
			t.Errorf("%s = %q, want %q", path, data, expected)
		}
	}
	if info, err := os.Stat(filepath.Join(itemDir, "Docs", "empty")); err != nil || !info.IsDir() {
		t.Errorf("the empty directory did not survive: %v", err)
	}
	target, err := os.Readlink(filepath.Join(itemDir, "Docs", "link"))
	if err != nil || target != "a.txt" {
		t.Errorf("symlink = %q, %v; want a.txt", target, err)
	}
	if got, want := clip.HistoryItem(sender.service.Identity().ID, time.Now()).FileSize, int64(len("A")+len("B")+len("from one")+len("from two")); got != want {
		t.Errorf("item size = %d, want %d", got, want)
	}
}

// TestFileTransferTooLargeStaysOnTheSender 检查超过上限的文件项根本不发：发送端
// 自己就记「文件过大」，接收端什么都收不到。
func TestFileTransferTooLargeStaysOnTheSender(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)
	sender.service.SetFileLimitBytes(4)

	source := writeFile(t, filepath.Join(t.TempDir(), "big.txt"), "way more than four bytes")
	item := fileItem(source)
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryTooLarge; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	// 发送端在 offer 之前就判了体积，所以连一次请求都不该有。
	if clips := receiver.received(); len(clips) != 0 {
		t.Errorf("the receiver got %d payloads, want none", len(clips))
	}
}

// TestFileTransferRefusedByTheReceiverLimit 检查接收端用自己的上限也判一次：
// 发送端上限宽松、接收端上限很小时，发送端看到的是「文件过大」。
func TestFileTransferRefusedByTheReceiverLimit(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)
	receiver.service.SetFileLimitBytes(4)

	source := writeFile(t, filepath.Join(t.TempDir(), "big.txt"), "way more than four bytes")
	item := fileItem(source)
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryTooLarge; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
}

// TestFileTransferRespectsTheReceiverDirection 检查接收端设成「仅发送」时不收文件，
// 发送端看到的是未送达。
func TestFileTransferRespectsTheReceiverDirection(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)
	if err := receiver.service.SetDirection(sender.service.Identity().ID, ContentFile, DirectionSendOnly); err != nil {
		t.Fatalf("SetDirection: %v", err)
	}

	source := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "kept here")
	item := fileItem(source)
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryPending; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
	if clips := receiver.received(); len(clips) != 0 {
		t.Errorf("the receiver got %d payloads, want none", len(clips))
	}
}

// TestFileTransferNeedsAReceiverFilesDirectory 检查没有落盘目录的设备如实拒收文件，
// 而不是收下一半。
func TestFileTransferNeedsAReceiverFilesDirectory(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)
	receiver.service.options.FilesDir = ""

	source := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "kept here")
	item := fileItem(source)
	sender.service.Send(item)

	if got, want := sender.deliveryFor(t, item.ID, receiver.service.Identity().ID), history.DeliveryPending; got != want {
		t.Fatalf("delivery state = %v, want %v", got, want)
	}
}

// TestClipEndpointRefusesAFilePayload 检查文件项不能走文本图片那条端点：那条路
// 只有 JSON，没有放文件字节的地方。
func TestClipEndpointRefusesAFilePayload(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	payload, err := json.Marshal(clipPayload{
		Version: clipProtocolVersion,
		Kind:    int(history.KindFile),
		Text:    "/Users/bytecolt/notes.md",
	})
	if err != nil {
		t.Fatalf("encode the payload: %v", err)
	}
	url := fmt.Sprintf("https://127.0.0.1:%d%s", receiver.port, pathClip)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build the request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := sender.service.client.Do(request)
	if err != nil {
		t.Fatalf("post the payload: %v", err)
	}
	defer response.Body.Close()
	var reply clipReply
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		t.Fatalf("decode the reply: %v", err)
	}
	if reply.Status != clipStatusIgnored || reply.Reason != clipReasonKind {
		t.Errorf("reply = %+v, want ignored because of the kind", reply)
	}
	if clips := receiver.received(); len(clips) != 0 {
		t.Errorf("the receiver stored %d payloads, want none", len(clips))
	}
}

// TestExtractArchiveRejectsEscapingEntries 检查压缩包里的条目不能写到条目目录
// 之外，不管它用相对路径还是绝对路径。
func TestExtractArchiveRejectsEscapingEntries(t *testing.T) {
	for _, name := range []string{"../escape.txt", "/etc/escape.txt", "a/../../escape.txt"} {
		dir := t.TempDir()
		archive := tarGz(t, tarEntry{name: name, body: "nope"})
		if _, err := extractArchive(dir, bytes.NewReader(archive), 1<<20); !errors.Is(err, errArchiveUnsafePath) {
			t.Errorf("entry %q: err = %v, want errArchiveUnsafePath", name, err)
		}
		if entries, err := os.ReadDir(filepath.Dir(dir)); err == nil {
			for _, entry := range entries {
				if entry.Name() == "escape.txt" {
					t.Fatalf("entry %q wrote outside the item directory", name)
				}
			}
		}
	}
}

// TestExtractArchiveRejectsAnUnsupportedEntry 检查表示不了的条目（设备文件、
// 硬链接）让整次传输失败，而不是被悄悄跳过。
func TestExtractArchiveRejectsAnUnsupportedEntry(t *testing.T) {
	dir := t.TempDir()
	archive := tarGz(t, tarEntry{name: "pipe", typeflag: tar.TypeFifo})
	if _, err := extractArchive(dir, bytes.NewReader(archive), 1<<20); !errors.Is(err, errArchiveUnsupported) {
		t.Errorf("err = %v, want errArchiveUnsupported", err)
	}
}

// TestExtractArchiveEnforcesTheDeclaredSize 检查解出来的字节数超过 offer 声明的
// 总数时整次传输失败：声明的小、实际的大不能落盘。
func TestExtractArchiveEnforcesTheDeclaredSize(t *testing.T) {
	dir := t.TempDir()
	archive := tarGz(t, tarEntry{name: "big.txt", body: "0123456789"})
	if _, err := extractArchive(dir, bytes.NewReader(archive), 4); !errors.Is(err, errFileTooLarge) {
		t.Errorf("err = %v, want errFileTooLarge", err)
	}
}

// TestSweepFileStagingDropsStaleOffers 检查没人上传的 offer 与崩溃留下的目录会被
// 清扫掉，而刚建立的还在。
func TestSweepFileStagingDropsStaleOffers(t *testing.T) {
	service := newTestService(t, "MacBook Pro").service
	filesDir := service.options.FilesDir
	stale := filepath.Join(filesDir, stagingDirName, "STALE")
	fresh := filepath.Join(filesDir, stagingDirName, "FRESH")
	for _, dir := range []string{stale, fresh} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	service.filesMu.Lock()
	service.files["STALE"] = &fileTransfer{dir: stale, createdAt: time.Now().Add(-2 * fileStagingLifetime)}
	service.files["FRESH"] = &fileTransfer{dir: fresh, createdAt: time.Now()}
	service.filesMu.Unlock()
	// 一个不在 map 里的旧目录，相当于上次运行崩溃留下的。
	orphan := filepath.Join(filesDir, stagingDirName, "ORPHAN")
	if err := os.MkdirAll(orphan, 0o700); err != nil {
		t.Fatalf("create %s: %v", orphan, err)
	}
	old := time.Now().Add(-2 * fileStagingLifetime)
	if err := os.Chtimes(orphan, old, old); err != nil {
		t.Fatalf("age the orphan: %v", err)
	}

	service.sweepFileStaging()

	for _, dir := range []string{stale, orphan} {
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s survived the sweep", dir)
		}
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("the fresh offer was swept: %v", err)
	}
}

// TestClipOfAFileItemKeepsItsIdentity 检查「本地条目 → Clip → 条目」仍然等价：
// ID 由路径算出，Text 是路径列表，体积跟着走。
func TestClipOfAFileItemKeepsItsIdentity(t *testing.T) {
	paths := []string{"/Users/bytecolt/report.pdf", "/Users/bytecolt/notes.md"}
	item := history.Item{
		ID:        history.ContentID(history.Content{Kind: history.KindFile, Files: paths}),
		Kind:      history.KindFile,
		Text:      strings.Join(paths, "\n"),
		FileSize:  2048,
		CreatedAt: time.Now(),
	}
	clip := ClipOf(item)
	if clip.ID() != item.ID {
		t.Errorf("clip ID = %q, want %q", clip.ID(), item.ID)
	}
	if len(clip.Files) != 2 || clip.Files[1] != paths[1] {
		t.Errorf("identity paths = %v, want %v", clip.Files, paths)
	}
	if len(clip.LocalFiles) != 2 || clip.LocalFiles[0] != paths[0] {
		t.Errorf("local paths = %v, want the same list", clip.LocalFiles)
	}
	round := clip.HistoryItem("", time.Now())
	if round.Text != item.Text || round.FileSize != item.FileSize {
		t.Errorf("round trip = %q/%d, want %q/%d", round.Text, round.FileSize, item.Text, item.FileSize)
	}
}

// TestClipOfAReceivedFileItemUsesItsLocalPaths 检查收到之后再投影成条目时，Text
// 用本机落盘路径，ID 仍由发送端路径算出。
func TestClipOfAReceivedFileItemUsesItsLocalPaths(t *testing.T) {
	identity := []string{"/Users/bytecolt/report.pdf"}
	local := []string{"/Users/other/Library/Application Support/GoPaste/Files/abc/report.pdf"}
	clip := Clip{Kind: history.KindFile, Files: identity, LocalFiles: local, FileSize: 12, CreatedAt: time.Now()}
	item := clip.HistoryItem("peer-a", time.Now())
	if item.Text != local[0] {
		t.Errorf("text = %q, want the local path %q", item.Text, local[0])
	}
	if item.ID != history.ContentID(history.Content{Kind: history.KindFile, Files: identity}) {
		t.Error("the item ID must come from the sending device's paths")
	}
	if item.FileSize != 12 {
		t.Errorf("size = %d, want 12", item.FileSize)
	}
}

// tarEntry is one entry of a hand-built archive.
type tarEntry struct {
	name     string
	body     string
	typeflag byte
}

// tarGz builds a tar.gz archive with these entries, so a test can hand the
// extractor something a hostile peer could have sent.
func tarGz(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		flag := entry.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		header := &tar.Header{
			Typeflag: flag,
			Name:     entry.name,
			Mode:     0o600,
			Size:     int64(len(entry.body)),
		}
		if flag != tar.TypeReg {
			header.Size = 0
		}
		if err := archive.WriteHeader(header); err != nil {
			t.Fatalf("write header %q: %v", entry.name, err)
		}
		if header.Size > 0 {
			if _, err := archive.Write([]byte(entry.body)); err != nil {
				t.Fatalf("write body %q: %v", entry.name, err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close the archive: %v", err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatalf("close the compressor: %v", err)
	}
	return buffer.Bytes()
}

// TestFileTransferKeepsOneDirectoryPerContent 检查同一内容送两次只落一个目录：第二次
// 沿用已有的文件，磁盘上不会出现两份同样的内容。
func TestFileTransferKeepsOneDirectoryPerContent(t *testing.T) {
	sender := newTestService(t, "MacBook Pro")
	receiver := newTestService(t, "工作室 PC")
	pairWith(t, sender, receiver)

	source := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "same content")
	item := fileItem(source)
	sender.service.Send(item)
	waitFor(t, func() bool { return len(receiver.received()) == 1 })
	sender.service.Send(item)
	waitFor(t, func() bool { return len(receiver.received()) == 2 })

	first := receiver.received()[0]
	second := receiver.received()[1]
	if !slices.Equal(first.LocalFiles, second.LocalFiles) {
		t.Errorf("local paths = %v then %v, want the same files", first.LocalFiles, second.LocalFiles)
	}
	entries, err := os.ReadDir(filepath.Join(receiver.service.options.FilesDir, first.ID()))
	if err != nil {
		t.Fatalf("read the item directory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("the item directory holds %d entries, want one copy", len(entries))
	}
}
