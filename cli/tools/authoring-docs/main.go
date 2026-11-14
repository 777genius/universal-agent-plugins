// authoring-docs is a preparation-only exporter, never a product entrypoint.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flags := flag.NewFlagSet("authoring-docs", flag.ContinueOnError)
	source := flags.String("source-sha", "", "exact checked-out source commit (required)")
	checkout := flags.String("checkout", ".", "repository checkout")
	out := flags.String("out-dir", "", "absent destination for prepared reference (required)")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if err := export(*checkout, *source, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
