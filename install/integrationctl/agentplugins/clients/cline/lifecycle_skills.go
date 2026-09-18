package cline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type clineSkillTxn struct {
	rename          clineRenameFunc
	activePath      string
	transactionRoot string
	cleanup         bool
	staged          map[string]string
	backups         map[string]string
	installed       map[string]domain.NativeObjectOwnership
	previousByID    map[string]domain.NativeObjectOwnership
	desiredByID     map[string]domain.NativeObjectOwnership
}

func newClineSkillTxn(prepared *clineNativeApply) (*clineSkillTxn, error) {
	skillsRoot := filepath.Join(prepared.configRoot, "skills")
	if err := pathpolicy.RequireContainedChild(prepared.configRoot, skillsRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create Cline skills root: %w", err)
	}
	transactionRoot, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return nil, err
	}
	return &clineSkillTxn{
		rename: prepared.rename, activePath: prepared.activePath, transactionRoot: transactionRoot, cleanup: true,
		staged: map[string]string{}, backups: map[string]string{},
		installed:    map[string]domain.NativeObjectOwnership{},
		previousByID: prepared.previousByID, desiredByID: prepared.desiredByID,
	}, nil
}

func (txn *clineSkillTxn) stageDesired() error {
	for id, object := range txn.desiredByID {
		if object.Kind != ClineSkillObjectKind {
			continue
		}
		if txn.activePath == "" {
			return fmt.Errorf("active package path is required for Cline skill installation")
		}
		source := filepath.Join(txn.activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(txn.activePath, source); err != nil {
			return err
		}
		target := filepath.Join(txn.transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return err
		}
		digest, err := shared.DigestSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Cline skill %q does not match its ownership digest", object.LogicalName)
		}
		txn.staged[id] = target
	}
	return nil
}

func (txn *clineSkillTxn) backupAndInstall() error {
	if err := txn.backupPrevious(); err != nil {
		return err
	}
	return txn.installStaged()
}

func (txn *clineSkillTxn) backupPrevious() error {
	for id, object := range txn.previousByID {
		if object.Kind != ClineSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(txn.transactionRoot, "old-"+object.LogicalName)
		if err := txn.rename(object.Path, backup); err != nil {
			return err
		}
		txn.backups[id] = backup
		digest, digestErr := shared.DigestSkillDirectory(backup)
		if digestErr != nil || digest != object.ManagedDigest {
			if digestErr != nil {
				return fmt.Errorf("verify isolated Cline skill backup %q: %w", object.LogicalName, digestErr)
			}
			return fmt.Errorf("isolated Cline skill backup %q changed outside agentplugins", object.LogicalName)
		}
	}
	return nil
}

func (txn *clineSkillTxn) installStaged() error {
	for id, source := range txn.staged {
		object := txn.desiredByID[id]
		if err := RenameClineDirectoryNoReplace(source, object.Path, txn.rename); err != nil {
			return err
		}
		txn.installed[id] = object
	}
	return nil
}

func (txn *clineSkillTxn) rollback() error {
	var first error
	attempted := map[string]bool{}
	for id, object := range txn.installed {
		attempted[id] = true
		if err := txn.removeManagedInstall(object); err != nil && first == nil {
			first = err
		}
		if err := txn.restoreSkillBackup(id, object.Path); err != nil && first == nil {
			first = err
		}
	}
	for id := range txn.backups {
		if attempted[id] {
			continue
		}
		if err := txn.restoreSkillBackup(id, txn.previousByID[id].Path); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (txn *clineSkillTxn) removeManagedInstall(object domain.NativeObjectOwnership) error {
	digest, digestErr := shared.DigestSkillDirectory(object.Path)
	if digestErr == nil && digest == object.ManagedDigest {
		return os.RemoveAll(object.Path)
	}
	return nil
}

func (txn *clineSkillTxn) restoreSkillBackup(id, dest string) error {
	backup := txn.backups[id]
	if backup == "" {
		return nil
	}
	if err := RenameClineDirectoryNoReplace(backup, dest, txn.rename); err != nil {
		return err
	}
	delete(txn.backups, id)
	return nil
}
