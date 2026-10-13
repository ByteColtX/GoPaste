// Package syntax detects the language of a clipboard code snippet and
// tokenises it for syntax highlighting. It depends on chroma but not on MyGo,
// so the UI layer maps its token types to theme colours and decides how to
// draw them.
package syntax

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v3"
	"github.com/alecthomas/chroma/v3/lexers"
)

// Token is one coalesced run of source text together with the chroma token
// type that classifies it.
type Token struct {
	Type  chroma.TokenType
	Value string
}

// Highlight is the analysis of one snippet: the detected language's display
// name, empty when nothing matched, and the coloured token runs in order.
type Highlight struct {
	Language string
	Tokens   []Token
}

// Empty reports whether the snippet carries no highlighting, so callers can
// fall back to plain text.
func (h Highlight) Empty() bool { return h.Language == "" || len(h.Tokens) == 0 }

// Tokenising is linear in the input and runs on the UI thread, so two bounds
// keep a pathological snippet from stalling a frame. Detection stays cheap at
// any size, so an over-budget snippet still reports its language and only
// drops the coloured tokens.
const (
	// maxHighlightBytes is the largest snippet that gets tokenised. Larger
	// snippets fall back to plain text while keeping their language label.
	maxHighlightBytes = 16 << 10
	// maxHighlightTokens bounds the coloured runs. Dense snippets such as
	// minified sources yield many runs from few bytes, and the UI rebuilds one
	// span per run on every frame, so past this bound the snippet stays plain.
	maxHighlightTokens = 6000
)

// Analyse detects the snippet's language and tokenises it. An unrecognised
// snippet yields a Highlight with no language and no tokens. A recognised but
// over-budget snippet keeps its language and returns no tokens, which callers
// render as plain text.
func Analyse(text string) Highlight {
	name := detectLanguage(text)
	if name == "" {
		return Highlight{}
	}
	lexer := lexers.Get(name)
	if lexer == nil {
		return Highlight{}
	}
	language := lexer.Config().Name
	if len(text) > maxHighlightBytes {
		return Highlight{Language: language}
	}
	tokens, err := tokenise(lexer, text)
	if err != nil || len(tokens) > maxHighlightTokens {
		return Highlight{Language: language}
	}
	return Highlight{Language: language, Tokens: tokens}
}

// tokenise runs the lexer over text and keeps its non-empty runs. Coalesce
// merges adjacent tokens of one type, so the result carries fewer, larger runs
// than the raw token stream.
func tokenise(lexer chroma.Lexer, text string) ([]Token, error) {
	iter, err := chroma.Coalesce(lexer).Tokenise(nil, text)
	if err != nil {
		return nil, err
	}
	var tokens []Token
	for token := range iter {
		if token.Value == "" {
			continue
		}
		tokens = append(tokens, Token{Type: token.Type, Value: token.Value})
	}
	return tokens, nil
}

// rule recognises one language from a snippet's text. Rules are tried in order
// and the first match wins, so more distinctive formats come first.
type rule struct {
	name  string
	match func(string) bool
}

var rules = []rule{
	{"PHP", regexpMatcher(`<\?php|<\?=`)},
	{"XML", regexpMatcher(`(?i)^\s*<\?xml\b`)},
	{"HTML", regexpMatcher(`(?i)<!doctype\s+html|<html[\s>/]|</html>|<head[\s>]|<body[\s>]|<div[\s>]|<span[\s>]|<meta[\s>]|<script[\s>]|<style[\s>]|<h[1-6][\s>]|<p[\s>]|<a\s+href`)},
	{"XML", looksLikeXML},
	{"JSON", looksLikeJSON},
	{"TOML", looksLikeTOML},
	{"YAML", looksLikeYAML},
	{"Go", regexpMatcher(`(?m)^\s*package\s+[A-Za-z_]\w*|\bfunc\s+(?:\([^)]*\)\s*)?[A-Za-z_]\w*\s*\(|:=|(?m)^\s*import\s*\(|(?m)^\s*import\s+"[^"]+"\s*$`)},
	{"Rust", regexpMatcher(`\bfn\s+[A-Za-z_]\w*|\blet\s+mut\b|\bimpl\b|\buse\s+[a-z]\w*::|\bpub\s+(?:fn|struct|enum|trait|mod)\b|\bprintln!\s*\(|\bmatch\s+[A-Za-z_]\w*\s*\{`)},
	{"TypeScript", regexpMatcher(`\binterface\s+[A-Za-z_]\w*|:\s*(?:string|number|boolean|any|void|unknown|never|object)\b|\benum\s+[A-Za-z_]\w*|(?m)^\s*type\s+[A-Za-z_]\w*\s*=|\bimplements\s+[A-Za-z_]\w*|\breadonly\s+[A-Za-z_]\w*`)},
	{"JavaScript", regexpMatcher(`\bfunction\s+[A-Za-z_$]\w*\s*\(|\bconst\s+[A-Za-z_$]\w*\s*=|\blet\s+[A-Za-z_$]\w*\s*=|=>|\bconsole\.\w+|\brequire\s*\(|(?m)^\s*import\s+[\w{},*\s]+\s+from\s+['"]|\bdocument\.\w+|\bwindow\.\w+|(?m)^\s*export\s+(?:default|const|function|class|let)\b`)},
	{"Python", regexpMatcher(`(?m)^\s*def\s+[A-Za-z_]\w*\s*\(|(?m)^\s*class\s+[A-Za-z_]\w*|\bprint\s*\(|__name__|(?m)^\s*(?:import|from)\s+[A-Za-z_]\w*|\bself\b|\belif\b`)},
	{"Bash", regexpMatcher(`(?m)^#!\s*\S*/(?:ba|z|k|c)?sh\b|\b(?:fi|done|esac|elif)\b|\bthen\b|\$\{?[A-Za-z_]\w*|\becho\s|\bsudo\b|\bchmod\b`)},
	{"Java", regexpMatcher(`\bpublic\s+(?:static\s+)?(?:final\s+)?(?:class|interface|void)\b|\bSystem\.out\.\w+|(?m)^\s*import\s+java\.|\bpublic\s+static\s+void\s+main\b|\bprivate\s+[A-Za-z_]\w*\s+[A-Za-z_]\w*\s*\(`)},
	{"C++", regexpMatcher(`\bstd::|\busing\s+namespace\b|#include\s*<(?:iostream|vector|string|map|set|memory|algorithm|sstream|fstream|utility|array|queue|stack)>|\btemplate\s*<|\b(?:cout|cin|endl)\b|\bnullptr\b|\bpublic\s*:`)},
	{"C", regexpMatcher(`#include\s*<|\bint\s+main\s*\(|\bprintf\s*\(|\bscanf\s*\(|\bstruct\s+[A-Za-z_]\w*\s*\{|\bvoid\s+[A-Za-z_]\w*\s*\(`)},
	{"SQL", regexpMatcher(`(?i)\bselect\b[^;]*\bfrom\b|\binsert\s+into\b|\bcreate\s+(?:table|index|view)\b|\bupdate\b[^;]*\bset\b|\bdelete\s+from\b|\balter\s+table\b|\bjoin\b[^;]*\bon\b`)},
	{"CSS", looksLikeCSS},
}

// detectLanguage returns the chroma lexer name of the first rule that matches,
// or "" when the snippet looks like no supported language.
func detectLanguage(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	for _, r := range rules {
		if r.match(text) {
			return r.name
		}
	}
	return ""
}

func regexpMatcher(pattern string) func(string) bool {
	re := regexp.MustCompile(pattern)
	return re.MatchString
}

var (
	tomlTableRe  = regexp.MustCompile(`^\s*\[\[?[A-Za-z0-9_.\-]+\]\]?\s*$`)
	tomlAssignRe = regexp.MustCompile(`^\s*[A-Za-z0-9_.\-]+\s*=\s*\S`)
	// A YAML block mapping needs whitespace or end-of-line after the colon
	// (key: value or key:). Requiring that keeps C++ scope operators (std::)
	// and Go short assignments (x := 1) from looking like mappings.
	yamlKeyRe  = regexp.MustCompile(`^\s*[A-Za-z0-9_.\-]+\s*:(?:\s|$)`)
	yamlItemRe = regexp.MustCompile(`^\s*-\s`)
	cssRuleRe  = regexp.MustCompile(`(?m)^\s*[.#]?[A-Za-z][\w-]*\s*(?:,[^{}]*)?\{`)
	cssDeclRe  = regexp.MustCompile(`\b[a-z-]+\s*:\s*[^;{}]+;`)
)

// looksLikeJSON reports whether text is a JSON object or array.
func looksLikeJSON(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false
	}
	return json.Valid([]byte(trimmed))
}

// looksLikeXML reports whether text is a markup document that is not HTML,
// recognised by a leading tag and a closing tag.
func looksLikeXML(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || trimmed[0] != '<' {
		return false
	}
	return strings.Contains(trimmed, ">") && strings.Contains(trimmed, "</")
}

// looksLikeTOML reports whether every significant line is a TOML comment, table
// header, or key/value assignment, with at least one table header. Requiring
// the whole document to fit the shape keeps ordinary code assignments out.
func looksLikeTOML(text string) bool {
	lines := significantLines(text)
	if len(lines) == 0 {
		return false
	}
	headers := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		switch {
		case tomlTableRe.MatchString(line):
			headers++
		case tomlAssignRe.MatchString(line):
		default:
			return false
		}
	}
	return headers > 0
}

// looksLikeYAML reports whether every significant line is a YAML mapping,
// sequence item, or document marker, with at least one mapping. The whole-document
// requirement keeps languages that merely contain a colon from matching.
func looksLikeYAML(text string) bool {
	lines := significantLines(text)
	if len(lines) == 0 {
		return false
	}
	mappings := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "---" || trimmed == "...":
		case strings.HasPrefix(trimmed, "#"):
		case yamlItemRe.MatchString(line):
		case yamlKeyRe.MatchString(line):
			mappings++
		default:
			return false
		}
	}
	return mappings > 0
}

// looksLikeCSS reports whether text pairs a selector rule with a declaration
// block, the shape of a stylesheet rather than of other brace languages.
func looksLikeCSS(text string) bool {
	return cssRuleRe.MatchString(text) && cssDeclRe.MatchString(text)
}

// significantLines splits text into its non-blank, trimmed lines.
func significantLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, strings.TrimRight(line, " \t\r"))
	}
	return lines
}
