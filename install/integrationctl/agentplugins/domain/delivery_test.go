package domain

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Red: mutable constructor slices/pointer bools or JSON omitempty can discard
// an explicit disabled choice, widening the authority used after a crash.
func TestSelectedDeliveryJSONRetainsFalseAndFreezesAuthority(t *testing.T) {
	root := t.TempDir()
	disabled := false
	facts := LocalDeliveryFacts{ProfileRoot: root, SettingsPath: filepath.Join(root, "settings.json"), ProfileIdentity: "TEST-profile", SettingsIdentity: "TEST-settings", Tuple: LocalQualifiedTuple{VSCodeVersion: "TEST-code", CopilotVersion: "TEST-copilot", TargetOS: "linux", QualificationID: "TEST-contract"}, MCPServers: []string{"notify"}, Skills: []string{"notify"}, CanonicalDigest: "sha256:" + strings.Repeat("a", 64), Registration: OwnedProfileEntry{ObjectID: "entry", Selector: filepath.Join(root, "plugin"), DesiredValue: &disabled, PreviousValue: &disabled}}
	selected, err := NewLocalDelivery(facts)
	if err != nil {
		t.Fatal(err)
	}
	selected, err = selected.WithProjectionDigest("sha256:" + strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	disabled = true
	facts.MCPServers[0] = "foreign"
	facts.Skills[0] = "foreign"
	copyFacts, _ := selected.LocalFacts()
	*copyFacts.Registration.DesiredValue = true
	copyFacts.MCPServers[0] = "changed"
	binding := ClientBinding{SelectedDelivery: selected}
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"desired_value":false`) || !strings.Contains(string(raw), `"previous_value":false`) {
		t.Fatalf("lost false authority: %s", raw)
	}
	var restored ClientBinding
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	roundtrip, _ := restored.SelectedDelivery.LocalFacts()
	if *roundtrip.Registration.DesiredValue || *roundtrip.Registration.PreviousValue || roundtrip.MCPServers[0] != "notify" || roundtrip.Skills[0] != "notify" || !selected.SameSelection(restored.SelectedDelivery) {
		t.Fatalf("persisted authority changed: %+v", roundtrip)
	}
	// An ordinary copied plan must also remain immutable if its copy is decoded.
	copied := selected
	if err := json.Unmarshal([]byte(`{"mode":"future"}`), &copied); err != nil {
		t.Fatal(err)
	}
	if selected.Mode() != DeliveryVSCodeLocalV1 || copied.Validate() == nil {
		t.Fatal("copy modified the original mode or accepted future authority")
	}
}

// Red: adding a mode must not rewrite the historical shared-CLI traits, while
// Local must track native effects without making native-only delivery need MCP.
func TestSelectedDeliveryHistoricalAndLocalFacts(t *testing.T) {
	var old ClientBinding
	if err := json.Unmarshal([]byte(`{"client_id":"vscode"}`), &old); err != nil {
		t.Fatal(err)
	}
	if !old.SelectedDelivery.IsZero() || !reflect.DeepEqual(old.SelectedDelivery.EffectiveTraits(ClientVSCode), ClientTraitsFor(ClientVSCode)) || !old.SelectedDelivery.SharesBackend(ClientVSCode) {
		t.Fatal("historical CLI policy changed")
	}
	raw, err := json.Marshal(old)
	if err != nil || strings.Contains(string(raw), "selected_delivery") {
		t.Fatalf("rewrote historical wire: %s %v", raw, err)
	}
	root := t.TempDir()
	enabled := true
	selected, err := NewLocalDelivery(LocalDeliveryFacts{ProfileRoot: root, SettingsPath: filepath.Join(root, "settings.json"), ProfileIdentity: "TEST-profile", SettingsIdentity: "TEST-settings", Tuple: LocalQualifiedTuple{VSCodeVersion: "TEST-code", CopilotVersion: "TEST-copilot", TargetOS: "linux", TargetShell: "bash", QualificationID: "TEST-contract"}, NativeStop: true, CanonicalDigest: "sha256:" + strings.Repeat("a", 64), Registration: OwnedProfileEntry{ObjectID: "entry", Selector: filepath.Join(root, "plugin"), DesiredValue: &enabled}})
	if err != nil {
		t.Fatal(err)
	}
	traits := selected.EffectiveTraits(ClientVSCode)
	if !traits.TracksNativeEffects || !traits.BindsNativeProfileRoot || traits.UsesManagedStdioLauncher || selected.SharesBackend(ClientVSCode) {
		t.Fatalf("Local inherited CLI or MCP requirements: %+v", traits)
	}
	var unknown SelectedDelivery
	if err := json.Unmarshal([]byte(`{"mode":"vscode-local-v2"}`), &unknown); err != nil || unknown.Validate() == nil {
		t.Fatalf("unknown mode was discarded or authorized: %v", err)
	}
}
