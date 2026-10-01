package pluginkitai_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	sdk "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

// Red: a new stop alias steals an old callback/output, adds a newline to ordinary
// RunContext, or neutralizes ordinary permission denial/decode diagnostics.
func TestCursorObserverLegacyInvocations(t *testing.T) {
	binary := buildCursorConsumer(t)
	cases := []struct {
		selector, input, kind, output string
		code                          int
	}{
		{"Stop", `{"hook_event_name":"Stop","session_id":"TEST_SESSION","stop_hook_active":true}`, "claude-stop", "{}", 0},
		{"Notification", `{"hook_event_name":"Notification","session_id":"TEST_SESSION","message":"TEST_MESSAGE"}`, "claude-notification", "{}", 0},
		{"CodexStop", `{"hook_event_name":"Stop","session_id":"TEST_SESSION","stop_hook_active":true}`, "codex", "", 0},
		{"GeminiAfterAgent", `{"hook_event_name":"AfterAgent","session_id":"TEST_SESSION","prompt":"TEST_PROMPT","prompt_response":"TEST_RESPONSE"}`, "gemini-agent", "{}", 0},
		{"GeminiNotification", `{"hook_event_name":"Notification","session_id":"TEST_SESSION","notification_type":"ToolPermission"}`, "gemini-notification", "{}", 0},
		{"CursorStop", cursorInput, "cursor", "{}", 0},
		{"cUrSoRsToP", cursorInput, "cursor", "{}", 0},
		{"PreToolUse", `{"hook_event_name":"PreToolUse","session_id":"TEST_SESSION","tool_name":"TEST_TOOL","tool_input":{}}`, "claude-permission", "", 2},
	}
	for _, tc := range cases {
		t.Run(tc.selector, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, tc.selector, "ordinary")
			cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
			cmd.Stdin = strings.NewReader(tc.input)
			var out, diag bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &diag
			code := 0
			if err := cmd.Run(); err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			// Ordinary permission exit-2 diagnostics follow the consumer record.
			first := strings.Split(diag.String(), "\n")[0]
			records := cursorRecords(t, first)
			if code != tc.code || out.String() != tc.output || len(records) != 1 || records[0].Kind != tc.kind {
				t.Fatalf("legacy behavior: exit=%d stdout=%q stderr=%q", code, out.String(), diag.String())
			}
			if tc.code == 0 && strings.TrimSpace(diag.String()) != first {
				t.Fatalf("unexpected ordinary SDK diagnostic: %q", diag.String())
			}
			if tc.code == 2 && !strings.Contains(diag.String(), "fixture denial") {
				t.Fatal("permission diagnostic was neutralized")
			}
		})
	}
	t.Run("ordinary malformed remains nonzero", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "CursorStop", "ordinary")
		cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
		cmd.Stdin = strings.NewReader(`{"status":`)
		var out, diag bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &diag
		err := cmd.Run()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 || out.Len() != 0 || !strings.Contains(diag.String(), "decode Cursor stop input") {
			t.Fatal("ordinary SDK error/output policy changed")
		}
	})
	// Red: opting into the observer accidentally runs a registered permission or
	// other-platform handler under neutral semantics.
	for _, selector := range []string{"Stop", "Notification", "CodexStop", "GeminiAfterAgent", "GeminiNotification", "PreToolUse", "UnknownEvent"} {
		t.Run("observer rejects "+selector, func(t *testing.T) {
			if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
				t.Skip("inherited pipe observer transport unqualified on this host")
			}
			code, out, records := runCursorConsumer(t, binary, selector, "normal", cursorInput)
			if code != 0 || out != "{}\n" || len(records) != 1 || !records[0].Clean {
				t.Fatal("observer dispatched a foreign selector")
			}
		})
	}
}

// Red: the beta metadata falsely qualifies all Cursor, promotes stop to stable,
// adds unrelated events, or converts the workspace packaging target to runtime.
func TestCursorStopSupportContract(t *testing.T) {
	count := 0
	for _, entry := range sdk.Supported() {
		if entry.Platform != "cursor" {
			continue
		}
		count++
		if entry.Event != "stop" || entry.Status != "runtime_supported" || entry.Maturity != "beta" || entry.V1Target || entry.Carrier != "stdin_json" || len(entry.Capabilities) != 1 || entry.Capabilities[0] != "cursor_stop" || entry.LiveTestProfile != "cursor_stop_contract" {
			t.Fatalf("wrong beta contract: %+v", entry)
		}
	}
	if count != 1 {
		t.Fatalf("Cursor event count = %d", count)
	}
	for _, profile := range platformmeta.All() {
		if profile.ID == "cursor-workspace" && (profile.SDK.Status != "scaffold_only" || profile.Contract.TargetClass != "workspace_config_lane" || profile.Contract.NativeRoot != ".cursor/mcp.json") {
			t.Fatalf("workspace identity changed: %+v", profile)
		}
	}
}
