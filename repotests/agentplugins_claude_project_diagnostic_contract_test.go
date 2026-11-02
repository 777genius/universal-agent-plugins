package pluginkitairepo_test

import (
	"context"
	"errors"
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
