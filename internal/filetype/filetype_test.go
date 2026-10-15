package filetype

import "testing"

// TestDetectClassifiesByExtension covers each category with a representative
// name, plus the case and path handling Detect promises.
func TestDetectClassifiesByExtension(t *testing.T) {
	cases := []struct {
		name string
		want Kind
	}{
		{"notes.txt", KindText},
		{"README.md", KindText},
		{"report.docx", KindDocument},
		{"budget.xlsx", KindSpreadsheet},
		{"quarterly.pptx", KindPresentation},
		{"manual.pdf", KindPDF},
		{"screenshot.png", KindImage},
		{"clip.mov", KindVideo},
		{"voice-memo.m4a", KindAudio},
		{"release.zip", KindArchive},
		{"main.go", KindCode},
		{"App.tsx", KindCode},
		{"Inter.ttf", KindFont},
		{"history.sqlite3", KindDatabase},
		{"mockup.fig", KindDesign},
		{"installer.dmg", KindArchive},
		{"setup.exe", KindExecutable},
		{"Dockerfile", KindCode},
		{"unknown.qqq", KindUnknown},
		{"no-extension", KindUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Detect(tc.name); got != tc.want {
				t.Errorf("Detect(%q) = %v，期望 %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestDetectUsesOnlyThePathTail checks that a full path is classified by its
// last element, whichever separator the path uses.
func TestDetectUsesOnlyThePathTail(t *testing.T) {
	cases := []struct {
		name string
		want Kind
	}{
		{"~/Documents/release-notes.md", KindText},
		{"/tmp/dir.with.dots/photo.png", KindImage},
		{`C:\Users\me\Desktop\report.pdf`, KindPDF},
		{"/tmp/archive.tar.gz", KindArchive},
		{"/tmp/trailing/", KindUnknown},
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
