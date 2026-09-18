package gemini

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type geminiSkillTxn struct {
	rename          geminiRenameFunc
	activePath      string
	transactionRoot string
	cleanup         bool
	staged          map[string]string
	backups         map[string]string
	installed       map[string]domain.NativeObjectOwnership
	previousByID    map[string]domain.NativeObjectOwnership
	desiredByID     map[string]domain.NativeObjectOwnership
}

func newGeminiSkillTxn(prepared *geminiNativeApply) (*geminiSkillTxn, error) {
	txn := &geminiSkillTxn{
		rename: prepared.rename, activePath: prepared.activePath, cleanup: true,
		staged: map[string]string{}, backups: map[string]string{},
		installed:    map[string]domain.NativeObjectOwnership{},
		previousByID: prepared.previousByID, desiredByID: prepared.desiredByID,
	}
	if !hasGeminiSkillObjects(prepared.previous) && !hasGeminiSkillObjects(prepared.desired) {
		return txn, nil
	}
	skillsRoot := filepath.Join(prepared.configRoot, "skills")
	if err := pathpolicy.RequireContainedChild(prepared.configRoot, skillsRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create Gemini skills root: %w", err)
	}
	root, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return nil, fmt.Errorf("create Gemini native transaction: %w", err)
	}
	txn.transactionRoot = root
	return txn, nil
}

func (txn *geminiSkillTxn) stageDesired() error {
	for id, object := range txn.desiredByID {
		if object.Kind != GeminiSkillObjectKind {
			continue
		}
		source := filepath.Join(txn.activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(txn.activePath, source); err != nil {
			return fmt.Errorf("unsafe Gemini skill source for %q: %w", object.LogicalName, err)
		}
		target := filepath.Join(txn.transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fmt.Errorf("stage Gemini skill %q: %w", object.LogicalName, err)
		}
		if digest, err := shared.DigestSkillDirectory(target); err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Gemini skill %q does not match its ownership digest", object.LogicalName)
		}
		txn.staged[id] = target
	}
	return nil
}

func (txn *geminiSkillTxn) backupPrevious() error {
	for id, object := range txn.previousByID {
		if object.Kind != GeminiSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(txn.transactionRoot, "old-"+object.LogicalName)
		if err := txn.rename(object.Path, backup); err != nil {
			return fmt.Errorf("backup Gemini skill %q: %w", object.LogicalName, err)
		}
		txn.backups[id] = backup
		if err := txn.verifyIsolatedBackup(object, backup); err != nil {
			return err
		}
	}
	return nil
}

func (txn *geminiSkillTxn) verifyIsolatedBackup(object domain.NativeObjectOwnership, backup string) error {
	digest, digestErr := shared.DigestSkillDirectory(backup)
	if digestErr != nil {
		return fmt.Errorf("verify isolated Gemini skill backup %q: %w", object.LogicalName, digestErr)
	}
	if digest != object.ManagedDigest {
		return fmt.Errorf("isolated Gemini skill backup %q changed outside agentplugins", object.LogicalName)
	}
	return nil
}

func (txn *geminiSkillTxn) installStaged() error {
	for id, source := range txn.staged {
		object := txn.desiredByID[id]
		if err := RenameGeminiDirectoryNoReplace(source, object.Path, txn.rename); err != nil {
			return fmt.Errorf("activate Gemini skill %q: %w", object.LogicalName, err)
		}
		txn.installed[id] = object
	}
	return nil
}

func (txn *geminiSkillTxn) rollback() error {
	var rollbackErr error
	attempted := map[string]bool{}
	for id, object := range txn.installed {
		attempted[id] = true
		if err := txn.removeManagedInstall(object); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
		if err := txn.restoreSkillBackup(id, object.Path); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
	}
	for id := range txn.backups {
		if attempted[id] {
			continue
		}
		if err := txn.restoreSkillBackup(id, txn.previousByID[id].Path); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
	}
	return rollbackErr
}

func (txn *geminiSkillTxn) removeManagedInstall(object domain.NativeObjectOwnership) error {
	digest, digestErr := shared.DigestSkillDirectory(object.Path)
	if digestErr == nil && digest == object.ManagedDigest {
		return os.RemoveAll(object.Path)
	}
	return nil
}

func (txn *geminiSkillTxn) restoreSkillBackup(id, dest string) error {
	backup := txn.backups[id]
	if backup == "" {
		return nil
	}
	if err := RenameGeminiDirectoryNoReplace(backup, dest, txn.rename); err != nil {
		return err
	}
	delete(txn.backups, id)
	return nil
}
