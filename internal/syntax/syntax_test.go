package syntax

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/chroma/v3"
	"github.com/alecthomas/chroma/v3/lexers"
)

// supportedSnippets holds one realistic snippet per language detectLanguage
// claims to recognise. The key is the chroma lexer's display name, which is
// also what Analyse reports, so the table doubles as a check that the detected
// name resolves back to a lexer.
var supportedSnippets = map[string]string{
	"PHP":        "<?php\n\nfunction greet($name) {\n    echo \"Hello, $name\";\n}\n",
	"XML":        "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<note>\n  <to>Bob</to>\n</note>\n",
	"HTML":       "<!DOCTYPE html>\n<html>\n<head><title>Hi</title></head>\n<body><h1>Hello</h1></body>\n</html>\n",
	"JSON":       "{\n  \"name\": \"gopaste\",\n  \"ok\": true,\n  \"list\": [1, 2, 3]\n}\n",
	"TOML":       "# GoPaste local settings\n[server]\nhost = \"127.0.0.1\"\nport = 8765\n\n[logging]\nlevel = \"info\"\nredact_clipboard = true\n\n[[devices]]\nid = \"macbook-pro\"\nplatform = \"macOS\"\naddress = \"192.168.1.21:8765\"\n",
	"YAML":       "server:\n  host: 127.0.0.1\n  port: 8765\ndevices:\n  - id: macbook-pro\n    platform: macOS\n",
	"Go":         "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n",
	"Docker":     "FROM golang:1.23-alpine AS build\nWORKDIR /src\nCOPY . .\nRUN CGO_ENABLED=0 go build -o /out/gopaste .\n\nFROM gcr.io/distroless/static\nCOPY --from=build /out/gopaste /gopaste\nENTRYPOINT [\"/gopaste\"]\n",
	"Rust":       "fn main() {\n    let mut x = 1;\n    println!(\"{}\", x);\n}\n",
	"TypeScript": "interface User {\n  id: number;\n  name: string;\n}\n\nfunction greet(u: User): void {\n  console.log(u.name);\n}\n",
	"JavaScript": "const x = 1;\nfunction add(a, b) {\n  return a + b;\n}\nconsole.log(add(x, 2));\n",
	"Python":     "def greet(name):\n    print(f\"Hello, {name}\")\n\nif __name__ == \"__main__\":\n    greet(\"world\")\n",
	"Bash":       "#!/bin/bash\nset -e\nfor f in *.txt; do\n  echo \"$f\"\ndone\n",
	"Java":       "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"hi\");\n    }\n}\n",
	"C++":        "#include <iostream>\n\nint main() {\n    std::cout << \"hi\" << std::endl;\n    return 0;\n}\n",
	"C":          "#include <stdio.h>\n\nint main(void) {\n    printf(\"hi\\n\");\n    return 0;\n}\n",
	"SQL":        "SELECT id, name FROM users WHERE active = 1 ORDER BY name;\n",
	"CSS":        ".card {\n  color: #fff;\n  margin: 0 auto;\n}\n",
}

// TestAnalyseDetectsSupportedLanguages pins the languages the detector claims
// to support: each snippet must resolve to its own language and produce at
// least one coloured run.
func TestAnalyseDetectsSupportedLanguages(t *testing.T) {
	for want, src := range supportedSnippets {
		if lexers.Get(want) == nil {
			t.Fatalf("no lexer named %q", want)
		}
		got := Analyse(src)
		if got.Language != want {
			t.Errorf("Analyse(%s): language = %q, want %q", want, got.Language, want)
			continue
		}
		if len(got.Tokens) == 0 {
			t.Errorf("Analyse(%s): no tokens", want)
		}
	}
}

// TestAnalyseDetectsCommonFragments covers snippets that carry a language's
// idiom without its file-level scaffolding, such as an anonymous Go callback
// or a Dockerfile stage, which the detectors must still recognise.
func TestAnalyseDetectsCommonFragments(t *testing.T) {
	cases := []struct {
		want string
		src  string
	}{
		{"Go", "mygo.App.WhenReady(func() {\n\tmygo.NewWindow(mygo.WindowOptions{\n\t\tTitle:   \"GoPaste\",\n\t\tContent: ui.View(app.view),\n\t})\n})\n"},
		{"Go", "if err != nil {\n\treturn err\n}\n"},
		{"Go", "ch := make(chan int)\n"},
		{"Docker", "FROM alpine:3.20\nRUN apk add --no-cache \\\n    git \\\n    make\nCMD [\"sh\"]\n"},
	}
	for _, tc := range cases {
		if got := Analyse(tc.src); got.Language != tc.want {
			t.Errorf("Analyse(%q): language = %q, want %q", tc.src, got.Language, tc.want)
		}
	}
}

// TestAnalyseLeavesPlainTextUnhighlighted checks that prose and blanks stay
// plain, so the preview never colours text that is not code.
func TestAnalyseLeavesPlainTextUnhighlighted(t *testing.T) {
	for _, src := range []string{"", "   \n\t\n", "hello world", "a quick brown fox."} {
		if got := Analyse(src); got.Language != "" || len(got.Tokens) != 0 {
			t.Errorf("Analyse(%q) = %+v, want empty", src, got)
		}
	}
}

// TestAnalyseBoundsLargeSnippets checks the byte bound: an over-budget snippet
// keeps its language label but drops the tokens, so a pathological paste cannot
// stall a frame.
func TestAnalyseBoundsLargeSnippets(t *testing.T) {
	src := "package main\n\nfunc main() {}\n" + strings.Repeat("// filler line\n", 2000)
	if len(src) <= maxHighlightBytes {
		t.Fatalf("test snippet is %d bytes, want > %d", len(src), maxHighlightBytes)
	}
	got := Analyse(src)
	if got.Language != "Go" {
		t.Errorf("language = %q, want Go", got.Language)
	}
	if len(got.Tokens) != 0 {
		t.Errorf("tokens = %d, want 0 (dropped over budget)", len(got.Tokens))
	}
}

// hangingLexers are chroma lexers whose backtracking (regexp2) rules do not
// terminate on ordinary snippets. They are skipped below and never selected by
// detectLanguage, so they cannot reach the UI; this records the upstream bug.
var hangingLexers = map[string]bool{"Jungle": true}

// TestTokeniseRoundTripsEveryLexer feeds a few snippet shapes through every
// lexer chroma ships and checks the tokeniser neither errors nor loses text:
// the concatenated token values must reconstruct the input exactly.
func TestTokeniseRoundTripsEveryLexer(t *testing.T) {
	samples := []string{
		"",
		"\n",
		"hello world\n",
		"// c\nint main(){return 0;}\n",
		"# c\nkey = \"v\"\n",
		"<a href=\"x\">y</a>\n",
	}
	for _, lx := range lexers.GlobalLexerRegistry.Lexers {
		name := lx.Config().Name
		if hangingLexers[name] {
			t.Logf("skipping %s: known to hang in chroma", name)
			continue
		}
		for _, src := range samples {
			tokens, err := tokeniseWithin(lx, src, 5*time.Second)
			if err != nil {
				t.Errorf("%s: tokenise(%q): %v", name, src, err)
				continue
			}
			var b strings.Builder
			for _, tk := range tokens {
				b.WriteString(tk.Value)
			}
			if got := b.String(); got != src {
				t.Errorf("%s: tokenise(%q) lost text, got %q", name, src, got)
			}
		}
	}
}

// tokeniseWithin runs tokenise with a wall-clock bound so an unexpected lexer
// hang fails the test instead of blocking it forever.
func tokeniseWithin(lx chroma.Lexer, src string, d time.Duration) ([]Token, error) {
	type result struct {
		tokens []Token
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{err: fmt.Errorf("panic: %v", r)}
			}
		}()
		tokens, err := tokenise(lx, src)
		ch <- result{tokens, err}
	}()
	select {
	case res := <-ch:
		return res.tokens, res.err
	case <-time.After(d):
		return nil, fmt.Errorf("timed out after %s", d)
	}
}
