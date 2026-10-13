package main

import (
	"gopaste/internal/history"
	"gopaste/internal/syntax"

	"github.com/alecthomas/chroma/v3"
	"github.com/egoist/mygo/ui"
)

// codeHighlightCache memoises the syntax analysis of code items by their
// stable item ID. The analysis depends only on the item's immutable text, so
// it is computed once and reused across frames and redraws.
type codeHighlightCache map[string]syntax.Highlight

// codeHighlight returns the cached syntax analysis of a code item, computing
// it on first use. Detection and tokenisation are pure CPU work with no I/O.
func (a *application) codeHighlight(item history.Item) syntax.Highlight {
	if item.Kind != kindCode {
		return syntax.Highlight{}
	}
	if cached, ok := a.codeHighlights[item.ID]; ok {
		return cached
	}
	highlight := syntax.Analyse(item.Text)
	if a.codeHighlights == nil {
		a.codeHighlights = make(codeHighlightCache)
	}
	a.codeHighlights[item.ID] = highlight
	return highlight
}

// codeTag names a code item's language for its auto tag, falling back to the
// generic kind label when the language is not recognised.
func (a *application) codeTag(item history.Item) string {
	if item.Kind == kindCode {
		if language := a.codeHighlight(item).Language; language != "" {
			return language
		}
	}
	return a.l10n().smartTag(item.Kind)
}

// codePreview renders a code item's text with syntax colouring, keeping the
// plain-text card when the language is not recognised.
func (a *application) codePreview(c *ui.Context, theme *ui.Theme, item history.Item) {
	highlight := a.codeHighlight(item)
	// ui.Text and ui.RichText each attach their element to the context on
	// call, so build exactly one of them: creating both would render the
	// snippet twice, once uncoloured above the styled block.
	var block ui.Element
	if highlight.Empty() {
		block = ui.Text(c, item.Text)
	} else {
		block = ui.RichText(c, codeSpans(theme, highlight)...)
	}
	block = block.Font("monospace").FontSize(font(theme, fontCode)).
		TextColor(primaryText(theme)).Padding(space(theme, 4)).Radius(radius(theme, cardRadius)).
		Background(panelFill(theme)).Border(1, borderColor(theme)).Wrap().Selectable()
}

// codeSpans turns a snippet's token runs into styled spans over the theme, so
// the same tokens read on both light and dark appearances.
func codeSpans(theme *ui.Theme, highlight syntax.Highlight) []ui.Span {
	spans := make([]ui.Span, len(highlight.Tokens))
	for i, token := range highlight.Tokens {
		spans[i] = ui.Span{Text: token.Value, Color: syntaxColor(theme, token.Type)}
		if token.Type.Category() == chroma.Comment {
			spans[i].Italic = true
		}
	}
	return spans
}

// syntaxColor maps a chroma token type to a theme colour. The palette is
// derived from the semantic roles the rest of the interface uses, so it stays
// legible in both appearances instead of hard-coding a separate scheme.
func syntaxColor(theme *ui.Theme, tokenType chroma.TokenType) ui.Color {
	switch {
	case tokenType == chroma.Error:
		return theme.Danger
	case tokenType.Category() == chroma.Comment:
		return theme.TextMuted
	case tokenType.Category() == chroma.Keyword:
		return theme.Accent
	case tokenType.SubCategory() == chroma.LiteralString:
		return theme.Success
	case tokenType.SubCategory() == chroma.LiteralNumber:
		return theme.Warning
	case tokenType == chroma.NameFunction, tokenType == chroma.NameFunctionMagic:
		return theme.Accent.Mix(theme.Success, 0.45)
	case tokenType == chroma.NameClass, tokenType == chroma.NameNamespace, tokenType == chroma.NameException:
		return theme.Warning.Mix(theme.Accent, 0.35)
	case tokenType == chroma.NameConstant, tokenType == chroma.NameBuiltin, tokenType == chroma.NameBuiltinPseudo:
		return theme.Warning
	case tokenType == chroma.NameTag, tokenType == chroma.NameDecorator:
		return theme.Accent
	case tokenType == chroma.NameAttribute, tokenType == chroma.NameProperty:
		return theme.Warning
	case tokenType == chroma.GenericDeleted:
		return theme.Danger
	case tokenType == chroma.GenericInserted:
		return theme.Success
	case tokenType == chroma.GenericHeading, tokenType == chroma.GenericSubheading:
		return theme.Accent
	case tokenType.Category() == chroma.Operator, tokenType.Category() == chroma.Punctuation:
		return theme.TextMuted
	default:
		return theme.Text
	}
}
