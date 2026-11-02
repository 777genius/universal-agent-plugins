package usecase

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// These pure fixtures choose a prepared dialect without probing a real host.
// Both dialects accept this fixture's skills and stdio transport; native host
// qualification remains the responsibility of the host-preparation tests.
type groupProjectionHost struct {
	contracttest.OpenCodeV1Host
	dialect string
}

func (h groupProjectionHost) ConfigDialect() string { return h.dialect }

type groupProjectionFixture struct {
	service Service
	store   statev2.Store
	inputs  []AddInput
	paths   nativeconfig.Paths
}

func newGroupProjectionFixture(t *testing.T, dialect string) groupProjectionFixture {
	t.Helper()
	service, store, cursor := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode")}
	paths := nativeconfig.Paths{JSON: filepath.Join(client.ConfigRoot, "opencode.json"), JSONC: filepath.Join(client.ConfigRoot, "opencode.jsonc")}
	if err := os.MkdirAll(client.ConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.JSONC, []byte("{ // retain this comment\n\"theme\":9007199254740993,\"mcp\":{}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	input := openCodePluginInput(t, client, "1.0.0", "sha256:group-projection", "sha256:group-projection-manifest", "sh")
	input.Client.OpenCodeHost = groupProjectionHost{dialect: dialect}
	other := input
	other.Client = cursor
	transitions := opencode.NativeTransitions{PackageVerifier: service.Stager, State: transaction.Kernel{StateStore: store}, Kernel: nativeconfig.New()}
	service.Kernel.NativeRecovery = transitions
	service.Activator = providerstest.NewActivator(providers.Activator{OpenCodeTransitions: transitions})
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	inputs := []AddInput{other, input}
	installed, err := service.AddGroup(t.Context(), GroupInput{Targets: inputs, Confirmed: true, OperationGroupID: "projection-install"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range inputs {
		inputs[i].InstallationID = installed.InstallationID
	}
	return groupProjectionFixture{service: service, store: store, inputs: inputs, paths: paths}
}

func (f groupProjectionFixture) binding(t *testing.T, clientID domain.ClientID) domain.ClientBinding {
	t.Helper()
	state := f.state(t)
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID == string(clientID) {
			return binding
		}
	}
	t.Fatalf("missing %s binding", clientID)
	return domain.ClientBinding{}
}

func (f groupProjectionFixture) state(t *testing.T) domain.StateFileV2 {
	t.Helper()
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func (f groupProjectionFixture) config(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(f.paths.JSONC)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func requireGroupProjection(t *testing.T, f groupProjectionFixture, dialect string) {
	t.Helper()
	codec, err := nativeconfig.OpenCodeCodecForDialect(dialect)
	if err != nil {
		t.Fatal(err)
	}
	present, _, err := nativeconfig.New().Inspect(f.paths, codec, "docs", nil)
	if err != nil || !present {
		t.Fatalf("requested native projection %s missing: present=%t, %v", dialect, present, err)
	}
	binding := f.binding(t, domain.ClientOpenCode)
	projection, err := opencode.ReadOpenCodeProjection(binding.TargetLocator)
	if err != nil || string(projection.Dialect) != dialect {
		t.Fatalf("package projection = %+v, %v", projection, err)
	}
	if binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
		t.Fatal("completed transition retained an unresolved attempt")
	}
	if err := opencode.VerifyOpenCodeNativeObjects(f.inputs[1].Client.ConfigRoot, binding.TargetLocator, binding.NativeObjects, nativeconfig.New(), false); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"// retain this comment", `"theme":9007199254740993`} {
		if !bytes.Contains(f.config(t), []byte(fragment)) {
			t.Fatalf("foreign config changed: %s", f.config(t))
		}
	}
}

func TestGroupSameRevisionNativeProjectionRefresh(t *testing.T) {
	for _, source := range []string{"opencode_v1", "opencode_v2"} {
		t.Run(source, func(t *testing.T) {
			f := newGroupProjectionFixture(t, source)
			target := "opencode_v2"
			if source == target {
				target = "opencode_v1"
			}
			before, sibling := f.binding(t, domain.ClientOpenCode), f.binding(t, domain.ClientCursor)
			f.inputs[1].Client.OpenCodeHost = groupProjectionHost{dialect: target}
			previewState, previewConfig := f.state(t), f.config(t)
			preview, err := f.service.UpdateGroup(t.Context(), GroupInput{Targets: f.inputs, DryRun: true, OperationGroupID: "projection-preview"})
			if err != nil || preview.Mutated || preview.Targets[1].NoChange || !reflect.DeepEqual(previewState, f.state(t)) || !bytes.Equal(previewConfig, f.config(t)) {
				t.Fatalf("projection update preview: phase=%s mutated=%t, %v", preview.Phase, preview.Mutated, err)
			}
			updated, err := f.service.UpdateGroup(t.Context(), GroupInput{Targets: f.inputs, Confirmed: true, OperationGroupID: "projection-update"})
			if err != nil || updated.Phase != GroupPhaseCompleted || !updated.Mutated || updated.Targets[1].NoChange || !updated.Targets[1].Mutated {
				t.Fatalf("same-revision transition: phase=%s mutated=%t noChange=%t, %v", updated.Phase, updated.Mutated, updated.Targets[1].NoChange, err)
			}
			requireGroupProjection(t, f, target)
			after := f.binding(t, domain.ClientOpenCode)
			if !reflect.DeepEqual(before.PackageRevision, after.PackageRevision) || before.DataReceiptID == "" || before.DataReceiptID != after.DataReceiptID || managedDigest(before) == managedDigest(after) {
				t.Fatal("transition lost its revision/data identity or did not restage")
			}
			if len(before.NativeObjects) != len(after.NativeObjects) {
				t.Fatal("transition changed the ownership count")
			}
			for i, object := range before.NativeObjects {
				if object.ObjectID != after.NativeObjects[i].ObjectID {
					t.Fatal("transition changed logical ownership identity")
				}
			}
			if got := f.binding(t, domain.ClientCursor); !reflect.DeepEqual(sibling, got) || updated.Targets[0].Mutated {
				t.Fatal("transition changed the independent target")
			}
			stableState := f.state(t)
			stableConfig := f.config(t)
			repeated, err := f.service.UpdateGroup(t.Context(), GroupInput{Targets: f.inputs, Confirmed: true, OperationGroupID: "projection-stable"})
			if err != nil || repeated.Mutated || !repeated.Targets[1].NoChange {
				t.Fatalf("matching-dialect update = %+v, %v", repeated, err)
			}
			afterRepeat := f.state(t)
			if !reflect.DeepEqual(stableState, afterRepeat) || !bytes.Equal(stableConfig, f.config(t)) {
				t.Fatal("matching-dialect update created another transaction")
			}
		})
	}
}

func TestGroupProjectionRefreshKeepsExistingRuntimeCheck(t *testing.T) {
	for _, dialect := range []string{"opencode_v1", "opencode_v2"} {
		t.Run(dialect, func(t *testing.T) {
			f := newGroupProjectionFixture(t, "opencode_v1")
			manager := &failingRuntimeDataManager{PluginDataManager: f.service.PluginData.(providers.PluginDataManager)}
			f.service.PluginData = manager
			f.inputs[1].Client.OpenCodeHost = groupProjectionHost{dialect: dialect}
			before := f.state(t)
			config := f.config(t)
			result, err := f.service.UpdateGroup(t.Context(), GroupInput{Targets: f.inputs[1:], CompatibilityChecks: f.inputs, Confirmed: true, OperationGroupID: "projection-runtime"})
			if err == nil || !strings.Contains(err.Error(), "cold npm unavailable") || result.Mutated || manager.calls != 1 {
				t.Fatalf("runtime failure = %+v, %v, calls=%d", result, err, manager.calls)
			}
			after := f.state(t)
			if !reflect.DeepEqual(before, after) || !bytes.Equal(config, f.config(t)) {
				t.Fatal("runtime refusal changed recorded or native state")
			}
		})
	}
}

func TestNativeProjectionRefreshPreservesRepeatedAddAndRepair(t *testing.T) {
	for _, operation := range []string{"add", "group-add", "repair"} {
		t.Run(operation, func(t *testing.T) {
			f := newGroupProjectionFixture(t, "opencode_v1")
			f.inputs[1].Client.OpenCodeHost = groupProjectionHost{dialect: "opencode_v2"}
			f.inputs[1].Confirmed = true
			before := f.state(t)
			config := f.config(t)
			var err error
			switch operation {
			case "add":
				_, err = f.service.Add(t.Context(), f.inputs[1])
			case "group-add":
				_, err = f.service.AddGroup(t.Context(), GroupInput{Targets: f.inputs, Confirmed: true, OperationGroupID: "projection-repeat-add"})
			case "repair":
				f.inputs[1].Confirmed = true
				_, err = f.service.Repair(t.Context(), f.inputs[1])
			}
			if err != nil {
				t.Fatal(err)
			}
			after := f.state(t)
			if !reflect.DeepEqual(before, after) || !bytes.Equal(config, f.config(t)) {
				t.Fatal("non-update operation changed the installed projection")
			}
			requireGroupProjection(t, f, "opencode_v1")
		})
	}
}

func TestGroupProjectionRefreshRetainsTransitionRefusals(t *testing.T) {
	for _, scenario := range []string{"missing recorder", "late foreign edit"} {
		t.Run(scenario, func(t *testing.T) {
			f := newGroupProjectionFixture(t, "opencode_v1")
			f.inputs[1].Client.OpenCodeHost = groupProjectionHost{dialect: "opencode_v2"}
			config := f.config(t)
			before := f.state(t)
			group := GroupInput{Targets: f.inputs, Confirmed: true, OperationGroupID: "projection-refusal"}
			if scenario == "missing recorder" {
				f.service.Activator = providerstest.NewActivator(providers.Activator{})
			} else {
				group.Progress = func(event GroupProgressEvent) {
					if event.ClientID != domain.ClientOpenCode || event.Phase != GroupProgressPreparing {
						return
					}
					config = bytes.ReplaceAll(config, []byte(`"sh"`), []byte(`"foreign-sh"`))
					if err := os.WriteFile(f.paths.JSONC, config, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			result, err := f.service.UpdateGroup(t.Context(), group)
			if err == nil || result.Phase == GroupPhaseCompleted || !bytes.Equal(config, f.config(t)) {
				t.Fatalf("transition refusal lost: phase=%s, %v", result.Phase, err)
			}
			if scenario == "missing recorder" {
				if !strings.Contains(err.Error(), "durable native transition authority is required") {
					t.Fatal(err)
				}
			} else if result.Mutated || !reflect.DeepEqual(before, f.state(t)) {
				t.Fatal("late foreign native edit did not stop managed commit")
			}
		})
	}
}

// A trusted custom planner can request staging for another adapter without the
// usecase knowing its host representation or client-specific receipt kinds.
type projectionRefreshPlanner struct{ ports.DeliveryPlanner }

func (p projectionRefreshPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	plan, err := p.DeliveryPlanner.Plan(ctx, request)
	return plan.WithNativeProjectionRefresh(true), err
}

func TestTrustedPlannerProjectionRefreshDecision(t *testing.T) {
	for _, group := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "group"}[group], func(t *testing.T) {
			service, _, cursor := serviceFixture(t)
			input := addInput(t, cursor, "https://example.test/custom-planner")
			input.Confirmed = true
			if _, err := service.Add(t.Context(), input); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "custom-projection-update"
			service.Planner = projectionRefreshPlanner{service.Planner}
			if group {
				result, err := service.UpdateGroup(t.Context(), GroupInput{Targets: []AddInput{input}, Confirmed: true, OperationGroupID: "custom-projection"})
				if err != nil || !result.Mutated || result.Targets[0].NoChange {
					t.Fatalf("custom group decision = %+v, %v", result, err)
				}
			} else {
				result, err := service.Update(t.Context(), input)
				if err != nil || !result.Mutated || result.NoChange {
					t.Fatalf("custom single decision = %+v, %v", result, err)
				}
			}
		})
	}
}
