// Package main counts Go source lines with tokens (not comments) for the legacy size ratchet.
package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"strings"
)

func codeLines(source []byte) (int, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("source.go", -1, len(source))
	var firstError error
	var scan scanner.Scanner
	scan.Init(file, source, func(pos token.Position, message string) {
		if firstError == nil {
			firstError = fmt.Errorf("%s: %s", pos, message)
		}
	}, scanner.ScanComments)

	lines := make(map[int]struct{})
	for {
		pos, kind, literal := scan.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.COMMENT || (kind == token.SEMICOLON && literal == "\n") {
			continue
		}
		first := file.Position(pos).Line
		last := first + strings.Count(literal, "\n")
		for line := first; line <= last; line++ {
			lines[line] = struct{}{}
		}
	}
	if firstError != nil {
		return 0, firstError
	}
	return len(lines), nil
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: legacy-go-lines < source.go")
		os.Exit(2)
	}
	// The caller opens an already-selected source file. Do not accept a
	// filesystem path argument (and never traverse caller-controlled paths).
	content, err := io.ReadAll(os.Stdin)
	if err == nil {
		var count int
		count, err = codeLines(content)
		if err == nil {
			fmt.Println(count)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "legacy-go-lines: %v\n", err)
	os.Exit(2)
}
