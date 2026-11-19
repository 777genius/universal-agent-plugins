package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func copilotProjectSkillFixture(t *testing.T) (domain.ActivationRequest, clients.Env, *copilotSkillRunner) {
	t.Helper()
	project, home := t.TempDir(), t.TempDir()
	managed := filepath.Join(project, ".uap", "managed")
	target := filepath.Join(managed, "clients", "copilot")
	active := filepath.Join(target, "package")
	layout, err := copilotComponents(project)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("---\nname: owned\ndescription: Inert Skill\nmetadata:\n  origin: source\n---\nInert instructions.\n")
	for _, root := range []string{filepath.Join(active, "skills", "owned"), filepath.Join(layout.SkillsRoot, "owned")} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "SKILL.md"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	object, err := layout.SkillObject("owned", "skills/owned", active)
	if err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, Scope: domain.ScopeProject, Status: domain.PlanReady,
		PackageMode: domain.PackageNative, InstallIntent: domain.InstallIntentAutomatic, Policy: domain.PolicyAllowed,
		Authentication: domain.AuthenticationNotChecked, Warnings: []string{"authentication_not_catalog_verified"},
		TargetContext:      domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: filepath.Join(home, ".copilot")},
		NativeRegistryRoot: project, TargetAnchor: managed, TargetRoot: target, ActivePath: active,
		PhysicalArtifactID: "package", DeclaredName: "package",
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "owned", Support: domain.SupportNative}}}
	artifact, err := (packagesnapshot.Builder{}).Build(t.Context(), active)
	if err != nil {
		t.Fatal(err)
	}
	digest := artifact.Digest
	if err := artifact.Close(); err != nil {
		t.Fatal(err)
	}
	request := domain.ActivationRequest{Plan: plan, DeclaredName: "package", VerifyOnly: true,
		Client: domain.DetectedClient{ClientID: domain.ClientCopilot, Status: domain.DetectionDetected,
			ConfigRoot: plan.TargetContext.UserConfigRoot, ExecutablePath: filepath.Join(home, "bin", "copilot"), Surfaces: []domain.ClientSurface{{ID: "copilot_cli", Detected: true}}},
		Delivery:              domain.StagedDelivery{ClientID: domain.ClientCopilot, OwnedBase: target, ActivePath: active, ArtifactDigest: digest, NativeObjects: []domain.NativeObjectOwnership{object}},
		PreviousNativeObjects: []domain.NativeObjectOwnership{object}}
	if err := os.MkdirAll(plan.TargetContext.UserConfigRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	trust, err := json.Marshal(map[string]any{"trustedFolders": []string{project}, "foreign": "retained"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plan.TargetContext.UserConfigRoot, "config.json"), trust, 0o600); err != nil {
		t.Fatal(err)
	}
	row := map[string]any{"name": "owned", "description": "Inert Skill", "path": object.Path, "source": "project", "enabled": true}
	catalog, err := json.Marshal([]any{row})
	if err != nil {
		t.Fatal(err)
	}
	runner := &copilotSkillRunner{result: legacyports.CommandResult{Stdout: catalog, CleanupComplete: true}}
	env := clients.Env{Runner: runner, NativeConfig: nativeconfig.New(), ParseSkill: conformance.ParseSkill, Snapshots: packagesnapshot.Builder{}}
	return request, env, runner
}

func TestCopilotProjectSkillsCompleteOnlyFreshOwnedCatalog(t *testing.T) {
	request, env, runner := copilotProjectSkillFixture(t)
	t.Setenv("COPILOT_HOME", "/foreign/profile")
	t.Setenv("GH_TOKEN", "not-a-real-key")
	t.Setenv("OPENAI_API_KEY", "not-a-real-key")
	out, err := New().Activate(t.Context(), env, request)
	if err != nil || out.Activation != domain.ActivationActive || out.Verification != domain.VerificationInstalled ||
		out.Authentication != domain.AuthenticationNotRequired || out.ActivationAttested || out.AuthenticationAttested || len(out.UserActions) != 0 {
		t.Fatalf("fresh catalog did not complete owned Skills: %+v %v", out, err)
	}
	want := []string{request.Client.ExecutablePath, "skill", "list", "--json"}
	if !reflect.DeepEqual(runner.command.Argv, want) || runner.command.Dir != request.Plan.TargetContext.ProjectRoot ||
		!runner.bounded || runner.command.StdoutLimitBytes != 1<<20 {
		t.Fatalf("catalog command escaped finite project scope: %+v", runner.command)
	}
	for _, item := range runner.command.Env {
		if strings.HasPrefix(item, "GH_TOKEN=") || strings.HasPrefix(item, "OPENAI_API_KEY=") || item == "COPILOT_HOME=/foreign/profile" {
			t.Fatalf("catalog inherited provider or MCP override: %q", item)
		}
	}
	if !slices.Contains(runner.command.Env, "COPILOT_OFFLINE=true") || !slices.Contains(runner.command.Env, "COPILOT_AUTO_UPDATE=false") || !slices.Contains(runner.command.Env, "COPILOT_HOME="+request.Client.ConfigRoot) {
		t.Fatal("catalog omitted selected profile/offline/updater policy")
	}
}

func TestCopilotProjectSkillsInstallAndRenderedCommandUseSameOwnedProof(t *testing.T) {
	for _, mode := range []string{"install", "rendered-command"} {
		t.Run(mode, func(t *testing.T) {
			request, env, _ := copilotProjectSkillFixture(t)
			if mode == "install" {
				request.VerifyOnly = false
				request.PreviousNativeObjects = nil
				if err := os.RemoveAll(request.Delivery.NativeObjects[0].Path); err != nil {
					t.Fatal(err)
				}
			} else {
				request.Plan.Components[0].Support = domain.SupportProjected
				request.Plan.Components[0].Source = domain.ComponentSourceCommand
				request.Plan.Components[0].SourceName = "rendered-command"
			}
			out, err := New().Activate(t.Context(), env, request)
			if err != nil || out.Activation != domain.ActivationActive || out.Verification != domain.VerificationInstalled || out.Authentication != domain.AuthenticationNotRequired {
				t.Fatalf("owned install/command proof did not complete: %+v %v", out, err)
			}
		})
	}
}

func TestCopilotProjectSkillsUnchangedRepeatRequiresAnotherCatalog(t *testing.T) {
	request, env, runner := copilotProjectSkillFixture(t)
	first, err := New().Activate(t.Context(), env, request)
	if err != nil || first.Activation != domain.ActivationActive {
		t.Fatalf("initial observation failed: %+v %v", first, err)
	}
	request.Replacing, request.PackageUnchanged = true, true
	request.PreviousNativeObjects = slices.Clone(first.NativeObjects)
	runner.command = legacyports.Command{}
	repeat, err := New().Activate(t.Context(), env, request)
	if err != nil || repeat.Activation != domain.ActivationActive || repeat.NativeEffect != domain.NativeEffectUnchanged || runner.command.Argv == nil {
		t.Fatalf("unchanged repeat omitted fresh catalog: %+v %v", repeat, err)
	}
	runner.command = legacyports.Command{}
	runner.result.Stdout = []byte("[]")
	missing, err := New().Activate(t.Context(), env, request)
	if err != nil || missing.Activation != domain.ActivationPrepared || runner.command.Argv == nil {
		t.Fatalf("saved success replaced fresh missing catalog: %+v %v", missing, err)
	}
}

func TestCopilotProjectSkillsRequireOrdinaryAuthority(t *testing.T) {
	for _, mode := range []string{"ide", "undetected", "relative-executable", "missing-parser", "missing-snapshots", "missing-runner", "missing-executable", "backend", "mixed", "empty", "duplicate", "policy", "receipt", "unknown-source", "source-name", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotProjectSkillFixture(t)
			out := copilotPreparedSkillOutcome(request)
			switch mode {
			case "ide":
				request.Client.Surfaces = []domain.ClientSurface{{ID: "copilot_vscode_extension", Detected: true}}
			case "undetected":
				request.Client.Status = domain.DetectionNotDetected
			case "relative-executable":
				request.Client.ExecutablePath = "copilot"
			case "missing-parser":
				env.ParseSkill = nil
			case "missing-snapshots":
				env.Snapshots = nil
			case "missing-runner":
				env.Runner = nil
			case "missing-executable":
				request.Client.ExecutablePath = ""
			case "backend":
				request.BackendExecutable = filepath.Join(t.TempDir(), "foreign")
			case "mixed":
				request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportNative})
			case "empty":
				request.Plan.Components = nil
			case "duplicate":
				request.Plan.Components = append(request.Plan.Components, request.Plan.Components[0])
			case "policy":
				request.Plan.Policy = domain.PolicyBlocked
			case "receipt":
				request.Delivery.NativeObjects[0].ManagedDigest = "foreign"
			case "unknown-source":
				request.Plan.Components[0].Source = "unknown"
			case "source-name":
				request.Plan.Components[0].SourceName = "unbound"
			case "unsupported":
				request.Plan.Components[0].Support = domain.SupportPrepared
			}
			got, err := observeCopilotProjectSkills(t.Context(), env, request, out)
			if err != nil || got.Activation != domain.ActivationPrepared || runner.command.Argv != nil {
				t.Fatalf("unqualified request reached catalog: %+v %v %+v", got, err, runner.command)
			}
		})
	}
}

func copilotPreparedSkillOutcome(request domain.ActivationRequest) domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
		Policy: domain.PolicyAllowed, Authentication: request.Plan.Authentication, NativeEffect: domain.NativeEffectUnchanged,
		NativeObjects: slices.Clone(request.Delivery.NativeObjects), UserActions: []string{copilotProjectInstalledAction, "retain unrelated action"}}
}

func TestCopilotProjectSkillsCatalogRejectsAmbiguityAndChangedContent(t *testing.T) {
	request, env, runner := copilotProjectSkillFixture(t)
	proof := captureCopilotProjectSkills(t.Context(), env, request)
	good := string(runner.result.Stdout)
	if proof == nil || !copilotProjectSkillsListed([]byte(good), proof.skills) {
		t.Fatal("measured catalog schema refused")
	}
	for _, bad := range []string{"null", "[]", "{}", "help", "[null]", good + "{}",
		strings.Replace(good, `"name":"owned"`, `"name":"owned","Name":"owned"`, 1),
		strings.Replace(good, `"source":"project"`, `"source":"personal-copilot"`, 1),
		strings.Replace(good, `"enabled":true`, `"enabled":false`, 1),
		strings.Replace(good, `"description":"Inert Skill"`, `"description":"foreign"`, 1),
		strings.Replace(good, `"description":"Inert Skill"`, `"description":null`, 1),
		strings.Replace(good, `"path":`, `"extra":true,"path":`, 1),
		strings.Replace(good, `"path":`, `"path":null,"extra":`, 1),
		good[:len(good)-1] + "," + good[1:]} {
		if copilotProjectSkillsListed([]byte(bad), proof.skills) {
			t.Fatalf("ambiguous or malformed catalog accepted: %s", bad)
		}
	}
	rows := make([]map[string]any, 0, 1)
	if err := json.Unmarshal(runner.result.Stdout, &rows); err != nil {
		t.Fatal(err)
	}
	ownedPath := rows[0]["path"]
	rows[0]["path"] = filepath.Join(t.TempDir(), "SKILL.md")
	body, _ := json.Marshal(rows)
	if copilotProjectSkillsListed(body, proof.skills) {
		t.Fatal("foreign path supplied selected Skill proof")
	}
	rows[0]["path"] = ownedPath.(string) + string(filepath.Separator) + "SKILL.md"
	body, _ = json.Marshal(rows)
	if copilotProjectSkillsListed(body, proof.skills) {
		t.Fatal("session file path substituted for ordinary catalog directory")
	}
	rows[0]["path"] = ownedPath
	foreign := map[string]any{"name": "foreign", "description": "Foreign", "source": "personal-copilot", "enabled": false, "path": filepath.Join(t.TempDir(), "foreign")}
	rows = append(rows, foreign)
	body, _ = json.Marshal(rows)
	if !copilotProjectSkillsListed(body, proof.skills) {
		t.Fatal("unrelated foreign Skill must coexist without supplying owned proof")
	}
	foreign["name"] = "owned"
	foreign["description"] = rows[0]["description"]
	body, _ = json.Marshal(rows)
	if copilotProjectSkillsListed(body, proof.skills) {
		t.Fatal("same-name foreign Skill made selected discovery ambiguous")
	}
}

func TestCopilotProjectSkillsRecheckSourceResourcesReceiptsAndPolicy(t *testing.T) {
	for _, mode := range []string{"installed-body", "source-body", "source-resource", "resource", "receipt", "component", "policy-warning", "trust"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotProjectSkillFixture(t)
			out := copilotPreparedSkillOutcome(request)
			runner.onRun = func() {
				path := filepath.Join(request.Delivery.NativeObjects[0].Path, "SKILL.md")
				switch mode {
				case "source-body":
					path = filepath.Join(request.Delivery.ActivePath, "skills", "owned", "SKILL.md")
				case "source-resource":
					path = filepath.Join(request.Delivery.ActivePath, "unselected-resource.txt")
				case "resource":
					path = filepath.Join(request.Delivery.NativeObjects[0].Path, "resource.txt")
				case "trust":
					body, err := json.Marshal(map[string]any{"trustedFolders": []string{request.Plan.TargetContext.ProjectRoot}, "foreign": "changed"})
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(request.Client.ConfigRoot, "config.json"), body, 0o600); err != nil {
						t.Fatal(err)
					}
					return
				case "receipt":
					request.Delivery.NativeObjects[0].ManagedDigest = "changed"
					return
				case "component":
					request.Plan.Components[0].Source = "unknown"
					return
				case "policy-warning":
					request.Plan.Warnings[0] = "authentication_requirement_unknown"
					return
				}
				if err := os.WriteFile(path, []byte("changed during query"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := observeCopilotProjectSkills(t.Context(), env, request, out)
			if mode == "component" || mode == "policy-warning" {
				if err != nil || got.Activation != domain.ActivationPrepared {
					t.Fatalf("late metadata/policy change qualified: %+v %v", got, err)
				}
			} else if err == nil || got.Activation != domain.ActivationFailed || got.Verification != domain.VerificationFailed || got.NativeEffect != domain.NativeEffectUncertain {
				t.Fatalf("changed owned bytes/receipts were not an explicit failure: %+v %v", got, err)
			}
			if got.Authentication != domain.AuthenticationNotChecked {
				t.Fatalf("late loss of owned authority qualified: %+v %v", got, err)
			}
		})
	}
}

func TestCopilotProjectSkillsFailedCatalogStillChecksOwnedBytes(t *testing.T) {
	for _, mode := range []string{"exit", "error", "stderr", "malformed", "missing"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotProjectSkillFixture(t)
			cause := errors.New("closed native refusal")
			switch mode {
			case "exit":
				runner.result.ExitCode = 1
			case "error":
				runner.err = cause
			case "stderr":
				runner.result.Stderr = []byte("unexpected")
			case "malformed":
				runner.result.Stdout = []byte("malformed")
			case "missing":
				runner.result.Stdout = []byte("[]")
			}
			runner.onRun = func() {
				path := filepath.Join(request.Delivery.NativeObjects[0].Path, "changed-resource.txt")
				if err := os.WriteFile(path, []byte("changed during failed catalog"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := observeCopilotProjectSkills(t.Context(), env, request, copilotPreparedSkillOutcome(request))
			if err == nil || got.Activation != domain.ActivationFailed || got.Verification != domain.VerificationFailed ||
				got.NativeEffect != domain.NativeEffectUncertain || got.Authentication != domain.AuthenticationNotChecked {
				t.Fatalf("negative catalog hid changed owned bytes: %+v %v", got, err)
			}
			if mode == "error" && !errors.Is(err, cause) {
				t.Fatalf("original query failure was lost: %v", err)
			}
		})
	}
}

func TestCopilotProjectSkillsRequireClosedSuccessfulQuery(t *testing.T) {
	for _, mode := range []string{"exit", "error", "no-child", "unclosed", "missing-closure", "oversize", "stderr", "pre-cancel", "late-cancel"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotProjectSkillFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch mode {
			case "exit":
				runner.result.ExitCode = 1
			case "error":
				runner.err = errors.New("closed native refusal")
			case "no-child":
				runner.result.NoChildStarted = true
			case "unclosed":
				runner.err = &domain.UnclosedClientProbe{Operation: "test", Err: errors.New("unclosed")}
			case "missing-closure":
				runner.result.CleanupComplete = false
			case "oversize":
				runner.result.Stdout = make([]byte, (1<<20)+1)
			case "stderr":
				runner.result.Stderr = []byte("unexpected")
			case "pre-cancel":
				cancel()
			case "late-cancel":
				runner.cancel = cancel
			}
			got, err := observeCopilotProjectSkills(ctx, env, request, copilotPreparedSkillOutcome(request))
			if got.Activation == domain.ActivationActive || got.Authentication != domain.AuthenticationNotChecked {
				t.Fatalf("failed catalog completed installation: %+v %v", got, err)
			}
			if (mode == "unclosed" || mode == "missing-closure") && (!domain.IsUnclosedClientProbe(err) || got.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatalf("process closure uncertainty was hidden: %+v %v", got, err)
			}
			if (mode == "pre-cancel" || mode == "late-cancel") && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation was hidden: %v", err)
			}
			if mode == "pre-cancel" && runner.command.Argv != nil {
				t.Fatal("pre-canceled observation reached runner")
			}
		})
	}
}

type copilotSkillTreeRunner struct {
	copilotSkillRunner
	grace time.Duration
}

func (runner *copilotSkillTreeRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	runner.grace = grace
	return runner.Run(ctx, command)
}

func TestCopilotProjectSkillsUseTreeClosureAndPreserveAuthentication(t *testing.T) {
	for _, auth := range []domain.AuthenticationState{domain.AuthenticationNotChecked, domain.AuthenticationPending} {
		request, env, runner := copilotProjectSkillFixture(t)
		request.Plan.Authentication = auth
		if auth == domain.AuthenticationNotChecked {
			request.Plan.Warnings = []string{"authentication_requirement_unknown"}
		}
		tree := &copilotSkillTreeRunner{copilotSkillRunner: *runner}
		env.Runner = tree
		got, err := observeCopilotProjectSkills(t.Context(), env, request, copilotPreparedSkillOutcome(request))
		if err != nil || got.Activation != domain.ActivationActive || got.Authentication != auth || tree.grace != time.Second ||
			!slices.Equal(got.UserActions, []string{"retain unrelated action"}) {
			t.Fatalf("explicit authentication or unrelated action changed: %+v %v grace=%v", got, err, tree.grace)
		}
	}
}

// An inert runner retains the ordinary catalog command; no native process is
// launched by these adapter contracts.
type copilotSkillRunner struct {
	command legacyports.Command
	result  legacyports.CommandResult
	err     error
	bounded bool
	cancel  context.CancelFunc
	onRun   func()
}

func (runner *copilotSkillRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.command = command
	_, runner.bounded = ctx.Deadline()
	if runner.onRun != nil {
		runner.onRun()
	}
	if runner.cancel != nil {
		runner.cancel()
	}
	return runner.result, runner.err
}

func TestCopilotProjectSkillsRemovalPreservesForeignAndDoesNotRunCatalog(t *testing.T) {
	request, env, runner := copilotProjectSkillFixture(t)
	foreign := filepath.Join(filepath.Dir(request.Delivery.NativeObjects[0].Path), "foreign")
	if err := os.MkdirAll(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	body := []byte("foreign retained")
	if err := os.WriteFile(filepath.Join(foreign, "SKILL.md"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	trust := trustRead(t, filepath.Join(request.Client.ConfigRoot, "config.json"))
	out, err := New().Deactivate(t.Context(), env, domain.DeactivationRequest{Client: request.Client,
		TargetContext: request.Plan.TargetContext, NativeRegistryRoot: request.Plan.NativeRegistryRoot,
		NativeObjects: request.Delivery.NativeObjects, Confirmed: true})
	if err != nil || !out.ExternalRemovalComplete || runner.command.Argv != nil {
		t.Fatalf("owned removal failed or ran catalog: %+v %v", out, err)
	}
	if _, err := os.Lstat(request.Delivery.NativeObjects[0].Path); !os.IsNotExist(err) {
		t.Fatal("owned Skill remains", err)
	}
	if !slices.Equal(trustRead(t, filepath.Join(foreign, "SKILL.md")), body) || !slices.Equal(trustRead(t, filepath.Join(request.Client.ConfigRoot, "config.json")), trust) {
		t.Fatal("removal changed foreign Skill or retained trust")
	}
}

func TestCopilotProjectSkillsRequireDeliveredPackageDigest(t *testing.T) {
	for _, phase := range []string{"before", "during"} {
		t.Run(phase, func(t *testing.T) {
			request, env, runner := copilotProjectSkillFixture(t)
			mutate := func() {
				if err := os.WriteFile(filepath.Join(request.Plan.ActivePath, "unselected.txt"), []byte("changed resource"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "before" {
				mutate()
			} else {
				runner.onRun = mutate
			}
			got, err := observeCopilotProjectSkills(t.Context(), env, request, copilotPreparedSkillOutcome(request))
			if got.Activation == domain.ActivationActive {
				t.Fatal("modified delivered package promoted")
			}
			if phase == "before" && (err != nil || runner.command.Argv != nil) {
				t.Fatal(got, err)
			}
			if phase == "during" && (err == nil || got.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatal(got, err)
			}
		})
	}
}
