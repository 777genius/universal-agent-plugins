package providers

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type preparedSkillCatalogRunner func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (run preparedSkillCatalogRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return run(ctx, command)
}

// Exercise the actual CLI preparation wrapper and provider composition. A direct
// NativePrepared in an adapter-only test cannot catch a rejected embedded host.
func TestOpenCodePreparedCLIWrapperObservesOwnedSkills(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		for _, observation := range []string{"exact", "shadowed", "resource drift"} {
			t.Run(string(scope)+"/"+observation, func(t *testing.T) {
				activator, request, nativePath := preparedOpenCodeSkillRequest(t, scope)
				reads := 0
				activator.Runner = preparedSkillCatalogRunner(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
					if command.Dir != request.Plan.NativeRegistryRoot || len(command.Argv) != 3 || command.Argv[0] != request.Client.ExecutablePath {
						t.Fatalf("native observation escaped its prepared target: %+v", command)
					}
					if slices.Equal(command.Argv[1:], []string{"debug", "config"}) {
						return legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("{}")}, nil
					}
					if !slices.Equal(command.Argv[1:], []string{"debug", "skill"}) {
						t.Fatalf("pure Skill observation invoked another native command: %v", command.Argv)
					}
					reads++
					location := nativePath
					if observation == "shadowed" {
						location = filepath.Join(request.Client.ConfigRoot, "foreign", "SKILL.md")
					}
					if observation == "resource drift" {
						writeOpenCodeTestFile(t, filepath.Join(filepath.Dir(nativePath), "reference.txt"), "changed during native observation\n")
					}
					body, err := json.Marshal([]map[string]string{{"name": "guide", "location": location, "content": "Owned guide body\n"}})
					if err != nil {
						t.Fatal(err)
					}
					return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
				})
				outcome, err := activator.Activate(t.Context(), request)
				if err != nil || reads != 1 {
					t.Fatalf("prepared provider did not observe one native Skill catalog: reads=%d outcome=%+v err=%v", reads, outcome, err)
				}
				if observation != "exact" {
					if outcome.Authentication != domain.AuthenticationNotChecked {
						t.Fatalf("unproven Skill became authentication-ready: %+v", outcome)
					}
					return
				}
				requirePreparedOpenCodeSkillReady(t, outcome)
				request.VerifyOnly, request.PreviousNativeObjects = true, slices.Clone(outcome.NativeObjects)
				repeated, err := activator.Activate(t.Context(), request)
				if err != nil || reads != 2 || repeated.NativeEffect != domain.NativeEffectUnchanged {
					t.Fatalf("repeat did not freshly verify the unchanged owned installation: reads=%d outcome=%+v err=%v", reads, repeated, err)
				}
				requirePreparedOpenCodeSkillReady(t, repeated)
			})
		}
	}
}

func preparedOpenCodeSkillRequest(t *testing.T, scope domain.InstallScope) (Activator, domain.ActivationRequest, string) {
	t.Helper()
	root := tempRoot(t)
	project, configRoot := filepath.Join(root, "project"), filepath.Join(root, "user-opencode")
	active := filepath.Join(project, "managed", "guide")
	writeOpenCodeTestFile(t, filepath.Join(active, "skills", "guide", "SKILL.md"), "---\nname: guide\ndescription: guide\n---\nOwned guide body\n")
	writeOpenCodeTestFile(t, filepath.Join(active, "skills", "guide", "reference.txt"), "owned resource\n")
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: configRoot, ExecutablePath: filepath.Join(root, "selected-opencode")}
	envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{"guide": {Name: "guide", RelativePath: "skills/guide/SKILL.md"}}}
	envelope.Manifest.Name = "guide"
	preparer := NewOpenCodeClientPreparation(registry)
	preparer.environment = []string{"PATH="}
	preparer.probe = func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		if target.Executable != client.ExecutablePath {
			t.Fatalf("preparation selected a different executable: %s", target.Executable)
		}
		return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture-native-bytes"}}, nil
	}
	host, err := preparer.PrepareClient(t.Context(), envelope, client, nil, client.ExecutablePath, false)
	if err != nil {
		t.Fatal(err)
	}
	requireDetachedOpenCodeSnapshot(t, host)
	client.OpenCodeHost = host
	target := domain.TargetContext{Scope: scope}
	nativeRoot, nativePath := configRoot, filepath.Join(configRoot, "skills", "guide", "SKILL.md")
	if scope == domain.ScopeProject {
		target.ProjectRoot, nativeRoot = project, project
		nativePath = filepath.Join(project, ".opencode", "skills", "guide", "SKILL.md")
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Status: domain.PlanReady, Scope: scope,
		TargetContext: target, NativeRegistryRoot: nativeRoot, ActivePath: active, TargetRoot: filepath.Dir(active), OpenCodeHost: host,
		PhysicalArtifactID: "fixture-guide", Policy: domain.PolicyAllowed,
		InstallIntent: domain.InstallIntentAutomatic, Authentication: domain.AuthenticationNotChecked,
		Warnings:   []string{"authentication_not_catalog_verified"},
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "guide", Support: domain.SupportPrepared}}}
	if err := opencode.ProjectOpenCodeNative(active, envelope, plan, ""); err != nil {
		t.Fatal(err)
	}
	objects, err := opencode.BuildOpenCodeNativeObjects(active, envelope, plan)
	if err != nil {
		t.Fatal(err)
	}
	// Use the real assembler: every production delivery also owns its package.
	delivery, err := testStager(Stager{Registry: registry}).assembleDelivery(t.Context(), envelope, plan, active, objects)
	if err != nil || len(delivery.NativeObjects) != 2 || delivery.NativeObjects[0].Kind != "managed_package_directory" {
		t.Fatalf("actual staged receipt composition: %+v %v", delivery, err)
	}
	request := domain.ActivationRequest{Client: client, Plan: plan, DeclaredName: "guide", Delivery: delivery}
	kernel := nativeconfig.New()
	return Activator{Registry: registry, NativeConfig: &kernel}, request, nativePath
}

func requireDetachedOpenCodeSnapshot(t *testing.T, host domain.OpenCodeHostAuthority) {
	t.Helper()
	authority, ok := host.(interface {
		NativeSnapshot() *opencodehost.NativePrepared
	})
	if !ok {
		t.Fatal("actual prepared CLI wrapper has no immutable native snapshot")
	}
	snapshot := authority.NativeSnapshot()
	if snapshot == nil || snapshot.Evidence().Version != "1.18.34" {
		t.Fatal("prepared snapshot lost its measured version")
	}
	*snapshot = opencodehost.NativePrepared{}
	if authority.NativeSnapshot().Evidence().Version != "1.18.34" || host.ValidateNative(true, nil) != nil {
		t.Fatal("mutating a returned snapshot changed the frozen CLI wrapper")
	}
	var absent *opencodehost.NativePrepared
	if absent.NativeSnapshot() != nil {
		t.Fatal("nil native authority acquired a snapshot")
	}
}

func requirePreparedOpenCodeSkillReady(t *testing.T, outcome domain.ActivationOutcome) {
	t.Helper()
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled ||
		outcome.Authentication != domain.AuthenticationNotRequired || outcome.Policy != domain.PolicyAllowed ||
		outcome.ActivationAttested || outcome.AuthenticationAttested || len(outcome.NativeObjects) != 2 || len(opencode.OpenCodeObjects(outcome.NativeObjects)) != 1 {
		t.Fatalf("fresh native Skill installation is not ready: %+v", outcome)
	}
}
