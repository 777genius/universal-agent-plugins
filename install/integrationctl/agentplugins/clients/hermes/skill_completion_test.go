package hermes

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type hermesSkillRunner func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (run hermesSkillRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return run(ctx, command)
}

func hermesObservationFixture(t *testing.T, scope domain.InstallScope) (domain.ActivationRequest, clients.Env, map[string]any) {
	t.Helper()
	base := canonicalTempDir(t)
	profile, project := filepath.Join(base, "profile"), filepath.Join(base, "project")
	for _, path := range []string{profile, project, filepath.Join(base, "source"), filepath.Join(base, "managed")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HERMES_HOME", profile)
	t.Setenv("HERMES_MANAGED_DIR", filepath.Join(base, "managed"))
	t.Setenv("HERMES_MANAGED", "")
	t.Setenv("HERMES_CONFIG_BACKEND", "file")
	selected := profile
	if scope == domain.ScopeProject {
		selected = project
	}
	f := newHermesSkillFixture(t, selected, scope)
	if err := applyHermesSkills(t, f, nil, f.objects); err != nil {
		t.Fatal(err)
	}
	plan := f.plan
	plan.Status, plan.PackageMode, plan.Policy = domain.PlanReady, domain.PackagePrepared, domain.PolicyAllowed
	plan.InstallIntent, plan.Authentication = domain.InstallIntentAutomatic, domain.AuthenticationNotChecked
	plan.Warnings = []string{"authentication_not_catalog_verified"}
	plan.TargetContext.UserConfigRoot = profile
	plan.TargetRoot, plan.PhysicalArtifactID, plan.DeclaredName = filepath.Dir(plan.ActivePath), "fixture", "fixture"
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), plan.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	objects := append(slices.Clone(f.objects), domain.NativeObjectOwnership{ObjectID: "package:hermes:fixture",
		Kind: "managed_package_directory", LogicalName: "fixture", Path: plan.ActivePath, ManagedDigest: digest, ProtectionClass: "managed"})
	request := domain.ActivationRequest{Plan: plan, DeclaredName: "fixture",
		Client: domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: profile, ExecutablePath: filepath.Join(base, "bin", "hermes"),
			Status: domain.DetectionDetected, Surfaces: []domain.ClientSurface{{ID: "hermes_cli", Detected: true}}},
		Delivery: domain.StagedDelivery{ClientID: domain.ClientHermes, ActivePath: plan.ActivePath, OwnedBase: plan.TargetRoot,
			ArtifactDigest: digest, NativeObjects: objects}, PreviousNativeObjects: objects}
	body, err := os.ReadFile(f.skill)
	if err != nil {
		t.Fatal(err)
	}
	projectValue, trusted, tier := "", false, 1
	if scope == domain.ScopeProject {
		projectValue, trusted, tier = project, true, 0
	}
	row := map[string]any{"name": "document", "path": f.skill, "digest": fmt.Sprintf("%x", sha256.Sum256(body)), "tier": tier}
	doc := map[string]any{"profile": profile, "project": projectValue, "project_trusted": trusted, "skills": []any{row}}
	return request, clients.Env{NativeConfig: nativeconfig.New()}, doc
}

func hermesObservationRunner(t *testing.T, request domain.ActivationRequest, doc map[string]any, mutate func(legacyports.CommandResult) (legacyports.CommandResult, error)) hermesSkillRunner {
	t.Helper()
	source := filepath.Join(filepath.Dir(request.Client.ConfigRoot), "source")
	python := filepath.Join(filepath.Dir(request.Client.ConfigRoot), "python")
	calls := 0
	return func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		calls++
		deadline, bounded := ctx.Deadline()
		if !bounded || time.Until(deadline) > 31*time.Second || command.StdoutLimitBytes != 1<<20 {
			t.Fatal("native Skill observation lost its process/output bounds")
		}
		cwd := request.Plan.TargetContext.ProjectRoot
		if request.Plan.Scope == domain.ScopeUser {
			cwd = filepath.VolumeName(request.Client.ConfigRoot) + string(filepath.Separator)
		}
		if command.Dir != cwd || !slices.Contains(command.Env, "HERMES_HOME="+request.Client.ConfigRoot) {
			t.Fatal("native Skill observation selected another context")
		}
		result := legacyports.CommandResult{CleanupComplete: true}
		switch calls {
		case 1:
			if !slices.Equal(command.Argv, []string{request.Client.ExecutablePath, "--version"}) {
				t.Fatal("unexpected version command")
			}
			result.Stdout = []byte("Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d\nInstall directory: " + source + "\n")
		case 2:
			if !slices.Equal(command.Argv, []string{request.Client.ExecutablePath, "config", "path"}) {
				t.Fatal("unexpected native profile query")
			}
			result.Stdout = []byte(filepath.Join(request.Client.ConfigRoot, "config.yaml") + "\n")
		case 3:
			if !slices.Equal(command.Argv, []string{request.Client.ExecutablePath, "--print-runtime-command"}) {
				t.Fatal("unexpected native runtime command")
			}
			result.Stdout, _ = json.Marshal([]string{python, "-I", "-c", "official native bootstrap"})
		case 4:
			selection, _ := json.Marshal(map[string]string{"document": filepath.Join(request.Plan.NativeRegistryRoot, "skills", "document", "SKILL.md")})
			if !slices.Equal(command.Argv, []string{python, "-I", "-c", hermesSkillBootstrap, source, hermesSkillObservation, string(selection), source}) {
				t.Fatal("observation did not use official installation-bound runtime API")
			}
			result.Stdout, _ = json.Marshal(doc)
			if mutate != nil {
				return mutate(result)
			}
		default:
			t.Fatal("unexpected extra native command")
		}
		return result, nil
	}
}

func TestHermesNativeSkillsRequireExactFreshInventoryAndResources(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		for _, mutation := range []string{"", "missing", "path", "digest", "tier", "duplicate", "extra-field", "profile", "project", "trust", "source-resource", "native-resource", "policy", "closed-error", "unclosed", "stderr"} {
			t.Run(string(scope)+"/"+mutation, func(t *testing.T) {
				request, env, doc := hermesObservationFixture(t, scope)
				row := doc["skills"].([]any)[0].(map[string]any)
				env.Runner = hermesObservationRunner(t, request, doc, func(result legacyports.CommandResult) (legacyports.CommandResult, error) {
					switch mutation {
					case "missing":
						doc["skills"] = []any{}
					case "path", "digest":
						row[mutation] = "different"
					case "tier":
						row["tier"] = 3
					case "duplicate":
						doc["skills"] = []any{row, row}
					case "extra-field":
						row["enabled"] = true
					case "profile", "project":
						doc[mutation] = "another scope"
					case "trust":
						doc["project_trusted"] = !doc["project_trusted"].(bool)
					case "source-resource":
						writeHermesObservationFile(t, filepath.Join(request.Plan.ActivePath, "skills", "document", "reference.txt"))
					case "native-resource":
						writeHermesObservationFile(t, filepath.Join(filepath.Dir(row["path"].(string)), "reference.txt"))
					case "policy":
						writeHermesObservationFile(t, filepath.Join(request.Client.ConfigRoot, "config.yaml"))
					case "closed-error":
						return result, errors.New("closed native refusal")
					case "unclosed":
						return legacyports.CommandResult{}, errors.New("unclosed native refusal")
					case "stderr":
						result.Stderr = []byte("unknown native warning")
					}
					result.Stdout, _ = json.Marshal(doc)
					return result, nil
				})
				outcome, err := finishHermesActivation(t.Context(), env, request, hermesPreparedSkillOutcome(), nil)
				if mutation == "" {
					if err != nil || outcome.Activation != domain.ActivationActive || outcome.Authentication != domain.AuthenticationNotRequired || len(outcome.UserActions) != 0 {
						t.Fatalf("fresh native discovery did not complete installation: %+v %v", outcome, err)
					}
				} else if outcome.Activation == domain.ActivationActive || mutation == "unclosed" && (!domain.IsUnclosedClientProbe(err) || outcome.NativeEffect != domain.NativeEffectUncertain) {
					t.Fatalf("unqualified native discovery promoted installation: %+v %v", outcome, err)
				}
			})
		}
	}
}

func writeHermesObservationFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func hermesPreparedSkillOutcome() domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
		Policy: domain.PolicyAllowed, Authentication: domain.AuthenticationNotChecked, UserActions: []string{hermesCompletedAction}}
}

func TestHermesNativeSkillsPreserveExplicitAuthority(t *testing.T) {
	for _, mutation := range []string{"mixed", "scope", "undetected", "source", "attested", "unknown-auth", "explicit-auth", "pending-action", "receipt"} {
		t.Run(mutation, func(t *testing.T) {
			request, env, doc := hermesObservationFixture(t, domain.ScopeUser)
			outcome := hermesPreparedSkillOutcome()
			switch mutation {
			case "mixed":
				request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportNative})
			case "scope":
				request.Plan.TargetContext.Scope = domain.ScopeProject
			case "undetected":
				request.Client.Status = domain.DetectionNotDetected
			case "source":
				request.Plan.Components[0].Source = "unknown"
			case "attested":
				outcome.AuthenticationAttested = true
			case "unknown-auth":
				request.Plan.Warnings = append(request.Plan.Warnings, "authentication_requirement_unknown")
			case "explicit-auth":
				outcome.Authentication = domain.AuthenticationPending
			case "pending-action":
				outcome.UserActions = append(outcome.UserActions, "Retain independent setup")
			case "receipt":
				request.Delivery.NativeObjects = request.Delivery.NativeObjects[:1]
			}
			env.Runner = hermesObservationRunner(t, request, doc, nil)
			outcome, err := completeHermesSkills(t.Context(), env, request, outcome, hermesCompletedAction)
			switch mutation {
			case "unknown-auth":
				if err != nil || outcome.Authentication != domain.AuthenticationNotChecked {
					t.Fatal("unknown authentication was cleared", outcome, err)
				}
			case "explicit-auth":
				if err != nil || outcome.Authentication != domain.AuthenticationPending {
					t.Fatal("explicit authentication was cleared", outcome, err)
				}
			case "pending-action":
				if err != nil || !slices.Equal(outcome.UserActions, []string{"Retain independent setup"}) {
					t.Fatal("independent action was cleared", outcome, err)
				}
			default:
				if outcome.Activation == domain.ActivationActive {
					t.Fatal("unsupported authority was promoted", mutation)
				}
			}
		})
	}
}

func TestHermesSkillObserverDoesNotInvokeEffectfulView(t *testing.T) {
	for _, forbidden := range []string{"skill_view", "pm.ensure", "preprocess", "_capture_required"} {
		if strings.Contains(hermesSkillObservation, forbidden) {
			t.Fatal("native discovery invokes activation/setup effects", forbidden)
		}
	}
}

func TestHermesNativeSkillObservationRequiresSelectedRuntimeAndProfile(t *testing.T) {
	for _, mutation := range []string{"version", "ambiguous-source", "native-profile", "runtime-argv", "runtime-relative", "runtime-json", "managed", "backend", "profile-environment"} {
		t.Run(mutation, func(t *testing.T) {
			request, env, doc := hermesObservationFixture(t, domain.ScopeUser)
			original := hermesObservationRunner(t, request, doc, nil)
			calls := 0
			env.Runner = hermesSkillRunner(func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				result, err := original(ctx, command)
				if len(command.Argv) == 2 && command.Argv[1] == "--version" {
					switch mutation {
					case "version":
						result.Stdout = []byte(strings.ReplaceAll(string(result.Stdout), "2bd378ed1b8d", "unknown"))
					case "ambiguous-source":
						result.Stdout = append(result.Stdout, []byte("Install directory: /another\n")...)
					}
				}
				if len(command.Argv) == 3 && mutation == "native-profile" {
					result.Stdout = []byte(filepath.Join(request.Client.ConfigRoot, "profiles", "different", "config.yaml") + "\n")
				}
				if len(command.Argv) == 2 && command.Argv[1] == "--print-runtime-command" {
					switch mutation {
					case "runtime-argv":
						result.Stdout, _ = json.Marshal([]string{request.Client.ExecutablePath, "-c", "code"})
					case "runtime-relative":
						result.Stdout, _ = json.Marshal([]string{"python", "-I", "-c", "code"})
					case "runtime-json":
						result.Stdout = []byte("[] []")
					}
				}
				return result, err
			})
			switch mutation {
			case "managed":
				writeHermesObservationFile(t, filepath.Join(os.Getenv("HERMES_MANAGED_DIR"), "config.yaml"))
			case "backend":
				t.Setenv("HERMES_CONFIG_BACKEND", "keyring")
			case "profile-environment":
				t.Setenv("HERMES_HOME", filepath.Dir(request.Client.ConfigRoot))
			}
			outcome, err := completeHermesSkills(t.Context(), env, request, hermesPreparedSkillOutcome(), hermesCompletedAction)
			if err != nil || outcome.Activation != domain.ActivationPrepared {
				t.Fatal("unqualified native runtime/profile was promoted", outcome, err)
			}
			if slices.Contains([]string{"managed", "backend", "profile-environment"}, mutation) && calls != 0 {
				t.Fatal("native command started before profile/policy admission")
			}
		})
	}
}

// A target-only fresh catalog cannot establish that the preceding trust step
// preserved every prior root or produced its required native confirmation.
func TestHermesTrustReportControlsSkillObservation(t *testing.T) {
	for _, scenario := range []string{"granted", "lost-prior", "unconfirmed", "readback", "unclosed"} {
		t.Run(scenario, func(t *testing.T) {
			request, env, doc := hermesObservationFixture(t, domain.ScopeProject)
			request.VerifyOnly, request.ProjectTrustConsent = false, true
			project, profile := request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot
			home := filepath.Join(filepath.Dir(profile), "home")
			for _, path := range []string{home, filepath.Join(project, ".git")} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			cli := &fakeHermesCLI{git: filepath.Join(home, "git"), hermes: request.Client.ExecutablePath,
				gitRoot: filepath.ToSlash(project), configPath: filepath.Join(profile, "config.yaml"),
				trusted: []string{"/srv/prior"}, discovery: true}
			cli.dropPrior = scenario == "lost-prior"
			cli.unclosedTrust = scenario == "unclosed"
			if scenario == "unconfirmed" {
				cli.trustStdout = "No exact trust confirmation"
			}
			observer := hermesObservationRunner(t, request, doc, nil)
			observing, queries := false, 0
			env.LookupExecutable = func(string) (string, error) { return cli.git, nil }
			env.Runner = hermesSkillRunner(func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				if slices.Equal(command.Argv, []string{cli.hermes, "--version"}) {
					observing = true
				}
				if observing {
					queries++
					return observer(ctx, command)
				}
				if scenario == "readback" && cli.trustCalls() != 0 && slices.Equal(command.Argv[1:], []string{"config", "get", "--json", "skills.trusted_project_dirs"}) {
					return legacyports.CommandResult{Stdout: []byte("{invalid"), CleanupComplete: true}, nil
				}
				return cli.Run(ctx, command)
			})
			initial := hermesPreparedSkillOutcome()
			initial.LocalActions = []string{"Keep independent local guidance"}
			outcome, err := finishHermesActivation(t.Context(), env, request, initial, nil)
			if scenario == "unclosed" {
				if !domain.IsUnclosedClientProbe(err) || outcome.NativeEffect != domain.NativeEffectUncertain || queries != 0 || outcome.Activation != domain.ActivationPrepared {
					t.Fatal("unclosed trust lost its typed error/effect boundary", outcome, err, queries)
				}
				return
			}
			if err != nil || cli.trustCalls() != 1 || !slices.Contains(outcome.LocalActions, "Keep independent local guidance") {
				t.Fatal("trust completion lost existing authority or guidance", outcome, err, cli.calls)
			}
			if scenario == "granted" {
				if queries != 4 || outcome.Activation != domain.ActivationActive || !slices.Contains(outcome.LocalActions, "Hermes trusted Git repository root: "+project) {
					t.Fatal("verified trust did not reach the existing observer or retain its informational guidance", outcome, queries)
				}
				return
			}
			if queries != 0 || outcome.Activation != domain.ActivationPrepared || !strings.Contains(strings.Join(outcome.UserActions, "\n"), "UAP could not confirm Hermes project trust:") || !slices.Contains(outcome.LocalActions, hermesTrustCommandAction(project)) {
				t.Fatal("unconfirmed whole-trust restoration was replaced by a target-only catalog", outcome, queries)
			}
			if scenario == "lost-prior" && !slices.Contains(outcome.LocalActions, "Previously trusted Hermes entries missing afterwards: /srv/prior") {
				t.Fatal("lost prior trust entry guidance was erased", outcome.LocalActions)
			}
		})
	}
}

func TestHermesSkillCompletionRequiresActingRequest(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, _ := hermesObservationFixture(t, scope)
			request.VerifyOnly, request.ProjectTrustConsent = true, true
			env.Runner = hermesSkillRunner(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				t.Fatal("read-only verification started native bootstrap or trust")
				return legacyports.CommandResult{}, nil
			})
			outcome, err := finishHermesActivation(t.Context(), env, request, hermesPreparedSkillOutcome(), nil)
			if err != nil || outcome.Activation != domain.ActivationPrepared || !slices.Contains(outcome.UserActions, hermesCompletedAction) {
				t.Fatal("read-only verification lost Prepared guidance", outcome, err)
			}
		})
	}
}

func TestHermesFacadeClientNeedsCompleteNativeProof(t *testing.T) {
	for _, scenario := range []string{"explicit-facade", "ide-only", "missing-executable", "relative-executable", "cannot-start", "native-mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			request, env, doc := hermesObservationFixture(t, domain.ScopeUser)
			request.Client.Surfaces = nil // The explicit installer Request has no surface field.
			switch scenario {
			case "ide-only":
				request.Client.Surfaces = []domain.ClientSurface{{ID: "hermes_ide", Detected: true}}
			case "missing-executable":
				request.Client.ExecutablePath = ""
			case "relative-executable":
				request.Client.ExecutablePath = "hermes"
			}
			original := hermesObservationRunner(t, request, doc, nil)
			calls := 0
			env.Runner = hermesSkillRunner(func(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				if scenario == "cannot-start" {
					return legacyports.CommandResult{NoChildStarted: true}, errors.New("selected file is not executable")
				}
				result, err := original(ctx, command)
				if scenario == "native-mismatch" && calls == 1 {
					result.Stdout = []byte("Another native client\n")
				}
				return result, err
			})
			outcome, err := finishHermesActivation(t.Context(), env, request, hermesPreparedSkillOutcome(), nil)
			if err != nil || (outcome.Activation == domain.ActivationActive) != (scenario == "explicit-facade") {
				t.Fatal("facade client bypassed complete native authority", outcome, err, calls)
			}
			if scenario == "explicit-facade" && calls != 4 || (scenario == "ide-only" || scenario == "missing-executable" || scenario == "relative-executable") && calls != 0 {
				t.Fatal("unexpected native completion calls", scenario, calls)
			}
		})
	}
}

func TestHermesUserPreparedSetupUsesExistingConfirmedCompletion(t *testing.T) {
	request, _, _ := hermesObservationFixture(t, domain.ScopeUser)
	request.Client.Surfaces = nil
	outcome := hermesPreparedSkillOutcome()
	if !New().CanCompletePreparedSetup(request, outcome) {
		t.Fatal("exact User Skills cannot use confirmed native completion")
	}
	for _, scenario := range []string{"verify-only", "trust-consent", "project-context", "foreign-profile", "mixed", "uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			changed, observed := request, outcome
			changed.Plan.Components = slices.Clone(request.Plan.Components)
			switch scenario {
			case "verify-only":
				changed.VerifyOnly = true
			case "trust-consent":
				changed.ProjectTrustConsent = true
			case "project-context":
				changed.Plan.TargetContext.ProjectRoot = request.Client.ConfigRoot
			case "foreign-profile":
				changed.Plan.NativeRegistryRoot = filepath.Dir(request.Client.ConfigRoot)
			case "mixed":
				changed.Plan.Components = append(changed.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportNative})
			case "uncertain":
				observed.NativeEffect = domain.NativeEffectUncertain
			}
			if New().CanCompletePreparedSetup(changed, observed) {
				t.Fatal("unqualified User completion was admitted", scenario)
			}
		})
	}
}
