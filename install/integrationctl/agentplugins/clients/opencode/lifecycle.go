package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateNative(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeMutation(env, clients.Ops{}, request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects)
}

func DeactivateNative(ctx context.Context, env clients.Env, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeMutation(env, clients.Ops{}, request.Client.ConfigRoot, "", request.NativeObjects, nil)
}

func applyOpenCodeNative(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyOpenCodeNativeMutation(clients.Env{NativeConfig: nativeconfig.New()}, clients.Ops{}, configRoot, activePath, previous, desired)
}

func applyOpenCodeNativeMutation(env clients.Env, ops clients.Ops, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (resultErr error) {
	rename := ops.Rename
	if rename == nil {
		rename = shared.RenameDirectoryExclusive
	}
	removeAll := ops.RemoveAll
	if removeAll == nil {
		removeAll = os.RemoveAll
	}
	kernel := env.NativeConfig
	if rename == nil {
		return fmt.Errorf("OpenCode rename operation is unavailable")
	}
	if removeAll == nil {
		return fmt.Errorf("OpenCode cleanup operation is unavailable")
	}
	if strings.TrimSpace(configRoot) == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("OpenCode config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	projection := openCodeProjection{}
	var err error
	if len(desired) > 0 {
		projection, err = readOpenCodeProjection(activePath)
		if err != nil {
			return err
		}
		if err := validateOpenCodeProjection(configRoot, activePath, projection); err != nil {
			return err
		}
	}
	if err := preflightOpenCodeObjects(configRoot, activePath, projection, previous, desired); err != nil {
		return err
	}
	expectedConfig := projection.ConfigPath
	if expectedConfig == "" {
		for _, object := range previous {
			if object.Kind == openCodeMCPObjectKind {
				expectedConfig = object.Path
				break
			}
		}
	}
	if expectedConfig != "" {
		selected, err := selectOpenCodeConfig(filepath.Join(configRoot, "opencode.json"), filepath.Join(configRoot, "opencode.jsonc"))
		if err != nil {
			return err
		}
		jsonExists, jsoncExists, err := openCodeConfigPresence(configRoot)
		if err != nil {
			return err
		}
		if !shared.SameCleanPath(selected, expectedConfig) {
			if len(desired) > 0 || jsonExists || jsoncExists {
				return fmt.Errorf("OpenCode config selection changed after staging; rerun the operation")
			}
			// Removal is already complete when both exact config variants are
			// absent. Do not recreate either path just to remove an absent entry.
		}
	}
	requests, err := openCodeMCPRequests(projection, previous, desired)
	if err != nil {
		return err
	}
	skills, err := installOpenCodeSkillsWithOps(configRoot, activePath, previous, desired, rename, removeAll)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if err := skills.rollback(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("rollback OpenCode skills: %w", err))
			}
		}
	}()
	if len(requests) > 0 {
		receipts, applyErr := kernel.ApplyBatch(requests)
		if applyErr != nil && !nativeconfig.IsCommittedCleanup(applyErr) {
			return applyErr
		}
		if len(receipts) != len(requests) {
			if applyErr != nil {
				return errors.Join(applyErr, fmt.Errorf("OpenCode native config committed without complete receipts"))
			}
			return fmt.Errorf("OpenCode native config returned incomplete receipts")
		}
		if applyErr != nil {
			// ApplyBatch promises that typed committed-cleanup failures include the
			// receipts for bytes already written. Preserve both those bytes and the
			// skills installed in the same provider transaction.
			committed = true
		}
		if applyErr == nil {
			committed = true
		}
		desiredByID := shared.ObjectMap(desired)
		for index, request := range requests {
			if request.Action == nativeconfig.ActionRemove {
				continue
			}
			expected := desiredByID["opencode-mcp:"+request.Name]
			if receipts[index].Digest != expected.ManagedDigest || receipts[index].Path != expected.Path {
				return fmt.Errorf("OpenCode native receipt differs from staged ownership")
			}
		}
		skills.commit()
		if applyErr != nil {
			return applyErr
		}
		return nil
	}
	committed = true
	// Config and skills are now externally committed. Transaction-root cleanup
	// is best effort and cannot truthfully turn this into a failed activation:
	// the lifecycle must persist the desired native receipts. A failed cleanup
	// leaves a committed marker in the private transaction root so the residue
	// is distinguishable from rollback recovery and safe to remove later.
	skills.commit()
	return nil
}

type openCodeBackup struct{ backup, target string }

type openCodeRenameFunc func(string, string) error

type openCodeSkillTxn struct {
	root      string
	backups   map[string]openCodeBackup
	installed map[string]domain.NativeObjectOwnership
	rename    openCodeRenameFunc
	removeAll func(string) error
}

func installOpenCodeSkills(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (*openCodeSkillTxn, error) {
	return installOpenCodeSkillsWithOps(configRoot, activePath, previous, desired, shared.RenameDirectoryExclusive, os.RemoveAll)
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
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	if !containsOpenCodeSkill(previous) && !containsOpenCodeSkill(desired) {
		return txn, nil
	}
	skillsRoot := filepath.Join(configRoot, "skills")
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return nil, err
	}
	root, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return nil, err
	}
	txn.root = root
	fail := func(err error) (*openCodeSkillTxn, error) {
		if rollbackErr := txn.rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("%v; OpenCode skill rollback failed: %w; recovery retained at %q", err, rollbackErr, txn.root)
		}
		return nil, err
	}
	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return fail(err)
		}
		target := filepath.Join(root, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fail(err)
		}
		digest, err := shared.DigestSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fail(fmt.Errorf("staged OpenCode skill %q differs from ownership digest", object.LogicalName))
		}
		staged[id] = target
	}
	for id, object := range previousByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fail(err)
		}
		digest, err := shared.DigestSkillDirectory(object.Path)
		if err != nil || digest != object.ManagedDigest {
			return fail(fmt.Errorf("managed OpenCode skill %q changed outside agentplugins", object.LogicalName))
		}
		backup := filepath.Join(root, "old-"+object.LogicalName)
		if err := os.Rename(object.Path, backup); err != nil {
			return fail(err)
		}
		txn.backups[id] = openCodeBackup{backup: backup, target: object.Path}
	}
	for id, object := range desiredByID {
		if object.Kind != openCodeSkillKind {
			continue
		}
		if err := renameOpenCodeDirectoryNoReplace(staged[id], object.Path, rename); err != nil {
			return fail(err)
		}
		txn.installed[id] = object
	}
	return txn, nil
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
			if err := renameOpenCodeDirectoryNoReplace(backup.backup, backup.target, txn.rename); err != nil {
				if result == nil {
					result = err
				}
			} else {
				delete(txn.backups, id)
			}
		}
	}
	for id, backup := range txn.backups {
		if _, err := os.Lstat(backup.target); os.IsNotExist(err) {
			if err := renameOpenCodeDirectoryNoReplace(backup.backup, backup.target, txn.rename); err != nil {
				if result == nil {
					result = err
				}
			} else {
				delete(txn.backups, id)
			}
		} else if err != nil {
			if result == nil {
				result = err
			}
		} else if result == nil {
			result = fmt.Errorf("preserve OpenCode skill backup %s: target is occupied", backup.backup)
		}
	}
	if txn.root != "" && len(txn.backups) == 0 {
		if err := os.RemoveAll(txn.root); err != nil && result == nil {
			result = err
		}
	}
	return result
}

func (txn *openCodeSkillTxn) commit() {
	if txn.root == "" {
		return
	}
	marker := filepath.Join(txn.root, ".agentplugins-committed")
	_ = atomicfile.Write(marker, []byte("OpenCode native config and skills committed; this transaction residue is safe to remove.\n"), 0o600)
	_ = txn.removeAll(txn.root)
}
