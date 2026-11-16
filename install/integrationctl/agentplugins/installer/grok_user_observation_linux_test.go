//go:build linux

package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type grokUserObservationNative struct {
	executable, source, status, version string
	allowInstall, missing, unknown      bool
	lists, installs, mutations          int
}

func (native *grokUserObservationNative) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if reflect.DeepEqual(command.Argv, []string{native.executable, "plugin", "list", "--json"}) {
		native.lists++
		if command.StdoutLimitBytes != 1<<20 {
			return legacyports.CommandResult{}, fmt.Errorf("unbounded list")
		}
		if native.unknown {
			return legacyports.CommandResult{Stdout: []byte(`{"unknown":true}`)}, nil
		}
		rows := []any{}
		if native.source != "" && !native.missing {
			rows = append(rows, map[string]any{"name": "grok-sdk-owned", "source": native.source, "status": native.status, "version": native.version})
		}
		body, err := json.Marshal(rows)
		return legacyports.CommandResult{Stdout: body}, err
	}
	native.mutations++
	if native.allowInstall && len(command.Argv) == 5 && reflect.DeepEqual(command.Argv[:4], []string{native.executable, "plugin", "install", "--trust"}) {
		native.installs++
		native.source = command.Argv[4]
		return legacyports.CommandResult{}, nil
	}
	return legacyports.CommandResult{}, fmt.Errorf("repeat/preview attempted native mutation: %v", command.Argv)
}

func newGrokUserObservationFixture(t *testing.T, authentication domain.AuthenticationState) (*grokEnableSDKFixture, *grokUserObservationNative) {
	t.Helper()
	f := newGrokEnableSDKFixture(t, false)
	native := &grokUserObservationNative{executable: f.native.executable, status: "installed", version: "1.0.0", allowInstall: true}
	registry, err := clients.NewRegistry(grok.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.engine, err = New(Config{UserHome: f.home, StateRoot: filepath.Join(f.project, ".uap"), Registry: registry,
		Runner: native, EnableNativeObserver: true, TrustedLocalPackages: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	f.request.Scope, f.request.ProjectRoot = domain.ScopeUser, ""
	f.install(t)
	native.allowInstall = false
	state, err := f.engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := f.binding(t)
	if binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled || native.installs != 1 {
		t.Fatal("initial actual User adapter route did not install", binding, native.installs)
	}
	// Seed only the prior saved authentication classification. The adapter's
	// actual initial package/registration/receipt remain the authority.
	binding.Authentication = authentication
	state.Installations[0].Clients[binding.ClientBindingID] = binding
	if err := f.engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	deathWrite(t, filepath.Join(f.home, ".grok", "trusted_folders.toml"), "# User trust must not be rewritten\n")
	return f, native
}

func TestGrokUserConvergedRepeatAndOnlinePreviewAreFreshAndReadOnly(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		authentication domain.AuthenticationState
		online         bool
	}{
		{"not-required-offline", domain.AuthenticationNotRequired, false},
		{"not-required-online", domain.AuthenticationNotRequired, true},
		{"complete-offline", domain.AuthenticationComplete, false},
		{"complete-online", domain.AuthenticationComplete, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			authentication := scenario.authentication
			f, native := newGrokUserObservationFixture(t, authentication)
			binding := f.binding(t)
			unchanged := grokObservedSDKPreserved(t, f, binding, filepath.Join(f.project, ".uap", "state-v2.json"))
			request := f.selected(t, OpInstall, false)
			request.OperationID = "grok-user-observed-repeat"
			request.OnlinePreview = scenario.online
			calls := native.lists
			prepared, err := f.engine.Prepare(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			plan := prepared.Plan()
			wantLists := calls
			if scenario.online {
				wantLists++
			}
			if native.lists != wantLists || plan.Client.Activation != string(domain.ActivationActive) || plan.Client.Authentication != string(authentication) {
				t.Fatal("converged preview violated its observation permission", plan, native.lists, wantLists)
			}
			unchanged()
			calls = native.lists
			result, err := f.engine.Apply(t.Context(), prepared, Decision{Confirmed: true})
			if closeErr := prepared.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil || result.Outcome != OutcomeUnchanged || !result.NoChange || result.Mutated ||
				result.Client.Activation != string(domain.ActivationActive) || result.Client.Authentication != string(authentication) ||
				native.lists != calls+1 || native.installs != 1 || native.mutations != 1 || !reflect.DeepEqual(binding, f.binding(t)) {
				t.Fatalf("converged repeat changed state or skipped exact list: %+v %v", result, err)
			}
			unchanged()
		})
	}
}

func TestGrokUserFreshObservationDoesNotReturnCachedReady(t *testing.T) {
	for _, failure := range []string{"disabled", "absent", "foreign-source", "version", "unknown"} {
		t.Run(failure, func(t *testing.T) {
			f, native := newGrokUserObservationFixture(t, domain.AuthenticationComplete)
			binding := f.binding(t)
			unchanged := grokObservedSDKPreserved(t, f, binding, filepath.Join(f.project, ".uap", "state-v2.json"))
			nativeUnchanged := grokObservedSDKPreserved(t, f, binding)
			switch failure {
			case "disabled":
				native.status = "disabled"
			case "absent":
				native.missing = true
			case "foreign-source":
				native.source = filepath.Join(f.home, "foreign")
			case "version":
				native.version = "9.9.9"
			case "unknown":
				native.unknown = true
			}
			request := f.selected(t, OpInstall, false)
			request.OperationID = "grok-user-negative-repeat"
			request.OnlinePreview = true
			calls := native.lists
			prepared, err := f.engine.Prepare(t.Context(), request)
			if err == nil {
				plan := prepared.Plan()
				if closeErr := prepared.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if plan.Client.Activation == string(domain.ActivationActive) {
					t.Fatal("negative online evidence retained cached Active", plan)
				}
			}
			if native.lists <= calls || native.installs != 1 || native.mutations != 1 {
				t.Fatal("negative preview skipped list or acted")
			}
			unchanged()
			request.OnlinePreview = false
			result, err := f.execute(t, request, Decision{Confirmed: true})
			if err == nil && result.Client.Activation == string(domain.ActivationActive) {
				t.Fatal("negative repeat returned cached Ready", result)
			}
			if native.installs != 1 || native.mutations != 1 {
				t.Fatal("negative repeat attempted native mutation")
			}
			// Negative Apply may persist its fresh failure. The native profile
			// and managed package still cannot be changed by an observation.
			nativeUnchanged()
		})
	}
}
