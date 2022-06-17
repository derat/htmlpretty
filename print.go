// Copyright 2020 Daniel Erat <dan@erat.org>.
// All rights reserved.

// Package htmlpretty pretty-prints HTML5 documents.
package htmlpretty

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Print pretty-prints the supplied HTML document to w.
// The supplied indent string is used for a single level of indenting.
// If wrap is positive, lines will be wrapped at that many bytes where possible.
func Print(w io.Writer, root *html.Node, indent string, wrap int) error {
	p := printer{
		w:         w,
		indentStr: indent,
		wrapWidth: wrap,
		lineStart: true,
	}
	if err := p.doc(root); err != nil {
		return err
	}
	return p.werr
}

// tagSet holds a set of HTML tag names.
type tagSet map[string]struct{}

func newTagSet(tags []string) tagSet {
	ts := make(tagSet)
	for _, t := range tags {
		ts[t] = struct{}{}
	}
	return ts
}

// has returns true if n's tag is contained in ts.
// Returns false if n is nil.
func (ts tagSet) has(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	_, ok := ts[n.Data]
	return ok
}

// Void elements per https://html.spec.whatwg.org/multipage/syntax.html.
// https://www.w3.org/TR/2011/WD-html-markup-20110405/syntax.html#syntax-elements lists a few more.
var voidTags = newTagSet(strings.Fields("area base br col embed hr img input link meta param source track wbr"))

// Elements that appear inline.
// No newline is added before the element or after it.
// Contents are not also not nested: The first child instead appears immediately after
// the opening tag, and the last child appears immediately before the closing tag.
// Spaces in text nodes adjacent to these tags are preserved.
// This is based on the list at https://developer.mozilla.org/en-US/docs/Web/HTML/Inline_elements.
var inlineTags = newTagSet(strings.Fields("a abbr acronym amp-img b big cite code data def del dfn em " +
	"i img ins kbd mark picture q s small span source strong sub sup svg time tt u wbr"))

// Elements whose children should be indented and displayed on their own lines.
// This overrides inlineTags's behavior, and it primarily exists to improve the
// formatting of picture elements containing source and img elements, and of
// nested amp-img elements.
var listTags = newTagSet(strings.Fields("amp-img ol picture svg ul"))

// Non-void elements whose closing tags are omitted.
// Similar to inline tags, these tags also don't nest their contents.
// A newline is printed at the point where the closing tag would have appeared, though.
var omitCloseTags = newTagSet(strings.Fields("li"))

// Elements whose contents should be preserved unchanged (i.e. no whitespace changes or escaping).
var literalTags = newTagSet(strings.Fields("noscript script style"))

// Elements whose contents should retain their original whitespace but still be escaped.
var keepSpaceTags = newTagSet(strings.Fields("pre"))

type printer struct {
	w         io.Writer
	werr      error // first error seen while writing to w
	indentStr string
	wrapWidth int

	level          int  // current indentation level
	literalDepth   int  // number of literalTags elements that we're nested in
	keepSpaceDepth int  // number of keepSpaceTags elements that we're nested in
	lineStart      bool // true if we're at the start of a line
	lineWidth      int  // width of the current line
}

func (p *printer) inLiteral() bool {
	return p.literalDepth > 0
}
func (p *printer) inKeepSpace() bool {
	return p.keepSpaceDepth > 0
}

// doc handles the supplied node of type html.DocumentNode.
// This is the main entry point into printer.
func (p *printer) doc(n *html.Node) error {
	if n.Type != html.DocumentNode {
		return fmt.Errorf("root node has non-document type %v", n.Type)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.DoctypeNode:
			p.write("<!DOCTYPE " + c.Data + ">")
			p.endl()
		case html.ElementNode:
			if err := p.element(c); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unhandled doc child %q with type %v", c.Data, c.Type)
		}
	}
	return nil
}

// element handles the supplied node of type html.ElementNode.
func (p *printer) element(n *html.Node) error {
	tag := n.Data
	if n.Type != html.ElementNode {
		return fmt.Errorf("got non-element node %q of type %v", tag, n.Type)
	}

	// Print the opening tag first.
	inline := inlineTags.has(n)
	if forceInline := p.openTag(n); forceInline {
		inline = true
	}

	// Preserve the formatting of the things that we'll print next if needed.
	literal := literalTags.has(n)
	if literal {
		p.literalDepth++
	}
	keepSpace := keepSpaceTags.has(n)
	if keepSpace {
		p.keepSpaceDepth++
	}

	if voidTags.has(n) {
		if literal || keepSpace {
			panic(fmt.Sprintf("<%s> is both literal/keep-space and void", n.Data))
		}
		return nil
	}

	hasChildren := n.FirstChild != nil
	listChildren := listTags.has(n)
	omitClose := omitCloseTags.has(n)

	if hasChildren {
		// Indent if needed before printing the children.
		if !inline || listChildren {
			if !omitClose {
				p.endl()
			}
			p.level++
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.ElementNode:
				if err := p.element(c); err != nil {
					return err
				}
				if listChildren {
					p.endl()
				}
			case html.TextNode:
				if err := p.text(c); err != nil {
					return err
				}
			case html.CommentNode:
				// TODO: Don't strip comments, maybe?
				continue
			default:
				return fmt.Errorf("unexpected node %q of type %d", c.Data, c.Type)
			}
		}
		if !inline || listChildren {
			p.level--
			p.endl()
		}
	}

	// Avoid wrapping the closing tag.
	if !omitClose {
		p.maybeIndent()
		p.write(closeTag(n))
	}
	if literal {
		p.literalDepth--
	}
	if keepSpace {
		p.keepSpaceDepth--
	}
	if !inline {
		p.endl()
	}
	return nil
}

// text handles the supplied node of type html.TextNode.
func (p *printer) text(n *html.Node) error {
	if n.Type != html.TextNode {
		panic(fmt.Sprintf("Got non-text node %q (type %v)", n.Data, n.Type))
	}
	// TODO: Can this actually happen?
	if len(n.Data) == 0 {
		return nil
	}

	// Write literal text... literally.
	if p.inLiteral() {
		p.write(n.Data)
		return nil
	}

	s := n.Data
	s = escapeText(s)

	// If we're preserving spaces (i.e. in <pre>), we need to perform escaping.
	if p.inKeepSpace() {
		p.write(s)
		return nil
	}

	// Otherwise, we additionally remove excess spaces.
	s = collapseText(s, n)
	if s == "" {
		return nil
	}

	p.maybeIndent()

	// If we collapsed the text node to a single space, just print it: the word-at-a-time code below
	// wouldn't print anything, which would result in us dropping deliberate whitespace between
	// inline elements. This is a bit hokey, as it can result in us adding a trailing space to a
	// line unnecessarily if the next line is wrapped, but that should be harmless.
	if s == " " {
		p.write(s)
		return nil
	}

	startSpace := s[0] == ' '
	endSpace := s[len(s)-1] == ' '

	// Avoid wrapping the first part of the text node if it follows or is in an inline element and doesn't already
	// start with whitespace, since we don't want to reformat input like "(<a>link</a>)" as "(<a>link</a>\n)".
	// We avoid "(\n<a>link</a>)" by being careful in how we wrap opening tags in openTag().
	wrapStart := 0
	if (inlineTags.has(n.PrevSibling) || inlineTags.has(n.Parent)) && !startSpace {
		wrapStart = 1
	}

	// Write the text one word at a time.
	// This is hopefully safe since we condensed spaces above.
	words := strings.Fields(strings.TrimSpace(s))
	for i, w := range words {
		// Try to preserve starting and ending spaces. Also prepend a space to each
		// word, and avoid adding two spaces if we started with just one word consisting
		// of a single space.
		if (i == 0 && startSpace) || i != 0 {
			w = " " + w
		}
		if i == len(words)-1 && endSpace && w != " " {
			w = w + " "
		}

		if i < wrapStart {
			p.write(w)
		} else {
			p.wrap(w, "")
		}
	}
	return nil
}

// maybeIndent writes the proper amount of whitespace if we're at the start of a line
// and not currently printing literally.
func (p *printer) maybeIndent() {
	if p.inLiteral() || p.inKeepSpace() || !p.lineStart {
		return
	}
	s := strings.Repeat(p.indentStr, p.level)
	p.write(s) // updates lineStart and lineWidth
}

// wrap writes s, first writing a newline and indentation if we would exceed p.wrapWidth.
// extra denotes extra indentation to use if the line is wrapped.
func (p *printer) wrap(s, extra string) {
	if !p.inLiteral() && !p.inKeepSpace() &&
		p.wrapWidth > 0 && p.lineWidth+len(s) > p.wrapWidth {
		p.endl()
		p.maybeIndent()
		s = extra + strings.TrimLeft(s, " ")
	}
	p.write(s)
}

// endl terminates the current line by writing a newline and setting lineStart to true.
// It does nothing if we're already at the start of a line or if we're printing literally.
func (p *printer) endl() {
	if p.inLiteral() || p.inKeepSpace() {
		return
	}
	if p.lineStart {
		return
	}
	p.write("\n")
	p.lineStart = true
	p.lineWidth = 0
}

// write outputs s, sets lineStart to false, and increments lineWidth.
func (p *printer) write(s string) {
	if p.werr != nil {
		return
	}
	_, p.werr = io.WriteString(p.w, s)
	p.lineStart = false
	p.lineWidth += len(s)
}

func (p *printer) openTag(n *html.Node) (forceInline bool) {
	// Construct the opening tag.
	// The tokens are of the form [`<foo`, ` abc`, ` def="123">`].
	tokens := append([]string{}, "<"+n.Data)
	for _, a := range n.Attr {
		as := " " + a.Key
		if len(a.Val) > 0 {
			// Escape double-quotes.
			// TODO: Ambiguous ampersands (/&[a-zA-Z0-9]+;/) are also disallowed, but I'm ignoring
			// those for now. See https://html.spec.whatwg.org/multipage/syntax.html#syntax-attributes.
			val := strings.Replace(a.Val, `"`, `&quot;`, -1)

			// Collapse repeated whitespace in 'class' attributes and remove leading and trailing
			// spaces (https://html.spec.whatwg.org/multipage/dom.html#global-attributes:classes-2).
			if a.Key == "class" {
				val = strings.TrimSpace(whitespace.ReplaceAllString(val, " "))
			}

			as += `="` + val + `"`
		}
		tokens = append(tokens, as)
	}
	tokens[len(tokens)-1] += ">" // avoid wrapping closing bracket since it'd look funny
	tagLen := len(strings.Join(tokens, ""))

	// Start a new line for non-inline nodes. Also start inline nodes on a new line if they'd
	// be wrapped... unless they're in or following another inline node or a text node that didn't end
	// with whitespace or another inline node, in which case we need to be careful to not introduce
	// new whitespace by wrapping.
	inline := inlineTags.has(n)
	wouldWrap := p.wrapWidth > 0 && p.lineWidth+tagLen > p.wrapWidth
	prev := n.PrevSibling
	prevTextNotSpace := prev != nil && prev.Type == html.TextNode &&
		(prev.Data == "" || !whitespace.MatchString(prev.Data[len(prev.Data)-1:]))
	startSpaceMatters := inlineTags.has(prev) || inlineTags.has(n.Parent) || prevTextNotSpace
	if !inline || (wouldWrap && !startSpaceMatters) {
		p.endl()
	}

	startedLine := p.lineStart
	p.maybeIndent()

	// If it looks like we can fit everything including the closing tag on a single line,
	// treat this tag as inline.
	if !literalTags.has(n) && !p.inLiteral() &&
		!keepSpaceTags.has(n) && !p.inKeepSpace() {
		childLen := -1
		if n.FirstChild == nil {
			childLen = 0
		} else if hasSingleChild(n) && n.FirstChild.Type == html.TextNode {
			childLen = len(collapseText(escapeText(n.FirstChild.Data), n.FirstChild))
		}
		if childLen >= 0 && (p.lineWidth+tagLen+childLen+len(closeTag(n)) < p.wrapWidth || p.wrapWidth <= 0) {
			forceInline = true
		}
	}

	var unwrapTokens int
	var wrapIndent string
	if startedLine {
		// Indent wrapped attributes two levels.
		wrapIndent = strings.Repeat(p.indentStr, 2)
		unwrapTokens = 1
		// If the first token is shorter than the amount of indenting on the next
		// line, it's better to put the second token on the first line.
		if len(tokens[0]) < len(wrapIndent) {
			unwrapTokens = 2
		}
	} else if (inline || forceInline) && startSpaceMatters {
		unwrapTokens = 1
	}
	for i, t := range tokens {
		if i < unwrapTokens {
			p.write(t)
		} else {
			p.wrap(t, wrapIndent)
		}
	}

	return forceInline
}

// hasSingleChild returns true if n has a single child.
func hasSingleChild(n *html.Node) bool {
	return n.FirstChild != nil && n.FirstChild == n.LastChild
}

// closeTag constructs a closing tag for n, e.g. "</strong>".
// An empty string is returned if n is a void element or should omit its closing tag.
func closeTag(n *html.Node) string {
	if n.Type != html.ElementNode || voidTags.has(n) || omitCloseTags.has(n) {
		return ""
	}
	return "</" + n.Data + ">"
}

// escapeText performs hacky, slow escaping on s.
// We avoid using html.EscapeString since its aggressiveness is a bit annoying:
// it also escapes `'` and `"`.
func escapeText(s string) string {
	s = strings.Replace(s, "&", "&amp;", -1)
	s = strings.Replace(s, "<", "&lt;", -1)
	s = strings.Replace(s, ">", "&gt;", -1)
	return s
}

// https://developer.mozilla.org/en-US/docs/Glossary/Whitespace:
// "HTML Living Standard specifies 5 characters as the ASCII whitespace:
// U+0009 TAB, U+000A LF, U+000C FF, U+000D CR, and U+0020 SPACE."
var whitespace *regexp.Regexp = regexp.MustCompile(`[\t\n\f\r ]+`)

// collapseText removes whitespace for an inline formatting context to achieve roughly the
// same effect as the process described in "How does CSS process whitespace?" in
// https://developer.mozilla.org/en-US/docs/Web/API/Document_Object_Model/Whitespace.
//
// This is probably woefully inadequate: HTML whitespace is very complicated and I don't
// think it's actually possible to determine what's safe to do without knowing whether we're
// an inline, block, or inline-block context, which seems like it'd require handling CSS.
func collapseText(s string, n *html.Node) string {
	s = whitespace.ReplaceAllString(s, " ")

	// Drop leading and trailing whitespace if we don't have siblings that will be printed
	// adjacent to us -- we can presumably just use the printer's whitespace in that case.
	// Preserve the whitespace if we're inside of an inline element, though.
	if !inlineTags.has(n.Parent) {
		if !inlineTags.has(n.PrevSibling) {
			s = strings.TrimLeft(s, " ")
		}
		if !inlineTags.has(n.NextSibling) {
			s = strings.TrimRight(s, " ")
		}
	}

	return s
}
