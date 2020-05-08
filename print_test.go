// Copyright 2020 Daniel Erat <dan@erat.org>.
// All rights reserved.

package pretty

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func checkPrint(t *testing.T, doc, indent string, wrap int, exp string) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal("Parse failed: ", err)
	}
	var b bytes.Buffer
	if err := Print(&b, root, indent, wrap); err != nil {
		t.Fatal("Print failed: ", err)
	}
	if b.String() != exp {
		t.Errorf("Print didn't produce expected output.\n"+
			"Got:\n---\n%s\n---\nWant:\n---\n%s\n---\n",
			strings.ReplaceAll(b.String(), "\n", "|\n"),
			strings.ReplaceAll(exp, "\n", "|\n"))
	}
}

func TestPrint_Simple(t *testing.T) {
	checkPrint(t, `
<!DOCTYPE html>
<html><head>
     <title>
	    Here's the title</title>
</head><body>
	Here's some body
	   text
 <a href="page.html">with a link</a>.
</body>

</html>
`, "  ", 80, `<!DOCTYPE html>
<html>
  <head>
    <title>
      Here's the title
    </title>
  </head>
  <body>
    Here's some body text <a href="page.html">with a link</a>.
  </body>
</html>
`)
}

func TestPrint_Wrapping(t *testing.T) {
	checkPrint(t, `<!DOCTYPE html>
<html>
  <head>
    <title>
      Here's a fairly long title with a lot of words in it.
    </title>
    <script>Stuff in this script tag shouldn't be wrapped</script>
    <style>Ditto for everything that's in this style tag</style>
  </head>
  <body>
    <p>
      Here's some text that should be safe to wrap. There's really nothing interesting going on here.
    </p>
    <p>
      Here's some text that has <a href="https://www.example.org/">an anchor tag</a> in it.
    </p>
    <p>
      Don't break the start of this tag (<a href="https://www.example.org/">an anchor tag</a>).
    </p>
    <p>
      Or the end here (<a>an anchor tag</a>).
    </p>
    <some-custom-element here="is an attribute with a value that can't be wrapped" and here are other attributes></some-custom-element>
   </body>
</html>
`, "  ", 41, `<!DOCTYPE html>
<html>
  <head>
    <title>
      Here's a fairly long title with a
      lot of words in it.
    </title>
    <script>Stuff in this script tag shouldn't be wrapped</script>
    <style>Ditto for everything that's in this style tag</style>
  </head>
  <body>
    <p>
      Here's some text that should be
      safe to wrap. There's really
      nothing interesting going on here.
    </p>
    <p>
      Here's some text that has 
      <a href="https://www.example.org/">an
      anchor tag</a> in it.
    </p>
    <p>
      Don't break the start of this tag (<a
      href="https://www.example.org/">an
      anchor tag</a>).
    </p>
    <p>
      Or the end here (<a>an anchor tag</a>).
    </p>
    <some-custom-element
        here="is an attribute with a value that can't be wrapped"
        and here are other attributes>
    </some-custom-element>
  </body>
</html>
`)
}
