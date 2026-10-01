package vscodelocalhooks_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	hooks "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

type recording struct {
	Args        []string
	CWD         string
	Home        string
	UserProfile string
	Shell       string
}

// Only an inert kernel/shell recorder, never an injected native lifecycle.
func TestTESTArgvRecorder(t *testing.T) {
	if os.Getenv("U2_TEST_RECORDER") != "1" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(3)
	}
	value := recording{Args: os.Args[3:], CWD: cwd, Home: os.Getenv("HOME"), UserProfile: os.Getenv("USERPROFILE"), Shell: os.Getenv("SHELL")}
	if json.NewEncoder(os.Stdout).Encode(value) != nil {
		os.Exit(4)
	}
	os.Exit(0)
}

// Red: apostrophe/expansion/empty/control/path quoting changes argv, or executes
// an injection sentinel. Actual target /bin/sh -c, with no ambient child env.
func TestTESTShellArgvProcess(t *testing.T) {
	root := t.TempDir() // Test runner must place TMPDIR in its own TEST scratch.
	home := filepath.Join(root, "TEST-HOME")
	profile := filepath.Join(root, "TEST-USERPROFILE")
	for _, dir := range []string{home, profile} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	recorder := filepath.Join(root, "TEST ü recorder's $ [brackets] `ticks`; \"quotes\" \\slash\x01")
	copyTestExecutable(t, recorder)
	sentinel := filepath.Join(root, "TEST-INJECTION-MUST-NOT-EXIST")
	values := []string{
		"", "spaces and\ttab", "apostrophe's value", "unicode-ü-中文-😀", "$TEST_LITERAL", "${HOME}",
		"[a-z]*?", "back`tick", ";semicolon", "double\"quote", "back\\slash", "end\\", "control\x01\x1b\x7f",
		"NFC-é", "NFD-e\u0301", "%TEST_LITERAL%", "!TEST_LITERAL!", "a&b",
		"$(/usr/bin/touch '" + sentinel + "')", "`/usr/bin/touch '" + sentinel + "'`",
		"'; /usr/bin/touch '" + sentinel + "'; '",
		"$SHELL", "~/.config", "--looks-like-a-flag", home,
	}
	args := append([]string{"-test.run=^TestTESTArgvRecorder$", "--"}, values...)
	body, err := hooks.Render(linux(), []hooks.Spec{{Event: hooks.Stop, Executable: recorder, Args: args, TimeoutSeconds: 5}})
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]map[string][]struct{ Linux string }
	if err := json.Unmarshal(body, &native); err != nil {
		t.Fatal(err)
	}
	command := native["hooks"]["Stop"][0].Linux
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	child.Dir = home // Reproduce observed default homedir, not inferred workspace.
	child.Env = []string{"HOME=" + home, "USERPROFILE=" + profile, "PATH=/TEST-no-PATH", "SHELL=/TEST-not-a-shell", "U2_TEST_RECORDER=1", "TEST_LITERAL=TEST-expansion-would-be-a-defect", "TMPDIR=" + root, "LC_ALL=C"}
	output, err := child.Output()
	if err != nil {
		t.Fatal("target shell recorder failed", err)
	}
	var got recording
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatal(err)
	}
	want := recording{Args: values, CWD: home, Home: home, UserProfile: profile, Shell: "/TEST-not-a-shell"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv/env/cwd changed: got %#v, want %#v", got, want)
	}
	if _, err := os.Lstat(sentinel); !os.IsNotExist(err) {
		t.Fatal("shell injection sentinel exists or cannot be checked", err)
	}
	t.Logf("TEST rendered native file: %s", body)
	t.Logf("TEST shell recorder result: %s", output)
}

func copyTestExecutable(t *testing.T, destination string) {
	t.Helper()
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, body, 0700); err != nil {
		t.Fatal(err)
	}
	t.Logf("TEST inert recorder SHA-256: %x", sha256.Sum256(body))
}
