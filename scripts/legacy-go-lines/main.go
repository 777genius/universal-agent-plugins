// Package main counts Go source lines with tokens (not comments) for the legacy size ratchet.
package main

import (
	"fmt"
	"go/scanner"
	"go/token"
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
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: legacy-go-lines <base.go> <current.go>")
		os.Exit(2)
	}
	var counts [2]int
	for i, path := range os.Args[1:] {
		content, err := os.ReadFile(path)
		if err == nil {
			counts[i], err = codeLines(content)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "legacy-go-lines: %s: %v\n", path, err)
			os.Exit(2)
		}
	}
	if counts[1] > counts[0] {
		fmt.Fprintf(os.Stderr, "legacy-go-lines: %s grew from %d to %d code lines\n", os.Args[2], counts[0], counts[1])
		os.Exit(1)
	}
}
