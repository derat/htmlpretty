package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func main() {
	root, err := html.Parse(os.Stdin)
	if err != nil {
		log.Print("Parse failed: ", err)
	} else {
		if err := Print(os.Stdout, root); err != nil {
			log.Print("Printing failed: ", err)
		}
	}
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
// the opening tag, and the last child appears immediately after the closing tag.
// Spaces in text nodes adjacent to these tags are preserved.
var inlineTags = newTagSet(strings.Fields("a b code em i span s strong"))

// Non-void elements whose closing tags are omitted.
// Similar to inline tags, these tags also don't nest their contents.
// A newline is printed at the point where the closing tag would have appeared, though.
var omitCloseTags = newTagSet(strings.Fields("li"))

// Elements whose contents should be preserved unchanged.
var literalTags = newTagSet(strings.Fields("pre script style"))

func Print(w io.Writer, root *html.Node) error {
	p := printer{
		w:         w,
		indentStr: "  ",
		wrapWidth: 120,
		lineStart: true,
	}
	if err := p.doc(root); err != nil {
		return err
	}
	return p.werr
}

type printer struct {
	w    io.Writer
	werr error // first error seen while writing to w

	literalDepth int

	indentStr string
	wrapWidth int

	lineStart bool
	lineWidth int
	level     int
}

func (p *printer) inLiteral() bool {
	return p.literalDepth > 0
}

func (p *printer) doc(n *html.Node) error {
	if n.Type != html.DocumentNode {
		return fmt.Errorf("root node has non-document type %v", n.Type)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.DoctypeNode:
			p.write("<!DOCTYPE>")
			p.endl()
		case html.ElementNode:
			if err := p.element(c); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unhandled document child with type %v", c.Type)
		}
	}
	return nil
}

// text handles the supplied node of type html.ElementNode.
func (p *printer) element(n *html.Node) error {
	tag := n.Data
	if n.Type != html.ElementNode {
		panic(fmt.Sprintf("Got non-element node %q (type %v)", tag, n.Type))
	}

	inline := inlineTags.has(n)
	if !inline {
		p.endl()
	}

	// If we're starting on a new line, indent attributes to the tag name plus a space when wrapping.
	wi := 0
	if p.lineStart {
		wi = len(tag) + 2
	}

	p.indent()
	p.wrap("<"+tag, 0)
	for _, a := range n.Attr {
		as := " " + a.Key
		if len(a.Val) > 0 {
			// Just escape double-quotes.
			// TODO: Ambiguous ampersands (/&[a-zA-Z0-9]+;/) are also disallowed, but I'm ignoring
			// those for now. See https://html.spec.whatwg.org/multipage/syntax.html#syntax-attributes.
			escaped := strings.ReplaceAll(a.Val, `"`, `&quot;`)
			as += `="` + escaped + `"`
		}
		p.wrap(as, wi)
	}
	p.write(">") // avoid wrapping closing bracket since it'd look funny

	literal := literalTags.has(n)
	if literal {
		p.literalDepth++
	}

	omitClose := omitCloseTags.has(n)
	if !inline && !omitClose {
		// TODO: It'd be nice to put the closing tag on the same line as the opening one if no children
		// get printed, but with the way this code is currently structured, that'd require a time machine.
		p.endl()
	}

	if voidTags.has(n) {
		if literal {
			panic(fmt.Sprintf("<%s> is both literal and void", tag))
		}
		return nil
	}

	// Indent if needed and print the children.
	if !inline {
		p.level++
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.ElementNode:
			if err := p.element(c); err != nil {
				return err
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
	if !inline {
		p.level--
		p.endl()
	}

	if literal {
		p.literalDepth--
	}

	if !omitClose {
		p.indent()
		p.write("</" + tag + ">")
	}
	if !inline {
		p.endl()
	}
	return nil
}

var spaceAroundNewline *regexp.Regexp = regexp.MustCompile("(?s)[ \t]*\n[ \t]*")
var repeatedSpace *regexp.Regexp = regexp.MustCompile("  +")

// text handles the supplied node of type html.TextNode.
func (p *printer) text(n *html.Node) error {
	if n.Type != html.TextNode {
		panic(fmt.Sprintf("Got non-text node %q (type %v)", n.Data, n.Type))
	}

	// TODO: Can this actually happen?
	if len(n.Data) == 0 {
		return nil
	}

	if p.inLiteral() {
		p.write(n.Data)
		return nil
	}

	// Collapse whitespace for an inline formatting context roughly following the process
	// described in "How does CSS process whitespace?" in
	// https://developer.mozilla.org/en-US/docs/Web/API/Document_Object_Model/Whitespace.
	s := n.Data
	s = spaceAroundNewline.ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = repeatedSpace.ReplaceAllString(s, " ")

	if !inlineTags.has(n.PrevSibling) {
		s = strings.TrimLeft(s, " ")
	}
	if !inlineTags.has(n.NextSibling) {
		s = strings.TrimRight(s, " ")
	}

	if s == "" {
		return nil
	}

	p.indent()

	// Write the text one word at a time.
	// This is hopefully safe since we condensed spaces above.
	startSpace := s[0] == ' '
	endSpace := s[len(s)-1] == ' '
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
		p.wrap(w, 0)
	}
	return nil
}

// indent writes the proper amount of whitespace if lineStart is true and literalDepth is 0.
func (p *printer) indent() {
	if p.inLiteral() || !p.lineStart {
		return
	}
	s := strings.Repeat(p.indentStr, p.level)
	p.write(s) // updates lineStart and lineWidth
}

func (p *printer) wrap(s string, extra int) {
	if !p.inLiteral() && p.lineWidth+len(s) > p.wrapWidth {
		p.endl()
		p.indent()
		s = strings.Repeat(" ", extra) + strings.TrimLeft(s, " ")
	}
	p.write(s)
}

// endl terminates the current line by writing a newline and setting lineStart to true.
// It does nothing if lineStart was already true or if we're printing literally.
func (p *printer) endl() {
	if p.inLiteral() {
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
