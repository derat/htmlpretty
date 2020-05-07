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
		lineStart: true,
	}
	return p.doc(root)
}

type printer struct {
	w   io.Writer
	err error

	literalDepth int

	indentStr string
	lineStart bool
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
			p.element(c)
		default:
			return fmt.Errorf("unhandled document child with type %v", c.Type)
		}
	}
	return nil
}

// text handles the supplied node of type html.ElementNode.
func (p *printer) element(n *html.Node) {
	if n.Type != html.ElementNode {
		panic(fmt.Sprintf("Got non-element node %q (type %v)", n.Data, n.Type))
	}

	inline := inlineTags.has(n)
	if !inline {
		p.endl()
	}

	tag := n.Data
	p.indent()
	p.write("<" + tag)
	for _, a := range n.Attr {
		p.write(" " + a.Key)
		if len(a.Val) > 0 {
			// Just escape double-quotes.
			// TODO: Ambiguous ampersands (/&[a-zA-Z0-9]+;/) are also disallowed, but I'm ignoring
			// those for now. See https://html.spec.whatwg.org/multipage/syntax.html#syntax-attributes.
			escaped := strings.ReplaceAll(a.Val, `"`, `&quot;`)
			p.write(`="` + escaped + `"`)
		}
	}
	p.write(">")

	literal := literalTags.has(n)
	if literal {
		p.literalDepth++
	}

	omitClose := omitCloseTags.has(n)
	if !inline && !omitClose {
		p.endl()
	}

	if voidTags.has(n) {
		if literal {
			panic(fmt.Sprintf("<%s> is both literal and void", tag))
		}
		return
	}

	// Indent if needed and print the children.
	if !inline {
		p.level++
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.ElementNode:
			p.element(c)
		case html.TextNode:
			p.text(c)
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
}

var spaceAroundNewline *regexp.Regexp = regexp.MustCompile("(?s)[ \t]*\n[ \t]*")
var repeatedSpace *regexp.Regexp = regexp.MustCompile("  +")

// text handles the supplied node of type html.TextNode.
func (p *printer) text(n *html.Node) {
	if n.Type != html.TextNode {
		panic(fmt.Sprintf("Got non-text node %q (type %v)", n.Data, n.Type))
	}

	// TODO: Can this actually happen?
	if len(n.Data) == 0 {
		return
	}

	if p.inLiteral() {
		p.write(n.Data)
		return
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

	if s != "" {
		p.indent()
		p.write(s)
	}
}

// indent writes the proper amount of whitespace if lineStart is true and literalDepth is 0.
func (p *printer) indent() {
	if !p.inLiteral() && p.lineStart {
		p.write(strings.Repeat(p.indentStr, p.level))
	}
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
}

// write outputs s and sets lineStart to false.
func (p *printer) write(s string) {
	if p.err != nil {
		return
	}
	_, p.err = io.WriteString(p.w, s)
	p.lineStart = false
}
