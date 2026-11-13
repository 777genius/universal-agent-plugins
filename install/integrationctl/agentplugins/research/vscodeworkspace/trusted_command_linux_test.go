//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func trustedRepeatAddFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/trusted-repeat-add-no-change.json")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	if hex.EncodeToString(digest[:]) != "0e452aed9e122d0f079389218a97990fea9cc110bba5311296938df5cc666e03" {
		t.Fatal("original repeated-add DTO bytes changed")
	}
	return body
}

func TestTrustedPublicOutcomeActualRepeatedAdd(t *testing.T) {
	body := trustedRepeatAddFixture(t)
	if err := trustedPublicOutcome(body, "add", "repeat-add-a", false); err != nil {
		t.Fatalf("actual native-run public no-change response refused: %v", err)
	}
	for _, tc := range []struct {
		command, stage string
		mutated        bool
	}{
		{"add", "add-a", false},
		{"add", "add-a", true},
		{"add", "repeat-add-a", true},
		{"add", "repeat-add-b", false},
		{"update", "repeat-add-a", false},
		{"repair", "repeat-add-a", false},
		{"remove", "repeat-add-a", false},
	} {
		if err := trustedPublicOutcome(body, tc.command, tc.stage, tc.mutated); err == nil {
			t.Fatalf("single no-change result accepted outside exact repeated-add expectation: %+v", tc)
		}
	}
}

func TestTrustedPublicOutcomeRepeatedAddCounterexamples(t *testing.T) {
	body := trustedRepeatAddFixture(t)
	for _, tc := range []struct {
		name, path string
		value      any
		remove     bool
	}{
		{"wrong-command", "command", "update", false},
		{"failure-envelope", "result", "failure", false},
		{"mixed-targets", "data.targets", []any{}, false},
		{"mixed-null-targets", "data.targets", nil, false},
		{"missing-dry-run", "data.dry_run", nil, true},
		{"numeric-dry-run", "data.dry_run", 0, false},
		{"dry-run", "data.dry_run", true, false},
		{"wrong-data-scope", "data.scope", "user", false},
		{"missing-result", "data.result", nil, true},
		{"null-result", "data.result", nil, false},
		{"mutated", "data.result.mutated", true, false},
		{"missing-mutated", "data.result.mutated", nil, true},
		{"numeric-mutated", "data.result.mutated", 0, false},
		{"changed", "data.result.no_change", false, false},
		{"missing-no-change", "data.result.no_change", nil, true},
		{"numeric-no-change", "data.result.no_change", 1, false},
		{"string-no-change", "data.result.no_change", "true", false},
		{"confirmation", "data.result.requires_confirmation", true, false},
		{"missing-confirmation", "data.result.requires_confirmation", nil, true},
		{"numeric-confirmation", "data.result.requires_confirmation", 0, false},
		{"missing-plan", "data.result.plan", nil, true},
		{"wrong-target", "data.result.plan.client_id", "cline", false},
		{"wrong-plan-scope", "data.result.plan.scope", "user", false},
		{"unsupported-plan", "data.result.plan.status", "unsupported", false},
		{"missing-plan-status", "data.result.plan.status", nil, true},
		{"wrong-package-mode", "data.result.plan.package_mode", "native_package", false},
		{"promoted-plan", "data.result.plan.activation", "active", false},
		{"promoted-plan-auth", "data.result.plan.authentication", "authenticated", false},
		{"blocked-plan", "data.result.plan.policy", "blocked", false},
		{"promoted-plan-verification", "data.result.plan.verification", "installation_verified", false},
		{"missing-activation", "data.result.activation", nil, true},
		{"promoted-activation", "data.result.activation.activation", "active", false},
		{"promoted-activation-auth", "data.result.activation.authentication", "authenticated", false},
		{"missing-activation-auth", "data.result.activation.authentication", nil, true},
		{"blocked-activation", "data.result.activation.policy", "blocked", false},
		{"unchecked-activation", "data.result.activation.verification", "not_checked", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := uniqueJSON(body)
			if err != nil {
				t.Fatal(err)
			}
			object := value.(map[string]any)
			parts := strings.Split(tc.path, ".")
			for _, part := range parts[:len(parts)-1] {
				object = object[part].(map[string]any)
			}
			key := parts[len(parts)-1]
			if tc.remove {
				delete(object, key)
			} else {
				object[key] = tc.value
			}
			changed, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := trustedPublicOutcome(changed, "add", "repeat-add-a", false); err == nil {
				t.Fatal("changed original public DTO accepted")
			}
		})
	}
}

func TestTrustedPublicOutcomeBatchRemainsSeparate(t *testing.T) {
	body := []byte(`{"schema_version":1,"command":"add","result":"success","data":{"targets":[{"target":"vscode","output":{"result":{"mutated":true}}}]}}`)
	if err := trustedPublicOutcome(body, "add", "add-a", true); err != nil {
		t.Fatal(err)
	}
	if err := trustedPublicOutcome(body, "add", "repeat-add-a", false); err == nil {
		t.Fatal("repeated add accepted batch instead of actual single no-change DTO")
	}
	if err := trustedPublicOutcome(body, "update", "update-a", true); err == nil {
		t.Fatal("command not bound to selected public argv")
	}
	wrong := strings.Replace(string(body), `"target":"vscode"`, `"target":"cline"`, 1)
	if err := trustedPublicOutcome([]byte(wrong), "add", "add-a", true); err == nil {
		t.Fatal("foreign target accepted")
	}
}

func TestPublicCommandTreeReapedIndependentOfCLIExit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result ports.CommandResult
		want   bool
	}{
		{"nonzero-reaped", ports.CommandResult{ExitCode: 1, CleanupComplete: true}, true},
		{"zero-reaped", ports.CommandResult{ExitCode: 0, CleanupComplete: true}, true},
		{"nonzero-unproved", ports.CommandResult{ExitCode: 1}, false},
		{"never-started", ports.CommandResult{CleanupComplete: true, NoChildStarted: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicCommandTreeReaped(tc.result); got != tc.want {
				t.Fatalf("cleanup proof differs: got %t, want %t", got, tc.want)
			}
		})
	}
}
