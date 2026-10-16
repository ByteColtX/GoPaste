package capture

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"image"
	"image/draw"
	"io"
	"strings"

	"gopaste/internal/history"
)

// The digest is what makes a captured item's ID content-derived. Writing a
// stored item back to the clipboard reads back as the same content, resolves
// to the same ID, and history.Store.Append rejects it as a duplicate instead
// of keeping the copy twice. The domain tag keeps the same bytes under
// different kinds apart, so a file list and the text with those bytes never
// collide.
const (
	domainText     = "text"
	domainRichText = "richtext"
	domainFile     = "file"
	domainImage    = "image"
)

// digest is the content digest of the reading, hex encoded for Item.ID.
//
// It is SHA-256 over a domain-separated encoding of the kind and the canonical
// content. The domain tag is written first, followed by a NUL byte, then the
// canonical content:
//
//   - plain text and HTML: CRLF normalised to LF, trailing NUL bytes dropped;
//   - files: the paths in order, newline-joined;
//   - image: the decoded pixels (width, height, RGBA bytes), so a PNG the
//     platform re-encoded still hashes the same; a PNG that cannot be decoded
//     falls back to its own bytes.
func (s Snapshot) digest() string {
	h := sha256.New()
	switch s.Kind {
	case history.KindText:
		writeDomain(h, domainText)
		h.Write(canonicalText([]byte(s.Text)))
	case history.KindRichText:
		writeDomain(h, domainRichText)
		h.Write(canonicalText([]byte(s.HTML)))
	case history.KindFile:
		writeDomain(h, domainFile)
		io.WriteString(h, strings.Join(s.Files, "\n"))
	case history.KindImage:
		writeDomain(h, domainImage)
		writeImageDigest(h, s.Image.PNG)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// writeDomain writes a domain tag and its separator so two kinds can never
// produce the same byte stream for the same content.
func writeDomain(h hash.Hash, domain string) {
	io.WriteString(h, domain)
	h.Write([]byte{0})
}

// canonicalText is the byte form hashed for text and markup.
func canonicalText(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.TrimRight(b, "\x00")
}

// writeImageDigest hashes an image's decoded pixels rather than its PNG bytes.
// Decoding to a canonical RGBA buffer makes the digest independent of how the
// PNG was encoded, so a re-encoded copy of the same pixels keeps its ID.
func writeImageDigest(h hash.Hash, png []byte) {
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		h.Write(png)
		return
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	var size [16]byte
	binary.BigEndian.PutUint64(size[0:8], uint64(width))
	binary.BigEndian.PutUint64(size[8:16], uint64(height))
	h.Write(size[:])

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	h.Write(rgba.Pix)
}
