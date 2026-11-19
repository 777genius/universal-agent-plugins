package cline

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

func clineProjectSkillFixture(t *testing.T) (domain.ActivationRequest, clients.Env, *sessionObservationRunner) {
	t.Helper()
	project, home := t.TempDir(), t.TempDir()
	managed := filepath.Join(project, ".uap", "managed")
	target := filepath.Join(managed, "clients", "cline")
	active := filepath.Join(target, "package")
	layout := clineProjectComponents(project)
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
	plan := domain.DeliveryPlan{ClientID: domain.ClientCline, Scope: domain.ScopeProject, Status: domain.PlanReady,
		PackageMode: domain.PackageNative, InstallIntent: domain.InstallIntentAutomatic, Policy: domain.PolicyAllowed,
		Authentication: domain.AuthenticationNotChecked, Warnings: []string{"authentication_not_catalog_verified"},
		TargetContext:      domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: filepath.Join(home, ".cline")},
		NativeRegistryRoot: project, TargetAnchor: managed, TargetRoot: target, ActivePath: active,
		PhysicalArtifactID: "package", DeclaredName: "package",
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "owned", Support: domain.SupportNative}}}
	request := domain.ActivationRequest{Plan: plan, DeclaredName: "package", VerifyOnly: true,
		Client: domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected,
			ConfigRoot: plan.TargetContext.UserConfigRoot, Surfaces: []domain.ClientSurface{{ID: "cline_cli", Detected: true}}},
		Delivery:              domain.StagedDelivery{ClientID: domain.ClientCline, OwnedBase: target, ActivePath: active, NativeObjects: []domain.NativeObjectOwnership{object}},
		PreviousNativeObjects: []domain.NativeObjectOwnership{object}}
	row := map[string]any{"name": "owned", "description": "Inert Skill", "instructions": "Inert instructions.",
		"frontmatter": map[string]any{"name": "owned", "description": "Inert Skill", "metadata": map[string]any{"origin": "source"}},
		"path":        filepath.Join(object.Path, "SKILL.md")}
	catalog, err := json.Marshal([]any{row})
	if err != nil {
		t.Fatal(err)
	}
	runner := &sessionObservationRunner{result: legacyports.CommandResult{Stdout: catalog, CleanupComplete: true}}
	env := clients.Env{Runner: runner, NativeConfig: nativeconfig.New(), ParseSkill: conformance.ParseSkill, Snapshots: packagesnapshot.Builder{}, DecodeSkillFrontmatter: clineFixtureFrontmatter, LookupExecutable: func(string) (string, error) {
		return filepath.Join(home, "bin", "cline"), nil
	}}
	return request, env, runner
}

func TestClineProjectSkillsCompleteOnlyFreshOwnedCatalog(t *testing.T) {
	request, env, runner := clineProjectSkillFixture(t)
	t.Setenv("CLINE_MCP_SETTINGS_PATH", "/foreign/mcp.json")
	t.Setenv("CLINE_API_KEY", "not-a-real-key")
	out, err := New().Activate(t.Context(), env, request)
	if err != nil || out.Activation != domain.ActivationActive || out.Verification != domain.VerificationInstalled ||
		out.Authentication != domain.AuthenticationNotRequired || out.ActivationAttested || out.AuthenticationAttested || len(out.UserActions) != 0 {
		t.Fatalf("fresh catalog did not complete owned Skills: %+v %v", out, err)
	}
	want := []string{filepath.Join(filepath.Dir(request.Client.ConfigRoot), "bin", "cline"), "--auto-approve", "false", "--cwd", request.Plan.TargetContext.ProjectRoot, "config", "skills", "--json"}
	if !reflect.DeepEqual(runner.command.Argv, want) || runner.command.Dir != request.Plan.TargetContext.ProjectRoot ||
		!runner.bounded || runner.command.StdoutLimitBytes != 1<<20 {
		t.Fatalf("catalog command escaped finite project scope: %+v", runner.command)
	}
	for _, item := range runner.command.Env {
		if strings.HasPrefix(item, "CLINE_MCP_SETTINGS_PATH=") || strings.HasPrefix(item, "CLINE_API_KEY=") {
			t.Fatalf("catalog inherited provider or MCP override: %q", item)
		}
	}
	if !slices.Contains(runner.command.Env, "CLINE_NO_AUTO_UPDATE=1") || request.Client.ExecutablePath != "" {
		t.Fatal("catalog changed file-only detection or omitted updater suppression")
	}
}

func TestClineProjectSkillsInstallAndRenderedCommandUseSameOwnedProof(t *testing.T) {
	for _, mode := range []string{"install", "rendered-command"} {
		t.Run(mode, func(t *testing.T) {
			request, env, _ := clineProjectSkillFixture(t)
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

func TestClineProjectSkillsUnchangedRepeatRequiresAnotherCatalog(t *testing.T) {
	request, env, runner := clineProjectSkillFixture(t)
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

func TestClineProjectSkillsRequireOrdinaryAuthority(t *testing.T) {
	for _, mode := range []string{"ide", "missing-parser", "missing-snapshots", "missing-decoder", "missing-runner", "missing-resolver", "backend", "mixed", "empty", "duplicate", "policy", "receipt", "unknown-source", "source-name", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := clineProjectSkillFixture(t)
			out := clinePreparedSkillOutcome(request)
			switch mode {
			case "ide":
				request.Client.Surfaces = []domain.ClientSurface{{ID: "cline_vscode_extension", Detected: true}}
			case "missing-parser":
				env.ParseSkill = nil
			case "missing-snapshots":
				env.Snapshots = nil
			case "missing-decoder":
				env.DecodeSkillFrontmatter = nil
			case "missing-runner":
				env.Runner = nil
			case "missing-resolver":
				env.LookupExecutable = nil
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
			got, err := observeClineProjectSkills(t.Context(), env, request, out)
			if err != nil || got.Activation != domain.ActivationPrepared || runner.command.Argv != nil {
				t.Fatalf("unqualified request reached catalog: %+v %v %+v", got, err, runner.command)
			}
		})
	}
}

func clinePreparedSkillOutcome(request domain.ActivationRequest) domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
		Policy: domain.PolicyAllowed, Authentication: request.Plan.Authentication, NativeEffect: domain.NativeEffectUnchanged,
		NativeObjects: slices.Clone(request.Delivery.NativeObjects), UserActions: []string{clineProjectAction(request.Plan), "retain unrelated action"}}
}

func TestClineProjectSkillsCatalogRejectsAmbiguityAndChangedContent(t *testing.T) {
	request, env, runner := clineProjectSkillFixture(t)
	proof := captureClineProjectSkills(t.Context(), env, request)
	good := string(runner.result.Stdout)
	if proof == nil || !clineProjectSkillsListed([]byte(good), proof.skills) {
		t.Fatal("measured catalog schema refused")
	}
	for _, bad := range []string{"null", "[]", "{}", "help", "[null]", good + "{}",
		strings.Replace(good, `"name":"owned"`, `"name":"owned","Name":"owned"`, 1),
		strings.Replace(good, `"instructions":"Inert instructions."`, `"instructions":"foreign"`, 1),
		strings.Replace(good, `"origin":"source"`, `"origin":"foreign"`, 1),
		strings.Replace(good, `"description":"Inert Skill"`, `"description":null`, 1),
		strings.Replace(good, `"path":`, `"enabled":true,"path":`, 1),
		strings.Replace(good, `"path":`, `"path":null,"extra":`, 1),
		good[:len(good)-1] + "," + good[1:]} {
		if clineProjectSkillsListed([]byte(bad), proof.skills) {
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
	if clineProjectSkillsListed(body, proof.skills) {
		t.Fatal("foreign path supplied selected Skill proof")
	}
	rows[0]["path"] = ownedPath
	foreign := map[string]any{"name": "foreign", "description": "Foreign", "instructions": "Inert.",
		"frontmatter": map[string]any{"name": "foreign", "description": "Foreign"}, "path": filepath.Join(t.TempDir(), "SKILL.md")}
	rows = append(rows, foreign)
	body, _ = json.Marshal(rows)
	if !clineProjectSkillsListed(body, proof.skills) {
		t.Fatal("unrelated foreign Skill must coexist without supplying owned proof")
	}
	foreign["name"] = "owned"
	foreign["frontmatter"] = rows[0]["frontmatter"]
	foreign["description"] = rows[0]["description"]
	body, _ = json.Marshal(rows)
	if clineProjectSkillsListed(body, proof.skills) {
		t.Fatal("same-name foreign Skill made selected discovery ambiguous")
	}
}

func TestClineProjectSkillsRecheckSourceResourcesReceiptsAndPolicy(t *testing.T) {
	for _, mode := range []string{"installed-body", "source-body", "resource", "receipt", "component", "policy-warning"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := clineProjectSkillFixture(t)
			out := clinePreparedSkillOutcome(request)
			runner.onRun = func() {
				path := filepath.Join(request.Delivery.NativeObjects[0].Path, "SKILL.md")
				switch mode {
				case "source-body":
					path = filepath.Join(request.Delivery.ActivePath, "skills", "owned", "SKILL.md")
				case "resource":
					path = filepath.Join(request.Delivery.NativeObjects[0].Path, "resource.txt")
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
			got, err := observeClineProjectSkills(t.Context(), env, request, out)
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

func TestClineProjectSkillsFailedCatalogStillChecksOwnedBytes(t *testing.T) {
	for _, mode := range []string{"exit", "error", "stderr", "malformed", "missing"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := clineProjectSkillFixture(t)
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
			got, err := observeClineProjectSkills(t.Context(), env, request, clinePreparedSkillOutcome(request))
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

func TestClineProjectSkillsRequireClosedSuccessfulQuery(t *testing.T) {
	for _, mode := range []string{"exit", "error", "no-child", "unclosed", "missing-closure", "oversize", "stderr", "pre-cancel", "late-cancel"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := clineProjectSkillFixture(t)
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
			got, err := observeClineProjectSkills(ctx, env, request, clinePreparedSkillOutcome(request))
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

type clineSkillTreeRunner struct {
	sessionObservationRunner
	grace time.Duration
}

type clineSkillMetadataABAIO struct {
	nativeconfig.FileIO
	path string
	body []byte
	test *testing.T
}

func (files clineSkillMetadataABAIO) ReadNoFollowBounded(selected string, limit int64) ([]byte, os.FileMode, bool, error) {
	// Substitute only the live owned metadata during its read, then restore A
	// before the final owned digest. Reading the sealed component must still
	// return A, and cannot authorize a native catalog containing B.
	substitute := []byte(strings.ReplaceAll(string(files.body), "Inert instructions.", "Substituted instructions."))
	if err := os.WriteFile(files.path, substitute, 0o600); err != nil {
		files.test.Fatal(err)
	}
	file, err := nativeconfig.New().ReadExactFileBounded(selected, limit)
	if restore := os.WriteFile(files.path, files.body, 0o600); restore != nil {
		files.test.Fatal(restore)
	}
	return file.Body, file.Mode, file.Exists, err
}

func TestClineProjectSkillsLiveMetadataABACannotQualifySubstituteCatalog(t *testing.T) {
	request, env, runner := clineProjectSkillFixture(t)
	path := filepath.Join(request.Delivery.NativeObjects[0].Path, "SKILL.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	env.NativeConfig = nativeconfig.NewWithFileIO(clineSkillMetadataABAIO{path: path, body: body, test: t})
	runner.result.Stdout = []byte(strings.ReplaceAll(string(runner.result.Stdout), "Inert instructions.", "Substituted instructions."))
	got, err := observeClineProjectSkills(t.Context(), env, request, clinePreparedSkillOutcome(request))
	if err != nil || got.Activation != domain.ActivationPrepared || got.Authentication != domain.AuthenticationNotChecked || runner.command.Argv == nil {
		t.Fatalf("transient live metadata authorized substituted native catalog: %+v %v", got, err)
	}
}

func (runner *clineSkillTreeRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	runner.grace = grace
	return runner.Run(ctx, command)
}

func TestClineProjectSkillsUseTreeClosureAndPreserveAuthentication(t *testing.T) {
	for _, auth := range []domain.AuthenticationState{domain.AuthenticationNotChecked, domain.AuthenticationPending} {
		request, env, runner := clineProjectSkillFixture(t)
		request.Plan.Authentication = auth
		if auth == domain.AuthenticationNotChecked {
			request.Plan.Warnings = []string{"authentication_requirement_unknown"}
		}
		tree := &clineSkillTreeRunner{sessionObservationRunner: *runner}
		env.Runner = tree
		got, err := observeClineProjectSkills(t.Context(), env, request, clinePreparedSkillOutcome(request))
		if err != nil || got.Activation != domain.ActivationActive || got.Authentication != auth || tree.grace != time.Second ||
			!slices.Equal(got.UserActions, []string{"retain unrelated action"}) {
			t.Fatalf("explicit authentication or unrelated action changed: %+v %v grace=%v", got, err, tree.grace)
		}
	}
}

// Keep the controlled fixture on the same canonical, complete metadata decoder
// injected by the provider; client production code depends only on its port.
func clineFixtureFrontmatter(ctx context.Context, body []byte) (map[string]any, []byte, error) {
	front, err := conformance.DecodeFrontmatter(ctx, body, conformance.DefaultLimits(), nil)
	if err != nil {
		return nil, nil, err
	}
	fields := make(map[string]struct{}, len(front.Keys))
	for _, key := range front.Keys {
		fields[key] = struct{}{}
	}
	front, err = conformance.DecodeFrontmatter(ctx, body, conformance.DefaultLimits(), fields)
	if err != nil {
		return nil, nil, err
	}
	return front.Values, front.Body, nil
}
