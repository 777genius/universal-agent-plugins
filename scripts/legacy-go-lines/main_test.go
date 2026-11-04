package main

import "testing"

func TestCodeLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		source string
		want   int
	}{
		{"blank_and_comments", "//go:build linux\npackage p\n\n/* explanatory\n   comment */\n// note\nvar n = 1 // inline\n", 2},
		{"inline_and_multiline", "package p\nvar n = 1 /* inline\ncomment */ + 2\n", 3},
		{"raw_string_spans_lines", "package p\nconst raw = `first\n// not a comment\nthird`\n", 4},
		{"comment_syntax_in_quoted_string", "package p\nconst text = \"// not a comment\"\n", 2},
		{"carriage_returns", "package p\r\n// note\r\nvar n = 1\r\n", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := codeLines([]byte(tc.source))
			if err != nil || got != tc.want {
				t.Fatalf("codeLines() = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}

func TestCodeLinesRefusesMalformedSource(t *testing.T) {
	t.Parallel()
	if _, err := codeLines([]byte("package p\nvar x = `unterminated\n")); err == nil {
		t.Fatal("expected lexical error")
	}
}
