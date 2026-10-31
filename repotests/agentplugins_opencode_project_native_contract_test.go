package pluginkitairepo_test

import (
	"context"
	"encoding/json"
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
	for _, location := range []string{filepath.Join(root, "other", "SKILL.md"), strings.Join([]string{filepath.Dir(selected), "..", "guide", "SKILL.md"}, string(os.PathSeparator))} {
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
func TestOpenCodeProjectV2NativeConfigRequiresExactServersLayout(t *testing.T) {
	t.Setenv("AGENTPLUGINS_OPENCODE_VERSION", "2.2.5")
	want := map[string]string{"marker": "http://127.0.0.1:9/A"}
	good := []byte(`{"mcp":{"servers":{"marker":{"type":"remote","url":"http://127.0.0.1:9/A","disabled":false}}}}`)
	value, err := openCodeProjectConfig(good)
	if err != nil || openCodeProjectCheckConfig(value, want, nil) != nil {
		t.Fatal("real V2 nested server schema refused", err)
	}
	for _, bad := range [][]byte{
		[]byte(`{"mcp":{"marker":{"type":"remote","url":"http://127.0.0.1:9/A"}}}`),
		[]byte(`{"mcp":{"servers":{"marker":{"type":"remote","url":"http://127.0.0.1:9/A","disabled":true}}}}`),
		[]byte(`{"mcp":{"servers":{"marker":{"type":"remote","url":"http://127.0.0.1:9/A","disabled":"false"}}}}`),
		[]byte(`{"mcp":{"servers":{"marker":{"type":"remote","url":"http://127.0.0.1:9/B"}}}}`),
	} {
		value, err := openCodeProjectConfig(bad)
		if err == nil && openCodeProjectCheckConfig(value, want, nil) == nil {
			t.Fatal("unsupported/disabled/foreign V2 server qualified")
		}
	}
	if openCodeProjectCheckConfig(map[string]any{"mcp": map[string]any{"servers": map[string]any{"marker": map[string]any{}}}}, nil, []string{"marker"}) == nil {
		t.Fatal("V2 configured server was misclassified as removed")
	}
}

func TestOpenCodeProjectV2NativeSourceInventoryBindsExactUserAndSelectedProject(t *testing.T) {
	t.Setenv("AGENTPLUGINS_OPENCODE_VERSION", "2.2.5")
	root := t.TempDir()
	user, project := filepath.Join(root, "global", "opencode"), filepath.Join(root, "project")
	userPath, projectPath := filepath.Join(user, "opencode.jsonc"), filepath.Join(project, "opencode.jsonc")
	config := func(name, marker string) map[string]any {
		return map[string]any{"mcp": map[string]any{"servers": map[string]any{
			name: map[string]any{"type": "remote", "url": "http://127.0.0.1:9/" + marker, "disabled": false},
		}}}
	}
	rows := []any{
		map[string]any{"type": "document", "path": userPath, "info": config("user", "USER")},
		map[string]any{"type": "directory", "path": user},
		map[string]any{"type": "document", "path": projectPath, "info": config("managed", "PROJECT")},
		map[string]any{"type": "directory", "path": filepath.Join(project, ".opencode")},
	}
	encode := func(items []any) []byte {
		t.Helper()
		body, err := json.Marshal(items)
		if err != nil { t.Fatal(err) }
		return body
	}
	actual, err := openCodeProjectV2SourceConfig(encode(rows), user, project, false, true)
	if err != nil { t.Fatal("true V2 source inventory refused", err) }
	if err := openCodeProjectCheckConfig(actual, map[string]string{
		"user": "http://127.0.0.1:9/USER", "managed": "http://127.0.0.1:9/PROJECT",
	}, nil); err != nil { t.Fatal(err) }
	for name, altered := range map[string][]any{
		"missing-user": rows[1:],
		"missing-project": rows[:2],
		"duplicate-project": append(append([]any{}, rows...), rows[2]),
		"unrequested-inline": append(append([]any{}, rows...), map[string]any{"type": "document", "info": config("inline", "INLINE")}),
		"foreign-path": []any{rows[0], rows[1], map[string]any{"type": "document", "path": filepath.Join(root, "foreign", "opencode.jsonc"), "info": config("managed", "PROJECT")}, rows[3]},
		"flat-v2": []any{rows[0], rows[1], map[string]any{"type": "document", "path": projectPath, "info": map[string]any{"mcp": map[string]any{"managed": map[string]any{"url":"http://127.0.0.1:9/PROJECT"}}}}, rows[3]},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := openCodeProjectV2SourceConfig(encode(altered), user, project, false, true); err == nil {
				t.Fatal("unauthorized V2 source or hidden merge admitted")
			}
		})
	}
	inline := append(append([]any{}, rows...), map[string]any{"type": "document", "info": config("managed", "INLINE")})
	merged, err := openCodeProjectV2SourceConfig(encode(inline), user, project, true, true)
	if err != nil { t.Fatal("authorized V2 inline document refused", err) }
	if err := openCodeProjectCheckConfig(merged, map[string]string{"managed": "http://127.0.0.1:9/INLINE"}, nil); err != nil { t.Fatal(err) }
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url("inert"), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Error(err)
	}
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
		if _, err := fmt.Fprint(os.Stdout, `{"ok":true}`); err != nil {
			os.Exit(7)
		}
	case "overflow":
		if _, err := fmt.Fprint(os.Stdout, strings.Repeat("x", 8192)); err != nil {
			os.Exit(7)
		}
	case "stderr":
		if _, err := fmt.Fprint(os.Stderr, strings.Repeat("x", 8192)); err != nil {
			os.Exit(7)
		}
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

func TestOpenCodeProjectPublicProgressIsExactAndLabelBound(t *testing.T) {
	resolve := "Resolving and validating one Agent Plugin package for every selected target...\n"
	cases := map[string]string{
		"add-user":           resolve + "Installing uap-opencode-user-control for 1 selected clients...\n",
		"add-A":              resolve + "Installing uap-commands-fixture for 1 selected clients...\n",
		"add-B":              resolve + "Installing uap-commands-fixture for 1 selected clients...\n",
		"add-same-user":      resolve + "Installing uap-commands-fixture for 1 selected clients...\n",
		"add-same-A":         resolve + "Installing uap-commands-fixture for 1 selected clients...\n",
		"add-json":           resolve + "Installing uap-commands-fixture for 1 selected clients...\n",
		"repeat-add-A":       resolve + "Applying transactional client package...\n",
		"unchanged-update-A": "Resolving and validating one updated Agent Plugin package for every selected target...\n",
		"update-A":           "Resolving and validating one updated Agent Plugin package for every selected target...\n",
		"intact-repair-A":    "Resolving and validating each unique exact installed package revision once...\n",
		"missing-repair-A":   "Resolving and validating each unique exact installed package revision once...\n",
		"remove-A":           "",
		"remove-json":        "",
	}
	for label, expected := range cases {
		t.Run(label, func(t *testing.T) {
			if !openCodeProjectProgress(label, []byte(expected)) {
				t.Fatal("source-established exact progress refused")
			}
			for _, dirty := range []string{expected + "warning\n", " " + expected, expected + "\r\n", strings.ReplaceAll(expected, "\n", "\r\n")} {
				if dirty != expected && openCodeProjectProgress(label, []byte(dirty)) {
					t.Fatal("changed/extra progress bytes accepted")
				}
			}
			if expected != "" && openCodeProjectProgress(label, nil) {
				t.Fatal("missing required progress admitted")
			}
		})
	}
	for _, label := range []string{"version", "initial-A-config", "git-A", "repeat-remove-A", "unknown"} {
		if openCodeProjectProgress(label, nil) || openCodeProjectProgress(label, []byte(cases["add-A"])) {
			t.Fatal("non-ordinary UAP lane acquired progress allowance", label)
		}
	}
	if openCodeProjectProgress("repeat-add-A", []byte(cases["add-A"])) || openCodeProjectProgress("add-A", []byte(cases["repeat-add-A"])) {
		t.Fatal("single/group imperative add progress conflated")
	}
}
