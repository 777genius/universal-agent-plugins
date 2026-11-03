package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func (p *managementLocalSkill) Check(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	unlock, err := p.backend.lockLocalSkill(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()
	return p.check(ctx)
}

func (p *managementLocalSkill) check(ctx context.Context) error {
	if p.closed {
		return management.ErrPlanStale
	}
	if err := p.backend.localSkillUnowned(ctx, filepath.Join(p.root.Path, p.input.SkillName)); err != nil {
		return err
	}
	physical, err := managementAbsolute(p.root.Path)
	if err != nil || physical != p.root.Path {
		return management.ErrPlanStale
	}
	info, err := os.Stat(p.rootAnchor)
	if err != nil || !os.SameFile(info, p.rootIdentity) {
		return management.ErrPlanStale
	}
	live, err := localSkillObserve(p.root.Path, p.input.SkillName, false)
	if err != nil || live.digest() != p.before.digest() {
		return management.ErrPlanStale
	}
	if string(p.input.Action) == "restore" {
		current, readErr := p.backend.readLocalSkillReceipt(p.input.RestoreOperationId, p.root, p.input.SkillName)
		if readErr != nil || (current == nil) != (p.restored == nil) || current != nil && *current != *p.restored {
			return management.ErrPlanStale
		}
	}
	return nil
}

func (p *managementLocalSkill) verifyChanges(tree localSkillTree) error {
	files := map[string]localSkillFile{}
	for _, file := range tree.Files {
		files[file.Path] = file
	}
	for _, change := range p.changes {
		current, exists := files[change.relative]
		if change.expected == nil && exists || change.expected != nil && (!exists || current.SHA256 != *change.expected) {
			return management.ErrPlanStale
		}
	}
	return nil
}

func localSkillRootAnchor(root string) (string, os.FileInfo, error) {
	for {
		info, err := os.Stat(root)
		if err == nil {
			if !info.IsDir() {
				return "", nil, errors.New("authoring root parent is not a directory")
			}
			return root, info, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", nil, err
		}
		root = parent
	}
}

func localSkillObserve(rootPath, name string, keep bool) (localSkillTree, error) {
	root, err := os.OpenRoot(rootPath)
	if os.IsNotExist(err) {
		return localSkillTree{Directories: []localSkillDirectory{}, Files: []localSkillFile{}}, nil
	}
	if err != nil {
		return localSkillTree{}, err
	}
	defer func() { _ = root.Close() }()
	return readLocalSkillTree(root, name, keep)
}

func (b *ManagementBackend) localSkillUnowned(ctx context.Context, target string) error {
	if b.app.Installer == nil || b.app.StateStore == nil {
		return localSkillRefusal("OWNERSHIP_UNAVAILABLE", "Authoritative installation ownership is unavailable")
	}
	view, err := b.app.Installer.Inspect(ctx)
	if err != nil {
		return err
	}
	if view.Recovery.Required {
		return localSkillRefusal("RECOVERY_REQUIRED", "Recover pending installer work before local authoring")
	}
	state, err := b.app.StateStore.Load()
	if err != nil {
		return err
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if err := localSkillCheckOwnershipPath(target, binding.TargetLocator); err != nil {
				return err
			}
			for _, object := range binding.NativeObjects {
				if object.ProtectionClass == "managed" {
					if err := localSkillCheckOwnershipPath(target, object.Path); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// CLI ownership can retain an alias such as /var or a Windows short path.
// Keep the lexical guard, then compare physical paths even for missing leaves.
func localSkillCheckOwnershipPath(target, owned string) error {
	if !filepath.IsAbs(owned) {
		return nil // A native selector does not describe filesystem ownership.
	}
	if localSkillPathOverlap(target, owned) {
		return localSkillRefusal("MANAGED_SKILL", "This path is managed by UAP; make an editable copy in a separate local directory")
	}
	physical, err := managementAbsolute(owned)
	if err != nil {
		return localSkillRefusal("OWNERSHIP_UNAVAILABLE", "Authoritative installation ownership paths could not be resolved")
	}
	if localSkillPathOverlap(target, physical) {
		return localSkillRefusal("MANAGED_SKILL", "This path is managed by UAP; make an editable copy in a separate local directory")
	}
	return nil
}
