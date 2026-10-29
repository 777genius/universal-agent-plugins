package copilot

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type profileRecordingRunner struct{ commands []legacyports.Command }

func (r *profileRecordingRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.commands = append(r.commands, command)
	return legacyports.CommandResult{Stdout: []byte(emptyBuiltinListing)}, nil
}
func TestUserCommandsUseSelectedCopilotProfile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("COPILOT_HOME", t.TempDir())
	for _, operation := range []string{"inspect", "activate", "deactivate"} {
		t.Run(operation, func(t *testing.T) {
			runner := &profileRecordingRunner{}
			env := clients.Env{Runner: runner}
			client := domain.DetectedClient{ClientID: domain.ClientCopilot, ConfigRoot: root}
			plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, NativeRegistryExecutable: "copilot", NativeRegistryRoot: root}
			var err error
			switch operation {
			case "inspect":
				_, err = New().InspectNativeRegistry(t.Context(), env, client, plan, nil)
			case "activate":
				_, err = New().Activate(t.Context(), env, domain.ActivationRequest{Client: client, Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientCopilot}, BackendExecutable: "copilot", DeclaredName: "fixture"})
			case "deactivate":
				_, err = New().Deactivate(t.Context(), env, domain.DeactivationRequest{Client: client, NativeRegistryRoot: root, CurrentActivation: domain.ActivationActive, Confirmed: true, PhysicalArtifactID: "fixture", BackendExecutable: "copilot"})
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(runner.commands) == 0 {
				t.Fatal("no native command")
			}
			for _, command := range runner.commands {
				count := 0
				for _, entry := range command.Env {
					if strings.EqualFold(strings.SplitN(entry, "=", 2)[0], "COPILOT_HOME") {
						count++
						if entry != "COPILOT_HOME="+root {
							t.Fatalf("foreign profile: %q", entry)
						}
					}
				}
				if count != 1 {
					t.Fatalf("selected profile not bound: count=%d", count)
				}
			}
		})
	}
}

type profileEvidenceRunner struct {
	command legacyports.Command
	ctx     context.Context
	result  legacyports.CommandResult
	err     error
}

func (r *profileEvidenceRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.ctx, r.command = ctx, command
	return r.result, r.err
}

type profileEvidenceTreeRunner struct {
	*profileEvidenceRunner
	grace time.Duration
}

func (r *profileEvidenceTreeRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	r.grace = grace
	return r.Run(ctx, command)
}

type profileContextKey struct{}

func TestProfileWrapperPreservesCommandAndExecutionEvidence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("COPILOT_HOME", "ambient-foreign")
	t.Setenv("COPILOT_PROFILE_TEST_KEEP", "ambient-value")
	for _, tree := range []bool{false, true} {
		for _, base := range [][]string{nil, {}, {"KEEP=one", "copilot_home=foreign", "Copilot_Home=other", "COPILOT_HOME=third", "VALUE=a=b"}} {
			original := append([]string{}, base...)
			sentinel := errors.New("execution evidence")
			evidence := legacyports.CommandResult{Stdout: []byte(emptyBuiltinListing), Stderr: []byte("stderr"), ExitCode: 9, NoChildStarted: true, CleanupComplete: true}
			capture := &profileEvidenceRunner{result: evidence, err: sentinel}
			var runner ports.CommandRunner = capture
			treeRunner := &profileEvidenceTreeRunner{profileEvidenceRunner: capture}
			if tree {
				runner = treeRunner
			}
			wrapped := listingEnv(clients.Env{Runner: runner}, root).Runner
			ctx := context.WithValue(t.Context(), profileContextKey{}, "context")
			command := legacyports.Command{Argv: []string{"copilot", "plugin", "list"}, Env: base, Dir: "/working-directory", StdoutLimitBytes: 123}
			var result legacyports.CommandResult
			var err error
			if tree {
				result, err = wrapped.(ports.TreeCommandRunner).RunWithTreeExitGrace(ctx, command, 3*time.Second)
			} else {
				result, err = wrapped.Run(ctx, command)
			}
			if !errors.Is(err, sentinel) || !reflect.DeepEqual(result, evidence) || capture.ctx != ctx {
				t.Fatalf("execution evidence changed: result=%+v err=%v", result, err)
			}
			if tree && treeRunner.grace != 3*time.Second {
				t.Fatal("tree grace changed")
			}
			if !reflect.DeepEqual(command.Argv, capture.command.Argv) || capture.command.Dir != command.Dir || capture.command.StdoutLimitBytes != command.StdoutLimitBytes {
				t.Fatal("command fields changed")
			}
			expected := []string{}
			if base == nil {
				for _, entry := range os.Environ() {
					key, _, _ := strings.Cut(entry, "=")
					if !strings.EqualFold(key, "COPILOT_HOME") {
						expected = append(expected, entry)
					}
				}
			} else {
				for _, entry := range base {
					key, _, _ := strings.Cut(entry, "=")
					if !strings.EqualFold(key, "COPILOT_HOME") {
						expected = append(expected, entry)
					}
				}
			}
			expected = append(expected, "COPILOT_HOME="+root)
			if !reflect.DeepEqual(expected, capture.command.Env) {
				t.Fatal("bound environment did not preserve unrelated entries and the selected profile")
			}
			if !reflect.DeepEqual(original, append([]string{}, base...)) {
				t.Fatal("caller environment mutated")
			}
			if len(base) > 0 {
				capture.command.Env[0] = "changed"
				if base[0] != "KEEP=one" {
					t.Fatal("caller environment shares backing slice")
				}
			}
		}
	}
}

func TestUserProfileConflictsFailBeforeNativeCommand(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	runner := &profileRecordingRunner{}
	env := clients.Env{Runner: runner}
	client := domain.DetectedClient{ClientID: domain.ClientCopilot, ConfigRoot: root}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, NativeRegistryRoot: other, NativeRegistryExecutable: "copilot"}
	if _, err := New().InspectNativeRegistry(t.Context(), env, client, plan, nil); err == nil {
		t.Fatal("inspection accepted conflicting root")
	}
	if _, err := New().Activate(t.Context(), env, domain.ActivationRequest{Client: client, Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientCopilot}, BackendExecutable: "copilot"}); err == nil {
		t.Fatal("activation accepted conflicting root")
	}
	if _, err := New().Deactivate(t.Context(), env, domain.DeactivationRequest{Client: client, NativeRegistryRoot: other, CurrentActivation: domain.ActivationActive, Confirmed: true, BackendExecutable: "copilot"}); err == nil {
		t.Fatal("removal accepted conflicting root")
	}
	if _, err := New().Deactivate(t.Context(), env, domain.DeactivationRequest{NativeRegistryRoot: root, TargetContext: domain.TargetContext{UserConfigRoot: other}}); err == nil {
		t.Fatal("removal accepted conflicting captured context")
	}
	plan.NativeRegistryRoot = root
	plan.TargetContext.UserConfigRoot = other
	if _, err := New().InspectNativeRegistry(t.Context(), env, client, plan, nil); err == nil {
		t.Fatal("inspection accepted conflicting captured context")
	}
	if _, err := New().Activate(t.Context(), env, domain.ActivationRequest{Client: client, Plan: plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientCopilot}, BackendExecutable: "copilot"}); err == nil {
		t.Fatal("activation accepted conflicting captured context")
	}
	if len(runner.commands) != 0 {
		t.Fatal("conflicting root reached native command")
	}
	// A recorded removal root remains authoritative when the direct caller has
	// no detected client root; ambient configuration must not take its place.
	if _, err := New().Deactivate(t.Context(), env, domain.DeactivationRequest{NativeRegistryRoot: root, CurrentActivation: domain.ActivationActive, Confirmed: true, PhysicalArtifactID: "fixture", BackendExecutable: "copilot"}); err != nil {
		t.Fatal(err)
	}
	for _, command := range runner.commands {
		if !slices.Contains(command.Env, "COPILOT_HOME="+root) {
			t.Fatal("recorded removal root not bound")
		}
	}
}
