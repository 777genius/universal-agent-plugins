package observercontract_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	gh "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/geminihooks"
)

// The actual Bash handler records argv before killing itself, independently
// of the supervising shell's signal diagnostic and the hook output.
const signalScript = `printf '%s\0' "${@:3}" > "$1"
printf '{"decision":"deny","continue":false}\n'
printf 'handler diagnostic\n' >&2
kill -s "$2" "$BASHPID"
exit 99
`

func scriptFixture(t *testing.T, extension string, body string) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "TEST script space ' $ & café")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "observer script"+extension)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path, filepath.Join(dir, "recorded argv")
}

func TestObserverBashSelfSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix native Bash signals; Gemini uses PowerShell on Windows")
	}
	executable, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("native shell executable unavailable: bash")
	}
	for _, mode := range []string{"ordinary", "errexit"} {
		for _, signal := range []string{"TERM", "KILL"} {
			t.Run(mode+"/"+signal, func(t *testing.T) {
				shell := nativeShell{gh.Bash, "bash", []string{"-c"}}
				if mode == "errexit" {
					shell.args = []string{"-e", "-c"}
				}
				assertSignalInvocation(t, shell, executable, signal)
			})
		}
	}
}

func assertSignalInvocation(t *testing.T, shell nativeShell, executable string, signal string) {
	t.Helper()
	path, record := scriptFixture(t, ".sh", signalScript)
	args := []string{"space inside", "apostrophe's", "literal$", "$", "$(never-run)", "café 日本語", "&;|><`!%", "-leading", "", `double"quote`}
	argv := append([]string{executable, path, record, signal}, args...)
	hook := gh.HookSpec{Event: "AfterAgent", Name: "fixture.signal", Argv: argv}
	t.Run("naked command", func(t *testing.T) {
		clearRecordedArgv(t, record)
		stdout, stderr, err := runNative(t, shell, executable, plannedCommand(t, shell.kind, hook))
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != -1 {
			t.Errorf("self-signal status=%v; want actual signal termination", err)
		}
		if stdout != "{\"decision\":\"deny\",\"continue\":false}\n" || stderr != "handler diagnostic\n" {
			t.Errorf("self-signal did not execute: stdout=%q stderr=%q status=%v", stdout, stderr, err)
		}
		assertSignalArgv(t, record, args)
	})
	t.Run("child redirection control", func(t *testing.T) {
		clearRecordedArgv(t, record)
		assertChildRedirection(t, shell, executable, argv)
		assertSignalArgv(t, record, args)
	})
	t.Run("observer", func(t *testing.T) {
		clearRecordedArgv(t, record)
		hook.Observer = true
		stdout, stderr, err := runNative(t, shell, executable, plannedCommand(t, shell.kind, hook))
		assertNeutral(t, stdout, stderr, err)
		assertSignalArgv(t, record, args)
	})
}

func assertChildRedirection(t *testing.T, shell nativeShell, executable string, argv []string) {
	t.Helper()
	invocation, err := gh.RenderArgv(shell.kind, argv)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce the old guard's behavior using a real shell, rather than
	// inspecting the renderer's source or matching its command string.
	guard := "(" + invocation + ") >/dev/null 2>&1 || :; printf '{}\\n'; exit 0"
	stdout, stderr, err := runNative(t, shell, executable, guard)
	if stdout != "{}\n" || err != nil {
		t.Errorf("child-only redirection stdout=%q status=%v; want {} LF and exit0", stdout, err)
	}
	if stderr == "" {
		t.Error("child-only redirection failed to reproduce supervising-shell stderr")
	}
	t.Logf("supervising Bash stderr: %q", stderr)
}

func assertSignalArgv(t *testing.T, record string, args []string) {
	t.Helper()
	body, err := os.ReadFile(record)
	if err != nil {
		t.Fatal("actual Bash handler did not record argv:", err)
	}
	parts := bytes.Split(body, []byte{0})
	actual := make([]string, len(parts)-1)
	for i := range actual {
		actual[i] = string(parts[i])
	}
	if !bytes.HasSuffix(body, []byte{0}) || !reflect.DeepEqual(actual, args) {
		t.Errorf("actual Bash argv=%q, want %q; record=%q", actual, args, body)
	}
	t.Logf("actual Bash handler argv: %q", actual)
}
