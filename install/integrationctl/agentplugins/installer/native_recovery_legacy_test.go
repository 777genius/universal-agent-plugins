package installer

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestInspectNativeIntentsPreservesLegacyUserNativeConfigAttempts(t *testing.T) {
	for _, client := range []string{"cline", "gemini", "windsurf", "kimi", "opencode"} {
		t.Run(client, func(t *testing.T) {
			binding := domain.ClientBinding{ClientID: client, Scope: "user", NativeActivationAttempt: "legacy-attempt"}
			state := domain.StateFileV2{Installations: []domain.Installation{{Clients: map[string]domain.ClientBinding{client: binding}}}}
			before := state.Installations[0].Clients[client]
			pending, err := inspectNativeIntents(state)
			if err != nil || len(pending) != 0 {
				t.Fatalf("inspection = %+v, %v", pending, err)
			}
			if !reflect.DeepEqual(before, state.Installations[0].Clients[client]) {
				t.Fatal("inspection changed legacy binding")
			}
		})
	}
}

func TestInspectNativeIntentsRefusesUnexplainedAttempts(t *testing.T) {
	var selected domain.SelectedDelivery
	if err := json.Unmarshal([]byte(`{"mode":"cursor-native-v1"}`), &selected); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, client, scope string
		selected            bool
	}{
		{"project", "cline", "project", false},
		{"unspecified-scope", "cline", "", false},
		{"unknown", "unknown", "user", false},
		{"directory", "codex", "user", false},
		{"selected", "cline", "user", true},
		{"selected-opencode", "opencode", "user", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binding := domain.ClientBinding{ClientID: tc.client, Scope: tc.scope, NativeActivationAttempt: "unexplained"}
			if tc.selected {
				binding.SelectedDelivery = selected
			}
			if tc.selected && selected.IsZero() {
				t.Fatal("selection fixture is zero")
			}
			state := domain.StateFileV2{Installations: []domain.Installation{{Clients: map[string]domain.ClientBinding{"binding": binding}}}}
			if _, err := inspectNativeIntents(state); err == nil {
				t.Fatal("unexplained attempt accepted")
			}
		})
	}
}
