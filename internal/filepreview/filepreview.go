// Package filepreview materialises the preview a file history item carries:
// a downscaled picture for an image file, a short excerpt for a text-like one,
// and nothing for everything else. It runs where the file's content lands — the
// capture goroutine for a local copy, the transfer handler for a received one —
// so drawing a frame never touches the filesystem.
//
// It depends on nothing but the standard library, golang.org/x/image and the
// two packages that own the item type and the file classification, so the rules
// can be tested on their own against real files.
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
	"gopaste/internal/history"

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

	// excerptBytes is how much of a text file is read. Everything the excerpt
	// keeps fits in it, so one read is enough and a huge file costs the same.
	excerptBytes = 4 << 10
	// excerptLines and excerptRunes bound the excerpt itself: a minified file
	// is one enormous line, a log is thousands of short ones, and both are cut
	// at whichever limit comes first.
	excerptLines = 30
	excerptRunes = 1200
	// ellipsis marks a cut excerpt, so the preview does not read as the whole
	// file.
	ellipsis = "…"
)

// Generate returns the preview to store for the file at path, or nil when the
// file has nothing to show: an unsupported format, a directory, a file that is
// gone, or content that does not match its name. A nil preview is not an error;
// it is what the interface renders as "no preview".
func Generate(path string) *history.FilePreview {
	format := filetype.Detect(path)
	switch {
	case format.Kind() == filetype.KindImage:
		return imagePreview(path)
	case isTextLike(format):
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
func imagePreview(path string) *history.FilePreview {
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
	return &history.FilePreview{Image: &history.Image{
		PNG:    payload.Bytes(),
		Width:  thumbnail.Bounds().Dx(),
		Height: thumbnail.Bounds().Dy(),
	}}
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

// textPreview returns the beginning of a text-like file, or nil when the bytes
// are not text: an excerpt of a binary file is worse than the "no preview"
// note, because it looks like the file's content.
func textPreview(path string) *history.FilePreview {
	file, ok := openRegular(path)
	if !ok {
		return nil
	}
	defer file.Close()

	buffer := make([]byte, excerptBytes)
	read, err := io.ReadFull(file, buffer)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil
	}
	data := trimPartialRune(buffer[:read])
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil
	}
	text := excerpt(string(data))
	if text == "" {
		return nil
	}
	return &history.FilePreview{Text: text}
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

// excerpt cuts the read prefix down to the line and rune bounds and marks the
// cut. Blank text yields "", so a file of whitespace reads as having nothing to
// preview.
func excerpt(text string) string {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	truncated := false
	if lines := strings.Split(text, "\n"); len(lines) > excerptLines {
		text = strings.Join(lines[:excerptLines], "\n")
		truncated = true
	}
	if runes := []rune(text); len(runes) > excerptRunes {
		text = string(runes[:excerptRunes])
		truncated = true
	}
	text = strings.TrimRight(text, " \t\n")
	if text == "" {
		return ""
	}
	if truncated {
		text += ellipsis
	}
	return text
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
