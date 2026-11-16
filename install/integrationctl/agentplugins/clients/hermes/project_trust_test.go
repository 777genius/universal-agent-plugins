package hermes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// fakeHermesCLI models the pinned Hermes 0.23 trust CLI, including its exit 0
// on refused or unsaved grants, plus the one Git query UAP issues.
type fakeHermesCLI struct {
	mu          sync.Mutex
	git, hermes string
	gitRoot     string
	configPath  string
	trusted     any
	discovery   any
	// trustStdout/trustStderr override the native confirmation; noSave and
	// dropPrior model managed scopes that discard or replace the list.
	trustStdout, trustStderr string
	noSave, dropPrior        bool
	unclosedTrust            bool
	calls                    [][]string
}

func (cli *fakeHermesCLI) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	cli.calls = append(cli.calls, slices.Clone(command.Argv))
	result := legacyports.CommandResult{CleanupComplete: true}
	reply := func(value any) {
		body, _ := json.Marshal(value)
		body = append(body, '\n')
		result.Stdout = body
	}
	switch {
	case command.Argv[0] == cli.git && slices.Equal(command.Argv[1:], []string{"-C", command.Dir, "rev-parse", "--show-toplevel"}):
		result.Stdout = []byte(cli.gitRoot + "\n")
	case command.Argv[0] != cli.hermes:
		result.ExitCode = 127
	case slices.Equal(command.Argv[1:], []string{"config", "path"}):
		result.Stdout = []byte(cli.configPath + "\n")
	case slices.Equal(command.Argv[1:], []string{"config", "get", "--json", "skills.trusted_project_dirs"}):
		reply(cli.trusted)
	case slices.Equal(command.Argv[1:], []string{"config", "get", "--json", "skills.project_discovery"}):
		reply(cli.discovery)
	case len(command.Argv) == 4 && command.Argv[1] == "skills" && command.Argv[2] == "trust":
		if cli.unclosedTrust {
			return legacyports.CommandResult{}, fmt.Errorf("supervised cleanup did not finish")
		}
		cli.trust(command.Argv[3], &result)
	default:
		result.ExitCode = 2
	}
	return result, nil
}

func (cli *fakeHermesCLI) trust(root string, result *legacyports.CommandResult) {
	entries := cli.entries()
	line := "Trusted: " + root
	if slices.Contains(entries, root) {
		line = "Already trusted: " + root
	} else if !cli.noSave {
		if cli.dropPrior {
			entries = nil
		}
		entries = append(entries, root)
		cli.trusted = entries
	}
	if cli.trustStdout != "" {
		line = cli.trustStdout
	}
	result.Stdout = []byte(line + "\nStart a new session to load project Skills.\n")
	result.Stderr = []byte(cli.trustStderr)
}

func (cli *fakeHermesCLI) entries() []string {
	switch typed := cli.trusted.(type) {
	case string:
		return []string{typed}
	case []string:
		return slices.Clone(typed)
	}
	return nil
}

func (cli *fakeHermesCLI) trustCalls() int {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	count := 0
	for _, call := range cli.calls {
		if len(call) > 2 && call[1] == "skills" && call[2] == "trust" {
			count++
		}
	}
	return count
}

type hermesTrustFixture struct {
	project, profile, home, managed string
	cli                             *fakeHermesCLI
	grant                           hermesTrustGrant
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func newHermesTrustFixture(t *testing.T) *hermesTrustFixture {
	t.Helper()
	base := canonicalTempDir(t)
	f := &hermesTrustFixture{
		project: filepath.Join(base, "repo"), profile: filepath.Join(base, "home", ".hermes"),
		home: filepath.Join(base, "home"), managed: filepath.Join(base, "managed"),
	}
	for _, dir := range []string{filepath.Join(f.project, ".git"), f.profile, f.managed} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hermes := filepath.Join(base, "bin", "hermes")
	if filepath.Separator == '\\' {
		hermes += ".exe"
	}
	f.cli = &fakeHermesCLI{git: filepath.Join(base, "bin", "git"), hermes: hermes, gitRoot: filepath.ToSlash(f.project),
		configPath: filepath.Join(f.profile, "config.yaml"), trusted: []string{"/srv/prior"}}
	f.grant = hermesTrustGrant{
		runner: f.cli, kernel: nativeconfig.New(), executable: hermes, profile: f.profile, project: f.project,
		lookup: func(name string) (string, error) {
			if name != "git" {
				return "", fmt.Errorf("unexpected lookup %q", name)
			}
			return f.cli.git, nil
		},
		environ: []string{"HOME=" + f.home, "USERPROFILE=" + f.home, "PATH=/usr/bin", "HERMES_MANAGED_DIR=" + f.managed, "FORCE_COLOR=1"},
	}
	return f
}

func (f *hermesTrustFixture) write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHermesProjectTrustGrantsExactGitRootAndVerifiesReadBack(t *testing.T) {
	f := newHermesTrustFixture(t)
	report, err := f.grant.run(context.Background())
	if err != nil || !report.granted || report.root != f.project {
		t.Fatalf("verified grant refused: %+v %v", report, err)
	}
	if got := f.cli.entries(); !slices.Equal(got, []string{"/srv/prior", f.project}) {
		t.Fatalf("trust list = %v", got)
	}
	for _, call := range f.cli.calls {
		if slices.Contains(call, "-p") || slices.Contains(call, "set") || slices.Contains(call, ".") {
			t.Fatalf("trust used a profile flag, config set or a relative path: %v", call)
		}
	}
	env := f.grant.environment()
	if slices.Contains(env, "FORCE_COLOR=1") || !slices.Contains(env, "NO_COLOR=1") || !slices.Contains(env, "HOME="+f.home) {
		t.Fatalf("trust environment differs from the user's Hermes environment: %v", env)
	}
	action := report.userAction()
	for _, fragment := range []string{f.project, ".agents/skills", "hermes skills untrust", "quarantined", "new Hermes session"} {
		if !strings.Contains(action, fragment) {
			t.Fatalf("granted result omits %q: %s", fragment, action)
		}
	}
	again, err := f.grant.run(context.Background())
	if err != nil || !again.granted || !slices.Equal(f.cli.entries(), []string{"/srv/prior", f.project}) {
		t.Fatalf("repeated grant was not idempotent: %+v %v %v", again, err, f.cli.entries())
	}
}

func TestHermesProjectTrustPreconditionsFallBackWithoutWriting(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		setup        func(*testing.T, *hermesTrustFixture)
	}{
		{"runner unavailable", "runner is unavailable", func(_ *testing.T, f *hermesTrustFixture) { f.grant.runner = nil }},
		{"lookup unavailable", "runner is unavailable", func(_ *testing.T, f *hermesTrustFixture) { f.grant.lookup = nil }},
		{"managed environment", "HERMES_MANAGED", func(_ *testing.T, f *hermesTrustFixture) {
			f.grant.environ = append(f.grant.environ, "HERMES_MANAGED=nix")
		}},
		{"managed marker", ".managed marker", func(t *testing.T, f *hermesTrustFixture) { f.write(t, filepath.Join(f.profile, ".managed"), "") }},
		{"container mode", ".container-mode marker", func(t *testing.T, f *hermesTrustFixture) { f.write(t, filepath.Join(f.profile, ".container-mode"), "") }},
		{"backend environment", "non-file configuration backend", func(_ *testing.T, f *hermesTrustFixture) {
			f.grant.environ = append(f.grant.environ, "HERMES_CONFIG_BACKEND=postgres")
		}},
		{"backend profile dotenv", "non-file configuration backend", func(t *testing.T, f *hermesTrustFixture) {
			f.write(t, filepath.Join(f.profile, ".env"), "# comment\nHERMES_CONFIG_BACKEND_NOTE=x\nexport HERMES_CONFIG_BACKEND=\"postgres\"\n")
		}},
		{"backend managed dotenv", "non-file configuration backend", func(t *testing.T, f *hermesTrustFixture) {
			f.write(t, filepath.Join(f.managed, ".env"), "HERMES_CONFIG_BACKEND=postgres\n")
		}},
		{"managed config pins trust", "pins skills.trusted_project_dirs", func(t *testing.T, f *hermesTrustFixture) {
			f.write(t, filepath.Join(f.managed, "config.yaml"), "skills:\n  trusted_project_dirs: []\n")
		}},
		{"relative managed dir", "HERMES_MANAGED_DIR", func(_ *testing.T, f *hermesTrustFixture) {
			f.grant.environ = append(f.grant.environ, "HERMES_MANAGED_DIR=relative")
		}},
		{"sticky profile selects another config", "different configuration", func(_ *testing.T, f *hermesTrustFixture) {
			f.cli.configPath = filepath.Join(f.profile, "profiles", "work", "config.yaml")
		}},
		{"unreadable trust snapshot", "existing Hermes trust list", func(_ *testing.T, f *hermesTrustFixture) { f.cli.trusted = 7 }},
		{"git root is home", "entire home directory", func(t *testing.T, f *hermesTrustFixture) {
			if err := os.MkdirAll(filepath.Join(f.home, ".git"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Join(f.project, ".git")); err != nil {
				t.Fatal(err)
			}
			f.project = filepath.Join(f.home, "nested")
			if err := os.MkdirAll(f.project, 0o700); err != nil {
				t.Fatal(err)
			}
			f.grant.project, f.cli.gitRoot = f.project, filepath.ToSlash(f.home)
		}},
		{"git root contains home", "entire home directory", func(t *testing.T, f *hermesTrustFixture) {
			base := filepath.Dir(f.project)
			if err := os.MkdirAll(filepath.Join(base, ".git"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Join(f.project, ".git")); err != nil {
				t.Fatal(err)
			}
			f.grant.project, f.cli.gitRoot = base, filepath.ToSlash(base)
		}},
		{"relative git root", "Git repository root could not be confirmed", func(_ *testing.T, f *hermesTrustFixture) { f.cli.gitRoot = "repo" }},
		{"git root differs from nearest marker", "Git repository root could not be confirmed", func(t *testing.T, f *hermesTrustFixture) {
			other := filepath.Join(filepath.Dir(f.project), "other")
			if err := os.MkdirAll(filepath.Join(other, ".git"), 0o700); err != nil {
				t.Fatal(err)
			}
			f.cli.gitRoot = filepath.ToSlash(other)
		}},
		{"git marker is a symlink", "Git repository root could not be confirmed", func(t *testing.T, f *hermesTrustFixture) {
			if err := os.RemoveAll(filepath.Join(f.project, ".git")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(f.managed, filepath.Join(f.project, ".git")); err != nil {
				t.Skip("symlinks unavailable:", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHermesTrustFixture(t)
			tc.setup(t, f)
			before := f.cli.entries()
			report, err := f.grant.run(context.Background())
			if err != nil || report.granted || report.attempted || !strings.Contains(report.reason, tc.reason) {
				t.Fatalf("precondition did not fall back: %+v %v", report, err)
			}
			if f.cli.trustCalls() != 0 || !slices.Equal(before, f.cli.entries()) {
				t.Fatalf("refused grant changed native trust: %v", f.cli.calls)
			}
			action := report.userAction()
			if !strings.HasPrefix(action, "UAP did not change Hermes project trust: ") || !strings.Contains(action, "`hermes skills trust '") {
				t.Fatalf("fallback lacks the exact manual command: %s", action)
			}
		})
	}
}

func TestHermesProjectTrustAllowsUnmanagedPackageInstalls(t *testing.T) {
	for _, value := range []string{"", "false", "0", "No", "off", "brew", "homebrew"} {
		f := newHermesTrustFixture(t)
		f.grant.environ = append(f.grant.environ, "HERMES_MANAGED="+value, "HERMES_CONFIG_BACKEND=file")
		f.write(t, filepath.Join(f.profile, ".env"), "HERMES_CONFIG_BACKEND_NOTE=x\nHERMES_CONFIG_BACKEND='file'\n")
		f.write(t, filepath.Join(f.managed, "config.yaml"), "skills:\n  project_discovery: true\n")
		if report, err := f.grant.run(context.Background()); err != nil || !report.granted {
			t.Fatalf("HERMES_MANAGED=%q refused: %+v %v", value, report, err)
		}
	}
}

func TestHermesProjectTrustPostCheckNeverTrustsExitCode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(*fakeHermesCLI)
		missing bool
	}{
		{"managed save refused on stderr", func(cli *fakeHermesCLI) {
			cli.noSave, cli.trustStderr = true, "Cannot save configuration: managed by Nix\n"
		}, false},
		{"confirmation line missing", func(cli *fakeHermesCLI) { cli.noSave, cli.trustStdout = true, "Not a directory" }, false},
		{"confirmation names another root", func(cli *fakeHermesCLI) { cli.trustStdout = "Trusted: /elsewhere" }, false},
		{"root absent afterwards", func(cli *fakeHermesCLI) { cli.noSave = true }, false},
		{"prior entries lost", func(cli *fakeHermesCLI) { cli.dropPrior = true }, true},
		{"project discovery disabled", func(cli *fakeHermesCLI) { cli.discovery = false }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newHermesTrustFixture(t)
			tc.setup(f.cli)
			report, err := f.grant.run(context.Background())
			if err != nil || report.granted || !report.attempted || report.reason == "" {
				t.Fatalf("post-check accepted an unverified grant: %+v %v", report, err)
			}
			action := report.userAction()
			if !strings.HasPrefix(action, "UAP could not confirm Hermes project trust: ") || !strings.Contains(action, hermesTrustCommand(f.project)) {
				t.Fatalf("post-check failure lacks the exact manual command: %s", action)
			}
			if tc.missing != strings.Contains(action, "/srv/prior") {
				t.Fatalf("lost prior entries were not reported exactly: %s", action)
			}
		})
	}
}

func TestHermesProjectTrustUnclosedProcessIsUncertainNotGranted(t *testing.T) {
	f := newHermesTrustFixture(t)
	f.cli.unclosedTrust = true
	report, err := f.grant.run(context.Background())
	if !domain.IsUnclosedClientProbe(err) || report.granted || !report.attempted {
		t.Fatalf("unclosed trust process was not surfaced: %+v %v", report, err)
	}
	request, env := hermesTrustActivation(t, filepath.Join(canonicalTempDir(t), "repo"), f.cli, f.profile)
	if err := os.MkdirAll(filepath.Join(request.Plan.TargetContext.ProjectRoot, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	f.cli.gitRoot = filepath.ToSlash(request.Plan.TargetContext.ProjectRoot)
	outcome, err := New().Activate(context.Background(), env, request)
	if !domain.IsUnclosedClientProbe(err) || outcome.NativeEffect != domain.NativeEffectUncertain ||
		!strings.HasPrefix(outcome.UserActions[len(outcome.UserActions)-1], "UAP could not confirm Hermes project trust: ") {
		t.Fatalf("activation hid an unclosed trust process: %+v %v", outcome, err)
	}
}

func TestHermesProjectTrustCanonicalizesSymlinkedProject(t *testing.T) {
	f := newHermesTrustFixture(t)
	alias := filepath.Join(filepath.Dir(f.project), "alias")
	if err := os.Symlink(f.project, alias); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	f.grant.project = alias
	report, err := f.grant.run(context.Background())
	if err != nil || !report.granted || report.root != f.project || !slices.Contains(f.cli.entries(), f.project) || slices.Contains(f.cli.entries(), alias) {
		t.Fatalf("trust did not use the physical Git root: %+v %v %v", report, err, f.cli.entries())
	}
	f.cli.gitRoot = filepath.ToSlash(alias)
	if report, err := f.grant.run(context.Background()); err != nil || report.granted || report.attempted {
		t.Fatalf("an aliased Git root was trusted: %+v %v", report, err)
	}
}

func TestHermesProjectTrustCaseInsensitiveProjectSelection(t *testing.T) {
	f := newHermesTrustFixture(t)
	upper := filepath.Join(filepath.Dir(f.project), "REPO")
	if !sameDirectory(upper, f.project) {
		t.Skip("case-sensitive filesystem")
	}
	f.grant.project = upper
	report, err := f.grant.run(context.Background())
	if err != nil || !report.granted || report.root != f.project {
		t.Fatalf("case alias did not resolve to Git's root: %+v %v", report, err)
	}
}

func hermesTrustActivation(t *testing.T, root string, cli *fakeHermesCLI, profile string) (domain.ActivationRequest, clients.Env) {
	t.Helper()
	fixture := newHermesSkillFixture(t, root, domain.ScopeProject)
	plan := fixture.plan
	plan.InstallIntent = domain.InstallIntentAutomatic
	plan.TargetContext.Scope = domain.ScopeProject
	request := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: profile, ExecutablePath: cli.hermes},
		Plan:   plan, Delivery: domain.StagedDelivery{ClientID: domain.ClientHermes, ActivePath: plan.ActivePath, NativeObjects: fixture.objects},
	}
	env := clients.Env{Runner: cli, NativeConfig: nativeconfig.New(), LookupExecutable: func(string) (string, error) { return cli.git, nil }}
	return request, env
}

func TestHermesConfirmedProjectActivationGrantsTrustOnlyOutsideVerification(t *testing.T) {
	f := newHermesTrustFixture(t)
	request, env := hermesTrustActivation(t, f.project, f.cli, f.profile)
	plan := request.Plan
	plan.UserActions = nil
	if err := New().RefinePlan(context.Background(), clients.PlanInput{}, &plan); err != nil || len(f.cli.calls) != 0 ||
		!strings.Contains(strings.Join(plan.UserActions, "\n"), "trust this whole Git repository root") {
		t.Fatalf("planning ran native trust or omitted the disclosure: %v %v %v", plan.UserActions, err, f.cli.calls)
	}
	outcome, err := New().Activate(context.Background(), env, request)
	if err != nil || outcome.Activation != domain.ActivationPrepared || outcome.ActivationAttested || f.cli.trustCalls() != 1 {
		t.Fatalf("confirmed activation did not ask Hermes once: %+v %v %v", outcome, err, f.cli.calls)
	}
	if slices.Contains(outcome.UserActions, hermesCompletedAction) || !strings.HasPrefix(outcome.UserActions[len(outcome.UserActions)-1], "Hermes now trusts the Git repository root "+f.project) {
		t.Fatalf("granted outcome is not reported: %v", outcome.UserActions)
	}
	calls := len(f.cli.calls)
	request.VerifyOnly = true
	verified, err := New().Activate(context.Background(), env, request)
	if err != nil || len(f.cli.calls) != calls || !slices.Contains(verified.UserActions, hermesCompletedAction) {
		t.Fatalf("read-only verification ran native trust: %+v %v %v", verified, err, f.cli.calls)
	}
	request.VerifyOnly, request.Replacing, request.PreviousNativeObjects = false, true, request.Delivery.NativeObjects
	env.LookupExecutable = nil
	fallback, err := New().Activate(context.Background(), env, request)
	if err != nil || len(f.cli.calls) != calls || !strings.HasPrefix(fallback.UserActions[len(fallback.UserActions)-1], "UAP did not change Hermes project trust: ") {
		t.Fatalf("a composition without explicit native lookup ran or claimed trust: %v %v %v", fallback.UserActions, err, f.cli.calls)
	}
}

func TestHermesPreparedSetupAdmitsOnlyExactSkillProject(t *testing.T) {
	project := canonicalTempDir(t)
	profile := filepath.Join(canonicalTempDir(t), ".hermes")
	request := domain.ActivationRequest{
		Client: domain.DetectedClient{ConfigRoot: profile, ExecutablePath: filepath.Join(profile, "bin", "hermes")},
		Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, InstallIntent: domain.InstallIntentAutomatic, NativeRegistryRoot: filepath.Join(project, ".hermes"),
			TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: profile},
			Components:    []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "audit", Support: domain.SupportNative}}},
	}
	outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed}
	adapter := New()
	if !adapter.CanCompletePreparedSetup(request, outcome) {
		t.Fatal("exact Skills-only Project install refused")
	}
	for name, mutate := range map[string]func(*domain.ActivationRequest, *domain.ActivationOutcome){
		"user scope": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.Scope = domain.ScopeUser },
		"prepare intent": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.InstallIntent = domain.InstallIntentPrepare
		},
		"foreign registry": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.NativeRegistryRoot = filepath.Join(project, "other")
		},
		"changed profile": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.TargetContext.UserConfigRoot = project
		},
		"no executable":       func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Client.ExecutablePath = "" },
		"relative executable": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Client.ExecutablePath = "hermes" },
		"project MCP session": func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) {
			r.Plan.Components = append(r.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative})
		},
		"no components":  func(r *domain.ActivationRequest, _ *domain.ActivationOutcome) { r.Plan.Components = nil },
		"already active": func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.Activation = domain.ActivationActive },
		"not installed": func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) {
			o.Verification = domain.VerificationPackageValid
		},
		"policy restricted": func(_ *domain.ActivationRequest, o *domain.ActivationOutcome) { o.Policy = domain.PolicyBlocked },
	} {
		changed, observed := request, outcome
		changed.Plan.Components = slices.Clone(request.Plan.Components)
		mutate(&changed, &observed)
		if adapter.CanCompletePreparedSetup(changed, observed) {
			t.Fatalf("%s qualified for prepared setup completion", name)
		}
	}
}
