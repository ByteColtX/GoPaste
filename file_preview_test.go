package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopaste/internal/capture"
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

// TestRowIconFollowsTheFileFormat checks the list row's icon and colour: a file
// item is drawn from its detected format, while every other kind keeps the
// content-kind icon it had.
func TestRowIconFollowsTheFileFormat(t *testing.T) {
	app := newApplication()
	theme := themeFor(t)

	cases := []struct {
		name   string
		path   string
		format filetype.Format
		kind   filetype.Kind
	}{
		{"PDF 文档", "~/Documents/manual.pdf", filetype.FormatPDF, filetype.KindPDF},
		{"Markdown", "~/Documents/notes.md", filetype.FormatMarkdown, filetype.KindText},
		{"Go 源码", "~/code/main.go", filetype.FormatGo, filetype.KindCode},
		{"PNG 图片", "~/Pictures/photo.png", filetype.FormatPNG, filetype.KindImage},
		{"压缩包", "~/Desktop/design-export.zip", filetype.FormatArchive, filetype.KindArchive},
		{"Excel 表格", "~/Downloads/report.xlsx", filetype.FormatXLS, filetype.KindSpreadsheet},
		{"未知扩展名", "~/Downloads/thing.qqq", filetype.FormatUnknown, filetype.KindUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := fileItem("file", tc.path, 1024)
			gotColor, gotIcon := app.rowIcon(theme, item)
			wantColor := colorForFileKind(theme, tc.kind)
			wantIcon := iconForFormat(tc.format)
			if gotIcon != wantIcon {
				t.Errorf("%s 的行图标不是该文件格式的图标", tc.name)
			}
			if gotColor != wantColor {
				t.Errorf("%s 的行图标颜色 = %v，期望 %v", tc.name, gotColor, wantColor)
			}
		})
	}

	// A non-file kind still uses its content-kind icon.
	linkColor, linkIcon := app.rowIcon(theme, history.Item{ID: "l", Kind: kindLink, Text: "https://example.com"})
	if linkIcon != iconForKind(kindLink) || linkColor != colorForKind(theme, kindLink) {
		t.Errorf("链接行不应走文件格式图标")
	}
}

// TestMarkdownHasItsOwnIcon pins the case the format table exists for: a
// Markdown file must not fall back to the generic text or file icon.
func TestMarkdownHasItsOwnIcon(t *testing.T) {
	markdown := iconForFormat(filetype.FormatMarkdown)
	if markdown == iconForFormat(filetype.FormatText) {
		t.Error("Markdown 应有独立图标，而不是回退到文本图标")
	}
	if markdown == iconForFormat(filetype.FormatUnknown) {
		t.Error("Markdown 不应回退到通用文件图标")
	}
}

// TestFormatsHaveDistinctIcons checks that every format the table calls out has
// an icon of its own: two formats silently sharing one icon would leave the
// list unable to tell them apart.
func TestFormatsHaveDistinctIcons(t *testing.T) {
	formats := []filetype.Format{
		filetype.FormatText, filetype.FormatMarkdown,
		filetype.FormatDocument, filetype.FormatSpreadsheet, filetype.FormatXLS,
		filetype.FormatCSV, filetype.FormatPresentation, filetype.FormatPDF,
		filetype.FormatImage, filetype.FormatPNG, filetype.FormatJPEG,
		filetype.FormatBMP, filetype.FormatSVG,
		filetype.FormatVideo, filetype.FormatAudio, filetype.FormatArchive,
		filetype.FormatCode, filetype.FormatGo, filetype.FormatPython,
		filetype.FormatJavaScript, filetype.FormatTypeScript, filetype.FormatVue,
		filetype.FormatSvelte, filetype.FormatRust, filetype.FormatPHP,
		filetype.FormatShell, filetype.FormatSQL, filetype.FormatHTML,
		filetype.FormatCSS, filetype.FormatSass, filetype.FormatXML,
		filetype.FormatJSON, filetype.FormatYAML, filetype.FormatConfig,
		filetype.FormatDockerfile, filetype.FormatFont, filetype.FormatDatabase,
		filetype.FormatDesign, filetype.FormatFigma, filetype.FormatBlender,
		filetype.FormatExecutable,
	}
	seen := make(map[*ui.SVG]filetype.Format, len(formats))
	for _, format := range formats {
		icon := iconForFormat(format)
		if icon == nil {
			t.Fatalf("%v 没有图标", format)
		}
		if other, ok := seen[icon]; ok {
			t.Errorf("%v 与 %v 共用了同一个图标", format, other)
		}
		seen[icon] = format
	}
}

// TestFilePreviewNamesTheFileAndItsFormat checks that the preview carries the
// file's identity without a separate naming card: the path ends in the file's
// own name, and the header names the detected format and the size.
func TestFilePreviewNamesTheFileAndItsFormat(t *testing.T) {
	app := fileApp(fileItem("file-report", "~/Documents/quarterly report.docx", 3482))
	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	for _, want := range []string{
		"~/Documents/quarterly report.docx",
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
	if !hasExactText(texts, "~/Pictures/photo.png") {
		t.Errorf("文件预览缺少路径，实际显示：%q", texts)
	}
	if !tt.HasText("PNG") || !tt.HasText("4.0 KB") {
		t.Errorf("文件预览缺少格式与大小，实际显示：%q", texts)
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
	for _, want := range []string{"design-export.zip", "ZIP", app.t(msgHistoryFilePreviewGone)} {
		if !tt.HasText(want) {
			t.Errorf("文件预览缺少 %q，实际显示：%q", want, tt.Texts())
		}
	}
}

// capturedFile runs one file copy through the capture path the way the running
// application does: the snapshot becomes an item, and the preview is generated
// from the file on disk.
func capturedFile(t *testing.T, paths ...string) history.Item {
	t.Helper()
	item, ok := (capture.Snapshot{Kind: kindFile, Files: paths}).Item()
	if !ok {
		t.Fatalf("捕获 %v 没有产出条目", paths)
	}
	return withFilePreview(item)
}

// writeTestFile writes body into a temporary file with the supplied name, so
// the preview path sees a real file with a real extension.
func writeTestFile(t *testing.T, name string, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("写入 %s 失败: %v", name, err)
	}
	return path
}

// TestCapturedFileGetsItsPreview covers the seam the bug was in: a captured
// file item carries the preview the capture path generated, so the preview
// pane has something to draw instead of the "no preview" note.
func TestCapturedFileGetsItsPreview(t *testing.T) {
	imageItem := capturedFile(t, writeTestFile(t, "screenshot.png", fileTestPNG(t)))
	if imageItem.FilePreview == nil || imageItem.FilePreview.Image == nil {
		t.Fatalf("复制的图片文件应带图片预览，实际 %#v", imageItem.FilePreview)
	}
	if imageItem.FilePreview.Image.Width != 4 || imageItem.FilePreview.Image.Height != 3 {
		t.Errorf("缩略图尺寸 = %dx%d，期望 4x3",
			imageItem.FilePreview.Image.Width, imageItem.FilePreview.Image.Height)
	}

	textItem := capturedFile(t, writeTestFile(t, "notes.md", []byte("# 标题\n\n正文\n")))
	if textItem.FilePreview == nil || !strings.Contains(textItem.FilePreview.Text, "# 标题") {
		t.Fatalf("复制的 Markdown 文件应带文本摘录，实际 %#v", textItem.FilePreview)
	}

	// A format with nothing to preview keeps no payload, and a copy of several
	// files previews the first one — the same file the list row names.
	if zipItem := capturedFile(t, writeTestFile(t, "release.zip", []byte("PK\x03\x04"))); zipItem.FilePreview != nil {
		t.Errorf("压缩包不该有预览，实际 %#v", zipItem.FilePreview)
	}
	many := capturedFile(t,
		writeTestFile(t, "first.txt", []byte("第一个文件\n")),
		writeTestFile(t, "second.png", fileTestPNG(t)),
	)
	if many.FilePreview == nil || !strings.Contains(many.FilePreview.Text, "第一个文件") {
		t.Errorf("多文件复制应只预览第一个文件，实际 %#v", many.FilePreview)
	}
}

// TestCapturedPreviewReachesThePreviewPane checks the whole path the user sees:
// the captured item renders its picture or excerpt rather than the "no
// preview" note.
func TestCapturedPreviewReachesThePreviewPane(t *testing.T) {
	imageItem := capturedFile(t, writeTestFile(t, "screenshot.png", fileTestPNG(t)))
	imageApp := fileApp(imageItem)
	tt := ui.NewTester(imageApp.view, defaultWindowWidth, 720)
	if tt.HasText(imageApp.t(msgHistoryFilePreviewGone)) {
		t.Errorf("带缩略图的图片文件不该显示「无预览」提示，实际显示：%q", tt.Texts())
	}
	if bitmap := imageApp.filePreviewBitmap(imageItem); bitmap == nil {
		t.Errorf("图片文件的缩略图应能解码成位图")
	}

	textItem := capturedFile(t, writeTestFile(t, "notes.md", []byte("# 标题\n\n正文\n")))
	app := fileApp(textItem)
	tt = ui.NewTester(app.view, defaultWindowWidth, 720)
	if !tt.HasText("# 标题") {
		t.Errorf("文本文件的摘录应显示在预览区，实际显示：%q", tt.Texts())
	}
	if tt.HasText(app.t(msgHistoryFilePreviewGone)) {
		t.Errorf("带摘录的文本文件不该显示「无预览」提示，实际显示：%q", tt.Texts())
	}
}

// TestOtherKindsKeepTheirOwnPayload checks that the preview step only touches
// file items: a text copy has no file payload to preview and must not gain one.
func TestOtherKindsKeepTheirOwnPayload(t *testing.T) {
	text := history.Item{ID: "text-1", Kind: kindText, Text: "hello"}
	if got := withFilePreview(text); got.FilePreview != nil {
		t.Errorf("文本条目不该带文件预览，实际 %#v", got.FilePreview)
	}
}

// TestTruncatePathMiddleKeepsBothEnds checks the shortening the clickable path
// rows use: whole directory segments are dropped from the middle, so the root
// and the file name both survive, and a path that already fits is left alone.
func TestTruncatePathMiddleKeepsBothEnds(t *testing.T) {
	const path = "~/Documents/GoPaste/design/reference/archive/quarterly-report.pdf"
	var shortened, untouched, squeezed string
	view := func(c *ui.Context) {
		size := font(c.Theme(), fontCaption)
		mono := func(value string) float32 {
			width, _ := c.MeasureText(0, ui.Span{Text: value, Font: "monospace", Size: size})
			return width
		}
		shortened = truncatePathMiddle(c, path, size, mono(path)-20)
		untouched = truncatePathMiddle(c, path, size, mono(path)+1)
		squeezed = truncatePathMiddle(c, path, size, mono("…/quarterly-report.pdf")-20)
		ui.Text(c, "x")
	}
	ui.NewTester(view, defaultWindowWidth, 720)

	if !strings.HasPrefix(shortened, "~/Documents/") {
		t.Errorf("省略后应保留根路径，实际 %q", shortened)
	}
	if !strings.HasSuffix(shortened, "/quarterly-report.pdf") {
		t.Errorf("省略后应保留文件名，实际 %q", shortened)
	}
	if !strings.Contains(shortened, "…/") {
		t.Errorf("省略应以整段目录为单位，实际 %q", shortened)
	}
	if untouched != path {
		t.Errorf("放得下的路径不应改动，实际 %q", untouched)
	}
	if squeezed == path || !strings.Contains(squeezed, "…") {
		t.Errorf("连根路径都放不下时仍应省略，实际 %q", squeezed)
	}
}

// TestListSubtypeLabelNamesTheLanguageAndFormat checks the subtype a list row
// shows after its kind: a code item's language, a file item's format, and
// nothing when neither was recognised.
func TestListSubtypeLabelNamesTheLanguageAndFormat(t *testing.T) {
	app := newApplication()
	cases := []struct {
		name string
		item history.Item
		want string
	}{
		{"代码语言", history.Item{ID: "c", Kind: kindCode, Text: "package main\n\nfunc main() {}"}, "Go"},
		{"图片格式", fileItem("f", "~/Pictures/photo.png", 1), "PNG"},
		{"文档格式", fileItem("f", "~/Documents/report.docx", 1), "DOCX"},
		{"未知扩展名", fileItem("f", "~/data/thing.qqq", 1), "QQQ"},
		{"无扩展名", fileItem("f", "~/data/no-extension", 1), ""},
		{"文本没有子类型", history.Item{ID: "t", Kind: kindText, Text: "hello"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := app.listSubtypeLabel(tc.item); got != tc.want {
				t.Errorf("listSubtypeLabel = %q，期望 %q", got, tc.want)
			}
		})
	}
}

// TestHistoryRowShowsTheFileFormat checks the list row for a file: it names the
// detected format after the kind, the way a code row names its language. The
// preview shows a text item here, so the format below can only come from the
// file's own list row.
func TestHistoryRowShowsTheFileFormat(t *testing.T) {
	app := newApplication()
	app.history = history.New([]history.Item{
		{ID: "t", Kind: kindText, Text: "普通文本记录", CreatedAt: fileItem("x", "a.txt", 1).CreatedAt},
		fileItem("f", "~/Pictures/photo.png", 1024),
	})
	app.pairedDevices = nil

	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	if !hasExactText(tt.Texts(), "PNG") {
		t.Errorf("文件列表行应显示格式后缀 %q，实际显示：%q", "PNG", tt.Texts())
	}
}

// recordingClipboard is a history.ClipboardWriter that records what a preview
// action wrote, so a test can assert what copying a path produced.
type recordingClipboard struct {
	texts  []string
	images [][]byte
	files  [][]string
}

func (r *recordingClipboard) WriteText(text string) { r.texts = append(r.texts, text) }

func (r *recordingClipboard) WriteImage(png []byte) error {
	r.images = append(r.images, png)
	return nil
}

func (r *recordingClipboard) WriteFiles(paths ...string) error {
	r.files = append(r.files, append([]string(nil), paths...))
	return nil
}

// longFilePath is longer than the list's text limit, so only the preview can
// show it in full: the list row truncates it. That keeps the assertions below
// about the preview from passing on the list row's text alone.
const longFilePath = "~/Documents/GoPaste/design/reference/archive/quarterly-report-final-v3-2026-10-16-reviewed.docx"

// TestFilePreviewShowsTheFullPath checks that a file preview names the file and
// then shows its whole path, which is the payload a copied file actually
// carries.
func TestFilePreviewShowsTheFullPath(t *testing.T) {
	app := fileApp(fileItem("file-report", longFilePath, 3482))
	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	texts := tt.Texts()
	if !hasExactText(texts, longFilePath) {
		t.Errorf("文件预览应显示完整路径，实际显示：%q", texts)
	}
	if !hasExactText(texts, filetype.BaseName(longFilePath)) {
		t.Errorf("文件预览应显示文件名，实际显示：%q", texts)
	}
}

// TestHistoryListShowsOnlyTheFileName checks the list row for a file: it names
// the file itself, never the path, which the preview shows in full.
func TestHistoryListShowsOnlyTheFileName(t *testing.T) {
	app := newApplication()
	if got, want := app.historyListText(fileItem("file-report", longFilePath, 3482)), filetype.BaseName(longFilePath); got != want {
		t.Errorf("列表行标题 = %q，期望文件名 %q", got, want)
	}
	if got, want := app.historyListText(fileItem("file-many", "~/Desktop/first.png\n~/Desktop/second.png", 2048)), "first.png"; got != want {
		t.Errorf("多文件列表行标题 = %q，期望第一个文件名 %q", got, want)
	}
}

// TestFilePathClickCopiesThePath checks the click-to-copy affordance: clicking
// the path writes exactly that path to the clipboard.
func TestFilePathClickCopiesThePath(t *testing.T) {
	app := fileApp(fileItem("file-report", longFilePath, 3482))
	clipboard := &recordingClipboard{}
	app.clipboard = clipboard

	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	if err := tt.Click(longFilePath); err != nil {
		t.Fatalf("点击路径失败: %v", err)
	}
	if len(clipboard.texts) != 1 || clipboard.texts[0] != longFilePath {
		t.Errorf("点击路径后剪贴板内容 = %q，期望 [%q]", clipboard.texts, longFilePath)
	}
}

// TestFilePathClickCopiesTheClickedPath checks a multi-file copy: every path is
// its own clickable row, and clicking one copies that path alone.
func TestFilePathClickCopiesTheClickedPath(t *testing.T) {
	item := fileItem("file-many", "~/Desktop/first.png\n~/Desktop/second.png", 2048)
	app := fileApp(item)
	clipboard := &recordingClipboard{}
	app.clipboard = clipboard

	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	texts := tt.Texts()
	for _, path := range []string{"~/Desktop/first.png", "~/Desktop/second.png"} {
		if !hasExactText(texts, path) {
			t.Errorf("多文件预览应逐条显示路径 %q，实际显示：%q", path, texts)
		}
	}
	if err := tt.Click("~/Desktop/second.png"); err != nil {
		t.Fatalf("点击第二条路径失败: %v", err)
	}
	if len(clipboard.texts) != 1 || clipboard.texts[0] != "~/Desktop/second.png" {
		t.Errorf("点击第二条路径后剪贴板内容 = %q，期望 [%q]", clipboard.texts, "~/Desktop/second.png")
	}
}

// TestFilePathListSummarizesTheRest checks the bound: a copy with more paths
// than the preview lists shows the first ones and summarizes the remainder
// instead of building a row per path every frame.
func TestFilePathListSummarizesTheRest(t *testing.T) {
	paths := make([]string, 0, filePathRowsLimit+2)
	for index := 0; index < filePathRowsLimit+2; index++ {
		paths = append(paths, fmt.Sprintf("~/Desktop/file-%02d.png", index))
	}
	app := fileApp(fileItem("file-many", strings.Join(paths, "\n"), 4096))
	tt := ui.NewTester(app.view, defaultWindowWidth, 720)
	texts := tt.Texts()

	if !hasExactText(texts, paths[0]) || !hasExactText(texts, paths[filePathRowsLimit-1]) {
		t.Errorf("预览应列出前 %d 条路径，实际显示：%q", filePathRowsLimit, texts)
	}
	if hasExactText(texts, paths[filePathRowsLimit]) {
		t.Errorf("超出上限的路径不应逐条渲染，实际显示：%q", texts)
	}
	if !hasExactText(texts, app.tf(msgHistoryFileMoreFormat, 2)) {
		t.Errorf("预览应汇总剩余路径数量，实际显示：%q", texts)
	}
}

// TestFileTypeLabelNamesTheFormatNotATranslation checks the tag shown for a
// file: a format with a name of its own uses it, every other format uses the
// uppercased extension, and a name with neither has no tag. The label never
// goes through the catalog, so a .zip reads ZIP in every UI language instead of
// a translated family word.
func TestFileTypeLabelNamesTheFormatNotATranslation(t *testing.T) {
	app := newApplication()
	cases := []struct {
		path string
		want string
	}{
		{"~/Pictures/photo.png", "PNG"},
		{"~/Documents/notes.md", "Markdown"},
		{"~/code/main.go", "Go"},
		{"~/Desktop/design-export.zip", "ZIP"},
		{"~/Documents/report.docx", "DOCX"},
		{"~/Documents/notes.txt", "TXT"},
		{"~/data/table.tsv", "TSV"},
		{"~/config/settings.toml", "TOML"},
		{"~/logs/app.log", "LOG"},
		{"~/styles/theme.less", "LESS"},
		{"~/data/thing.qqq", "QQQ"},
		{"~/data/no-extension", ""},
	}
	for _, tc := range cases {
		if got := app.fileTypeLabel(fileItem("f", tc.path, 10)); got != tc.want {
			t.Errorf("%s 的类型标签 = %q，期望 %q", tc.path, got, tc.want)
		}
	}
}
