package capture

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopaste/internal/history"
)

// testNow is the fixed reading time the tests' Options.Now returns.
var testNow = time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)

// testOptions settles fast and pins the clock so the tests are quick and
// deterministic.
func testOptions() Options {
	return Options{
		Quiet: 20 * time.Millisecond,
		Now:   func() time.Time { return testNow },
	}
}

// receiveSnapshot waits for one emitted snapshot.
func receiveSnapshot(t *testing.T, emits <-chan Snapshot) Snapshot {
	t.Helper()
	select {
	case snap := <-emits:
		return snap
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a snapshot")
		return Snapshot{}
	}
}

// runLoop starts a Loop over the given source and trigger and returns the
// channel its emissions arrive on.
func runLoop(t *testing.T, src Source, trigger Trigger) (<-chan Snapshot, context.CancelFunc, <-chan error) {
	t.Helper()
	return runLoopWith(t, src, trigger, testOptions())
}

// runLoopWith is runLoop with the options spelled out, for the tests that need
// a non-default one.
func runLoopWith(t *testing.T, src Source, trigger Trigger, opts Options) (<-chan Snapshot, context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	emits := make(chan Snapshot, 8)
	done := make(chan error, 1)
	go func() {
		done <- New(src, trigger, opts).Loop(ctx, func(snap Snapshot) {
			emits <- snap
		})
	}()
	return emits, cancel, done
}

func TestLoopEmitsOnceForRepeatedSignals(t *testing.T) {
	src := newFakeSource()
	src.setText("first")
	trigger := newFakeTrigger()
	emits, cancel, _ := runLoop(t, src, trigger)
	defer cancel()

	// The reading at start reports the content already on the clipboard.
	if got := receiveSnapshot(t, emits); got.Text != "first" {
		t.Fatalf("initial reading text = %q, want first", got.Text)
	}

	// One copy signals several times; the quiet timer must coalesce the burst.
	src.setText("second")
	trigger.signal()
	trigger.signal()
	trigger.signal()

	if got := receiveSnapshot(t, emits); got.Text != "second" {
		t.Fatalf("settled reading text = %q, want second", got.Text)
	}

	select {
	case got := <-emits:
		t.Fatalf("burst produced a second emission: %+v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestReadClassifiesText(t *testing.T) {
	src := newFakeSource()
	src.setText("hello world")

	snap := New(src, newFakeTrigger(), testOptions()).read(context.Background())
	if snap.Kind != history.KindText {
		t.Fatalf("kind = %v, want KindText", snap.Kind)
	}
	if snap.Text != "hello world" {
		t.Fatalf("text = %q", snap.Text)
	}

	item, ok := snap.Item()
	if !ok {
		t.Fatal("a plain text reading must be keepable")
	}
	if item.Kind != history.KindText || item.Text != "hello world" {
		t.Fatalf("item = %+v", item)
	}
	if item.ID == "" {
		t.Fatal("item ID must be a content digest, not empty")
	}
	if !item.CreatedAt.Equal(testNow) {
		t.Fatalf("CreatedAt = %v, want the reading time %v", item.CreatedAt, testNow)
	}
	if item.Favorite || item.Sync != history.SyncStateNone || item.SourceDeviceID != "" {
		t.Fatalf("locally captured item must be fresh and local: %+v", item)
	}
}

func TestReadClassifiesRichText(t *testing.T) {
	src := newFakeSource()
	src.setHTML("<b>hi</b>")
	src.setText("hi")

	snap := New(src, newFakeTrigger(), testOptions()).read(context.Background())
	if snap.Kind != history.KindRichText {
		t.Fatalf("kind = %v, want KindRichText", snap.Kind)
	}
	if snap.HTML != "<b>hi</b>" {
		t.Fatalf("html = %q", snap.HTML)
	}
	if snap.Text != "hi" {
		t.Fatalf("plain text must be kept alongside markup: %q", snap.Text)
	}

	item, ok := snap.Item()
	if !ok {
		t.Fatal("a rich text reading must be keepable")
	}
	// 标记决定这条记录是不是富文本，纯文本表示与它并存：列表和预览要显示可读的
	// 文字，写回剪贴板时两者都要写出去。
	if item.Kind != history.KindRichText || item.HTML != "<b>hi</b>" || item.Text != "hi" {
		t.Fatalf("item must carry markup and text apart: %+v", item)
	}
}

func TestReadClassifiesImage(t *testing.T) {
	src := newFakeSource()
	src.setImage(testPNG(t, 4, 3))

	snap := New(src, newFakeTrigger(), testOptions()).read(context.Background())
	if snap.Kind != history.KindImage {
		t.Fatalf("kind = %v, want KindImage", snap.Kind)
	}
	if snap.Image == nil {
		t.Fatal("image payload must be kept")
	}
	if snap.Image.Width != 4 || snap.Image.Height != 3 {
		t.Fatalf("image size = %dx%d, want 4x3", snap.Image.Width, snap.Image.Height)
	}

	item, ok := snap.Item()
	if !ok {
		t.Fatal("an image reading must be keepable")
	}
	if item.Kind != history.KindImage || item.Image == nil || !bytes.Equal(item.Image.PNG, snap.Image.PNG) {
		t.Fatalf("item image = %+v", item.Image)
	}
}

func TestReadClassifiesFilesOverIconImage(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(first, []byte("aaaa"), 0o600); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(second, []byte("bb"), 0o600); err != nil {
		t.Fatalf("write second file: %v", err)
	}

	src := newFakeSource()
	src.setFiles(first, second)
	src.setImage(testPNG(t, 2, 2)) // the icon a file copy also carries

	snap := New(src, newFakeTrigger(), testOptions()).read(context.Background())
	if snap.Kind != history.KindFile {
		t.Fatalf("kind = %v, want KindFile to beat the icon image", snap.Kind)
	}

	item, ok := snap.Item()
	if !ok {
		t.Fatal("a file reading must be keepable")
	}
	if item.Kind != history.KindFile {
		t.Fatalf("item kind = %v, want KindFile", item.Kind)
	}
	if want := first + "\n" + second; item.Text != want {
		t.Fatalf("item text = %q, want %q", item.Text, want)
	}
	if item.FileSize != 6 {
		t.Fatalf("item file size = %d, want 6", item.FileSize)
	}
	if item.Image != nil {
		t.Fatalf("a file item must not keep the icon image: %+v", item.Image)
	}
}

func TestFileSizeIgnoresUnstattablePaths(t *testing.T) {
	dir := t.TempDir()
	exists := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(exists, []byte("12345"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	missing := filepath.Join(dir, "missing.txt")

	snap := Snapshot{
		Kind:  history.KindFile,
		Files: []string{exists, missing},
		At:    testNow,
	}
	item, ok := snap.Item()
	if !ok {
		t.Fatal("a file reading with one statable path must be keepable")
	}
	if item.FileSize != 5 {
		t.Fatalf("file size = %d, want 5 (the statable file only)", item.FileSize)
	}
}

func TestSensitiveReadingIsNeverEmitted(t *testing.T) {
	src := newFakeSource()
	src.setText("secret")
	src.setSensitive(true)

	emits, cancel, _ := runLoop(t, src, newFakeTrigger())
	defer cancel()
	<-src.started

	select {
	case got := <-emits:
		t.Fatalf("sensitive reading was emitted: %+v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestUnreadableMarkerIsTreatedAsSensitive 覆盖标记读不出来的情况：宁可漏掉一次
// 复制，也不能因为标记读不到就把密码存进历史。
func TestUnreadableMarkerIsTreatedAsSensitive(t *testing.T) {
	src := newFakeSource()
	src.setText("secret")
	src.setSensitiveErr(errors.New("clipboard busy"))

	emits, cancel, _ := runLoop(t, src, newFakeTrigger())
	defer cancel()
	<-src.started

	select {
	case got := <-emits:
		t.Fatalf("reading with an unreadable marker was emitted: %+v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestMarkerReadAfterContentWins 覆盖两次读取之间剪贴板被标记的情况：读内容之前
// 还没标记，读完时已经标记，这一次读取仍然不能保留。
func TestMarkerReadAfterContentWins(t *testing.T) {
	src := newFakeSource()
	src.setText("secret")
	src.setSensitiveAfter(2)

	snap := New(src, newFakeTrigger(), testOptions()).read(context.Background())
	if !snap.Sensitive {
		t.Fatalf("marker set while reading must win: %+v", snap)
	}
	if snap.Kind != kindUnknown {
		t.Fatalf("kind = %v, want the reading to be dropped", snap.Kind)
	}
	if _, ok := snap.Item(); ok {
		t.Fatalf("a sensitive reading must not be keepable: %+v", snap)
	}
}

func TestEmptyClipboardIsNeverEmitted(t *testing.T) {
	src := newFakeSource()
	trigger := newFakeTrigger()
	emits, cancel, _ := runLoop(t, src, trigger)
	defer cancel()
	<-src.started

	trigger.signal()

	select {
	case got := <-emits:
		t.Fatalf("empty clipboard was emitted: %+v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestReadFailureDoesNotStopOtherRepresentations(t *testing.T) {
	src := newFakeSource()
	src.setTextErr(errors.New("text unavailable"))
	src.setImageErr(errors.New("image unavailable"))
	src.setFilesErr(errors.New("files unavailable"))
	src.setHTML("<i>rich</i>")

	snap := New(src, newFakeTrigger(), testOptions()).read(context.Background())
	if snap.Kind != history.KindRichText {
		t.Fatalf("kind = %v, want KindRichText after the other reads failed", snap.Kind)
	}
	if snap.HTML != "<i>rich</i>" {
		t.Fatalf("html = %q", snap.HTML)
	}

	// The same failures must not kill Loop: it still emits the rich text.
	emits, cancel, _ := runLoop(t, src, newFakeTrigger())
	defer cancel()
	if got := receiveSnapshot(t, emits); got.Kind != history.KindRichText {
		t.Fatalf("Loop kind = %v, want KindRichText", got.Kind)
	}
}

func TestDigestStableAcrossReencodedPNG(t *testing.T) {
	img := testImage(64, 64)
	fast := encodePNG(t, img, png.BestSpeed)
	small := encodePNG(t, img, png.BestCompression)
	if bytes.Equal(fast, small) {
		t.Fatal("the two PNG encodings are identical; the test would prove nothing")
	}

	first, ok := Snapshot{Kind: history.KindImage, Image: &history.Image{PNG: fast}, At: testNow}.Item()
	if !ok {
		t.Fatal("first image must be keepable")
	}
	second, ok := Snapshot{Kind: history.KindImage, Image: &history.Image{PNG: small}, At: testNow}.Item()
	if !ok {
		t.Fatal("second image must be keepable")
	}
	if first.ID != second.ID {
		t.Fatalf("re-encoded PNG got a new ID: %s != %s", first.ID, second.ID)
	}
}

func TestDigestStableAcrossCRLFAndLF(t *testing.T) {
	crlf, ok := Snapshot{Kind: history.KindText, Text: "one\r\ntwo"}.Item()
	if !ok {
		t.Fatal("CRLF text must be keepable")
	}
	lf, ok := Snapshot{Kind: history.KindText, Text: "one\ntwo"}.Item()
	if !ok {
		t.Fatal("LF text must be keepable")
	}
	if crlf.ID != lf.ID {
		t.Fatalf("CRLF and LF text got different IDs: %s != %s", crlf.ID, lf.ID)
	}

	nul, ok := Snapshot{Kind: history.KindText, Text: "one\ntwo\x00\x00"}.Item()
	if !ok {
		t.Fatal("text with trailing NULs must be keepable")
	}
	if nul.ID != lf.ID {
		t.Fatalf("trailing NULs changed the ID: %s != %s", nul.ID, lf.ID)
	}
}

func TestSkipStartReadIgnoresContentAlreadyOnTheClipboard(t *testing.T) {
	src := newFakeSource()
	src.setText("copied while paused")
	trigger := newFakeTrigger()
	opts := testOptions()
	opts.SkipStartRead = true
	emits, cancel, _ := runLoopWith(t, src, trigger, opts)
	defer cancel()

	// Nothing may be emitted at start: that reading is what a paused capture
	// would otherwise file as history the moment it resumes.
	select {
	case got := <-emits:
		t.Fatalf("skipped start read emitted %q, want nothing", got.Text)
	case <-time.After(4 * opts.Quiet):
	}

	src.setText("copied after resume")
	trigger.signal()
	if got := receiveSnapshot(t, emits); got.Text != "copied after resume" {
		t.Fatalf("settled reading text = %q, want the content copied after resume", got.Text)
	}
}

func TestLoopReturnsWhenContextCancelled(t *testing.T) {
	src := newFakeSource()
	_, cancel, done := runLoop(t, src, newFakeTrigger())
	<-src.started

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Loop returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Loop did not return after the context was cancelled")
	}
}

func TestLoopReturnsWhenTriggerCloses(t *testing.T) {
	src := newFakeSource()
	trigger := newFakeTrigger()
	_, cancel, done := runLoop(t, src, trigger)
	defer cancel()
	<-src.started

	trigger.close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Loop returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Loop did not return after the trigger channel closed")
	}
}

func TestItemReportsFalseForUnknownSnapshot(t *testing.T) {
	cases := []struct {
		name string
		snap Snapshot
	}{
		{"unknown kind", Snapshot{Kind: kindUnknown}},
		{"zero snapshot", Snapshot{}},
		{"empty text", Snapshot{Kind: history.KindText}},
		{"empty rich text", Snapshot{Kind: history.KindRichText}},
		{"empty image", Snapshot{Kind: history.KindImage}},
		{"empty files", Snapshot{Kind: history.KindFile}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if item, ok := tc.snap.Item(); ok {
				t.Fatalf("Item() = %+v, true; want false", item)
			}
		})
	}
}

// testImage builds a deterministic patterned image large enough that two PNG
// compression levels produce different bytes.
func testImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x*7 + y*13) % 256),
				G: uint8((x*13 + y*7) % 256),
				B: uint8((x*y + x + y) % 256),
				A: 0xff,
			})
		}
	}
	return img
}

// testPNG encodes a test image as PNG with the default encoder.
func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	return encodePNG(t, testImage(width, height), png.DefaultCompression)
}

// encodePNG encodes an image at a specific compression level.
func encodePNG(t *testing.T, img image.Image, level png.CompressionLevel) []byte {
	t.Helper()
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: level}
	if err := encoder.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buf.Bytes()
}
