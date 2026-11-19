package windsurf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func devinGlobalSkillFixture(t *testing.T) (domain.ActivationRequest, clients.Env, map[string]any, string) {
	t.Helper()
	request, env, row := devinSkillFixture(t)
	foreignProject := request.Plan.NativeRegistryRoot
	root := request.Client.NativeConfigRoot
	active := filepath.Join(filepath.Dir(filepath.Dir(root)), "independent-state", "managed", "clients", "windsurf", "owned")
	if err := os.MkdirAll(filepath.Dir(active), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(request.Plan.ActivePath, active); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "reference.txt"} {
		body, err := os.ReadFile(filepath.Join(active, "skills", "owned-skill", name))
		if err != nil {
			t.Fatal(err)
		}
		globalOfflineWrite(t, filepath.Join(root, "skills", "owned-skill", name), string(body))
	}
	request.Plan.Scope, request.Plan.TargetContext.Scope = domain.ScopeUser, domain.ScopeUser
	request.Plan.TargetContext.ProjectRoot = ""
	request.Plan.NativeRegistryRoot = root
	request.Plan.TargetRoot, request.Plan.ActivePath = filepath.Dir(active), active
	object, err := devinGlobalComponents(root, active).SkillObject("owned-skill", "skills/owned-skill", active)
	if err != nil {
		t.Fatal(err)
	}
	packageObject := request.Delivery.NativeObjects[1]
	packageObject.Path = active
	request.Delivery.ActivePath, request.Delivery.OwnedBase = active, request.Plan.TargetRoot
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{object, packageObject}
	request.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
	env.NativeConfig = nativeconfig.NewWithFileIO(&devinSkillContextFiles{kernel: env.NativeConfig})
	row["base_dir"] = object.Path
	return request, env, row, foreignProject
}

func TestDevinGlobalSkillsObserveOriginalProfileAndRejectScopeSubstitution(t *testing.T) {
	for _, mutation := range []string{"", "project-receipt", "legacy-root", "project-shadow", "native-resource", "package-resource", "global-policy", "unobserved-version"} {
		t.Run(mutation, func(t *testing.T) {
			request, env, row, foreignProject := devinGlobalSkillFixture(t)
			foreignFile := filepath.Join(foreignProject, "skills", "owned-skill", "reference.txt")
			foreignBefore, err := os.ReadFile(foreignFile)
			if err != nil {
				t.Fatal(err)
			}
			switch mutation {
			case "project-receipt":
				request.Delivery.NativeObjects[0].Kind = DevinProjectSkillKind
				request.PreviousNativeObjects[0].Kind = DevinProjectSkillKind
			case "legacy-root":
				request.Client.NativeConfigRoot = request.Client.ConfigRoot
			case "unobserved-version":
				request.Client.Version = "3000.12.58"
			}
			calls := 0
			env.Runner = devinSkillRunner(func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				if command.Dir != filepath.VolumeName(request.Client.NativeConfigRoot)+string(filepath.Separator) || !reflect.DeepEqual(command.Argv, []string{request.BackendExecutable, "skills", "list", "--json"}) || command.StdoutLimitBytes != 1<<20 {
					t.Fatalf("Global observation escaped selected original profile: %+v", command)
				}
				if _, bounded := ctx.Deadline(); !bounded {
					t.Fatal("Global native observation has no deadline")
				}
				switch mutation {
				case "project-shadow":
					row["base_dir"] = filepath.Join(foreignProject, "skills", "owned-skill")
				case "native-resource":
					globalOfflineWrite(t, filepath.Join(request.Plan.NativeRegistryRoot, "skills", "owned-skill", "reference.txt"), "changed native resource")
				case "package-resource":
					globalOfflineWrite(t, filepath.Join(request.Plan.ActivePath, "skills", "owned-skill", "reference.txt"), "changed package resource")
				case "global-policy":
					globalOfflineWrite(t, filepath.Join(request.Client.NativeConfigRoot, "config.json"), `{"unknown":true}`)
				}
				body, marshalErr := json.Marshal([]any{row})
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				return legacyports.CommandResult{Stdout: body}, nil
			})
			outcome, activationErr := New().Activate(t.Context(), env, request)
			if mutation == "" {
				if activationErr != nil || calls != 1 || outcome.Activation != domain.ActivationActive || outcome.Authentication != domain.AuthenticationNotRequired || outcome.Verification != domain.VerificationInstalled || len(outcome.UserActions) != 0 {
					t.Fatalf("exact Global observation did not qualify: %+v %v", outcome, activationErr)
				}
			} else if mutation == "project-shadow" {
				if calls != 1 || activationErr != nil || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled {
					t.Fatalf("closed Global source mismatch lost Prepared state: calls=%d outcome=%+v err=%v", calls, outcome, activationErr)
				}
			} else if mutation == "native-resource" || mutation == "package-resource" || mutation == "global-policy" {
				if calls != 1 || activationErr == nil || outcome.Activation == domain.ActivationActive {
					t.Fatalf("Global callback mutation did not reach the probe or fail: calls=%d outcome=%+v err=%v", calls, outcome, activationErr)
				}
			} else if calls != 0 || outcome.Activation == domain.ActivationActive {
				t.Fatalf("unproven Global authority promoted: %+v %v", outcome, activationErr)
			}
			foreignAfter, readErr := os.ReadFile(foreignFile)
			if readErr != nil || string(foreignBefore) != string(foreignAfter) {
				t.Fatal("Global observation changed the unrelated Project Skill")
			}
		})
	}
}

func TestDevinGlobalSkillCompletionPreservesExplicitAuthenticationAndOtherActions(t *testing.T) {
	for _, authentication := range []domain.AuthenticationState{domain.AuthenticationNotChecked, domain.AuthenticationPending} {
		t.Run(string(authentication), func(t *testing.T) {
			request, env, row, _ := devinGlobalSkillFixture(t)
			request.Plan.Authentication = authentication
			request.Plan.Warnings = append(request.Plan.Warnings, "authentication_requirement_unknown")
			outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Authentication: authentication,
				Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed,
				UserActions: []string{DevinGlobalAction, DevinProjectAction, "keep pending action"}}
			env.Runner = devinSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				body, _ := json.Marshal([]any{row})
				return legacyports.CommandResult{Stdout: body}, nil
			})
			got, err := completeDevinSkills(t.Context(), env, request, outcome)
			if err != nil || got.Activation != domain.ActivationActive || got.Authentication != authentication ||
				!reflect.DeepEqual(got.UserActions, []string{DevinProjectAction, "keep pending action"}) {
				t.Fatalf("Global completion discarded independent authority: %+v %v", got, err)
			}
		})
	}
}

func TestDevinPureSkillObservationAdmitsQualifiedVersions(t *testing.T) {
	for _, version := range []string{"3000.11.205", "3000.12.57"} {
		for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
			t.Run(string(scope)+"-"+version, func(t *testing.T) {
				var request domain.ActivationRequest
				if scope == domain.ScopeUser {
					request, _, _, _ = devinGlobalSkillFixture(t)
				} else {
					request, _, _ = devinSkillFixture(t)
				}
				request.Client.Version = version
				if !devinPureObservedSkills(request) {
					t.Fatal("owner-qualified exact version refused by pure Skill predicate", scope, version)
				}
			})
		}
	}
}
