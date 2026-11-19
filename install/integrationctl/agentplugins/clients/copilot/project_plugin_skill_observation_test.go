package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type copilotWholeSkillRunner struct {
	plugins, skills []byte
	commands        []legacyports.Command
	mutate          func()
	result          legacyports.CommandResult
	err             error
	expire          bool
}

func (r *copilotWholeSkillRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	panic("tree-aware native seam required")
}
func (r *copilotWholeSkillRunner) RunWithTreeExitGrace(ctx context.Context, c legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	if grace != time.Second {
		panic("changed native registry closure grace")
	}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 30*time.Second {
		panic("unbounded ordinary catalogs")
	}
	r.commands = append(r.commands, c)
	if r.expire {
		<-ctx.Done()
		if r.mutate != nil {
			r.mutate()
		}
		return r.result, ctx.Err()
	}
	if r.mutate != nil {
		r.mutate()
	}
	result := r.result
	result.Stdout = r.plugins
	if c.Argv[1] == "skill" {
		result.Stdout = r.skills
	}
	return result, r.err
}

func copilotWholeSkillFixture(t *testing.T) (domain.ActivationRequest, clients.Env, *copilotWholeSkillRunner) {
	t.Helper()
	request, env, _ := copilotProjectSkillFixture(t)
	active := request.Plan.ActivePath
	body := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"package","version":"1.0.0"}`)
	if err := os.WriteFile(filepath.Join(active, "plugin.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "package", Version: "1.0.0"}}
	if err := shared.ProjectCopilotMarketplace(active, envelope, request.Plan); err != nil {
		t.Fatal(err)
	}
	object, err := copilotProjectPluginObject(request.Plan)
	if err != nil {
		t.Fatal(err)
	}
	reg, path, err := copilotProjectRegistration(request.Plan.NativeRegistryRoot, request.Plan.PhysicalArtifactID, "package")
	if err != nil {
		t.Fatal(err)
	}
	settings, _, err := nativeconfig.PatchCopilotPluginSettings([]byte("{\"foreign\":42}"), reg, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, settings, 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := (packagesnapshot.Builder{}).Digest(t.Context(), active)
	if err != nil {
		t.Fatal(err)
	}
	request.Plan.SetNativePluginRegistration(true)
	request.Delivery.ArtifactDigest = digest
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{{ObjectID: "package:copilot:package", Kind: "managed_package_directory", LogicalName: "package", Path: active, ManagedDigest: digest, ProtectionClass: "managed"}, object}
	request.PreviousNativeObjects = slices.Clone(request.Delivery.NativeObjects)
	plugins, _ := json.Marshal([]any{map[string]any{"name": "package", "marketplace": reg.Marketplace, "version": "1.0.0", "enabled": true, "source": "live", "installedFrom": active}, map[string]any{"name": "browser-use", "marketplace": "", "version": "0.0.4", "enabled": false, "source": "builtin"}})
	skills, _ := json.Marshal([]any{map[string]any{"name": "owned", "description": "Inert Skill", "enabled": true, "source": "plugin", "path": filepath.Join(active, "skills", "owned")}})
	runner := &copilotWholeSkillRunner{plugins: plugins, skills: skills, result: legacyports.CommandResult{CleanupComplete: true}}
	env.Runner = runner
	return request, env, runner
}

func TestCopilotWholePureSkillsRequireBothOrdinaryOwnedCatalogs(t *testing.T) {
	request, env, runner := copilotWholeSkillFixture(t)
	out := copilotPreparedSkillOutcome(request)
	out.UserActions = []string{copilotWholePluginInstalledAction, "unrelated action"}
	got, err := observeCopilotWholePluginSkills(t.Context(), env, request, out)
	if err != nil || got.Activation != domain.ActivationActive || got.Authentication != domain.AuthenticationNotRequired || !slices.Equal(got.UserActions, []string{"unrelated action"}) || len(runner.commands) != 2 {
		t.Fatalf("whole native proof did not complete: %+v %v", got, err)
	}
	for i, c := range runner.commands {
		kind := []string{"plugin", "skill"}[i]
		if !slices.Equal(c.Argv, []string{request.Client.ExecutablePath, kind, "list", "--json"}) || c.Dir != request.Plan.TargetContext.ProjectRoot || !slices.Contains(c.Env, "COPILOT_HOME="+request.Client.ConfigRoot) {
			t.Fatalf("ordinary command authority changed: %+v", c)
		}
	}
	if runner.commands[1].StdoutLimitBytes != (1<<20)-len(runner.plugins) {
		t.Fatal("catalog output budgets were multiplied")
	}
	request.Replacing, request.PackageUnchanged = true, true
	runner.commands = nil
	repeat, err := observeCopilotWholePluginSkills(t.Context(), env, request, out)
	if err != nil || repeat.Activation != domain.ActivationActive || len(runner.commands) != 2 {
		t.Fatal("unchanged replacement omitted fresh native catalogs", err)
	}
}

func TestCopilotWholeSkillsRejectUnqualifiedAuthorityBeforeQuery(t *testing.T) {
	for _, mode := range []string{"parser", "snapshots", "runner", "mixed", "projected", "unowned-package", "native-name", "trust", "attested"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotWholeSkillFixture(t)
			out := copilotPreparedSkillOutcome(request)
			switch mode {
			case "parser":
				env.ParseSkill = nil
			case "snapshots":
				env.Snapshots = nil
			case "runner":
				env.Runner = nil
			case "mixed":
				request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "mcp", Support: domain.SupportNative})
			case "projected":
				request.Plan.Components[0].Support = domain.SupportProjected
			case "unowned-package":
				request.Delivery.NativeObjects[0].ManagedDigest = "foreign"
				out.NativeObjects = slices.Clone(request.Delivery.NativeObjects)
			case "native-name":
				request.DeclaredName = "foreign"
			case "trust":
				if err := os.WriteFile(filepath.Join(request.Client.ConfigRoot, "config.json"), []byte(`{"trustedFolders":[]}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "attested":
				out.ActivationAttested = true
			}
			got, err := observeCopilotWholePluginSkills(t.Context(), env, request, out)
			if err != nil || got.Activation != domain.ActivationPrepared || len(runner.commands) != 0 {
				t.Fatalf("unqualified authority queried: %+v %v", got, err)
			}
		})
	}
}

func TestCopilotWholeSkillsRefuseCatalogAmbiguityAndScopeSubstitution(t *testing.T) {
	for _, mode := range []string{"null", "missing-plugin", "wrong-marketplace", "wrong-install-root", "wrong-version", "disabled", "duplicate-plugin", "unknown-field", "wrong-skill-source", "wrong-skill-path", "wrong-description", "stderr", "exit", "unclosed"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotWholeSkillFixture(t)
			switch mode {
			case "null":
				runner.plugins = []byte("null")
			case "missing-plugin":
				runner.plugins = []byte("[]")
			case "wrong-marketplace":
				runner.plugins = []byte(strings.ReplaceAll(string(runner.plugins), "agentplugins-", "foreign-"))
			case "wrong-install-root":
				runner.plugins = copilotMutateCatalogPath(t, runner.plugins, "installedFrom", filepath.Join(t.TempDir(), "foreign"))
			case "wrong-version":
				runner.plugins = []byte(strings.ReplaceAll(string(runner.plugins), "1.0.0", "9.0.0"))
			case "disabled":
				runner.plugins = []byte(strings.ReplaceAll(string(runner.plugins), `"enabled":true`, `"enabled":false`))
			case "duplicate-plugin":
				runner.plugins = append(runner.plugins[:len(runner.plugins)-1], append([]byte(","), runner.plugins[1:]...)...)
			case "unknown-field":
				runner.plugins = []byte(strings.ReplaceAll(string(runner.plugins), `"installedFrom":`, `"unknown":true,"installedFrom":`))
			case "wrong-skill-source":
				runner.skills = []byte(strings.ReplaceAll(string(runner.skills), `"plugin"`, `"project"`))
			case "wrong-skill-path":
				runner.skills = copilotMutateCatalogPath(t, runner.skills, "path", filepath.Join(t.TempDir(), "other"))
			case "wrong-description":
				runner.skills = []byte(strings.ReplaceAll(string(runner.skills), "Inert Skill", "Foreign Skill"))
			case "stderr":
				runner.result.Stderr = []byte("unexpected diagnostics")
			case "exit":
				runner.result.ExitCode = 1
			case "unclosed":
				runner.result.CleanupComplete = false
			}
			got, err := observeCopilotWholePluginSkills(t.Context(), env, request, copilotPreparedSkillOutcome(request))
			if got.Activation == domain.ActivationActive {
				t.Fatal("catalog mismatch promoted")
			}
			if mode == "unclosed" && (!domain.IsUnclosedClientProbe(err) || got.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatal("unclosed catalog lost uncertainty", err)
			}
		})
	}
}

type copilotWholeSnapshots struct {
	packagesnapshot.Builder
	capture  func(string)
	closeErr error
}

func (s copilotWholeSnapshots) CaptureDirectory(ctx context.Context, root string) (string, func() error, error) {
	sealed, closeSnapshot, err := s.Builder.CaptureDirectory(ctx, root)
	if err != nil {
		return sealed, closeSnapshot, err
	}
	if s.capture != nil {
		s.capture(root)
	}
	return sealed, func() error { return errors.Join(closeSnapshot(), s.closeErr) }, nil
}

func TestCopilotWholeSkillsReadSealedMetadataAcrossLiveABA(t *testing.T) {
	request, env, runner := copilotWholeSkillFixture(t)
	path := filepath.Join(request.Plan.ActivePath, "skills", "owned", "SKILL.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	env.Snapshots = copilotWholeSnapshots{capture: func(string) {
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(original), "Inert Skill", "Foreign Skill")), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	parsed := 0
	env.ParseSkill = func(ctx context.Context, name string, body []byte) (domain.Skill, error) {
		parsed++
		if string(body) != string(original) {
			t.Fatal("live metadata substituted for sealed source")
		}
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
		return conformance.ParseSkill(ctx, name, body)
	}
	got, err := observeCopilotWholePluginSkills(t.Context(), env, request, copilotPreparedSkillOutcome(request))
	if err != nil || got.Activation != domain.ActivationActive || parsed != 2 || len(runner.commands) != 2 {
		t.Fatalf("sealed metadata proof failed: %+v %v", got, err)
	}
}

func TestCopilotWholeSkillsFailClosedOnCleanupCustodyAndCancellation(t *testing.T) {
	for _, mode := range []string{"snapshot-close", "resource-change", "settings-change", "pre-cancel", "late-cancel"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotWholeSkillFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch mode {
			case "snapshot-close":
				env.Snapshots = copilotWholeSnapshots{closeErr: errors.New("snapshot cleanup")}
			case "resource-change":
				runner.mutate = func() {
					if err := os.WriteFile(filepath.Join(request.Plan.ActivePath, "extra.txt"), []byte("changed"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "settings-change":
				runner.mutate = func() {
					path, _ := copilotProjectPluginSettingsPath(request.Plan.NativeRegistryRoot)
					if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "pre-cancel":
				cancel()
			case "late-cancel":
				runner.mutate = cancel
			}
			got, err := observeCopilotWholePluginSkills(ctx, env, request, copilotPreparedSkillOutcome(request))
			if got.Activation == domain.ActivationActive {
				t.Fatal("lost authority promoted")
			}
			if strings.Contains(mode, "cancel") && !errors.Is(err, context.Canceled) {
				t.Fatal("cancel lost", err)
			}
			if (mode == "resource-change" || mode == "settings-change") && (err == nil || got.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatal("late change uncertainty lost", err)
			}
			if mode == "snapshot-close" && len(runner.commands) != 0 {
				t.Fatal("failed snapshot cleanup reached native command")
			}
		})
	}
}

func TestCopilotWholeSkillsShareAggregateCaptureBudget(t *testing.T) {
	for _, mode := range []string{"first-exhausted", "second-overflow"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotWholeSkillFixture(t)
			if mode == "first-exhausted" {
				runner.plugins = append(runner.plugins, []byte(strings.Repeat(" ", (1<<20)-len(runner.plugins)))...)
			} else {
				runner.skills = append(runner.skills, []byte(strings.Repeat(" ", 1<<20))...)
			}
			got, err := observeCopilotWholePluginSkills(t.Context(), env, request, copilotPreparedSkillOutcome(request))
			if err != nil || got.Activation != domain.ActivationPrepared {
				t.Fatal("capture budget excess promoted or changed stable native effect", err)
			}
			if mode == "first-exhausted" && len(runner.commands) != 1 {
				t.Fatal("exhausted capture started another process")
			}
		})
	}
}

func copilotMutateCatalogPath(t *testing.T, body []byte, key, path string) []byte {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatal(err)
	}
	rows[0][key] = path
	out, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCopilotWholeSkillsClosedTimeoutStillRechecksCustody(t *testing.T) {
	for _, mode := range []string{"unchanged", "source", "trust", "unclosed", "parent-canceled"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := copilotWholeSkillFixture(t)
			out := copilotPreparedSkillOutcome(request)
			proof := captureCopilotWholeSkills(t.Context(), env, request)
			if proof == nil {
				t.Fatal("missing initial proof")
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			queryErr := error(context.DeadlineExceeded)
			switch mode {
			case "source":
				if err := os.WriteFile(filepath.Join(request.Plan.ActivePath, "late.txt"), []byte("late"), 0600); err != nil {
					t.Fatal(err)
				}
			case "trust":
				if err := os.WriteFile(filepath.Join(request.Client.ConfigRoot, "config.json"), []byte(`{"trustedFolders":[]}`), 0600); err != nil {
					t.Fatal(err)
				}
			case "unclosed":
				queryErr = &domain.UnclosedClientProbe{Operation: "test", Err: context.DeadlineExceeded}
			case "parent-canceled":
				cancel()
			}
			got, err := finishCopilotWholeSkills(ctx, env, request, out, proof, copilotWholeSkillCatalog{}, queryErr)
			if got.Activation == domain.ActivationActive || len(runner.commands) != 0 {
				t.Fatal("timeout promoted or launched a child")
			}
			if mode == "unchanged" && (err != nil || got.NativeEffect != out.NativeEffect || got.Activation != domain.ActivationPrepared) {
				t.Fatal(got, err)
			}
			if (mode == "source" || mode == "trust" || mode == "unclosed") && (err == nil || got.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatal(got, err)
			}
			if mode == "parent-canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal(got, err)
			}
		})
	}
}

func TestCopilotWholePluginCatalogRejectsForeignNULPath(t *testing.T) {
	request, env, runner := copilotWholeSkillFixture(t)
	proof := captureCopilotWholeSkills(t.Context(), env, request)
	var rows []map[string]any
	if err := json.Unmarshal(runner.plugins, &rows); err != nil {
		t.Fatal(err)
	}
	rows[1]["installedFrom"] = filepath.Join(t.TempDir(), "foreign")
	good, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if !copilotWholePluginListed(good, proof) {
		t.Fatal("valid foreign row refused")
	}
	rows[1]["installedFrom"] = rows[1]["installedFrom"].(string) + "\x00plugin"
	bad, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if copilotWholePluginListed(bad, proof) {
		t.Fatal("foreign NUL path accepted")
	}
}

// The catalog's real fixed deadline expires while the parent stays alive. A
// direct finisher test cannot detect accidentally passing that expired context
// to the post-query custody capture after registration has already committed.
func TestCopilotWholeSkillsInternalTimeoutKeepsVerifiedPrepared(t *testing.T) {
	request, env, runner := copilotWholeSkillFixture(t)
	runner.expire = true
	after := false
	reads := 0
	env.NativeConfig = nativeconfig.NewWithFileIO(copilotTimeoutCustodyFiles{after: &after, reads: &reads})
	runner.mutate = func() { after = true }
	out := copilotPreparedSkillOutcome(request)
	out.NativeEffect = domain.NativeEffectCommitted
	got, err := observeCopilotWholePluginSkills(t.Context(), env, request, out)
	if t.Context().Err() != nil {
		t.Fatal("parent expired instead of internal catalog budget")
	}
	if err != nil || got.Activation != domain.ActivationPrepared || got.Verification != out.Verification || got.NativeEffect != domain.NativeEffectCommitted || len(runner.commands) != 1 || reads == 0 {
		t.Fatalf("closed catalog timeout skipped intact custody or promoted: %+v %v", got, err)
	}
}

// Only post-catalog bounded profile reads count, so an early timeout return
// cannot satisfy the caller regression with its initial capture.
type copilotTimeoutCustodyFiles struct {
	nativeconfig.FileIO
	after *bool
	reads *int
}

func (f copilotTimeoutCustodyFiles) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	if *f.after && strings.HasSuffix(path, "config.json") {
		*f.reads++
	}
	v, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	return v.Body, v.Mode, v.Exists, err
}
