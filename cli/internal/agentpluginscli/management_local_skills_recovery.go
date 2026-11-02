package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

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
