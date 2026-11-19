package pi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type piSkillCatalogRunner struct {
	rows          []any
	command       legacyports.Command
	input         bytes.Buffer
	calls         int
	before, after func()
	mutate        func(map[string]any)
	cleanup       error
	bounded       bool
	expire        bool
}

func (*piSkillCatalogRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("ordinary command not expected")
}

func (runner *piSkillCatalogRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	runner.calls++
	runner.command = command
	if runner.expire {
		<-ctx.Done()
		if runner.after != nil {
			runner.after()
		}
		return ctx.Err()
	}
	runner.input.Reset()
	if deadline, ok := ctx.Deadline(); ok {
		runner.bounded = time.Until(deadline) > 0 && time.Until(deadline) <= 30*time.Second
	}
	if runner.before != nil {
		runner.before()
	}
	reader := &piSkillReply{runner: runner}
	err := exchange(&runner.input, reader)
	if runner.after != nil {
		runner.after()
	}
	return errors.Join(err, runner.cleanup)
}

type piSkillReply struct {
	runner *piSkillCatalogRunner
	body   *bytes.Reader
}

func (reply *piSkillReply) Read(target []byte) (int, error) {
	if reply.body == nil {
		var request map[string]any
		if err := json.Unmarshal(reply.runner.input.Bytes(), &request); err != nil {
			return 0, err
		}
		if request["type"] != "get_commands" || len(request) != 2 {
			return 0, errors.New("unexpected native request")
		}
		envelope := map[string]any{"type": "response", "command": "get_commands", "id": request["id"], "success": true, "data": map[string]any{"commands": reply.runner.rows}}
		if reply.runner.mutate != nil {
			reply.runner.mutate(envelope)
		}
		body, err := json.Marshal(envelope)
		if err != nil {
			return 0, err
		}
		reply.body = bytes.NewReader(append(body, '\n'))
	}
	return reply.body.Read(target)
}

func piSkillFixture(t *testing.T, scope domain.InstallScope) (domain.ActivationRequest, clients.Env, *piSkillCatalogRunner) {
	t.Helper()
	root := piTrustTemp(t)
	project := filepath.Join(root, "project")
	user := filepath.Join(root, "home", ".pi", "agent")
	piTrustWrite(t, filepath.Join(user, "foreign.txt"), "user marker")
	piTrustWrite(t, filepath.Join(project, "foreign.txt"), "project marker")
	request := piTrustRequest(t, user, project, "owned-package", []string{"owned"}, nil)
	if scope == domain.ScopeUser {
		target, err := domain.NewTargetContext(scope, "", user)
		if err != nil {
			t.Fatal(err)
		}
		request.Plan.Scope = scope
		request.Plan.TargetContext = target
		request.Plan.NativeRegistryRoot = user
		request.Plan.TargetAnchor = user
		request.Plan.TargetRoot = filepath.Join(user, "extensions")
		request.Plan.ActivePath = filepath.Join(request.Plan.TargetRoot, "owned-package")
	}
	request.Plan.Status = domain.PlanReady
	request.Plan.Policy = domain.PolicyAllowed
	request.Plan.DeclaredName = "owned-package"
	request.DeclaredName = "owned-package"
	request.Plan.Authentication = domain.AuthenticationNotChecked
	request.Plan.Warnings = []string{"authentication_not_catalog_verified"}
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "owned", Support: domain.SupportNative}}
	request.Client.Status = domain.DetectionDetected
	request.Client.ExecutablePath = filepath.Join(root, "pi")
	request.Client.Surfaces = []domain.ClientSurface{{ID: "pi_cli", Detected: true}}
	request.BackendExecutable = request.Client.ExecutablePath
	request.Delivery.ActivePath = request.Plan.ActivePath
	request.Delivery.OwnedBase = request.Plan.TargetRoot
	body := "---\nname: owned\ndescription: Original Skill\n---\nInert text only.\n"
	piTrustWrite(t, filepath.Join(request.Plan.ActivePath, "plugin.json"), "{\"name\":\"owned-package\",\"version\":\"1.0.0\"}")
	piTrustWrite(t, filepath.Join(request.Plan.ActivePath, "skills", "owned", "SKILL.md"), body)
	layout, err := piComponents(request.Plan.NativeRegistryRoot, scope)
	if err != nil {
		t.Fatal(err)
	}
	piTrustWrite(t, filepath.Join(layout.SkillsRoot, "owned", "SKILL.md"), body)
	object, err := layout.SkillObject("owned", "skills/owned", request.Plan.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := (packagesnapshot.Builder{}).Build(t.Context(), request.Plan.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	digest := artifact.Digest
	if err := artifact.Close(); err != nil {
		t.Fatal(err)
	}
	request.Delivery.ArtifactDigest = digest
	request.Delivery.NativeObjects = []domain.NativeObjectOwnership{
		{ObjectID: "package:pi:owned-package", Kind: "managed_package_directory", LogicalName: "owned-package", Path: request.Plan.ActivePath, ManagedDigest: digest, ProtectionClass: "managed"}, object}
	env := clients.Env{NativeConfig: nativeconfig.New(), ParseSkill: conformance.ParseSkill, Snapshots: packagesnapshot.Builder{}}
	if scope == domain.ScopeProject {
		if err := grantPiProjectTrust(env, request); err != nil {
			t.Fatal(err)
		}
		if blocker, err := piTrustBlocker(env.NativeConfig, user, project); err != nil || blocker != "" {
			t.Fatalf("invalid test trust: %q %v", blocker, err)
		}
	}
	request.VerifyOnly = true
	request.PreviousNativeObjects = slices.Clone(request.Delivery.NativeObjects)
	row := map[string]any{"name": "skill:owned", "description": "Original Skill", "source": "skill",
		"sourceInfo": map[string]any{"path": filepath.Join(object.Path, "SKILL.md"), "source": "auto", "scope": string(scope), "origin": "top-level", "baseDir": request.Plan.NativeRegistryRoot}}
	runner := &piSkillCatalogRunner{rows: []any{row}}
	env.Runner = runner
	return request, env, runner
}

func piSkillOutcome(request domain.ActivationRequest) domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled,
		Authentication: request.Plan.Authentication, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged,
		NativeObjects: slices.Clone(request.Delivery.NativeObjects), UserActions: []string{piCompletedSkillAction(request.Plan.Scope), "unrelated action"}}
}

func TestPiSkillsNativeCatalogCompletesOnlySelectedScope(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, runner := piSkillFixture(t, scope)
			t.Setenv("ANTHROPIC_API_KEY", "not-a-real-key")
			t.Setenv("PI_CODING_AGENT_DIR", "/foreign")
			got, err := NewProject().Activate(t.Context(), env, request)
			if err != nil || got.Activation != domain.ActivationActive || got.Authentication != domain.AuthenticationNotRequired || got.ActivationAttested || got.AuthenticationAttested || !slices.Equal(got.UserActions, piExpectedCompletedActions(scope)) || len(got.PendingUserActions()) != 0 {
				t.Fatalf("scoped catalog did not complete: %+v %v", got, err)
			}
			cwd := request.Client.ConfigRoot
			if scope == domain.ScopeProject {
				cwd = request.Plan.TargetContext.ProjectRoot
			}
			want := []string{request.Client.ExecutablePath, "--mode", "rpc", "--no-session", "--offline"}
			if runner.calls != 1 || !runner.bounded || !slices.Equal(runner.command.Argv, want) || runner.command.Dir != cwd || runner.command.StdoutLimitBytes != 1<<20 {
				t.Fatalf("finite native query differs: %+v", runner.command)
			}
			if !slices.Contains(runner.command.Env, "PI_CODING_AGENT_DIR="+request.Client.ConfigRoot) || !slices.Contains(runner.command.Env, "PI_OFFLINE=1") {
				t.Fatal("selected profile/offline authority lost")
			}
			for _, item := range runner.command.Env {
				if strings.HasPrefix(item, "ANTHROPIC_API_KEY=") || item == "PI_CODING_AGENT_DIR=/foreign" {
					t.Fatal("ambient provider/profile override inherited")
				}
			}
		})
	}
}

func TestPiSkillsRepeatRequiresFreshNativeCatalog(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, runner := piSkillFixture(t, scope)
			before, _ := capturePiSkills(t.Context(), env, request)
			first, err := NewProject().Activate(t.Context(), env, request)
			if err != nil || first.Activation != domain.ActivationActive {
				t.Fatal(first, err)
			}
			request.Replacing = true
			request.PackageUnchanged = true
			request.PreviousNativeObjects = slices.Clone(first.NativeObjects)
			repeat, err := NewProject().Activate(t.Context(), env, request)
			if err != nil || repeat.Activation != domain.ActivationActive || runner.calls != 2 {
				t.Fatal(repeat, err)
			}
			after, _ := capturePiSkills(t.Context(), env, request)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("query or repeat changed native/trust authority")
			}
			runner.rows = []any{}
			pending, err := NewProject().Activate(t.Context(), env, request)
			if err != nil || pending.Activation != domain.ActivationPrepared || runner.calls != 3 {
				t.Fatal("saved result replaced fresh inventory", pending, err)
			}
		})
	}
}

func TestPiSkillsInitialConfirmedProjectInstallUsesExistingTrustHook(t *testing.T) {
	request, env, runner := piSkillFixture(t, domain.ScopeProject)
	_, entry, projects := piTrustHookPaths(request.Client.ConfigRoot)
	for _, path := range []string{entry, projects, filepath.Join(request.Plan.NativeRegistryRoot, "skills", "owned")} {
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
	}
	request.VerifyOnly = false
	request.PreviousNativeObjects = nil
	got, err := NewProject().Activate(t.Context(), env, request)
	if err != nil || got.Activation != domain.ActivationActive || runner.calls != 1 {
		t.Fatalf("confirmed install did not use ordinary trust setup: %+v %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(request.Client.ConfigRoot, "trust.json")); !os.IsNotExist(err) {
		t.Fatal("installer wrote vendor trust.json")
	}
}

func TestPiSkillsUnqualifiedAuthorityNeverStartsNativeQuery(t *testing.T) {
	for _, mode := range []string{"missing-runner", "missing-parser", "missing-snapshots", "ordinary-runner", "missing-bounded-reader", "backend", "surface", "empty", "mixed", "unsupported", "duplicate", "unknown-source", "policy", "attestation", "original-profile", "wrong-selected-name", "no-package-receipt"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := piSkillFixture(t, domain.ScopeProject)
			out := piSkillOutcome(request)
			switch mode {
			case "missing-runner":
				env.Runner = nil
			case "missing-parser":
				env.ParseSkill = nil
			case "missing-snapshots":
				env.Snapshots = nil
			case "ordinary-runner":
				env.Runner = piOrdinarySkillRunner{}
			case "missing-bounded-reader":
				env.NativeConfig = nativeconfig.NewWithFileIO(piReadOnlySkillFiles{})
			case "backend":
				request.BackendExecutable = filepath.Join(t.TempDir(), "foreign")
			case "surface":
				request.Client.Surfaces = []domain.ClientSurface{{ID: "pi_config", Detected: true}}
			case "empty":
				request.Plan.Components = nil
			case "mixed":
				request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportNative})
			case "unsupported":
				request.Plan.Components[0].Support = domain.SupportUnsupported
			case "duplicate":
				request.Plan.Components = append(request.Plan.Components, request.Plan.Components[0])
			case "unknown-source":
				request.Plan.Components[0].Source = "unknown"
			case "policy":
				request.Plan.Policy = domain.PolicyBlocked
			case "attestation":
				request.ActivationComplete = true
			case "original-profile":
				request.Plan.TargetContext.UserConfigRoot = t.TempDir()
			case "wrong-selected-name":
				request.Plan.Components[0].Name = "another"
			case "no-package-receipt":
				request.Delivery.NativeObjects = request.Delivery.NativeObjects[1:]
				out.NativeObjects = slices.Clone(request.Delivery.NativeObjects)
			}
			got, err := completePiSkills(t.Context(), env, request, out)
			if mode == "missing-bounded-reader" || mode == "wrong-selected-name" || mode == "no-package-receipt" {
				if err == nil || got.Activation != domain.ActivationFailed || got.NativeEffect != domain.NativeEffectUncertain || runner.calls != 0 {
					t.Fatalf("unqualified capture did not fail closed: %+v %v calls%d", got, err, runner.calls)
				}
			} else if err != nil || got.Activation != domain.ActivationPrepared || runner.calls != 0 {
				t.Fatalf("unqualified request queried native: %+v %v calls%d", got, err, runner.calls)
			}
		})
	}
}

type piOrdinarySkillRunner struct{}

func (piOrdinarySkillRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("unexpected")
}

func TestPiSkillsMalformedOrForeignCatalogStaysPrepared(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		for _, mode := range []string{"wrong-id", "failed", "wrong-command", "wrong-name", "wrong-description", "wrong-path", "wrong-source", "wrong-scope", "package-origin", "wrong-base", "duplicate", "null-row", "no-callback", "host-preflight"} {
			t.Run(string(scope)+"/"+mode, func(t *testing.T) {
				request, env, runner := piSkillFixture(t, scope)
				row := runner.rows[0].(map[string]any)
				info := row["sourceInfo"].(map[string]any)
				switch mode {
				case "wrong-id":
					runner.mutate = func(v map[string]any) { v["id"] = "other" }
				case "failed":
					runner.mutate = func(v map[string]any) { v["success"] = false }
				case "wrong-command":
					runner.mutate = func(v map[string]any) { v["command"] = "get_state" }
				case "wrong-name":
					row["name"] = "skill:other"
				case "wrong-description":
					row["description"] = "other"
				case "wrong-path":
					info["path"] = filepath.Join(t.TempDir(), "SKILL.md")
				case "wrong-source":
					info["source"] = "cli"
				case "wrong-scope":
					info["scope"] = "temporary"
				case "package-origin":
					info["origin"] = "package"
					info["source"] = "npm:foreign"
					info["baseDir"] = t.TempDir()
				case "wrong-base":
					info["baseDir"] = t.TempDir()
				case "duplicate":
					runner.rows = append(runner.rows, row)
				case "null-row":
					runner.rows = append(runner.rows, nil)
				case "no-callback":
					env.Runner = piNoExchangeSkillRunner{}
				case "host-preflight":
					runner.cleanup = errors.New("ordinary containment unsupported: live task children unavailable")
				}
				got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
				if err != nil || got.Activation != domain.ActivationPrepared {
					t.Fatalf("bad native catalog promoted: %+v %v", got, err)
				}
			})
		}
	}
}

type piNoExchangeSkillRunner struct{ piOrdinarySkillRunner }

func (piNoExchangeSkillRunner) RunDuplexToCompletion(context.Context, legacyports.Command, func(io.Writer, io.Reader) error) error {
	return nil
}

func TestPiSkillsTrustChangesAndOwnedMutationsFailClosed(t *testing.T) {
	for _, mode := range []string{"native-resource", "native-missing", "source-resource", "trust", "foreign-hook", "profile-policy", "component"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := piSkillFixture(t, domain.ScopeProject)
			runner.after = func() {
				switch mode {
				case "native-resource":
					piTrustWrite(t, filepath.Join(request.Delivery.NativeObjects[1].Path, "extra.txt"), "late native edit")
				case "native-missing":
					if err := os.RemoveAll(request.Delivery.NativeObjects[1].Path); err != nil {
						t.Fatal(err)
					}
				case "foreign-hook":
					_, entry, _ := piTrustHookPaths(request.Client.ConfigRoot)
					piTrustWrite(t, entry, "foreign extension bytes")
				case "source-resource":
					piTrustWrite(t, filepath.Join(request.Plan.ActivePath, "late.txt"), "late source edit")
				case "trust":
					_, _, record := piTrustHookPaths(request.Client.ConfigRoot)
					piTrustWrite(t, record, "{}")
				case "profile-policy":
					piTrustWrite(t, filepath.Join(request.Client.ConfigRoot, "settings.json"), "{\"defaultProjectTrust\":\"never\"}")
				case "component":
					request.Plan.Components[0].Name = "changed"
				}
			}
			got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
			if err == nil || got.Activation == domain.ActivationActive || got.NativeEffect != domain.NativeEffectUncertain {
				t.Fatalf("late authority change accepted: %+v %v", got, err)
			}
		})
	}
}

func TestPiSkillsMissingProjectTrustIsReadOnlyAndUserNeedsNoProjectTrust(t *testing.T) {
	request, env, runner := piSkillFixture(t, domain.ScopeProject)
	_, _, record := piTrustHookPaths(request.Client.ConfigRoot)
	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}
	got, err := NewProject().Activate(t.Context(), env, request)
	if err != nil || got.Activation != domain.ActivationPrepared || runner.calls != 0 {
		t.Fatal(got, err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("VerifyOnly recreated trust")
	}
	user, userEnv, userRunner := piSkillFixture(t, domain.ScopeUser)
	piTrustWrite(t, filepath.Join(user.Client.ConfigRoot, "settings.json"), "{\"defaultProjectTrust\":\"never\"}")
	active, err := NewProject().Activate(t.Context(), userEnv, user)
	if err != nil || active.Activation != domain.ActivationActive || userRunner.calls != 1 {
		t.Fatal(active, err)
	}
	_, hook, projects := piTrustHookPaths(user.Client.ConfigRoot)
	for _, path := range []string{hook, projects, filepath.Join(user.Client.ConfigRoot, "trust.json")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("User observation created Project trust", path)
		}
	}
}

func TestPiSkillsCancellationAndUnclosedQueryCannotPromote(t *testing.T) {
	for _, mode := range []string{"cancel", "unclosed"} {
		t.Run(mode, func(t *testing.T) {
			request, env, runner := piSkillFixture(t, domain.ScopeProject)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "cancel" {
				runner.after = cancel
			} else {
				runner.cleanup = &legacyports.ProcessClosureError{Cause: errors.New("owned descendant remains")}
			}
			got, err := completePiSkills(ctx, env, request, piSkillOutcome(request))
			if err == nil || got.Activation == domain.ActivationActive {
				t.Fatal(got, err)
			}
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation lost", err)
			}
			if mode == "unclosed" && (!domain.IsUnclosedClientProbe(err) || got.NativeEffect != domain.NativeEffectUncertain) {
				t.Fatal("unclosed cleanup lost", got, err)
			}
		})
	}
}

type piReadOnlySkillFiles struct{}

func (piReadOnlySkillFiles) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	v, err := nativeconfig.New().ReadExactFile(path)
	return v.Body, v.Mode, v.Exists, err
}
func (piReadOnlySkillFiles) WriteAtomic(string, []byte, os.FileMode) error {
	return errors.New("unexpected write")
}
func (piReadOnlySkillFiles) RemoveNoFollow(string) error { return errors.New("unexpected remove") }

type piSkillMetadataABA struct {
	piReadOnlySkillFiles
	path  string
	body  []byte
	test  *testing.T
	reads int
}

func (files *piSkillMetadataABA) ReadNoFollowBounded(selected string, limit int64) ([]byte, os.FileMode, bool, error) {
	if strings.HasSuffix(selected, string(filepath.Separator)+"SKILL.md") {
		files.reads++
		if err := os.WriteFile(files.path, bytes.ReplaceAll(files.body, []byte("Original Skill"), []byte("Substitute Skill")), 0o600); err != nil {
			files.test.Fatal(err)
		}
		v, err := nativeconfig.New().ReadExactFileBounded(selected, limit)
		if restore := os.WriteFile(files.path, files.body, 0o600); restore != nil {
			files.test.Fatal(restore)
		}
		return v.Body, v.Mode, v.Exists, err
	}
	v, err := nativeconfig.New().ReadExactFileBounded(selected, limit)
	return v.Body, v.Mode, v.Exists, err
}

func TestPiSkillsSealedMetadataRejectsTransientLiveSubstitute(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, runner := piSkillFixture(t, scope)
			path := filepath.Join(request.Delivery.NativeObjects[1].Path, "SKILL.md")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			files := &piSkillMetadataABA{path: path, body: body, test: t}
			env.NativeConfig = nativeconfig.NewWithFileIO(files)
			runner.rows[0].(map[string]any)["description"] = "Substitute Skill"
			got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
			if err != nil || got.Activation != domain.ActivationPrepared || runner.calls != 1 || files.reads < 2 {
				t.Fatalf("live metadata ABA authorized substituted catalog: %+v %v reads%d", got, err, files.reads)
			}
		})
	}
}

func TestPiSkillsKeepsExplicitAuthenticationAndUnrelatedActions(t *testing.T) {
	for _, auth := range []domain.AuthenticationState{domain.AuthenticationPending, domain.AuthenticationNotChecked} {
		request, env, _ := piSkillFixture(t, domain.ScopeUser)
		request.Plan.Authentication = auth
		if auth == domain.AuthenticationNotChecked {
			request.Plan.Warnings = []string{"authentication_requirement_unknown"}
		}
		got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
		if err != nil || got.Activation != domain.ActivationActive || got.Authentication != auth || !slices.Equal(got.UserActions, []string{"unrelated action"}) {
			t.Fatal(got, err)
		}
	}
}
func TestPiSkillCatalogResponseRejectsAmbiguity(t *testing.T) {
	good := `{"id":"owned","type":"response","command":"get_commands","success":true,"data":{"commands":[]}}`
	if rows, err := piSkillCatalogResponse([]byte(good), "owned"); err != nil || rows == nil {
		t.Fatal(rows, err)
	}
	for _, raw := range []string{
		`{"id":"owned","ID":"other","type":"response","command":"get_commands","success":true,"data":{"commands":[]}}`,
		`{"id":"owned","type":"response","command":"get_commands","success":true,"data":{"commands":[],"Commands":[]}}`,
		`{"id":"owned","type":"response","command":"get_commands","success":true,"data":{"commands":null}}`,
		good + "{}",
	} {
		if _, err := piSkillCatalogResponse([]byte(raw), "owned"); err == nil {
			t.Fatalf("ambiguous protocol accepted: %s", raw)
		}
	}
}

func TestPiSkillsLiveCatalogByteLimitAndPreCancelledContext(t *testing.T) {
	request, env, runner := piSkillFixture(t, domain.ScopeUser)
	runner.rows[0].(map[string]any)["description"] = strings.Repeat("x", (1<<20)+1)
	got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
	if err != nil || got.Activation != domain.ActivationPrepared || runner.calls != 1 {
		t.Fatal(got, err)
	}
	request, env, runner = piSkillFixture(t, domain.ScopeUser)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err = completePiSkills(ctx, env, request, piSkillOutcome(request))
	if !errors.Is(err, context.Canceled) || runner.calls != 0 || got.Activation == domain.ActivationActive {
		t.Fatal(got, err)
	}
}

func piExpectedCompletedActions(scope domain.InstallScope) []string {
	if scope == domain.ScopeProject {
		return []string{piCompletedTrustNotice}
	}
	return nil
}

func TestPiSkillsCompletedTrustDisclosureRemainsInformational(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		t.Run(string(scope), func(t *testing.T) {
			request, env, _ := piSkillFixture(t, scope)
			got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
			if err != nil || got.Activation != domain.ActivationActive || !slices.Equal(got.PendingUserActions(), []string{"unrelated action"}) {
				t.Fatalf("completed disclosure hid pending action: %+v %v", got, err)
			}
			if scope == domain.ScopeProject {
				if !slices.Contains(got.UserActions, piCompletedTrustNotice) || !slices.Contains(got.CompletedUserActions, piCompletedTrustNotice) {
					t.Fatal("completed trust scope disclosure lost")
				}
			} else if slices.Contains(got.UserActions, piCompletedTrustNotice) || len(got.CompletedUserActions) != 0 {
				t.Fatal("User catalog gained Project trust disclosure")
			}
		})
	}
}

// Inject only read behavior; the observer must not obtain an OS fallback or
// turn a failed read into evidence that a native mutation occurred.
type piSkillFinalReadFiles struct {
	piReadOnlySkillFiles
	afterQuery *bool
	settings   string
	cancel     context.CancelFunc
	fail       bool
}

func (f piSkillFinalReadFiles) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	if *f.afterQuery && path == f.settings && f.fail {
		return nil, 0, false, &os.PathError{Op: "read", Path: path, Err: syscall.EACCES}
	}
	v, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	if *f.afterQuery && strings.HasSuffix(path, "trust.json") && f.cancel != nil {
		f.cancel()
	}
	return v.Body, v.Mode, v.Exists, err
}
func TestPiSkillsFailedObservationReadPreservesVerifiedBinding(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		for _, effect := range []domain.NativeEffectState{domain.NativeEffectCommitted, domain.NativeEffectUnchanged} {
			t.Run(string(scope)+"/"+string(effect), func(t *testing.T) {
				request, env, runner := piSkillFixture(t, scope)
				after := false
				env.NativeConfig = nativeconfig.NewWithFileIO(piSkillFinalReadFiles{afterQuery: &after, settings: filepath.Join(request.Client.ConfigRoot, "settings.json"), fail: true})
				runner.after = func() { after = true }
				out := piSkillOutcome(request)
				out.NativeEffect = effect
				got, err := completePiSkills(t.Context(), env, request, out)
				if err != nil || !reflect.DeepEqual(got, out) || runner.calls != 1 {
					t.Fatalf("read failure replaced verified binding: %+v %v", got, err)
				}
				after = false
				runner.after = nil
				active, err := completePiSkills(t.Context(), env, request, out)
				if err != nil || active.Activation != domain.ActivationActive || runner.calls != 2 {
					t.Fatalf("fresh retry blocked: %+v %v", active, err)
				}
			})
		}
	}
}

func TestPiSkillsCancellationDuringFinalTrustReadCannotPromote(t *testing.T) {
	request, env, runner := piSkillFixture(t, domain.ScopeUser)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	after := false
	env.NativeConfig = nativeconfig.NewWithFileIO(piSkillFinalReadFiles{afterQuery: &after, cancel: cancel})
	runner.after = func() { after = true }
	got, err := completePiSkills(ctx, env, request, piSkillOutcome(request))
	if !errors.Is(err, context.Canceled) || got.Activation == domain.ActivationActive || runner.calls != 1 {
		t.Fatal(got, err)
	}
}

func TestPiSkillsInternalTimeoutKeepsVerifiedPrepared(t *testing.T) {
	request, env, runner := piSkillFixture(t, domain.ScopeUser)
	runner.expire = true
	after := false
	files := &piReviewTrustFiles{after: &after}
	env.NativeConfig = nativeconfig.NewWithFileIO(files)
	runner.after = func() { after = true }
	out := piSkillOutcome(request)
	out.NativeEffect = domain.NativeEffectCommitted
	got, err := completePiSkills(t.Context(), env, request, out)
	if t.Context().Err() != nil {
		t.Fatal("parent expired instead of internal catalog budget")
	}
	if err != nil || !reflect.DeepEqual(got, out) || runner.calls != 1 || files.finalReads == 0 {
		t.Fatalf("closed catalog timeout replaced verified binding or skipped custody: %+v %v", got, err)
	}
}

func TestPiSkillsReadFailureClassifierPreservesStrictErrors(t *testing.T) {
	ioFailure := &os.PathError{Op: "read", Path: "authority.json", Err: syscall.EIO}
	for _, tc := range []struct {
		name string
		err  error
		soft bool
	}{
		{"direct-permission", &os.PathError{Op: "open", Path: "authority.json", Err: syscall.EACCES}, true},
		{"direct-io", ioFailure, true},
		{"missing", &os.PathError{Op: "lstat", Path: "authority.json", Err: syscall.ENOENT}, false},
		{"not-directory", &os.PathError{Op: "lstat", Path: "authority.json", Err: syscall.ENOTDIR}, false},
		{"symlink-loop", &os.PathError{Op: "open", Path: "authority.json", Err: syscall.ELOOP}, false},
		{"wrapped", fmt.Errorf("unknown observation: %w", ioFailure), false},
		{"joined-race", errors.Join(ioFailure, nativeconfig.ErrConcurrentChange), false},
		{"nested-race", &os.PathError{Op: "read", Path: "authority.json", Err: errors.Join(syscall.EIO, nativeconfig.ErrConcurrentChange)}, false},
		{"opaque", errors.New("unknown reader refusal"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := piSkillReadFailure(tc.err)
			if errors.Is(got, errPiSkillObservationRead) != tc.soft {
				t.Fatalf("incorrect read classification: %v", got)
			}
			if !tc.soft && got != tc.err {
				t.Fatal("strict error identity changed")
			}
		})
	}
}

type piSkillFinalAdmissionFiles struct {
	piReadOnlySkillFiles
	afterQuery *bool
	target     string
	beforeRead func()
	failure    error
	observed   error
}

func (f *piSkillFinalAdmissionFiles) ReadNoFollowBounded(path string, limit int64) ([]byte, os.FileMode, bool, error) {
	if *f.afterQuery && path == f.target {
		if f.beforeRead != nil {
			before := f.beforeRead
			f.beforeRead = nil
			before()
		}
		if f.failure != nil {
			f.observed = f.failure
			return nil, 0, false, f.failure
		}
	}
	v, err := nativeconfig.New().ReadExactFileBounded(path, limit)
	if *f.afterQuery && path == f.target {
		f.observed = err
	}
	return v.Body, v.Mode, v.Exists, err
}

func TestPiSkillsFinalAuthorityAdmissionRefusalsStayHard(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		for _, mode := range []string{"directory", "symlink", "oversized", "joined-race"} {
			t.Run(string(scope)+"/"+mode, func(t *testing.T) {
				request, env, runner := piSkillFixture(t, scope)
				after := false
				path := filepath.Join(request.Client.ConfigRoot, "settings.json")
				files := &piSkillFinalAdmissionFiles{afterQuery: &after, target: path}
				if mode == "joined-race" {
					files.failure = errors.Join(&os.PathError{Op: "lstat", Path: path, Err: syscall.EACCES}, nativeconfig.ErrConcurrentChange)
				} else {
					files.beforeRead = func() {
						if err := os.RemoveAll(path); err != nil {
							t.Fatal(err)
						}
						if mode == "directory" {
							if err := os.Mkdir(path, 0700); err != nil {
								t.Fatal(err)
							}
						} else if mode == "symlink" {
							target := filepath.Join(t.TempDir(), "ordinary.json")
							if err := os.WriteFile(target, []byte("{}"), 0600); err != nil {
								t.Fatal(err)
							}
							if err := os.Symlink(target, path); err != nil {
								t.Skipf("symlink unavailable: %v", err)
							}
						} else {
							if err := os.WriteFile(path, make([]byte, (1<<20)+1), 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				env.NativeConfig = nativeconfig.NewWithFileIO(files)
				runner.after = func() { after = true }
				got, err := completePiSkills(t.Context(), env, request, piSkillOutcome(request))
				if files.observed == nil {
					t.Fatal("final bounded-read admission path was not reached")
				}
				if err == nil || got.Activation != domain.ActivationFailed || got.NativeEffect != domain.NativeEffectUncertain || errors.Is(err, errPiSkillObservationRead) {
					t.Fatalf("strict admission softened: %+v %v", got, err)
				}
			})
		}
	}
}
