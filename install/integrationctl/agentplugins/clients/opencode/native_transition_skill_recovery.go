package opencode

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func transitionSkillDigest(root, path string) (string, bool, error) {
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return "", false, err
	}
	digest, err := shared.DigestSkillDirectory(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	return digest, err == nil, err
}
func verifyTransitionTargetSkills(r transitionRecord) error {
	for _, s := range r.Skills {
		if s.Staged != "" {
			_, exists, err := transitionSkillDigest(r.Root, s.Staged)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("target skill has no completed exclusive staging move")
			}
		}
		digest, exists, err := transitionSkillDigest(r.Identity.NativeRoot, s.Target)
		if err != nil {
			return err
		}
		if s.New == nil {
			if exists {
				return fmt.Errorf("removed transition skill is occupied")
			}
		} else if !exists || digest != s.New.ManagedDigest {
			return fmt.Errorf("target transition skill differs from receipt")
		}
	}
	return nil
}

func restoreTransitionSkills(r transitionRecord) error {
	for _, skill := range r.Skills {
		if err := restoreTransitionSkill(r, skill); err != nil {
			return err
		}
	}
	return nil
}

func restoreTransitionSkill(r transitionRecord, s transitionSkill) error {
	current, exists, err := transitionSkillDigest(r.Identity.NativeRoot, s.Target)
	if err != nil {
		return err
	}
	backup, backupExists, err := transitionSkillDigest(r.Root, s.Backup)
	if err != nil {
		return err
	}
	if backupExists && (s.Old == nil || !s.OldExists || backup != s.Old.ManagedDigest) {
		return fmt.Errorf("transition backup differs from source intent")
	}
	if s.OldExists && exists && current == s.Old.ManagedDigest && !backupExists {
		return nil
	}
	if exists {
		if err := removeTransitionTargetForRestore(r, s, current, backupExists); err != nil {
			return err
		}
	}
	return restoreTransitionSkillBackup(r, s, backupExists)
}

func removeTransitionTargetForRestore(r transitionRecord, s transitionSkill, current string, backupExists bool) error {
	if s.Staged != "" {
		_, stagedExists, err := transitionSkillDigest(r.Root, s.Staged)
		if err != nil {
			return err
		}
		if stagedExists {
			return fmt.Errorf("preserve equal foreign skill; staged move never completed")
		}
	}
	if s.New == nil || current != s.New.ManagedDigest || s.OldExists && !backupExists {
		return fmt.Errorf("preserve edited transition skill")
	}
	if err := os.RemoveAll(s.Target); err != nil {
		return err
	}
	return atomicfile.SyncDirectory(filepath.Dir(s.Target))
}

func restoreTransitionSkillBackup(r transitionRecord, s transitionSkill, backupExists bool) error {
	if s.OldExists {
		if !backupExists {
			return fmt.Errorf("source skill backup missing")
		}
		if err := renameOpenCodeDirectoryNoReplace(s.Backup, s.Target, shared.RenameDirectoryExclusive); err != nil {
			return err
		}
		if err := atomicfile.SyncDirectory(filepath.Dir(s.Target)); err != nil {
			return err
		}
		if err := atomicfile.SyncDirectory(r.Root); err != nil {
			return err
		}
	} else if backupExists {
		return fmt.Errorf("unexpected source skill backup")
	}
	return nil
}

func cleanupTransitionPayload(root, path string, object *domain.NativeObjectOwnership) error {
	if path == "" {
		return nil
	}
	digest, exists, err := transitionSkillDigest(root, path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if object == nil || digest != object.ManagedDigest {
		return fmt.Errorf("preserve changed transition residue")
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if err := atomicfile.SyncDirectory(root); err != nil {
		return err
	}
	return nil
}

func cleanupTransition(r transitionRecord) error {
	// All owned payloads are proved before destructive cleanup. Unknown residue
	// is retained; an edited backup is never deleted by a path-only RemoveAll.
	for _, s := range r.Skills {
		for _, item := range []struct {
			path   string
			object *domain.NativeObjectOwnership
		}{{s.Backup, s.Old}, {s.Staged, s.New}} {
			if err := cleanupTransitionPayload(r.Root, item.path, item.object); err != nil {
				return err
			}
		}
	}
	entries, err := os.ReadDir(r.Root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != transitionRecordFile {
			return fmt.Errorf("preserve unrecognized transition residue")
		}
	}
	if err := os.Remove(filepath.Join(r.Root, transitionRecordFile)); err != nil {
		return err
	}
	if err := atomicfile.SyncDirectory(r.Root); err != nil {
		return err
	}
	if err := os.Remove(r.Root); err != nil {
		return err
	}
	return atomicfile.SyncDirectory(filepath.Dir(r.Root))
}
