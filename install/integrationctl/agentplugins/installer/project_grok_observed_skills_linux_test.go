//go:build linux

package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Native transport remains an inert inspect-only fixture; package projection,
// saved receipts, Prepared capability routing and state persistence are real.
type grokObservedSDKNative struct{ base *grokEnableSDKNative }

func (native grokObservedSDKNative) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	native.base.calls++
	if !reflect.DeepEqual(command.Argv, []string{native.base.executable, "inspect", "--json"}) {
		return legacyports.CommandResult{}, fmt.Errorf("acting native command is forbidden: %v", command.Argv)
	}
	rows, err := native.base.plugins(command.Dir)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	trusted, err := grokObservedSDKTrust(native.base.home, command.Dir)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	plugins, skills := []any{}, []any{}
	for _, row := range rows {
		name, root := row["name"].(string), row["root"].(string)
		plugins = append(plugins, map[string]any{"name": name, "scope": "project", "path": root, "enabled": row["enabled"],
			"provides": map[string]any{"skills": 1, "agents": 0, "hooks": false, "mcpServers": 0}})
		if trusted && row["enabled"] == true {
			skills = append(skills, map[string]any{"name": "docs", "description": "inert Grok SDK fixture", "userInvocable": true,
				"source": map[string]any{"type": "plugin", "plugin_name": name, "path": filepath.Join(root, "skills", "docs", "SKILL.md")}})
		}
	}
	body, err := json.Marshal(map[string]any{"grokVersion": "1.0.70", "cwd": command.Dir, "projectRoot": command.Dir,
		"projectTrusted": trusted, "plugins": plugins, "skills": skills, "mcpServers": []any{}})
	return legacyports.CommandResult{Stdout: body}, err
}

func grokObservedSDKTrust(home, project string) (bool, error) {
	body, err := os.ReadFile(filepath.Join(home, ".grok", "trusted_folders.toml"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var document struct {
		Folders map[string]struct {
			Trusted bool `toml:"trusted"`
		} `toml:"folders"`
	}
	if err := toml.Unmarshal(body, &document); err != nil {
		return false, err
	}
	return document.Folders[project].Trusted, nil
}

func grokObservedSDKEngine(t *testing.T, f *grokEnableSDKFixture) *Engine {
	t.Helper()
	registry, err := clients.NewRegistry(grok.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{ProjectRoot: f.project, UserHome: f.home, StateRoot: filepath.Join(f.project, ".uap"),
		Registry: registry, Runner: grokObservedSDKNative{base: f.native}, EnableNativeObserver: true,
		TrustedLocalPackages: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestProjectGrokSDKPreparedRepeatObservesOnlyExternallyEnabledTrustedSkills(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		trusted, enabled bool
	}{
		{"trusted-enabled", true, true}, {"untrusted", false, true}, {"disabled", true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newGrokEnableSDKFixture(t, true)
			f.engine = grokObservedSDKEngine(t, f)
			f.install(t)
			before := f.binding(t)
			if before.Activation != domain.ActivationPrepared || before.Verification != domain.VerificationPackageValid ||
				before.Authentication != domain.AuthenticationNotChecked || before.NativeActivationAttempt != "" {
				t.Fatal("untrusted initial installation was promoted", before)
			}
			grokObservedSDKExternalSetup(t, f, scenario.trusted, scenario.enabled)
			unchanged := grokObservedSDKPreserved(t, f, before)
			calls := f.native.calls
			request := f.selected(t, OpInstall, false)
			request.OperationID = "grok-sdk-observed-repeat"
			result, err := f.execute(t, request, Decision{Confirmed: true})
			if err != nil || result.RequiresConfirmation || f.native.calls <= calls || f.native.duplex != 0 {
				t.Fatalf("saved Prepared repeat did not perform only read-only observation: %+v %v", result, err)
			}
			after := f.binding(t)
			grokObservedSDKOutcome(t, scenario.trusted && scenario.enabled, result, before, after)
			unchanged()
		})
	}
}

func TestProjectGrokSDKRepairPreviewRequiresObservationPermission(t *testing.T) {
	for _, scenario := range []struct {
		name              string
		online, confirmed bool
	}{
		{"offline-cancelled", false, false}, {"online-cancelled", true, false},
		{"offline-confirmed", false, true}, {"online-confirmed", true, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newGrokEnableSDKFixture(t, true)
			f.engine = grokObservedSDKEngine(t, f)
			f.install(t)
			before := f.binding(t)
			if before.Activation != domain.ActivationPrepared || before.Authentication != domain.AuthenticationNotChecked {
				t.Fatal("initial untrusted binding was promoted", before)
			}
			grokObservedSDKExternalSetup(t, f, true, true)
			unchanged := grokObservedSDKPreserved(t, f, before, filepath.Join(f.project, ".uap", "state-v2.json"))
			nativeUnchanged := grokObservedSDKPreserved(t, f, before)
			request := f.selected(t, OpRepair, false)
			request.OperationID = "grok-sdk-repair-" + scenario.name
			request.OnlinePreview = scenario.online
			calls := f.native.calls
			prepared, err := f.engine.Prepare(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := prepared.Close(); err != nil {
					t.Error(err)
				}
			})
			wantCalls := calls
			wantActivation, wantAuthentication := domain.ActivationPrepared, domain.AuthenticationNotChecked
			if scenario.online {
				wantCalls++
				wantActivation, wantAuthentication = domain.ActivationActive, domain.AuthenticationNotRequired
			}
			plan := prepared.Plan()
			if f.native.calls != wantCalls || f.native.duplex != 0 ||
				plan.Client.Activation != string(wantActivation) || plan.Client.Authentication != string(wantAuthentication) ||
				!reflect.DeepEqual(before, f.binding(t)) {
				t.Fatalf("repair preview violated observation permission: %+v calls=%d want=%d", plan, f.native.calls, wantCalls)
			}
			unchanged()
			calls = f.native.calls
			result, err := f.engine.Apply(t.Context(), prepared, Decision{Confirmed: scenario.confirmed})
			if closeErr := prepared.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if !scenario.confirmed {
				if !errors.Is(err, ErrCancelled) || result.Outcome != OutcomeCancelled || f.native.calls != calls || f.native.duplex != 0 {
					t.Fatalf("cancelled repair observed or acted: %+v %v calls=%d", result, err, f.native.calls)
				}
				unchanged()
				return
			}
			if err != nil || result.RequiresConfirmation || f.native.calls != calls+1 || f.native.duplex != 0 {
				t.Fatalf("confirmed repair did not freshly observe exactly once: %+v %v calls=%d", result, err, f.native.calls)
			}
			grokObservedSDKOutcome(t, true, result, before, f.binding(t))
			nativeUnchanged()
		})
	}
}

func grokObservedSDKExternalSetup(t *testing.T, f *grokEnableSDKFixture, trusted, enabled bool) {
	t.Helper()
	rows, err := f.native.plugins(f.project)
	if err != nil || len(rows) != 1 {
		t.Fatalf("owned native metadata: %+v %v", rows, err)
	}
	id, err := json.Marshal(rows[0]["id"])
	if err != nil {
		t.Fatal(err)
	}
	members := "['foreign']"
	if enabled {
		members = "['foreign'," + string(id) + "]"
	}
	deathWrite(t, f.config, "# private foreign settings\n[plugins]\nenabled="+members+"\ndisabled=['other']\n")
	project, err := json.Marshal(f.project)
	if err != nil {
		t.Fatal(err)
	}
	deathWrite(t, filepath.Join(f.home, ".grok", "trusted_folders.toml"), fmt.Sprintf("[folders.%s]\ntrusted = %t\n", project, trusted))
}

func grokObservedSDKOutcome(t *testing.T, ready bool, result Result, before, after domain.ClientBinding) {
	t.Helper()
	if after.NativeActivationAttempt != "" || after.PendingNativeIntent != nil ||
		!reflect.DeepEqual(before.NativeObjects, after.NativeObjects) || !reflect.DeepEqual(before.Receipts, after.Receipts) ||
		before.ClientBindingID != after.ClientBindingID || before.TargetLocator != after.TargetLocator {
		t.Fatal("read-only completion changed managed ownership", before, after)
	}
	if ready {
		if after.Activation != domain.ActivationActive || after.Verification != domain.VerificationInstalled ||
			after.Authentication != domain.AuthenticationNotRequired || after.Policy != domain.PolicyAllowed ||
			result.Client.Activation != string(domain.ActivationActive) || len(result.ManualActions) != 0 || len(result.NextActions) != 0 {
			t.Fatalf("fresh trusted exact Skill did not finish installation: %+v %+v", result, after)
		}
		return
	}
	if after.Activation != domain.ActivationPrepared || after.Verification != domain.VerificationPackageValid ||
		after.Authentication != domain.AuthenticationNotChecked || result.Mutated || !reflect.DeepEqual(before, after) {
		t.Fatalf("untrusted/disabled repeat promoted or changed saved state: %+v %+v", result, after)
	}
	for _, action := range result.NextActions {
		if action.Kind == "repair" {
			t.Fatal("read-only opt-in offered acting repair", result)
		}
	}
}

func grokObservedSDKPreserved(t *testing.T, f *grokEnableSDKFixture, binding domain.ClientBinding, extra ...string) func() {
	t.Helper()
	paths := make([]string, 0, 4+len(extra))
	paths = append(paths, f.config, filepath.Join(f.home, ".grok", "trusted_folders.toml"),
		filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), filepath.Join(binding.TargetLocator, ".grok-plugin", "plugin.json"))
	paths = append(paths, extra...)
	before := map[string]string{}
	identities := map[string]os.FileInfo{}
	for _, path := range paths {
		before[path] = deathRead(t, path)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		identities[path] = info
	}
	return func() {
		t.Helper()
		for _, path := range paths {
			info, err := os.Stat(path)
			old := identities[path]
			if err != nil || deathRead(t, path) != before[path] || !os.SameFile(old, info) ||
				old.Mode() != info.Mode() || !old.ModTime().Equal(info.ModTime()) {
				t.Fatal("repeat changed native settings or package", path, err)
			}
		}
	}
}
