package claude

import (
	"encoding/json"
	"testing"
)

func TestPermissionApproveWithUpdatesMapsDecision(t *testing.T) {
	t.Parallel()

	resp := PermissionApproveWithUpdates(json.RawMessage(`{"mode":"safe"}`), []PermissionUpdate{{
		Type:     "replaceRules",
		Behavior: PermissionAllow,
		Mode:     "acceptEdits",
	}})
	resp.Decision = "approve"
	resp.Reason = "allowed"

	out := permissionOutcomeFromResponse(resp)
	if out.Decision != "approve" {
		t.Fatalf("decision = %q", out.Decision)
	}
	if out.Permission == nil {
		t.Fatal("expected permission decision")
	}
	if out.Permission.Behavior != PermissionAllow {
		t.Fatalf("behavior = %q", out.Permission.Behavior)
	}
	if string(out.Permission.UpdatedInput) != `{"mode":"safe"}` {
		t.Fatalf("updated input = %s", out.Permission.UpdatedInput)
	}
	if len(out.Permission.UpdatedPermissions) != 1 || out.Permission.UpdatedPermissions[0].Mode != "acceptEdits" {
		t.Fatalf("updated permissions = %+v", out.Permission.UpdatedPermissions)
	}
}

func TestPostToolUseOutcomeFromResponsePreservesToolOutput(t *testing.T) {
	t.Parallel()

	resp := &PostToolUseResponse{
		CommonResponse:       CommonResponse{SystemMessage: "sync"},
		AdditionalContext:    "extra",
		UpdatedMCPToolOutput: json.RawMessage(`{"ok":true}`),
	}

	out := postToolUseOutcomeFromResponse(resp)
	if out.SystemMessage != "sync" {
		t.Fatalf("system message = %q", out.SystemMessage)
	}
	if out.AdditionalContext != "extra" {
		t.Fatalf("additional context = %q", out.AdditionalContext)
	}
	if string(out.UpdatedMCPToolOutput) != `{"ok":true}` {
		t.Fatalf("tool output = %s", out.UpdatedMCPToolOutput)
	}
}

func TestPermissionBlockBuildsDenyResponse(t *testing.T) {
	t.Parallel()

	resp := PermissionBlock("blocked", true)
	if resp.Permission == nil {
		t.Fatal("expected permission decision")
	}
	if resp.Permission.Behavior != PermissionDeny {
		t.Fatalf("behavior = %q", resp.Permission.Behavior)
	}
	if resp.Permission.Message != "blocked" {
		t.Fatalf("message = %q", resp.Permission.Message)
	}
	if !resp.Permission.Interrupt {
		t.Fatal("expected interrupt")
	}
}
