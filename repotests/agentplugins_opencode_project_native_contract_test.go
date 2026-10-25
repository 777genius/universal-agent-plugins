package pluginkitairepo_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenCodeProjectNativeStrictJSONAndSelectedSkill(t *testing.T) {
	for _, bad := range [][]byte{[]byte(`{"mcp":{},"mcp":{}}`), []byte(`{"mcp":{},"MCP":{}}`), []byte(`{} {}`), []byte(`{"x":NaN}`), {0xff}, []byte(strings.Repeat("[", 66) + strings.Repeat("]", 66))} {
		if _, err := openCodeProjectJSON(bad); err == nil {
			t.Fatalf("malformed whole JSON accepted: %q", bad)
		}
	}
	root := t.TempDir()
	selected := filepath.Join(root, "project", ".opencode", "skills", "guide", "SKILL.md")
	want := map[string]openCodeProjectSkill{"guide": {Name: "guide", Location: selected, Content: "inert body"}}
	for _, bad := range []string{`[]`, `[{"name":"guide","location":12,"content":"inert body"}]`, `[{"name":"guide","location":"x","content":"inert body"},{"name":"guide","location":"y","content":"inert body"}]`} {
		found, err := openCodeProjectSkills([]byte(bad))
		if err == nil && openCodeProjectCheckSkills(found, want, nil) == nil {
			t.Fatal("empty/malformed listing became discovery")
		}
	}
	for _, location := range []string{filepath.Join(root, "other", "SKILL.md"), filepath.Dir(selected) + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "guide" + string(os.PathSeparator) + "SKILL.md"} {
		got := map[string]openCodeProjectSkill{"guide": {Name: "guide", Location: location, Content: "inert body"}}
		if err := openCodeProjectCheckSkills(got, want, nil); err == nil {
			t.Fatal("foreign/traversing output normalized into selected ownership")
		}
	}
	if err := openCodeProjectCheckSkills(want, want, nil); err != nil {
		t.Fatal(err)
	}
	if err := openCodeProjectCheckSkills(want, want, []string{"guide"}); err == nil {
		t.Fatal("withheld skill accepted")
	}
	body, err := openCodeProjectSkillContent([]byte("---\nname: guide\n---\n\ninert body\n"))
	if err != nil || body != "inert body" {
		t.Fatalf("native markdown body: %q %v", body, err)
	}
}
func TestOpenCodeProjectNativeConfigSourceChecks(t *testing.T) {
	good, err := openCodeProjectConfig([]byte(`{"mcp":{"marker":{"type":"remote","url":"http://127.0.0.1:9/A","enabled":true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := openCodeProjectCheckConfig(good, map[string]string{"marker": "http://127.0.0.1:9/A"}, nil); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"http://127.0.0.1:9/B", ""} {
		if err := openCodeProjectCheckConfig(good, map[string]string{"marker": url}, nil); err == nil {
			t.Fatal("other source accepted")
		}
	}
	if err := openCodeProjectCheckConfig(good, nil, []string{"marker"}); err == nil {
		t.Fatal("removed member accepted")
	}
}
func TestOpenCodeProjectNativeFiniteTableAndEnvironment(t *testing.T) {
	table := openCodeProjectCommandTable()
	counts := map[string]int{}
	for _, class := range table {
		counts[class]++
	}
	if len(table) != 43 || counts["git"] != 4 || counts["native"] != 25 || counts["uap"] != 14 {
		t.Fatalf("finite table changed: %v", counts)
	}
	root := t.TempDir()
	f := &openCodeProjectFixture{work: root, home: filepath.Join(root, "home"), state: filepath.Join(root, "installer-state"), client: filepath.Join(root, "bin", "opencode"), git: filepath.Join(root, "git", "git")}
	t.Setenv("OPENCODE_CONFIG", "ambient source")
	t.Setenv("OPENCODE_CONFIG_CONTENT", "ambient inline")
	t.Setenv("OPENCODE_CONFIG_DIR", "ambient dir")
	t.Setenv("TOKEN", "secret")
	env := map[string]string{}
	for _, item := range f.env() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || env[key] != "" {
			t.Fatal("duplicate environment")
		}
		env[key] = value
	}
	for _, key := range []string{"OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT", "OPENCODE_CONFIG_DIR", "OPENCODE_DISABLE_PROJECT_CONFIG", "TOKEN", "HTTPS_PROXY"} {
		if env[key] != "" {
			t.Fatal("ambient authority inherited", key)
		}
	}
	if env["HOME"] != f.home || env["OPENCODE_PURE"] != "1" || env["OPENCODE_DISABLE_DEFAULT_PLUGINS"] != "1" || env["GIT_CONFIG_NOSYSTEM"] != "1" {
		t.Fatal("fresh authority/control lost")
	}
}
func TestOpenCodePassiveRecorderDetectsTCPAndHTTPWithoutMCP(t *testing.T) {
	r := newOpenCodePassiveRecorder(t)
	defer func() {
		if err := r.close(); err != nil {
			t.Error(err)
		}
	}()
	if r.counts() != [2]int64{} {
		t.Fatal("recorder not initially empty")
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(r.url("inert"))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable || r.counts()[0] < 1 || r.counts()[1] != 1 {
		t.Fatalf("passive audit missed connection/request: %v", r.counts())
	}
}
func TestProjectNativeCaptureChild(t *testing.T) {
	mode := os.Getenv("UAP_PROJECT_CAPTURE_CHILD")
	if mode == "" {
		t.Skip("inert test subprocess only")
	}
	switch mode {
	case "empty":
		body, err := io.ReadAll(io.LimitReader(os.Stdin, 256))
		if err != nil || len(body) != 0 {
			os.Exit(8)
		}
		fmt.Fprint(os.Stdout, `{"ok":true}`)
	case "overflow":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 8192))
	case "stderr":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 8192))
	case "deadline":
		time.Sleep(time.Second)
	default:
		os.Exit(9)
	}
	os.Exit(0)
}
func TestProjectNativeCaptureCapsEveryChildClass(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"empty", "overflow", "stderr", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			timeout := time.Second
			if mode == "deadline" {
				timeout = 30 * time.Millisecond
			}
			result := captureProjectNative(context.Background(), executable, t.TempDir(), []string{"UAP_PROJECT_CAPTURE_CHILD=" + mode}, []string{"-test.run=^TestProjectNativeCaptureChild$"}, 128, timeout)
			if len(result.Stdout) > 128 || len(result.Stderr) > 128 {
				t.Fatal("live output cap exceeded")
			}
			switch mode {
			case "empty":
				if result.Exit != 0 || string(result.Stdout) != `{"ok":true}` || len(result.Stderr) != 0 {
					t.Fatalf("fixed child/empty stdin: %+v", result)
				}
			case "overflow":
				if !result.StdoutLimit {
					t.Fatal("stdout overflow not reported")
				}
			case "stderr":
				if !result.StderrLimit {
					t.Fatal("stderr overflow not reported")
				}
			case "deadline":
				if !result.Deadline || result.ElapsedMS > 1500 {
					t.Fatalf("deadline/reap not bounded: %+v", result)
				}
			}
		})
	}
}

func TestOpenCodeProjectCaptureRefusesWaitDelayAndRecognizesOnlyExactAbsence(t *testing.T) {
	if projectNativeErrorKind(exec.ErrWaitDelay, nil) != "capture" || projectNativeErrorKind(&exec.ExitError{}, nil) != "exit" || projectNativeErrorKind(errors.Join(&exec.ExitError{}, exec.ErrWaitDelay), nil) != "capture" || projectNativeErrorKind(&exec.ExitError{}, context.Canceled) != "context" {
		t.Fatal("capture/exit/context causes were conflated")
	}
	partial := projectNativeCaptureResult{Exit: 0, Stdout: []byte(`{"mcp":{}}`), Error: exec.ErrWaitDelay.Error(), ErrorKind: "capture"}
	if projectNativeCaptureAdmission(partial, false) == nil || projectNativeCaptureAdmission(partial, true) == nil {
		t.Fatal("valid partial JSON after WaitDelay was admitted")
	}
	first := "agentplugins: installation \"" + commandsPluginName + "\" was not found\n"
	second := "agentplugins: plugin is not installed for target \"opencode\" in project scope; no target was changed\n"
	for _, line := range []string{first, second} {
		result := projectNativeCaptureResult{Exit: 1, Error: "exit status 1", ErrorKind: "exit", Stderr: []byte(line)}
		if projectNativeCaptureAdmission(result, true) != nil || !openCodeProjectAbsentRemoval(result) {
			t.Fatal("exact source absence refused")
		}
	}
	for _, kind := range []string{"other-stderr", "extra-stdout", "extra-stderr", "signal", "capture", "context", "no-typed-cause"} {
		result := projectNativeCaptureResult{Exit: 1, Error: "exit status 1", ErrorKind: "exit", Stderr: []byte(first)}
		switch kind {
		case "other-stderr":
			result.Stderr = []byte("not found somewhere else\n")
		case "extra-stdout":
			result.Stdout = []byte(`{}`)
		case "extra-stderr":
			result.Stderr = append(result.Stderr, []byte("warning\n")...)
		case "signal":
			result.Exit = -1
		case "capture":
			result.ErrorKind = "capture"
		case "context":
			result.ErrorKind = "context"
		case "no-typed-cause":
			result.ErrorKind = ""
		}
		if openCodeProjectAbsentRemoval(result) {
			t.Fatal("unrecognized failed operation became absence", kind)
		}
	}
}
func TestOpenCodeProjectSnapshotIsBoundedStableAndDoesNotFollowLeafLinks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "owned.json")
	if err := os.WriteFile(path, []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := readProjectNativeSnapshot(path, before, 5)
	if err != nil || string(body) != "inert" {
		t.Fatalf("stable snapshot %q %v", body, err)
	}
	if _, err := readProjectNativeSnapshot(path, before, 4); err == nil {
		t.Fatal("snapshot cap ignored")
	}
	if err := os.WriteFile(path, []byte("larger replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readProjectNativeSnapshot(path, before, 5); err == nil {
		t.Fatal("post-stat growth accepted")
	}
	foreign := filepath.Join(root, "foreign")
	if err := os.WriteFile(foreign, []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, path); err != nil {
		t.Log("symlink unavailable", err)
		return
	}
	if _, err := readProjectNativeSnapshot(path, before, 5); err == nil {
		t.Fatal("leaf alias accepted")
	}
}
func TestOpenCodeProjectConfigRejectsMalformedEnabled(t *testing.T) {
	for _, flag := range []string{`null`, `"true"`, `0`, `false`} {
		config, err := openCodeProjectConfig([]byte(`{"mcp":{"marker":{"type":"remote","url":"http://127.0.0.1:9/A","enabled":` + flag + `}}}`))
		if err != nil {
			t.Fatal(err)
		}
		if openCodeProjectCheckConfig(config, map[string]string{"marker": "http://127.0.0.1:9/A"}, nil) == nil {
			t.Fatal("malformed or disabled entry qualified", flag)
		}
	}
}
