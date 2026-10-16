package capture

import (
	"bytes"
	"context"
	"image"
	_ "image/png" // register the PNG decoder for image.Decode and DecodeConfig
	"os"
	"strings"
	"time"

	"gopaste/internal/history"
)

// defaultQuiet is how long the clipboard must stay unchanged before a change
// is read when Options.Quiet is zero.
const defaultQuiet = 400 * time.Millisecond

// kindUnknown is the Kind of a reading that held nothing the capture path
// keeps. history has no such constant because every stored item carries a real
// kind; the sentinel only exists between a read and its classification.
const kindUnknown history.Kind = 0xff

// read reads every representation the source offers and classifies the result.
// A representation that cannot be read is skipped: its absence never stops the
// others from being read, and the next change reads it again.
func (c *Capture) read(ctx context.Context) Snapshot {
	snap := Snapshot{At: c.now()}

	// The marker is read before the content so a marked clipboard is never read
	// at all, and again afterwards: the clipboard can change in between, and the
	// answer has to describe the content that was just read. Either answer drops
	// the reading whole, so a marked snapshot carries no payload and cannot be
	// turned into an item even by a caller that ignores Sensitive.
	if marked(ctx, c.src) {
		return Snapshot{At: c.now(), Kind: kindUnknown, Sensitive: true}
	}
	if files, err := c.src.ReadFiles(ctx); err == nil {
		snap.Files = append([]string(nil), files...)
	}
	if png, err := c.src.Read(ctx, RepImage); err == nil && len(png) > 0 {
		width, height := decodeImageSize(png)
		snap.Image = &history.Image{
			PNG:    append([]byte(nil), png...),
			Width:  width,
			Height: height,
		}
	}
	if html, err := c.src.Read(ctx, RepHTML); err == nil && len(html) > 0 {
		snap.HTML = string(html)
	}
	if text, err := c.src.Read(ctx, RepText); err == nil && len(text) > 0 {
		snap.Text = string(text)
	}
	if marked(ctx, c.src) {
		return Snapshot{At: snap.At, Kind: kindUnknown, Sensitive: true}
	}

	snap.Kind = classify(snap)
	return snap
}

// marked reports whether the clipboard carries the password-manager marker. A
// marker that cannot be read counts as marked: a tool that syncs the clipboard
// must not keep a password because the marker happened to be unreadable, and
// the next change reads the clipboard again. The platforms GoPaste runs on
// report "no such marker here" as false rather than as an error, so only a
// clipboard that cannot be read at all reaches the error case.
func marked(ctx context.Context, src Source) bool {
	sensitive, err := src.Sensitive(ctx)
	return err != nil || sensitive
}

// classify picks the richest representation the reading carries. Files beat
// images because a file copy also carries an icon image, images beat rich text,
// and rich text beats plain text. A reading with none of them is kindUnknown.
func classify(s Snapshot) history.Kind {
	switch {
	case len(s.Files) > 0:
		return history.KindFile
	case s.Image != nil:
		return history.KindImage
	case s.HTML != "":
		return history.KindRichText
	case s.Text != "":
		return history.KindText
	default:
		return kindUnknown
	}
}

// decodeImageSize reads the pixel dimensions from the PNG header. A payload
// that cannot be decoded leaves them zero; that is not an error, because the
// image still has its own bytes to keep and hash.
func decodeImageSize(png []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(png))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// filePathsText is the file list as the item's text: the absolute paths joined
// by a single newline.
func filePathsText(paths []string) string {
	return strings.Join(paths, "\n")
}

// totalFileSize sums the sizes os.Stat reports, ignoring paths that cannot be
// stat'd so one missing file never discards the whole reading.
func totalFileSize(paths []string) int64 {
	var total int64
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		total += info.Size()
	}
	return total
}

// now returns the reading time, using time.Now unless the caller supplied a
// clock for tests.
func (c *Capture) now() time.Time {
	if c.opts.Now != nil {
		return c.opts.Now()
	}
	return time.Now()
}

// emitReading reads once and passes the reading to emit unless it is sensitive
// or holds nothing worth keeping.
func (c *Capture) emitReading(ctx context.Context, emit func(Snapshot)) {
	snap := c.read(ctx)
	if snap.Sensitive || snap.Kind == kindUnknown {
		return
	}
	emit(snap)
}
