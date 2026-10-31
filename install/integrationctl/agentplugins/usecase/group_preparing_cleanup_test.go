package usecase

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestGroupedPreparingProgressFailureCleansUncommittedResources(t *testing.T) {
	for _, tc := range []struct {
		name     string
		refuseAt int
		reuse    bool
	}{
		{name: "later target with new data", refuseAt: 1},
		{name: "later target with reused data", refuseAt: 1, reuse: true},
		{name: "first target", refuseAt: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newPreparingCleanupFixture(t)
			var retained map[string][]byte
			if tc.reuse {
				retained = fixture.retainData(t)
			}
			group := fixture.group()
			preparing := 0
			group.Progress = func(event GroupProgressEvent) {
				if event.Phase != GroupProgressPreparing {
					return
				}
				preparing++
				if event.ClientID == fixture.targets[tc.refuseAt].Client.ClientID {
					fixture.authority.failNext = true
				}
			}
			result, err := fixture.service.AddGroup(t.Context(), group)
			fixture.requirePreparationFailure(t, result, err, fixture.authority.failure)
			if preparing != tc.refuseAt+1 || len(fixture.stager.staged) != tc.refuseAt {
				t.Fatalf("preparing events=%d staged=%d, refusal index=%d", preparing, len(fixture.stager.staged), tc.refuseAt)
			}
			for _, delivery := range fixture.stager.staged {
				requirePreparingPathAbsent(t, delivery.StagingPath)
			}
			if tc.reuse {
				requirePreparingBytes(t, retained)
			} else {
				requirePreparingPathAbsent(t, fixture.dataPath())
			}
		})
	}
}

func TestGroupedPreparingFailurePreservesRecoveryCustody(t *testing.T) {
	for _, mode := range []string{"pending journal", "replaced profile"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newPreparingCleanupFixture(t)
			group := fixture.group()
			want := fixture.authority.failure
			var preserved map[string][]byte
			group.Progress = func(event GroupProgressEvent) {
				if event.Phase != GroupProgressPreparing || event.ClientID != domain.ClientKiro {
					return
				}
				if len(fixture.stager.staged) != 1 {
					t.Fatal("custody control did not reach real first-target staging")
				}
				preserved = fixture.uncommittedBytes(t)
				if mode == "pending journal" {
					journal := filepath.Join(fixture.service.Kernel.Directory.JournalDir, group.OperationGroupID+"-001.json")
					// Presence alone claims cleanup custody; parsing or replaying
					// transaction recovery is outside this guard regression.
					writePreparingFile(t, journal, []byte("TEST pending recovery custody"))
					preserved[journal] = []byte("TEST pending recovery custody")
					fixture.authority.failNext = true
					return
				}
				root := fixture.targets[1].Client.ConfigRoot
				if err := os.Rename(root, root+"-held"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
				want = directoryidentity.ErrChanged
			}
			result, err := fixture.service.AddGroup(t.Context(), group)
			fixture.requirePreparationFailure(t, result, err, want)
			if len(preserved) == 0 {
				t.Fatal("custody control did not reach its preparing callback")
			}
			requirePreparingBytes(t, preserved)
		})
	}
}

func TestGroupedPreparingFailurePreservesManagedNoChange(t *testing.T) {
	fixture := newPreparingCleanupFixture(t)
	installed, err := fixture.service.AddGroup(t.Context(), fixture.group())
	if err != nil || !installed.Mutated {
		t.Fatalf("seed committed group: %+v, %v", installed, err)
	}
	stateBefore, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	preserved := fixture.retainData(t)
	preserved[fixture.store.Path] = stateBefore
	for _, target := range installed.Targets {
		path := filepath.Join(target.Plan.ActivePath, "plugin.json")
		preserved[path] = readPreparingFile(t, path)
	}
	fixture.stager.staged = nil
	group := fixture.group()
	group.OperationGroupID += "-no-change"
	group.Progress = func(event GroupProgressEvent) {
		if event.Phase == GroupProgressPreparing && event.ClientID == domain.ClientKiro {
			fixture.authority.failNext = true
		}
	}
	result, err := fixture.service.AddGroup(t.Context(), group)
	if !errors.Is(err, fixture.authority.failure) || result.Phase != GroupPhasePreparationFailed || result.Mutated || len(result.Receipts) != 0 {
		t.Fatalf("no-change preparation refusal: %+v, %v", result, err)
	}
	if len(fixture.stager.staged) != 0 || len(result.Targets) != 2 || !result.Targets[0].NoChange || !result.Targets[1].NoChange {
		t.Fatalf("no-change refusal staged resources or changed planning: %+v", result)
	}
	requirePreparingBytes(t, preserved)
}

// Only the next authority observation is injected. Capture and all other
// observations retain the real directory authority algorithm and guards.
type preparingCleanupAuthority struct {
	failNext bool
	failure  error
}

func (*preparingCleanupAuthority) CaptureProfileAuthority(ctx context.Context, client domain.DetectedClient) (domain.ProfileAuthority, error) {
	return profileauthority.Capture(ctx, client.ConfigRoot)
}

func (authority *preparingCleanupAuthority) RevalidateProfileAuthority(ctx context.Context, _ domain.ClientID, token domain.ProfileAuthority) error {
	if authority.failNext {
		authority.failNext = false
		return authority.failure
	}
	return profileauthority.Revalidate(ctx, token)
}

type preparingCleanupStager struct {
	providers.Stager
	staged []domain.StagedDelivery
}

func (stager *preparingCleanupStager) StageWithPluginData(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operation string, hints domain.CompatibilityHints, data string) (domain.StagedDelivery, error) {
	delivery, err := stager.Stager.StageWithPluginData(ctx, envelope, plan, operation, hints, data)
	if err == nil {
		stager.staged = append(stager.staged, delivery)
	}
	return delivery, err
}

type preparingCleanupFixture struct {
	service   Service
	store     statev2.Store
	manager   providers.PluginDataManager
	stager    *preparingCleanupStager
	authority *preparingCleanupAuthority
	targets   []AddInput
}

func newPreparingCleanupFixture(t *testing.T) preparingCleanupFixture {
	t.Helper()
	service, store, cursor := serviceFixture(t)
	service.Kernel.Namespace = filepath.Dir(store.Path)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	for _, root := range []string{cursor.ConfigRoot, kiro.ConfigRoot} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	helper := filepath.Join(t.TempDir(), "helper")
	writePreparingFile(t, helper, []byte("inert helper bytes; never executed"))
	if err := os.Chmod(helper, 0o700); err != nil {
		t.Fatal(err)
	}
	source, err := managedstdio.NewSource(helper, "TEST")
	if err != nil {
		t.Fatal(err)
	}
	stager := service.Stager.(providers.Stager)
	stager.LauncherSource = source
	recorder := &preparingCleanupStager{Stager: stager}
	service.Stager = recorder
	authority := &preparingCleanupAuthority{failure: errors.New("TEST profile observation failed")}
	service.PhysicalAuthority = authority
	first := addInput(t, cursor, "https://example.test/preparing-cleanup")
	first.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
		"local": {Name: "local", Type: "stdio", Decoded: map[string]any{
			"command": filepath.Base(helper), "args": []any{"${PLUGIN_DATA}"},
			"env": map[string]any{"PATH": filepath.Dir(helper)},
		}},
	}}
	first.Envelope.Inventory.MCPServers = []string{"local"}
	second := first
	second.Client = kiro
	return preparingCleanupFixture{
		service: service, store: store, manager: service.PluginData.(providers.PluginDataManager),
		stager: recorder, authority: authority, targets: []AddInput{first, second},
	}
}

func (fixture preparingCleanupFixture) group() GroupInput {
	return GroupInput{Targets: fixture.targets, OperationGroupID: "TEST-preparing-cleanup", Confirmed: true}
}

func (fixture preparingCleanupFixture) dataPath() string {
	first := fixture.targets[0]
	return filepath.Join(fixture.manager.Base, domain.ComputePhysicalArtifactID(first.Envelope.Manifest.Name, first.InstallationID))
}

func (fixture preparingCleanupFixture) retainData(t *testing.T) map[string][]byte {
	t.Helper()
	first := fixture.targets[0]
	physical := domain.ComputePhysicalArtifactID(first.Envelope.Manifest.Name, first.InstallationID)
	if _, _, err := fixture.manager.EnsureData(t.Context(), first.InstallationID, physical, string(first.Scope)); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(fixture.dataPath(), ".agentplugins-data-owner.json")
	userFile := filepath.Join(fixture.dataPath(), "retained-user-bytes")
	writePreparingFile(t, userFile, []byte("retain these existing user bytes"))
	return map[string][]byte{marker: readPreparingFile(t, marker), userFile: readPreparingFile(t, userFile)}
}

func (fixture preparingCleanupFixture) uncommittedBytes(t *testing.T) map[string][]byte {
	t.Helper()
	manifest := filepath.Join(fixture.stager.staged[0].StagingPath, "plugin.json")
	marker := filepath.Join(fixture.dataPath(), ".agentplugins-data-owner.json")
	return map[string][]byte{manifest: readPreparingFile(t, manifest), marker: readPreparingFile(t, marker)}
}

func (fixture preparingCleanupFixture) requirePreparationFailure(t *testing.T, result GroupResult, err, want error) {
	t.Helper()
	if !errors.Is(err, want) || result.Phase != GroupPhasePreparationFailed || result.Mutated || len(result.Receipts) != 0 {
		t.Fatalf("preparation refusal: %+v, %v; want %v", result, err, want)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil || len(state.Installations) != 0 {
		t.Fatalf("preparation refusal changed state: %+v, %v", state, loadErr)
	}
	requirePreparingPathAbsent(t, fixture.store.Path)
	for _, target := range result.Targets {
		requirePreparingPathAbsent(t, target.Plan.ActivePath)
	}
}

func requirePreparingPathAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("uncommitted path remains %s: %v", path, err)
	}
}

func requirePreparingBytes(t *testing.T, preserved map[string][]byte) {
	t.Helper()
	for path, before := range preserved {
		if after := readPreparingFile(t, path); !bytes.Equal(before, after) {
			t.Errorf("preserved bytes changed: %s", path)
		}
	}
}

func readPreparingFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func writePreparingFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
