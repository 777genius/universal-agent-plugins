package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectFilesNoRunner struct{ calls int }

func (r *projectFilesNoRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("forbidden client/MCP command: %v", command.Argv)
}

type projectFilesFixture struct {
	sequence                                int
	plan                                    domain.DeliveryPlan
	base, project, user, pkg, skill, config string
	cfg                                     Config
	req                                     Request
	engine                                  *Engine
	runner                                  *projectFilesNoRunner
}

func newProjectFilesFixture(t *testing.T, id string, mode string) *projectFilesFixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &projectFilesFixture{base: base, project: filepath.Join(base, "A"), user: filepath.Join(base, "user", "."+id), pkg: filepath.Join(base, "package"), runner: &projectFilesNoRunner{}}
	deathWrite(t, filepath.Join(f.project, "foreign.txt"), "project control")
	deathWrite(t, filepath.Join(base, "B", "keep.txt"), "other project")
	deathWrite(t, filepath.Join(f.user, "keep.txt"), "original user")
	deathWrite(t, filepath.Join(base, "user-uap", "state-v2.json"), "original user state control")
	deathWrite(t, filepath.Join(f.pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"files-observer","version":"1.0.0"}`)
	deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), deathSkill("docs", "original"))
	deathWrite(t, filepath.Join(f.pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"http://127.0.0.1:9/inert"}}}`)
	registry, err := clients.NewRegistry(cline.New(), codex.New(), vscode.NewProject())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.cfg = Config{ProjectRoot: f.project, StateRoot: filepath.Join(f.project, ".uap"), Registry: registry, TrustedLocalPackages: true, HelperExecutable: helper}
	switch mode {
	case "default":
	case "disabled-with-runner":
		f.cfg.Runner = f.runner
	case "enabled-without-runner":
		f.cfg.EnableNativeObserver = true
	default:
		t.Fatal("unknown fixture mode", mode)
	}
	f.req = Request{Scope: domain.ScopeProject, ProjectRoot: f.project, Operation: OpInstall, PackageRoot: f.pkg, ClientID: id, ClientConfigRoot: f.user, InstallationID: "00000000-0000-4000-8000-000000000074", RequiredComponents: []string{"skills", "mcp"}}
	if id == "cline" {
		f.skill = filepath.Join(f.project, ".cline", "skills", "docs", "SKILL.md")
		f.config = filepath.Join(f.project, ".uap", "clients", "cline", "mcp.json")
	} else if id == "vscode" {
		f.skill = filepath.Join(f.project, ".github", "skills", "docs", "SKILL.md")
		f.config = filepath.Join(f.project, ".vscode", "mcp.json")
	} else {
		f.skill = filepath.Join(f.project, ".agents", "skills", "docs", "SKILL.md")
		f.config = filepath.Join(f.project, ".codex", "config.toml")
	}
	f.reopen(t)
	return f
}
func (f *projectFilesFixture) reopen(t *testing.T) {
	t.Helper()
	var err error
	f.engine, err = New(f.cfg)
	if err != nil {
		t.Fatal(err)
	}
}
func (f *projectFilesFixture) execute(t *testing.T, op Operation) (Result, error) {
	t.Helper()
	req := f.req
	f.sequence++
	req.Operation, req.OperationID = op, fmt.Sprintf("project-files-%s-%s-%d", op, req.ClientID, f.sequence)
	handle, err := f.engine.Prepare(t.Context(), req)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	if len(handle.deliveries) > 0 {
		f.plan = handle.deliveries[0].plan
	} else if op != OpRemove {
		t.Fatal("missing exact prepared delivery")
	}
	return f.engine.Apply(t.Context(), handle, Decision{Confirmed: true})
}
func (f *projectFilesFixture) apply(t *testing.T, op Operation) Result {
	t.Helper()
	out, err := f.execute(t, op)
	if err != nil || out.Outcome != OutcomeCompleted && out.Outcome != OutcomeUnchanged {
		t.Fatalf("%s: %+v %v", op, out, err)
	}
	return out
}
func (f *projectFilesFixture) state(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.cfg.StateRoot, "state-v2.json"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
func (f *projectFilesFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	state, err := f.engine.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state %+v %v", state, err)
	}
	for _, b := range state.Installations[0].Clients {
		if b.ClientID == f.req.ClientID {
			return b
		}
	}
	t.Fatal("missing project binding")
	return domain.ClientBinding{}
}
func (f *projectFilesFixture) untouched(t *testing.T) map[string]map[string]deathTreeEntry {
	t.Helper()
	out := map[string]map[string]deathTreeEntry{}
	for _, root := range []string{filepath.Join(f.base, "user"), filepath.Join(f.base, "B"), filepath.Join(f.base, "user-uap")} {
		out[root] = deathTree(t, root)
	}
	return out
}
func (f *projectFilesFixture) assertUntouched(t *testing.T, want map[string]map[string]deathTreeEntry) {
	t.Helper()
	if !reflect.DeepEqual(want, f.untouched(t)) || f.runner.calls != 0 {
		t.Fatal("project operation changed original user/B or ran client/MCP")
	}
}

// The original default-SDK gap committed a managed package before discovering
// this collision. All unavailable/disabled discovery compositions must refuse
// in public Prepare/Apply before durable installation publication.
func TestProjectFileAdmissionWithoutLiveObserver(t *testing.T) {
	for _, mode := range []string{"default", "disabled-with-runner", "enabled-without-runner"} {
		for _, collision := range []string{"skill", "mcp", "malformed-mcp"} {
			t.Run(mode+"/"+collision, func(t *testing.T) {
				f := newProjectFilesFixture(t, "cline", mode)
				foreign := f.skill
				body := "foreign skill bytes"
				if collision != "skill" {
					foreign = f.config
					body = `{"mcpServers":{"docs":{"transport":{"type":"streamableHttp","url":"https://foreign.test/mcp"}}}}`
					if collision == "malformed-mcp" {
						body = `{"mcpServers":`
					}
				}
				deathWrite(t, foreign, body)
				before, other := f.state(t), f.untouched(t)
				out, err := f.execute(t, OpInstall)
				if err == nil || out.Mutated {
					t.Fatalf("foreign admission: %+v %v", out, err)
				}
				if f.state(t) != before || deathRead(t, foreign) != body {
					t.Fatal("refusal changed state or foreign component")
				}
				for _, name := range []string{"managed", "data"} {
					if _, err := os.Lstat(filepath.Join(f.cfg.StateRoot, name)); !os.IsNotExist(err) {
						t.Fatalf("refusal published %s: %v", name, err)
					}
				}
				f.assertUntouched(t, other)
			})
		}
	}
}

func TestProjectFileLifecycleWithoutLiveObserver(t *testing.T) {
	for _, id := range []string{"cline", "codex", "vscode"} {
		t.Run(id, func(t *testing.T) {
			f := newProjectFilesFixture(t, id, "disabled-with-runner")
			other := f.untouched(t)
			first := f.apply(t, OpInstall)
			if !first.Mutated {
				t.Fatal("first install did not publish")
			}
			binding := f.binding(t)
			if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
				t.Fatalf("invented native activation %+v", binding)
			}
			before := f.state(t)
			again := f.apply(t, OpInstall)
			if again.Mutated || f.state(t) != before {
				t.Fatal("unchanged replay changed durable state")
			}
			deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), deathSkill("docs", "updated"))
			f.apply(t, OpUpdate)
			if deathRead(t, f.skill) != deathSkill("docs", "updated") {
				t.Fatal("update did not project new skill")
			}
			f.reopen(t)
			for _, path := range []string{f.skill, f.config} {
				original := deathRead(t, path)
				deathWrite(t, path, "foreign replacement")
				before = f.state(t)
				out, err := f.execute(t, OpRepair)
				if err == nil || out.Mutated || before != f.state(t) || deathRead(t, path) != "foreign replacement" {
					t.Fatalf("tampered component was accepted: %+v %v", out, err)
				}
				deathWrite(t, path, original)
			}
			if err := os.RemoveAll(filepath.Dir(f.skill)); err != nil {
				t.Fatal(err)
			}
			f.apply(t, OpRepair)
			if deathRead(t, f.skill) != deathSkill("docs", "updated") {
				t.Fatal("missing skill not repaired")
			}
			binding = f.binding(t)
			managedSkill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
			deathWrite(t, managedSkill, "known managed drift")
			f.apply(t, OpRepair)
			if deathRead(t, managedSkill) != deathSkill("docs", "updated") {
				t.Fatal("known managed drift not restored")
			}
			if err := os.RemoveAll(binding.TargetLocator); err != nil {
				t.Fatal(err)
			}
			f.apply(t, OpRepair)
			if deathRead(t, managedSkill) != deathSkill("docs", "updated") {
				t.Fatal("known absent managed package not restored")
			}
			f.apply(t, OpRemove)
			if _, err := os.Lstat(filepath.Dir(f.skill)); !os.IsNotExist(err) {
				t.Fatal("owned skill survived remove", err)
			}
			f.reopen(t)
			view, err := f.engine.Inspect(t.Context())
			if err != nil || view.Recovery.Required {
				t.Fatalf("reopen %+v %v", view, err)
			}
			f.assertUntouched(t, other)
		})
	}
}

func TestProjectFileObserverGroupRepairsOnlyKnownMissingManagedPackage(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		t.Run(fmt.Sprint(foreign), func(t *testing.T) {
			f := newProjectFilesFixture(t, "cline", "default")
			f.apply(t, OpInstall)
			codexUser := filepath.Join(f.base, "user", ".codex")
			deathWrite(t, filepath.Join(codexUser, "keep.txt"), "original Codex user")
			f.req.ClientID, f.req.ClientConfigRoot = "codex", codexUser
			f.apply(t, OpInstall)
			f.req.ClientID, f.req.ClientConfigRoot = "cline", f.user
			binding := f.binding(t)
			native, config := deathRead(t, f.skill), deathRead(t, f.config)
			if err := os.RemoveAll(binding.TargetLocator); err != nil {
				t.Fatal(err)
			}
			if foreign {
				deathWrite(t, f.skill, "foreign replacement")
			}
			before, other := f.state(t), f.untouched(t)
			f.req.Targets = []ClientTarget{{ClientID: "cline", ClientConfigRoot: f.user}, {ClientID: "codex", ClientConfigRoot: codexUser}}
			out, err := f.execute(t, OpRepair)
			if foreign {
				if err == nil || out.Mutated || before != f.state(t) || deathRead(t, f.skill) != "foreign replacement" {
					t.Fatalf("group adopted foreign native file: %+v %v", out, err)
				}
				if _, statErr := os.Stat(binding.TargetLocator); !os.IsNotExist(statErr) {
					t.Fatal("foreign refusal restored managed package", statErr)
				}
			} else {
				// The file operation completed; the actual client lifecycles
				// and remaining native-loading instructions stay Prepared.
				if err != nil || out.Outcome != OutcomeCompleted || !out.Mutated || out.NoChange || len(out.ManualActions) == 0 {
					t.Fatalf("prepared group file completion: %+v %v", out, err)
				}
				for _, target := range out.Targets {
					if target.Activation != "prepared" || target.Verification != "installation_verified" {
						t.Fatalf("promoted project client: %+v", target)
					}
				}
				if deathRead(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")) != native {
					t.Fatal("known managed package was not restored")
				}
				if deathRead(t, f.skill) != native || deathRead(t, f.config) != config {
					t.Fatal("group repair changed intact native files")
				}
				if got := f.binding(t); got.Activation != domain.ActivationPrepared || got.Verification != domain.VerificationInstalled {
					t.Fatalf("group repair promoted native lifecycle %+v", got)
				}
			}
			f.assertUntouched(t, other)
		})
	}
}

func TestProjectFileObserverRetainsActualPendingAttempt(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
	f.apply(t, OpInstall)
	deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), deathSkill("docs", "updated before failed handoff"))
	sentinel := errors.New("real host handoff stopped before native activation")
	f.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
		if deathRead(t, filepath.Join(facts.TargetPath, "skills", "docs", "SKILL.md")) == deathSkill("docs", "updated before failed handoff") {
			return sentinel
		}
		return nil
	}
	f.reopen(t)
	out, err := f.execute(t, OpUpdate)
	if !errors.Is(err, sentinel) || !out.Mutated {
		t.Fatalf("did not create actual unresolved effect: %+v %v", out, err)
	}
	binding := f.binding(t)
	if binding.NativeActivationAttempt == "" {
		t.Fatal("actual effect omitted persisted attempt")
	}
	// Native files still match the last acknowledged receipts. The newly
	// committed package also has its exact new digest. Neither clears uncertainty.
	helper, _ := f.engine.helper()
	svc := f.engine.lifecycle(helper, BindingFacts{}, map[domain.ClientID]domain.DetectedClient{})
	client := domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: f.user}
	plan := f.plan
	plan.PreviousNativeObjects = binding.NativeObjects
	kernel := nativeconfig.New()
	finding, findErr := cline.New().InspectNativeRegistry(t.Context(), clients.Env{NativeConfig: kernel}, client, plan, &binding)
	if findErr != nil || finding != clients.RegistryExpected {
		t.Fatalf("fixture did not have exact expected native files: %v %v", finding, findErr)
	}
	observed, obsErr := svc.NativeObserver.ObserveNativeIdentity(t.Context(), client, plan, &binding)
	if obsErr != nil || observed.State != domain.NativeIdentityManaged || !observed.ReceiptReconciled || observed.NativeDiscoveryAttempted || observed.NativeDiscoveryReconciled {
		t.Fatalf("prepared file check fabricated native evidence: %+v %v", observed, obsErr)
	}
	before, other := f.state(t), f.untouched(t)
	native, config := deathRead(t, f.skill), deathRead(t, f.config)
	f.cfg.OnCommittedBinding = nil
	f.reopen(t)
	view, inspectErr := f.engine.Inspect(t.Context())
	if inspectErr == nil || !view.Recovery.Required || !strings.Contains(view.Recovery.Reason, "retain uncertainty") {
		t.Fatalf("lost actual uncertainty %+v %v", view, inspectErr)
	}
	recovered, recoverErr := f.engine.Recover(t.Context(), view)
	if !errors.Is(recoverErr, ErrRecoveryRequired) || recovered.Outcome != OutcomeRecovery {
		t.Fatalf("default observer guessed recovery %+v %v", recovered, recoverErr)
	}
	if f.state(t) != before || deathRead(t, f.skill) != native || deathRead(t, f.config) != config {
		t.Fatal("refused recovery changed retained ownership")
	}
	f.assertUntouched(t, other)
}

// An existing adapter seam makes the foreign edit after successful initial
// ownership admission, while the real staged projection is being produced.
type projectFilesEditDuringStage struct {
	*cline.Adapter
	after func()
}

func (a projectFilesEditDuringStage) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	objects, err := a.Adapter.Project(ctx, in)
	if err == nil {
		a.after()
	}
	return objects, err
}
func TestProjectFileRepairRefusesForeignStateWithManagedDamage(t *testing.T) {
	for _, damage := range []string{"absent", "drift"} {
		for _, component := range []string{"skill", "mcp"} {
			t.Run(damage+"/"+component, func(t *testing.T) {
				f := newProjectFilesFixture(t, "cline", "default")
				f.apply(t, OpInstall)
				binding := f.binding(t)
				if damage == "absent" {
					if err := os.RemoveAll(binding.TargetLocator); err != nil {
						t.Fatal(err)
					}
				} else {
					deathWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "managed drift")
				}
				path := f.skill
				if component == "mcp" {
					path = f.config
				}
				deathWrite(t, path, "foreign native bytes")
				before, other := f.state(t), f.untouched(t)
				out, err := f.execute(t, OpRepair)
				if err == nil || out.Mutated || f.state(t) != before || deathRead(t, path) != "foreign native bytes" {
					t.Fatalf("managed damage masked native collision %+v %v", out, err)
				}
				f.assertUntouched(t, other)
			})
		}
	}
}
func TestProjectFileRepairRechecksOwnershipAfterStaging(t *testing.T) {
	f := newProjectFilesFixture(t, "cline", "disabled-with-runner")
	f.apply(t, OpInstall)
	binding := f.binding(t)
	managedSkill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
	deathWrite(t, managedSkill, "known managed drift")
	staged := 0
	adapter := projectFilesEditDuringStage{Adapter: cline.New(), after: func() { staged++; deathWrite(t, f.skill, "foreign edit during staging") }}
	registry, err := clients.NewRegistry(adapter, codex.New())
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Registry = registry
	f.reopen(t)
	before, other := f.state(t), f.untouched(t)
	out, err := f.execute(t, OpRepair)
	if err == nil || out.Mutated || staged != 1 || f.state(t) != before || deathRead(t, managedSkill) != "known managed drift" || deathRead(t, f.skill) != "foreign edit during staging" {
		t.Fatalf("staging-time edit crossed publication: %+v %v stage=%d", out, err, staged)
	}
	f.assertUntouched(t, other)
}
