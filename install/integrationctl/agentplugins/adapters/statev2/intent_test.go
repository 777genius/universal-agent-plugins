package statev2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestInstallIntentStateRoundtripAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, client string
		intent       domain.InstallIntent
		valid        bool
	}{
		{"historical empty", "kiro", "", true}, {"prepared", "kiro", domain.InstallIntentPrepare, true},
		{"unknown", "kiro", "automatic", false}, {"malformed", "kiro", "PREPARE", false}, {"unsupported target", "codex", domain.InstallIntentPrepare, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
			installation := validInstallation("00000000-0000-4000-8000-000000000001", "source", "demo-id")
			for key, binding := range installation.Clients {
				binding.ClientID = tc.client
				binding.InstallIntent = tc.intent
				installation.Clients[key] = binding
			}
			state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
			err := store.Save(state)
			if (err == nil) != tc.valid {
				t.Fatalf("Save: %v", err)
			}
			// Exercise the on-disk decoder separately from Save's validation.
			body, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.Path, body, 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := store.Load()
			if (err == nil) != tc.valid {
				t.Fatalf("Load: %v", err)
			}
			if tc.valid {
				for _, binding := range loaded.Installations[0].Clients {
					if binding.InstallIntent != tc.intent {
						t.Fatalf("intent: %q", binding.InstallIntent)
					}
				}
			}
		})
	}
}
