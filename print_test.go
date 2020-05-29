// Copyright 2020 Daniel Erat <dan@erat.org>.
// All rights reserved.

package htmlpretty

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
		// Show the strings starting at the first non-matching line.
		got := strings.Replace(b.String(), "\n", "|\n", -1)
		want := strings.Replace(exp, "\n", "|\n", -1)
		start := 0
		for i := 0; i < len(got) && i < len(want) && got[i] == want[i]; i++ {
			if got[i] == '\n' {
				start = i + 1
			}
		}
		var pre string
		if start > 0 {
			pre = "...\n"
		}

		t.Errorf("Print didn't produce expected output.\n"+
			"Got:\n---\n%s%s\n---\nWant:\n---\n%s%s\n---\n", pre, got[start:], pre, want[start:])
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
    <title>Here's the title</title>
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
    <style>Keep newline, no indenting
</style>
    <script>
   Also preserve leading whitespace</script>
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
    <p>
      It's okay to break if space before  <a href="https://www.example.org/">an anchor tag</a>).
    </p>
    <p>
      <span>Safe to wrap text</span> after inline and starting with space.
    </p>
    <p>
      <span>But don't wrap if inside<span>of an inline tag.</span></span>
    </p>
    <p>
      <span>Need to keep this trail </span>space in text inside inline tag.
    </p>
    <p>
      Inline because short.
    </p>
    <p>
      <a href="http://dont.wrap.consecutive.inline.tags.tld"><picture><source type="image/webp" srcset="image.webp 40w" sizes="40px"><img src="image.png" width="40" height="30"></picture></a>
    </p>
    <no-child></no-child>
    <no-child-wrap>
    </no-child-wrap>
    <no-child-but-long-tag-name-should-wrap>
    </no-child-but-long-tag-name-should-wrap>
    <pre> Preserve  
  space    here.
		</pre>
    <some-custom-element here="is an attribute with a value that can't be wrapped" and here are other attributes></some-custom-element>
    <p>
      <a href="/keep/this/long/first/attribute/on/the/same/line/as/the/opening/tag" but wrap everything else>link</a>
    </p>
    <p>
      Only do that<a href="/when/the/tag/starts/the/line/though" attr>link</a>.
    </p>
    <p>
      <code><span>keep</span><span>this</span><span>all</span><span>on</span><span>a</span><span>single</span><span>line</span></code>
    </p>
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
    <style>Keep newline, no indenting
</style>
    <script>
   Also preserve leading whitespace</script>
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
    <p>
      It's okay to break if space before 
      <a href="https://www.example.org/">an
      anchor tag</a>).
    </p>
    <p>
      <span>Safe to wrap text</span>
      after inline and starting with
      space.
    </p>
    <p>
      <span>But don't wrap if inside<span>of
      an inline tag.</span></span>
    </p>
    <p>
      <span>Need to keep this trail </span>space
      in text inside inline tag.
    </p>
    <p>Inline because short.</p>
    <p>
      <a href="http://dont.wrap.consecutive.inline.tags.tld"><picture><source
      type="image/webp"
      srcset="image.webp 40w"
      sizes="40px"><img src="image.png"
      width="40" height="30"></picture></a>
    </p>
    <no-child></no-child>
    <no-child-wrap></no-child-wrap>
    <no-child-but-long-tag-name-should-wrap>
    </no-child-but-long-tag-name-should-wrap>
    <pre> Preserve  
  space    here.
		</pre>
    <some-custom-element
        here="is an attribute with a value that can't be wrapped"
        and here are other attributes>
    </some-custom-element>
    <p>
      <a href="/keep/this/long/first/attribute/on/the/same/line/as/the/opening/tag"
          but wrap everything else>link</a>
    </p>
    <p>
      Only do that<a
      href="/when/the/tag/starts/the/line/though"
      attr>link</a>.
    </p>
    <p>
      <code><span>keep</span><span>this</span><span>all</span><span>on</span><span>a</span><span>single</span><span>line</span></code>
    </p>
  </body>
</html>
`)
}

func TestPrint_Escaping(t *testing.T) {
	const doc = `<!DOCTYPE html>
<html>
  <head>
    <script>
      var i = 1 > 2; var j = 1 < 2;
    </script>
    <noscript><style>body {color: red}<style></noscript>
    <style>
      div>span { color: black; }
    </style>
  </head>
  <body>
    Here's an escaped &lt;tag&gt; &amp; an "ampersand".
    <pre>Here's another
              escaped &lt;tag&gt;.</pre>
    <noscript>
         JavaScript <b>must</b> be enabled.
    </noscript>
  </body>
</html>
`
	checkPrint(t, doc, "  ", 80, doc)
}

func TestPrint_NoWrap(t *testing.T) {
	checkPrint(t, `<!DOCTYPE html>
<html>
  <head>
  </head>
  <body>
    Here's a very long line. It keeps going and going, without any end in sight. Whatever will we do? I guess we'll just need to wait and see if its author gets tired of typing nonsense at some point.
  </body>
</html>
`, "  ", 0, `<!DOCTYPE html>
<html>
  <head></head>
  <body>Here's a very long line. It keeps going and going, without any end in sight. Whatever will we do? I guess we'll just need to wait and see if its author gets tired of typing nonsense at some point.</body>
</html>
`)
}
