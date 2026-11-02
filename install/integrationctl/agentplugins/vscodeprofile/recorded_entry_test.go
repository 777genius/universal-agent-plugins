package vscodeprofile_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	vp "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

// VerifyOwned exposes only status; the new boundary must return the actual
// parsed receipt without rewriting JSONC, adopting absence or re-enabling false.
func TestVerifyRecordedEntryNativeSnapshot(t *testing.T) {
	id := identity()
	installed := plan(t, vp.Request{Settings: []byte(`{/* TEST foreign */"foreign":42,"chat.pluginLocations":{"/foreign":false,},}`), Identity: id, Action: vp.Install})
	disabled := bytes.Replace(installed.Settings, []byte(`: true`), []byte(`: false`), 1)
	if bytes.Equal(disabled, installed.Settings) {
		disabled = bytes.Replace(installed.Settings, []byte(`:true`), []byte(`:false`), 1)
	}
	if bytes.Equal(disabled, installed.Settings) {
		t.Fatal("native false fixture unchanged")
	}
	result, err := vp.VerifyRecordedEntry(disabled, id, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || !result.Disabled || result.Receipt == nil || result.Receipt.Enabled || !bytes.Equal(result.Settings, disabled) || result.BeforeDigest != result.AfterDigest {
		t.Fatal("verification changed bytes or discarded native false")
	}
	if _, err := vp.VerifyOwned(disabled, id, result.Receipt); err != nil {
		t.Fatal(err)
	}
	absent := []byte(`{/* late sibling */"foreign":43,"chat.pluginLocations":{"/foreign":false}}`)
	restored := plan(t, vp.Request{Settings: absent, Identity: id, Previous: result.Receipt, Action: vp.Repair})
	if restored.Receipt.Enabled || !bytes.Contains(restored.Settings, []byte(`/* late sibling */`)) || decoded(t, restored.Settings)["chat.pluginLocations"].(map[string]any)[id.PluginRoot] != false {
		t.Fatal("recorded false not restored with siblings")
	}
	for _, body := range [][]byte{absent, bytes.ReplaceAll(disabled, []byte(`false`), []byte(`null`)), installed.Settings} {
		refused, err := vp.VerifyRecordedEntry(body, id, false)
		if errors.Is(err, vp.ErrRecordedEntryAbsent) != bytes.Equal(body, absent) {
			t.Fatalf("parser absence conflated with other conflict: %v", err)
		}
		if !errors.Is(err, vp.ErrConflict) || refused.Receipt != nil || refused.Changed || !bytes.Equal(refused.Settings, body) {
			t.Fatalf("invalid recorded false verified: %v", err)
		}
	}
	result.Settings[0] = ' '
	if disabled[0] != '{' {
		t.Fatal("result aliases native input")
	}
	malformed := id
	malformed.ProjectionDigest = "unsealed"
	if result, err := vp.VerifyRecordedEntry(disabled, malformed, true); err == nil || result.Receipt != nil {
		t.Fatal("invalid identity verified")
	}
}

func observedFixture(t *testing.T) (*domain.LocalEntryObservation, domain.LocalDeliveryFacts) {
	t.Helper()
	root := t.TempDir()
	desired := true
	f := domain.LocalDeliveryFacts{ProfileRoot: root, SettingsPath: filepath.Join(root, "settings.json"), ProfileIdentity: "TEST-profile", SettingsIdentity: "TEST-settings", Tuple: domain.LocalQualifiedTuple{VSCodeVersion: "TEST-code", CopilotVersion: "TEST-copilot", TargetOS: "linux", TargetShell: "bash", QualificationID: "TEST-observed"}, NativeStop: true, MCPServers: []string{"notify"}, Skills: []string{"notify"}, CanonicalDigest: "sha256:" + strings.Repeat("a", 64), ProjectionDigest: "sha256:" + strings.Repeat("b", 64), Registration: domain.OwnedProfileEntry{ObjectID: "TEST-entry", Selector: filepath.Join(root, "plugin"), DesiredValue: &desired}}
	basis, err := domain.NewLocalDelivery(f)
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
	observation, err := domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{RevisionBasis: basis, Enabled: false, ReceiptDigest: verified.Receipt.Digest})
	if err != nil {
		t.Fatal(err)
	}
	return observation, f
}

// Regression/gap recorded before authoring in .research/s1/test-contracts.md:
// observation accessors or bool omission can lose a verified native false.
func TestLocalEntryObservationImmutableFalse(t *testing.T) {
	observation, input := observedFixture(t)
	original := observation.Clone()
	*input.Registration.DesiredValue = false
	input.MCPServers[0] = "foreign"
	facts := observation.Facts()
	local, _ := facts.RevisionBasis.LocalFacts()
	*local.Registration.DesiredValue = false
	local.Skills[0] = "foreign"
	if !observation.Equal(original) {
		t.Fatal("accessor or constructor mutation changed observation")
	}
	raw, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	var restored *domain.LocalEntryObservation
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	desired, _ := restored.Facts().RevisionBasis.LocalFacts()
	if restored.Facts().Enabled || !*desired.Registration.DesiredValue || !restored.Equal(observation) {
		t.Fatal("false confused with desired true or revision changed")
	}
	clone := observation.Clone()
	trueFacts := observation.Facts()
	trueFacts.Enabled = true
	f, _ := trueFacts.RevisionBasis.LocalFacts()
	id := vp.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: f.ProjectionDigest}
	installed, err := vp.Plan(vp.Request{Settings: []byte(`{}`), Identity: id, Action: vp.Install})
	if err != nil {
		t.Fatal(err)
	}
	trueFacts.ReceiptDigest = installed.Receipt.Digest
	enabled, err := domain.NewLocalEntryObservation(trueFacts)
	if err != nil {
		t.Fatal(err)
	}
	enabledRaw, err := json.Marshal(enabled)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(enabledRaw, clone); err != nil {
		t.Fatal(err)
	}
	if clone.Equal(observation) || observation.Facts().Enabled {
		t.Fatal("clone decode changed original false authority")
	}
	var absent *domain.LocalEntryObservation
	if absent.Clone() != nil || absent.Equal(observation) || !absent.Equal(nil) {
		t.Fatal("nil semantics changed")
	}
}

// The observation wire must refuse incomplete bool/basis, duplicates, unknown
// nested fields and corrupt receipt integrity without weakening legacy delivery.
func TestLocalEntryObservationStrictWire(t *testing.T) {
	observation, _ := observedFixture(t)
	raw, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	replacements := [][2]string{
		{`,"enabled":false`, ""}, {`"enabled":false`, `"enabled":null`}, {`"enabled":false`, `"enabled":"false"`},
		{`"enabled":false`, `"enabled":false,"enabled":true`}, {`"enabled":false`, `"enabled":false,"ENABLED":false`},
		{`"mode":"vscode-local-v1"`, `"mode":"vscode-local-v1","extra":0`},
		{`"profile_root":`, `"unknown":0,"profile_root":`}, {`"qualified_tuple":{`, `"qualified_tuple":{"unknown":0,`},
		{`"registration":{`, `"registration":{"unknown":0,`}, {`"native_stop":true`, `"native_stop":true,"native_stop":false`},
		{`"object_id":"TEST-entry"`, `"object_id":"TEST-entry","object_id":"foreign"`},
		{`"profile_identity":"TEST-profile"`, `"profile_identity":"\ud800"`},
		{`"canonical_digest":"sha256:aaa`, `"canonical_digest":"sha256:caa`},
		{`"receipt_digest":"sha256:`, `"receipt_digest":"sha256:0`},
	}
	for i, pair := range replacements {
		changed := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
		if bytes.Equal(raw, changed) {
			t.Fatalf("case %d did not change fixture", i)
		}
		preserved := observation.Clone()
		if err := json.Unmarshal(changed, preserved); err == nil {
			t.Fatalf("case %d accepted", i)
		}
		if !preserved.Equal(observation) {
			t.Fatalf("case %d changed target on failed decode", i)
		}
	}
	exact := append(bytes.Clone(raw[:len(raw)-1]), bytes.Repeat([]byte(" "), (1<<20)-len(raw))...)
	exact = append(exact, '}')
	var decoded domain.LocalEntryObservation
	if err := json.Unmarshal(exact, &decoded); err != nil {
		t.Fatalf("subtree byte boundary refused: %v", err)
	}
	over := append(bytes.Clone(exact[:len(exact)-1]), ' ', '}')
	if err := json.Unmarshal(over, &decoded); err == nil {
		t.Fatal("observation byte limit not enforced")
	}
	deep := []byte(`{"revision_basis":{"local":{"skills":` + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + `}}}`)
	if err := json.Unmarshal(deep, &decoded); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("depth ceiling not enforced before typed decoding: %v", err)
	}
	if err := decoded.UnmarshalJSON(append(bytes.Clone(raw), []byte(` {}`)...)); err == nil {
		t.Fatal("trailing value accepted")
	}
	// A valid receipt must not serialize into a state the same reader refuses.
	largeFacts, _ := observation.Facts().RevisionBasis.LocalFacts()
	largeFacts.Skills = make([]string, 10000)
	for i := range largeFacts.Skills {
		largeFacts.Skills[i] = fmt.Sprintf("test-%08d-%s", i, strings.Repeat("a", 128))
	}
	largeBasis, err := domain.NewLocalDelivery(largeFacts)
	if err != nil {
		t.Fatal(err)
	}
	id := vp.Identity{SettingsPath: largeFacts.SettingsPath, ProfileID: largeFacts.ProfileIdentity, PluginRoot: largeFacts.Registration.Selector, PackageID: largeFacts.Registration.ObjectID, PackageDigest: largeFacts.CanonicalDigest, ProjectionDigest: largeFacts.ProjectionDigest}
	selector, err := json.Marshal(id.PluginRoot)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := vp.VerifyRecordedEntry([]byte(`{"chat.pluginLocations":{`+string(selector)+`:false}}`), id, true)
	if err != nil {
		t.Fatal(err)
	}
	large, err := domain.NewLocalEntryObservation(domain.LocalEntryObservationFacts{RevisionBasis: largeBasis, Enabled: false, ReceiptDigest: verified.Receipt.Digest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(large); err == nil {
		t.Fatal("unreadable observation encoding allowed")
	}
}

// Regression/gap: callback absence classification must not match a different
// sealed revision or historical nil. Use an actual parser-generated receipt.
func TestLocalEntryAbsenceExactRecordedRevision(t *testing.T) {
	recorded, input := observedFixture(t)
	basis := recorded.Facts().RevisionBasis
	absent, err := domain.NewLocalEntryAbsence(basis)
	if err != nil {
		t.Fatal(err)
	}
	*input.Registration.DesiredValue = false
	input.Skills[0] = "foreign"
	if !absent.Matches(recorded) || absent.Matches(nil) || (&domain.LocalEntryAbsence{}).Matches(recorded) {
		t.Fatal("classification aliases inputs or grants nil/zero authority")
	}
	changed, _ := basis.LocalFacts()
	changed.Tuple.QualificationID += "-changed"
	other, err := domain.NewLocalDelivery(changed)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := domain.NewLocalEntryAbsence(other)
	if err != nil {
		t.Fatal(err)
	}
	if foreign.Matches(recorded) {
		t.Fatal("absence matched different recorded revision")
	}
	changed.ProjectionDigest = ""
	unsealed, err := domain.NewLocalDelivery(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := domain.NewLocalEntryAbsence(unsealed); err == nil {
		t.Fatal("unsealed classification accepted")
	}
	if _, err := domain.NewLocalEntryAbsence(domain.SelectedDelivery{}); err == nil {
		t.Fatal("historical nil classified as recorded absence")
	}
}
