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

func ts(tags []string) map[string]struct{} {
	ts := make(map[string]struct{})
	for _, t := range tags {
		ts[t] = struct{}{}
	}
	return ts
}

// Void elements per https://html.spec.whatwg.org/multipage/syntax.html.
// https://www.w3.org/TR/2011/WD-html-markup-20110405/syntax.html#syntax-elements lists a few more.
var voidTags = ts(strings.Fields("area base br col embed hr img input link meta param source track wbr"))

// Elements that don't nest their contents.
// The first child instead appears immediately after the opening tag.
// Space-only text nodes are also preserved within these elements.
var inlineTags = ts(strings.Fields("a b code em i span s strong"))

func isInline(n *html.Node) bool {
	if n == nil || n.Type != html.ElementNode {
		return false
	}
	_, ok := inlineTags[n.Data]
	return ok
}

// Non-void elements whose closing tags are omitted.
// Similar to inline tags, these tags also don't nest their contents.
var omitCloseTags = ts(strings.Fields("li"))

// Elements whose text should be preserved unchanged.
var literalTags = ts(strings.Fields("pre script style"))

type printer struct {
	w   io.Writer
	err error

	literalDepth int

	indentStr string
	lineStart bool
	level     int
}

func Print(w io.Writer, root *html.Node) error {
	p := printer{
		w:         w,
		indentStr: "  ",
		lineStart: true,
	}
	return p.doc(root)
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

func (p *printer) element(n *html.Node) {
	inline := isInline(n)
	if !inline && p.literalDepth == 0 && !p.lineStart {
		p.endl()
	}

	tag := n.Data
	p.write("<" + tag)
	for _, a := range n.Attr {
		p.write(" " + a.Key)
		if len(a.Val) > 0 {
			// Just escape double-quotes.
			// TODO: Ambiguous ampersands (/&[a-zA-Z0-9]+;/) are also disallowed, but I'm ignoring
			// those for now. See https://html.spec.whatwg.org/multipage/syntax.html#syntax-attributes.
			p.writef(`="%s"`, strings.ReplaceAll(a.Val, `"`, `&quot;`))
		}
	}
	p.write(">")

	_, literal := literalTags[tag]
	_, omitClose := omitCloseTags[tag]
	if !inline && !literal && !omitClose {
		p.endl()
	}

	if _, void := voidTags[tag]; void {
		return
	}

	if !inline && !literal {
		p.level++
	}
	if literal {
		p.literalDepth++
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.ElementNode:
			p.element(c)
		case html.TextNode:
			p.text(c)
		}
	}

	if literal {
		p.literalDepth--
	}
	if !inline && !literal {
		p.level--
		p.endl()
	}

	if _, omit := omitCloseTags[tag]; !omit {
		p.write("</" + tag + ">")
	}
	if !inline && !literal {
		p.endl()
	}
}

var spaceAroundNewline *regexp.Regexp = regexp.MustCompile("(?s)[ \t]*\n[ \t]*")
var repeatedSpace *regexp.Regexp = regexp.MustCompile("  +")

func (p *printer) text(n *html.Node) {
	if p.literalDepth > 0 {
		p.write(n.Data)
		return
	}

	// Collapse whitespace for an inline formatting context as described under
	// "How does CSS process whitespace?" in
	// https://developer.mozilla.org/en-US/docs/Web/API/Document_Object_Model/Whitespace.
	s := n.Data
	s = spaceAroundNewline.ReplaceAllString(s, "\n")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = repeatedSpace.ReplaceAllString(s, " ")

	if !isInline(n.PrevSibling) {
		s = strings.TrimLeft(s, " ")
	}
	if !isInline(n.NextSibling) {
		s = strings.TrimRight(s, " ")
	}

	if s != "" {
		p.write(s)
	}
}

func (p *printer) writef(format string, args ...interface{}) {
	p.write(fmt.Sprintf(format, args...))
}

func (p *printer) write(s string) {
	if p.err != nil {
		return
	}
	if p.lineStart && p.literalDepth == 0 {
		s = strings.Repeat(p.indentStr, p.level) + s
		p.lineStart = false
	}
	_, p.err = io.WriteString(p.w, s)
}

func (p *printer) endl() {
	if p.lineStart {
		return
	}
	p.write("\n")
	p.lineStart = true
}
