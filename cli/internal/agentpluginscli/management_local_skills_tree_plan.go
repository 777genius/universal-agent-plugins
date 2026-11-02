package agentpluginscli

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

func (p *managementLocalSkill) desiredLocalTree(current localSkillTree) (localSkillTree, error) {
	if !current.Exists {
		current.Mode = localSkillDefaultMode(true)
	}
	current.Exists = true
	files := map[string]localSkillFile{}
	dirs := map[string]uint32{}
	for _, f := range current.Files {
		files[f.Path] = f
	}
	for _, d := range current.Directories {
		dirs[d.Path] = d.Mode
	}
	for _, change := range p.changes {
		if change.remove {
			delete(files, change.relative)
			continue
		}
		mode := localSkillDefaultMode(false)
		if previous, exists := files[change.relative]; exists {
			mode = previous.Mode
		}
		files[change.relative] = localSkillFile{Path: change.relative, Mode: mode, SHA256: localSkillDigest(change.content), Bytes: change.content}
		for parent := path.Dir(change.relative); parent != "."; parent = path.Dir(parent) {
			if _, exists := dirs[parent]; !exists {
				dirs[parent] = localSkillDefaultMode(true)
			}
		}
	}
	current.Files = []localSkillFile{}
	current.Directories = []localSkillDirectory{}
	total := 0
	for _, file := range files {
		total += len(file.Bytes)
		current.Files = append(current.Files, file)
	}
	for directory, mode := range dirs {
		current.Directories = append(current.Directories, localSkillDirectory{Path: directory, Mode: mode})
	}
	if len(current.Files) > localSkillMaxFiles || total > localSkillMaxBytes {
		return current, localSkillRefusal("REQUEST_TOO_LARGE", "The resulting local Skill exceeds 200 files or 10 MiB")
	}
	for _, file := range current.Files {
		if _, exists := dirs[file.Path]; exists {
			return current, localSkillRefusal("INVALID_REQUEST", "A file cannot also be a directory")
		}
	}
	sortLocalSkillTree(&current)
	return current, nil
}

func (b *ManagementBackend) lockLocalSkill(ctx context.Context) (func() error, error) {
	unlockEnvironment, err := (processlock.Lock{Path: b.app.EnvironmentLockPath}).Acquire(ctx)
	if err != nil {
		return nil, err
	}
	nativePath := filepath.Join(b.config.StateRoot, "mutation.lock")
	if filepath.Clean(b.app.EnvironmentLockPath) == nativePath {
		return unlockEnvironment, nil
	}
	unlockNative, err := (processlock.Lock{Path: nativePath}).Acquire(ctx)
	if err != nil {
		_ = unlockEnvironment()
		return nil, err
	}
	return func() error { return errors.Join(unlockNative(), unlockEnvironment()) }, nil
}
