package filepreview

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
)

// writeFile writes data into a fresh file under a temporary directory and
// returns its path. The name decides the detected format, so every case can
// pick its own extension.
func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("写入测试文件 %s 失败: %v", name, err)
	}
	return path
}

// solidImage builds a width×height image whose pixels run from left to right
// through the supplied colours, so a rotation or flip is visible in the result.
func solidImage(width, height int, colors ...color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, colors[x%len(colors)])
		}
	}
	return img
}

// halfImage builds an image whose left half is one colour and its right half
// another, wide enough that JPEG chroma subsampling keeps the two apart.
func halfImage(width, height int, left, right color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x < width/2 {
				img.Set(x, y, left)
				continue
			}
			img.Set(x, y, right)
		}
	}
	return img
}

// encode runs one image encoder and returns the bytes, failing the test when
// the encoder refuses the fixture.
func encode(t *testing.T, write func(*bytes.Buffer) error) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		t.Fatalf("编码测试图片失败: %v", err)
	}
	return buf.Bytes()
}

var (
	red  = color.RGBA{R: 220, G: 30, B: 40, A: 255}
	blue = color.RGBA{R: 20, G: 60, B: 220, A: 255}
)

// TestGenerateThumbnailsImages covers the image path: every format the package
// registers a decoder for produces a PNG payload with the source's dimensions,
// because a small image is not enlarged.
func TestGenerateThumbnailsImages(t *testing.T) {
	small := solidImage(4, 3, red, blue)
	cases := []struct {
		name string
		file string
		data []byte
	}{
		{"png", "photo.png", encode(t, func(b *bytes.Buffer) error { return png.Encode(b, small) })},
		{"jpeg", "photo.jpeg", encode(t, func(b *bytes.Buffer) error { return jpeg.Encode(b, small, nil) })},
		{"jpg", "photo.JPG", encode(t, func(b *bytes.Buffer) error { return jpeg.Encode(b, small, nil) })},
		{"gif", "photo.gif", encode(t, func(b *bytes.Buffer) error { return gif.Encode(b, small, nil) })},
		{"bmp", "photo.bmp", encode(t, func(b *bytes.Buffer) error { return bmp.Encode(b, small) })},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			preview := Generate(writeFile(t, tc.file, tc.data))
			if preview == nil || preview.Image == nil {
				t.Fatalf("%s 应生成图片预览", tc.name)
			}
			if preview.Text != "" {
				t.Errorf("图片预览不该带文本摘录，实际 %q", preview.Text)
			}
			if preview.Image.Width != 4 || preview.Image.Height != 3 {
				t.Errorf("缩略图尺寸 = %dx%d，期望 4x3（小图不放大）", preview.Image.Width, preview.Image.Height)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(preview.Image.PNG))
			if err != nil || format != "png" {
				t.Fatalf("缩略图载荷应是可解码的 PNG，实际 format=%q err=%v", format, err)
			}
			if cfg.Width != 4 || cfg.Height != 3 {
				t.Errorf("PNG 载荷尺寸 = %dx%d，期望 4x3", cfg.Width, cfg.Height)
			}
		})
	}
}

// TestGenerateDownscalesToTheLongEdge checks the scale rule: a big image is
// shrunk so its longer edge is the thumbnail bound and its aspect ratio holds.
func TestGenerateDownscalesToTheLongEdge(t *testing.T) {
	big := solidImage(2000, 1000, red, blue)
	path := writeFile(t, "wide.png", encode(t, func(b *bytes.Buffer) error { return png.Encode(b, big) }))
	preview := Generate(path)
	if preview == nil || preview.Image == nil {
		t.Fatal("大图应生成图片预览")
	}
	if preview.Image.Width != thumbLongEdge || preview.Image.Height != thumbLongEdge/2 {
		t.Errorf("缩略图尺寸 = %dx%d，期望 %dx%d",
			preview.Image.Width, preview.Image.Height, thumbLongEdge, thumbLongEdge/2)
	}

	tall := solidImage(1000, 2000, red, blue)
	path = writeFile(t, "tall.png", encode(t, func(b *bytes.Buffer) error { return png.Encode(b, tall) }))
	preview = Generate(path)
	if preview == nil || preview.Image == nil {
		t.Fatal("竖图应生成图片预览")
	}
	if preview.Image.Width != thumbLongEdge/2 || preview.Image.Height != thumbLongEdge {
		t.Errorf("竖图缩略图尺寸 = %dx%d，期望 %dx%d",
			preview.Image.Width, preview.Image.Height, thumbLongEdge/2, thumbLongEdge)
	}
}

// TestGenerateSkipsHugeImages checks the pixel bound: an image whose header
// announces more pixels than the package is willing to decode gets no preview,
// so one pasted monster cannot stall the capture thread.
func TestGenerateSkipsHugeImages(t *testing.T) {
	huge := solidImage(8, 8, red)
	data := encode(t, func(b *bytes.Buffer) error { return png.Encode(b, huge) })
	// Announce 9000×9000 in the header and fix its checksum, so the header is
	// the only part of the file the bound has to read.
	binary.BigEndian.PutUint32(data[16:], 9000)
	binary.BigEndian.PutUint32(data[20:], 9000)
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	if preview := Generate(writeFile(t, "huge.png", data)); preview != nil {
		t.Errorf("超过像素上限的图片不该生成预览，实际 %+v", preview)
	}
}

// TestGenerateAppliesJPEGOrientation checks the EXIF path: a phone photo whose
// pixels are stored sideways is rotated upright, so the preview matches what
// every other viewer shows.
func TestGenerateAppliesJPEGOrientation(t *testing.T) {
	sideways := halfImage(8, 4, red, blue)
	data := encode(t, func(b *bytes.Buffer) error { return jpeg.Encode(b, sideways, nil) })

	// Orientation 6 means "rotate 90° clockwise for display".
	upright := Generate(writeFile(t, "portrait.jpg", withOrientation(t, data, 6)))
	if upright == nil || upright.Image == nil {
		t.Fatal("带 EXIF 方向的 JPEG 应生成图片预览")
	}
	if upright.Image.Width != 4 || upright.Image.Height != 8 {
		t.Fatalf("旋正后的缩略图尺寸 = %dx%d，期望 4x8", upright.Image.Width, upright.Image.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(upright.Image.PNG))
	if err != nil {
		t.Fatalf("解码缩略图失败: %v", err)
	}
	if top := img.At(0, 0); !isReddish(top) {
		t.Errorf("旋正后顶部应是红色（原图左侧），实际 %v", top)
	}
	if bottom := img.At(0, 7); !isBluish(bottom) {
		t.Errorf("旋正后底部应是蓝色（原图右侧），实际 %v", bottom)
	}

	// The same bytes without the tag keep their stored orientation.
	plain := Generate(writeFile(t, "landscape.jpg", data))
	if plain == nil || plain.Image == nil {
		t.Fatal("无 EXIF 方向的 JPEG 应生成图片预览")
	}
	if plain.Image.Width != 8 || plain.Image.Height != 4 {
		t.Errorf("无方向标记时缩略图尺寸 = %dx%d，期望 8x4", plain.Image.Width, plain.Image.Height)
	}
}

// TestGenerateTextExcerpt covers the text path: the start of a text-like file
// becomes the preview, whatever the code or markup format.
func TestGenerateTextExcerpt(t *testing.T) {
	cases := []struct{ name, body string }{
		{"notes.txt", "第一行\n第二行\n"},
		{"README.md", "# 标题\n\n正文\n"},
		{"server.log", "2026-10-16 10:00:00 boot\n"},
		{"main.go", "package main\n"},
		{"package.json", "{\"name\": \"gopaste\"}\n"},
		{"rows.csv", "name,size\nnotes.txt,12\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			preview := Generate(writeFile(t, tc.name, []byte(tc.body)))
			if preview == nil {
				t.Fatalf("%s 应生成文本摘录", tc.name)
			}
			if preview.Image != nil {
				t.Errorf("文本预览不该带图片载荷")
			}
			if got, want := preview.Text, strings.TrimRight(tc.body, "\n"); got != want {
				t.Errorf("摘录 = %q，期望 %q", got, want)
			}
		})
	}
}

// TestTextExcerptLimits checks the bounds the excerpt keeps: a long file is cut
// to the line and rune limits with a marker, so one minified file cannot turn
// the preview into a wall of text.
func TestTextExcerptLimits(t *testing.T) {
	var manyLines strings.Builder
	for i := 0; i < 100; i++ {
		manyLines.WriteString("line\n")
	}
	preview := Generate(writeFile(t, "many.txt", []byte(manyLines.String())))
	if preview == nil {
		t.Fatal("多行文本应生成摘录")
	}
	if lines := strings.Count(preview.Text, "\n") + 1; lines != excerptLines {
		t.Errorf("摘录行数 = %d，期望 %d", lines, excerptLines)
	}
	if !strings.HasSuffix(preview.Text, ellipsis) {
		t.Errorf("截断的摘录应以 %q 结尾，实际 %q", ellipsis, preview.Text[len(preview.Text)-10:])
	}

	long := strings.Repeat("字", excerptRunes*2)
	preview = Generate(writeFile(t, "long.md", []byte(long)))
	if preview == nil {
		t.Fatal("长行文本应生成摘录")
	}
	if got := len([]rune(strings.TrimSuffix(preview.Text, ellipsis))); got != excerptRunes {
		t.Errorf("摘录 rune 数 = %d，期望 %d", got, excerptRunes)
	}
}

// TestTextExcerptSurvivesATruncatedRune checks the byte bound: cutting the
// prefix inside a multi-byte character must not make a Chinese file read as
// binary. The half character is dropped, the rest of the excerpt survives.
func TestTextExcerptSurvivesATruncatedRune(t *testing.T) {
	// Two bytes of a three-byte character sit exactly at the read bound.
	body := strings.Repeat("a", excerptBytes-2) + "中" + "\n尾行"
	preview := Generate(writeFile(t, "cut.txt", []byte(body)))
	if preview == nil {
		t.Fatal("截断处落在多字节字符中间时仍应生成摘录")
	}
	if strings.ContainsRune(preview.Text, '中') {
		t.Errorf("被截断的半个字符不该出现在摘录里，实际尾部 %q", preview.Text[len(preview.Text)-6:])
	}
	if !strings.HasPrefix(preview.Text, "aaa") {
		t.Errorf("摘录应保留截断点之前的内容，实际 %q", preview.Text[:10])
	}
}

// TestGenerateRejectsWhatItCannotPreview covers the fallback path: nothing
// unreadable, binary, or unsupported produces a preview, so the interface
// keeps its "no preview" note instead of drawing an empty card.
func TestGenerateRejectsWhatItCannotPreview(t *testing.T) {
	binaryText := []byte("text\x00with\x00nul")
	invalidUTF8 := []byte{0xff, 0xfe, 0x41, 0x42}
	cases := []struct {
		name string
		path string
	}{
		{"二进制内容伪装成文本", writeFile(t, "nul.txt", binaryText)},
		{"非法 UTF-8", writeFile(t, "broken.md", invalidUTF8)},
		{"空文件", writeFile(t, "empty.txt", nil)},
		{"只有空白", writeFile(t, "blank.md", []byte("   \n\n\t\n"))},
		{"不支持预览的格式", writeFile(t, "release.zip", []byte("PK\x03\x04"))},
		{"扩展名与内容不符", writeFile(t, "fake.png", []byte("not an image"))},
		{"不存在的文件", filepath.Join(t.TempDir(), "missing.png")},
		{"目录", t.TempDir()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if preview := Generate(tc.path); preview != nil {
				t.Errorf("%s 不该生成预览，实际 %+v", tc.name, preview)
			}
		})
	}
}

// withOrientation inserts an EXIF APP1 segment carrying orientation into a
// JPEG payload, the way a camera writes it.
func withOrientation(t *testing.T, jpegData []byte, orientation uint16) []byte {
	t.Helper()
	if len(jpegData) < 2 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 {
		t.Fatal("测试载荷不是 JPEG")
	}
	// Big-endian TIFF header, one IFD entry: tag 0x0112 (orientation), type 3
	// (SHORT), count 1, value orientation, then "no next IFD".
	tiff := make([]byte, 0, 26)
	tiff = append(tiff, 'M', 'M', 0x00, 0x2A, 0x00, 0x00, 0x00, 0x08)
	tiff = append(tiff, 0x00, 0x01)
	tiff = append(tiff, 0x01, 0x12, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01)
	tiff = append(tiff, byte(orientation>>8), byte(orientation), 0x00, 0x00)
	tiff = append(tiff, 0x00, 0x00, 0x00, 0x00)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1, 0x00, 0x00}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	segment = append(segment, payload...)

	out := make([]byte, 0, len(jpegData)+len(segment))
	out = append(out, jpegData[:2]...)
	out = append(out, segment...)
	return append(out, jpegData[2:]...)
}

// isReddish and isBluish report the dominant channel, which survives JPEG
// compression far better than an exact value would.
func isReddish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > g+0x4000 && r > b+0x4000
}

func isBluish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return b > r+0x4000 && b > g+0x4000
}
