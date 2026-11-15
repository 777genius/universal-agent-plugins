//go:build linux

package process

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestReadonlyChildEntryRetainsUnsealedPolicyRefusal(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := os.CreateTemp(t.TempDir(), "unsealed-policy-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = policy.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	// This trusted test entry must refuse descriptor 3 before parsing policy,
	// inspecting namespaces, invoking bubblewrap or executing a native client.
	command := exec.CommandContext(ctx, executable, "-test.run=^$")
	command.Env = []string{"PLUGIN_KIT_AI_READONLY_DUPLEX_BOOTSTRAP=1"}
	command.ExtraFiles = []*os.File{policy}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	var exit *exec.ExitError
	if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 2 || ctx.Err() != nil {
		t.Fatal("unsealed bootstrap did not retain exit 2", err, ctx.Err())
	}
	text := stderr.String()
	if stdout.Len() != 0 || len(text) >= 4096 || strings.Count(text, "\n") != 1 ||
		!strings.HasPrefix(text, "readonly native child gate refused: ") ||
		!strings.Contains(text, "readonly private policy is not immutable") {
		t.Fatal("bootstrap discarded its original refusal", text)
	}
	for _, character := range strings.TrimSuffix(text, "\n") {
		if character < 32 || character > 126 {
			t.Fatalf("bootstrap refusal contains an unescaped rune %U", character)
		}
	}
}

func TestReadonlyChildRefusalRetainsOriginalCause(t *testing.T) {
	if got := readonlyChildRefusal(nil); got != "readonly native child gate refused\n" {
		t.Fatal("nil result changed the unconditional refusal", got)
	}
	err := errors.Join(errors.New("original policy refusal"), errors.New("original close refusal"))
	got := readonlyChildRefusal(err)
	if !strings.Contains(got, "cause="+strconv.QuoteToASCII(err.Error())) ||
		!strings.Contains(got, "cause_truncated=false") || !strings.HasPrefix(got, "readonly native child gate refused: ") {
		t.Fatal("original joined refusal was discarded", got)
	}
}

func TestReadonlyChildRefusalEscapesAndBoundsOriginalBytes(t *testing.T) {
	for _, cause := range []string{"newline\ncontrol\r\t\x00\x1bUnicode \u2028雪\xff", strings.Repeat("\xff", 4096), strings.Repeat("x", 512), strings.Repeat("雪", 512)} {
		got := readonlyChildRefusal(errors.New(cause))
		if len(got) >= 4096 || strings.Count(got, "\n") != 1 || !strings.HasSuffix(got, "\n") {
			t.Fatal("refusal exceeded its single-line stream bound", len(got))
		}
		for _, character := range strings.TrimSuffix(got, "\n") {
			if character < 32 || character > 126 {
				t.Fatalf("refusal contains an unescaped rune %U", character)
			}
		}
		preview := cause[:min(len(cause), 512)]
		for _, expected := range []string{"cause=" + strconv.QuoteToASCII(preview), "cause_bytes=" + strconv.Itoa(len(cause)), "cause_truncated=" + strconv.FormatBool(len(cause) > 512)} {
			if !strings.Contains(got, expected) {
				t.Fatal("refusal lost bounded original evidence", expected)
			}
		}
	}
}
