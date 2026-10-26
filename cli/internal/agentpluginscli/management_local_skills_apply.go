package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// The existing dirswap manager owns physical publication, crash receipts and
// rollback. This small record preserves user-visible undo data after commit; it
// is never installation ownership and is written before any active mutation.
type localSkillReceipt struct {
	Version      int    `json:"version"`
	OperationID  string `json:"operationId"`
	RootID       string `json:"rootId"`
	RootPath     string `json:"rootPath"`
	SkillName    string `json:"skillName"`
	BeforeDigest string `json:"beforeDigest"`
	AfterDigest  string `json:"afterDigest"`
	BeforeExists bool   `json:"beforeExists"`
	BackupName   string `json:"backupName"`
	Phase        string `json:"phase"`
}

func (b *ManagementBackend) localSkillManager() dirswap.Manager {
	return dirswap.Manager{JournalDir: filepath.Join(b.config.StateRoot, "local-skills", "journals")}
}
func (b *ManagementBackend) localSkillReceiptPath(id string) string {
	return filepath.Join(b.config.StateRoot, "local-skills", "receipts", id+".json")
}
func (b *ManagementBackend) saveLocalSkillReceipt(receipt localSkillReceipt) error {
	body, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	if !localSkillSHA(receipt.OperationID) {
		return errors.New("invalid local receipt identity")
	}
	if err = pathcontract.RequireContainedChild(b.config.StateRoot, b.localSkillReceiptPath(receipt.OperationID)); err != nil {
		return err
	}
	parent := filepath.Dir(b.localSkillReceiptPath(receipt.OperationID))
	if err = os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	if err = os.Chmod(parent, 0700); err != nil {
		return err
	}
	return atomicfile.Write(b.localSkillReceiptPath(receipt.OperationID), append(body, '\n'), 0600)
}
func (b *ManagementBackend) readLocalSkillReceipt(id string, root management.LocalSkillRoot, name string) (*localSkillReceipt, error) {
	receipt, err := b.loadLocalSkillReceipt(id)
	if err != nil || receipt == nil {
		return receipt, err
	}
	if receipt.RootID != root.Id || receipt.RootPath != root.Path || receipt.SkillName != name {
		return nil, localSkillRefusal("RECOVERY_CONTEXT_MISMATCH", "The retained backup belongs to a different captured Skill")
	}
	return receipt, nil
}
func (b *ManagementBackend) loadLocalSkillReceipt(id string) (*localSkillReceipt, error) {
	if !localSkillSHA(id) {
		return nil, localSkillRefusal("INVALID_REQUEST", "Restore requires a retained local operation identity")
	}
	filename := b.localSkillReceiptPath(id)
	if err := pathcontract.RequireContainedChild(b.config.StateRoot, filename); err != nil {
		return nil, err
	}
	// Refuse nonregular files before opening; a named pipe could block the open.
	info, err := os.Lstat(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return nil, errors.New("invalid local Skill recovery record")
	}
	file, err := os.OpenInRoot(b.config.StateRoot, filepath.Join("local-skills", "receipts", id+".json"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return nil, errors.New("invalid local Skill recovery record")
	}
	body, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil {
		return nil, err
	}
	if len(body) > 65536 {
		return nil, errors.New("invalid local Skill recovery record")
	}
	var receipt localSkillReceipt
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&receipt); err != nil {
		return nil, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, errors.New("invalid local Skill recovery record")
	}
	if !validLocalSkillReceipt(receipt, id) {
		return nil, localSkillRefusal("RECOVERY_CONTEXT_MISMATCH", "The retained backup belongs to a different captured Skill")
	}
	return &receipt, nil
}
func validLocalSkillReceipt(receipt localSkillReceipt, id string) bool {
	return receipt.Version == 1 && receipt.OperationID == id && receipt.RootID != "" && filepath.IsAbs(receipt.RootPath) && filepath.Clean(receipt.RootPath) == receipt.RootPath && localSkillName(receipt.SkillName) && receipt.BackupName == id && localSkillSHA(receipt.BeforeDigest) && localSkillSHA(receipt.AfterDigest) && (receipt.Phase == "prepared" || receipt.Phase == "committed")
}
func (p *managementLocalSkill) validateRestoreSnapshot() error {
	manager := p.backend.localSkillManager()
	journal, err := manager.Load(p.input.RestoreOperationId)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil && (journal.OwnedBase != p.root.Path || journal.ActivePath != filepath.Join(p.root.Path, p.input.SkillName)) {
		return localSkillRefusal("RECOVERY_CONTEXT_MISMATCH", "The local receipt belongs to a different captured Skill")
	}
	if p.restored == nil {
		if err == nil {
			return localSkillRefusal("RECOVERY_REQUIRED", "Preserve the local receipt; its review metadata is unavailable")
		}
		return nil // By construction no active mutation can precede both records.
	}
	if p.before.digest() != p.restored.BeforeDigest && p.before.digest() != p.restored.AfterDigest && p.before.Exists {
		return localSkillRefusal("LOCAL_EDIT_CONFLICT", "The Skill changed after the saved operation; preserve its files and review those edits before restoration")
	}
	if p.restored.BeforeExists {
		backup, readErr := localSkillObserve(p.backend.localSkillBackupRoot(), p.restored.BackupName, false)
		if readErr != nil || backup.digest() != p.restored.BeforeDigest {
			return localSkillRefusal("BACKUP_CHANGED", "The retained backup is missing or changed; no local files were restored")
		}
	}
	return nil
}
func (p *managementLocalSkill) Apply(ctx context.Context, progress func(management.Progress)) (management.Outcome, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	unlock, err := p.backend.lockLocalSkill(ctx)
	if err != nil {
		return p.localUnchanged(err)
	}
	defer func() { _ = unlock() }()
	if err = p.check(ctx); err != nil {
		return p.localUnchanged(err)
	}
	p.closed = true
	if progress != nil {
		progress(management.Progress{Phase: "local_skill", Message: "Applying the reviewed local Skill", Total: 1})
	}
	if string(p.input.Action) == "restore" {
		return p.applyLocalRestore(ctx)
	}
	if err = os.MkdirAll(p.root.Path, 0700); err != nil {
		return p.localUnchanged(err)
	}
	root, err := os.OpenRoot(p.root.Path)
	if err != nil {
		return p.localUnchanged(err)
	}
	defer func() { _ = root.Close() }()
	before, err := readLocalSkillTree(root, p.input.SkillName, true)
	if err != nil || before.digest() != p.before.digest() {
		return p.localUnchanged(management.ErrPlanStale)
	}
	desired := localSkillTree{Directories: []localSkillDirectory{}, Files: []localSkillFile{}}
	if string(p.input.Action) == "upsert" {
		desired, err = p.desiredLocalTree(before)
		if err != nil {
			return p.localUnchanged(err)
		}
	}
	return p.publishLocalTree(ctx, root, before, desired)
}
func (p *managementLocalSkill) publishLocalTree(ctx context.Context, root *os.Root, before, desired localSkillTree) (out management.Outcome, resultErr error) {
	if err := ctx.Err(); err != nil {
		return p.localUnchanged(err)
	}
	if before.digest() == desired.digest() {
		return p.localSuccess(false, nil), nil
	}
	receipt, stage, err := p.prepareLocalPublication(before, desired)
	if err != nil {
		return p.localUnchanged(err)
	}
	if stage != nil {
		defer func() {
			if closeErr := stage.close(out.State != management.Interrupted); closeErr != nil {
				out, resultErr = p.localInterrupted(errors.Join(resultErr, closeErr))
			}
		}()
	}
	manager := p.backend.localSkillManager()
	input := p.localSwapInput(ctx, root, before, desired, stage)
	swap, err := manager.Apply(ctx, input)
	if err != nil {
		return p.localSwapFailure(ctx, root, before, swap, err)
	}
	receipt.Phase = "committed"
	if err = p.backend.saveLocalSkillReceipt(receipt); err != nil {
		return p.localInterrupted(err)
	}
	if err = manager.Commit(ctx, swap); err != nil {
		return p.localInterrupted(err)
	}
	observed, err := readLocalSkillTree(root, p.input.SkillName, false)
	if err != nil || observed.digest() != desired.digest() {
		return p.localInterrupted(management.ErrPlanStale)
	}
	return p.localSuccess(true, &receipt), nil
}
func (p *managementLocalSkill) localSwapInput(_ context.Context, root *os.Root, before, desired localSkillTree, stage *localSkillStage) dirswap.Input {
	input := dirswap.Input{OperationID: p.operationID, ClientBindingID: "local-skill", Sequence: 1, OwnedBase: p.root.Path, ActivePath: filepath.Join(p.root.Path, p.input.SkillName), Remove: !desired.Exists, RequireAbsent: !before.Exists}
	if stage != nil {
		input.StagingPath = stage.path
	}
	input.VerifyActive = func(ctx context.Context, physical string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if filepath.Dir(physical) != p.root.Path {
			return management.ErrPlanStale
		}
		observed, err := readLocalSkillTree(root, filepath.Base(physical), false)
		if err != nil || observed.digest() != before.digest() {
			return management.ErrPlanStale
		}
		return nil
	}
	return input
}
func (p *managementLocalSkill) localSwapFailure(ctx context.Context, root *os.Root, before localSkillTree, swap dirswap.Receipt, cause error) (management.Outcome, error) {
	if swap.OperationID == "" {
		return p.localUnchanged(cause)
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := p.backend.localSkillManager().Recover(cleanupCtx, p.operationID, false); err != nil {
		return p.localInterrupted(errors.Join(cause, err))
	}
	observed, err := readLocalSkillTree(root, p.input.SkillName, false)
	if err != nil || observed.digest() != before.digest() {
		return p.localInterrupted(cause)
	}
	return p.localUnchanged(cause)
}
func (b *ManagementBackend) localSkillBackupRoot() string {
	return filepath.Join(b.config.StateRoot, "local-skills", "backups")
}
func (p *managementLocalSkill) prepareLocalPublication(before, desired localSkillTree) (localSkillReceipt, *localSkillStage, error) {
	receipt := localSkillReceipt{Version: 1, OperationID: p.operationID, RootID: p.root.Id, RootPath: p.root.Path, SkillName: p.input.SkillName, BeforeDigest: before.digest(), AfterDigest: desired.digest(), BeforeExists: before.Exists, BackupName: p.operationID, Phase: "prepared"}
	if before.Exists {
		if err := p.backend.backupLocalSkill(p.operationID, before); err != nil {
			return receipt, nil, err
		}
	}
	if err := p.backend.saveLocalSkillReceipt(receipt); err != nil {
		return receipt, nil, err
	}
	if !desired.Exists {
		return receipt, nil, nil
	}
	stage, err := stageLocalSkill(filepath.Dir(p.root.Path), p.operationID, desired)
	return receipt, stage, err
}
func (b *ManagementBackend) backupLocalSkill(id string, tree localSkillTree) error {
	directory := b.localSkillBackupRoot()
	if err := pathcontract.RequireContainedChild(b.config.StateRoot, directory); err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err = writeLocalSkillTree(root, id, tree); err != nil {
		return err
	}
	return syncLocalSkillRoot(root, ".")
}

// Staging lives in the exact parent allowed by dirswap, outside the native
// Skills discovery root. Retained backups live in private engine state.
type localSkillStage struct {
	root       *os.Root
	path, name string
	identity   os.FileInfo
}

func stageLocalSkill(parent, id string, tree localSkillTree) (*localSkillStage, error) {
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	stage := &localSkillStage{root: root, name: ".agentplugins-staging-local-" + id}
	stage.path = filepath.Join(parent, stage.name)
	if err = root.Mkdir(stage.name, 0700); err != nil {
		_ = root.Close()
		return nil, err
	}
	stage.identity, err = root.Lstat(stage.name)
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if err = populateLocalSkillTree(root, stage.name, tree); err != nil {
		return nil, errors.Join(err, stage.close(true))
	}
	if err = syncLocalSkillRoot(root, "."); err != nil {
		return nil, errors.Join(err, stage.close(true))
	}
	return stage, nil
}
func (s *localSkillStage) close(remove bool) error {
	defer func() { _ = s.root.Close() }()
	if !remove {
		return nil
	}
	current, err := s.root.Lstat(s.name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !os.SameFile(current, s.identity) {
		return errors.New("local staging directory changed; preserve its files")
	}
	if err = s.root.RemoveAll(s.name); err != nil {
		return err
	}
	return syncLocalSkillRoot(s.root, ".")
}
func (p *managementLocalSkill) applyLocalRestore(ctx context.Context) (management.Outcome, error) {
	if err := p.validateRestoreSnapshot(); err != nil {
		return p.localUnchanged(err)
	}
	if p.restored == nil {
		out := p.localSuccess(false, nil)
		out.ReconciledOperationIDs = []string{p.input.RestoreOperationId}
		return out, nil
	}
	manager := p.backend.localSkillManager()
	if _, err := manager.Load(p.input.RestoreOperationId); err == nil {
		if err = manager.Recover(ctx, p.input.RestoreOperationId, p.restored.Phase == "committed"); err != nil {
			return p.localInterrupted(err)
		}
	} else if !os.IsNotExist(err) {
		return p.localUnchanged(err)
	}
	root, err := os.OpenRoot(p.root.Path)
	if err != nil {
		return p.localUnchanged(err)
	}
	defer func() { _ = root.Close() }()
	current, err := readLocalSkillTree(root, p.input.SkillName, true)
	if err != nil {
		return p.localInterrupted(err)
	}
	if current.digest() == p.restored.BeforeDigest {
		out := p.localSuccess(p.before.digest() != current.digest(), nil)
		out.ReconciledOperationIDs = []string{p.input.RestoreOperationId}
		return out, nil
	}
	desired := localSkillTree{Directories: []localSkillDirectory{}, Files: []localSkillFile{}}
	if p.restored.BeforeExists {
		desired, err = localSkillObserve(p.backend.localSkillBackupRoot(), p.restored.BackupName, true)
		if err != nil || desired.digest() != p.restored.BeforeDigest {
			return p.localInterrupted(management.ErrPlanStale)
		}
	}
	out, err := p.publishLocalTree(ctx, root, current, desired)
	if err == nil && out.State == management.Succeeded {
		out.ReconciledOperationIDs = []string{p.input.RestoreOperationId}
	}
	return out, err
}
func (p *managementLocalSkill) localUnchanged(err error) (management.Outcome, error) {
	state := management.Failed
	if errors.Is(err, context.Canceled) {
		state = management.Canceled
	}
	out := p.localSuccess(false, nil)
	out.State = state
	out.Result.Outcome = "unchanged"
	out.Result.Message = "Local Skill files were not changed"
	return out, err
}
func (p *managementLocalSkill) localInterrupted(err error) (management.Outcome, error) {
	return management.Outcome{State: management.Interrupted, Diagnostics: []management.Diagnostic{managementDiagnostic("LOCAL_RECOVERY_REQUIRED", "Local Skill completion is uncertain; restore this operation using its retained receipt")}}, err
}
func (p *managementLocalSkill) localSuccess(mutated bool, receipt *localSkillReceipt) management.Outcome {
	result := management.OperationResult{Outcome: "completed", Mutated: mutated, Message: "Reviewed local Skill operation completed", Items: []management.OperationItem{{Alias: p.input.SkillName, TargetId: p.root.TargetId, Status: "completed", DestinationPath: filepath.Join(p.root.Path, p.input.SkillName)}}, NextActions: []management.NextAction{}}
	if !mutated {
		result.Outcome = "unchanged"
	}
	if receipt != nil && receipt.BeforeExists {
		result.LocalSkillBackup = &management.LocalSkillBackup{OperationId: p.operationID, RootId: p.root.Id, SkillName: p.input.SkillName, Path: filepath.Join(p.backend.localSkillBackupRoot(), receipt.BackupName), OriginalAction: management.LocalSkillBackupOriginalAction(p.input.Action)}
	}
	return management.Outcome{State: management.Succeeded, Result: result, Diagnostics: []management.Diagnostic{}}
}
