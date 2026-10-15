package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"gopaste/internal/filetype"
	"gopaste/internal/history"

	"github.com/egoist/mygo/ui"
)

// fileTestPNG builds a tiny PNG payload for the image-preview tests. The
// payload is real PNG, so the same decode path the application uses is
// exercised rather than a stub.
func fileTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码测试图片失败: %v", err)
	}
	return buf.Bytes()
}

// fileItem is a file history item with a fixed timestamp, so its rendered
// labels do not depend on the machine's clock or time zone.
func fileItem(id, path string, size int64) history.Item {
	return history.Item{
		ID:        id,
		Kind:      kindFile,
		Text:      path,
		FileSize:  size,
		CreatedAt: time.Date(2020, 3, 4, 5, 6, 7, 0, time.Local),
	}
}

// fileApp renders one file item on the history page with no paired devices, so
// the frame holds only the list row and the preview.
func fileApp(item history.Item) *application {
	app := newApplication()
	app.history = history.New([]history.Item{item})
	app.pairedDevices = nil
	return app
}

// themeFor runs one frame and returns the theme it settled on, for the helpers
// that need a theme outside a render callback.
func themeFor(t *testing.T) *ui.Theme {
	t.Helper()
	var theme *ui.Theme
	ui.NewTester(func(c *ui.Context) {
		theme = c.Theme()
		ui.Text(c, "x")
	}, defaultWindowWidth, 720)
	if theme == nil {
		t.Fatal("测试帧没有产出主题")
	}
	return theme
}

// TestRowIconFollowsTheFileType checks the list row's icon and colour: a file
// item is drawn from its detected type, while every other kind keeps the
// content-kind icon it had.
func TestRowIconFollowsTheFileType(t *testing.T) {
	app := newApplication()
	theme := themeFor(t)

	cases := []struct {
		name string
		path string
		want filetype.Kind
	}{
		{"PDF 文档", "~/Documents/manual.pdf", filetype.KindPDF},
		{"图片文件", "~/Pictures/photo.png", filetype.KindImage},
		{"压缩包", "~/Desktop/design-export.zip", filetype.KindArchive},
		{"表格", "~/Downloads/report.xlsx", filetype.KindSpreadsheet},
		{"未知扩展名", "~/Downloads/thing.qqq", filetype.KindUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := fileItem("file", tc.path, 1024)
			gotColor, gotIcon := app.rowIcon(theme, item)
			wantColor := colorForFileType(theme, tc.want)
			wantIcon := iconForFileType(tc.want)
			if gotIcon != wantIcon {
				t.Errorf("%s 的行图标不是该文件类型的图标", tc.name)
			}
			if gotColor != wantColor {
				t.Errorf("%s 的行图标颜色 = %v，期望 %v", tc.name, gotColor, wantColor)
			}
		})
	}

	// A non-file kind still uses its content-kind icon.
	linkColor, linkIcon := app.rowIcon(theme, history.Item{ID: "l", Kind: kindLink, Text: "https://example.com"})
	if linkIcon != iconForKind(kindLink) || linkColor != colorForKind(theme, kindLink) {
		t.Errorf("链接行不应走文件类型图标")
	}
}

// TestFilePreviewNamesTheFileAndItsType checks the preview's naming card and
// the type tag in the meta row: a reader should see the file's own name, its
// detected type, its extension and its size without reading the full path.
func TestFilePreviewNamesTheFileAndItsType(t *testing.T) {
	app := fileApp(fileItem("file-report", "~/Documents/quarterly report.docx", 3482))
	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	for _, want := range []string{
		"quarterly report.docx",
		app.t(msgFileTypeDocument),
		"DOCX",
		"3.4 KB",
	} {
		if !tt.HasText(want) {
			t.Errorf("文件预览缺少 %q，实际显示：%q", want, tt.Texts())
		}
	}
}

// TestFilePreviewRendersAnImagePayload checks the image path: a file that
// carries an image preview decodes and draws it instead of the empty note.
func TestFilePreviewRendersAnImagePayload(t *testing.T) {
	item := fileItem("file-photo", "~/Pictures/photo.png", 4096)
	item.FilePreview = &history.FilePreview{Image: &history.Image{PNG: fileTestPNG(t), Width: 4, Height: 3}}
	app := fileApp(item)

	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	texts := tt.Texts()
	if !hasExactText(texts, "photo.png") {
		t.Errorf("文件预览缺少文件名，实际显示：%q", texts)
	}
	if !hasExactText(texts, "PNG · 4.0 KB") {
		t.Errorf("文件预览缺少类型与大小，实际显示：%q", texts)
	}
	if tt.HasText(app.t(msgHistoryFilePreviewGone)) {
		t.Errorf("带图片预览的文件不应显示“无预览”提示，实际显示：%q", texts)
	}
	if bitmap := app.filePreviewBitmap(item); bitmap == nil {
		t.Errorf("图片文件应解码出预览位图")
	}
}

// TestFilePreviewRendersATextExcerpt checks the text path: a file that carries
// a text excerpt shows it as plain, selectable text.
func TestFilePreviewRendersATextExcerpt(t *testing.T) {
	item := fileItem("file-notes", "~/Documents/release-notes.md", 512)
	item.FilePreview = &history.FilePreview{Text: "第一行摘录"}
	app := fileApp(item)

	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	if !tt.HasText("第一行摘录") {
		t.Errorf("文件预览缺少文本摘录，实际显示：%q", tt.Texts())
	}
	if tt.HasText(app.t(msgHistoryFilePreviewGone)) {
		t.Errorf("带文本摘录的文件不应显示“无预览”提示，实际显示：%q", tt.Texts())
	}
}

// TestFilePreviewFallsBackToANote checks the empty path: a file with no
// payload still names itself and says why there is nothing to draw.
func TestFilePreviewFallsBackToANote(t *testing.T) {
	app := fileApp(fileItem("file-export", "~/Desktop/design-export.zip", 18_874_368))
	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	for _, want := range []string{"design-export.zip", app.t(msgFileTypeArchive), app.t(msgHistoryFilePreviewGone)} {
		if !tt.HasText(want) {
			t.Errorf("文件预览缺少 %q，实际显示：%q", want, tt.Texts())
		}
	}
}

// TestFilePreviewNamesTheFirstOfSeveralFiles checks a multi-file copy: the
// preview names the first file and reports how many were copied.
func TestFilePreviewNamesTheFirstOfSeveralFiles(t *testing.T) {
	item := fileItem("file-many", "~/Desktop/first.png\n~/Desktop/second.png", 2048)
	app := fileApp(item)

	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	texts := tt.Texts()
	if !hasExactText(texts, "first.png") {
		t.Errorf("多文件预览应显示第一个文件名，实际显示：%q", texts)
	}
	if !tt.HasText(app.l10n().fileCount(2)) {
		t.Errorf("多文件预览应显示文件数量，实际显示：%q", texts)
	}
	if hasExactText(texts, "second.png") {
		t.Errorf("多文件预览不应把第二个文件当成主文件名，实际显示：%q", texts)
	}
}

// TestFileTypeLabelFallsBackToTheExtension checks the tag shown for a file
// whose format is not recognised: the extension is better than nothing, and a
// name with no extension has no tag at all.
func TestFileTypeLabelFallsBackToTheExtension(t *testing.T) {
	app := newApplication()
	if got, want := app.fileTypeLabel(fileItem("f", "~/data/thing.qqq", 10)), "QQQ"; got != want {
		t.Errorf("未知扩展名的类型标签 = %q，期望 %q", got, want)
	}
	if got := app.fileTypeLabel(fileItem("f", "~/data/no-extension", 10)); got != "" {
		t.Errorf("没有扩展名时不应有类型标签，实际 %q", got)
	}
	if got, want := app.fileTypeLabel(fileItem("f", "~/Documents/report.docx", 10)), app.t(msgFileTypeDocument); got != want {
		t.Errorf("文档类型标签 = %q，期望 %q", got, want)
	}
}
