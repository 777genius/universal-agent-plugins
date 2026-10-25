package agentpluginscli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type preparedClaudeCLIRunner struct {
	t                         *testing.T
	project, user, executable string
	calls                     int
}

func (r *preparedClaudeCLIRunner) Run(_ context.Context, cmd legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	if !reflect.DeepEqual(cmd.Argv, []string{r.executable, "plugin", "list", "--json"}) || cmd.Dir != r.project {
		r.t.Fatalf("unexpected fixture command=%+v", cmd)
	}
	found := false
	for _, v := range cmd.Env {
		if v == "CLAUDE_CONFIG_DIR="+r.user {
			found = true
		}
	}
	if !found {
		r.t.Fatal("original user profile was replaced")
	}
	r.calls++
	entries, err := os.ReadDir(filepath.Join(r.project, ".claude", "skills"))
	if err != nil && !os.IsNotExist(err) {
		return legacyports.CommandResult{}, err
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			count++
		}
	}
	if count == 0 {
		return legacyports.CommandResult{Stdout: []byte("[]")}, nil
	}
	if count != 1 {
		return legacyports.CommandResult{}, fmt.Errorf("unexpected fixture project directories=%d", count)
	}
	return legacyports.CommandResult{Stdout: []byte(`[{"id":"(suppressed)@skills-dir","version":"unknown","scope":"project","enabled":false,"installPath":"","notes":["1 project-scope directory under ./.claude/skills/ that may load as a plugin was skipped because this workspace was not trusted when plugins were scanned. After accepting the trust dialog, run /reload-plugins (or relaunch) to load what qualifies."]}]`)}, nil
}

// The public saved-binding update command uses grouped activation even for one
// target. The runner is inert; real native failure evidence is retained separately.
func TestProjectClaudePreparedCLIGroupedUpdateReportsActualMutation(t *testing.T) {
	client := fixtureClient(t, domain.ClientClaude)
	f := newCLIFixture(t, []domain.DetectedClient{client})
	project := filepath.Join(f.root, "A")
	other := filepath.Join(f.root, "B")
	for _, p := range []string{project, other, client.ConfigRoot} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	client.ExecutablePath = filepath.Join(f.root, "inert-claude")
	if err := os.WriteFile(client.ExecutablePath, []byte("inert fixture, never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	f.app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	userState := f.store.Path
	projectForeign := filepath.Join(project, ".claude", "settings.local.json")
	for _, p := range []string{filepath.Join(client.ConfigRoot, "foreign.json"), filepath.Join(other, ".claude", "settings.local.json"), filepath.Join(other, ".uap", "state-v2.json"), userState, projectForeign} {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("preserve exact foreign bytes\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	foreign := map[string]string{}
	for _, p := range []string{client.ConfigRoot, other, userState, projectForeign} {
		for name, body := range preparedClaudeCLIFiles(t, p) {
			foreign[name] = body
		}
	}
	stateRoot := filepath.Join(project, ".uap")
	f.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	f.app.Scope, f.app.ProjectRoot = domain.ScopeProject, project
	f.app.ManagedRoot = filepath.Join(stateRoot, "managed")
	f.app.StateStore = f.store
	f.app.EnvironmentLockPath = filepath.Join(stateRoot, "environment.lock")
	planner := plannertest.NewPlanner(clientplanner.Planner{ProjectRoot: project, ManagedRoot: f.app.ManagedRoot})
	f.app.Planner, f.app.Targets = planner, planner
	runner := &preparedClaudeCLIRunner{t: t, project: project, user: client.ConfigRoot, executable: client.ExecutablePath}
	f.app.Lifecycle.ProjectRoot = project
	f.app.Lifecycle.StateStore = f.store
	f.app.Lifecycle.Planner, f.app.Lifecycle.Targets = planner, planner
	f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	f.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Runner: runner, Stager: f.app.Lifecycle.Stager})
	f.app.Lifecycle.Lock = processlock.Lock{Path: filepath.Join(stateRoot, "mutation.lock")}
	f.app.Lifecycle.Kernel = transaction.Kernel{StateStore: f.store, Directory: dirswap.Manager{JournalDir: filepath.Join(stateRoot, "operations-v2")}}
	var err error
	f.app.Installer, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: stateRoot, Registry: f.app.ClientRegistry, Runner: runner, HelperExecutable: client.ExecutablePath, EnableNativeObserver: true, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Lifecycle.CheckNamespace = f.app.Installer.CheckProjectNamespace
	pkg := writeCLIPlugin(t)
	if err := os.MkdirAll(filepath.Join(pkg, "skills", "guide"), 0700); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(pkg, "skills", "guide", "SKILL.md")
	writeSkill := func(marker string) {
		t.Helper()
		if err := os.WriteFile(skill, []byte("---\nname: guide\ndescription: Inert marker\n---\n"+marker+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill("version one")
	run := func(args ...string) map[string]any {
		t.Helper()
		args = append(args, "--target", "claude", "--scope", "project", "--project-root", project, "--format", "json")
		stdout, stderr, err := f.execute(false, args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s\n%s", args, err, stdout, stderr)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatal(err)
		}
		data := envelope["data"].(map[string]any)
		if targets, ok := data["targets"].([]any); ok {
			data = targets[0].(map[string]any)["output"].(map[string]any)
		}
		result := data["result"].(map[string]any)
		activation := result["activation"].(map[string]any)
		if activation["activation"] != "prepared" || activation["verification"] != "installation_verified" {
			t.Fatalf("project observation promoted=%+v", result)
		}
		return result
	}
	run("add", pkg, "--no-save")
	before, err := os.ReadFile(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var managed string
	for _, b := range state.Installations[0].Clients {
		managed = b.TargetLocator
	}
	treeBefore := preparedClaudeCLIFiles(t, managed)
	for _, args := range [][]string{{"add", pkg, "--no-save"}, {"update", "demo"}} {
		calls := runner.calls
		result := run(args...)
		if result["mutated"] != false || result["no_change"] != true || runner.calls <= calls {
			t.Fatalf("unchanged Prepared did not reobserve without mutation=%+v calls%d/%d", result, runner.calls, calls)
		}
		after, err := os.ReadFile(f.store.Path)
		if err != nil || string(after) != string(before) {
			t.Fatalf("unchanged state rewritten: %v", err)
		}
		if !reflect.DeepEqual(treeBefore, preparedClaudeCLIFiles(t, managed)) {
			t.Fatal("unchanged update changed owned bytes")
		}
	}
	writeSkill("version two")
	changed := run("update", "demo")
	if changed["mutated"] != true || changed["no_change"] == true || reflect.DeepEqual(treeBefore, preparedClaudeCLIFiles(t, managed)) {
		t.Fatalf("changed Prepared bytes misclassified=%+v", changed)
	}
	for path, want := range foreign {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != want {
			t.Fatalf("foreign file changed %s: %v", path, err)
		}
	}
}
func preparedClaudeCLIFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		if e.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unexpected fixture symlink %s", path)
		}
		b, err := os.ReadFile(path)
		if err == nil {
			out[path] = string(b)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return out
}
