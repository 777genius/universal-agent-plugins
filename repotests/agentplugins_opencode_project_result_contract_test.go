package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOpenCodeProjectAssertionEnvelopePreservesActualResults(t *testing.T) {
	for _, batch := range []bool{false, true} {
		raw := openCodeProjectResultFixture(batch)
		if batch {
			outcome, _ := commandsTargetOutcome(raw)
			outcome["mutated"] = true
			delete(outcome, "no_change")
		}
		before, _ := json.Marshal(raw)
		view, err := openCodeProjectAssertionEnvelope(raw)
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := commandsTargetOutcome(view)
		if err != nil || outcome["mutated"] != batch || !batch && outcome["no_change"] != true {
			t.Fatal("assertion view lost actual replay fields", err)
		}
		if _, exists := outcome["no_change"]; batch && exists {
			t.Fatal("assertion view invented a batch replay flag")
		}
		after, _ := json.Marshal(raw)
		if !bytes.Equal(before, after) {
			t.Fatal("assertion adapter mutated captured input")
		}
	}
}

func TestOpenCodeProjectAssertionEnvelopeRefusesContradictoryResults(t *testing.T) {
	for _, name := range []string{"mixed", "missing", "failed", "status", "error", "top-level error", "targets", "multiple", "target", "target status", "client", "scope", "activation", "verification", "attestation", "malformed attestation", "missing no_change", "typed no_change", "missing mutated", "typed mutated"} {
		t.Run(name, func(t *testing.T) {
			batch := name == "targets" || name == "multiple" || name == "target" || name == "target status"
			raw := openCodeProjectResultFixture(batch)
			data := raw["data"].(map[string]any)
			view := codexProjectAssertionEnvelope(raw)
			outcome, _ := commandsTargetOutcome(view)
			plan := outcome["plan"].(map[string]any)
			activation := outcome["activation"].(map[string]any)
			switch name {
			case "mixed":
				data["targets"] = []any{}
			case "missing":
				delete(data, "result")
			case "failed":
				raw["result"] = "error"
			case "status":
				data["status"] = "managed_committed_activation_failed"
			case "error":
				data["error"] = "actual failure"
			case "top-level error":
				raw["error"] = "actual failure"
			case "targets":
				data["targets"] = "malformed"
			case "multiple":
				data["targets"] = append(data["targets"].([]any), data["targets"].([]any)[0])
			case "target":
				data["targets"].([]any)[0].(map[string]any)["target"] = "codex"
			case "target status":
				data["targets"].([]any)[0].(map[string]any)["status"] = "external_failed"
			case "client":
				plan["client_id"] = "codex"
			case "scope":
				plan["scope"] = "user"
			case "activation":
				activation["activation"] = "active"
			case "verification":
				activation["verification"] = "package_validated"
			case "attestation":
				activation["activation_attested"] = true
			case "malformed attestation":
				activation["activation_attested"] = "false"
			case "missing no_change":
				delete(outcome, "no_change")
			case "typed no_change":
				outcome["no_change"] = "true"
			case "missing mutated":
				delete(outcome, "mutated")
			case "typed mutated":
				outcome["mutated"] = "false"
			}
			if _, err := openCodeProjectAssertionEnvelope(raw); err == nil {
				t.Fatal("contradictory project result gained assertion authority")
			}
		})
	}
}

func openCodeProjectResultFixture(batch bool) map[string]any {
	outcome := map[string]any{"no_change": true, "mutated": false,
		"plan":       map[string]any{"client_id": "opencode", "scope": "project"},
		"activation": map[string]any{"activation": "prepared", "verification": "installation_verified"}}
	data := map[string]any{"result": outcome}
	if batch {
		data = map[string]any{"status": "completed", "targets": []any{map[string]any{"target": "opencode", "status": "external_completed", "output": map[string]any{"result": outcome}}}}
	}
	return map[string]any{"result": "success", "data": data}
}
