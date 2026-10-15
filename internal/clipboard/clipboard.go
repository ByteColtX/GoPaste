// Package clipboard adapts the operating-system clipboard to the capture
// module's Source and Trigger seams.
//
// It is the only package in GoPaste that imports a clipboard library, and the
// only one that knows how each platform names the representations it
// publishes, so internal/capture stays free of platform code and keeps its
// own tests running without a clipboard. It reads and watches; it never writes
// to the clipboard, so nothing here can disturb what the user copied.
//
// The adapter is deliberately not wired into the application yet: nothing
// constructs it, and it owns no state beyond the seam it reads through.
package clipboard

import (
	"context"
	"errors"

	xclipboard "golang.design/x/clipboard"

	"gopaste/internal/capture"
)

// The clipboard names the adapter asks for. FmtText, FmtImage and FmtFiles are
// the library's built-ins. The rest are registered on demand: Register is the
// raw passthrough that reaches a platform-native format, which is also how
// Windows' CF_HTML is found under its registered name.
var (
	formatHTML   = xclipboard.Register("text/html")
	formatCFHTML = xclipboard.Register("HTML Format")
	formatRawPNG = xclipboard.Register("image/png")
)

// reader is the part of golang.design/x/clipboard the adapter uses, behind an
// interface so the format fallbacks and the error mapping below are tested
// without a real clipboard. It is narrower than the library on purpose: the
// adapter only ever reads the ordinary clipboard, never the X11/Wayland
// primary selection, so no option is threaded through here. The library's own
// functions are the production implementation.
type reader interface {
	Read(ctx context.Context, format xclipboard.Format) ([]byte, error)
	ReadFiles(ctx context.Context) ([]string, error)
	Sensitive(ctx context.Context) (bool, error)
	Watch(ctx context.Context) <-chan xclipboard.Data
}

// systemClipboard is the real reader: the clipboard library's package
// functions, which are addressed by name rather than through a value.
type systemClipboard struct{}

func (systemClipboard) Read(ctx context.Context, format xclipboard.Format) ([]byte, error) {
	return xclipboard.Read(ctx, format)
}

func (systemClipboard) ReadFiles(ctx context.Context) ([]string, error) {
	return xclipboard.ReadFiles(ctx)
}

func (systemClipboard) Sensitive(ctx context.Context) (bool, error) {
	return xclipboard.Sensitive(ctx)
}

func (systemClipboard) Watch(ctx context.Context) <-chan xclipboard.Data {
	return xclipboard.Watch(ctx, watchFormats()...)
}

// Clipboard is the operating-system clipboard as a capture.Source and a
// capture.Trigger. Construct it with New.
type Clipboard struct {
	reader reader
}

var (
	_ capture.Source  = (*Clipboard)(nil)
	_ capture.Trigger = (*Clipboard)(nil)
)

// New prepares the clipboard and returns it. It fails when there is no
// clipboard to talk to, such as on a Linux machine with no display: capture
// cannot run there, and the error says so once instead of every read failing
// quietly later. Calling New again returns the same result.
func New() (*Clipboard, error) {
	if err := xclipboard.Init(); err != nil {
		return nil, err
	}
	return &Clipboard{reader: systemClipboard{}}, nil
}

// Read returns one representation's payload, or capture.ErrNoData when the
// clipboard does not offer it. Image payloads are PNG.
func (c *Clipboard) Read(ctx context.Context, rep capture.Representation) ([]byte, error) {
	switch rep {
	case capture.RepText:
		return c.read(ctx, xclipboard.FmtText)
	case capture.RepHTML:
		return c.readHTML(ctx)
	case capture.RepImage:
		return c.readImage(ctx)
	default:
		return nil, capture.ErrNoData
	}
}

// ReadFiles returns the local paths in the clipboard's file list, or
// capture.ErrNoData when it holds none.
func (c *Clipboard) ReadFiles(ctx context.Context) ([]string, error) {
	paths, err := c.reader.ReadFiles(ctx)
	if err != nil {
		return nil, absent(err)
	}
	if len(paths) == 0 {
		return nil, capture.ErrNoData
	}
	return paths, nil
}

// Sensitive reports the marker password managers set: content that must not be
// kept or shared.
//
// A platform with no such marker reports false rather than failing. The
// library answers ErrUnsupported there, and capture treats any error as
// marked, so passing that error on would drop every copy on iOS, Android and
// in the browser. Every other error is passed on, which keeps capture
// failing closed when the clipboard itself cannot be read.
func (c *Clipboard) Sensitive(ctx context.Context) (bool, error) {
	sensitive, err := c.reader.Sensitive(ctx)
	switch {
	case err == nil:
		return sensitive, nil
	case errors.Is(err, xclipboard.ErrUnsupported), errors.Is(err, xclipboard.ErrNoData):
		return false, nil
	default:
		return false, err
	}
}

// Watch returns a channel that receives one value per clipboard change and is
// closed when ctx ends. It watches every representation capture can keep, so a
// copy that carries rich text is noticed through its markup and not only
// through the plain text beside it.
//
// Watch is a hint, not the content: capture reads the clipboard itself once the
// change settles, so a missed or duplicated signal only costs a re-read.
func (c *Clipboard) Watch(ctx context.Context) <-chan struct{} {
	changes := make(chan struct{})
	events := c.reader.Watch(ctx)

	go func() {
		defer close(changes)
		for {
			select {
			case _, ok := <-events:
				if !ok {
					return
				}
				// Give up on a signal nobody is listening for rather than
				// holding the goroutine past the context that owns it.
				select {
				case changes <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return changes
}

// read reads one format and maps the library's ordinary "nothing there" cases
// onto capture.ErrNoData, so a format that is simply absent, or one this
// platform cannot express, is skipped instead of reported as a failure.
func (c *Clipboard) read(ctx context.Context, format xclipboard.Format) ([]byte, error) {
	buf, err := c.reader.Read(ctx, format)
	if err != nil {
		return nil, absent(err)
	}
	if len(buf) == 0 {
		return nil, capture.ErrNoData
	}
	return buf, nil
}

// readHTML reads the rich-text representation. text/html is tried first: it is
// the portable name and carries the markup verbatim.
//
// Windows publishes the same content as CF_HTML under the registered name
// "HTML Format" instead, and its payload wraps the fragment in a header of
// byte offsets. The fragment is what the user selected, so it is extracted
// before the markup is returned; a payload that is not really CF_HTML is
// reported as absent rather than filed with its header as the copied markup.
func (c *Clipboard) readHTML(ctx context.Context) ([]byte, error) {
	html, err := c.read(ctx, formatHTML)
	switch {
	case err == nil:
		return html, nil
	case !errors.Is(err, capture.ErrNoData):
		return nil, err
	}

	payload, err := c.read(ctx, formatCFHTML)
	if err != nil {
		return nil, err
	}
	fragment, ok := cfhtmlFragment(payload)
	if !ok {
		return nil, capture.ErrNoData
	}
	return fragment, nil
}

// readImage reads the image representation. FmtImage is tried first: it is the
// one that transcodes whatever the platform stores into PNG.
//
// On Windows it reads CF_DIBV5 alone, while Chromium, Firefox, Office and
// Snip & Sketch publish PNG bytes under the de-facto "PNG" format instead — a
// format the library already advertises as image/png, so the clipboard looks
// like it holds an image that FmtImage then cannot find. Register("image/png")
// is the raw passthrough to that format (and to public.png on macOS), so it is
// the fallback. Elsewhere it finds nothing and costs one failed read.
func (c *Clipboard) readImage(ctx context.Context) ([]byte, error) {
	png, err := c.read(ctx, xclipboard.FmtImage)
	if err == nil {
		return png, nil
	}
	if !errors.Is(err, capture.ErrNoData) {
		return nil, err
	}
	return c.read(ctx, formatRawPNG)
}

// watchFormats is every representation the adapter reads, so a change in any
// of them is signalled.
//
// Rich text has to be watched under both of its names: a browser copy carries
// text/html, and Windows publishes the same content as CF_HTML. "HTML Format"
// is the Windows clipboard format name, not a MIME type, so Register carries it
// through verbatim; on macOS and Linux no data ever appears under it, which
// costs one idle watcher.
//
// RTF is not watched. The history has no RTF payload, so a copy that carries
// only RTF is not something capture can keep, and a copy that carries RTF
// alongside text or HTML is already signalled through those.
func watchFormats() []xclipboard.Option {
	return []xclipboard.Option{
		xclipboard.FmtText,
		xclipboard.FmtImage,
		xclipboard.FmtFiles,
		formatHTML,
		formatCFHTML,
	}
}

// absent maps the clipboard library's "nothing there" errors onto capture's.
// ErrUnsupported counts: a platform that cannot express a format does not offer
// it, and capture skips what it cannot read. Anything else is passed through
// unchanged, so a clipboard that cannot be reached still reads as a failure.
func absent(err error) error {
	switch {
	case errors.Is(err, xclipboard.ErrNoData), errors.Is(err, xclipboard.ErrUnsupported):
		return capture.ErrNoData
	default:
		return err
	}
}
