package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type observedGroupCline struct {
	*cline.Adapter
	observations int
	activations  []domain.ActivationRequest
}

func (a *observedGroupCline) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, binding *domain.ClientBinding) (clients.RegistryFinding, error) {
	a.observations++
	return a.Adapter.InspectNativeRegistry(ctx, env, client, plan, binding)
}

func (a *observedGroupCline) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome, err := a.Adapter.Activate(ctx, env, req)
	a.activations = append(a.activations, req)
	return outcome, err
}

func TestProjectPreparedGroupPublicLifecycleAndHandoff(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
	codexUser := filepath.Join(f.base, "user", ".codex")
	deathWrite(t, filepath.Join(codexUser, "keep.txt"), "original Codex profile")
	adapter := &observedGroupCline{Adapter: cline.New()}
	registry, err := clients.NewRegistry(adapter, codex.New())
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Registry = registry
	f.req.Targets = []ClientTarget{{ClientID: "cline", ClientConfigRoot: f.user}, {ClientID: "codex", ClientConfigRoot: codexUser}}
	f.reopen(t)
	untouched := f.untouched(t)
	installed := f.apply(t, OpInstall)
	if installed.Outcome != OutcomeCompleted || !installed.Mutated || len(installed.Targets) != 2 || len(installed.ManualActions) == 0 {
		t.Fatalf("install %+v", installed)
	}
	nativePaths := []string{f.skill, f.config, filepath.Join(f.project, ".agents", "skills", "docs", "SKILL.md"), filepath.Join(f.project, ".codex", "config.toml")}
	native := func() map[string]string {
		out := map[string]string{}
		for _, path := range nativePaths {
			out[path] = deathRead(t, path)
		}
		return out
	}
	beforeNative := native()
	for _, operation := range []Operation{OpInstall, OpUpdate} {
		beforeState, observations, activations := f.state(t), adapter.observations, len(adapter.activations)
		f.reopen(t)
		result := f.apply(t, operation)
		if result.Outcome != OutcomeUnchanged || result.Mutated || !result.NoChange || len(result.ManualActions) == 0 {
			t.Fatalf("repeat %s mutated=%v unchanged=%v stateEqual=%v nativeEqual=%v", operation, result.Mutated, result.NoChange, f.state(t) == beforeState, reflect.DeepEqual(native(), beforeNative))
		}
		if f.state(t) != beforeState || !reflect.DeepEqual(native(), beforeNative) || adapter.observations <= observations || len(adapter.activations) <= activations || !adapter.activations[len(adapter.activations)-1].VerifyOnly {
			t.Fatalf("repeat %s did not freshly observe identical files/state", operation)
		}
		f.assertUntouched(t, untouched)
	}
	// Group repair deliberately restages managed packages. Preserve that actual
	// mutation instead of calling it unchanged because the native bytes match.
	beforeRepair := f.state(t)
	intact := f.apply(t, OpRepair)
	if intact.Outcome != OutcomeCompleted || !intact.Mutated || intact.NoChange || f.state(t) == beforeRepair || !reflect.DeepEqual(native(), beforeNative) {
		t.Fatalf("intact group repair %+v", intact)
	}
	// A successful host callback does not falsely promote Prepared or skip
	// the subsequent real adapter observation.
	callbacks := 0
	f.cfg.OnCommittedBinding = func(context.Context, BindingFacts) error { callbacks++; return nil }
	f.reopen(t)
	observations := adapter.observations
	successful := f.apply(t, OpInstall)
	if successful.Outcome != OutcomeUnchanged || callbacks < 2 || adapter.observations <= observations {
		t.Fatalf("successful callback skipped fresh core %+v callbacks=%d", successful, callbacks)
	}
	for _, target := range successful.Targets {
		if target.Activation != "prepared" || target.Verification != "installation_verified" {
			t.Fatalf("promoted prepared client %+v", target)
		}
	}
	sentinel := errors.New("host callback refused handoff")
	f.cfg.OnCommittedBinding = func(context.Context, BindingFacts) error { return sentinel }
	f.reopen(t)
	before := f.state(t)
	refused, err := f.execute(t, OpInstall)
	if !errors.Is(err, sentinel) || refused.Outcome != OutcomeIncomplete || refused.Mutated || f.state(t) != before || !reflect.DeepEqual(native(), beforeNative) {
		t.Fatalf("host refusal %+v %v", refused, err)
	}
	f.cfg.OnCommittedBinding = nil
	f.reopen(t)
	changed := deathSkill("docs", "new group revision")
	deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), changed)
	updated := f.apply(t, OpUpdate)
	if updated.Outcome != OutcomeCompleted || !updated.Mutated || updated.NoChange {
		t.Fatalf("changed source %+v", updated)
	}
	for _, path := range []string{f.skill, nativePaths[2]} {
		if deathRead(t, path) != changed {
			t.Fatal("updated skill missing", path)
		}
	}
	beforeNative = native()
	if err := os.Remove(f.skill); err != nil {
		t.Fatal(err)
	}
	// A partially altered directory is not proof that the whole owned native
	// object is absent. Preserve that refusal before testing typed absence.
	beforePartial := f.state(t)
	partial, partialErr := f.execute(t, OpRepair)
	if partialErr == nil || partial.Mutated || f.state(t) != beforePartial {
		t.Fatalf("partial object accepted %+v %v", partial, partialErr)
	}
	if err := os.RemoveAll(filepath.Dir(f.skill)); err != nil {
		t.Fatal(err)
	}
	repaired := f.apply(t, OpRepair)
	if repaired.Outcome != OutcomeCompleted || !repaired.Mutated || !reflect.DeepEqual(native(), beforeNative) {
		t.Fatalf("native repair %+v", repaired)
	}
	f.reopen(t)
	removed := f.apply(t, OpRemove)
	if removed.Outcome != OutcomeCompleted || !removed.Mutated {
		t.Fatalf("remove %+v", removed)
	}
	for _, path := range []string{f.skill, nativePaths[2]} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("owned skill survived remove", path, err)
		}
	}
	f.assertUntouched(t, untouched)
}

func TestProjectPreparedSingleReplayFreshlyVerifies(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
	adapter := &observedGroupCline{Adapter: cline.New()}
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Registry = registry
	f.reopen(t)
	other := f.untouched(t)
	f.apply(t, OpInstall)
	before, skill, config := f.state(t), deathRead(t, f.skill), deathRead(t, f.config)
	for _, operation := range []Operation{OpInstall, OpUpdate} {
		f.reopen(t)
		observations, activations := adapter.observations, len(adapter.activations)
		result := f.apply(t, operation)
		if result.Outcome != OutcomeCompleted && result.Outcome != OutcomeUnchanged || result.Mutated || f.state(t) != before || deathRead(t, f.skill) != skill || deathRead(t, f.config) != config {
			t.Fatalf("single repeat %s: %+v", operation, result)
		}
		if adapter.observations <= observations || len(adapter.activations) <= activations || !adapter.activations[len(adapter.activations)-1].VerifyOnly {
			t.Fatal("single replay skipped fresh read-only adapter verification")
		}
		f.assertUntouched(t, other)
	}
}
