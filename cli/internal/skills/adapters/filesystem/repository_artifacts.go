package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func writeRepositoryArtifacts(root string, artifacts []domain.Artifact) error {
	for _, artifact := range artifacts {
		full := filepath.Join(root, artifact.RelPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, artifact.Content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func removeRepositoryArtifacts(root string, relPaths []string) error {
	for _, relPath := range relPaths {
		full := filepath.Join(root, relPath)
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := pruneEmptyParents(root, filepath.Dir(full)); err != nil {
			return err
		}
	}
	return nil
}

func (Repository) ListManagedArtifacts(root string, selectedTargets map[string]struct{}) ([]string, error) {
	seen := make(map[string]struct{})
	if _, ok := selectedTargets["claude"]; ok {
		if err := walkManagedFiles(filepath.Join(root, "generated", "skills", "claude"), func(relPath string) {
			seen[filepath.ToSlash(relPath)] = struct{}{}
		}); err != nil {
			return nil, err
		}
	}
	if _, ok := selectedTargets["codex"]; ok {
		if err := walkManagedFiles(filepath.Join(root, "generated", "skills", "codex"), func(relPath string) {
			seen[filepath.ToSlash(relPath)] = struct{}{}
		}); err != nil {
			return nil, err
		}
	}
	if err := walkGeneratedCommandDocs(filepath.Join(root, "commands"), func(relPath string) {
		seen[filepath.ToSlash(relPath)] = struct{}{}
	}); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(seen))
	for relPath := range seen {
		out = append(out, filepath.FromSlash(relPath))
	}
	sort.Strings(out)
	return out, nil
}

func walkManagedFiles(root string, add func(relPath string)) error {
	base := filepath.Dir(filepath.Dir(filepath.Dir(root)))
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		add(relPath)
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func walkGeneratedCommandDocs(root string, add func(relPath string)) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		full := filepath.Join(root, entry.Name())
		body, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		text := string(body)
		if !strings.Contains(text, "This file is generated from `skills/") || !strings.Contains(text, "Regenerate with `plugin-kit-ai skills generate`.") {
			continue
		}
		add(filepath.Join("commands", entry.Name()))
	}
	return nil
}

func pruneEmptyParents(root, dir string) error {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	for dir != root && dir != "." && dir != string(filepath.Separator) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				dir = filepath.Dir(dir)
				continue
			}
			return err
		}
		if len(entries) > 0 {
			return nil
		}
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
	return nil
}
