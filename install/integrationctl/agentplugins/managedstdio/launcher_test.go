package managedstdio

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// Test subprocesses are explicitly invoked fixtures, never an implicit source
// executable fallback in production staging.
func TestSubprocess(t *testing.T) {
	if os.Getenv("UAP_TEST_LAUNCHER") != "1" {
		return
	}
	args := os.Args
	for i, v := range args {
		if v == "fixture" {
			args = args[i+1:]
			break
		}
	}
	if len(args) > 0 && args[0] == "child" {
		if len(args) > 1 && args[1] == "sleep" {
			fmtReady()
			time.Sleep(time.Minute)
			os.Exit(0)
		}
		cwd, _ := os.Getwd()
		body, _ := os.ReadFile("relative.txt")
		input, _ := io.ReadAll(os.Stdin)
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"cwd": cwd, "file": string(body), "args": args[1:], "stdin": string(input), "env": os.Getenv("OPAQUE")})
		_, _ = os.Stderr.WriteString("child-stderr")
		os.Exit(23)
	}
	handled, code := Dispatch(args, os.Stderr)
	if !handled {
		os.Exit(99)
	}
	os.Exit(code)
}
func fmtReady() { _, _ = os.Stdout.WriteString("ready\n") }
func fixtureCommand(t *testing.T, args []string) *exec.Cmd {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, append([]string{"-test.run=^TestSubprocess$", "fixture"}, args...)...)
	cmd.Env = append(os.Environ(), "UAP_TEST_LAUNCHER=1", "OPAQUE=${PLUGIN_ROOT};$(untouched)")
	cmd.Dir = t.TempDir()
	return cmd
}
func TestExecPreservesCWDArgumentsStreamsAndExit(t *testing.T) {
	if !Supported() {
		t.Skip("unsupported native lifecycle")
	}
	for _, anchor := range []pathcontract.Anchor{pathcontract.Plugin, pathcontract.Data} {
		t.Run(string(anchor), func(t *testing.T) {
			plugin, data := t.TempDir(), t.TempDir()
			root := plugin
			if anchor == pathcontract.Data {
				root = data
			}
			if err := os.WriteFile(filepath.Join(root, "relative.txt"), []byte("relative-ok"), 0600); err != nil {
				t.Fatal(err)
			}
			exe, _ := os.Executable()
			body, err := os.ReadFile(exe)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(plugin, "child"), body, 0700); err != nil {
				t.Fatal(err)
			}
			opaque := []string{"", "a b", "'quoted'", "\"double\"", "雪", "$(touch forbidden)", "--flag", "../opaque"}
			childArgs := append([]string{"-test.run=^TestSubprocess$", "fixture", "child"}, opaque...)
			cmd := fixtureCommand(t, Arguments(plugin, data, root+"/./", anchor, "./child", childArgs))
			cmd.Stdin = strings.NewReader("roundtrip\n")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err = cmd.Run()
			if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 23 {
				t.Fatalf("exit: %v; stderr %s", err, stderr.String())
			}
			var got struct {
				CWD, File, Stdin, Env string
				Args                  []string
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			canonical, _ := filepath.EvalSymlinks(root)
			if got.CWD != canonical || got.File != "relative-ok" || got.Stdin != "roundtrip\n" || !reflect.DeepEqual(got.Args, opaque) || got.Env != "${PLUGIN_ROOT};$(untouched)" || stderr.String() != "child-stderr" {
				t.Fatalf("bad child contract %+v stderr %q", got, stderr.String())
			}
		})
	}
}
func TestMalformedMissingAndEscapeFailWithoutStdout(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	outside := t.TempDir()
	_ = os.Symlink(outside, filepath.Join(root, "escape"))
	tests := [][]string{{Mode}, Arguments(root, data, root, "invalid", "runtime", nil), Arguments(root, data, root+"/missing", pathcontract.Plugin, "runtime", nil), Arguments(root, data, root+"/escape", pathcontract.Plugin, "runtime", nil), Arguments(root, data, root, pathcontract.Plugin, "./missing", nil), Arguments(root, data, root, pathcontract.Plugin, "./../escape", nil)}
	for _, args := range tests {
		cmd := fixtureCommand(t, args)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 126 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatalf("bad refusal %v %s %s", err, stdout.String(), stderr.String())
		}
	}
}
func TestPlatformGate(t *testing.T) {
	want := runtime.GOOS == "darwin" || runtime.GOOS == "linux"
	if Supported() != want {
		t.Fatal("unsupported platform claimed")
	}
}
func TestCopiedHelperPinsBytesAndSurvivesSourceRemoval(t *testing.T) {
	if !Supported() {
		t.Skip("unsupported")
	}
	exe, _ := os.Executable()
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(t.TempDir(), "cli")
	if err := os.WriteFile(original, body, 0700); err != nil {
		t.Fatal(err)
	}
	source, err := NewSource(original, "A")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := source.Deliver(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(original); err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(root, filepath.FromSlash(RelativeDirectory), ExecutableName)
	cmd := exec.Command(helper, "-test.run=^TestSubprocess$", "fixture", Mode)
	cmd.Env = append(os.Environ(), "UAP_TEST_LAUNCHER=1")
	output, err := cmd.CombinedOutput()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 126 || !strings.Contains(string(output), "invalid protocol") {
		t.Fatalf("copied executable: %v %s", err, output)
	}
	if err := source.Deliver(t.TempDir()); err == nil {
		t.Fatal("missing trusted source accepted")
	}
	if err := source.Deliver(root); err == nil {
		t.Fatal("collision overwritten")
	}
}

func TestBareExecutableUsesInheritedPATHWithoutShell(t *testing.T) {
	if !Supported() {
		t.Skip("unsupported")
	}
	plugin, data, bin := t.TempDir(), t.TempDir(), t.TempDir()
	exe, _ := os.Executable()
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "uap-test-native"), body, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := fixtureCommand(t, Arguments(plugin, data, plugin, pathcontract.Plugin, "uap-test-native", []string{"-test.run=^TestSubprocess$", "fixture", "child"}))
	cmd.Env = append(cmd.Env, "PATH="+bin)
	output, err := cmd.Output()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 23 {
		t.Fatalf("bare PATH lookup: %v %s", err, output)
	}
}
func TestSourceBytesCannotChangeAfterTrustCapture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli")
	if err := os.WriteFile(path, []byte("A"), 0700); err != nil {
		t.Fatal(err)
	}
	source, err := NewSource(path, "A")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("B"), 0700); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := source.Deliver(root); err == nil {
		t.Fatal("changed source was copied")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("source failure wrote artifact")
	}
}
