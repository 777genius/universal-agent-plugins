package cursorhooks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	ch "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
)

// Red condition: quoting would execute shell syntax, drop empty args or mutate
// Unicode/quotes. This executes /bin/sh against a real fresh TEST recorder; it
// proves the target shell contract, not a new native Cursor stop/platform claim.
func TestLiteralArgvThroughJSONAndTargetShell(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("qualified Linux shell only; Windows native CI remains pending")
	}
	recorder, home := freshRecorder(t)
	args := []string{"", "arg with spaces", "apostrophe's", "double\"quote", "back\\slash",
		"ü-中文-😀", "é", "é", "semi;literal", "percent%bang!", "&|<>#(){}*?[]~", "-dash",
		"; touch TEST-injected", strings.Repeat("long", 900)}
	command, err := ch.RenderArgv(ch.LinuxUserShell32212, append([]string{recorder}, args...))
	if err != nil {
		t.Fatal(err)
	}
	// Model the actual command's JSON transport, rather than skipping it.
	encoded, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	facts := executeRecorder(t, decoded["command"], home)
	assertRecorded(t, facts, args, home)
	if _, err := os.Stat(filepath.Join(home, ".cursor", "TEST-injected")); !os.IsNotExist(err) {
		t.Fatal("argument executed shell syntax")
	}
	// The planner's fixed entry also executes with the exact public argv.
	req := request(nil, ch.Install, nil)
	req.Specs[0] = ch.HookSpec{Executable: recorder, Selector: filepath.Join(home, "control TEST", "binding's ü.json")}
	i := mustPlan(t, req)
	var entry struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(commandEntry(t, i.Desired), &entry); err != nil {
		t.Fatal(err)
	}
	assertRecorded(t, executeRecorder(t, entry.Command, home),
		[]string{"cursor-event", "stop", "--binding", req.Specs[0].Selector}, home)
}

type recorded struct {
	Argv  []string `json:"argv"`
	Cwd   string   `json:"cwd"`
	Stdin string   `json:"stdin"`
}

func freshRecorder(t *testing.T) (string, string) {
	t.Helper()
	root := os.Getenv("CURSORHOOKS_TEST_ROOT")
	if root == "" {
		root = t.TempDir()
	} else if !filepath.IsAbs(root) || !strings.Contains(filepath.ToSlash(root), "/.research/tmp/") {
		t.Fatal("set CURSORHOOKS_TEST_ROOT to own absolute .research/tmp TEST directory")
	}
	dir, err := os.MkdirTemp(root, "TEST-argv-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	home := filepath.Join(dir, "TEST HOME ü")
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0700); err != nil {
		t.Fatal(err)
	}
	recorder := filepath.Join(dir, "TEST recorder's ü.py")
	script := "#!/usr/bin/python3\nimport sys,json,os\njson.dump({'argv':sys.argv[1:],'cwd':os.getcwd(),'stdin':sys.stdin.read(1024)},sys.stdout,ensure_ascii=False)\n"
	if err := os.WriteFile(recorder, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return recorder, home
}

func executeRecorder(t *testing.T, command, home string) recorded {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + home, "USERPROFILE=" + home, "LANG=C.UTF-8", "TMPDIR=" + filepath.Dir(home)}
	cmd.Dir = filepath.Join(home, ".cursor")
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"stop","status":"completed"}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shell recorder: %v", err)
	}
	var facts recorded
	if err := json.Unmarshal(out, &facts); err != nil {
		t.Fatal(err)
	}
	return facts
}

func assertRecorded(t *testing.T, facts recorded, argv []string, home string) {
	t.Helper()
	want, _ := json.Marshal(argv)
	got, _ := json.Marshal(facts.Argv)
	if !bytes.Equal(want, got) || facts.Cwd != filepath.Join(home, ".cursor") ||
		facts.Stdin != `{"hook_event_name":"stop","status":"completed"}` {
		t.Fatalf("literal process contract mismatch: %+v", facts)
	}
}

// Red condition: an invented Windows/cmd/PowerShell contract, placeholder or
// unsafe/unbounded path would be rendered despite missing native evidence.
func TestUnsupportedLiteralsAndPaths(t *testing.T) {
	for _, shell := range []ch.ShellContract{ch.WindowsUnqualified, "powershell", "cmd", "bash", ""} {
		if _, err := ch.RenderArgv(shell, []string{"/TEST/bin"}); !errors.Is(err, ch.ErrUnsupported) {
			t.Fatalf("unqualified shell accepted: %s", shell)
		}
	}
	for _, token := range []string{"$HOME", "${CURSOR_PLUGIN_ROOT}", "$(touch TEST)", "dollar$literal", "`touch TEST`", "\x00", "line\nfeed", "\r", string([]byte{0xff}), strings.Repeat("x", ch.MaxTokenBytes+1)} {
		if _, err := ch.RenderArgv(ch.LinuxUserShell32212, []string{"/TEST/bin", token}); !errors.Is(err, ch.ErrUnsupported) {
			t.Fatalf("unsupported token accepted: %q", token)
		}
	}
	for _, path := range []string{"", "relative", "/", "//TEST/bin", "/TEST/../bin", "/TEST//bin", "/TEST/bin/", "C:\\TEST\\bin.exe"} {
		if _, err := ch.RenderArgv(ch.LinuxUserShell32212, []string{path}); !errors.Is(err, ch.ErrUnsupported) {
			t.Fatalf("unsafe executable path accepted: %q", path)
		}
		req := request(nil, ch.Install, nil)
		req.Specs[0].Selector = path
		refused(t, req, ch.ErrUnsupported)
	}
	for _, argv := range [][]string{nil, append([]string{"/TEST/bin"}, make([]string, ch.MaxArgs)...),
		{"/TEST/bin", strings.Repeat("x", 4096), strings.Repeat("x", 4096), strings.Repeat("x", 4096), strings.Repeat("x", 4096)}} {
		if _, err := ch.RenderArgv(ch.LinuxUserShell32212, argv); !errors.Is(err, ch.ErrUnsupported) {
			t.Fatal("unbounded argv accepted")
		}
	}
}
