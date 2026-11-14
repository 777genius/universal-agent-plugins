package claude

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func TestEncodePermissionRequestOutcomeIncludesDecisionPayload(t *testing.T) {
	t.Parallel()

	result := EncodePermissionRequestOutcome(PermissionRequestOutcome{
		CommonOutcome: CommonOutcome{Decision: "approve"},
		Permission: &PermissionDecision{
			Behavior: PermissionAllow,
			Message:  "approved",
		},
	})
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}

	var body map[string]any
	if err := json.Unmarshal(result.Stdout, &body); err != nil {
		t.Fatal(err)
	}
	hookSpecific, ok := body["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("hookSpecificOutput = %#v", body["hookSpecificOutput"])
	}
	if hookSpecific["hookEventName"] != "PermissionRequest" {
		t.Fatalf("hookEventName = %#v", hookSpecific["hookEventName"])
	}
	decision, ok := hookSpecific["decision"].(map[string]any)
	if !ok {
		t.Fatalf("decision = %#v", hookSpecific["decision"])
	}
	if decision["behavior"] != "allow" || decision["message"] != "approved" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestDecodePostToolUseRequiresToolName(t *testing.T) {
	t.Parallel()

	_, _, err := DecodePostToolUse(runtime.Envelope{
		Stdin: []byte(`{"hook_event_name":"PostToolUse","tool_name":"   "}`),
	})
	if err == nil || !strings.Contains(err.Error(), "tool_name required") {
		t.Fatalf("DecodePostToolUse error = %v", err)
	}
}
