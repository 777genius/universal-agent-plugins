package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Native readiness is an inert boundary here; acquisition, sealing, ownership,
// retained review and accepted repair all use the production Skills/SDK path.
// The actual client classifier still owns whether acting setup is supported.
type readySetupClaude struct {
	*claude.Adapter
	ready                 bool
	observeErr            error
	effects, observations int
}

func (adapter *readySetupClaude) Activate(_ context.Context, _ clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.VerifyOnly {
		adapter.observations++
		if adapter.observeErr != nil {
			return domain.ActivationOutcome{}, adapter.observeErr
		}
	} else {
		adapter.effects++
		adapter.ready = true
	}
	activation := domain.ActivationPrepared
	if adapter.ready {
		activation = domain.ActivationActive
	}
	return domain.ActivationOutcome{Activation: activation, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotRequired, NativeEffect: domain.NativeEffectUnchanged}, nil
}

func readySetupSkillsFixture(t *testing.T) (savedJourney, SkillsBatchRequest, *readySetupClaude) {
	t.Helper()
	_, journey, source, _ := managementMutableWebsite(t)
	adapter := &readySetupClaude{Adapter: claude.New()}
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := journey.cli.app.DependenciesRoot
	journey.cli.app.ClientRegistry = registry
	journey.cli.app.Installer, err = installer.New(installer.Config{StateRoot: root, StateFile: journey.cli.store.Path, LockFile: filepath.Join(root, "mutation.lock"), OperationsDir: journey.cli.operations, PluginDataBase: filepath.Join(root, "plugin-data"), ManagedRoot: journey.cli.app.ManagedRoot, TempRoot: filepath.Join(root, "installer-tmp"), Registry: registry, Runner: journey.runner, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"claude"}}
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	result, err := plan.Apply(t.Context())
	if err != nil || !skillsNativeReady(result.Items) {
		t.Fatalf("initial native installation=%+v,%v", result, err)
	}
	return journey, request, adapter
}

func TestSkillsReadyRepeatReviewsFreshlyPreparedSetupBeforeAcceptance(t *testing.T) {
	journey, request, adapter := readySetupSkillsFixture(t)
	binding := savedJourneyBinding(t, journey.state(t), "claude")
	if binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired {
		t.Fatal(binding)
	}
	adapter.ready = false // Native setup disappeared; saved lifecycle stays Active.
	effects := adapter.effects
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != "repair" {
		t.Fatalf("fresh Prepared setup was not reviewed: %+v", actions)
	}
	if err := plan.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	if adapter.effects != effects || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("planning changed setup or saved readiness before acceptance")
	}
	result, err := plan.Apply(t.Context())
	if err != nil || !skillsNativeReady(result.Items) || adapter.effects != effects+1 {
		t.Fatalf("same accepted repeat did not complete native setup: %+v,%v effects=%d", result, err, adapter.effects)
	}
	if !adapter.ready || result.Items[0].Action != "repair" {
		t.Fatal("repair did not finish fresh native setup")
	}
	repeat, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer repeat.Close()
	if actions := repeat.Actions(); len(actions) != 1 || actions[0].Action != "unchanged" {
		t.Fatalf("verified repeat scheduled setup: %+v", actions)
	}
}

func TestSkillsReadyRepeatUnavailableObservationDoesNotAuthorizeSetup(t *testing.T) {
	journey, request, adapter := readySetupSkillsFixture(t)
	adapter.ready = false
	adapter.observeErr = errors.New("inert native inventory unavailable")
	effects := adapter.effects
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != "unchanged" {
		t.Fatalf("unavailable observation invented setup candidate: %+v", actions)
	}
	result, err := plan.Apply(t.Context())
	if err != nil || skillsNativeReady(result.Items) || adapter.effects != effects || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatalf("unavailable observation mutated or fabricated readiness: %+v,%v", result, err)
	}
}

func TestSkillsReadyRepeatUnclosedObservationStopsReview(t *testing.T) {
	journey, request, adapter := readySetupSkillsFixture(t)
	adapter.ready = false
	adapter.observeErr = &domain.UnclosedClientProbe{Operation: "inert native observation", Err: errors.New("fixture closure unknown")}
	effects := adapter.effects
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if plan != nil {
		plan.Close()
	}
	if !domain.IsUnclosedClientProbe(err) {
		t.Fatalf("unclosed observation was suppressed: %v", err)
	}
	if adapter.effects != effects || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("unclosed observation changed setup or saved state")
	}
}

func TestSkillsUnchangedObservationPropagatesUnclosedProbe(t *testing.T) {
	journey, request, adapter := readySetupSkillsFixture(t)
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != "unchanged" {
		t.Fatal(actions)
	}
	adapter.observeErr = &domain.UnclosedClientProbe{Operation: "inert final observation", Err: errors.New("fixture closure unknown")}
	effects := adapter.effects
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	_, err = plan.observeUnchangedSkills(t.Context())
	if !domain.IsUnclosedClientProbe(err) {
		t.Fatalf("unclosed final observation was suppressed: %v", err)
	}
	if adapter.effects != effects || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("unclosed final observation changed saved setup")
	}
}
