package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

// ForgetObservation is called only for an expired settled observation while the
// transport holds its acceptance lock. It removes verified private undo data,
// never an active Skill, and refuses ambiguous or user-modified backup contents.
func (b *ManagementBackend) ForgetObservation(ctx context.Context, operation management.Operation) error {
	if operation.Action != management.LocalSkill {
		return nil
	}
	if !localSkillSHA(operation.OperationId) || operation.LocalSkill == nil {
		return errors.New("local observation lacks its retained identity")
	}
	unlock, err := b.lockLocalSkill(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	if _, err = b.localSkillManager().Load(operation.OperationId); err == nil {
		return errors.New("local recovery journal must be resolved before undo expiry")
	} else if !os.IsNotExist(err) {
		return err
	}
	receipt, err := b.loadLocalSkillReceipt(operation.OperationId)
	if err != nil {
		return err
	}
	if receipt != nil && (receipt.RootID != operation.LocalSkill.RootId || receipt.SkillName != operation.LocalSkill.SkillName) {
		return errors.New("local undo identity differs from accepted observation")
	}
	if err = b.forgetLocalSkillBackup(ctx, operation.OperationId, receipt); err != nil {
		return err
	}
	if receipt == nil {
		return nil
	}
	if err = os.Remove(b.localSkillReceiptPath(operation.OperationId)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return atomicfile.SyncDirectory(filepath.Dir(b.localSkillReceiptPath(operation.OperationId)))
}
func (b *ManagementBackend) forgetLocalSkillBackup(ctx context.Context, id string, receipt *localSkillReceipt) error {
	directory := b.localSkillBackupRoot()
	if err := pathpolicy.RequireContainedChild(b.config.StateRoot, directory); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	identity, err := root.Lstat(id)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if receipt == nil || !receipt.BeforeExists || !identity.IsDir() || identity.Mode()&os.ModeSymlink != 0 {
		return errors.New("private backup lacks a matching retained receipt")
	}
	observed, err := readLocalSkillTree(root, id, false)
	if err != nil || observed.digest() != receipt.BeforeDigest {
		return errors.New("private backup changed; preserve it")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	current, err := root.Lstat(id)
	if err != nil || !os.SameFile(current, identity) {
		return errors.New("private backup identity changed; preserve it")
	}
	if err = root.RemoveAll(id); err != nil {
		return err
	}
	return syncLocalSkillRoot(root, ".")
}
