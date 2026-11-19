package windsurf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type devinSkillMetadataABAIO struct {
	nativeconfig.FileIO
	live                  string
	original, substituted []byte
	reads                 int
}

func (files *devinSkillMetadataABAIO) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	if filepath.Base(path) != "SKILL.md" {
		file, err := nativeconfig.New().ReadExactFileBounded(path, limit)
		return file.Body, file.Mode, file.Exists, err
	}
	files.reads++
	if err := os.WriteFile(files.live, files.substituted, 0600); err != nil {
		return nil, 0, false, err
	}
	file, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	restore := os.WriteFile(files.live, files.original, 0600)
	return file.Body, file.Mode, file.Exists, errors.Join(err, restore)
}

func TestDevinSkillObservationBindsMetadataDuringLiveABA(t *testing.T) {
	for _, nativeMetadata := range []string{"owned", "substituted"} {
		t.Run(nativeMetadata, func(t *testing.T) {
			request, env, row := devinSkillFixture(t)
			live := filepath.Join(request.Plan.NativeRegistryRoot, "skills", "owned-skill", "SKILL.md")
			original, err := os.ReadFile(live)
			if err != nil {
				t.Fatal(err)
			}
			files := &devinSkillMetadataABAIO{live: live, original: original,
				substituted: bytes.ReplaceAll(original, []byte("Exact owned metadata"), []byte("Substituted metadata"))}
			env.NativeConfig = nativeconfig.NewWithFileIO(files)
			if nativeMetadata == "substituted" {
				row["description"] = "Substituted metadata"
			}
			calls := 0
			env.Runner = devinSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				body, err := json.Marshal([]any{row})
				return legacyports.CommandResult{Stdout: body}, err
			})
			outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
				Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked, UserActions: []string{DevinProjectAction}}
			got, err := completeDevinSkills(t.Context(), env, request, outcome)
			if err != nil || calls != 1 || files.reads != 2 || (got.Activation == domain.ActivationActive) != (nativeMetadata == "owned") {
				t.Fatalf("live metadata ABA changed native proof: outcome=%+v calls=%d reads=%d err=%v", got, calls, files.reads, err)
			}
			body, err := os.ReadFile(live)
			if err != nil || !bytes.Equal(body, original) {
				t.Fatalf("live Skill was not restored: %v", err)
			}
		})
	}
}

type devinSkillRunner func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (run devinSkillRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return run(ctx, command)
}

func devinSkillFixture(t *testing.T) (domain.ActivationRequest, clients.Env, map[string]any) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(base, "project")
	root := filepath.Join(project, ".devin")
	active := filepath.Join(project, ".uap", "managed", "clients", "windsurf", "owned")
	body := "---\nname: owned-skill\ndescription: Exact owned metadata\n---\n# Owned\n"
	for _, dir := range []string{filepath.Join(active, "skills", "owned-skill"), filepath.Join(root, "skills", "owned-skill")} {
		globalOfflineWrite(t, filepath.Join(dir, "SKILL.md"), body)
		globalOfflineWrite(t, filepath.Join(dir, "reference.txt"), "complete resource bytes")
	}
	global := filepath.Join(base, "xdg", "devin")
	legacy := filepath.Join(base, ".codeium", "windsurf")
	for _, dir := range []string{global, legacy} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	client := domain.DetectedClient{ClientID: domain.ClientWindsurf, Version: "3000.11.205", ExecutablePath: filepath.Join(base, "bin", "devin"), ConfigRoot: legacy, NativeConfigRoot: global}
	plan := domain.DeliveryPlan{ClientID: domain.ClientWindsurf, Scope: domain.ScopeProject, PackageMode: domain.PackagePrepared,
		TargetContext:      domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: legacy},
		NativeRegistryRoot: root, TargetRoot: filepath.Dir(active), ActivePath: active, PhysicalArtifactID: "owned", DeclaredName: "owned-package",
		Status: domain.PlanReady, InstallIntent: domain.InstallIntentAutomatic, Policy: domain.PolicyAllowed,
		Authentication: domain.AuthenticationNotChecked, Warnings: []string{"authentication_not_catalog_verified"},
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "owned-skill", Support: domain.SupportPrepared}}}
	object, err := devinComponents(root).SkillObject("owned-skill", "skills/owned-skill", active)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), active)
	if err != nil {
		t.Fatal(err)
	}
	objects := []domain.NativeObjectOwnership{object, {ObjectID: "package:windsurf:owned", Kind: "managed_package_directory", LogicalName: plan.DeclaredName, Path: active, ManagedDigest: digest, ProtectionClass: "managed"}}
	request := domain.ActivationRequest{Client: client, Plan: plan, DeclaredName: plan.DeclaredName, BackendExecutable: client.ExecutablePath, VerifyOnly: true,
		Delivery: domain.StagedDelivery{ClientID: domain.ClientWindsurf, ActivePath: active, OwnedBase: plan.TargetRoot, ArtifactDigest: digest, NativeObjects: objects}, PreviousNativeObjects: objects}
	row := map[string]any{"name": "owned-skill", "description": "Exact owned metadata", "provider": "Devin", "base_dir": object.Path,
		"display_name": "owned-skill", "triggers": []any{"user", "model"}, "warnings": []any{}, "errors": []any{}}
	return request, clients.Env{NativeConfig: nativeconfig.New(), ParseSkill: conformance.ParseSkill, Snapshots: packagesnapshot.Builder{}}, row
}

func TestDevinProjectSkillsRequireFreshExactNativeAndOwnedResources(t *testing.T) {
	for _, mutation := range []string{"", "missing", "origin", "description", "warnings", "duplicate", "unknown-field", "source-resource", "native-resource", "policy-change", "closed-error", "unclosed"} {
		t.Run(mutation, func(t *testing.T) {
			request, env, row := devinSkillFixture(t)
			calls := 0
			cause := errors.New("native query refused")
			env.Runner = devinSkillRunner(func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				if !reflect.DeepEqual(command.Argv, []string{request.BackendExecutable, "skills", "list", "--json"}) || command.Dir != request.Plan.TargetContext.ProjectRoot || command.StdoutLimitBytes != 1<<20 {
					t.Fatalf("unexpected native query: %+v", command)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("native query is unbounded")
				}
				rows := make([]any, 1, 2)
				rows[0] = row
				switch mutation {
				case "missing":
					rows = rows[:0]
				case "origin":
					row["provider"] = "Windsurf"
				case "description":
					row["description"] = "other source"
				case "warnings":
					row["warnings"] = []any{"disabled"}
				case "duplicate":
					rows = append(rows, row)
				case "unknown-field":
					row["disabled"] = false
				case "source-resource":
					globalOfflineWrite(t, filepath.Join(request.Plan.ActivePath, "skills", "owned-skill", "reference.txt"), "changed")
				case "native-resource":
					globalOfflineWrite(t, filepath.Join(request.Plan.NativeRegistryRoot, "skills", "owned-skill", "reference.txt"), "changed")
				case "policy-change":
					globalOfflineWrite(t, filepath.Join(request.Plan.NativeRegistryRoot, "config.json"), `{"unknown":true}`)
				case "closed-error":
					return legacyports.CommandResult{CleanupComplete: true}, cause
				case "unclosed":
					return legacyports.CommandResult{}, cause
				}
				body, err := json.Marshal(rows)
				if err != nil {
					t.Fatal(err)
				}
				return legacyports.CommandResult{Stdout: body}, nil
			})
			outcome, err := New().Activate(t.Context(), env, request)
			if calls != 1 {
				t.Fatalf("expected one fresh native query, got %d: %v", calls, err)
			}
			if mutation == "" {
				if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || outcome.Authentication != domain.AuthenticationNotRequired || len(outcome.UserActions) != 0 {
					t.Fatalf("fresh exact discovery did not qualify: %+v %v", outcome, err)
				}
			} else if outcome.Activation == domain.ActivationActive || mutation == "unclosed" && (!domain.IsUnclosedClientProbe(err) || outcome.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatalf("unqualified discovery promoted: %+v %v", outcome, err)
			}
		})
	}
}

func TestDevinProjectSkillsPreserveUnknownAndExplicitAuthority(t *testing.T) {
	for _, mutation := range []string{"missing-parser", "missing-snapshots", "version", "mixed", "user", "unknown-policy", "attested", "unknown-auth", "explicit-auth", "pending-action"} {
		t.Run(mutation, func(t *testing.T) {
			request, env, row := devinSkillFixture(t)
			outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed,
				Authentication: domain.AuthenticationNotChecked, UserActions: []string{DevinProjectAction, "keep pending action"}}
			switch mutation {
			case "missing-parser":
				env.ParseSkill = nil
			case "missing-snapshots":
				env.Snapshots = nil
			case "version":
				request.Client.Version = "3000.11.234"
			case "mixed":
				request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportPrepared})
			case "user":
				request.Plan.Scope = domain.ScopeUser
			case "unknown-policy":
				globalOfflineWrite(t, filepath.Join(request.Plan.NativeRegistryRoot, "config.json"), `{"unknown":true}`)
			case "attested":
				outcome.AuthenticationAttested = true
			case "unknown-auth":
				request.Plan.Warnings = append(request.Plan.Warnings, "authentication_requirement_unknown")
			case "explicit-auth":
				request.Plan.Authentication, outcome.Authentication = domain.AuthenticationPending, domain.AuthenticationPending
			}
			calls := 0
			env.Runner = devinSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				body, _ := json.Marshal([]any{row})
				return legacyports.CommandResult{Stdout: body}, nil
			})
			got, err := completeDevinSkills(t.Context(), env, request, outcome)
			if err != nil {
				t.Fatal(err)
			}
			eligible := mutation == "unknown-auth" || mutation == "explicit-auth" || mutation == "pending-action"
			if eligible {
				if calls != 1 || got.Activation != domain.ActivationActive || !reflect.DeepEqual(got.UserActions, []string{"keep pending action"}) || got.Authentication != outcome.Authentication && mutation != "pending-action" {
					t.Fatalf("explicit authority lost: %+v", got)
				}
			} else if calls != 0 || !reflect.DeepEqual(got, outcome) {
				t.Fatalf("unknown or ineligible authority observed/promoted: %+v", got)
			}
		})
	}
}

func TestDevinProjectSkillsAllowOnlyNativeGlobalSchemaMarkerMigration(t *testing.T) {
	for _, mutation := range []string{"version", "absent-version", "public-mode", "project-config", "changed-policy", "unknown-field", "wrong-version", "string-version", "existing-version"} {
		t.Run(mutation, func(t *testing.T) {
			request, env, row := devinSkillFixture(t)
			path := filepath.Join(request.Client.NativeConfigRoot, "config.json")
			before := `{"auto_update":false}`
			if mutation == "existing-version" {
				before = `{"auto_update":false,"version":1}`
			}
			if mutation != "absent-version" {
				globalOfflineWrite(t, path, before)
			}
			if mutation == "public-mode" {
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			}
			env.Runner = devinSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				target, after := path, "{\n  \"auto_update\": false,\n  \"version\": 1\n}\n"
				switch mutation {
				case "absent-version":
					after = "{\n  \"version\": 1\n}\n"
				case "project-config":
					target = filepath.Join(request.Plan.NativeRegistryRoot, "config.json")
				case "changed-policy":
					after = `{"auto_update":true,"version":1}`
				case "unknown-field":
					after = `{"auto_update":false,"version":1,"unknown":true}`
				case "wrong-version":
					after = `{"auto_update":false,"version":2}`
				case "string-version":
					after = `{"auto_update":false,"version":"1"}`
				}
				globalOfflineWrite(t, target, after)
				if err := os.Chmod(target, 0600); err != nil {
					t.Fatal(err)
				}
				body, _ := json.Marshal([]any{row})
				return legacyports.CommandResult{Stdout: body}, nil
			})
			outcome, err := New().Activate(t.Context(), env, request)
			if mutation == "version" || mutation == "absent-version" || mutation == "public-mode" {
				if err != nil || outcome.Activation != domain.ActivationActive || outcome.Authentication != domain.AuthenticationNotRequired || len(outcome.UserActions) != 0 {
					t.Fatalf("native schema marker prevented completion: %+v %v", outcome, err)
				}
			} else if err == nil || outcome.Activation == domain.ActivationActive {
				t.Fatalf("native policy mutation accepted: %+v %v", outcome, err)
			}
		})
	}
}

func TestDevinInitialConfigMigrationPreservesExistingFileMode(t *testing.T) {
	before := nativeconfig.FileSnapshot{Exists: true, Mode: 0600, Body: []byte(`{"auto_update":false}`)}
	after := nativeconfig.FileSnapshot{Exists: true, Mode: 0644, Body: []byte(`{"auto_update":false,"version":1}`)}
	if devinInitialConfigMigration(before, after) {
		t.Fatal("schema marker authorized a mode change")
	}
}
