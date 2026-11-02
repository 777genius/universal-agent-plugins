package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

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
