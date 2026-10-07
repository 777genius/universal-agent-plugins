package observercontract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	gh "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/geminihooks"
)

// A real native child records argv independently of its noisy hook output.
// This executable is an ordinary fixture, never an agent or delivery runtime.
func TestMain(m *testing.M) {
	if len(os.Args) > 3 && os.Args[1] == "--observer-fixture" {
		body, err := json.Marshal(os.Args[4:])
		if err != nil || os.WriteFile(os.Args[2], body, 0600) != nil {
			os.Exit(99)
		}
		_, _ = os.Stdout.WriteString("{\"decision\":\"deny\",\"continue\":false}\n")
		_, _ = os.Stderr.WriteString("observer diagnostic\n")
		status, err := strconv.Atoi(os.Args[3])
		if err != nil {
			os.Exit(99)
		}
		os.Exit(status)
	}
	os.Exit(m.Run())
}

func command(t *testing.T, executable string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	return exec.CommandContext(ctx, executable, args...)
}

func fixture(t *testing.T) (string, string) {
	t.Helper()
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "fixture space ' $ & café")
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	name := "observer fixture"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, body, 0700); err != nil {
		t.Fatal(err)
	}
	return path, filepath.Join(dir, "recorded argv.json")
}

type nativeShell struct {
	kind gh.Shell
	name string
	args []string
}

func TestObserverNativeShellContract(t *testing.T) {
	pwsh, windowsPS := "pwsh", "powershell"
	if runtime.GOOS == "windows" {
		pwsh, windowsPS = "pwsh.exe", "powershell.exe"
	}
	for _, shell := range []nativeShell{
		{gh.Bash, "bash", []string{"-c"}},
		{gh.PowerShell, pwsh, []string{"-NoProfile", "-NonInteractive", "-Command"}},
		{gh.PowerShell, windowsPS, []string{"-NoProfile", "-NonInteractive", "-Command"}},
	} {
		t.Run(shell.name, func(t *testing.T) {
			executable, err := exec.LookPath(shell.name)
			if err != nil {
				t.Skip("native shell executable unavailable: " + shell.name)
			}
			for _, status := range []string{"0", "2", "127", "missing"} {
				for _, observer := range []bool{false, true} {
					t.Run(status+"/observer="+strconv.FormatBool(observer), func(t *testing.T) {
						assertNativeContract(t, shell, executable, status, observer)
					})
				}
			}
			if shell.kind == gh.PowerShell {
				t.Run("thrown invocation", func(t *testing.T) { assertThrownInvocation(t, shell, executable) })
				for _, failure := range []string{"success", "throw", "terminating error"} {
					t.Run("direct console/"+failure, func(t *testing.T) {
						assertConsoleInvocation(t, shell, executable, failure)
					})
				}
			}
		})
	}
}

func assertNativeContract(t *testing.T, shell nativeShell, executable string, status string, observer bool) {
	t.Helper()
	path, record := fixture(t)
	args := []string{"space inside", "apostrophe's", "literal$", "$", "$(never-run)", "café 日本語", "&;|><`!%", "-leading"}
	exit := status
	if status == "missing" {
		exit = "0"
	}
	argv := append([]string{path, "--observer-fixture", record, exit}, args...)
	hook := gh.HookSpec{Event: "AfterAgent", Name: "fixture.observer", Argv: argv, Timeout: 2500, Observer: observer}
	text := plannedCommand(t, shell.kind, hook)
	if status == "missing" {
		// Model Gemini's cached command surviving normal host removal.
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	var nativeStderr string
	if !observer && status == "0" {
		// A coverage-instrumented TestMain fixture can emit a Go runtime
		// diagnostic at exit. Compare generic rendering with the independent
		// direct executable, preserving every byte instead of filtering stderr.
		direct := command(t, path, argv[1:]...)
		var directOut, directErr bytes.Buffer
		direct.Stdout, direct.Stderr = &directOut, &directErr
		if err := direct.Run(); err != nil || directOut.String() != "{\"decision\":\"deny\",\"continue\":false}\n" || !strings.HasPrefix(directErr.String(), "observer diagnostic\n") {
			t.Fatalf("direct native fixture contract: %q %q %v", directOut.String(), directErr.String(), err)
		}
		nativeStderr = directErr.String()
		clearRecordedArgv(t, record)
	}
	stdout, stderr, err := runNative(t, shell, executable, text)
	if observer {
		assertNeutral(t, stdout, stderr, err)
	} else {
		assertGeneric(t, status, stdout, stderr, nativeStderr, err)
	}
	if status == "missing" {
		if _, err := os.Stat(record); !os.IsNotExist(err) {
			t.Fatal("missing executable unexpectedly recorded argv:", err)
		}
		return
	}
	assertRecordedArgv(t, record, args)
}

func assertRecordedArgv(t *testing.T, record string, args []string) {
	t.Helper()
	body, err := os.ReadFile(record)
	if err != nil {
		t.Fatal("real child did not record argv:", err)
	}
	var actual []string
	if err = json.Unmarshal(body, &actual); err != nil || !reflect.DeepEqual(actual, args) {
		t.Fatalf("argv = %q, want %q; decode: %v", actual, args, err)
	}
	t.Logf("actual handler argv: %q", actual)
}

func clearRecordedArgv(t *testing.T, record string) {
	t.Helper()
	if err := os.Remove(record); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func assertNeutral(t *testing.T, stdout string, stderr string, err error) {
	t.Helper()
	if stdout != "{}\n" {
		t.Errorf("neutral stdout=%q; want exact {} LF bytes", stdout)
	}
	if stderr != "" {
		t.Errorf("neutral stderr=%q; want empty", stderr)
	}
	if err != nil {
		t.Errorf("neutral status=%v; want exit0", err)
	}
}

func assertGeneric(t *testing.T, status string, stdout string, stderr string, nativeStderr string, err error) {
	t.Helper()
	if status == "0" {
		if err != nil || stdout != "{\"decision\":\"deny\",\"continue\":false}\n" || stderr != nativeStderr {
			t.Fatalf("generic behavior changed: %q %q %v", stdout, stderr, err)
		}
		return
	}
	// Behavioral red control: the original naked invocation cannot meet the
	// observer contract on deletion/nonzero, regardless of native error text
	// or exit status (PowerShell can report a missing command with exit zero).
	if err == nil && stdout == "{}\n" && stderr == "" {
		t.Fatalf("naked invocation unexpectedly neutral: %q %q %v", stdout, stderr, err)
	}
	if status != "missing" {
		code, _ := strconv.Atoi(status)
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != code {
			t.Fatalf("generic exit status=%v, want %d", err, code)
		}
	}
}

func runNative(t *testing.T, shell nativeShell, executable string, text string) (string, string, error) {
	t.Helper()
	// Run the unchanged official settings resolver before invoking the shell.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node >=22.18 required for native resolver fixture:", err)
	}
	input, _ := json.Marshal(map[string]string{"command": text})
	resolver := command(t, node, "../testdata/expand.mjs")
	resolver.Stdin = bytes.NewReader(input)
	out, err := resolver.Output()
	if err != nil {
		t.Fatal("native resolver:", err)
	}
	var expanded string
	if err = json.Unmarshal(out, &expanded); err != nil || expanded != text {
		t.Fatalf("settings expansion changed command: %q %v", expanded, err)
	}
	if shell.kind == gh.PowerShell {
		// Actual 0.62.0 hookRunner appends this AFTER settings expansion.
		expanded += "; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }"
	}
	cmd := command(t, executable, append(append([]string(nil), shell.args...), expanded)...)
	cmd.Stdin = bytes.NewBufferString("{\"hook_event_name\":\"AfterAgent\"}\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	return stdout.String(), stderr.String(), err
}

func assertThrownInvocation(t *testing.T, shell nativeShell, executable string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "throw ' $.ps1")
	if err := os.WriteFile(path, []byte("Write-Output 'deny'; throw 'invocation failed'"), 0600); err != nil {
		t.Fatal(err)
	}
	hook := gh.HookSpec{Event: "Notification", Name: "fixture.throw", Argv: []string{path}}
	stdout, stderr, err := runNative(t, shell, executable, plannedCommand(t, shell.kind, hook))
	if !strings.Contains(stdout, "deny") || !strings.Contains(stderr, "invocation failed") {
		t.Fatalf("script did not actually throw: stdout=%q stderr=%q status=%v", stdout, stderr, err)
	}
	hook.Observer = true
	stdout, stderr, err = runNative(t, shell, executable, plannedCommand(t, shell.kind, hook))
	assertNeutral(t, stdout, stderr, err)
}
