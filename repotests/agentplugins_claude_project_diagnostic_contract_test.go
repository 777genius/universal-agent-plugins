package pluginkitairepo_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClaudeProjectDiagnosticLiveCapPreservesExactBoundary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &claudeDiagnosticStream{limit: 4, cancel: cancel}
	if n, err := stream.Write([]byte("abcd")); n != 4 || err != nil || ctx.Err() != nil {
		t.Fatalf("exact boundary=%d,%v", n, err)
	}
	if n, err := stream.Write([]byte("e")); n != 0 || err == nil || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("overflow not canceled=%d,%v", n, err)
	}
	if n, err := stream.Write([]byte(strings.Repeat("x", 4096))); n != 0 || err == nil {
		t.Fatal("later writes exceeded cap")
	}
	body, overflow := stream.snapshot()
	if string(body) != "abcd" || !overflow {
		t.Fatalf("captured=%q overflow=%t", body, overflow)
	}
	body[0] = 'z'
	snapshot, _ := stream.snapshot()
	if string(snapshot) != "abcd" {
		t.Fatal("snapshot aliases capture")
	}
}

func TestClaudeProjectMCPFailureDiagnosticUsesOriginalBoundedContext(t *testing.T) {
	home := t.TempDir()
	project, config := filepath.Join(home, "project-A"), filepath.Join(home, "claude-config")
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + home,
		"ANTHROPIC_AUTH_TOKEN=inert-secret", "ANTHROPIC_BASE_URL=http://model.invalid", "CLAUDE_CONFIG_DIR=ignored"}
	original := append([]string(nil), env...)
	wantEnv := []string{"HOME=" + home, "PATH=" + home, "USERPROFILE=" + home,
		"CLAUDE_CONFIG_DIR=" + config, "DISABLE_AUTOUPDATER=1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1"}
	calls := 0
	record, logs := claudeFailureMCPRecord("managed-mcp-add", "/verified/claude", project, config, env, func(client, cwd string, childEnv, args []string) claudeDiagnosticResult {
		calls++
		if client != "/verified/claude" || cwd != project || !reflect.DeepEqual(childEnv, wantEnv) || !reflect.DeepEqual(args, []string{"mcp", "get", "plugin:uap-commands-fixture:uap_project_fixture"}) {
			t.Fatal("diagnostic escaped the fixed original context")
		}
		childEnv[0] = "inert modified child copy"
		return claudeDiagnosticResult{Stdout: []byte("unknown native format\r\n"), Stderr: []byte("separate diagnostic"), Exit: 0}
	})
	if calls != 1 || !reflect.DeepEqual(env, original) || len(logs) != 2 || record["capture_attempted"] != true || record["post_failure"] != true || record["qualification"] != false || record["config_root"] != config {
		t.Fatal("diagnostic changed original inputs or qualified the failed installer")
	}
	if !reflect.DeepEqual(record["argv"], []string{"/verified/claude", "mcp", "get", "plugin:uap-commands-fixture:uap_project_fixture"}) || record["cwd"] != project {
		t.Fatal("diagnostic recorded a different command")
	}
	if string(logs[record["stdout_file"].(string)]) != "unknown native format\r\n" || string(logs[record["stderr_file"].(string)]) != "separate diagnostic" || record["stdout_bytes"] != 23 || record["stderr_bytes"] != 19 {
		t.Fatal("diagnostic lost the original separate streams")
	}
}

func TestClaudeProjectMCPFailureDiagnosticRefusesUnselectedOrInvalidContext(t *testing.T) {
	home := t.TempDir()
	env := []string{"HOME=" + home, "USERPROFILE=" + home}
	capture := func(string, string, []string, []string) claudeDiagnosticResult {
		t.Fatal("refused diagnostic started a native command")
		return claudeDiagnosticResult{}
	}
	for _, label := range []string{"", "managed-user-mcp-add", "managed-mcp-repair", "trusted-foreign-mcp-add"} {
		if record, logs := claudeFailureMCPRecord(label, "client", home, home, env, capture); record != nil || logs != nil {
			t.Fatal("unselected failure received an MCP diagnostic")
		}
	}
	for _, invalid := range [][]string{nil, append(append([]string(nil), env...), "home="+home), append(append([]string(nil), env...), "PATH=x\x00y")} {
		record, logs := claudeFailureMCPRecord("managed-mcp-add", "client", home, home, invalid, capture)
		if record["capture_attempted"] != false || record["post_failure"] != true || record["qualification"] != false || record["exit"] != -1 || record["error"] == "" || record["stdout_bytes"] != 0 || record["stderr_bytes"] != 0 || len(logs) != 2 {
			t.Fatal("invalid environment was hidden or qualified")
		}
	}
	if _, err := claudeFailureMCPEnvironment("relative-config", env); err == nil {
		t.Fatal("relative config root accepted")
	}
}

func TestClaudeProjectMCPFailureDiagnosticRetainsIncompleteCapture(t *testing.T) {
	home := t.TempDir()
	env := []string{"HOME=" + home, "USERPROFILE=" + home}
	record, logs := claudeFailureMCPRecord("managed-mcp-add", "client", home, home, env, func(string, string, []string, []string) claudeDiagnosticResult {
		return claudeDiagnosticResult{Stdout: []byte("prefix"), Stderr: []byte("error"), Exit: -1, Error: "original diagnostic timeout", Deadline: true, StdoutLimit: true, StderrLimit: true, Elapsed: 10 * time.Second}
	})
	if record["qualification"] != false || record["capture_attempted"] != true || record["exit"] != -1 || record["error"] != "original diagnostic timeout" || record["deadline"] != true || record["stdout_limit"] != true || record["stderr_limit"] != true || record["elapsed_ms"] != int64(10000) {
		t.Fatal("incomplete capture was promoted to success")
	}
	if string(logs[record["stdout_file"].(string)]) != "prefix" || string(logs[record["stderr_file"].(string)]) != "error" {
		t.Fatal("incomplete output was replaced")
	}
}

func TestClaudeProjectDiagnosticFiniteReadonlySelectionAndSeparateStreams(t *testing.T) {
	calls := 0
	env := []string{"HOME=/fresh/home", "CLAUDE_CONFIG_DIR=/fresh/original"}
	records, logs := claudeFailureListingRecords("/verified/claude", "/fresh/A", "/fresh/B", env, func(client, cwd string, childEnv, args []string) claudeDiagnosticResult {
		calls++
		if client != "/verified/claude" || cwd != []string{"/fresh/A", "/fresh/B"}[calls-1] || !reflect.DeepEqual(childEnv, env) || !reflect.DeepEqual(args, []string{"plugin", "list", "--json"}) {
			t.Fatal("diagnostic selection changed")
		}
		childEnv[0] = "inert changed copy"
		return claudeDiagnosticResult{Stdout: []byte("unknown raw JSON"), Stderr: []byte("bounded failure"), Exit: 1, Error: "original child error", Deadline: true, StderrLimit: true, Elapsed: time.Millisecond}
	})
	if calls != 2 || len(records) != 2 || len(logs) != 4 || env[0] != "HOME=/fresh/home" {
		t.Fatal("diagnostic count/env/log boundary changed")
	}
	for _, r := range records {
		if r["qualification"] != false || r["exit"] != 1 || r["deadline"] != true || r["stderr_limit"] != true || r["error"] != "original child error" || r["stdout_bytes"] != len("unknown raw JSON") || r["stderr_bytes"] != len("bounded failure") {
			t.Fatalf("failure metadata fabricated success: %+v", r)
		}
		if len(r["stdout_sha256"].(string)) != 64 || string(logs[r["stdout_file"].(string)]) != "unknown raw JSON" || string(logs[r["stderr_file"].(string)]) != "bounded failure" {
			t.Fatal("raw streams/hash metadata changed")
		}
	}
	if len(boundedClaudeDiagnosticError(errors.New(strings.Repeat("x", 2048)))) != 512 {
		t.Fatal("unbounded diagnostic error")
	}
}
