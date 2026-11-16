package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func observedOpenCodeSkillFixture(t *testing.T) (domain.ActivationRequest, clients.Env, domain.ActivationOutcome) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "managed", "probe")
	source := filepath.Join(active, "skills", "probe")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"SKILL.md":      "---\nname: probe\ndescription: probe\n---\nExpected body\n",
		"reference.txt": "owned resource bytes\n",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	request := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: filepath.Join(root, "user-opencode"), ExecutablePath: filepath.Join(root, "opencode")},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Status: domain.PlanReady, Scope: domain.ScopeProject,
			TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: root}, NativeRegistryRoot: root,
			ActivePath: active, InstallIntent: domain.InstallIntentAutomatic, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed,
			Warnings:   []string{"authentication_not_catalog_verified"},
			Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "probe", Support: domain.SupportPrepared}}},
		Delivery:     domain.StagedDelivery{ClientID: domain.ClientOpenCode, ActivePath: active, OwnedBase: filepath.Dir(active)},
		DeclaredName: "probe",
	}
	qualifyOpenCodeObservation(&request, "1.18.34")
	envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{"probe": {Name: "probe", RelativePath: "skills/probe/SKILL.md"}}}
	if err := ProjectOpenCodeNative(active, envelope, request.Plan, ""); err != nil {
		t.Fatal(err)
	}
	objects, err := BuildOpenCodeNativeObjects(active, envelope, request.Plan)
	if err != nil {
		t.Fatal(err)
	}
	request.Delivery.NativeObjects = objects
	env := clients.Env{NativeConfig: nativeconfig.New()}
	if err := ActivateOpenCodeNativeWithKernel(t.Context(), request, env.NativeConfig); err != nil {
		t.Fatal(err)
	}
	outcome := preparedOpenCodeObservation()
	outcome.Authentication, outcome.NativeObjects = domain.AuthenticationNotChecked, slices.Clone(objects)
	return request, env, outcome
}

func TestObservedOpenCodeSkillAuthenticationRequiresFullOwnedProof(t *testing.T) {
	for _, test := range []struct {
		name          string
		change        func(*domain.ActivationRequest, *domain.ActivationOutcome)
		allow         bool
		driftResource bool
		keepOutcome   bool
	}{
		{name: "fresh owned prepared Skill", allow: true},
		{name: "unknown catalog", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.Warnings = append(r.Plan.Warnings, "authentication_requirement_unknown")
		}},
		{name: "missing catalog marker", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.Warnings = nil }},
		{name: "explicit plan auth", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.Authentication = domain.AuthenticationPending
		}},
		{name: "explicit outcome auth", change: func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) {
			o.Authentication = domain.AuthenticationPending
		}},
		{name: "unsupported plan", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.Status = domain.PlanUnsupported }},
		{name: "unknown origin", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.Components[0].Source = "extension"
		}},
		{name: "unsupported Skill", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.Components[0].Support = domain.SupportUnsupported
		}},
		{name: "activation attested", change: func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.ActivationAttested = true }},
		{name: "authentication attested", change: func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.AuthenticationAttested = true }},
		{name: "request attested", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.ActivationComplete = true }},
		{name: "plan policy not allowed", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.Policy = "" }},
		{name: "policy not allowed", change: func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.Policy = "" }},
		{name: "uncertain effect", change: func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) {
			o.NativeEffect = domain.NativeEffectUncertain
		}},
		{name: "unknown effect", change: func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.NativeEffect = "" }},
		{name: "no receipts", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Delivery.NativeObjects = nil }},
		{name: "duplicate receipt", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Delivery.NativeObjects = append(r.Delivery.NativeObjects, r.Delivery.NativeObjects[0])
		}},
		{name: "foreign kind", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Delivery.NativeObjects[0].Kind = "foreign"
		}},
		{name: "foreign identity", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Delivery.NativeObjects[0].ObjectID = "foreign"
		}},
		{name: "foreign path", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Delivery.NativeObjects[0].Path = r.Client.ConfigRoot
		}},
		{name: "foreign protection", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Delivery.NativeObjects[0].ProtectionClass = "foreign"
		}},
		{name: "user modified", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Delivery.NativeObjects[0].UserModified = true
		}},
		{name: "wrong digest", change: func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Delivery.NativeObjects[0].ManagedDigest = "sha256:wrong"
		}},
		{name: "outcome receipt differs", keepOutcome: true, change: func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.NativeObjects = nil }},
		{name: "resource changed during native read", driftResource: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, env, outcome := observedOpenCodeSkillFixture(t)
			path := filepath.Join(request.Plan.NativeRegistryRoot, ".opencode", "skills", "probe", "SKILL.md")
			if test.change != nil {
				test.change(&request, &outcome)
			}
			if !test.keepOutcome {
				outcome.NativeObjects = slices.Clone(request.Delivery.NativeObjects)
			}
			want := outcome.Authentication
			if test.allow {
				want = domain.AuthenticationNotRequired
			}
			env.Runner = skillObservationRunner(func(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				if command.Dir != request.Plan.NativeRegistryRoot || len(command.Argv) != 3 || command.Argv[0] != request.Client.ExecutablePath {
					t.Fatal("native observation escaped selected authority")
				}
				if slices.Equal(command.Argv[1:], []string{"debug", "config"}) {
					return legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("{}")}, nil
				}
				if !slices.Equal(command.Argv[1:], []string{"debug", "skill"}) {
					t.Fatalf("pure Skill observation started another command: %v", command.Argv)
				}
				if test.driftResource {
					if err := os.WriteFile(filepath.Join(filepath.Dir(path), "reference.txt"), []byte("changed resource\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				body, err := json.Marshal([]openCodeObservedSkill{{Name: "probe", Location: path, Content: "Expected body\n"}})
				if err != nil {
					t.Fatal(err)
				}
				return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
			})
			actual, err := observeOpenCodeNativeComponents(t.Context(), env, request, outcome)
			if err != nil || actual.Authentication != want {
				t.Fatalf("authentication=%s want=%s err=%v outcome=%+v", actual.Authentication, want, err, actual)
			}
			if test.allow && (actual.Activation != domain.ActivationActive || actual.ActivationAttested || actual.AuthenticationAttested) {
				t.Fatalf("fresh owned Skill did not qualify native installation: %+v", actual)
			}
		})
	}
}

// Native Skill.all() can return the owned row after effective policy or a
// same-name external Skill changed. Owned bytes alone cannot qualify that row.
func TestObservedOpenCodeSkillsRecheckLatePolicyAndAliases(t *testing.T) {
	for _, test := range []struct {
		name, config, alias string
		allow               bool
	}{
		{name: "unchanged", allow: true},
		{name: "benign_config_change", config: `{"default_agent":"build","agent":{},"mode":{}}`, allow: true},
		{name: "late_policy", config: `{"permission":{"skill":"deny"}}`},
		{name: "late_agent_policy", config: `{"agent":{"build":{"permission":{"skill":"deny"}}}}`},
		{name: "late_mode_tools", config: `{"mode":{"build":{"tools":{"skill":false}}}}`},
		{name: "late_agents_alias", alias: ".agents"},
		{name: "late_claude_alias", alias: ".claude"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, env, outcome := observedOpenCodeSkillFixture(t)
			skillRoot := request.Delivery.NativeObjects[0].Path
			before, err := shared.DigestSkillDirectory(skillRoot)
			if err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(request.Client.ConfigRoot, "opencode.json")
			if err := os.MkdirAll(request.Client.ConfigRoot, 0700); err != nil {
				t.Fatal(err)
			}
			initialConfig := `{"agent":{},"mode":{}}`
			if err := os.WriteFile(configPath, []byte(initialConfig), 0600); err != nil {
				t.Fatal(err)
			}
			catalog, err := json.Marshal([]openCodeObservedSkill{{Name: "probe", Location: filepath.Join(skillRoot, "SKILL.md"), Content: "Expected body\n"}})
			if err != nil {
				t.Fatal(err)
			}
			foreign := []byte("---\nname: probe\ndescription: Foreign Skill\n---\nForeign body\n")
			var aliasPath string
			var probeContext context.Context
			var calls []string
			env.Runner = skillObservationRunner(func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				if command.Dir != request.Plan.NativeRegistryRoot || len(command.Argv) != 3 || command.Argv[0] != request.Client.ExecutablePath || command.StdoutLimitBytes == 0 {
					t.Fatal("native revalidation escaped selected bounded authority")
				}
				if _, bounded := ctx.Deadline(); !bounded {
					t.Fatal("native revalidation has no shared deadline")
				}
				if probeContext == nil {
					probeContext = ctx
				} else if ctx != probeContext {
					t.Fatal("native revalidation replaced its original bounded context")
				}
				calls = append(calls, command.Argv[1]+" "+command.Argv[2])
				if slices.Equal(command.Argv[1:], []string{"debug", "config"}) {
					body, err := os.ReadFile(configPath)
					if err != nil {
						t.Fatal(err)
					}
					return legacyports.CommandResult{CleanupComplete: true, Stdout: body}, nil
				}
				if !slices.Equal(command.Argv[1:], []string{"debug", "skill"}) {
					t.Fatalf("pure Skill observation started another command: %v", command.Argv)
				}
				if test.config != "" {
					if err := os.WriteFile(configPath, []byte(test.config), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if test.alias != "" {
					aliasPath = filepath.Join(request.Plan.NativeRegistryRoot, test.alias, "skills", "probe", "SKILL.md")
					if err := os.MkdirAll(filepath.Dir(aliasPath), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(aliasPath, foreign, 0600); err != nil {
						t.Fatal(err)
					}
				}
				return legacyports.CommandResult{CleanupComplete: true, Stdout: catalog}, nil
			})
			actual, err := observeOpenCodeNativeComponents(t.Context(), env, request, outcome)
			after, digestErr := shared.DigestSkillDirectory(skillRoot)
			if digestErr != nil || after != before || !slices.Equal(actual.NativeObjects, outcome.NativeObjects) {
				t.Fatalf("native observation changed owned Skill bytes or receipts: %v", digestErr)
			}
			wantConfig := initialConfig
			if test.config != "" {
				wantConfig = test.config
			}
			if body, readErr := os.ReadFile(configPath); readErr != nil || string(body) != wantConfig {
				t.Fatalf("native observation changed external policy: %v", readErr)
			}
			if test.alias != "" {
				if body, readErr := os.ReadFile(aliasPath); readErr != nil || string(body) != string(foreign) {
					t.Fatalf("late external alias was not preserved: %v", readErr)
				}
			}
			wantActivation, wantAuthentication := domain.ActivationPrepared, domain.AuthenticationNotChecked
			if test.allow {
				wantActivation, wantAuthentication = domain.ActivationActive, domain.AuthenticationNotRequired
			}
			if err != nil || actual.Activation != wantActivation || actual.Authentication != wantAuthentication || actual.Verification != domain.VerificationInstalled || actual.Policy != domain.PolicyAllowed || actual.NativeEffect != domain.NativeEffectUnchanged || actual.ActivationAttested || actual.AuthenticationAttested {
				t.Fatalf("late native state was qualified incorrectly: %+v err=%v", actual, err)
			}
			if !slices.Equal(calls, []string{"debug config", "debug skill", "debug config"}) {
				t.Fatalf("unexpected final policy observation sequence: %v", calls)
			}
		})
	}
}
