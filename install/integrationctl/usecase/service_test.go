package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/evidence"
	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/journal"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/jsonstate"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/manifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/source"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type stubResolver struct {
	resolve func(domain.IntegrationRef) (ports.ResolvedSource, error)
}

func (s stubResolver) Resolve(_ context.Context, ref domain.IntegrationRef) (ports.ResolvedSource, error) {
	return s.resolve(ref)
}

type stubWorkspaceLockStore struct {
	load func() (domain.WorkspaceLock, error)
	path string
}

func (s stubWorkspaceLockStore) Load(context.Context) (domain.WorkspaceLock, error) {
	return s.load()
}

func (s stubWorkspaceLockStore) Save(context.Context, domain.WorkspaceLock) error {
	return nil
}

func (s stubWorkspaceLockStore) Path() string {
	return s.path
}

type stubManifestLoader struct {
	load func(ports.ResolvedSource) (domain.IntegrationManifest, error)
}

func (s stubManifestLoader) Load(_ context.Context, resolved ports.ResolvedSource) (domain.IntegrationManifest, error) {
	return s.load(resolved)
}

type stubTargetAdapter struct {
	id           domain.TargetID
	inspect      func(ports.InspectInput) (ports.InspectResult, error)
	planInstall  func(ports.PlanInstallInput) (ports.AdapterPlan, error)
	applyInstall func(ports.ApplyInput) (ports.ApplyResult, error)
	planUpdate   func(ports.PlanUpdateInput) (ports.AdapterPlan, error)
	applyUpdate  func(ports.ApplyInput) (ports.ApplyResult, error)
	planRemove   func(ports.PlanRemoveInput) (ports.AdapterPlan, error)
	applyRemove  func(ports.ApplyInput) (ports.ApplyResult, error)
	planEnable   func(ports.PlanToggleInput) (ports.AdapterPlan, error)
	applyEnable  func(ports.ApplyInput) (ports.ApplyResult, error)
	planDisable  func(ports.PlanToggleInput) (ports.AdapterPlan, error)
	applyDisable func(ports.ApplyInput) (ports.ApplyResult, error)
	repair       func(ports.RepairInput) (ports.ApplyResult, error)
}

func (s stubTargetAdapter) ID() domain.TargetID { return s.id }
func (s stubTargetAdapter) Capabilities(context.Context) (ports.Capabilities, error) {
	return ports.Capabilities{EvidenceKey: "test." + string(s.id)}, nil
}
func (s stubTargetAdapter) Inspect(_ context.Context, in ports.InspectInput) (ports.InspectResult, error) {
	if s.inspect != nil {
		return s.inspect(in)
	}
	return ports.InspectResult{TargetID: s.id, State: domain.InstallRemoved}, nil
}
func (s stubTargetAdapter) PlanInstall(_ context.Context, in ports.PlanInstallInput) (ports.AdapterPlan, error) {
	if s.planInstall != nil {
		return s.planInstall(in)
	}
	return ports.AdapterPlan{TargetID: s.id, ActionClass: "install_missing", EvidenceKey: "test." + string(s.id)}, nil
}
func (s stubTargetAdapter) ApplyInstall(_ context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if s.applyInstall != nil {
		return s.applyInstall(in)
	}
	return ports.ApplyResult{TargetID: s.id, State: domain.InstallInstalled}, nil
}
func (s stubTargetAdapter) PlanUpdate(_ context.Context, in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
	if s.planUpdate != nil {
		return s.planUpdate(in)
	}
	return ports.AdapterPlan{TargetID: s.id, ActionClass: "update_version", EvidenceKey: "test." + string(s.id)}, nil
}
func (s stubTargetAdapter) ApplyUpdate(_ context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if s.applyUpdate != nil {
		return s.applyUpdate(in)
	}
	return ports.ApplyResult{TargetID: s.id, State: domain.InstallInstalled, ActivationState: domain.ActivationComplete}, nil
}
func (s stubTargetAdapter) PlanRemove(_ context.Context, in ports.PlanRemoveInput) (ports.AdapterPlan, error) {
	if s.planRemove != nil {
		return s.planRemove(in)
	}
	return ports.AdapterPlan{TargetID: s.id, ActionClass: "remove_orphaned_target", EvidenceKey: "test." + string(s.id)}, nil
}
func (s stubTargetAdapter) ApplyRemove(_ context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if s.applyRemove != nil {
		return s.applyRemove(in)
	}
	return ports.ApplyResult{TargetID: s.id, State: domain.InstallRemoved}, nil
}
func (s stubTargetAdapter) PlanEnable(_ context.Context, in ports.PlanToggleInput) (ports.AdapterPlan, error) {
	if s.planEnable != nil {
		return s.planEnable(in)
	}
	return ports.AdapterPlan{TargetID: s.id, ActionClass: "enable_target", EvidenceKey: "test." + string(s.id)}, nil
}
func (s stubTargetAdapter) ApplyEnable(_ context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if s.applyEnable != nil {
		return s.applyEnable(in)
	}
	return ports.ApplyResult{TargetID: s.id, State: domain.InstallInstalled, ActivationState: domain.ActivationRestartPending}, nil
}
func (s stubTargetAdapter) PlanDisable(_ context.Context, in ports.PlanToggleInput) (ports.AdapterPlan, error) {
	if s.planDisable != nil {
		return s.planDisable(in)
	}
	return ports.AdapterPlan{TargetID: s.id, ActionClass: "disable_target", EvidenceKey: "test." + string(s.id)}, nil
}
func (s stubTargetAdapter) ApplyDisable(_ context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if s.applyDisable != nil {
		return s.applyDisable(in)
	}
	return ports.ApplyResult{TargetID: s.id, State: domain.InstallDisabled, ActivationState: domain.ActivationRestartPending}, nil
}
func (s stubTargetAdapter) Repair(_ context.Context, in ports.RepairInput) (ports.ApplyResult, error) {
	if s.repair != nil {
		return s.repair(in)
	}
	return ports.ApplyResult{TargetID: s.id, State: domain.InstallInstalled, ActivationState: domain.ActivationComplete}, nil
}

func updateFailingAdapter(id domain.TargetID, evidenceKey string) stubTargetAdapter {
	return stubTargetAdapter{
		id: id,
		inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
			return ports.InspectResult{TargetID: id, State: domain.InstallInstalled, SourceAccessState: "ok"}, nil
		},
		planUpdate: func(in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
			return ports.AdapterPlan{TargetID: id, ActionClass: "update_version", EvidenceKey: evidenceKey}, nil
		},
		applyUpdate: func(in ports.ApplyInput) (ports.ApplyResult, error) {
			return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "forced update failure", nil)
		},
	}
}

func TestAddDryRunBuildsPlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "plugin.yaml"), "api_version: v1\nname: demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - claude\n  - cursor\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.claude.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetClaude: claude.Adapter{},
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: root, UserHome: filepath.Join(root, "home")},
		},
	}
	report, err := svc.Add(context.Background(), AddInput{
		Source: root,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("add dry-run: %v", err)
	}
	if len(report.Targets) != 2 {
		t.Fatalf("target count = %d, want 2", len(report.Targets))
	}
	if report.OperationID == "" {
		t.Fatal("expected operation id")
	}
}

func TestDisableNonDryRunUsesToggleAdapterAndPersistsDisabledState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.gemini","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	record := domain.InstallationRecord{
		IntegrationID:      "gemini-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
		ResolvedVersion:    "0.1.0",
		SourceDigest:       "sha256:test",
		ManifestDigest:     "sha256:test-manifest",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetGemini: {TargetID: domain.TargetGemini, DeliveryKind: domain.DeliveryGeminiExtension, State: domain.InstallInstalled},
		},
	}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	inspectCalls := 0
	svc := Service{
		StateStore:  store,
		LockManager: locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:     journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:    evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetGemini: stubTargetAdapter{
				id: domain.TargetGemini,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					inspectCalls++
					if inspectCalls == 1 {
						return ports.InspectResult{TargetID: domain.TargetGemini, State: domain.InstallInstalled}, nil
					}
					return ports.InspectResult{TargetID: domain.TargetGemini, State: domain.InstallDisabled, ActivationState: domain.ActivationRestartPending}, nil
				},
				planDisable: func(in ports.PlanToggleInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetGemini, ActionClass: "disable_target", EvidenceKey: "test.gemini"}, nil
				},
				applyDisable: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetGemini, State: domain.InstallDisabled, ActivationState: domain.ActivationRestartPending}, nil
				},
			},
		},
	}

	report, err := svc.Disable(context.Background(), NamedDryRunInput{Name: "gemini-demo", DryRun: false})
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallDisabled) {
		t.Fatalf("report = %+v", report)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if got := state.Installations[0].Targets[domain.TargetGemini].State; got != domain.InstallDisabled {
		t.Fatalf("state = %s, want disabled", got)
	}
}

func TestUpdateAllDryRunAggregatesManagedIntegrations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.cursor","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"test.opencode","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	writeFile(t, filepath.Join(root, "cursor-demo", "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(root, "opencode-demo", "plugin", "plugin.yaml"), "api_version: v1\nname: opencode-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - opencode\n")
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	if err := store.Save(context.Background(), ports.StateFile{
		SchemaVersion: 1,
		Installations: []domain.InstallationRecord{
			{
				IntegrationID:      "cursor-demo",
				RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "cursor-demo")},
				ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "cursor-demo")},
				ResolvedVersion:    "0.1.0",
				Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
				Targets: map[domain.TargetID]domain.TargetInstallation{
					domain.TargetCursor: {TargetID: domain.TargetCursor, DeliveryKind: domain.DeliveryCursorMCP, State: domain.InstallInstalled},
				},
			},
			{
				IntegrationID:      "opencode-demo",
				RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "opencode-demo")},
				ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "opencode-demo")},
				ResolvedVersion:    "0.1.0",
				Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
				Targets: map[domain.TargetID]domain.TargetInstallation{
					domain.TargetOpenCode: {TargetID: domain.TargetOpenCode, DeliveryKind: domain.DeliveryOpenCodePlugin, State: domain.InstallInstalled},
				},
			},
		},
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	svc := Service{
		StateStore:  store,
		LockManager: locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:     journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:    evidence.Registry{FS: fs, Path: evidencePath},
		SourceResolver: stubResolver{resolve: func(ref domain.IntegrationRef) (ports.ResolvedSource, error) {
			return ports.ResolvedSource{LocalPath: ref.Raw}, nil
		}},
		ManifestLoader: manifest.Loader{},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: stubTargetAdapter{
				id: domain.TargetCursor,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, SourceAccessState: "ok"}, nil
				},
				planUpdate: func(in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "update_version", EvidenceKey: "test.cursor"}, nil
				},
			},
			domain.TargetOpenCode: stubTargetAdapter{
				id: domain.TargetOpenCode,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetOpenCode, State: domain.InstallInstalled, SourceAccessState: "ok"}, nil
				},
				planUpdate: func(in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetOpenCode, ActionClass: "update_version", EvidenceKey: "test.opencode"}, nil
				},
			},
		},
	}

	report, err := svc.UpdateAll(context.Background(), true)
	if err != nil {
		t.Fatalf("update all dry-run: %v", err)
	}
	if len(report.Targets) != 2 {
		t.Fatalf("target count = %d, want 2", len(report.Targets))
	}
	if report.OperationID == "" {
		t.Fatal("expected batch operation id")
	}
}

func TestAddNonDryRunInstallsCursorAndPersistsState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver:       source.Resolver{},
		ManifestLoader:       manifest.Loader{},
		StateStore:           jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:          locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:             evidence.Registry{FS: fs, Path: evidencePath},
		CurrentWorkspaceRoot: projectRoot,
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}

	report, err := svc.Add(context.Background(), AddInput{
		Source: sourceRoot,
		Scope:  "project",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report targets = %+v", report.Targets)
	}

	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("installations = %d, want 1", len(state.Installations))
	}
	record := state.Installations[0]
	target := record.Targets[domain.TargetCursor]
	if target.State != domain.InstallInstalled {
		t.Fatalf("target state = %s, want installed", target.State)
	}
	if len(target.OwnedNativeObjects) == 0 {
		t.Fatal("expected owned native objects")
	}
	configPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("stat cursor config: %v", err)
	}

	openOps, err := svc.Journal.ListOpen(context.Background())
	if err != nil {
		t.Fatalf("list open ops: %v", err)
	}
	if len(openOps) != 0 {
		t.Fatalf("open operations = %d, want 0", len(openOps))
	}
}

func TestSyncNonDryRunInstallsFromWorkspaceLockRelativeSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	sourceRoot := filepath.Join(workspace, "plugins", "cursor-demo")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n")
	writeFile(t, filepath.Join(workspace, ".uap-integrations.lock"), "api_version: v1\nintegrations:\n  - source: ./plugins/cursor-demo\n    targets:\n      - cursor\n    policy:\n      scope: project\n      auto_update: true\n      adopt_new_targets: manual\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		WorkspaceLock: stubWorkspaceLockStore{path: filepath.Join(workspace, ".uap-integrations.lock"), load: func() (domain.WorkspaceLock, error) {
			return domain.WorkspaceLock{APIVersion: "v1", Integrations: []domain.WorkspaceLockIntegration{{Source: "./plugins/cursor-demo", Targets: []string{"cursor"}, Policy: domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"}}}}, nil
		}},
		LockManager: locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:     journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:    evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: workspace, UserHome: filepath.Join(root, "home")},
		},
	}

	report, err := svc.Sync(context.Background(), false)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("sync report = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("installations = %d, want 1", len(state.Installations))
	}
	if _, err := os.Stat(filepath.Join(workspace, ".cursor", "mcp.json")); err != nil {
		t.Fatalf("stat cursor config: %v", err)
	}
}

func TestAddNonDryRunAllowsMaterializedGitSourceAndCleansTempRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "materialized", "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	materializedRoot := filepath.Join(root, "materialized")
	svc := Service{
		SourceResolver: stubResolver{resolve: func(ref domain.IntegrationRef) (ports.ResolvedSource, error) {
			return ports.ResolvedSource{
				Kind:        "git_url",
				Requested:   domain.RequestedSourceRef{Kind: "git_url", Value: ref.Raw},
				Resolved:    domain.ResolvedSourceRef{Kind: "git_commit", Value: ref.Raw + "@abc123"},
				LocalPath:   sourceRoot,
				Cleanup:     func() error { return os.RemoveAll(materializedRoot) },
				ImportRoots: []string{materializedRoot},
			}, nil
		}},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}

	report, err := svc.Add(context.Background(), AddInput{
		Source: "https://example.com/acme/demo.git",
		Scope:  "project",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	if _, err := os.Stat(materializedRoot); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup path to be removed, err=%v", err)
	}
}

func TestAddNonDryRunInstallsMultipleTargetsAndPersistsUnifiedState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  context7:\n    type: stdio\n    stdio:\n      command: npx\n      args:\n        - -y\n        - '@upstash/context7-mcp'\n    targets:\n      - cursor\n      - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "export const ExamplePlugin = async () => ({})\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver:       source.Resolver{},
		ManifestLoader:       manifest.Loader{},
		StateStore:           jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:          locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:             evidence.Registry{FS: fs, Path: evidencePath},
		CurrentWorkspaceRoot: projectRoot,
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor:   cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}

	report, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false})
	if err != nil {
		t.Fatalf("add multi-target: %v", err)
	}
	if len(report.Targets) != 2 {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Targets) != 2 {
		t.Fatalf("state installations = %+v", state.Installations)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".cursor", "mcp.json")); err != nil {
		t.Fatalf("stat cursor config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "opencode.json")); err != nil {
		t.Fatalf("stat opencode config: %v", err)
	}
}

func TestAddNonDryRunRollsBackAppliedTargetsWhenLaterTargetFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n  - claude\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n    targets:\n      - cursor\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"test.claude","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
			domain.TargetClaude: stubTargetAdapter{
				id: domain.TargetClaude,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetClaude, State: domain.InstallRemoved}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetClaude, ActionClass: "install_missing", EvidenceKey: "test.claude"}, nil
				},
				applyInstall: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "forced failure", nil)
				},
			},
		},
	}

	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err == nil {
		t.Fatal("expected add to fail")
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("expected no persisted installation after successful rollback, got %+v", state.Installations)
	}
	configPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	if body, err := os.ReadFile(configPath); err == nil && strings.Contains(string(body), "release-checks") {
		t.Fatalf("managed cursor entry still present after rollback:\n%s", body)
	}
}

func TestUpdateNonDryRunRefreshesCursorManagedEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/v1.mjs\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver:       source.Resolver{},
		ManifestLoader:       manifest.Loader{},
		StateStore:           jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:          locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:             evidence.Registry{FS: fs, Path: evidencePath},
		CurrentWorkspaceRoot: projectRoot,
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/v2.mjs\n")
	report, err := svc.Update(context.Background(), NamedDryRunInput{Name: "cursor-demo", DryRun: false})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if got := state.Installations[0].ResolvedVersion; got != "0.2.0" {
		t.Fatalf("resolved version = %s, want 0.2.0", got)
	}
	configBody, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if !strings.Contains(string(configBody), "/bin/v2.mjs") {
		t.Fatalf("cursor config did not update managed entry:\n%s", configBody)
	}
}

func TestUpdateNonDryRunRefreshesMultipleTargetsAndPersistsUnifiedState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/v1.mjs\n    targets:\n      - cursor\n      - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "v1\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver:       source.Resolver{},
		ManifestLoader:       manifest.Loader{},
		StateStore:           jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:          locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:             evidence.Registry{FS: fs, Path: evidencePath},
		CurrentWorkspaceRoot: projectRoot,
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor:   cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/v2.mjs\n    targets:\n      - cursor\n      - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-next-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "v2\n")
	report, err := svc.Update(context.Background(), NamedDryRunInput{Name: "multi-demo", DryRun: false})
	if err != nil {
		t.Fatalf("update multi-target: %v", err)
	}
	if len(report.Targets) != 2 {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if got := state.Installations[0].ResolvedVersion; got != "0.2.0" {
		t.Fatalf("resolved version = %s, want 0.2.0", got)
	}
	cursorConfig, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if !strings.Contains(string(cursorConfig), "/bin/v2.mjs") {
		t.Fatalf("cursor config did not update managed entry:\n%s", cursorConfig)
	}
	opencodeConfig, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	if !strings.Contains(string(opencodeConfig), "@acme/opencode-next-plugin") || strings.Contains(string(opencodeConfig), "@acme/opencode-demo-plugin") {
		t.Fatalf("OpenCode config did not update managed entry:\n%s", opencodeConfig)
	}
}

func TestUpdateDryRunCleansMaterializedResolvedSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	localSourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(localSourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(localSourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/v1.mjs\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver:       source.Resolver{},
		ManifestLoader:       manifest.Loader{},
		StateStore:           jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:          locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:             evidence.Registry{FS: fs, Path: evidencePath},
		CurrentWorkspaceRoot: projectRoot,
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: localSourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	materializedRoot := filepath.Join(root, "materialized")
	materializedPlugin := filepath.Join(materializedRoot, "plugin")
	writeFile(t, filepath.Join(materializedPlugin, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(materializedPlugin, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/v2.mjs\n")
	svc.SourceResolver = stubResolver{resolve: func(ref domain.IntegrationRef) (ports.ResolvedSource, error) {
		return ports.ResolvedSource{
			Kind:        "git_url",
			Requested:   domain.RequestedSourceRef{Kind: "git_url", Value: ref.Raw},
			Resolved:    domain.ResolvedSourceRef{Kind: "git_commit", Value: ref.Raw + "@def456"},
			LocalPath:   materializedPlugin,
			Cleanup:     func() error { return os.RemoveAll(materializedRoot) },
			ImportRoots: []string{materializedRoot},
		}, nil
	}}

	report, err := svc.Update(context.Background(), NamedDryRunInput{Name: "cursor-demo", DryRun: true})
	if err != nil {
		t.Fatalf("update dry-run: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].ActionClass != "update_version" {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	if _, err := os.Stat(materializedRoot); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup path to be removed after dry-run, err=%v", err)
	}
}

func TestUpdateNonDryRunPersistsPartialProgressWhenLaterTargetFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/v2.mjs\n    targets:\n      - cursor\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"test.opencode","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	statePath := filepath.Join(root, "state.json")
	store := jsonstate.Store{FS: fs, Path: statePath}
	writeFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"release-checks":{"command":"node","args":["/bin/v1.mjs"]}}}`)
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"plugin":["@acme/opencode-demo-plugin"]}`)
	record := domain.InstallationRecord{
		IntegrationID:      "multi-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedVersion:    "0.1.0",
		SourceDigest:       "sha256:old",
		ManifestDigest:     "sha256:old-manifest",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				TargetID:        domain.TargetCursor,
				DeliveryKind:    domain.DeliveryCursorMCP,
				State:           domain.InstallInstalled,
				NativeRef:       filepath.Join(projectRoot, ".cursor", "mcp.json"),
				ActivationState: domain.ActivationNotRequired,
			},
			domain.TargetOpenCode: {
				TargetID:        domain.TargetOpenCode,
				DeliveryKind:    domain.DeliveryOpenCodePlugin,
				State:           domain.InstallInstalled,
				NativeRef:       filepath.Join(projectRoot, "opencode.json"),
				ActivationState: domain.ActivationRestartPending,
			},
		},
	}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     store,
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor:   cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	svc.Adapters[domain.TargetOpenCode] = updateFailingAdapter(domain.TargetOpenCode, "test.opencode")

	if _, err := svc.Update(context.Background(), NamedDryRunInput{Name: "multi-demo", DryRun: false}); err == nil {
		t.Fatal("expected update to fail")
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	got := state.Installations[0]
	if got.ResolvedVersion != "0.2.0" {
		t.Fatalf("resolved version = %s, want 0.2.0", got.ResolvedVersion)
	}
	if got.Targets[domain.TargetCursor].State != domain.InstallInstalled {
		t.Fatalf("cursor state = %s, want installed", got.Targets[domain.TargetCursor].State)
	}
	if got.Targets[domain.TargetOpenCode].State != domain.InstallDegraded {
		t.Fatalf("opencode state = %s, want degraded", got.Targets[domain.TargetOpenCode].State)
	}
	cursorConfig, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if !strings.Contains(string(cursorConfig), "/bin/v2.mjs") {
		t.Fatalf("cursor config did not preserve successful partial update:\n%s", cursorConfig)
	}
}

func TestRemoveNonDryRunDeletesRecordAndManagedCursorEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]}}}`)
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	report, err := svc.Remove(context.Background(), NamedDryRunInput{Name: "cursor-demo", DryRun: false})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallRemoved) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("installations = %d, want 0", len(state.Installations))
	}
	configBody, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if strings.Contains(string(configBody), "release-checks") {
		t.Fatalf("managed entry still present after remove:\n%s", configBody)
	}
	if !strings.Contains(string(configBody), "user-owned") {
		t.Fatalf("unmanaged entry was removed unexpectedly:\n%s", configBody)
	}
}

func TestRemoveNonDryRunRemovesMultipleTargetsAndClearsUnifiedState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  context7:\n    type: stdio\n    stdio:\n      command: npx\n      args:\n        - -y\n        - '@upstash/context7-mcp'\n    targets:\n      - cursor\n      - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "export const ExamplePlugin = async () => ({})\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]}}}`)
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"theme":"midnight","plugin":["@user/existing"]}`)
	svc := Service{
		SourceResolver:       source.Resolver{},
		ManifestLoader:       manifest.Loader{},
		StateStore:           jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:          locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:             evidence.Registry{FS: fs, Path: evidencePath},
		CurrentWorkspaceRoot: projectRoot,
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor:   cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	report, err := svc.Remove(context.Background(), NamedDryRunInput{Name: "multi-demo", DryRun: false})
	if err != nil {
		t.Fatalf("remove multi-target: %v", err)
	}
	if len(report.Targets) != 2 {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("installations = %d, want 0", len(state.Installations))
	}
	cursorConfig, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if strings.Contains(string(cursorConfig), "context7") || !strings.Contains(string(cursorConfig), "user-owned") {
		t.Fatalf("unexpected cursor config after remove:\n%s", cursorConfig)
	}
	opencodeConfig, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	if strings.Contains(string(opencodeConfig), "@acme/opencode-demo-plugin") || !strings.Contains(string(opencodeConfig), "@user/existing") {
		t.Fatalf("unexpected OpenCode config after remove:\n%s", opencodeConfig)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "plugins", "example.js")); !os.IsNotExist(err) {
		t.Fatalf("owned OpenCode plugin file still exists: %v", err)
	}
}

func TestRemoveNonDryRunRollsBackEarlierTargetsWhenLaterTargetFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n  - claude\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n    targets:\n      - cursor\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"test.claude","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
			domain.TargetClaude: stubTargetAdapter{
				id: domain.TargetClaude,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetClaude, State: domain.InstallInstalled}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetClaude, ActionClass: "install_missing", EvidenceKey: "test.claude"}, nil
				},
				applyInstall: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetClaude, State: domain.InstallInstalled}, nil
				},
				planRemove: func(in ports.PlanRemoveInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetClaude, ActionClass: "remove_orphaned_target", EvidenceKey: "test.claude"}, nil
				},
				applyRemove: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "forced remove failure", nil)
				},
			},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	if _, err := svc.Remove(context.Background(), NamedDryRunInput{Name: "multi-demo", DryRun: false}); err == nil {
		t.Fatal("expected remove to fail")
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Targets) != 2 {
		t.Fatalf("expected original installation to remain after rollback, got %+v", state.Installations)
	}
	cursorConfig, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if !strings.Contains(string(cursorConfig), "release-checks") {
		t.Fatalf("cursor managed entry was not restored after rollback:\n%s", cursorConfig)
	}
}

func TestRemoveNonDryRunPersistsDegradedStateWhenRollbackFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: degraded-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.cursor","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]},{"key":"test.opencode","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	statePath := filepath.Join(root, "state.json")
	store := jsonstate.Store{FS: fs, Path: statePath}
	record := domain.InstallationRecord{
		IntegrationID:      "degraded-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedVersion:    "0.1.0",
		SourceDigest:       "sha256:test",
		ManifestDigest:     "sha256:test-manifest",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor:   {TargetID: domain.TargetCursor, DeliveryKind: domain.DeliveryCursorMCP, State: domain.InstallInstalled},
			domain.TargetOpenCode: {TargetID: domain.TargetOpenCode, DeliveryKind: domain.DeliveryOpenCodePlugin, State: domain.InstallInstalled},
		},
	}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     store,
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: stubTargetAdapter{
				id: domain.TargetCursor,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "install_missing", EvidenceKey: "test.cursor"}, nil
				},
				applyInstall: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "forced rollback failure", nil)
				},
				planRemove: func(in ports.PlanRemoveInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "remove_orphaned_target", EvidenceKey: "test.cursor"}, nil
				},
				applyRemove: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetCursor, State: domain.InstallRemoved}, nil
				},
			},
			domain.TargetOpenCode: stubTargetAdapter{
				id: domain.TargetOpenCode,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetOpenCode, State: domain.InstallInstalled}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetOpenCode, ActionClass: "install_missing", EvidenceKey: "test.opencode"}, nil
				},
				planRemove: func(in ports.PlanRemoveInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetOpenCode, ActionClass: "remove_orphaned_target", EvidenceKey: "test.opencode"}, nil
				},
				applyRemove: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "forced remove failure", nil)
				},
			},
		},
	}

	if _, err := svc.Remove(context.Background(), NamedDryRunInput{Name: "degraded-demo", DryRun: false}); err == nil {
		t.Fatal("expected remove to fail")
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("installations = %+v", state.Installations)
	}
	got := state.Installations[0]
	if got.Targets[domain.TargetCursor].State != domain.InstallDegraded {
		t.Fatalf("cursor state = %s, want degraded", got.Targets[domain.TargetCursor].State)
	}
	if got.Targets[domain.TargetOpenCode].State != domain.InstallDegraded {
		t.Fatalf("opencode state = %s, want degraded", got.Targets[domain.TargetOpenCode].State)
	}
}

func TestRepairNonDryRunRestoresManagedCursorEntry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: cursor-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]}}}`)
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	writeFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]}}}`)
	report, err := svc.Repair(context.Background(), NamedDryRunInput{Name: "cursor-demo", DryRun: false})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	configBody, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if !strings.Contains(string(configBody), "release-checks") {
		t.Fatalf("managed entry was not restored:\n%s", configBody)
	}
	if !strings.Contains(string(configBody), "user-owned") {
		t.Fatalf("unmanaged entry was removed unexpectedly:\n%s", configBody)
	}
}

func TestRepairNonDryRunRepairsMultipleTargetsAndPreservesUnifiedState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: multi-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  context7:\n    type: stdio\n    stdio:\n      command: npx\n      args:\n        - -y\n        - '@upstash/context7-mcp'\n    targets:\n      - cursor\n      - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "export const ExamplePlugin = async () => ({})\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.cursor.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]}}}`)
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"theme":"midnight","plugin":["@user/existing"]}`)
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor:   cursor.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	writeFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]}}}`)
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"theme":"midnight","plugin":["@user/existing"]}`)
	report, err := svc.Repair(context.Background(), NamedDryRunInput{Name: "multi-demo", DryRun: false})
	if err != nil {
		t.Fatalf("repair multi-target: %v", err)
	}
	if len(report.Targets) != 2 {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Targets) != 2 {
		t.Fatalf("unexpected state: %+v", state.Installations)
	}
	cursorConfig, err := os.ReadFile(filepath.Join(projectRoot, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("read cursor config: %v", err)
	}
	if !strings.Contains(string(cursorConfig), "context7") || !strings.Contains(string(cursorConfig), "user-owned") {
		t.Fatalf("cursor config missing repaired state:\n%s", cursorConfig)
	}
	opencodeConfig, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	if !strings.Contains(string(opencodeConfig), "@acme/opencode-demo-plugin") || !strings.Contains(string(opencodeConfig), "@user/existing") {
		t.Fatalf("OpenCode config missing repaired state:\n%s", opencodeConfig)
	}
}

func TestRepairNonDryRunPersistsPartialProgressWhenLaterTargetFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: repair-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.cursor","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]},{"key":"test.opencode","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	statePath := filepath.Join(root, "state.json")
	store := jsonstate.Store{FS: fs, Path: statePath}
	record := domain.InstallationRecord{
		IntegrationID:      "repair-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedVersion:    "0.1.0",
		SourceDigest:       "sha256:old",
		ManifestDigest:     "sha256:old-manifest",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor:   {TargetID: domain.TargetCursor, DeliveryKind: domain.DeliveryCursorMCP, State: domain.InstallDegraded, NativeRef: "cursor.json"},
			domain.TargetOpenCode: {TargetID: domain.TargetOpenCode, DeliveryKind: domain.DeliveryOpenCodePlugin, State: domain.InstallDegraded, NativeRef: "opencode.json"},
		},
	}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     store,
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: stubTargetAdapter{
				id: domain.TargetCursor,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					if in.Record != nil {
						if target, ok := in.Record.Targets[domain.TargetCursor]; ok {
							for _, obj := range target.OwnedNativeObjects {
								if obj.Kind == "config_file" && obj.Path == "cursor.json" {
									return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, ActivationState: domain.ActivationComplete, SourceAccessState: "ok"}, nil
								}
							}
						}
					}
					return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallDegraded, SourceAccessState: "ok"}, nil
				},
				repair: func(in ports.RepairInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{
						TargetID:           domain.TargetCursor,
						State:              domain.InstallInstalled,
						ActivationState:    domain.ActivationComplete,
						OwnedNativeObjects: []domain.NativeObjectRef{{Kind: "config_file", Path: "cursor.json"}},
						AdapterMetadata:    map[string]any{"repaired": true},
					}, nil
				},
			},
			domain.TargetOpenCode: stubTargetAdapter{
				id: domain.TargetOpenCode,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetOpenCode, State: domain.InstallDegraded, SourceAccessState: "ok"}, nil
				},
				repair: func(in ports.RepairInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "forced repair failure", nil)
				},
			},
		},
	}

	if _, err := svc.Repair(context.Background(), NamedDryRunInput{Name: "repair-demo", DryRun: false}); err == nil {
		t.Fatal("expected repair to fail")
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	got := state.Installations[0]
	if got.ResolvedVersion != "0.2.0" {
		t.Fatalf("resolved version = %s, want 0.2.0", got.ResolvedVersion)
	}
	if got.Targets[domain.TargetCursor].State != domain.InstallInstalled {
		t.Fatalf("cursor state = %s, want installed", got.Targets[domain.TargetCursor].State)
	}
	if got.Targets[domain.TargetOpenCode].State != domain.InstallDegraded {
		t.Fatalf("opencode state = %s, want degraded", got.Targets[domain.TargetOpenCode].State)
	}
}

func TestAddNonDryRunInstallsOpenCodeAndPersistsState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: opencode-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  context7:\n    type: stdio\n    stdio:\n      command: npx\n      args:\n        - -y\n        - '@upstash/context7-mcp'\n    targets:\n      - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "export const ExamplePlugin = async () => ({})\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.json"), "{\n  \"name\": \"demo\",\n  \"private\": true\n}\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}

	report, err := svc.Add(context.Background(), AddInput{
		Source: sourceRoot,
		Scope:  "project",
		DryRun: false,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("installations = %d, want 1", len(state.Installations))
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "opencode.json")); err != nil {
		t.Fatalf("stat opencode.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "plugins", "example.js")); err != nil {
		t.Fatalf("stat projected plugin: %v", err)
	}
}

func TestUpdateNonDryRunRefreshesOpenCodeManagedEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: opencode-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  context7:\n    type: stdio\n    stdio:\n      command: npx\n      args:\n        - -y\n        - '@upstash/context7-mcp'\n    targets:\n      - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "v1\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"theme":"midnight","plugin":["@user/existing"]}`)
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: opencode-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-next-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "v2\n")
	report, err := svc.Update(context.Background(), NamedDryRunInput{Name: "opencode-demo", DryRun: false})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if got := state.Installations[0].ResolvedVersion; got != "0.2.0" {
		t.Fatalf("resolved version = %s, want 0.2.0", got)
	}
	configBody, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	if !strings.Contains(string(configBody), "@acme/opencode-next-plugin") || !strings.Contains(string(configBody), "@user/existing") {
		t.Fatalf("OpenCode config missing expected plugin refs:\n%s", configBody)
	}
	if strings.Contains(string(configBody), "@acme/opencode-demo-plugin") {
		t.Fatalf("OpenCode config still contains stale owned plugin ref:\n%s", configBody)
	}
}

func TestRemoveNonDryRunDeletesOpenCodeRecordAndOwnedFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: opencode-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "v1\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"theme":"midnight","plugin":["@user/existing"]}`)
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	report, err := svc.Remove(context.Background(), NamedDryRunInput{Name: "opencode-demo", DryRun: false})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallRemoved) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("installations = %d, want 0", len(state.Installations))
	}
	configBody, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	if !strings.Contains(string(configBody), "@user/existing") || strings.Contains(string(configBody), "@acme/opencode-demo-plugin") {
		t.Fatalf("unexpected opencode config after remove:\n%s", configBody)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "plugins", "example.js")); !os.IsNotExist(err) {
		t.Fatalf("owned OpenCode plugin file still exists: %v", err)
	}
}

func TestRepairNonDryRunRestoresOpenCodeManagedEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: opencode-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - opencode\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "v1\n")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "permission.json"), "{\"bash\":\"ask\"}\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"target.opencode.native_surface","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	projectRoot := filepath.Join(root, "workspace")
	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"theme":"midnight","plugin":["@user/existing"]}`)
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")},
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetOpenCode: opencode.Adapter{FS: fs, ProjectRoot: projectRoot, UserHome: filepath.Join(root, "home")},
		},
	}
	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err != nil {
		t.Fatalf("add: %v", err)
	}

	writeFile(t, filepath.Join(projectRoot, "opencode.json"), `{"theme":"midnight","plugin":["@user/existing"]}`)
	report, err := svc.Repair(context.Background(), NamedDryRunInput{Name: "opencode-demo", DryRun: false})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	configBody, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read opencode config: %v", err)
	}
	if !strings.Contains(string(configBody), "@acme/opencode-demo-plugin") || !strings.Contains(string(configBody), "\"permission\"") || !strings.Contains(string(configBody), "@user/existing") {
		t.Fatalf("OpenCode config missing repaired managed entries:\n%s", configBody)
	}
}

func TestUpdateNonDryRunAdoptsNewTargetWhenPolicyAuto(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: adoption-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n  - opencode\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.cursor","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]},{"key":"test.opencode","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	record := domain.InstallationRecord{
		IntegrationID:      "adoption-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedVersion:    "0.1.0",
		SourceDigest:       "sha256:old",
		ManifestDigest:     "sha256:old-manifest",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "auto"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				TargetID:          domain.TargetCursor,
				DeliveryKind:      domain.DeliveryCursorMCP,
				CapabilitySurface: []string{"mcp"},
				State:             domain.InstallInstalled,
			},
		},
	}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	var opencodeInstalled bool
	opencodeInspectCalls := 0
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     store,
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: stubTargetAdapter{
				id: domain.TargetCursor,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, SourceAccessState: "ok"}, nil
				},
				planUpdate: func(in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "update_version", EvidenceKey: "test.cursor"}, nil
				},
				applyUpdate: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, ActivationState: domain.ActivationComplete}, nil
				},
			},
			domain.TargetOpenCode: stubTargetAdapter{
				id: domain.TargetOpenCode,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					opencodeInspectCalls++
					if in.IntegrationID != "adoption-demo" {
						t.Fatalf("inspect integration_id = %q, want adoption-demo", in.IntegrationID)
					}
					if opencodeInspectCalls == 1 {
						return ports.InspectResult{TargetID: domain.TargetOpenCode, State: domain.InstallRemoved, SourceAccessState: "ok"}, nil
					}
					return ports.InspectResult{TargetID: domain.TargetOpenCode, State: domain.InstallInstalled, SourceAccessState: "ok"}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetOpenCode, ActionClass: "install_missing", EvidenceKey: "test.opencode"}, nil
				},
				applyInstall: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					opencodeInstalled = true
					return ports.ApplyResult{TargetID: domain.TargetOpenCode, State: domain.InstallInstalled, ActivationState: domain.ActivationComplete}, nil
				},
			},
		},
	}

	report, err := svc.Update(context.Background(), NamedDryRunInput{Name: "adoption-demo", DryRun: false})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !opencodeInstalled {
		t.Fatal("expected adopted target to use ApplyInstall")
	}
	if len(report.Targets) != 2 {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	var adopted domain.TargetReport
	for _, target := range report.Targets {
		if target.TargetID == string(domain.TargetOpenCode) {
			adopted = target
			break
		}
	}
	if adopted.ActionClass != "adopt_new_target" || adopted.State != string(domain.InstallInstalled) {
		t.Fatalf("adopted target report = %+v", adopted)
	}
	if len(adopted.CapabilitySurface) == 0 {
		t.Fatalf("expected capability surface in adopted target report: %+v", adopted)
	}
	state, err := svc.StateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	got := state.Installations[0]
	if got.ResolvedVersion != "0.2.0" {
		t.Fatalf("resolved version = %s, want 0.2.0", got.ResolvedVersion)
	}
	if got.Targets[domain.TargetOpenCode].State != domain.InstallInstalled {
		t.Fatalf("opencode state = %s, want installed", got.Targets[domain.TargetOpenCode].State)
	}
	if len(got.Targets[domain.TargetOpenCode].CapabilitySurface) == 0 {
		t.Fatalf("expected persisted capability surface for adopted target: %+v", got.Targets[domain.TargetOpenCode])
	}
}

func TestDoctorReportsRecoveryWarningsAndAttentionTargets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	record := domain.InstallationRecord{
		IntegrationID:      "doctor-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
		ResolvedVersion:    "0.1.0",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				TargetID:                domain.TargetCursor,
				DeliveryKind:            domain.DeliveryCursorMCP,
				CapabilitySurface:       []string{"mcp"},
				State:                   domain.InstallDegraded,
				ActivationState:         domain.ActivationComplete,
				EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{domain.RestrictionRestartRequired},
			},
			domain.TargetCodex: {
				TargetID:                domain.TargetCodex,
				DeliveryKind:            domain.DeliveryCodexMarketplace,
				CapabilitySurface:       []string{"plugin_bundle"},
				State:                   domain.InstallActivationPending,
				ActivationState:         domain.ActivationNewThreadPending,
				CatalogPolicy:           &domain.CatalogPolicySnapshot{Installation: "manual", Authentication: "oauth"},
				EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{domain.RestrictionNewThreadRequired},
			},
			domain.TargetGemini: {
				TargetID:             domain.TargetGemini,
				DeliveryKind:         domain.DeliveryGeminiExtension,
				CapabilitySurface:    []string{"contexts", "mcp"},
				State:                domain.InstallAuthPending,
				ActivationState:      domain.ActivationNativePending,
				InteractiveAuthState: "pending",
				EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{
					domain.RestrictionNativeAuthRequired,
				},
			},
		},
	}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	j := journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")}
	if err := j.Start(context.Background(), domain.OperationRecord{OperationID: "op-in-progress", Type: "update", IntegrationID: "doctor-demo", Status: "in_progress"}); err != nil {
		t.Fatalf("start journal: %v", err)
	}
	if err := j.Start(context.Background(), domain.OperationRecord{OperationID: "op-degraded", Type: "repair", IntegrationID: "doctor-demo", Status: "degraded"}); err != nil {
		t.Fatalf("start journal: %v", err)
	}
	if err := j.Start(context.Background(), domain.OperationRecord{OperationID: "op-failed", Type: "remove", IntegrationID: "doctor-demo", Status: "failed"}); err != nil {
		t.Fatalf("start journal: %v", err)
	}
	svc := Service{StateStore: store, Journal: j}

	report, err := svc.Doctor(context.Background())
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if !strings.Contains(report.Summary, "1 degraded target(s)") || !strings.Contains(report.Summary, "1 activation-pending target(s)") || !strings.Contains(report.Summary, "1 auth-pending target(s)") {
		t.Fatalf("summary = %q", report.Summary)
	}
	if len(report.Targets) != 3 {
		t.Fatalf("doctor targets = %+v", report.Targets)
	}
	joinedWarnings := strings.Join(report.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "run plugin-kit-ai integrations repair doctor-demo") {
		t.Fatalf("warnings missing degraded recovery guidance: %v", report.Warnings)
	}
	if !strings.Contains(joinedWarnings, "still marked in_progress") || !strings.Contains(joinedWarnings, "failed before commit") {
		t.Fatalf("warnings missing journal classification: %v", report.Warnings)
	}
	var codexTarget domain.TargetReport
	for _, target := range report.Targets {
		if target.TargetID == string(domain.TargetCodex) {
			codexTarget = target
			break
		}
	}
	if codexTarget.CatalogPolicy == nil || codexTarget.CatalogPolicy.Installation != "manual" {
		t.Fatalf("codex target missing catalog policy: %+v", codexTarget)
	}
	if !strings.Contains(strings.Join(codexTarget.ManualSteps, " "), "new agent thread") {
		t.Fatalf("codex target manual steps = %+v", codexTarget.ManualSteps)
	}
}

func TestListIncludesCapabilitySurfaceAndCatalogPolicy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	record := domain.InstallationRecord{
		IntegrationID:      "codex-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
		ResolvedVersion:    "0.1.0",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				TargetID:          domain.TargetCodex,
				DeliveryKind:      domain.DeliveryCodexMarketplace,
				CapabilitySurface: []string{"plugin_bundle", "mcp"},
				State:             domain.InstallActivationPending,
				ActivationState:   domain.ActivationNewThreadPending,
				CatalogPolicy:     &domain.CatalogPolicySnapshot{Installation: "manual", Authentication: "oauth", Category: "developer_tools"},
			},
		},
	}
	if err := store.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	svc := Service{StateStore: store}

	report, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(report.Targets) != 1 {
		t.Fatalf("list targets = %+v", report.Targets)
	}
	target := report.Targets[0]
	if len(target.CapabilitySurface) != 2 || target.CapabilitySurface[0] != "plugin_bundle" {
		t.Fatalf("capability surface = %+v", target.CapabilitySurface)
	}
	if target.CatalogPolicy == nil || target.CatalogPolicy.Authentication != "oauth" || target.CatalogPolicy.Category != "developer_tools" {
		t.Fatalf("catalog policy = %+v", target.CatalogPolicy)
	}
}

func TestAddNonDryRunFailsWhenPostApplyVerifyDoesNotObserveInstalledState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: verify-demo\nversion: 0.1.0\ndescription: test\ntargets:\n  - cursor\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.cursor","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	stateStore := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	inspectCalls := 0
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     stateStore,
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: stubTargetAdapter{
				id: domain.TargetCursor,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					inspectCalls++
					if inspectCalls == 1 {
						return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallRemoved, SourceAccessState: "ok"}, nil
					}
					return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallRemoved, SourceAccessState: "ok"}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "install_missing", EvidenceKey: "test.cursor"}, nil
				},
				applyInstall: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, ActivationState: domain.ActivationComplete}, nil
				},
				planRemove: func(in ports.PlanRemoveInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "remove_orphaned_target", EvidenceKey: "test.cursor"}, nil
				},
				applyRemove: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetCursor, State: domain.InstallRemoved}, nil
				},
			},
		},
	}

	if _, err := svc.Add(context.Background(), AddInput{Source: sourceRoot, Scope: "project", DryRun: false}); err == nil {
		t.Fatal("expected add to fail when verify still observes removed state")
	}
	state, err := stateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("installations = %+v, want none after verify rollback", state.Installations)
	}
}

func TestUpdateNonDryRunPersistsVerifiedStateInsteadOfRawApplyState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: verify-update-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.cursor","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	stateStore := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	record := domain.InstallationRecord{
		IntegrationID:      "verify-update-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedVersion:    "0.1.0",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				TargetID:          domain.TargetCursor,
				DeliveryKind:      domain.DeliveryCursorMCP,
				CapabilitySurface: []string{"mcp"},
				State:             domain.InstallInstalled,
			},
		},
	}
	if err := stateStore.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	inspectCalls := 0
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     stateStore,
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: stubTargetAdapter{
				id: domain.TargetCursor,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					inspectCalls++
					if inspectCalls == 1 {
						return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, SourceAccessState: "ok"}, nil
					}
					return ports.InspectResult{
						TargetID:                domain.TargetCursor,
						State:                   domain.InstallActivationPending,
						ActivationState:         domain.ActivationReloadPending,
						CatalogPolicy:           &domain.CatalogPolicySnapshot{Installation: "manual"},
						EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{domain.RestrictionReloadRequired},
						SourceAccessState:       "verified",
					}, nil
				},
				planUpdate: func(in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "update_version", EvidenceKey: "test.cursor"}, nil
				},
				applyUpdate: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, ActivationState: domain.ActivationComplete, SourceAccessState: "apply"}, nil
				},
			},
		},
	}

	report, err := svc.Update(context.Background(), NamedDryRunInput{Name: "verify-update-demo", DryRun: false})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(report.Targets) != 1 {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	if report.Targets[0].State != string(domain.InstallActivationPending) || report.Targets[0].ActivationState != string(domain.ActivationReloadPending) {
		t.Fatalf("report target = %+v", report.Targets[0])
	}
	state, err := stateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	target := state.Installations[0].Targets[domain.TargetCursor]
	if target.State != domain.InstallActivationPending || target.ActivationState != domain.ActivationReloadPending {
		t.Fatalf("persisted target = %+v", target)
	}
	if target.SourceAccessState != "verified" {
		t.Fatalf("source access state = %q, want verified", target.SourceAccessState)
	}
	if len(target.EnvironmentRestrictions) != 1 || target.EnvironmentRestrictions[0] != domain.RestrictionReloadRequired {
		t.Fatalf("restrictions = %+v", target.EnvironmentRestrictions)
	}
}

func TestUpdateNonDryRunVerifiesAgainstProvisionalOwnedState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	sourceRoot := filepath.Join(root, "plugin")
	writeFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: verify-owned-update-demo\nversion: 0.2.0\ndescription: test\ntargets:\n  - cursor\n")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.cursor","claim":"x","evidence_class":"project_policy","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	stateStore := jsonstate.Store{FS: fs, Path: filepath.Join(root, "state.json")}
	record := domain.InstallationRecord{
		IntegrationID:      "verify-owned-update-demo",
		RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedSourceRef:  domain.ResolvedSourceRef{Kind: "local_path", Value: sourceRoot},
		ResolvedVersion:    "0.1.0",
		Policy:             domain.InstallPolicy{Scope: "project", AutoUpdate: true, AdoptNewTargets: "manual"},
		WorkspaceRoot:      filepath.Join(root, "workspace-a"),
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				TargetID:          domain.TargetCursor,
				DeliveryKind:      domain.DeliveryCursorMCP,
				CapabilitySurface: []string{"mcp"},
				State:             domain.InstallInstalled,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "cursor_mcp_server", Name: "release-checks"},
				},
			},
		},
	}
	if err := stateStore.Save(context.Background(), ports.StateFile{SchemaVersion: 1, Installations: []domain.InstallationRecord{record}}); err != nil {
		t.Fatalf("seed state: %v", err)
	}
	inspectCalls := 0
	svc := Service{
		SourceResolver: source.Resolver{},
		ManifestLoader: manifest.Loader{},
		StateStore:     stateStore,
		LockManager:    locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:        journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:       evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetCursor: stubTargetAdapter{
				id: domain.TargetCursor,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					inspectCalls++
					if inspectCalls == 1 {
						return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, SourceAccessState: "ok"}, nil
					}
					if in.Record == nil {
						t.Fatal("expected verify inspect to receive provisional record")
					}
					target := in.Record.Targets[domain.TargetCursor]
					for _, obj := range target.OwnedNativeObjects {
						if obj.Kind == "cursor_mcp_server" && obj.Name == "release-checks-v2" {
							return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallInstalled, SourceAccessState: "verified"}, nil
						}
					}
					return ports.InspectResult{TargetID: domain.TargetCursor, State: domain.InstallRemoved, SourceAccessState: "verified"}, nil
				},
				planUpdate: func(in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetCursor, ActionClass: "update_version", EvidenceKey: "test.cursor"}, nil
				},
				applyUpdate: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{
						TargetID: domain.TargetCursor,
						State:    domain.InstallInstalled,
						OwnedNativeObjects: []domain.NativeObjectRef{
							{Kind: "cursor_mcp_server", Name: "release-checks-v2"},
						},
					}, nil
				},
			},
		},
		CurrentWorkspaceRoot: filepath.Join(root, "workspace-b"),
	}

	report, err := svc.Update(context.Background(), NamedDryRunInput{Name: "verify-owned-update-demo", DryRun: false})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(report.Targets) != 1 || report.Targets[0].State != string(domain.InstallInstalled) {
		t.Fatalf("report = %+v", report)
	}
	state, err := stateStore.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	target := state.Installations[0].Targets[domain.TargetCursor]
	if len(target.OwnedNativeObjects) != 1 || target.OwnedNativeObjects[0].Name != "release-checks-v2" {
		t.Fatalf("owned native objects = %+v", target.OwnedNativeObjects)
	}
}

func TestAddPersistsWorkspaceRootForProjectScope(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	statePath := filepath.Join(root, "state.json")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.gemini","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: statePath}
	workspaceRoot := filepath.Join(root, "workspace-a")
	inspectCalls := 0

	svc := Service{
		SourceResolver: stubResolver{
			resolve: func(ref domain.IntegrationRef) (ports.ResolvedSource, error) {
				return ports.ResolvedSource{
					Kind:      "local_path",
					Requested: domain.RequestedSourceRef{Kind: "local_path", Value: ref.Raw},
					Resolved:  domain.ResolvedSourceRef{Kind: "local_path", Value: ref.Raw},
					LocalPath: ref.Raw,
				}, nil
			},
		},
		ManifestLoader: stubManifestLoader{
			load: func(ports.ResolvedSource) (domain.IntegrationManifest, error) {
				return domain.IntegrationManifest{
					IntegrationID: "gemini-demo",
					Version:       "0.1.0",
					RequestedRef:  domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
					ResolvedRef:   domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
					Deliveries: []domain.Delivery{{
						TargetID:      domain.TargetGemini,
						DeliveryKind:  domain.DeliveryGeminiExtension,
						Name:          "gemini-demo",
						NativeRefHint: "gemini-demo",
					}},
				}, nil
			},
		},
		StateStore:           store,
		LockManager:          locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:              journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:             evidence.Registry{FS: fs, Path: evidencePath},
		CurrentWorkspaceRoot: workspaceRoot,
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetGemini: stubTargetAdapter{
				id: domain.TargetGemini,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					inspectCalls++
					if inspectCalls == 1 {
						return ports.InspectResult{TargetID: domain.TargetGemini, State: domain.InstallRemoved}, nil
					}
					return ports.InspectResult{TargetID: domain.TargetGemini, State: domain.InstallInstalled}, nil
				},
				applyInstall: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetGemini, State: domain.InstallInstalled}, nil
				},
			},
		},
	}

	_, err := svc.Add(context.Background(), AddInput{
		Source:  filepath.Join(root, "plugin"),
		Targets: []string{"gemini"},
		Scope:   "project",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	state, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("installations = %d, want 1", len(state.Installations))
	}
	if got := state.Installations[0].WorkspaceRoot; got != workspaceRoot {
		t.Fatalf("workspace root = %q, want %q", got, workspaceRoot)
	}
}

func TestAddAppliesInstallableTargetsAndSkipsBlockedOnes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	statePath := filepath.Join(root, "state.json")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.claude","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]},{"key":"test.gemini","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: statePath}
	geminiInspectCalls := 0

	svc := Service{
		SourceResolver: stubResolver{
			resolve: func(ref domain.IntegrationRef) (ports.ResolvedSource, error) {
				return ports.ResolvedSource{
					Kind:      "local_path",
					Requested: domain.RequestedSourceRef{Kind: "local_path", Value: ref.Raw},
					Resolved:  domain.ResolvedSourceRef{Kind: "local_path", Value: ref.Raw},
					LocalPath: ref.Raw,
				}, nil
			},
		},
		ManifestLoader: stubManifestLoader{
			load: func(ports.ResolvedSource) (domain.IntegrationManifest, error) {
				return domain.IntegrationManifest{
					IntegrationID: "demo",
					Version:       "0.1.0",
					RequestedRef:  domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
					ResolvedRef:   domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
					Deliveries: []domain.Delivery{
						{TargetID: domain.TargetClaude, DeliveryKind: domain.DeliveryClaudeMarketplace, Name: "demo-claude"},
						{TargetID: domain.TargetGemini, DeliveryKind: domain.DeliveryGeminiExtension, Name: "demo-gemini"},
					},
				}, nil
			},
		},
		StateStore:  store,
		LockManager: locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:     journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:    evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetClaude: stubTargetAdapter{
				id: domain.TargetClaude,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					return ports.InspectResult{
						TargetID:                domain.TargetClaude,
						State:                   domain.InstallInstalled,
						ActivationState:         domain.ActivationReloadPending,
						EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{domain.RestrictionSourceToolMissing},
					}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{
						TargetID:    domain.TargetClaude,
						ActionClass: "install_missing",
						EvidenceKey: "test.claude",
						Blocking:    true,
						ManualSteps: []string{"Claude Code CLI is not available on PATH; install/configure Claude Code and rerun from a shell where `claude` works"},
					}, nil
				},
			},
			domain.TargetGemini: stubTargetAdapter{
				id: domain.TargetGemini,
				inspect: func(in ports.InspectInput) (ports.InspectResult, error) {
					geminiInspectCalls++
					if geminiInspectCalls == 1 {
						return ports.InspectResult{TargetID: domain.TargetGemini, State: domain.InstallRemoved}, nil
					}
					return ports.InspectResult{TargetID: domain.TargetGemini, State: domain.InstallInstalled, ActivationState: domain.ActivationRestartPending}, nil
				},
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					return ports.AdapterPlan{TargetID: domain.TargetGemini, ActionClass: "install_missing", EvidenceKey: "test.gemini"}, nil
				},
				applyInstall: func(in ports.ApplyInput) (ports.ApplyResult, error) {
					return ports.ApplyResult{TargetID: domain.TargetGemini, State: domain.InstallInstalled, ActivationState: domain.ActivationRestartPending}, nil
				},
			},
		},
	}

	report, err := svc.Add(context.Background(), AddInput{Source: filepath.Join(root, "plugin")})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if report.Summary != `Installed integration "demo" at version 0.1.0 on 1 target.` {
		t.Fatalf("summary = %q", report.Summary)
	}
	if report.RequestedTargetCount != 2 {
		t.Fatalf("requested target count = %d, want 2", report.RequestedTargetCount)
	}
	if len(report.SkippedTargets) != 1 || report.SkippedTargets[0] != "claude" {
		t.Fatalf("skipped targets = %+v", report.SkippedTargets)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], `Skipped "claude"`) {
		t.Fatalf("warnings = %+v", report.Warnings)
	}
	if len(report.Targets) != 1 || report.Targets[0].TargetID != "gemini" {
		t.Fatalf("report targets = %+v", report.Targets)
	}
	state, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("installations = %d, want 1", len(state.Installations))
	}
	if _, ok := state.Installations[0].Targets[domain.TargetClaude]; ok {
		t.Fatalf("blocked target should not be persisted: %+v", state.Installations[0].Targets)
	}
	if got := state.Installations[0].Targets[domain.TargetGemini].State; got != domain.InstallInstalled {
		t.Fatalf("gemini state = %s, want installed", got)
	}
}

func TestAddRejectsAlreadyManagedIntegrationBeforePlanningTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	statePath := filepath.Join(root, "state.json")
	evidencePath := filepath.Join(root, "evidence.json")
	writeFile(t, evidencePath, `{"schema_version":1,"entries":[{"key":"test.claude","claim":"x","evidence_class":"confirmed_vendor_fact","urls":["https://example.com"]}]}`)
	fs := fsadapter.OS{}
	store := jsonstate.Store{FS: fs, Path: statePath}
	if err := store.Save(context.Background(), ports.StateFile{
		SchemaVersion: 1,
		Installations: []domain.InstallationRecord{{
			IntegrationID: "demo",
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetClaude: {TargetID: domain.TargetClaude, DeliveryKind: domain.DeliveryClaudeMarketplace, State: domain.InstallInstalled},
			},
		}},
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	planned := false
	svc := Service{
		SourceResolver: stubResolver{
			resolve: func(ref domain.IntegrationRef) (ports.ResolvedSource, error) {
				return ports.ResolvedSource{
					Kind:      "local_path",
					Requested: domain.RequestedSourceRef{Kind: "local_path", Value: ref.Raw},
					Resolved:  domain.ResolvedSourceRef{Kind: "local_path", Value: ref.Raw},
					LocalPath: ref.Raw,
				}, nil
			},
		},
		ManifestLoader: stubManifestLoader{
			load: func(ports.ResolvedSource) (domain.IntegrationManifest, error) {
				return domain.IntegrationManifest{
					IntegrationID: "demo",
					Version:       "0.1.0",
					RequestedRef:  domain.RequestedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
					ResolvedRef:   domain.ResolvedSourceRef{Kind: "local_path", Value: filepath.Join(root, "plugin")},
					Deliveries: []domain.Delivery{
						{TargetID: domain.TargetClaude, DeliveryKind: domain.DeliveryClaudeMarketplace, Name: "demo-claude"},
					},
				}, nil
			},
		},
		StateStore:  store,
		LockManager: locks.FileLock{BaseDir: filepath.Join(root, "locks")},
		Journal:     journal.FileJournal{FS: fs, BaseDir: filepath.Join(root, "ops")},
		Evidence:    evidence.Registry{FS: fs, Path: evidencePath},
		Adapters: map[domain.TargetID]ports.TargetAdapter{
			domain.TargetClaude: stubTargetAdapter{
				id: domain.TargetClaude,
				planInstall: func(in ports.PlanInstallInput) (ports.AdapterPlan, error) {
					planned = true
					return ports.AdapterPlan{TargetID: domain.TargetClaude, ActionClass: "install_missing", EvidenceKey: "test.claude", Blocking: true}, nil
				},
			},
		},
	}

	_, err := svc.Add(context.Background(), AddInput{Source: filepath.Join(root, "plugin")})
	if err == nil || !strings.Contains(err.Error(), "integration already exists in state: demo") {
		t.Fatalf("error = %v", err)
	}
	if planned {
		t.Fatal("target planning should not run when the integration is already managed")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
