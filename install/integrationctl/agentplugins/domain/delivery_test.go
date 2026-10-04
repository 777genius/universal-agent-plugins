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

// Breaking behavior: per-attempt sealing discards the pure receipt/basis,
// static comparison permits a different command, or zero receipt becomes proof.
func TestSelectedCursorPacketSealingAndAuthority(t *testing.T) {
	root := t.TempDir()
	digest := "sha256:" + strings.Repeat("a", 64)
	r := CursorHookReceipt{Version: 1, Event: "stop", Executable: filepath.Join(root, "observer"), Selector: filepath.Join(root, "binding"), Shell: "cursor-linux-user-3.22.12-single-quote", EntryDigest: digest, RemainderDigest: digest}
	f := CursorDeliveryFacts{ProfileRoot: root, HooksPath: filepath.Join(root, "hooks.json"), ProfileIdentity: "TEST-profile", CursorVersion: "2026.09.28-64d2043", TargetOS: "linux", TargetArch: "amd64", QualificationID: "TEST-contract", Executable: r.Executable, Selector: r.Selector, Shell: r.Shell, ObjectID: "TEST-stop", EntryDigest: digest, CanonicalDigest: digest, PlannedReceipt: r, OriginalExists: true, OriginalRawDigest: digest}
	selected, err := NewCursorDelivery(f)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := selected.WithProjectionDigest(digest)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := sealed.CursorFacts()
	if got.PlannedReceipt != r || got.OriginalRawDigest != f.OriginalRawDigest || !got.OriginalExists {
		t.Fatal("sealing lost attempt proof")
	}
	raw, err := json.Marshal(sealed)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SelectedDelivery
	if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, sealed) {
		t.Fatalf("value packet roundtrip: %v", err)
	}
	changed := got
	changed.OriginalExists = false
	changed.OriginalRawDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	changed.PlannedReceipt.RemainderDigest = "sha256:" + strings.Repeat("b", 64)
	next, err := NewCursorDelivery(changed)
	if err != nil || !selected.SameSelection(next) {
		t.Fatalf("per-attempt basis changed selection: %v", err)
	}
	changed.Executable = filepath.Join(root, "different")
	changed.PlannedReceipt.Executable = changed.Executable
	next, err = NewCursorDelivery(changed)
	if err != nil || selected.SameSelection(next) {
		t.Fatal("static executable could change")
	}
	if selected.OwnsProfileEntry([]NativeObjectOwnership{selected.CursorOwnership(CursorHookReceipt{})}) {
		t.Fatal("zero receipt grants ownership")
	}
	owned := selected.CursorOwnership(r)
	if !selected.OwnsProfileEntry([]NativeObjectOwnership{owned}) || selected.OwnsProfileEntry([]NativeObjectOwnership{owned, owned}) {
		t.Fatal("ownership is absent or duplicated")
	}
	for _, bad := range []string{
		strings.Replace(string(raw), `"original_exists":true,`, "", 1),
		strings.Replace(string(raw), `"original_exists":true`, `"original_exists":true,"ORIGINAL_EXISTS":false`, 1),
		strings.Replace(string(raw), `"version":1`, `"version":1,"future_authority":true`, 1),
	} {
		if err := json.Unmarshal([]byte(bad), &decoded); err == nil {
			t.Fatal("incomplete/shadow/unknown Cursor carrier accepted")
		}
	}
	// Direct decoding has its own duplicate gate, independent of Store scanning.
	for _, tc := range []struct{ field, alias, value string }{
		{"original_raw_digest", "original_raw_digeſt", digest},
		{"shell", "ſhell", r.Shell},
		{"remainder_digest", "remainder_digeſt", digest},
	} {
		t.Run(tc.field, func(t *testing.T) {
			needle := `"` + tc.field + `":"` + tc.value + `"`
			bad := strings.Replace(string(raw), needle, needle+`,"`+tc.alias+`":"`+tc.value+`"`, 1)
			if bad == string(raw) {
				t.Fatal("fixture did not locate authority field")
			}
			before := decoded
			if err := json.Unmarshal([]byte(bad), &decoded); err == nil {
				t.Error("Unicode shadow accepted by direct decode")
			}
			if !reflect.DeepEqual(decoded, before) {
				t.Fatal("rejected decode changed value")
			}
		})
	}
	legacy, err := json.Marshal(NativeObjectOwnership{ObjectID: "TEST-legacy"})
	if err != nil || strings.Contains(string(legacy), "cursor_receipt") {
		t.Fatal("legacy wire gained zero receipt")
	}
}
