package app

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestNormalizeDoctorCommandsDropsBlankEntries(t *testing.T) {
	t.Parallel()

	got := normalizeDoctorCommands([]string{" python ", "", "  ", "python3"})
	if len(got) != 2 || got[0] != "python" || got[1] != "python3" {
		t.Fatalf("commands = %#v", got)
	}
}

func TestFirstDoctorOutputLineReturnsFirstNonBlankLine(t *testing.T) {
	t.Parallel()

	if got := firstDoctorOutputLine("\n  v1.2.3 \nsecond"); got != "v1.2.3" {
		t.Fatalf("line = %q", got)
	}
}

func TestDoctorFindBinaryUsesNormalizedCommands(t *testing.T) {
	// This fixture replaces a package global and must not run in parallel.

	restore := runtimecheck.LookPath
	runtimecheck.LookPath = func(name string) (string, error) {
		if name == "python3" {
			return "/mock/bin/python3", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { runtimecheck.LookPath = restore })

	path, command, err := doctorFindBinary([]string{" ", "python3"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/mock/bin/python3" || command != "python3" {
		t.Fatalf("got (%q, %q)", path, command)
	}
}

func TestDoctorPATHHintEndsWithPeriod(t *testing.T) {
	t.Parallel()

	if hint := doctorPATHHint(); !strings.HasSuffix(hint, ".") {
		t.Fatalf("hint = %q", hint)
	}
}
