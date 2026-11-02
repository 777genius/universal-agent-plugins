package pluginkitairepo_test

import "fmt"

// The CLI can reuse one prepared target through its single-result envelope.
// Adapt assertions only; captured raw JSON and the input map remain unchanged.
func openCodeProjectAssertionEnvelope(result map[string]any) (map[string]any, error) {
	data, ok := result["data"].(map[string]any)
	if result["result"] != "success" || result["error"] != nil || !ok || data["error"] != nil {
		return nil, fmt.Errorf("project result is not explicit success")
	}
	_, batch := data["targets"]
	_, single := data["result"]
	if batch == single {
		return nil, fmt.Errorf("project result envelope is mixed or missing")
	}
	if status, exists := data["status"]; batch && status != "completed" || exists && status != "completed" {
		return nil, fmt.Errorf("project result has missing or contradictory completion status")
	}
	if batch {
		targets, ok := data["targets"].([]any)
		if !ok || len(targets) != 1 {
			return nil, fmt.Errorf("project batch requires exactly one target")
		}
		target, ok := targets[0].(map[string]any)
		if !ok || target["target"] != "opencode" || target["status"] != "external_completed" || target["error"] != nil {
			return nil, fmt.Errorf("project batch target did not complete")
		}
		output, _ := target["output"].(map[string]any)
		if output["error"] != nil {
			return nil, fmt.Errorf("project batch output carries an explicit error")
		}
	}
	view := codexProjectAssertionEnvelope(result)
	outcome, err := commandsTargetOutcome(view)
	if err != nil || outcome["error"] != nil {
		return nil, fmt.Errorf("project result has no successful target outcome")
	}
	if single && (outcome["no_change"] != true || outcome["mutated"] != false) {
		return nil, fmt.Errorf("single project replay lacks actual unchanged flags")
	}
	plan, _ := outcome["plan"].(map[string]any)
	activation, _ := outcome["activation"].(map[string]any)
	if plan["client_id"] != "opencode" || plan["scope"] != "project" || activation["activation"] != "prepared" || activation["verification"] != "installation_verified" {
		return nil, fmt.Errorf("project result has contradictory client, scope or Prepared identity")
	}
	for _, key := range []string{"activation_attested", "authentication_attested"} {
		if value, exists := activation[key]; exists && value != false {
			return nil, fmt.Errorf("project result carries malformed or positive attestation")
		}
	}
	return view, nil
}
