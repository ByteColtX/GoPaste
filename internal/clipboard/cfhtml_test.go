package clipboard

import (
	"fmt"
	"strings"
	"testing"
)

func TestCFHTMLFragmentPrefersFragmentOverDocument(t *testing.T) {
	document := "<html><body><p>hello</p></body></html>"
	payload := []byte(cfhtmlPayload(document,
		strings.Index(document, "<p>"),
		strings.Index(document, "</p>")+len("</p>")))

	fragment, ok := cfhtmlFragment(payload)
	if !ok {
		t.Fatal("cfhtmlFragment reported no fragment")
	}
	if string(fragment) != "<p>hello</p>" {
		t.Fatalf("fragment = %q, want %q", fragment, "<p>hello</p>")
	}
}

func TestCFHTMLFragmentFallsBackToWholeDocument(t *testing.T) {
	document := "<html><body>plain</body></html>"
	// A payload with no fragment offsets, only the document span.
	payload := []byte(cfhtmlPayload(document, 0, 0))

	fragment, ok := cfhtmlFragment(payload)
	if !ok {
		t.Fatal("cfhtmlFragment reported no fragment for a document-only payload")
	}
	if string(fragment) != document {
		t.Fatalf("fragment = %q, want the whole document %q", fragment, document)
	}
}

func TestCFHTMLFragmentRejectsBrokenOffsets(t *testing.T) {
	cases := map[string]string{
		"past the end":  "Version:0.9\r\nStartFragment:0000000010\r\nEndFragment:9999999999\r\n<html>x</html>",
		"reversed span": "Version:0.9\r\nStartFragment:0000000090\r\nEndFragment:0000000010\r\n<html>x</html>",
		"no header":     "<html>x</html>",
		"empty":         "",
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if fragment, ok := cfhtmlFragment([]byte(payload)); ok {
				t.Fatalf("cfhtmlFragment = %q, true; want no fragment", fragment)
			}
		})
	}
}

func TestCFHTMLFragmentAcceptsSpacePaddedOffsets(t *testing.T) {
	// Windows pads the offsets with zeros; some applications pad with spaces
	// instead, so the parser has to tolerate both.
	document := "<b>hi</b>"
	header := func(startHTML, endHTML int) string {
		return fmt.Sprintf("Version:0.9\r\nStartHTML:%10d\r\nEndHTML:%10d\r\n", startHTML, endHTML)
	}
	shift := len(header(0, 0))
	payload := []byte(header(shift, shift+len(document)) + document)

	fragment, ok := cfhtmlFragment(payload)
	if !ok {
		t.Fatal("cfhtmlFragment rejected an unpadded header")
	}
	if string(fragment) != document {
		t.Fatalf("fragment = %q, want %q", fragment, document)
	}
}
