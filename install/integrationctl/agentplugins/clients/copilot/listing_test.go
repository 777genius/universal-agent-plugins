package copilot

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Captured from the pinned Copilot 1.0.103 binary in an isolated empty profile.
const emptyBuiltinListing = copilotEmptyListingBody + copilot103BuiltinSection

type listingRunner struct{ stdout string }

func (r listingRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{Stdout: []byte(r.stdout)}, nil
}

func TestNativeRegistryEmptyBuiltinListing(t *testing.T) {
	for _, body := range []string{emptyBuiltinListing, copilotEmptyListingBody + copilot112BuiltinSection} {
		finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{Runner: listingRunner{body}}, domain.DetectedClient{}, domain.DeliveryPlan{NativeRegistryExecutable: "copilot", DeclaredName: "demo"}, nil)
		if err != nil || finding != clients.RegistryClear {
			t.Fatalf("actual empty listing %q: finding=%v err=%v", body, finding, err)
		}
	}
}

// Regression: the adapter's listing normalization removed the builtin section
// before the same-name guard could see it, so a plugin named like a builtin
// was reported absent and verified through the adapter.
func TestNativeRegistryRefusesSameNameBuiltinThroughAdapter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managed-artifact")
	for _, name := range []string{"computer-use", "Browser-Use"} {
		plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, Scope: domain.ScopeUser, NativeRegistryExecutable: "copilot",
			DeclaredName: name, DeclaredVersion: "1.0.0", PhysicalArtifactID: "fixture-physical-artifact", ActivePath: path}
		empty := clients.Env{Runner: listingRunner{copilotEmptyListingBody + copilot112BuiltinSection}}
		if finding, err := New().InspectNativeRegistry(context.Background(), empty, domain.DetectedClient{}, plan, nil); err != nil || finding != clients.RegistryIndeterminate {
			t.Fatalf("%s: empty listing finding=%v err=%v", name, finding, err)
		}
		live := copilot112BuiltinSection + "Live Plugins (loaded from a local marketplace directory, never copied):\n  • " + name + "@" +
			shared.ManagedMarketplaceName(plan.PhysicalArtifactID) + " (v1.0.0) (enabled)\n      from " + path + "\n"
		outcome, err := New().Activate(context.Background(), clients.Env{Runner: listingRunner{live}}, domain.ActivationRequest{
			Client: domain.DetectedClient{ClientID: domain.ClientCopilot}, DeclaredName: name, Plan: plan,
			Delivery: domain.StagedDelivery{ClientID: domain.ClientCopilot, ActivePath: path}, BackendExecutable: "copilot", VerifyOnly: true})
		if err != nil || outcome.Verification == domain.VerificationInstalled {
			t.Fatalf("%s: same-name builtin verified: outcome=%+v err=%v", name, outcome, err)
		}
	}
}

func TestNativeRegistryLiveBuiltinListing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managed-artifact")
	live := "Live Plugins (loaded from a local marketplace directory, never copied):\n  • uap-commands-fixture@agentplugins-012345abcdef (v1.0.0) (enabled)\n      from " + path + "\n"
	// Captured after actual marketplace add and install using Copilot 1.0.103.
	marketplace := shared.ManagedMarketplaceName("fixture-physical-artifact")
	body := strings.ReplaceAll(copilot103BuiltinSection+live, "agentplugins-012345abcdef", marketplace)
	plan := domain.DeliveryPlan{NativeRegistryExecutable: "copilot", DeclaredName: "uap-commands-fixture", PhysicalArtifactID: "fixture-physical-artifact", DeclaredVersion: "1.0.0", ActivePath: path}
	for _, test := range []struct {
		name, body string
		owned      bool
		want       clients.RegistryFinding
	}{
		{"owned", body, true, clients.RegistryExpected},
		{"unowned", body, false, clients.RegistryCollision},
		{"wrong version", strings.Replace(body, "(v1.0.0)", "(v2.0.0)", 1), true, clients.RegistryIndeterminate},
		{"disabled", strings.Replace(body, "(enabled)", "(disabled)", 1), true, clients.RegistryIndeterminate},
		{"wrong path", strings.Replace(body, path, path+"-foreign", 1), true, clients.RegistryIndeterminate},
		{"foreign marketplace", strings.Replace(body, marketplace, "foreign", 1), true, clients.RegistryClear},
		{"duplicate identity", body + strings.TrimPrefix(strings.ReplaceAll(live, "agentplugins-012345abcdef", marketplace), shared.CopilotLiveHeader+"\n"), true, clients.RegistryIndeterminate},
		{"1.0.112 builtins", strings.Replace(body, copilot103BuiltinSection, copilot112BuiltinSection, 1), true, clients.RegistryExpected},
		{"other builtin release", strings.Replace(body, "browser-use (v0.0.4)", "other (v9.9.9)", 1), true, clients.RegistryExpected},
		{"duplicate builtin section", copilot103BuiltinSection + body, true, clients.RegistryIndeterminate},
		{"malformed builtin", strings.Replace(body, "[disabled]", "[unknown]", 1), true, clients.RegistryIndeterminate},
		{"arbitrary prefix", "unexpected\n" + body, true, clients.RegistryIndeterminate},
		{"arbitrary suffix", body + "unexpected\n", true, clients.RegistryIndeterminate},
	} {
		t.Run(test.name, func(t *testing.T) {
			var binding *domain.ClientBinding
			if test.owned {
				binding = &domain.ClientBinding{TargetLocator: path}
			}
			env := clients.Env{Runner: listingRunner{test.body}}
			got, err := New().InspectNativeRegistry(context.Background(), env, domain.DetectedClient{}, plan, binding)
			if err != nil || got != test.want {
				t.Fatalf("finding=%v want=%v err=%v", got, test.want, err)
			}
			outcome, err := New().Activate(context.Background(), env, domain.ActivationRequest{DeclaredName: plan.DeclaredName, Plan: plan, Delivery: domain.StagedDelivery{ActivePath: path}, BackendExecutable: "copilot", VerifyOnly: true})
			if err != nil && test.name != "foreign marketplace" {
				t.Fatal(err)
			}
			if installed := outcome.Verification == domain.VerificationInstalled; installed != (test.want == clients.RegistryExpected || test.want == clients.RegistryCollision) {
				t.Fatalf("verification=%v activation=%v", outcome.Verification, outcome.Activation)
			}
		})
	}
}

type treeListingRunner struct {
	called  bool
	grace   time.Duration
	command legacyports.Command
	result  legacyports.CommandResult
	err     error
}

func (r *treeListingRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	panic("tree registry inspection must preserve tree runner")
}
func (r *treeListingRunner) RunWithTreeExitGrace(_ context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	r.called, r.grace, r.command = true, grace, command
	return r.result, r.err
}

func TestListingRunnerPreservesCapabilitiesAndFailures(t *testing.T) {
	if _, ok := listingEnv(clients.Env{Runner: listingRunner{}}, "", "").Runner.(ports.TreeCommandRunner); ok {
		t.Fatal("advertised unsupported tree capability")
	}
	sentinel := errors.New("containment failure")
	original := legacyports.CommandResult{Stdout: []byte(emptyBuiltinListing), Stderr: []byte("diagnostic"), ExitCode: 7, NoChildStarted: true, CleanupComplete: true}
	runner := &treeListingRunner{result: original, err: sentinel}
	env := listingEnv(clients.Env{Runner: runner}, "", "")
	_, err := New().InspectNativeRegistry(context.Background(), env, domain.DetectedClient{}, domain.DeliveryPlan{NativeRegistryExecutable: "copilot"}, nil)
	if !errors.Is(err, sentinel) || !runner.called || runner.grace != time.Second || !reflect.DeepEqual(runner.command.Argv, []string{"copilot", "plugin", "list"}) {
		t.Fatalf("lost tree invocation: runner=%+v err=%v", runner, err)
	}
	result, err := env.Runner.(ports.TreeCommandRunner).RunWithTreeExitGrace(context.Background(), runner.command, time.Second)
	if !errors.Is(err, sentinel) || !reflect.DeepEqual(result, original) {
		t.Fatalf("lost failure evidence: result=%+v err=%v", result, err)
	}
	runner.err = nil
	runner.result.ExitCode = 0
	result, err = env.Runner.(ports.TreeCommandRunner).RunWithTreeExitGrace(context.Background(), runner.command, time.Second)
	expected := runner.result
	expected.Stdout = []byte(copilotEmptyListingBody)
	if err != nil || !reflect.DeepEqual(result, expected) {
		t.Fatalf("lost successful execution evidence: result=%+v err=%v", result, err)
	}
}
