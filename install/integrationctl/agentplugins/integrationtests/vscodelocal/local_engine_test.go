package vscodelocal_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Red: actual NewLocal silently uses CLI registration, changes another profile,
// loses source/projection identity or re-enables a native false during maintenance.
// Boundary: public Engine with genuine recorded TEST executable (not missing helper).
func TestLocalPublicEngineMaintenanceAndTwoProfiles(t *testing.T) {
	f := freshLocal(t, true)
	engine := f.engine(t, true)
	installed := applyLocal(t, engine, f.request(installer.OpInstall, ""))
	if installed.Binding.TreeDigest == "" || installed.Binding.SelectedDelivery.IsZero() {
		t.Fatal("selected committed handoff absent")
	}
	facts, ok := installed.Binding.SelectedDelivery.LocalFacts()
	if !ok || facts.ProjectionDigest == facts.CanonicalDigest || facts.SettingsPath != f.settings {
		t.Fatal("committed selected facts differ")
	}
	assertForeign(t, readLocal(t, f.settings))
	repeat := applyLocal(t, engine, f.request(installer.OpInstall, installed.InstallationID))
	if repeat.Mutated {
		t.Fatalf("repeat mutated registration: %+v", repeat)
	}
	other := freshLocal(t, false)
	applyLocal(t, other.engine(t, true), other.request(installer.OpInstall, ""))
	beforeOther := readLocal(t, other.settings)
	nativeFalse := strings.Replace(string(readLocal(t, f.settings)), facts.Registration.Selector+`":true`, facts.Registration.Selector+`":false`, 1)
	if nativeFalse == string(readLocal(t, f.settings)) {
		t.Fatal("fixture failed to disable exact native entry")
	}
	writeLocal(t, f.settings, []byte(nativeFalse), 0600)
	for _, op := range []installer.Operation{installer.OpInstall, installer.OpRepair, installer.OpRefreshProjection} {
		applyLocal(t, engine, f.request(op, installed.InstallationID))
		if string(readLocal(t, f.settings)) != nativeFalse {
			t.Fatalf("%s rewrote disabled/foreign settings", op)
		}
	}
	writeLocal(t, filepath.Join(f.pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-local","version":"1.0.1"}`), 0600)
	updated := applyLocal(t, engine, f.request(installer.OpUpdate, installed.InstallationID))
	if string(readLocal(t, f.settings)) != nativeFalse || updated.Binding.TreeDigest == installed.Binding.TreeDigest {
		t.Fatal("update lost false or canonical revision")
	}
	if string(beforeOther) != string(readLocal(t, other.settings)) {
		t.Fatal("other physical profile changed")
	}
	inspect, err := engine.Inspect(t.Context())
	must(t, err)
	if len(inspect.Installations) != 1 || inspect.Recovery.Required {
		t.Fatalf("completed state inspect differs: %+v", inspect)
	}
	// 040's public Remove does not pass selected authority. Keep exact evidence;
	// the approved lifecycle composition must run the successful remove control.
	handle, err := engine.Prepare(t.Context(), f.request(installer.OpRemove, installed.InstallationID))
	must(t, err)
	defer func() { _ = handle.Close() }()
	removed, err := engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	if err == nil {
		if strings.Contains(string(readLocal(t, f.settings)), facts.Registration.Selector) {
			t.Fatal("remove left owned selector")
		}
		return
	}
	if !strings.Contains(err.Error(), "selected delivery mode differs") && !strings.Contains(err.Error(), "Local selected delivery required") {
		t.Fatalf("unexpected removal failure: %+v / %v", removed, err)
	}
	t.Log("FOUNDATION GAP: public Remove omitted persisted SelectedDelivery; no CLI/profile cleanup accepted")
	if string(readLocal(t, f.settings)) != nativeFalse {
		t.Fatal("failed remove changed profile")
	}
}

// Red: native-only actualconstructor fabricates helper readiness or falls back
// to MCP/CLI. 040 permits planning/staging but unconditionally requires helper
// in Apply. This exposes that exact facade gap without a synthetic helper.
func TestLocalPublicEngineMissingHelperFoundationGap(t *testing.T) {
	for _, optional := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent-MCP", true: "unselected-MCP"}[optional], func(t *testing.T) {
			f := freshLocal(t, optional)
			engine := f.engine(t, false)
			before := readLocal(t, f.settings)
			handle, err := engine.Prepare(t.Context(), f.request(installer.OpInstall, ""))
			must(t, err)
			defer func() { _ = handle.Close() }()
			if len(handle.Plan().Delivery.Components) != 0 {
				t.Fatal("native-only selected optional components")
			}
			result, err := engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
			if err == nil {
				if result.Binding.SelectedDelivery.IsZero() {
					t.Fatal("native-only succeeded without selected facts")
				}
				return
			}
			if !errors.Is(err, installer.ErrInvalidConfig) || !strings.Contains(err.Error(), "HelperExecutable is required") {
				t.Fatalf("unexpected gap: %+v / %v", result, err)
			}
			if string(before) != string(readLocal(t, f.settings)) {
				t.Fatal("missing helper touched profile")
			}
			if _, err := os.Lstat(filepath.Join(f.state, "mutation.lock")); !os.IsNotExist(err) {
				t.Fatal("missing helper acquired mutation lock")
			}
			t.Log("FOUNDATION GAP: Apply requires HelperExecutable although SelectedDelivery selects no MCP")
		})
	}
}

// Red: owned nonboolean drift bypasses read-only validation; identical foreign
// true/false is adopted. Boundary: actual adapter through public Engine preflight.
func TestLocalPublicProfileConflictsHaveZeroEffects(t *testing.T) {
	for _, value := range []string{"true", "false", "42"} {
		t.Run(value, func(t *testing.T) {
			f := freshLocal(t, false)
			engine := f.engine(t, true)
			h, err := engine.Prepare(t.Context(), f.request(installer.OpInstall, ""))
			must(t, err)
			selector := h.Plan().TargetPath
			id := h.Plan().InstallationID
			must(t, h.Close())
			body := []byte(`{"chat.pluginLocations":{` + quoteLocal(selector) + `:` + value + `},"foreign":"unchanged"}`)
			writeLocal(t, f.settings, body, 0600)
			h, err = engine.Prepare(t.Context(), f.request(installer.OpInstall, id))
			if h != nil {
				defer func() { _ = h.Close() }()
			}
			if err == nil {
				_, err = engine.Apply(t.Context(), h, installer.Decision{Confirmed: true})
			}
			if err == nil {
				t.Fatal("unowned entry adopted")
			}
			if string(readLocal(t, f.settings)) != string(body) {
				t.Fatal("foreign selector bytes changed")
			}
			if _, err := os.Lstat(filepath.Join(f.state, "state-v2.json")); !os.IsNotExist(err) {
				t.Fatal("foreign collision committed state")
			}
		})
	}
}

// Red: caller mutation or a switched real constructor bypasses frozen facts
// during Prepare->Apply; same-installation second profile is silently redirected.
func TestLocalPublicFrozenConstructorAndStaleConfirmation(t *testing.T) {
	f := freshLocal(t, false)
	engine := f.engine(t, true)
	handle, err := engine.Prepare(t.Context(), f.request(installer.OpInstall, ""))
	must(t, err)
	original := handle.Plan().SelectedDelivery
	f.config.HookSpecs[0].Args[0] = "caller mutation"
	if _, err := engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true}); err != nil {
		t.Fatal("caller slice modified frozen constructor", err)
	}
	replacement := f.config
	replacement.HookSpecs = localSpecs(f.runtime)
	replacement.NativeStop = false
	replacement.HookSpecs = nil
	other, err := vscode.NewLocal(replacement)
	must(t, err)
	h, err := engine.Prepare(t.Context(), f.request(installer.OpRefreshProjection, handle.Plan().InstallationID))
	must(t, err)
	defer func() { _ = h.Close() }()
	before := readLocal(t, f.settings)
	state := readLocal(t, filepath.Join(f.state, "state-v2.json"))
	*f.adapter = *other
	result, err := engine.Apply(t.Context(), h, installer.Decision{Confirmed: true})
	if !errors.Is(err, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict {
		t.Fatalf("stale constructor accepted: %+v %v", result, err)
	}
	if string(before) != string(readLocal(t, f.settings)) || string(state) != string(readLocal(t, filepath.Join(f.state, "state-v2.json"))) {
		t.Fatal("stale confirmation caused effects")
	}
	// Distinct profile with a separately valid constructor; same state forbids it.
	second := freshLocal(t, false)
	second.state = f.state
	_, err = second.engine(t, true).Prepare(t.Context(), second.request(installer.OpInstall, handle.Plan().InstallationID))
	if err == nil {
		t.Fatal("same installation admitted a second profile")
	}
	if original.IsZero() {
		t.Fatal("initial plan lacked selected authority")
	}
}

func quoteLocal(s string) string { body, _ := json.Marshal(s); return string(body) }

// Red: profile aliases/errors choose ambient default settings or inspector
// writes a lock. Boundary: explicit constructor and no-follow public profile IO.
func TestLocalPhysicalProfileAndReadOnlyRegistry(t *testing.T) {
	f := freshLocal(t, false)
	alias := filepath.Join(f.root, "profile-alias")
	must(t, os.Symlink(filepath.Dir(f.settings), alias))
	cfg := f.config
	cfg.ProfileSettingsPath = filepath.Join(alias, "settings.json")
	a, err := vscode.NewLocal(cfg)
	must(t, err)
	if _, err := a.ResolveProfileRoot(alias); err != nil {
		t.Fatal("explicit alias failed physical normalization", err)
	}
	_, plan, staged := f.stage(t)
	facts, _ := plan.SelectedDelivery.LocalFacts()
	for _, value := range []string{"true", "false", "null", "absent"} {
		body := []byte(`{"chat.pluginLocations":{` + quoteLocal(facts.Registration.Selector) + `:` + value + `}}`)
		if value == "absent" {
			body = []byte(`{"chat.pluginLocations":{}}`)
		}
		writeLocal(t, f.settings, body, 0600)
		state, err := a.InspectRegistration(t.Context(), nativeconfig.New(), plan.SelectedDelivery, staged.NativeObjects)
		want := map[string]vscode.RegistrationState{"true": vscode.RegistrationActive, "false": vscode.RegistrationDisabled, "null": vscode.RegistrationConflict, "absent": vscode.RegistrationMissing}[value]
		if state != want || (value == "null") != (err != nil) {
			t.Fatalf("native %s => %s %v", value, state, err)
		}
		if string(body) != string(readLocal(t, f.settings)) {
			t.Fatal("readonly inspector changed profile")
		}
	}
	locks, err := nativeconfig.WriterLockPaths(nativeconfig.Paths{JSON: f.settings}, nativeconfig.CodecMCPServers)
	must(t, err)
	for _, lock := range locks {
		if _, err := os.Lstat(lock); !os.IsNotExist(err) {
			t.Fatal("readonly inspector created native lock")
		}
	}
	cfg = f.config
	cfg.QualifiedTuple.VSCodeVersion = "unqualified"
	if _, err := vscode.NewLocal(cfg); err == nil {
		t.Fatal("unqualified tuple admitted")
	}
	cfg = f.config
	cfg.HookSpecs[0].Args = []string{"${HOME}"}
	if _, err := vscode.NewLocal(cfg); err == nil {
		t.Fatal("unknown root token admitted")
	}
}

var _ clients.RegistryInspector = (*vscode.LocalAdapter)(nil)

// Red: observing false fails to persist its native boolean receipt, then
// absent-entry repair resurrects true. This exercises the exact 040 contract
// gap: outcomes carry objects, but immutable selected bool authority stays true.
// Until that receipt can be recorded, absence must retain bytes and refuse.
func TestLocalDisabledThenAbsentCannotBeReenabled(t *testing.T) {
	f := freshLocal(t, false)
	engine := f.engine(t, true)
	installed := applyLocal(t, engine, f.request(installer.OpInstall, ""))
	facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
	disabled := []byte(`{"chat.pluginLocations":{` + quoteLocal(facts.Registration.Selector) + `:false}}`)
	writeLocal(t, f.settings, disabled, 0600)
	applyLocal(t, engine, f.request(installer.OpRepair, installed.InstallationID))
	absent := []byte(`{"chat.pluginLocations":{},"TEST-late-foreign":true}`)
	writeLocal(t, f.settings, absent, 0600)
	handle, err := engine.Prepare(t.Context(), f.request(installer.OpRepair, installed.InstallationID))
	if handle != nil {
		defer func() { _ = handle.Close() }()
	}
	if err == nil {
		_, err = engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	}
	if err == nil {
		t.Fatal("lost false receipt: absent repair silently re-enabled Local")
	}
	if string(readLocal(t, f.settings)) != string(absent) {
		t.Fatal("absent disabled repair changed profile bytes")
	}
	t.Log("CONTRACT GAP: 040 cannot persist observed false receipt separately from immutable selected desired bool")
}

// Red: changed declared bytes or a native schema change survives constructor
// and plan checks. Boundary: actual public Prepare, retaining profile/state.
func TestLocalCanonicalHookAdmissionFailsClosed(t *testing.T) {
	for _, drift := range []string{"digest", "schema", "JSONC"} {
		t.Run(drift, func(t *testing.T) {
			f := freshLocal(t, false)
			before := readLocal(t, f.settings)
			if drift == "JSONC" {
				before = []byte(`{"chat.pluginLocations":{},"chat.pluginLocations":{}}`)
				writeLocal(t, f.settings, before, 0600)
			} else {
				path := filepath.Join(f.pkg, hookPath())
				changed := []byte(strings.Replace(string(readLocal(t, path)), `"timeout": 5`, `"timeout": 4`, 1))
				if testDigest(changed) == f.config.DeclaredHookDigest {
					t.Fatal("fixture did not change timeout")
				}
				writeLocal(t, path, changed, 0600)
				if drift == "schema" {
					f.config.DeclaredHookDigest = testDigest(changed)
					replacement, err := vscode.NewLocal(f.config)
					must(t, err)
					*f.adapter = *replacement
				}
			}
			handle, err := f.engine(t, true).Prepare(t.Context(), f.request(installer.OpInstall, ""))
			if handle != nil {
				defer func() { _ = handle.Close() }()
			}
			if err == nil {
				t.Fatal("changed canonical declaration admitted")
			}
			if string(before) != string(readLocal(t, f.settings)) {
				t.Fatal("admission refusal changed profile")
			}
			for _, path := range []string{"state-v2.json", "mutation.lock"} {
				if _, err := os.Lstat(filepath.Join(f.state, path)); !os.IsNotExist(err) {
					t.Fatal("admission committed state or acquired mutation lock", path)
				}
			}
		})
	}
}
