package filetype

import "testing"

// TestDetectClassifiesByExtension covers the formats that have an icon of their
// own, plus the case and path handling Detect promises.
func TestDetectClassifiesByExtension(t *testing.T) {
	cases := []struct {
		name string
		want Format
	}{
		{"notes.txt", FormatText},
		{"README.md", FormatMarkdown},
		{"changes.markdown", FormatMarkdown},
		{"component.mdx", FormatMarkdown},
		{"server.log", FormatLog},
		{"manual.rtf", FormatRTF},
		{"report.docx", FormatDocument},
		{"budget.xlsx", FormatXLS},
		{"sheet.ods", FormatSpreadsheet},
		{"export.csv", FormatCSV},
		{"quarterly.pptx", FormatPresentation},
		{"manual.pdf", FormatPDF},
		{"screenshot.png", FormatPNG},
		{"holiday.JPG", FormatJPEG},
		{"logo.bmp", FormatBMP},
		{"icon.svg", FormatSVG},
		{"animation.gif", FormatImage},
		{"clip.mov", FormatVideo},
		{"voice-memo.m4a", FormatAudio},
		{"release.zip", FormatArchive},
		{"main.go", FormatGo},
		{"app.py", FormatPython},
		{"index.mjs", FormatJavaScript},
		{"App.tsx", FormatTypeScript},
		{"Widget.vue", FormatVue},
		{"App.svelte", FormatSvelte},
		{"lib.rs", FormatRust},
		{"index.php", FormatPHP},
		{"deploy.sh", FormatShell},
		{"query.sql", FormatSQL},
		{"page.html", FormatHTML},
		{"theme.css", FormatCSS},
		{"theme.scss", FormatSass},
		{"feed.xml", FormatXML},
		{"package.json", FormatJSON},
		{"ci.yml", FormatYAML},
		{"Cargo.toml", FormatConfig},
		{"Dockerfile", FormatDockerfile},
		{"App.java", FormatCode},
		{"Inter.ttf", FormatFont},
		{"history.sqlite3", FormatDatabase},
		{"mockup.fig", FormatFigma},
		{"scene.blend", FormatBlender},
		{"mockup.psd", FormatDesign},
		{"installer.dmg", FormatArchive},
		{"setup.exe", FormatExecutable},
		{"unknown.qqq", FormatUnknown},
		{"no-extension", FormatUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Detect(tc.name); got != tc.want {
				t.Errorf("Detect(%q) = %v，期望 %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestFormatKindGroupsFormats checks the coarse family each format reports, so
// the label and colour vocabulary stays independent of the icon table.
func TestFormatKindGroupsFormats(t *testing.T) {
	cases := []struct {
		format Format
		want   Kind
	}{
		{FormatText, KindText},
		{FormatMarkdown, KindText},
		{FormatLog, KindText},
		{FormatRTF, KindDocument},
		{FormatDocument, KindDocument},
		{FormatSpreadsheet, KindSpreadsheet},
		{FormatXLS, KindSpreadsheet},
		{FormatCSV, KindSpreadsheet},
		{FormatPresentation, KindPresentation},
		{FormatPDF, KindPDF},
		{FormatPNG, KindImage},
		{FormatJPEG, KindImage},
		{FormatBMP, KindImage},
		{FormatSVG, KindImage},
		{FormatImage, KindImage},
		{FormatVideo, KindVideo},
		{FormatAudio, KindAudio},
		{FormatArchive, KindArchive},
		{FormatGo, KindCode},
		{FormatPython, KindCode},
		{FormatJavaScript, KindCode},
		{FormatTypeScript, KindCode},
		{FormatVue, KindCode},
		{FormatSvelte, KindCode},
		{FormatRust, KindCode},
		{FormatPHP, KindCode},
		{FormatShell, KindCode},
		{FormatSQL, KindCode},
		{FormatHTML, KindCode},
		{FormatCSS, KindCode},
		{FormatSass, KindCode},
		{FormatXML, KindCode},
		{FormatJSON, KindCode},
		{FormatYAML, KindCode},
		{FormatConfig, KindCode},
		{FormatDockerfile, KindCode},
		{FormatCode, KindCode},
		{FormatFont, KindFont},
		{FormatDatabase, KindDatabase},
		{FormatFigma, KindDesign},
		{FormatBlender, KindDesign},
		{FormatDesign, KindDesign},
		{FormatExecutable, KindExecutable},
		{FormatUnknown, KindUnknown},
	}
	for _, tc := range cases {
		if got := tc.format.Kind(); got != tc.want {
			t.Errorf("%v.Kind() = %v，期望 %v", tc.format, got, tc.want)
		}
	}
}

// TestDetectUsesOnlyThePathTail checks that a full path is classified by its
// last element, whichever separator the path uses.
func TestDetectUsesOnlyThePathTail(t *testing.T) {
	cases := []struct {
		name string
		want Format
	}{
		{"~/Documents/release-notes.md", FormatMarkdown},
		{"/tmp/dir.with.dots/photo.png", FormatPNG},
		{`C:\Users\me\Desktop\report.pdf`, FormatPDF},
		{"/tmp/archive.tar.gz", FormatArchive},
		{"/tmp/trailing/", FormatUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Detect(tc.name); got != tc.want {
				t.Errorf("Detect(%q) = %v，期望 %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestExtension covers the extension reader on its own, including the hidden
// file and trailing-dot cases that must not read as an extension.
func TestExtension(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"photo.PNG", "png"},
		{"archive.tar.gz", "gz"},
		{"/tmp/Report.PDF", "pdf"},
		{".gitignore", ""},
		{"trailing.", ""},
		{"no-extension", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Extension(tc.name); got != tc.want {
				t.Errorf("Extension(%q) = %q，期望 %q", tc.name, got, tc.want)
			}
		})
	}
}

// TestBaseName covers the path tail on both separator styles.
func TestBaseName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"~/Documents/file.md", "file.md"},
		{`C:\Users\me\file.md`, "file.md"},
		{"/tmp/trailing/", "trailing"},
		{"  /tmp/spaced.pdf  ", "spaced.pdf"},
		{"bare.txt", "bare.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := BaseName(tc.name); got != tc.want {
				t.Errorf("BaseName(%q) = %q，期望 %q", tc.name, got, tc.want)
			}
		})
	}
}
