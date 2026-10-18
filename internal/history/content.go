package history

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
)

// Content is a clipboard payload in the form item identity is derived from. The
// capture path builds one from a clipboard reading and the sync path builds one
// from a peer's message, so both sides derive the same ID for the same content.
type Content struct {
	Kind Kind
	// Text is the plain-text representation, empty when absent.
	Text string
	// HTML is the markup of a rich-text payload, empty for every other kind.
	HTML string
	// Image is the image payload, nil for every other kind.
	Image *Image
	// Files are the paths of a file payload, in clipboard order. A file item
	// stores them joined as Text, so identity needs the list itself.
	Files []string
}

// The domain tag keeps the same bytes under different kinds apart, so a file
// list and the text with those bytes never collide.
const (
	domainText     = "text"
	domainRichText = "richtext"
	domainFile     = "file"
	domainImage    = "image"
)

// ContentID is the content-derived ID an item with this content carries, hex
// encoded.
//
// It is what makes an item's ID stable: writing a stored item back to the
// clipboard reads back as the same content, resolves to the same ID, and
// Store.Append rejects it as a duplicate instead of keeping the copy twice. The
// same property holds across devices, so the receiver of a synced payload
// derives the ID itself rather than trusting one a peer sends, and the same
// content copied on two machines becomes one ID rather than two records.
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
func ContentID(c Content) string {
	h := sha256.New()
	switch c.Kind {
	case KindText:
		writeDomain(h, domainText)
		h.Write(canonicalText([]byte(c.Text)))
	case KindRichText:
		writeDomain(h, domainRichText)
		h.Write(canonicalText([]byte(c.HTML)))
	case KindFile:
		writeDomain(h, domainFile)
		io.WriteString(h, strings.Join(c.Files, "\n"))
	case KindImage:
		writeDomain(h, domainImage)
		if c.Image != nil {
			writeImageDigest(h, c.Image.PNG)
		}
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
