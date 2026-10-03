package vscodelocal_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Red: original compiling adapter repeatedly commits state for an empty unowned route.
// Boundary: unchanged independent public Engine reproduction, fresh TEST filesystem.
func TestIndependentLocalNeitherSelected(t *testing.T) {
	f := freshLocal(t, true)
	f.config.NativeStop = false
	f.config.HookSpecs = nil
	f.config.QualifiedTuple.TargetShell = ""
	f.config.TargetShell.Shell = ""
	a, err := vscode.NewLocal(f.config)
	must(t, err)
	*f.adapter = *a
	before := readLocal(t, f.settings)
	engine := f.engine(t, true)
	installed := applyLocal(t, engine, f.request(installer.OpInstall, ""))
	binding := onlyLocalBinding(t, loadLocalState(t, f.state))
	if string(before) != string(readLocal(t, f.settings)) || binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) || binding.LocalEntryObservation != nil {
		t.Fatal("neither selection invented native effect or ownership")
	}
	for _, path := range []string{hookPath(), "mcp.json", "skills/notify/SKILL.md"} {
		if _, err := os.Lstat(filepath.Join(installed.Binding.TargetPath, path)); !os.IsNotExist(err) {
			t.Fatal("unselected component survived", path, err)
		}
	}
	// A prepared-only repeat must remain read-only without manufacturing ownership.
	stateBefore := readLocal(t, filepath.Join(f.state, "state-v2.json"))
	repeat := applyLocal(t, engine, f.request(installer.OpInstall, installed.InstallationID))
	t.Logf("prepared-only repeat mutated=%v native_unchanged=%v state_before=%s state_after=%s native_objects=%v", repeat.Mutated, string(before) == string(readLocal(t, f.settings)), testDigest(stateBefore), testDigest(readLocal(t, filepath.Join(f.state, "state-v2.json"))), binding.NativeObjects)
	if repeat.Mutated || string(before) != string(readLocal(t, f.settings)) {
		t.Fatal("prepared-only repeat changed profile/state")
	}
	if !bytes.Equal(stateBefore, readLocal(t, filepath.Join(f.state, "state-v2.json"))) {
		t.Fatal("prepared-only repeat changed exact durable bytes")
	}
	after := onlyLocalBinding(t, loadLocalState(t, f.state))
	if len(after.NativeObjects) != 1 || after.NativeObjects[0].Kind != "managed_package_directory" || after.PendingNativeIntent != nil || after.NativeActivationAttempt != "" || after.LocalEntryObservation != nil {
		t.Fatal("empty unowned route manufactured native authority")
	}
}

// Red: deselecting the last contribution loses an already owned selector or
// rewrites a disabled profile. Boundary: public refresh/maintenance/state and
// actual NewLocal inspector; previous durable objects remain the only authority.
// 040 has no confirmed-ownership projector input: retain this strict no-op
// assertion for the brokered carrier composition, rather than accepting churn.
func TestLocalOwnedToEmptyRetainsSelector(t *testing.T) {
	t.Run("historical-nil", func(t *testing.T) { assertHistoricalAbsentRefusal(t, true) })
	f := freshLocal(t, false)
	engine := f.engine(t, true)
	installed := applyLocal(t, engine, f.request(installer.OpInstall, ""))
	prior := onlyLocalBinding(t, loadLocalState(t, f.state))
	facts, _ := prior.SelectedDelivery.LocalFacts()
	before := strings.Replace(string(readLocal(t, f.settings)), facts.Registration.Selector+`":true`, facts.Registration.Selector+`":false`, 1)
	if before == string(readLocal(t, f.settings)) {
		t.Fatal("fixture did not disable owned selector")
	}
	writeLocal(t, f.settings, []byte(before), 0600)
	f.config.NativeStop = false
	f.config.HookSpecs = nil
	// Keep the same qualified shell: this changes only the explicit component
	// selection, not the physical selector, desired bool or qualification basis.
	a, err := vscode.NewLocal(f.config)
	must(t, err)
	*f.adapter = *a
	applyLocal(t, engine, f.request(installer.OpRefreshProjection, installed.InstallationID))
	for _, op := range []installer.Operation{installer.OpInstall, installer.OpRepair} {
		stateBefore := readLocal(t, filepath.Join(f.state, "state-v2.json"))
		result := applyLocal(t, engine, f.request(op, installed.InstallationID))
		binding := onlyLocalBinding(t, loadLocalState(t, f.state))
		current, _ := binding.SelectedDelivery.LocalFacts()
		if current.NativeStop || len(current.MCPServers) != 0 || len(current.Skills) != 0 || current.Registration.ObjectID != facts.Registration.ObjectID || !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
			t.Fatal("empty transition lost exact previously owned selector")
		}
		if string(readLocal(t, f.settings)) != before || binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" {
			t.Fatal("maintenance changed disabled bytes or retained pending attempt")
		}
		observed, err := f.adapter.InspectRegistration(t.Context(), nativeconfig.New(), binding.SelectedDelivery, binding.NativeObjects)
		must(t, err)
		if observed != vscode.RegistrationDisabled {
			t.Fatal("retained receipt no longer owns disabled selector", observed)
		}
		t.Logf("owned-empty %s mutated=%v; exact profile/selector ownership retained; state_before=%s state_after=%s", op, result.Mutated, testDigest(stateBefore), testDigest(readLocal(t, filepath.Join(f.state, "state-v2.json"))))
		if result.Mutated || !bytes.Equal(stateBefore, readLocal(t, filepath.Join(f.state, "state-v2.json"))) {
			t.Errorf("owned-empty %s changed durable state: confirmed ownership must reach the existing projector", op)
		}
	}
	binding := assertLocalObservation(t, f, false)
	absent := []byte(`{"chat.pluginLocations":{},"TEST-late-foreign":true}`)
	writeLocal(t, f.settings, absent, 0600)
	applyLocal(t, engine, f.request(installer.OpRepair, installed.InstallationID))
	assertLocalObservation(t, f, false)
	if !strings.Contains(string(readLocal(t, f.settings)), quoteLocal(facts.Registration.Selector)+":false") || !strings.Contains(string(readLocal(t, f.settings)), `"TEST-late-foreign":true`) {
		t.Fatal("owned empty repair lost false/foreign bytes")
	}
	if !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
		t.Fatal("lost prior authority")
	}
	assertStableLocal(t, f, engine, installed.InstallationID, false)
}
