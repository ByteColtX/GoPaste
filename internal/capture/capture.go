// Package capture turns clipboard changes into history items.
//
// It owns the whole capture path: noticing that the clipboard changed,
// settling the several notifications one copy produces, reading and
// classifying what was copied, and deciding whether it is worth keeping. It
// depends on no UI toolkit and no network, so it runs and is tested on its
// own.
package capture

import (
	"context"
	"errors"
	"time"

	"gopaste/internal/history"
)

// Representation is one clipboard payload the capture path understands.
type Representation uint8

const (
	// RepText is plain text, encoded as UTF-8.
	RepText Representation = iota
	// RepHTML is rich text, encoded as markup.
	RepHTML
	// RepImage is an image, always encoded as PNG.
	RepImage
)

// ErrNoData means the clipboard holds nothing in the requested representation.
// It is the ordinary empty case, not a failure to reach the clipboard.
var ErrNoData = errors.New("capture: no data for representation")

// Source is the clipboard surface capture reads through. The application
// supplies an adapter over MyGo; tests supply a fake.
//
// Read is called once per representation, in preference order, on the same
// goroutine as Loop. A source reads the clipboard and never writes to it.
type Source interface {
	// Read returns one representation's payload, or ErrNoData when the
	// clipboard does not offer it. Image payloads are PNG.
	Read(ctx context.Context, rep Representation) ([]byte, error)
	// ReadFiles returns the local paths in the clipboard's file list, or
	// ErrNoData when it holds none.
	ReadFiles(ctx context.Context) ([]string, error)
	// Sensitive reports the marker password managers set: content that must
	// not be kept or shared. A platform with no such marker reports false.
	Sensitive(ctx context.Context) (bool, error)
}

// Trigger reports that the clipboard may have changed. A trigger may signal
// more than once for one change, and may signal late: capture coalesces a
// burst into a single reading, so a trigger never has to be exact.
type Trigger interface {
	// Watch returns a channel that receives one value per change and is
	// closed when ctx ends.
	Watch(ctx context.Context) <-chan struct{}
}

// Snapshot is one settled reading of the clipboard.
type Snapshot struct {
	At time.Time
	// Kind is the richest representation found: files beat images, images
	// beat rich text, rich text beats plain text. KindUnknown means the
	// clipboard held nothing the capture path keeps.
	Kind history.Kind
	// Text is the plain-text representation, empty when absent.
	Text string
	// HTML is the rich-text representation as markup, empty when absent.
	HTML string
	// Image is the image representation, nil when absent.
	Image *history.Image
	// Files are the local paths in the clipboard's file list.
	Files []string
	// Sensitive records the password-manager marker. Loop never emits a
	// sensitive snapshot.
	Sensitive bool
}

// Item converts the snapshot into the history item it should become, and
// reports false when nothing worth keeping was copied.
//
// The item ID is a digest of the content, so copying the same content twice
// yields the same ID. That is also what keeps a change this process caused
// itself out of the history: writing a stored item back to the clipboard
// reads back as the same content, resolves to the ID already stored, and the
// store rejects it as a duplicate.
func (s Snapshot) Item() (history.Item, bool) {
	switch s.Kind {
	case history.KindText:
		if s.Text == "" {
			return history.Item{}, false
		}
		return history.Item{
			ID:        s.digest(),
			Kind:      history.KindText,
			Text:      s.Text,
			CreatedAt: s.At,
		}, true
	case history.KindRichText:
		if s.HTML == "" {
			return history.Item{}, false
		}
		// The markup is what makes the item rich text, and Text keeps the
		// plain-text representation beside it so the list and the preview stay
		// readable and Store.Copy can write both back out.
		return history.Item{
			ID:        s.digest(),
			Kind:      history.KindRichText,
			Text:      s.Text,
			HTML:      s.HTML,
			CreatedAt: s.At,
		}, true
	case history.KindImage:
		if s.Image == nil || len(s.Image.PNG) == 0 {
			return history.Item{}, false
		}
		return history.Item{
			ID:        s.digest(),
			Kind:      history.KindImage,
			Image:     s.Image,
			CreatedAt: s.At,
		}, true
	case history.KindFile:
		if len(s.Files) == 0 {
			return history.Item{}, false
		}
		return history.Item{
			ID:        s.digest(),
			Kind:      history.KindFile,
			Text:      filePathsText(s.Files),
			FileSize:  totalFileSize(s.Files),
			CreatedAt: s.At,
		}, true
	default:
		return history.Item{}, false
	}
}

// Options configures a Capture. The zero value is usable.
type Options struct {
	// Quiet is how long the clipboard must stay unchanged before a change is
	// read. One copy produces one signal per representation it carries, so
	// Quiet turns that burst into a single reading. Zero means 400ms.
	Quiet time.Duration
	// Now returns the current time. Tests set it; nil means time.Now.
	Now func() time.Time
}

// Capture turns clipboard changes into snapshots.
type Capture struct {
	src     Source
	trigger Trigger
	opts    Options
}

// New builds a Capture over a clipboard source and a change trigger.
func New(src Source, trigger Trigger, opts Options) *Capture {
	return &Capture{src: src, trigger: trigger, opts: opts}
}

// Loop reads the clipboard once, then once per settled change, calling emit
// for each reading worth keeping until ctx ends. emit runs on Loop's
// goroutine: it must publish to the UI thread itself and must not block.
//
// A representation that cannot be read is skipped rather than reported,
// because the next change reads it again. Loop reports an error only when the
// trigger itself fails.
func (c *Capture) Loop(ctx context.Context, emit func(Snapshot)) error {
	quiet := c.opts.Quiet
	if quiet <= 0 {
		quiet = defaultQuiet
	}

	// Read once at start, before any change is signalled, so a capture that
	// starts with content already on the clipboard does not miss it.
	c.emitReading(ctx, emit)

	changes := c.trigger.Watch(ctx)
	var timer *time.Timer
	var timerC <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-changes:
			if !ok {
				return nil
			}
			// Restart the quiet timer so a burst of signals from one
			// copy settles into a single reading.
			if timer == nil {
				timer = time.NewTimer(quiet)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(quiet)
			}
			timerC = timer.C
		case <-timerC:
			timerC = nil
			c.emitReading(ctx, emit)
		}
	}
}
