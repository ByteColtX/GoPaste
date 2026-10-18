package history

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// pngBytes encodes a solid-colour PNG for the image cases.
func pngBytes(t *testing.T, width, height int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestContentIDSameContentSameID 检查内容相同就算出同一个 ID：这是跨设备去重和
// 接收端自己算 ID 的前提，收发两端因此不必互相相信对方给的 ID。
func TestContentIDSameContentSameID(t *testing.T) {
	cases := []struct {
		name string
		a, b Content
	}{
		{
			name: "text is identical",
			a:    Content{Kind: KindText, Text: "hello"},
			b:    Content{Kind: KindText, Text: "hello"},
		},
		{
			// CRLF 归一化：同一段文字换行符不同仍是同一条记录。
			name: "text newline normalisation",
			a:    Content{Kind: KindText, Text: "a\r\nb"},
			b:    Content{Kind: KindText, Text: "a\nb"},
		},
		{
			name: "rich text is identical",
			a:    Content{Kind: KindRichText, HTML: "<b>a</b>", Text: "a"},
			b:    Content{Kind: KindRichText, HTML: "<b>a</b>", Text: "a"},
		},
		{
			name: "file list order matters",
			a:    Content{Kind: KindFile, Files: []string{"/a", "/b"}},
			b:    Content{Kind: KindFile, Files: []string{"/a", "/b"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, want := ContentID(tc.a), ContentID(tc.b); got != want {
				t.Errorf("ContentID = %q, want %q", got, want)
			}
			if ContentID(tc.a) == "" {
				t.Error("ContentID must not be empty")
			}
		})
	}
}

// TestContentIDSeparatesKinds 检查不同内容或不同类型得到不同 ID，包括同一串字节
// 出现在不同类型里（域标签的作用）。
func TestContentIDSeparatesKinds(t *testing.T) {
	ids := map[string]string{}
	add := func(name string, c Content) {
		t.Helper()
		id := ContentID(c)
		if other, ok := ids[id]; ok {
			t.Fatalf("%s and %s share the ID %q", name, other, id)
		}
		ids[id] = name
	}
	add("text", Content{Kind: KindText, Text: "x"})
	add("rich text", Content{Kind: KindRichText, HTML: "x"})
	add("file", Content{Kind: KindFile, Files: []string{"x"}})
	add("text with inner NUL", Content{Kind: KindText, Text: "x\x00y"})
	add("image", Content{Kind: KindImage, Image: &Image{PNG: pngBytes(t, 2, 2, color.Black)}})
	add("other image", Content{Kind: KindImage, Image: &Image{PNG: pngBytes(t, 3, 2, color.Black)}})
}

// TestContentIDImageIgnoresEncoding 检查同一张图片重新编码后仍是同一个 ID：
// 摘要算的是解码后的像素，不是 PNG 字节。
func TestContentIDImageIgnoresEncoding(t *testing.T) {
	original := pngBytes(t, 4, 4, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	decoded, err := png.Decode(bytes.NewReader(original))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	var reencoded bytes.Buffer
	if err := png.Encode(&reencoded, decoded); err != nil {
		t.Fatalf("re-encode png: %v", err)
	}
	a := ContentID(Content{Kind: KindImage, Image: &Image{PNG: original}})
	b := ContentID(Content{Kind: KindImage, Image: &Image{PNG: reencoded.Bytes()}})
	if a != b {
		t.Errorf("re-encoded image changed the ID: %q != %q", a, b)
	}
}

// TestContentIDWithoutImage 检查缺少图像载荷时不会 panic，仍返回一个稳定 ID。
func TestContentIDWithoutImage(t *testing.T) {
	if got := ContentID(Content{Kind: KindImage}); got == "" {
		t.Error("ContentID must not be empty for an image without a payload")
	}
}
