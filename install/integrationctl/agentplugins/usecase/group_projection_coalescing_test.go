package usecase

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func projectionCoalescingTrees(t *testing.T, f groupProjectionFixture) map[string]string {
	t.Helper()
	// The fixture's first root includes state, mutation receipts, package trees
	// and PLUGIN_DATA. The second owns OpenCode's native config and skills.
	trees := geminiGroupTrees(t, filepath.Dir(filepath.Dir(f.store.Path)), f.inputs[1].Client.ConfigRoot)
	// Acquiring the process mutation lock refreshes its owner token even when
	// preflight refuses. It is coordination, not persisted installation state.
	delete(trees, filepath.Join(filepath.Dir(f.store.Path), "mutation.lock"))
	return trees
}

func TestGroupProjectionCoalescingRefusesDisagreement(t *testing.T) {
	for _, refreshFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "refresh second", true: "refresh first"}[refreshFirst], func(t *testing.T) {
			f := newGroupProjectionFixture(t, "opencode_v1")
			same := f.inputs[1]
			changed := same
			changed.Client.OpenCodeHost = groupProjectionHost{dialect: "opencode_v2"}
			targets := []AddInput{same, changed}
			if refreshFirst {
				targets = []AddInput{changed, same}
			}
			state, trees := f.state(t), projectionCoalescingTrees(t, f)
			result, err := f.service.UpdateGroup(t.Context(), GroupInput{Targets: targets, CompatibilityChecks: f.inputs, Confirmed: true, OperationGroupID: "coalescing-conflict"})
			if err == nil || !strings.Contains(err.Error(), "different delivery facts") || result.Mutated {
				t.Fatalf("conflicting update decisions were coalesced: phase=%s mutated=%t, %v", result.Phase, result.Mutated, err)
			}
			if !reflect.DeepEqual(state, f.state(t)) || !reflect.DeepEqual(trees, projectionCoalescingTrees(t, f)) {
				t.Fatal("conflicting decisions changed state, packages, native config, data or receipts")
			}
		})
	}
}

func TestGroupProjectionCoalescingEquivalentDecisions(t *testing.T) {
	for _, refresh := range []bool{false, true} {
		t.Run(map[bool]string{false: "both false", true: "both true"}[refresh], func(t *testing.T) {
			f := newGroupProjectionFixture(t, "opencode_v1")
			input := f.inputs[1]
			dialect := "opencode_v1"
			if refresh {
				dialect = "opencode_v2"
				input.Client.OpenCodeHost = groupProjectionHost{dialect: dialect}
			}
			state, trees := f.state(t), projectionCoalescingTrees(t, f)
			binding, sibling := f.binding(t, domain.ClientOpenCode), f.binding(t, domain.ClientCursor)
			result, err := f.service.UpdateGroup(t.Context(), GroupInput{Targets: []AddInput{input, input}, CompatibilityChecks: f.inputs, Confirmed: true, OperationGroupID: "coalescing-equivalent"})
			if err != nil || result.Phase != GroupPhaseCompleted || result.Mutated != refresh || len(result.Targets) != 2 {
				t.Fatalf("equivalent decisions failed: phase=%s mutated=%t, %v", result.Phase, result.Mutated, err)
			}
			for _, target := range result.Targets {
				if target.Plan.RequiresNativeProjectionRefresh() != refresh || target.Mutated != refresh || target.NoChange == refresh || target.GroupPhase != GroupTargetExternalCompleted {
					t.Fatalf("coalesced target lost its shared decision/effect: %+v", target)
				}
			}
			requireGroupProjection(t, f, dialect)
			if refresh {
				after := f.binding(t, domain.ClientOpenCode)
				if len(after.Receipts) != len(binding.Receipts)+1 || result.Targets[0].Receipt.OperationID == "" || result.Targets[0].Receipt.OperationID != result.Targets[1].Receipt.OperationID {
					t.Fatal("equivalent refresh decisions did not use one physical package mutation")
				}
				if !reflect.DeepEqual(sibling, f.binding(t, domain.ClientCursor)) {
					t.Fatal("coalesced refresh changed the independent client")
				}
			} else if !reflect.DeepEqual(state, f.state(t)) || !reflect.DeepEqual(trees, projectionCoalescingTrees(t, f)) {
				t.Fatal("equivalent no-change decisions created an effect")
			}
		})
	}
}

func TestGroupProjectionCoalescingPreservesRepeatedAdd(t *testing.T) {
	for _, refreshFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "refresh second", true: "refresh first"}[refreshFirst], func(t *testing.T) {
			f := newGroupProjectionFixture(t, "opencode_v1")
			same := f.inputs[1]
			changed := same
			changed.Client.OpenCodeHost = groupProjectionHost{dialect: "opencode_v2"}
			targets := []AddInput{same, changed}
			if refreshFirst {
				targets = []AddInput{changed, same}
			}
			state, trees := f.state(t), projectionCoalescingTrees(t, f)
			result, err := f.service.AddGroup(t.Context(), GroupInput{Targets: targets, Confirmed: true, OperationGroupID: "coalescing-repeated-add"})
			if err != nil || result.Mutated || len(result.Targets) != 2 || !result.Targets[0].NoChange || !result.Targets[1].NoChange {
				t.Fatalf("repeated add acquired update-only conflict semantics: phase=%s mutated=%t, %v", result.Phase, result.Mutated, err)
			}
			if !reflect.DeepEqual(state, f.state(t)) || !reflect.DeepEqual(trees, projectionCoalescingTrees(t, f)) {
				t.Fatal("repeated add changed the retained projection or data")
			}
			requireGroupProjection(t, f, "opencode_v1")
		})
	}
}
