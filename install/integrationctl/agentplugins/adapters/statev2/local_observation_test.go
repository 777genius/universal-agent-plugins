package statev2

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

func observedStateFixture(t *testing.T) (domain.StateFileV2, string) {
	t.Helper()
	root := t.TempDir()
	desired := true
	f := domain.LocalDeliveryFacts{ProfileRoot: root, SettingsPath: filepath.Join(root, "settings.json"), ProfileIdentity: "TEST-profile", SettingsIdentity: "TEST-settings", Tuple: domain.LocalQualifiedTuple{VSCodeVersion: "TEST-code", CopilotVersion: "TEST-copilot", TargetOS: "linux", QualificationID: "TEST-state"}, CanonicalDigest: "sha256:" + strings.Repeat("a", 64), ProjectionDigest: "sha256:" + strings.Repeat("b", 64), Registration: domain.OwnedProfileEntry{ObjectID: "TEST-entry", Selector: filepath.Join(root, "plugin"), DesiredValue: &desired}}
	selected, err := domain.NewLocalDelivery(f)
	if err != nil {
		t.Fatal(err)
	}
	id := vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}
	installed, err := vp.Plan(vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := vp.VerifyRecordedEntry(bytes.Replace(installed.Settings, []byte("true"), []byte("false"), 1), id, true)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{RevisionBasis: selected, Enabled: false, ReceiptDigest: verified.Receipt.Digest})
	if err != nil {
		t.Fatal(err)
	}
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "TEST-source", "TEST-artifact")
	installation.OriginMode = domain.OriginModeDirect
	var key string
	for k, binding := range installation.Clients {
		key = k
		binding.ClientID = "vscode"
		binding.SelectedDelivery = selected
		binding.TargetLocator = f.Registration.Selector
		binding.NativeProfileRoot = root
		binding.LocalEntryObservation = observation
		binding.NativeObjects = []domain.NativeObjectOwnership{f.Registration.Ownership(f.SettingsPath)}
		binding.PackageRevision = &domain.ClientPackageRevision{TreeDigest: f.CanonicalDigest, ManifestDigest: "sha256:" + strings.Repeat("c", 64)}
		binding.NativeActivationAttempt = "TEST-attempt"
		binding.PendingNativeIntent = &domain.PendingNativeIntent{AttemptID: "TEST-attempt", Direction: domain.NativeIntentRegister, Delivery: selected, LocalEntryObservation: observation.Clone()}
		installation.Clients[k] = binding
	}
	return domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}, key
}

// Regression/gap: an earlier receipt can disappear before typed state decoding;
// exercise actual files at each carrier and ancestor, not a decoder mock.
func TestObservedStateCarrierShadowing(t *testing.T) {
	state, key := observedStateFixture(t)
	store := Store{Path: filepath.Join(t.TempDir(), "state.json")}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := loaded.Installations[0].Clients[key]
	if binding.LocalEntryObservation == nil || binding.LocalEntryObservation.Facts().Enabled || !binding.PendingNativeIntent.LocalEntryObservation.Equal(binding.LocalEntryObservation) {
		t.Fatal("durable false/predecessor lost")
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	installationRaw, _ := json.Marshal(state.Installations[0])
	clientRaw, _ := json.Marshal(state.Installations[0].Clients[key])
	clientsRaw, _ := json.Marshal(state.Installations[0].Clients)
	observationRaw, _ := json.Marshal(binding.LocalEntryObservation)
	intentRaw, _ := json.Marshal(binding.PendingNativeIntent)
	cases := [][]byte{
		bytes.Replace(raw, append([]byte(`"local_entry_observation":`), observationRaw...), append(append([]byte(`"local_entry_observation":`), observationRaw...), []byte(`,"local_entry_observation":null`)...), 1),
		bytes.Replace(raw, []byte(`"clients":`+string(clientsRaw)), []byte(`"clients":`+string(clientsRaw)+`,"clients":{}`), 1),
		bytes.Replace(raw, []byte(`"`+key+`":`+string(clientRaw)), []byte(`"`+key+`":`+string(clientRaw)+`,"`+key+`":null`), 1),
		[]byte(`{"schema_version":4,"installations":[` + string(installationRaw) + `],"installations":[]}`),
		bytes.Replace(raw, []byte(`"pending_native_intent":`+string(intentRaw)), []byte(`"pending_native_intent":`+string(intentRaw)+`,"pending_native_intent":null`), 1),
	}
	for i, body := range cases {
		if bytes.Equal(raw, body) {
			t.Fatalf("case %d did not change fixture", i)
		}
		if err := os.WriteFile(store.Path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("shadowed carrier %d accepted: %v", i, err)
		}
	}

	// A first-confirmed receipt may have no pending intent to expose shadowing.
	aliasState := state
	aliasState.Installations = append([]domain.Installation(nil), state.Installations...)
	aliasBinding := binding
	aliasBinding.PendingNativeIntent = nil
	aliasBinding.NativeActivationAttempt = ""
	aliasState.Installations[0].Clients = map[string]domain.ClientBinding{key: aliasBinding}
	aliasRaw, err := json.Marshal(aliasState)
	if err != nil {
		t.Fatal(err)
	}
	aliasInstallation, err := json.Marshal(aliasState.Installations[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path, aliasRaw, 0600); err != nil {
		t.Fatal(err)
	}
	if valid, err := store.Load(); err != nil || valid.Installations[0].Clients[key].LocalEntryObservation == nil {
		t.Fatalf("valid false alias control: %v", err)
	}
	// Unicode tags accepted by encoding/json must share the duplicate fence.
	for _, shadow := range []struct{ name, body string }{
		{"carrier-long-s", strings.Replace(string(aliasRaw), `"local_entry_observation":`+string(observationRaw), `"local_entry_observation":`+string(observationRaw)+`,"local_entry_obſervation":null`, 1)},
		{"ancestor-long-s", `{"schema_version":4,"installations":[` + string(aliasInstallation) + `],"inſtallations":[]}`},
	} {
		t.Run(shadow.name, func(t *testing.T) {
			body := []byte(shadow.body)
			if err := os.WriteFile(store.Path, body, 0600); err != nil {
				t.Fatal(err)
			}
			_, loadErr := store.Load()
			unchanged, err := os.ReadFile(store.Path)
			if err != nil || !bytes.Equal(body, unchanged) {
				t.Fatal("refused Load changed state bytes")
			}
			if loadErr == nil {
				t.Fatal("Unicode shadow accepted")
			}
		})
	}
	// Bounds belong to the observation, not the historical state document.
	padded := append(bytes.Clone(raw[:len(raw)-1]), bytes.Repeat([]byte(" "), 1<<20)...)
	padded = append(padded, '}')
	if err := os.WriteFile(store.Path, padded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("full-state byte budget retrofitted: %v", err)
	}
	for _, change := range []func(*domain.ClientBinding){
		func(b *domain.ClientBinding) { b.NativeProfileRoot = filepath.Dir(b.NativeProfileRoot) },
		func(b *domain.ClientBinding) { b.NativeObjects = nil },
		func(b *domain.ClientBinding) { b.PendingNativeIntent.LocalEntryObservation = nil },
	} {
		bad := binding
		bad.PendingNativeIntent = binding.PendingNativeIntent.Clone()
		change(&bad)
		state.Installations[0].Clients[key] = bad
		if err := store.Save(state); err == nil {
			t.Fatal("invalid observation linkage saved")
		}
		body, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(store.Path, body, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Load(); err == nil {
			t.Fatal("invalid observation linkage loaded")
		}
	}
	// Directly authored historical fixture, never stripping a supported receipt.
	legacy := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{validInstallation("00000000-0000-4000-8000-000000000002", "TEST-legacy", "TEST-legacy-artifact")}}
	legacy.Installations[0].OriginMode = domain.OriginModeDirect
	legacyRaw, _ := json.Marshal(legacy)
	legacyRaw = bytes.Replace(legacyRaw, []byte(`"client_id":"codex"`), []byte(`"client_id":"codex","selected_delivery":{"mode":"future-mode","ignored":1e100000,"local_entry_observation":{"enabled":false},"local_entry_observation":null},"local_entry_observation":null`), 1)
	legacyRaw = bytes.Replace(legacyRaw, []byte(`"schema_version":4`), []byte(`"schema_version":4,"schema_version":4`), 1)
	if err := os.WriteFile(store.Path, legacyRaw, 0600); err != nil {
		t.Fatal(err)
	}
	historical, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range historical.Installations[0].Clients {
		if b.LocalEntryObservation != nil || b.SelectedDelivery.Mode() != "future-mode" {
			t.Fatal("historical nil/unknown evidence changed")
		}
	}
	if err := store.Save(historical); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(store.Path)
	if err != nil || bytes.Contains(saved, []byte("local_entry_observation")) {
		t.Fatal("nil observation emitted on save")
	}
}
