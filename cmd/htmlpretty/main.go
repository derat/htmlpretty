// Copyright 2020 Daniel Erat <dan@erat.org>.
// All rights reserved.

package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/net/html"

	"github.com/derat/htmlpretty"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\nPretty-print an HTML5 document from stdin.\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	indent := flag.String("indent", "  ", "String to use for each level of indenting")
	wrap := flag.Int("wrap", 120, "Line wrap length")
	flag.Parse()

	node, err := html.Parse(os.Stdin)
	if err != nil {
		fmt.Fprint(os.Stderr, "Failed parsing HTML: ", err)
		os.Exit(1)
	}
	if err := htmlpretty.Print(os.Stdout, node, *indent, *wrap); err != nil {
		fmt.Fprint(os.Stderr, "Failed printing HTML: ", err)
		os.Exit(1)
	}
}
