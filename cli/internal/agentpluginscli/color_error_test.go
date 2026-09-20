package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func TestErrorTextMissingAddArguments(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"add", "--format=" + format, "--color=always"}
			root := NewRoot(App{Output: &stdout, ErrorOutput: &stderr})
			root.SetArgs(args)
			err := root.Execute()
			if err == nil {
				t.Fatal("expected argument error")
			}
			want := "accepts 1 arg(s), received 0"
			if format == "human" {
				want += "\nProvide a plugin name or source, for example:\n  agentplugins add ./my-plugin\nSee agentplugins add --help"
				want = (terminaltheme.Theme{Enabled: true}).Text(terminaltheme.Error, want)
			}
			if got := ErrorText(args, &stderr, err); got != want {
				t.Fatalf("diagnostic = %q; want %q", got, want)
			}
			if stdout.Len() != 0 {
				t.Fatal("argument error wrote to stdout")
			}
		})
	}
}

func TestErrorTextLongWrappedGuidance(t *testing.T) {
	err := fmt.Errorf("resolve package: %w\nSee agentplugins add --help", errors.New(strings.Repeat("package detail 界 ", 80)))
	for _, flags := range [][]string{{"--color=never"}, {"--color=always", "--format=json"}, {"--unknown"}} {
		if got := ErrorText(append([]string{"add"}, flags...), io.Discard, err); got != err.Error() {
			t.Fatalf("long diagnostic changed: got %d bytes, want %d", len(got), len(err.Error()))
		}
	}
}

func TestErrorTextHostileMultiline(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"safe", "failure\n\n  example 界\nSee --help", "failure\n\n  example 界\nSee --help"},
		{"ANSI OSC CR", "\x1b[31mfailure\x1b[0m\r\n\x1b]52;c;payload\aexample\x1b]8;;https://evil.example\x1b\\ link\x1b]8;;\x1b\\\nSee\x00\x7f\u0085\u202e --help", "failure\nexample link\nSee --help"},
		{"unterminated OSC", "failure\x1b]52;c;payload\nSee --help", "failure\nSee --help"},
		{"CR overwrite", "failure\roverwrite\b\t\nSee --help", "failureoverwrite\nSee --help"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []string{"never", "always"} {
				want := (terminaltheme.Theme{Enabled: mode == "always"}).Text(terminaltheme.Error, tc.want)
				if got := ErrorText([]string{"add", "--color=" + mode}, io.Discard, errors.New(tc.input)); got != want {
					t.Fatalf("%s diagnostic = %q; want %q", mode, got, want)
				}
			}
		})
	}
}
