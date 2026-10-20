package filepreview

import (
	"bufio"
	"encoding/binary"
	"image"
	"io"
)

const (
	// exifScanBytes bounds how much of a JPEG header is read while looking for
	// the EXIF segment. Real cameras put it within the first few kilobytes; a
	// file that does not is treated as having no orientation.
	exifScanBytes = 64 << 10
	// orientationTag is the EXIF tag holding the display orientation.
	orientationTag = 0x0112
	// exifShortType is the TIFF field type of that tag.
	exifShortType = 3
)

// jpegOrientation returns the EXIF orientation a JPEG stream declares, or 1
// when it declares none — a non-JPEG stream, a file without the segment, or a
// value outside the eight defined ones. It reads only header segments and stops
// at the image data, so it costs a few kilobytes of the file at most.
func jpegOrientation(source io.Reader) int {
	reader := bufio.NewReader(source)
	var startOfImage [2]byte
	if _, err := io.ReadFull(reader, startOfImage[:]); err != nil {
		return 1
	}
	if startOfImage[0] != 0xFF || startOfImage[1] != 0xD8 {
		return 1
	}

	for scanned := 0; scanned < exifScanBytes; {
		var header [2]byte
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return 1
		}
		scanned += len(header)
		if header[0] != 0xFF {
			return 1
		}
		marker := header[1]
		// A marker may be preceded by any number of 0xFF fill bytes.
		for marker == 0xFF {
			next, err := reader.ReadByte()
			if err != nil {
				return 1
			}
			marker = next
			scanned++
		}

		switch {
		case marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			// Standalone markers carry no length field.
			continue
		case marker == 0xDA || marker == 0xD9:
			// The image data starts before any EXIF segment could.
			return 1
		}

		var length [2]byte
		if _, err := io.ReadFull(reader, length[:]); err != nil {
			return 1
		}
		size := int(binary.BigEndian.Uint16(length[:]))
		if size < len(length) {
			return 1
		}
		scanned += size

		if marker != 0xE1 {
			if _, err := io.CopyN(io.Discard, reader, int64(size-len(length))); err != nil {
				return 1
			}
			continue
		}
		payload := make([]byte, size-len(length))
		if _, err := io.ReadFull(reader, payload); err != nil {
			return 1
		}
		if orientation := exifOrientation(payload); orientation > 1 {
			return orientation
		}
		return 1
	}
	return 1
}

// exifOrientation reads the orientation tag out of an APP1 payload, which is
// the EXIF marker's body: "Exif\0\0" followed by a TIFF directory. Anything
// unexpected reads as "no orientation" rather than an error, because a file
// with a damaged header still has a picture to show.
func exifOrientation(payload []byte) int {
	const headerBytes = len("Exif\x00\x00")
	if len(payload) < headerBytes+8 || string(payload[:headerBytes]) != "Exif\x00\x00" {
		return 0
	}
	tiff := payload[headerBytes:]

	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	if order.Uint16(tiff[2:]) != 42 {
		return 0
	}
	offset := int(order.Uint32(tiff[4:]))
	if offset < 8 || offset+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[offset:]))
	entries := tiff[offset+2:]
	if count > len(entries)/12 {
		count = len(entries) / 12
	}
	for index := 0; index < count; index++ {
		entry := entries[index*12 : index*12+12]
		if order.Uint16(entry) != orientationTag {
			continue
		}
		if order.Uint16(entry[2:]) != exifShortType || order.Uint32(entry[4:]) != 1 {
			return 0
		}
		value := int(order.Uint16(entry[8:]))
		if value < 1 || value > 8 {
			return 0
		}
		return value
	}
	return 0
}

// applyOrientation turns a decoded picture upright as its EXIF orientation
// says. The eight values describe the transforms a camera applies to stored
// pixels for display: identity, both flips, both 90° turns, both mirrors across
// a diagonal, and the 180° turn. It runs on the thumbnail, so the per-pixel
// copy costs little.
func applyOrientation(source *image.RGBA, orientation int) *image.RGBA {
	if orientation <= 1 || orientation > 8 {
		return source
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	targetWidth, targetHeight := width, height
	if orientation >= 5 {
		// The diagonal and quarter-turn cases swap the axes.
		targetWidth, targetHeight = height, width
	}
	target := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			var sourceX, sourceY int
			switch orientation {
			case 2: // mirror horizontally
				sourceX, sourceY = width-1-x, y
			case 3: // rotate 180°
				sourceX, sourceY = width-1-x, height-1-y
			case 4: // mirror vertically
				sourceX, sourceY = x, height-1-y
			case 5: // mirror across the main diagonal
				sourceX, sourceY = y, x
			case 6: // rotate 90° clockwise
				sourceX, sourceY = y, height-1-x
			case 7: // mirror across the anti-diagonal
				sourceX, sourceY = width-1-y, height-1-x
			case 8: // rotate 90° counter-clockwise
				sourceX, sourceY = width-1-y, x
			}
			target.SetRGBA(x, y, source.RGBAAt(sourceX, sourceY))
		}
	}
	return target
}
