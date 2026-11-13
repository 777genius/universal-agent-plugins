package cline

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestSessionNativeInventoryRequiresExactSource(t *testing.T) {
	good := `[{"name":"owned","path":"/project/mcp.json","transportType":"stdio","disabled":false}]`
	if err := verifySessionInventory([]byte(good), "/project/mcp.json", []string{"owned"}); err != nil {
		t.Fatal(err)
	}
	// A native client that reports an empty effective approval list is safe;
	// inherited/nonempty tool approvals must never qualify an inspected session.
	emptyApprovals := strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":[]`, 1)
	if err := verifySessionInventory([]byte(emptyApprovals), "/project/mcp.json", []string{"owned"}); err != nil {
		t.Fatal(err)
	}
	bad := []string{"[]", "null", "help", good + "{}", strings.Replace(good, "owned", "foreign", 1),
		strings.Replace(good, "/project/mcp.json", "/user/mcp.json", 1), strings.Replace(good, "false", "true", 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"disabled":false`, 1),
		strings.Replace(good, `"stdio"`, `""`, 1),
		strings.Replace(good, `"stdio"`, `"sse"`, 1),
		strings.Replace(good, `"stdio"`, `"http"`, 1),
		strings.Replace(good, `"stdio"`, `"websocket"`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":["*"]`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":["read_file"]`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":true`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":null`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":{}`, 1),
		strings.Replace(good, `"disabled":false`, `"disabled":false,"autoApprove":[],"autoApprove":[]`, 1),
		"[null]", "[{},{}]"}
	for _, body := range bad {
		if err := verifySessionInventory([]byte(body), "/project/mcp.json", []string{"owned"}); err == nil {
			t.Fatal("invalid inventory accepted", body)
		}
	}
}

type sessionObservationRunner struct {
	command legacyports.Command
	result  legacyports.CommandResult
	err     error
	bounded bool
	cancel  context.CancelFunc
}

func (runner *sessionObservationRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.command = command
	_, runner.bounded = ctx.Deadline()
	if runner.cancel != nil {
		runner.cancel()
	}
	return runner.result, runner.err
}

func TestSessionNativeInventoryExitAndCapability(t *testing.T) {
	plan := clients.SessionPlan{Executable: "/native/cline", Directory: "/project", Servers: []string{"owned"},
		Environment: map[string]string{"CLINE_MCP_SETTINGS_PATH": "/project/mcp.json", "CLINE_NO_AUTO_UPDATE": "1"}}
	good := []byte(`[{"name":"owned","path":"/project/mcp.json","transportType":"stdio","disabled":false}]`)
	for _, failure := range []struct {
		result legacyports.CommandResult
		err    error
	}{{legacyports.CommandResult{ExitCode: 1, Stdout: good, CleanupComplete: true}, nil}, {legacyports.CommandResult{Stdout: good, CleanupComplete: true}, errors.New("native failure")},
		{legacyports.CommandResult{Stdout: []byte("Usage: cline"), CleanupComplete: true}, nil}, {legacyports.CommandResult{Stdout: make([]byte, (1<<20)+1), CleanupComplete: true}, nil},
		{legacyports.CommandResult{Stdout: good}, nil}, {legacyports.CommandResult{Stdout: good, CleanupComplete: true, NoChildStarted: true}, nil}} {
		runner := &sessionObservationRunner{result: failure.result, err: failure.err}
		if err := New().VerifySession(context.Background(), clients.Env{Runner: runner}, plan); err == nil {
			t.Fatal("native failure accepted")
		}
	}
	runner := &sessionObservationRunner{result: legacyports.CommandResult{Stdout: good, CleanupComplete: true}}
	if err := New().VerifySession(context.Background(), clients.Env{Runner: runner}, plan); err != nil {
		t.Fatal(err)
	}
	if !runner.bounded || runner.command.StdoutLimitBytes != 1<<20 || runner.command.Dir != "/project" ||
		!reflect.DeepEqual(runner.command.Argv, []string{"/native/cline", "--auto-approve", "false", "--cwd", "/project", "config", "mcp", "--json"}) {
		t.Fatalf("native observation changed scope or command: %+v", runner.command)
	}
	canceled, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner.cancel = cancel
	if err := New().VerifySession(canceled, clients.Env{Runner: runner}, plan); !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation admitted native inventory: %v", err)
	}
}
