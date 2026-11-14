package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestOperationReceiptIsOptIn(t *testing.T) {
	probe := buildReceiptProbe(t)
	for _, test := range []struct {
		name, value string
		configured  bool
		included    bool
	}{
		{name: "legacy"},
		{name: "empty", configured: true},
		{name: "disabled", value: "0", configured: true},
		{name: "malformed", value: "true", configured: true},
		{name: "padded", value: " 1", configured: true},
		{name: "opted-in", value: "1", configured: true, included: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			env := []string{"UAP_TEST_ROOT=" + root, "UAP_TEST_EVENTS=" + filepath.Join(root, "events.jsonl"),
				"UAP_TEST_NONCE=owned", "UAP_TEST_REVISION=receipt", "PLUGIN_ROOT=" + root, "PLUGIN_DATA=" + filepath.Join(root, "data")}
			if test.configured {
				env = append(env, "UAP_TEST_REPORT_OPERATION="+test.value)
			}
			written := runReceiptProbe(t, probe, root, env, "write", "owned", false)
			assertReceiptFacts(t, written, "write", test.included)
			marker := filepath.Join(root, "data", "native-marker.txt")
			fixed := time.Unix(1700000000, 0)
			if err := os.Chtimes(marker, fixed, fixed); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(marker)
			if err != nil {
				t.Fatal(err)
			}
			read := runReceiptProbe(t, probe, root, env, "read", "owned", false)
			assertReceiptFacts(t, read, "read", test.included)
			for _, rejected := range [][2]string{{"erase", "owned"}, {"write", "foreign"}} {
				runReceiptProbe(t, probe, root, env, rejected[0], rejected[1], true)
			}
			after, err := os.Stat(marker)
			if err != nil || !os.SameFile(before, after) || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
				t.Fatalf("read or refused request changed the marker: %v", err)
			}
			body, err := os.ReadFile(marker)
			if err != nil || string(body) != "owned-marker" {
				t.Fatalf("retained marker differs: %q %v", body, err)
			}
			assertReceiptEvents(t, root, written, read)
		})
	}
}

func buildReceiptProbe(t *testing.T) string {
	t.Helper()
	name := "probe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(t.TempDir(), name)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", path, "main.go")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build real probe: %v\n%s", err, output)
	}
	return path
}

func runReceiptProbe(t *testing.T, probe, root string, env []string, operation, nonce string, refused bool) map[string]any {
	t.Helper()
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	for _, request := range []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-11-25"}},
		{"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
		{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{"name": "inspect_runtime", "arguments": map[string]any{"nonce": nonce, "operation": operation, "marker": "owned-marker"}}},
	} {
		if err := encoder.Encode(request); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, probe)
	cmd.Dir, cmd.Env, cmd.Stdin = root, env, &input
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("run real probe: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("expected three actual protocol responses: %s", output)
	}
	var response struct {
		Result struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
		Error *struct{ Code int } `json:"error"`
	}
	if err := json.Unmarshal(lines[2], &response); err != nil {
		t.Fatal(err)
	}
	if refused {
		if response.Error == nil || response.Error.Code != -32602 || len(response.Result.Content) != 0 {
			t.Fatalf("invalid operation/nonce was not refused: %s", lines[2])
		}
		return nil
	}
	if response.Error != nil || response.Result.IsError || len(response.Result.Content) != 1 {
		t.Fatalf("actual marker call failed: %s", lines[2])
	}
	var facts map[string]any
	if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &facts); err != nil {
		t.Fatal(err)
	}
	return facts
}

func assertReceiptFacts(t *testing.T, facts map[string]any, operation string, included bool) {
	t.Helper()
	want := strings.Fields("argv cwd literal marker nonce once pid plugin_data plugin_root revision session")
	if included {
		want = append(want, "operation")
		if facts["operation"] != operation {
			t.Fatalf("reported operation is not the executed branch: %+v", facts)
		}
	}
	got := make([]string, 0, len(facts))
	for key := range facts {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) || facts["nonce"] != "owned" || facts["marker"] != "owned-marker" {
		t.Fatalf("closed probe facts changed: got %v want %v; %+v", got, want, facts)
	}
}

func assertReceiptEvents(t *testing.T, root string, written, read map[string]any) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var calls []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		var event struct {
			Method string         `json:"method"`
			Facts  map[string]any `json:"facts"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Method == "tools/call" {
			calls = append(calls, event.Facts)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []map[string]any{written, read}) {
		t.Fatalf("original call events differ or refused request caused an effect: %+v", calls)
	}
}

func TestHermesParentEventIsOptIn(t *testing.T) {
	probe := buildReceiptProbe(t)
	for _, test := range []struct {
		name, value string
		configured  bool
		included    bool
	}{
		{name: "legacy"},
		{name: "empty", configured: true},
		{name: "disabled", value: "0", configured: true},
		{name: "malformed", value: "true", configured: true},
		{name: "padded", value: " 1", configured: true},
		{name: "hermes-public", value: "1", configured: true, included: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			env := []string{"UAP_TEST_ROOT=" + root, "UAP_TEST_EVENTS=" + filepath.Join(root, "events.jsonl"),
				"UAP_TEST_NONCE=owned", "UAP_TEST_REVISION=receipt", "PLUGIN_ROOT=" + root, "PLUGIN_DATA=" + filepath.Join(root, "data")}
			if test.configured {
				env = append(env, "UAP_TEST_HERMES_PUBLIC_PARENT="+test.value)
			}
			called := runReceiptProbe(t, probe, root, env, "write", "owned", false)
			assertReceiptFacts(t, called, "write", false)
			assertHermesParentEvents(t, root, called, test.included)
		})
	}
}

func assertHermesParentEvents(t *testing.T, root string, called map[string]any, included bool) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("expected the unchanged three event methods, got %d", len(lines))
	}
	for index, method := range []string{"initialize", "tools/list", "tools/call"} {
		var event map[string]any
		if err := json.Unmarshal(lines[index], &event); err != nil {
			t.Fatal(err)
		}
		if len(event) != 6 || event["method"] != method || event["nonce"] != "owned" ||
			event["pid"] != called["pid"] || event["session"] != called["session"] {
			t.Fatalf("original closed event identity differs: %+v", event)
		}
		var expected any
		if method == "tools/call" {
			expected = called
		} else if included {
			expected = map[string]any{"kind": "hermes-public-parent-v1", "parent_pid": float64(os.Getpid())}
		}
		if !reflect.DeepEqual(event["facts"], expected) {
			t.Fatalf("event facts differ for %s (opt-in %v): %+v", method, included, event["facts"])
		}
	}
}

func TestHermesWindowsLineageModeIsSeparateFromLegacyFacts(t *testing.T) {
	original := hermesWindowsParent
	t.Cleanup(func() { hermesWindowsParent = original })
	calls := 0
	hermesWindowsParent = func() (any, error) {
		calls++
		return "owned-lineage", nil
	}
	t.Setenv("UAP_TEST_HERMES_PUBLIC_PARENT", "windows-lineage-v1")
	for _, method := range []string{"initialize", "tools/list"} {
		value, err := probeEventFacts(method, nil)
		if err != nil || value != "owned-lineage" {
			t.Fatalf("exact Windows mode did not observe its own process: %v %v", value, err)
		}
	}
	f := &facts{Nonce: "owned"}
	value, err := probeEventFacts("tools/call", f)
	if err != nil || value != f || calls != 2 {
		t.Fatalf("Windows mode changed existing tool facts: %v %v calls=%d", value, err, calls)
	}
	t.Setenv("UAP_TEST_HERMES_PUBLIC_PARENT", "1")
	value, err = probeEventFacts("initialize", nil)
	if err != nil || !reflect.DeepEqual(value, map[string]any{"kind": "hermes-public-parent-v1", "parent_pid": os.Getppid()}) || calls != 2 {
		t.Fatalf("legacy parent receipt changed: %v %v calls=%d", value, err, calls)
	}
}
