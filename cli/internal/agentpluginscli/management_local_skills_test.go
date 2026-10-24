package agentpluginscli

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func localSkillWrite(t *testing.T, filename, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filename, body)
}
func localSkillTestBackend(t *testing.T) (*ManagementBackend, string) {
	t.Helper()
	j := newSavedJourney(t, t.TempDir())
	base := managementJourneyBackend(t, j)
	cfg := base.config
	root := filepath.Join(t.TempDir(), "project", ".agents", "skills")
	cfg.AuthoringRoots = []management.LocalSkillRoot{{Id: "project-local", Path: root, Scope: management.Project}}
	backend, err := NewManagementBackend(j.cli.app, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.Close)
	return backend, root
}
func localSkillTestFile(name string, old *string, body []byte) management.LocalSkillFile {
	content := base64.StdEncoding.EncodeToString(body)
	return management.LocalSkillFile{RelativePath: name, ExpectedSHA256: old, ContentBase64: &content}
}
func localSkillTestRequest(files ...management.LocalSkillFile) management.LocalSkillRequest {
	return management.LocalSkillRequest{Action: management.LocalSkillRequestAction("upsert"), RootId: "project-local", SkillName: "draft", Files: files}
}
func localSkillPrepare(t *testing.T, b *ManagementBackend, request management.LocalSkillRequest, id string) management.Prepared {
	t.Helper()
	p, err := b.PrepareLocalSkill(t.Context(), request, id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
func localSkillApply(t *testing.T, p management.Prepared) management.Outcome {
	t.Helper()
	out, err := p.Apply(t.Context(), nil)
	if err != nil || out.State != management.Succeeded {
		t.Fatalf("local apply: %+v %v", out, err)
	}
	return out
}
func TestManagementLocalSkillCreateEditDeleteRestore(t *testing.T) {
	b, root := localSkillTestBackend(t)
	first := localSkillPrepare(t, b, localSkillTestRequest(localSkillTestFile("SKILL.md", nil, []byte("# Draft\n")), localSkillTestFile("assets/example.bin", nil, []byte{0, 255, 1})), strings.Repeat("1", 64))
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("review created the authoring root", err)
	}
	localSkillApply(t, first)
	if body, err := os.ReadFile(filepath.Join(root, "draft", "assets", "example.bin")); err != nil || string(body) != string([]byte{0, 255, 1}) {
		t.Fatal(body, err)
	}
	before := localSkillDigest([]byte("# Draft\n"))
	edit := localSkillPrepare(t, b, localSkillTestRequest(localSkillTestFile("SKILL.md", &before, []byte("# Reviewed\n"))), strings.Repeat("2", 64))
	localSkillApply(t, edit)
	deleted := localSkillApply(t, localSkillPrepare(t, b, management.LocalSkillRequest{Action: management.LocalSkillRequestAction("delete"), RootId: "project-local", SkillName: "draft"}, strings.Repeat("3", 64)))
	if deleted.Result.LocalSkillBackup == nil {
		t.Fatal("delete did not retain reversible data")
	}
	if _, err := os.Stat(filepath.Join(root, "draft")); !os.IsNotExist(err) {
		t.Fatal("delete retained active Skill", err)
	}
	backup := deleted.Result.LocalSkillBackup
	if localSkillPathOverlap(root, backup.Path) {
		t.Fatal("retained backup is visible to native Skill discovery", backup.Path)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) != 0 {
		t.Fatal("deleted Skill or staging remained in native discovery root", entries, readErr)
	}
	if body, err := os.ReadFile(filepath.Join(backup.Path, "SKILL.md")); err != nil || string(body) != "# Reviewed\n" {
		t.Fatal(body, err)
	}
	restored := localSkillApply(t, localSkillPrepare(t, b, management.LocalSkillRequest{Action: management.LocalSkillRequestAction("restore"), RootId: "project-local", SkillName: "draft", RestoreOperationId: backup.OperationId}, strings.Repeat("4", 64)))
	if len(restored.ReconciledOperationIDs) != 1 || restored.ReconciledOperationIDs[0] != backup.OperationId {
		t.Fatal("restore lacks exact reconciliation proof", restored)
	}
	if body, err := os.ReadFile(filepath.Join(root, "draft", "SKILL.md")); err != nil || string(body) != "# Reviewed\n" {
		t.Fatal(body, err)
	}
	manifest, lock := b.app.dependencyPaths()
	for _, filename := range []string{manifest, lock} {
		if _, err := os.Stat(filename); !os.IsNotExist(err) {
			t.Fatal("local authoring wrote dependency intent", filename, err)
		}
	}
}
func TestManagementLocalSkillStaleTreeAndManagedOwnershipRace(t *testing.T) {
	b, root := localSkillTestBackend(t)
	filename := filepath.Join(root, "draft", "SKILL.md")
	localSkillWrite(t, filename, "old")
	old := localSkillDigest([]byte("old"))
	request := localSkillTestRequest(localSkillTestFile("SKILL.md", &old, []byte("new")))
	stale := localSkillPrepare(t, b, request, strings.Repeat("5", 64))
	localSkillWrite(t, filepath.Join(root, "draft", "extra.txt"), "unreviewed")
	out, err := stale.Apply(t.Context(), nil)
	if !errors.Is(err, management.ErrPlanStale) || out.Result.Mutated {
		t.Fatal("unreviewed tree changed", out, err)
	}
	owned := localSkillPrepare(t, b, request, strings.Repeat("6", 64))
	state, err := b.app.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	// The real installer remains in the fixture. This application-state seam
	// makes a new CLI ownership receipt visible between review and application.
	state.Installations = append(state.Installations, domain.Installation{Clients: map[string]domain.ClientBinding{"native": {NativeObjects: []domain.NativeObjectOwnership{{Path: filepath.Dir(filename), ProtectionClass: "managed"}}}}})
	b.app.StateStore = &localSkillObservedState{state: state}
	out, err = owned.Apply(t.Context(), nil)
	var refused *management.Refusal
	if !errors.As(err, &refused) || refused.Code != "MANAGED_SKILL" || out.Result.Mutated {
		t.Fatal("new managed ownership was overwritten", out, err)
	}
	if got := dependencyRead(t, filename); got != "old" {
		t.Fatal(got)
	}
}

type localSkillObservedState struct{ state domain.StateFileV2 }

func (s *localSkillObservedState) Load() (domain.StateFileV2, error)   { return s.state, nil }
func (s *localSkillObservedState) Save(state domain.StateFileV2) error { s.state = state; return nil }
func TestManagementLocalSkillHonorsNativeMutationLock(t *testing.T) {
	b, root := localSkillTestBackend(t)
	p := localSkillPrepare(t, b, localSkillTestRequest(localSkillTestFile("SKILL.md", nil, []byte("# Draft"))), strings.Repeat("7", 64))
	unlock, err := (processlock.Lock{Path: filepath.Join(b.config.StateRoot, "mutation.lock")}).Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()
	out, err := p.Apply(t.Context(), nil)
	if !errors.Is(err, processlock.ErrActive) || out.Result.Mutated {
		t.Fatal(out, err)
	}
	if _, err := os.Stat(filepath.Join(root, "draft")); !os.IsNotExist(err) {
		t.Fatal("lock refusal created active Skill", err)
	}
}
func TestManagementLocalSkillRejectsTraversalSymlinksAndChangedBackup(t *testing.T) {
	b, root := localSkillTestBackend(t)
	for _, name := range []string{"../escape", "assets/../../escape", "/absolute", "a\\b", "NUL"} {
		_, err := b.PrepareLocalSkill(t.Context(), localSkillTestRequest(localSkillTestFile(name, nil, []byte("x"))), strings.Repeat("8", 64))
		if err == nil {
			t.Fatalf("accepted unsafe relative name %q", name)
		}
	}
	localSkillWrite(t, filepath.Join(root, "draft", "SKILL.md"), "original")
	outside := filepath.Join(t.TempDir(), "important")
	localSkillWrite(t, outside, "unchanged")
	if err := os.Symlink(outside, filepath.Join(root, "draft", "link")); err != nil {
		t.Skip("symlinks unavailable", err)
	}
	_, err := b.PrepareLocalSkill(t.Context(), management.LocalSkillRequest{Action: management.LocalSkillRequestAction("delete"), RootId: "project-local", SkillName: "draft"}, strings.Repeat("8", 64))
	if err == nil {
		t.Fatal("accepted a symbolic-link tree")
	}
	if err = os.Remove(filepath.Join(root, "draft", "link")); err != nil {
		t.Fatal(err)
	}
	deleted := localSkillApply(t, localSkillPrepare(t, b, management.LocalSkillRequest{Action: management.LocalSkillRequestAction("delete"), RootId: "project-local", SkillName: "draft"}, strings.Repeat("9", 64)))
	backup := deleted.Result.LocalSkillBackup
	localSkillWrite(t, filepath.Join(backup.Path, "SKILL.md"), "edited backup")
	_, err = b.PrepareLocalSkill(t.Context(), management.LocalSkillRequest{Action: management.LocalSkillRequestAction("restore"), RootId: "project-local", SkillName: "draft", RestoreOperationId: backup.OperationId}, strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("restored a changed backup")
	}
	if got := dependencyRead(t, outside); got != "unchanged" {
		t.Fatal("outside file modified")
	}
}
func TestManagementLocalSkillRestoresInterruptedDirectorySwap(t *testing.T) {
	b, configuredRoot := localSkillTestBackend(t)
	// Internal receipts use the captured physical root, as publication does.
	rootPath := b.config.AuthoringRoots[0].Path
	localSkillWrite(t, filepath.Join(rootPath, "draft", "SKILL.md"), "before crash")
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	before, err := readLocalSkillTree(root, "draft", true)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("b", 64)
	backupName := id
	if err = b.backupLocalSkill(backupName, before); err != nil {
		t.Fatal(err)
	}
	absent := localSkillTree{Directories: []localSkillDirectory{}, Files: []localSkillFile{}}
	receipt := localSkillReceipt{Version: 1, OperationID: id, RootID: "project-local", RootPath: rootPath, SkillName: "draft", BeforeDigest: before.digest(), AfterDigest: absent.digest(), BeforeExists: true, BackupName: backupName, Phase: "prepared"}
	if err = b.saveLocalSkillReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	manager := b.localSkillManager()
	manager.Fault = func(phase string) error {
		if phase == dirswap.FaultBackupRenamed {
			return errors.New("simulated interruption")
		}
		return nil
	}
	_, err = manager.Apply(t.Context(), dirswap.Input{OperationID: id, ClientBindingID: "local-skill", Sequence: 1, OwnedBase: rootPath, ActivePath: filepath.Join(rootPath, "draft"), Remove: true, VerifyActive: func(_ context.Context, physical string) error {
		observed, e := readLocalSkillTree(root, filepath.Base(physical), false)
		if e != nil || observed.digest() != before.digest() {
			return errors.New("changed original")
		}
		return nil
	}})
	if err == nil {
		t.Fatal("fault did not interrupt")
	}
	if _, err = os.Stat(filepath.Join(rootPath, "draft")); !os.IsNotExist(err) {
		t.Fatal("fault not in rename window", err)
	}
	localSkillAssertRestoreRefusesNewOwner(t, b, configuredRoot, id)
	out := localSkillApply(t, localSkillPrepare(t, b, management.LocalSkillRequest{Action: management.LocalSkillRequestAction("restore"), RootId: "project-local", SkillName: "draft", RestoreOperationId: id}, strings.Repeat("c", 64)))
	if len(out.ReconciledOperationIDs) != 1 || dependencyRead(t, filepath.Join(rootPath, "draft", "SKILL.md")) != "before crash" {
		t.Fatal(out)
	}
	if _, err = b.localSkillManager().Load(id); !os.IsNotExist(err) {
		t.Fatal("recovery retained unfinished native journal", err)
	}
}

// An independent CLI can claim an absent path after a crash. Its authoritative
// ownership must win over our older local undo, even with a valid swap journal.
func localSkillAssertRestoreRefusesNewOwner(t *testing.T, b *ManagementBackend, root, id string) {
	t.Helper()
	original := b.app.StateStore
	state, err := original.Load()
	if err != nil {
		t.Fatal(err)
	}
	state.Installations = append(state.Installations, domain.Installation{Clients: map[string]domain.ClientBinding{"native": {NativeObjects: []domain.NativeObjectOwnership{{Path: filepath.Join(root, "draft"), ProtectionClass: "managed"}}}}})
	b.app.StateStore = &localSkillObservedState{state: state}
	defer func() { b.app.StateStore = original }()
	_, err = b.PrepareLocalSkill(t.Context(), management.LocalSkillRequest{Action: management.LocalSkillRequestAction("restore"), RootId: "project-local", SkillName: "draft", RestoreOperationId: id}, strings.Repeat("f", 64))
	var refused *management.Refusal
	if !errors.As(err, &refused) || refused.Code != "MANAGED_SKILL" {
		t.Fatal("interrupted local restore ignored a newer native owner", err)
	}
	if got := dependencyRead(t, filepath.Join(b.localSkillBackupRoot(), id, "SKILL.md")); got != "before crash" {
		t.Fatal("refused restoration changed private backup", got)
	}
	if _, err = b.localSkillManager().Load(id); err != nil {
		t.Fatal("refused restoration discarded the pending swap journal", err)
	}
}

func TestManagementLocalSkillRestoreRejectsNewReceiptAfterReview(t *testing.T) {
	b, _ := localSkillTestBackend(t)
	root := b.config.AuthoringRoots[0].Path
	id := strings.Repeat("d", 64)
	p := localSkillPrepare(t, b, management.LocalSkillRequest{Action: management.LocalSkillRequestAction("restore"), RootId: "project-local", SkillName: "draft", RestoreOperationId: id}, strings.Repeat("e", 64))
	absent := localSkillTree{Directories: []localSkillDirectory{}, Files: []localSkillFile{}}
	receipt := localSkillReceipt{Version: 1, OperationID: id, RootID: "project-local", RootPath: root, SkillName: "draft", BeforeDigest: absent.digest(), AfterDigest: absent.digest(), BackupName: id, Phase: "prepared"}
	if err := b.saveLocalSkillReceipt(receipt); err != nil {
		t.Fatal(err)
	}
	out, err := p.Apply(t.Context(), nil)
	if !errors.Is(err, management.ErrPlanStale) || len(out.ReconciledOperationIDs) != 0 {
		t.Fatal("new original effects were reconciled through stale absence proof", out, err)
	}
}

func TestManagementLocalSkillRetentionTouchesOnlyVerifiedPrivateUndo(t *testing.T) {
	b, root := localSkillTestBackend(t)
	active := filepath.Join(root, "draft", "SKILL.md")
	localSkillWrite(t, active, "original")
	id := strings.Repeat("f", 64)
	localSkillApply(t, localSkillPrepare(t, b, management.LocalSkillRequest{Action: management.LocalSkillRequestAction("delete"), RootId: "project-local", SkillName: "draft"}, id))
	localSkillWrite(t, active, "new local content")
	operation := management.Operation{Action: management.LocalSkill, OperationId: id, LocalSkill: &management.LocalSkillIdentity{RootId: "project-local", SkillName: "draft"}}
	backup := filepath.Join(b.localSkillBackupRoot(), id, "SKILL.md")
	localSkillWrite(t, backup, "user edited undo")
	if err := b.ForgetObservation(t.Context(), operation); err == nil {
		t.Fatal("expiry deleted a changed backup")
	}
	if got := dependencyRead(t, backup); got != "user edited undo" {
		t.Fatal("changed undo data lost")
	}
	localSkillWrite(t, backup, "original")
	if err := b.ForgetObservation(t.Context(), operation); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(backup)); !os.IsNotExist(err) {
		t.Fatal("verified private undo not removed", err)
	}
	if _, err := os.Stat(b.localSkillReceiptPath(id)); !os.IsNotExist(err) {
		t.Fatal("undo metadata not removed", err)
	}
	if got := dependencyRead(t, active); got != "new local content" {
		t.Fatal("expiry changed active Skill", got)
	}
	if err := b.ForgetObservation(t.Context(), operation); err != nil {
		t.Fatal("expiry cleanup is not idempotent", err)
	}
}
