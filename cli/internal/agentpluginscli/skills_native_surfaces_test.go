package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Only ordinary native catalogs are controlled. Package projection, registration,
// ownership receipts, activation and public rendering use the real installer.
type rawSurfaceRunner struct {
	t                            *testing.T
	client                       domain.ClientID
	project, executable, profile string
	falseCatalog                 bool
	calls                        []string
}

func (r *rawSurfaceRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return r.catalog(ctx, command)
}

func (r *rawSurfaceRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	if grace != time.Second {
		r.t.Fatalf("native catalog closure grace: %s", grace)
	}
	return r.catalog(ctx, command)
}

func (r *rawSurfaceRunner) catalog(ctx context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	if _, bounded := ctx.Deadline(); !bounded || c.Dir != r.project || c.StdoutLimitBytes <= 0 || c.StdoutLimitBytes > 1<<20 {
		r.t.Fatalf("unbounded or wrong-project native query: %+v", c)
	}
	kind := "skills"
	want := []string{r.executable, "--auto-approve", "false", "--cwd", r.project, "config", "skills", "--json"}
	if r.client == domain.ClientCopilot {
		if len(c.Argv) != 4 {
			r.t.Fatalf("unexpected Copilot command: %+v", c)
		}
		kind = c.Argv[1]
		want = []string{r.executable, kind, "list", "--json"}
		if kind != "plugin" && kind != "skill" {
			r.t.Fatalf("unexpected catalog kind %q", kind)
		}
	}
	if !reflect.DeepEqual(c.Argv, want) {
		r.t.Fatalf("native query changed: %+v", c)
	}
	r.calls = append(r.calls, kind)
	rows := []map[string]any{}
	if !r.falseCatalog {
		if r.client == domain.ClientCopilot {
			rows = r.copilotRows(kind)
		} else {
			rows = r.clineRows()
		}
	}
	body, err := json.Marshal(rows)
	return legacyports.CommandResult{Stdout: body, CleanupComplete: true}, err
}

func (r *rawSurfaceRunner) readJSON(path string, target any) {
	r.t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := json.Unmarshal(body, target); err != nil {
		r.t.Fatal(path, err)
	}
}

func (r *rawSurfaceRunner) copilotRows(kind string) []map[string]any {
	r.t.Helper()
	var settings struct {
		Marketplaces map[string]struct{ Source struct{ Source, Path string } } `json:"extraKnownMarketplaces"`
		Enabled      map[string]bool                                           `json:"enabledPlugins"`
	}
	r.readJSON(filepath.Join(r.project, ".github", "copilot", "settings.local.json"), &settings)
	if len(settings.Enabled) != 1 || len(settings.Marketplaces) != 1 {
		r.t.Fatalf("unexpected real registration: %+v", settings)
	}
	rows := []map[string]any{}
	for key, enabled := range settings.Enabled {
		name, market, ok := strings.Cut(key, "@")
		source := settings.Marketplaces[market].Source
		if !ok || !enabled || source.Source != "directory" {
			r.t.Fatalf("invalid installed registration: %q %+v", key, source)
		}
		active := source.Path
		if !filepath.IsAbs(active) {
			active = filepath.Join(r.project, filepath.FromSlash(active))
		}
		var manifest struct{ Name string }
		r.readJSON(filepath.Join(active, "plugin.json"), &manifest)
		if manifest.Name != name {
			r.t.Fatalf("registration/manifest mismatch: %q %+v", key, manifest)
		}
		if kind == "plugin" {
			var marketplace struct {
				Name    string
				Plugins []struct{ Name, Version, Source string }
			}
			r.readJSON(filepath.Join(active, ".github", "plugin", "marketplace.json"), &marketplace)
			if marketplace.Name != market || len(marketplace.Plugins) != 1 || marketplace.Plugins[0].Name != name || marketplace.Plugins[0].Source != "." {
				r.t.Fatalf("projected marketplace differs from registration: %+v", marketplace)
			}
			rows = append(rows, map[string]any{"name": name, "marketplace": market, "version": marketplace.Plugins[0].Version, "enabled": enabled, "source": "live", "installedFrom": active})
		} else {
			rows = append(rows, r.skillRows(filepath.Join(active, "skills"), false)...)
		}
	}
	return rows
}

func (r *rawSurfaceRunner) clineRows() []map[string]any {
	return r.skillRows(filepath.Join(r.project, ".cline", "skills"), true)
}

func (r *rawSurfaceRunner) skillRows(root string, clineCatalog bool) []map[string]any {
	r.t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		r.t.Fatal(err)
	}
	rows := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name(), "SKILL.md")
		body, err := os.ReadFile(path)
		if err != nil {
			r.t.Fatal(err)
		}
		// This source is owned by managementMutableWebsite. Assert the real
		// installed bytes before producing a catalog; no private receipts are made.
		if string(body) != "---\nname: pdf\ndescription: Read PDFs\n---\nFirst reviewed content.\n" {
			r.t.Fatalf("unexpected installed Skill: %s", body)
		}
		row := map[string]any{"name": entry.Name(), "description": "Read PDFs", "path": filepath.Dir(path), "source": "plugin", "enabled": true}
		if clineCatalog {
			row = map[string]any{"name": entry.Name(), "description": "Read PDFs", "path": path, "instructions": "First reviewed content.", "frontmatter": map[string]any{"name": "pdf", "description": "Read PDFs"}}
		}
		rows = append(rows, row)
	}
	if len(rows) != 1 {
		r.t.Fatalf("expected exactly one actually installed Skill: %+v", rows)
	}
	return rows
}

func rawSurfaceFixture(t *testing.T, id domain.ClientID, mode string) (savedJourney, *rawSurfaceRunner, string) {
	t.Helper()
	_, journey, source, _ := managementMutableWebsite(t)
	base, err := filepath.EvalSymlinks(journey.cli.root)
	if err != nil {
		t.Fatal(err)
	}
	project, profile := filepath.Join(base, "project"), filepath.Join(base, "native-profile")
	for _, path := range []string{project, profile, filepath.Join(base, "bin")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	dependencyWrite(t, filepath.Join(project, "foreign.txt"), "foreign original bytes")
	executable := filepath.Join(base, "bin", string(id))
	dependencyWrite(t, executable, "inert fixture; controlled native runner only")
	if err := os.Chmod(executable, 0700); err != nil {
		t.Fatal(err)
	}
	version := "1.0.105"
	if id == domain.ClientCline {
		version = "3.0.74"
	}
	surface := domain.ClientSurface{ID: string(id) + "_cli", Detected: true, Evidence: "fixture observed CLI"}
	switch mode {
	case "false-surface":
		surface.Detected = false
	case "wrong-surface":
		surface.ID = "editor"
	}
	client := domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, Version: version, ExecutablePath: executable, ConfigRoot: profile, Surfaces: []domain.ClientSurface{surface}}
	// Cline ordinary detection resolves its CLI only at the guarded session seam.
	if id == domain.ClientCline {
		client.ExecutablePath = ""
	}
	if mode == "missing-surface" {
		client.Surfaces = nil
	}
	if id == domain.ClientCopilot {
		body, err := json.Marshal(map[string]any{"trustedFolders": []string{project}, "foreign": "retained"})
		if err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(profile, "config.json"), string(body))
	}
	runner := &rawSurfaceRunner{t: t, client: id, project: project, profile: profile, executable: executable, falseCatalog: mode == "false-catalog"}
	configureRawSurfaceApp(t, &journey, client, runner)
	return journey, runner, source
}

func configureRawSurfaceApp(t *testing.T, journey *savedJourney, client domain.DetectedClient, runner *rawSurfaceRunner) {
	t.Helper()
	stateRoot := filepath.Join(runner.project, ".uap")
	journey.cli.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: runner.project}
	journey.cli.operations = filepath.Join(stateRoot, "operations-v2")
	app := &journey.cli.app
	app.Scope, app.ProjectRoot, app.ManagedRoot = domain.ScopeProject, runner.project, filepath.Join(stateRoot, "managed")
	app.DependenciesRoot, app.StateStore = runner.project, journey.cli.store
	app.EnvironmentLockPath, app.EnvironmentStoreRoot = filepath.Join(stateRoot, "environment.lock"), filepath.Join(stateRoot, "store")
	app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	var err error
	app.ClientRegistry, err = clients.NewRegistry(copilot.New(), cline.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	app.Installer, err = installer.New(installer.Config{ProjectRoot: runner.project, StateRoot: stateRoot, StateFile: journey.cli.store.Path,
		Registry: app.ClientRegistry, Runner: runner, HelperExecutable: helper, EnableNativeObserver: true,
		LookupExecutable: func(name string) (string, error) {
			if name != string(client.ClientID) {
				t.Fatalf("unexpected executable lookup %q", name)
			}
			return runner.executable, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRawSkillsPreserveObservedNativeCLISurfaces(t *testing.T) {
	for _, id := range []domain.ClientID{domain.ClientCopilot, domain.ClientCline} {
		for _, mode := range []string{"observed", "missing-surface", "false-surface", "wrong-surface", "false-catalog"} {
			t.Run(string(id)+"/"+mode, func(t *testing.T) { checkRawSurfaceAdd(t, id, mode) })
		}
	}
}

func TestPreparedRawSkillsRejectChangedCLISurface(t *testing.T) {
	for _, id := range []domain.ClientID{domain.ClientCopilot, domain.ClientCline} {
		t.Run(string(id), func(t *testing.T) {
			journey, runner, source := rawSurfaceFixture(t, id, "observed")
			plan, err := journey.cli.app.PrepareSkills(t.Context(), SkillsBatchRequest{
				Selections: []SkillsSelection{{Website: source, Name: "pdf"}}, TargetIDs: []string{string(id)},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			// The detector deliberately reuses its backing slice. A fresh host
			// observation must not mutate the already reviewed surface evidence.
			detector := journey.cli.app.Detector.(staticDetector)
			detector.clients[0].Surfaces[0].Detected = false
			if err := plan.Check(t.Context()); !errors.Is(err, ErrSkillsPlanStale) {
				t.Fatalf("changed surface passed prepared review: %v", err)
			}
			if _, err := plan.Apply(t.Context()); !errors.Is(err, ErrSkillsPlanStale) {
				t.Fatalf("changed surface reached installation: %v", err)
			}
			if len(runner.calls) != 0 || len(journey.state(t).Installations) != 0 {
				t.Fatal("stale surface plan ran native catalogs or committed installation")
			}
			for _, path := range []string{filepath.Join(runner.project, "uap.toml"), filepath.Join(runner.project, "uap.lock")} {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("stale surface plan published intent at %s: %v", path, err)
				}
			}
		})
	}
}

func checkRawSurfaceAdd(t *testing.T, id domain.ClientID, mode string) {
	t.Helper()
	journey, runner, source := rawSurfaceFixture(t, id, mode)
	profile := savedJourneyTree(t, runner.profile)
	out := journey.run(t, "skills", "add", source, "--skill", "pdf", "--target", string(id), "--scope", "project", "--project-root", runner.project, "--no-ui", "--yes", "--format", "json")
	var document map[string]any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		t.Fatal(err)
	}
	binding := savedJourneyBinding(t, journey.state(t), string(id))
	if binding.Verification != domain.VerificationInstalled {
		t.Fatalf("real installation not verified: %+v", binding)
	}
	if mode == "observed" {
		want := []string{"skills"}
		if id == domain.ClientCopilot {
			want = []string{"plugin", "skill"}
		}
		if !reflect.DeepEqual(runner.calls, want) || binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired || document["readiness"] != "ready" {
			t.Fatalf("observed surface lost across public Skills path: calls=%v binding=%+v output=%s", runner.calls, binding, out)
		}
	} else {
		if binding.Activation != domain.ActivationPrepared || document["readiness"] == "ready" {
			t.Fatalf("unqualified native evidence promoted: %+v %s", binding, out)
		}
		expected := 0
		if mode == "false-catalog" {
			expected = 1
		}
		if len(runner.calls) != expected {
			t.Fatalf("unexpected native calls without qualified surface/catalog: %v", runner.calls)
		}
	}
	after := savedJourneyTree(t, runner.profile)
	if id == domain.ClientCopilot {
		// Explicit add bootstraps the ordinary native-config advisory lock.
		// Its exact empty regular file is separate from existing profile bytes.
		lock, exists := after["config.json.agentplugins.lock"]
		if !exists || !lock.Mode.IsRegular() || lock.Body != "" {
			t.Fatalf("unexpected native-config lock: %+v", lock)
		}
		delete(after, "config.json.agentplugins.lock")
	}
	if !reflect.DeepEqual(profile, after) {
		t.Fatalf("native observation changed original user profile: %s", savedJourneyDifferences(profile, after))
	}
	if body, err := os.ReadFile(filepath.Join(runner.project, "foreign.txt")); err != nil || string(body) != "foreign original bytes" {
		t.Fatal("foreign project bytes changed", err)
	}
}
