package shared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectSkillSwap struct {
	mutation        ProjectComponentMutation
	root            string
	identity        os.FileInfo
	known           map[string]domain.NativeObjectOwnership
	staged, backups map[string]string
	installed       map[string]domain.NativeObjectOwnership
	unchanged       map[string]bool
}

func newProjectSkillSwap(mutation ProjectComponentMutation) *projectSkillSwap {
	return &projectSkillSwap{mutation: mutation, known: map[string]domain.NativeObjectOwnership{},
		staged: map[string]string{}, backups: map[string]string{}, installed: map[string]domain.NativeObjectOwnership{}, unchanged: map[string]bool{}}
}

func (txn *projectSkillSwap) create() error {
	if txn.root != "" {
		return txn.ownedRoot()
	}
	layout := txn.mutation.Layout
	if err := pathpolicy.RequireContainedChild(layout.ProjectRoot, layout.SkillsRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(layout.SkillsRoot, 0700); err != nil {
		return err
	}
	root, err := os.MkdirTemp(layout.SkillsRoot, ".agentplugins-native-")
	if err != nil {
		return err
	}
	txn.root = root
	txn.identity, err = os.Lstat(root)
	return err
}

func (txn *projectSkillSwap) stage(ctx context.Context) error {
	previous := ObjectMap(txn.mutation.Previous)
	for _, object := range projectSkillObjects(txn.mutation) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := txn.stageSkill(object, previous); err != nil {
			return err
		}
	}
	for _, object := range txn.mutation.Previous {
		if object.Kind == txn.mutation.Layout.SkillKind && !txn.unchanged[object.ObjectID] {
			if err := txn.create(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (txn *projectSkillSwap) stageSkill(object domain.NativeObjectOwnership, previous map[string]domain.NativeObjectOwnership) error {
	source := filepath.Join(txn.mutation.ActivePath, filepath.FromSlash(object.SourceRelative))
	if err := pathpolicy.RequireContainedChild(txn.mutation.ActivePath, source); err != nil {
		return err
	}
	sourceObject := object
	sourceObject.Path = source
	if err := verifyComponentSkill(sourceObject, false); err != nil {
		return err
	}
	if prior, ok := previous[object.ObjectID]; ok && prior.ManagedDigest == object.ManagedDigest {
		if _, err := os.Lstat(object.Path); err == nil {
			if err := verifyComponentSkill(object, false); err != nil {
				return err
			}
			txn.unchanged[object.ObjectID] = true
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := txn.create(); err != nil {
		return err
	}
	name := "new-" + object.LogicalName
	target := filepath.Join(txn.root, name)
	txn.known[name] = object
	if err := filetree.CopyDir(source, target); err != nil {
		return err
	}
	staged := object
	staged.Path = target
	if err := verifyComponentSkill(staged, false); err != nil {
		return err
	}
	txn.staged[object.ObjectID] = name
	return nil
}

func (txn *projectSkillSwap) swap(ctx context.Context) error {
	if txn.root != "" {
		if err := txn.ownedRoot(); err != nil {
			return err
		}
	}
	for _, object := range txn.mutation.Previous {
		if object.Kind != txn.mutation.Layout.SkillKind || txn.unchanged[object.ObjectID] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		name := "old-" + object.LogicalName
		if err := pathpolicy.RequireContainedChild(txn.mutation.Layout.ProjectRoot, object.Path); err != nil {
			return err
		}
		if err := RenameDirectoryExclusive(object.Path, filepath.Join(txn.root, name)); err != nil {
			return err
		}
		txn.backups[object.ObjectID], txn.known[name] = name, object
		isolated := object
		isolated.Path = filepath.Join(txn.root, name)
		if err := verifyComponentSkill(isolated, false); err != nil {
			return err
		}
	}
	desired := ObjectMap(txn.mutation.Desired)
	for id, name := range txn.staged {
		if err := ctx.Err(); err != nil {
			return err
		}
		object := desired[id]
		if err := pathpolicy.RequireContainedChild(txn.mutation.Layout.ProjectRoot, object.Path); err != nil {
			return err
		}
		if err := RenameDirectoryExclusive(filepath.Join(txn.root, name), object.Path); err != nil {
			return err
		}
		delete(txn.known, name)
		txn.installed[id] = object
	}
	return nil
}

func (txn *projectSkillSwap) restore() error {
	if txn.root != "" {
		if err := txn.ownedRoot(); err != nil {
			return err
		}
	}
	var failures []error
	for _, object := range txn.installed {
		if err := txn.removeInstalled(object); err != nil {
			failures = append(failures, err)
		}
	}
	previous := ObjectMap(txn.mutation.Previous)
	for id, name := range txn.backups {
		object := previous[id]
		isolated := object
		isolated.Path = filepath.Join(txn.root, name)
		if err := txn.ownedRoot(); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := verifyComponentSkill(isolated, false); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := pathpolicy.RequireContainedChild(txn.mutation.Layout.ProjectRoot, object.Path); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := RenameDirectoryExclusive(isolated.Path, object.Path); err != nil {
			failures = append(failures, err)
			continue
		}
		delete(txn.known, name)
	}
	return errors.Join(failures...)
}

func (txn *projectSkillSwap) removeInstalled(object domain.NativeObjectOwnership) error {
	if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
		return nil
	}
	if err := pathpolicy.RequireContainedChild(txn.mutation.Layout.ProjectRoot, object.Path); err != nil {
		return err
	}
	if err := verifyComponentSkill(object, false); err != nil {
		return err
	}
	return os.RemoveAll(object.Path)
}

func (txn *projectSkillSwap) ownedRoot() error {
	if err := pathpolicy.RequireContainedChild(txn.mutation.Layout.ProjectRoot, txn.root); err != nil {
		return err
	}
	observed, err := os.Lstat(txn.root)
	if err != nil {
		return err
	}
	if txn.identity == nil || !os.SameFile(txn.identity, observed) {
		return fmt.Errorf("private project skill transaction identity changed")
	}
	return nil
}

func (txn *projectSkillSwap) cleanup() error {
	if txn.root == "" {
		return nil
	}
	if err := txn.validateCleanup(); err != nil {
		return fmt.Errorf("preserved project skill recovery at %q: %w", txn.root, err)
	}
	for name := range txn.known {
		if err := os.RemoveAll(filepath.Join(txn.root, name)); err != nil {
			return fmt.Errorf("cleanup project skill recovery at %q: %w", txn.root, err)
		}
	}
	return os.Remove(txn.root)
}

func (txn *projectSkillSwap) validateCleanup() error {
	if err := txn.ownedRoot(); err != nil {
		return err
	}
	entries, err := os.ReadDir(txn.root)
	if err != nil {
		return err
	}
	if len(entries) != len(txn.known) {
		return fmt.Errorf("private project skill transaction contents changed")
	}
	for _, entry := range entries {
		object, ok := txn.known[entry.Name()]
		if !ok {
			return fmt.Errorf("unknown private project skill transaction entry")
		}
		object.Path = filepath.Join(txn.root, entry.Name())
		if err := verifyComponentSkill(object, false); err != nil {
			return err
		}
	}
	return nil
}
