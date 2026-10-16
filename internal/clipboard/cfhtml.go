package clipboard

import (
	"bytes"
	"strconv"
	"strings"
)

// cfhtmlFragment extracts the HTML fragment from a Windows CF_HTML payload.
// CF_HTML is a plain-text header of byte offsets into the payload, followed by
// the document itself:
//
//	Version:0.9
//	StartHTML:0000000105
//	EndHTML:0000012345
//	StartFragment:0000000141
//	EndFragment:0000012345
//	<html>...
//
// Applications may emit the lines in any order and pad the numbers, so the
// header is parsed rather than assumed. The fragment is preferred over the
// whole document, since that is the part the user actually selected.
//
// It reports false when the payload carries no usable span, which is the case
// for a payload that only looks like CF_HTML. The caller then treats the
// reading as absent instead of keeping the header as the copied markup.
func cfhtmlFragment(payload []byte) ([]byte, bool) {
	startHTML, endHTML, startFragment, endFragment := -1, -1, -1, -1

	for _, line := range bytes.Split(payload, []byte{'\n'}) {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 || line[0] == '<' {
			break // the header ends where the document begins
		}
		key, value, ok := bytes.Cut(line, []byte{':'})
		if !ok {
			break
		}
		offset, err := strconv.Atoi(strings.TrimSpace(string(value)))
		if err != nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(string(key))) {
		case "starthtml":
			startHTML = offset
		case "endhtml":
			endHTML = offset
		case "startfragment":
			startFragment = offset
		case "endfragment":
			endFragment = offset
		}
	}

	for _, span := range [][2]int{{startFragment, endFragment}, {startHTML, endHTML}} {
		if span[0] >= 0 && span[1] > span[0] && span[1] <= len(payload) {
			return payload[span[0]:span[1]], true
		}
	}
	return nil, false
}
