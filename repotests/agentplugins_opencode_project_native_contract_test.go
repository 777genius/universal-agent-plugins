package pluginkitairepo_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
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
