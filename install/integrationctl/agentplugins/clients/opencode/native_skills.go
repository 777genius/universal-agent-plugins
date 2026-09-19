package opencode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type openCodeBackup struct{ backup, target string }
type openCodeRenameFunc func(string, string) error
type openCodeSkillTxn struct {
	root      string
	backups   map[string]openCodeBackup
	installed map[string]domain.NativeObjectOwnership
	rename    openCodeRenameFunc
	removeAll func(string) error
}

func renameOpenCodeDirectoryNoReplace(oldPath, newPath string, rename openCodeRenameFunc) error {
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", newPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return rename(oldPath, newPath)
}

func installOpenCodeSkillsWithOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename openCodeRenameFunc, removeAll func(string) error) (*openCodeSkillTxn, error) {
	if rename == nil {
		return nil, fmt.Errorf("OpenCode rename operation is unavailable")
	}
	if removeAll == nil {
		return nil, fmt.Errorf("OpenCode cleanup operation is unavailable")
	}
	txn := &openCodeSkillTxn{backups: map[string]openCodeBackup{}, installed: map[string]domain.NativeObjectOwnership{}, rename: rename, removeAll: removeAll}
	if !containsOpenCodeSkill(previous) && !containsOpenCodeSkill(desired) {
		return txn, nil
	}
	if err := txn.createRoot(configRoot); err != nil {
		return nil, err
	}
	if err := txn.stageAndSwap(activePath, previous, desired); err != nil {
		return txn.fail(err)
	}
	return txn, nil
}

func (txn *openCodeSkillTxn) createRoot(configRoot string) error {
	skillsRoot := filepath.Join(configRoot, "skills")
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return err
	}
	root, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return err
	}
	txn.root = root
	return nil
}

func (txn *openCodeSkillTxn) fail(err error) (*openCodeSkillTxn, error) {
	if rollbackErr := txn.rollback(); rollbackErr != nil {
		return nil, fmt.Errorf("%w; OpenCode skill rollback failed: %w; recovery retained at %q", err, rollbackErr, txn.root)
	}
	return nil, err
}

func (txn *openCodeSkillTxn) stageAndSwap(activePath string, previous, desired []domain.NativeObjectOwnership) error {
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	staged, err := txn.stageDesired(activePath, desiredByID)
	if err != nil {
		return err
	}
	if err := txn.backupPrevious(previousByID); err != nil {
		return err
	}
	return txn.installStaged(staged, desiredByID)
}

func (txn *openCodeSkillTxn) stageDesired(activePath string, desiredByID map[string]domain.NativeObjectOwnership) (map[string]string, error) {
	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return nil, err
		}
		target := filepath.Join(txn.root, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return nil, err
		}
		digest, err := shared.DigestSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return nil, fmt.Errorf("staged OpenCode skill %q differs from ownership digest", object.LogicalName)
		}
		staged[id] = target
	}
	return staged, nil
}

func (txn *openCodeSkillTxn) backupPrevious(previousByID map[string]domain.NativeObjectOwnership) error {
	for id, object := range previousByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		digest, err := shared.DigestSkillDirectory(object.Path)
		if err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("managed OpenCode skill %q changed outside agentplugins", object.LogicalName)
		}
		backup := filepath.Join(txn.root, "old-"+object.LogicalName)
		if err := os.Rename(object.Path, backup); err != nil {
			return err
		}
		txn.backups[id] = openCodeBackup{backup: backup, target: object.Path}
	}
	return nil
}

func (txn *openCodeSkillTxn) installStaged(staged map[string]string, desiredByID map[string]domain.NativeObjectOwnership) error {
	for id, object := range desiredByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		if err := renameOpenCodeDirectoryNoReplace(staged[id], object.Path, txn.rename); err != nil {
			return err
		}
		txn.installed[id] = object
	}
	return nil
}

func containsOpenCodeSkill(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == openCodeSkillKind {
			return true
		}
	}
	return false
}

func (txn *openCodeSkillTxn) rollback() error {
	result := txn.removeInstalled()
	if err := txn.restoreRemainingBackups(); err != nil && result == nil {
		result = err
	}
	if txn.root != "" && len(txn.backups) == 0 {
		if err := os.RemoveAll(txn.root); err != nil && result == nil {
			result = err
		}
	}
	return result
}

func (txn *openCodeSkillTxn) removeInstalled() error {
	var result error
	for id, object := range txn.installed {
		digest, err := shared.DigestSkillDirectory(object.Path)
		if err == nil && digest == object.ManagedDigest {
			err = os.RemoveAll(object.Path)
		}
		if err != nil && !os.IsNotExist(err) && result == nil {
			result = err
		}
		if backup, ok := txn.backups[id]; ok {
			if err := txn.restoreBackup(id, backup); err != nil && result == nil {
				result = err
			}
		}
	}
	return result
}

func (txn *openCodeSkillTxn) restoreBackup(id string, backup openCodeBackup) error {
	if err := renameOpenCodeDirectoryNoReplace(backup.backup, backup.target, txn.rename); err != nil {
		return err
	}
	delete(txn.backups, id)
	return nil
}

func (txn *openCodeSkillTxn) restoreRemainingBackups() error {
	var result error
	for id, backup := range txn.backups {
		if err := txn.restoreRemainingBackup(id, backup); err != nil && result == nil {
			result = err
		}
	}
	return result
}

func (txn *openCodeSkillTxn) restoreRemainingBackup(id string, backup openCodeBackup) error {
	if _, err := os.Lstat(backup.target); os.IsNotExist(err) {
		if err := renameOpenCodeDirectoryNoReplace(backup.backup, backup.target, txn.rename); err != nil {
			return err
		}
		delete(txn.backups, id)
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("preserve OpenCode skill backup %s: target is occupied", backup.backup)
}

func (txn *openCodeSkillTxn) commit() {
	if txn.root == "" {
		return
	}
	marker := filepath.Join(txn.root, ".agentplugins-committed")
	_ = atomicfile.Write(marker, []byte("OpenCode native config and skills committed; this transaction residue is safe to remove.\n"), 0o600)
	_ = txn.removeAll(txn.root)
}
