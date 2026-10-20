// Package filepreview produces the preview of a file on disk: a downscaled
// picture for an image file, the file's text for a text-like one, and nothing
// for everything else. It is what the preview pane shows when a file item is
// selected, so it reads the file — callers run it off the UI thread and keep
// the result for as long as they like.
//
// It depends on nothing but the standard library, golang.org/x/image and the
// package that classifies file names, so the rules can be tested on their own
// against real files.
package filepreview

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf8"

	xdraw "golang.org/x/image/draw"

	"gopaste/internal/filetype"

	// The blank imports register every format a copied file may be in with
	// image.Decode. PNG comes with the encoder above; the rest are decode-only.
	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

const (
	// thumbLongEdge is the longest edge of a downscaled picture, in pixels. It
	// is the trade-off between how sharp the preview card looks and how much a
	// thumbnail adds to the database: a photo lands around 200–400 KB as PNG,
	// a screenshot far below that, and an image already this small is kept at
	// its own size instead of being enlarged.
	thumbLongEdge = 640
	// maxSourcePixels bounds the picture a preview may decode. A pasted file
	// can be far larger than any clipboard screenshot, and decoding one costs
	// memory in proportion to its pixel count, so past this bound the item
	// shows the "no preview" note instead of stalling the capture.
	maxSourcePixels = 40_000_000

	// maxTextBytes is how much of a text file is read and shown. A preview is
	// for reading, not for editing: this covers a README or a few thousand log
	// lines, and bounds both the read and what the interface has to lay out.
	maxTextBytes = 32 << 10
	// maxTextLines bounds the same content by lines, because a file of short
	// lines reaches a lot of them inside maxTextBytes. Laying out one text block
	// of a few thousand lines costs tens of milliseconds per frame (measured:
	// past ~1200 lines the preview drops from well under a millisecond to ~25),
	// so a preview stops at a thousand and says it was cut.
	maxTextLines = 1000
	// ellipsis marks a cut text, so the preview does not read as the whole file.
	ellipsis = "…"
)

// previewKind is what a format's content can be shown as.
type previewKind uint8

const (
	// previewNone means the format has nothing this package can show.
	previewNone previewKind = iota
	// previewPicture is a downscaled rendering of an image file.
	previewPicture
	// previewExcerpt is the beginning of a text-like file.
	previewExcerpt
)

// previewOf decides what a format is previewed as. It is the one place that
// answers the question, so CanPreview and Generate cannot drift apart.
func previewOf(format filetype.Format) previewKind {
	switch {
	case format.Kind() == filetype.KindImage:
		return previewPicture
	case isTextLike(format):
		return previewExcerpt
	default:
		return previewNone
	}
}

// CanPreview reports whether Generate would attempt a preview for a name. It
// reads the format only, so it costs no I/O: the preview pane asks it before
// starting a background generation, so an archive or a font never causes one.
func CanPreview(path string) bool {
	return previewOf(filetype.Detect(path)) != previewNone
}

// Preview is what a file is previewed as: either a picture or the text itself.
// Exactly one of the two is filled in, because a format is previewed as one or
// the other.
type Preview struct {
	// PNG is a downscaled PNG rendering of an image file.
	PNG []byte
	// Width and Height are the PNG's pixel size.
	Width  int
	Height int
	// Text is the content of a text-like file, cut at maxTextBytes.
	Text string
}

// Generate returns the preview of the file at path, or nil when the file has
// nothing to show: an unsupported format, a directory, a file that is gone, or
// content that does not match its name. A nil preview is not an error; it is
// what the interface renders as "no preview".
func Generate(path string) *Preview {
	switch previewOf(filetype.Detect(path)) {
	case previewPicture:
		return imagePreview(path)
	case previewExcerpt:
		return textPreview(path)
	default:
		return nil
	}
}

// isTextLike reports whether a format's content is text the preview can show.
// CSV joins the text and code families: it is a spreadsheet to the icon table,
// but its bytes are still lines of text.
func isTextLike(format filetype.Format) bool {
	switch format.Kind() {
	case filetype.KindText, filetype.KindCode:
		return true
	default:
		return format == filetype.FormatCSV
	}
}

// imagePreview decodes an image file and returns a downscaled PNG of it.
//
// The header is read first: it gives the pixel count for the bound, and it is
// also where the EXIF orientation lives, so the picture is turned upright
// before it is scaled. Applying the turn after scaling keeps it cheap — the
// transform runs over the thumbnail, not the original.
func imagePreview(path string) *Preview {
	file, ok := openRegular(path)
	if !ok {
		return nil
	}
	defer file.Close()

	orientation := jpegOrientation(file)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil
	}
	config, _, err := image.DecodeConfig(file)
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return nil
	}
	if int64(config.Width)*int64(config.Height) > maxSourcePixels {
		return nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil
	}
	source, _, err := image.Decode(file)
	if err != nil {
		return nil
	}

	thumbnail := applyOrientation(downscale(source), orientation)
	var payload bytes.Buffer
	if err := png.Encode(&payload, thumbnail); err != nil {
		return nil
	}
	return &Preview{
		PNG:    payload.Bytes(),
		Width:  thumbnail.Bounds().Dx(),
		Height: thumbnail.Bounds().Dy(),
	}
}

// downscale returns the picture fitted into thumbLongEdge, keeping its aspect
// ratio. An image already inside the bound is copied as-is rather than blown
// up: a 16×16 icon has no more detail to show.
func downscale(source image.Image) *image.RGBA {
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	targetWidth, targetHeight := width, height
	if longest := max(width, height); longest > thumbLongEdge {
		scale := float64(thumbLongEdge) / float64(longest)
		targetWidth = max(1, int(math.Round(float64(width)*scale)))
		targetHeight = max(1, int(math.Round(float64(height)*scale)))
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	if targetWidth == width && targetHeight == height {
		xdraw.Draw(target, target.Bounds(), source, bounds.Min, xdraw.Src)
		return target
	}
	xdraw.CatmullRom.Scale(target, target.Bounds(), source, bounds, xdraw.Src, nil)
	return target
}

// textPreview returns the content of a text-like file, or nil when the bytes
// are not text: a rendering of a binary file is worse than the "no preview"
// note, because it looks like the file's content.
func textPreview(path string) *Preview {
	file, ok := openRegular(path)
	if !ok {
		return nil
	}
	defer file.Close()

	buffer := make([]byte, maxTextBytes)
	read, err := io.ReadFull(file, buffer)
	truncated := err == nil
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil
	}
	data := trimPartialRune(buffer[:read])
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil
	}
	text := readableText(string(data))
	if text == "" {
		return nil
	}
	if lines := strings.Split(text, "\n"); len(lines) > maxTextLines {
		text = strings.TrimRight(strings.Join(lines[:maxTextLines], "\n"), " \t\n")
		truncated = true
	}
	if truncated {
		text += ellipsis
	}
	return &Preview{Text: text}
}

// trimPartialRune drops the incomplete character a byte bound can leave at the
// end of the read, so a file of Chinese text is not mistaken for binary just
// because the cut landed inside one of its characters. Content that is invalid
// anywhere else is left alone for the caller to reject.
func trimPartialRune(data []byte) []byte {
	for i := 0; i < utf8.UTFMax && len(data) > 0; i++ {
		if utf8.Valid(data) {
			return data
		}
		if r, size := utf8.DecodeLastRune(data); r != utf8.RuneError || size != 1 {
			return data
		}
		data = data[:len(data)-1]
	}
	return data
}

// readableText normalises line endings and drops the byte-order mark, so the
// text reads the same as in an editor. Blank text yields "", so a file of
// whitespace reads as having nothing to preview.
func readableText(text string) string {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimRight(text, " \t\n")
}

// openRegular opens path when it is a regular file. A directory, a device or a
// path that is gone has nothing to preview, and a named pipe must not be read:
// it would block the caller.
func openRegular(path string) (*os.File, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	return file, true
}
