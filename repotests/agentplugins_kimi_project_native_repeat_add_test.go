//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// Only the already-installed single-target add has the public direct result.
// Its authority remains the first independently validated receipt, never the
// installation or physical identity returned by this later no-change command.
func kimiProjectRepeatAddOutcome(label, scope string, mutated bool, args []string, value map[string]any, p *kimiProjectNativePackage) (bool, error) {
	if label != "repeat-add-a" {
		return false, nil
	}
	refuse := func() (bool, error) { return true, fmt.Errorf("Kimi repeated add differs from held no-change result") }
	if scope != "a" || mutated || value["command"] != "add" || p == nil || p.Proof == nil || p.Proof.stable == nil ||
		p.Name == "" || p.Source == "" || p.Proof.stable.installation == "" || p.Proof.stable.artifact == "" ||
		!reflect.DeepEqual(args, []string{"add", p.Source, "--no-save"}) {
		return refuse()
	}
	data, ok := value["data"].(map[string]any)
	if !ok || data["plugin"] != p.Name || data["dry_run"] != false {
		return refuse()
	}
	if _, exists := data["targets"]; exists {
		return refuse()
	}
	outcome, ok := data["result"].(map[string]any)
	if !ok || outcome["installation_id"] != p.Proof.stable.installation || outcome["mutated"] != false ||
		outcome["no_change"] != true || outcome["requires_confirmation"] != false {
		return refuse()
	}
	plan, ok := outcome["plan"].(map[string]any)
	if !ok || plan["physical_artifact_id"] != p.Proof.stable.artifact {
		return refuse()
	}
	for key, expected := range map[string]string{"client_id": "kimi", "scope": "project", "status": "ready", "package_mode": "native", "activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "package_validated"} {
		if plan[key] != expected {
			return refuse()
		}
	}
	activation, ok := outcome["activation"].(map[string]any)
	if !ok {
		return refuse()
	}
	for key, expected := range map[string]string{"activation": "prepared", "authentication": "not_checked", "policy": "allowed", "verification": "installation_verified"} {
		if activation[key] != expected {
			return refuse()
		}
	}
	return true, nil
}

func kimiProjectRepeatAddContract(t *testing.T) (map[string]any, *kimiProjectNativePackage) {
	t.Helper()
	// Independently stated public no-change shape; neither identity is learned
	// from this result. Strict whole JSON parsing and successful versioned-
	// envelope validation remain in f.uap.
	const body = `{"schema_version":1,"command":"add","result":"success","data":{"plugin":"held-package","dry_run":false,"result":{"installation_id":"held-installation","mutated":false,"no_change":true,"requires_confirmation":false,"plan":{"client_id":"kimi","scope":"project","status":"ready","package_mode":"native","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"package_validated","physical_artifact_id":"held-physical"},"activation":{"activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified"}}}}`
	var value map[string]any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatal(err)
	}
	p := &kimiProjectNativePackage{Name: "held-package", Source: "/held/source", Proof: &kimiProjectProof{stable: &kimiProjectProofAuthority{installation: "held-installation", artifact: "held-physical"}}}
	return value, p
}

func TestKimiProjectRepeatAddRequiresHeldDirectNoChange(t *testing.T) {
	value, p := kimiProjectRepeatAddContract(t)
	if handled, err := kimiProjectRepeatAddOutcome("repeat-add-a", "a", false, []string{"add", p.Source, "--no-save"}, value, p); !handled || err != nil {
		t.Fatalf("held direct repeated add refused: %v", err)
	}
	if _, err := commandsTargetOutcome(value); err == nil {
		t.Fatal("shared array-only contract unexpectedly accepts direct result")
	}
	for _, label := range []string{"add-a", "add-b", "add-user", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a"} {
		if handled, err := kimiProjectRepeatAddOutcome(label, "a", false, []string{"add", p.Source, "--no-save"}, value, p); handled || err != nil {
			t.Fatalf("unrelated stage %s selected direct-result branch", label)
		}
	}
}

func TestKimiProjectRepeatAddRefusesIdentityShapeAndMutationChanges(t *testing.T) {
	for _, path := range [][]string{{"command"}, {"data"}, {"data", "plugin"}, {"data", "dry_run"}, {"data", "result"}, {"data", "result", "installation_id"}, {"data", "result", "mutated"}, {"data", "result", "no_change"}, {"data", "result", "requires_confirmation"}, {"data", "result", "plan"}, {"data", "result", "plan", "physical_artifact_id"}, {"data", "result", "plan", "client_id"}, {"data", "result", "plan", "scope"}, {"data", "result", "plan", "status"}, {"data", "result", "plan", "package_mode"}, {"data", "result", "plan", "activation"}, {"data", "result", "plan", "authentication"}, {"data", "result", "plan", "policy"}, {"data", "result", "plan", "verification"}, {"data", "result", "activation"}, {"data", "result", "activation", "activation"}, {"data", "result", "activation", "authentication"}, {"data", "result", "activation", "policy"}, {"data", "result", "activation", "verification"}} {
		for _, replacement := range []any{nil, "wrong", true, false, float64(0), []any{}, map[string]any{}} {
			value, p := kimiProjectRepeatAddContract(t)
			parent := value
			for _, key := range path[:len(path)-1] {
				parent = parent[key].(map[string]any)
			}
			key := path[len(path)-1]
			if reflect.DeepEqual(parent[key], replacement) {
				continue
			}
			if replacement == nil {
				delete(parent, key)
			} else {
				parent[key] = replacement
			}
			if handled, err := kimiProjectRepeatAddOutcome("repeat-add-a", "a", false, []string{"add", p.Source, "--no-save"}, value, p); !handled || err == nil {
				t.Fatalf("changed repeated-add field %v accepted", path)
			}
		}
	}
}

func TestKimiProjectRepeatAddRefusesAmbiguityAndAuthority(t *testing.T) {
	for _, targets := range []any{nil, []any{}, []any{map[string]any{"target": "kimi"}}} {
		value, p := kimiProjectRepeatAddContract(t)
		value["data"].(map[string]any)["targets"] = targets
		if _, err := kimiProjectRepeatAddOutcome("repeat-add-a", "a", false, []string{"add", p.Source, "--no-save"}, value, p); err == nil {
			t.Fatal("ambiguous direct/group result accepted")
		}
	}
	for _, mutate := range []func(*kimiProjectNativePackage){func(p *kimiProjectNativePackage) { p.Proof = nil }, func(p *kimiProjectNativePackage) { p.Proof.stable = nil }, func(p *kimiProjectNativePackage) { p.Proof.stable.installation = "replacement" }, func(p *kimiProjectNativePackage) { p.Proof.stable.artifact = "replacement" }} {
		value, p := kimiProjectRepeatAddContract(t)
		mutate(p)
		if _, err := kimiProjectRepeatAddOutcome("repeat-add-a", "a", false, []string{"add", p.Source, "--no-save"}, value, p); err == nil {
			t.Fatal("missing or rebound first receipt accepted")
		}
	}
	for _, args := range [][]string{nil, {"update", "/held/source", "--no-save"}, {"add", "/other/source", "--no-save"}, {"add", "/held/source"}, {"add", "/held/source", "--no-save", "--dry-run"}} {
		value, p := kimiProjectRepeatAddContract(t)
		if _, err := kimiProjectRepeatAddOutcome("repeat-add-a", "a", false, args, value, p); err == nil {
			t.Fatal("different repeated-add command accepted")
		}
	}
	value, p := kimiProjectRepeatAddContract(t)
	for _, scope := range []string{"user", "b", "c", ""} {
		if _, err := kimiProjectRepeatAddOutcome("repeat-add-a", scope, false, []string{"add", p.Source, "--no-save"}, value, p); err == nil {
			t.Fatal("different repeated-add scope accepted")
		}
	}
	if _, err := kimiProjectRepeatAddOutcome("repeat-add-a", "a", true, []string{"add", p.Source, "--no-save"}, value, p); err == nil {
		t.Fatal("mutating repeated-add stage accepted")
	}
}
