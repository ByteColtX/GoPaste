package clipboard

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	xclipboard "golang.design/x/clipboard"

	"gopaste/internal/capture"
)

// fakeReader is a programmable clipboard library. The adapter's job is to
// translate this seam, so the tests drive it directly and never touch a real
// clipboard.
type fakeReader struct {
	mu sync.Mutex
	// data and errs are keyed by the format the adapter asks for, so a test
	// can put a payload on one format and make another read fail.
	data map[xclipboard.Format][]byte
	errs map[xclipboard.Format]error
	// reads records the formats asked for, in order, so a test can prove the
	// fallback was not reached.
	reads        []xclipboard.Format
	files        []string
	filesErr     error
	sensitive    bool
	sensitiveErr error
	events       chan xclipboard.Data
}

func newFakeReader() *fakeReader {
	return &fakeReader{
		data:   map[xclipboard.Format][]byte{},
		errs:   map[xclipboard.Format]error{},
		events: make(chan xclipboard.Data, 8),
	}
}

func (f *fakeReader) Read(_ context.Context, format xclipboard.Format) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, format)
	if err := f.errs[format]; err != nil {
		return nil, err
	}
	buf, ok := f.data[format]
	if !ok {
		return nil, xclipboard.ErrNoData
	}
	return append([]byte(nil), buf...), nil
}

func (f *fakeReader) ReadFiles(context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.filesErr != nil {
		return nil, f.filesErr
	}
	if len(f.files) == 0 {
		return nil, xclipboard.ErrNoData
	}
	return append([]string(nil), f.files...), nil
}

func (f *fakeReader) Sensitive(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sensitive, f.sensitiveErr
}

func (f *fakeReader) Watch(context.Context) <-chan xclipboard.Data {
	return f.events
}

// readFormats returns the formats read so far, so a test can assert on the
// order and on which fallbacks ran.
func (f *fakeReader) readFormats() []xclipboard.Format {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]xclipboard.Format(nil), f.reads...)
}

// newTestClipboard builds the adapter over a fake, bypassing New so no real
// clipboard is initialized.
func newTestClipboard(r *fakeReader) *Clipboard {
	return &Clipboard{reader: r}
}

func TestReadTextUsesPlainTextFormat(t *testing.T) {
	r := newFakeReader()
	r.data[xclipboard.FmtText] = []byte("hello 世界")

	buf, err := newTestClipboard(r).Read(context.Background(), capture.RepText)
	if err != nil {
		t.Fatalf("Read(RepText): %v", err)
	}
	if string(buf) != "hello 世界" {
		t.Fatalf("text = %q, want the clipboard bytes", buf)
	}
	if formats := r.readFormats(); len(formats) != 1 || formats[0] != xclipboard.FmtText {
		t.Fatalf("read formats = %v, want only FmtText", formats)
	}
}

func TestReadMapsAbsentAndUnsupportedToNoData(t *testing.T) {
	cases := map[string]error{
		"nothing there": xclipboard.ErrNoData,
		"unsupported":   xclipboard.ErrUnsupported,
	}
	for name, cause := range cases {
		t.Run(name, func(t *testing.T) {
			r := newFakeReader()
			r.errs[xclipboard.FmtText] = cause

			_, err := newTestClipboard(r).Read(context.Background(), capture.RepText)
			if !errors.Is(err, capture.ErrNoData) {
				t.Fatalf("Read error = %v, want capture.ErrNoData", err)
			}
		})
	}
}

func TestReadPassesThroughRealFailures(t *testing.T) {
	busy := errors.New("clipboard busy")
	r := newFakeReader()
	r.errs[xclipboard.FmtText] = busy

	_, err := newTestClipboard(r).Read(context.Background(), capture.RepText)
	if !errors.Is(err, busy) {
		t.Fatalf("Read error = %v, want the clipboard failure", err)
	}
}

func TestReadHTMLPrefersTextHTML(t *testing.T) {
	r := newFakeReader()
	r.data[formatHTML] = []byte("<b>hi</b>")
	r.data[formatCFHTML] = []byte("should not be read")

	html, err := newTestClipboard(r).Read(context.Background(), capture.RepHTML)
	if err != nil {
		t.Fatalf("Read(RepHTML): %v", err)
	}
	if string(html) != "<b>hi</b>" {
		t.Fatalf("html = %q, want the markup verbatim", html)
	}
	for _, format := range r.readFormats() {
		if format == formatCFHTML {
			t.Fatal("CF_HTML was read even though text/html was present")
		}
	}
}

func TestReadHTMLFallsBackToCFHTML(t *testing.T) {
	document := "<html><body><p>hello</p></body></html>"
	r := newFakeReader()
	r.data[formatCFHTML] = []byte(cfhtmlPayload(document, 12, 20))

	html, err := newTestClipboard(r).Read(context.Background(), capture.RepHTML)
	if err != nil {
		t.Fatalf("Read(RepHTML): %v", err)
	}
	if string(html) != document[12:20] {
		t.Fatalf("html = %q, want the CF_HTML fragment %q", html, document[12:20])
	}
}

func TestReadHTMLRejectsPayloadThatIsNotCFHTML(t *testing.T) {
	r := newFakeReader()
	r.data[formatCFHTML] = []byte("<html>x</html>")

	_, err := newTestClipboard(r).Read(context.Background(), capture.RepHTML)
	if !errors.Is(err, capture.ErrNoData) {
		t.Fatalf("Read error = %v, want capture.ErrNoData for a header-less payload", err)
	}
}

func TestReadHTMLStopsOnRealFailure(t *testing.T) {
	busy := errors.New("pasteboard busy")
	r := newFakeReader()
	r.errs[formatHTML] = busy
	r.data[formatCFHTML] = []byte(cfhtmlPayload("<b>hi</b>", 0, 9))

	_, err := newTestClipboard(r).Read(context.Background(), capture.RepHTML)
	if !errors.Is(err, busy) {
		t.Fatalf("Read error = %v, want the pasteboard failure", err)
	}
	for _, format := range r.readFormats() {
		if format == formatCFHTML {
			t.Fatal("CF_HTML was read after text/html failed for a real reason")
		}
	}
}

func TestReadImagePrefersFmtImage(t *testing.T) {
	png := []byte("\x89PNG from the transcoder")
	r := newFakeReader()
	r.data[xclipboard.FmtImage] = png
	r.data[formatRawPNG] = []byte("should not be read")

	got, err := newTestClipboard(r).Read(context.Background(), capture.RepImage)
	if err != nil {
		t.Fatalf("Read(RepImage): %v", err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("image = %q, want the transcoded PNG", got)
	}
	for _, format := range r.readFormats() {
		if format == formatRawPNG {
			t.Fatal("the raw PNG format was read even though FmtImage worked")
		}
	}
}

// TestReadImageFallsBackToRawPNG covers the Windows quirk the probe found:
// FmtImage reads CF_DIBV5 alone, while Chromium, Office and Snip & Sketch
// publish PNG bytes under the de-facto "PNG" format, which the library exposes
// as image/png.
func TestReadImageFallsBackToRawPNG(t *testing.T) {
	png := []byte("\x89PNG from the app")
	r := newFakeReader()
	r.errs[xclipboard.FmtImage] = xclipboard.ErrNoData
	r.data[formatRawPNG] = png

	got, err := newTestClipboard(r).Read(context.Background(), capture.RepImage)
	if err != nil {
		t.Fatalf("Read(RepImage): %v", err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("image = %q, want the app's PNG bytes", got)
	}
}

func TestReadImageDoesNotFallBackOnRealFailure(t *testing.T) {
	busy := errors.New("clipboard busy")
	r := newFakeReader()
	r.errs[xclipboard.FmtImage] = busy
	r.data[formatRawPNG] = []byte("\x89PNG from the app")

	_, err := newTestClipboard(r).Read(context.Background(), capture.RepImage)
	if !errors.Is(err, busy) {
		t.Fatalf("Read error = %v, want the clipboard failure", err)
	}
	for _, format := range r.readFormats() {
		if format == formatRawPNG {
			t.Fatal("the raw PNG format was read after FmtImage failed for a real reason")
		}
	}
}

func TestReadUnknownRepresentationIsNoData(t *testing.T) {
	r := newFakeReader()

	_, err := newTestClipboard(r).Read(context.Background(), capture.Representation(0xfe))
	if !errors.Is(err, capture.ErrNoData) {
		t.Fatalf("Read error = %v, want capture.ErrNoData", err)
	}
	if formats := r.readFormats(); len(formats) != 0 {
		t.Fatalf("read formats = %v, want no clipboard read", formats)
	}
}

func TestReadFiles(t *testing.T) {
	r := newFakeReader()
	r.files = []string{"/tmp/a.txt", "/tmp/b.txt"}

	paths, err := newTestClipboard(r).ReadFiles(context.Background())
	if err != nil {
		t.Fatalf("ReadFiles: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/tmp/a.txt" {
		t.Fatalf("paths = %v, want the clipboard file list", paths)
	}
}

func TestReadFilesAbsentIsNoData(t *testing.T) {
	r := newFakeReader()
	r.filesErr = xclipboard.ErrUnsupported

	_, err := newTestClipboard(r).ReadFiles(context.Background())
	if !errors.Is(err, capture.ErrNoData) {
		t.Fatalf("ReadFiles error = %v, want capture.ErrNoData", err)
	}
}

func TestSensitiveReportsMarker(t *testing.T) {
	r := newFakeReader()
	r.sensitive = true

	sensitive, err := newTestClipboard(r).Sensitive(context.Background())
	if err != nil {
		t.Fatalf("Sensitive: %v", err)
	}
	if !sensitive {
		t.Fatal("Sensitive = false, want the clipboard's marker")
	}
}

// TestSensitiveWithoutMarkerReportsFalse covers the platforms with no
// password-manager marker: the library answers ErrUnsupported there, and
// capture treats any error as marked, so reporting false is what keeps every
// copy on those platforms from being dropped.
func TestSensitiveWithoutMarkerReportsFalse(t *testing.T) {
	r := newFakeReader()
	r.sensitiveErr = xclipboard.ErrUnsupported

	sensitive, err := newTestClipboard(r).Sensitive(context.Background())
	if err != nil {
		t.Fatalf("Sensitive: %v, want no error when the platform has no marker", err)
	}
	if sensitive {
		t.Fatal("Sensitive = true, want false when the platform has no marker")
	}
}

func TestSensitivePassesThroughRealFailures(t *testing.T) {
	busy := errors.New("clipboard busy")
	r := newFakeReader()
	r.sensitiveErr = busy

	_, err := newTestClipboard(r).Sensitive(context.Background())
	if !errors.Is(err, busy) {
		t.Fatalf("Sensitive error = %v, want the clipboard failure", err)
	}
}

func TestWatchFormatsCoverEveryReadableRepresentation(t *testing.T) {
	// A change is only signalled in a format Watch was given, so every format
	// Read can return has to be watched, rich text under both of its names.
	want := []xclipboard.Format{
		xclipboard.FmtText,
		xclipboard.FmtImage,
		xclipboard.FmtFiles,
		formatHTML,
		formatCFHTML,
	}
	got := watchFormats()
	if len(got) != len(want) {
		t.Fatalf("watchFormats = %v, want %v", got, want)
	}
	for i, format := range want {
		if got[i] != format {
			t.Fatalf("watchFormats = %v, want %v", got, want)
		}
	}
}

func TestWatchSignalsOncePerChange(t *testing.T) {
	r := newFakeReader()
	c := newTestClipboard(r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := c.Watch(ctx)
	r.events <- xclipboard.Data{Format: xclipboard.FmtText}
	r.events <- xclipboard.Data{Format: xclipboard.FmtImage}

	for i := 0; i < 2; i++ {
		select {
		case _, ok := <-changes:
			if !ok {
				t.Fatalf("Watch closed after %d signals", i)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for signal %d", i)
		}
	}
}

func TestWatchClosesWhenContextEnds(t *testing.T) {
	r := newFakeReader()
	c := newTestClipboard(r)
	ctx, cancel := context.WithCancel(context.Background())

	changes := c.Watch(ctx)
	cancel()

	select {
	case _, ok := <-changes:
		if ok {
			t.Fatal("Watch delivered a signal after the context ended")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not close its channel when the context ended")
	}
}

func TestWatchClosesWhenClipboardStopsWatching(t *testing.T) {
	r := newFakeReader()
	c := newTestClipboard(r)

	changes := c.Watch(context.Background())
	close(r.events)

	select {
	case _, ok := <-changes:
		if ok {
			t.Fatal("Watch delivered a signal from a closed source")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not close when the clipboard stopped watching")
	}
}

// TestWatchDoesNotLeakWhenNobodyReads covers a consumer that stops reading
// while the context is still alive: the adapter must abandon the signal and
// return instead of blocking on a channel nobody drains.
func TestWatchDoesNotLeakWhenNobodyReads(t *testing.T) {
	r := newFakeReader()
	c := newTestClipboard(r)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes := c.Watch(ctx)
	// Nobody reads changes while the context is alive, so the forwarding
	// goroutine ends up offering a signal that is never taken.
	for i := 0; i < cap(r.events); i++ {
		r.events <- xclipboard.Data{Format: xclipboard.FmtText}
	}
	cancel()

	// A signal already in flight may still arrive; what must not happen is the
	// channel staying open, which would leave the goroutine behind it blocked
	// for the lifetime of the process.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-changes:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("Watch did not close after the context ended")
		}
	}
}

// cfhtmlPayload builds a CF_HTML payload the way a Windows application does: a
// fixed-width header of byte offsets into the payload, then the document.
func cfhtmlPayload(document string, fragmentStart, fragmentEnd int) string {
	header := func(startHTML, endHTML, startFragment, endFragment int) string {
		return "Version:0.9\r\n" +
			"StartHTML:" + pad(startHTML) + "\r\n" +
			"EndHTML:" + pad(endHTML) + "\r\n" +
			"StartFragment:" + pad(startFragment) + "\r\n" +
			"EndFragment:" + pad(endFragment) + "\r\n"
	}
	shift := len(header(0, 0, 0, 0))
	return header(shift, shift+len(document), shift+fragmentStart, shift+fragmentEnd) + document
}

// pad renders a CF_HTML offset the way Windows does: ten digits, zero padded.
func pad(offset int) string {
	digits := []byte("0000000000")
	for i := len(digits) - 1; i >= 0 && offset > 0; i-- {
		digits[i] = byte('0' + offset%10)
		offset /= 10
	}
	return string(digits)
}
