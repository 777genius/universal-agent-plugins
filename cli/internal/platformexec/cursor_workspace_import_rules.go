package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func ensureCursorRulesMigrationGuard(root string) error {
	if _, err := os.Stat(filepath.Join(root, removedCursorRulesFileName)); err == nil {
		return fmt.Errorf("unsupported Cursor repo-root rules file: use .cursor/rules/*.mdc")
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func importCursorRuleArtifacts(root string) ([]pluginmodel.Artifact, error) {
	full := filepath.Join(root, ".cursor", "rules")
	if _, err := os.Stat(full); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var artifacts []pluginmodel.Artifact
	err := filepath.WalkDir(full, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("cursor native import does not support symlinks under .cursor/rules")
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(full, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.ToLower(filepath.Ext(rel)) != ".mdc" {
			return fmt.Errorf("cursor native import only supports .mdc files under .cursor/rules: %s", filepath.ToSlash(filepath.Join(".cursor", "rules", rel)))
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.ToSlash(filepath.Join("targets", "cursor-workspace", "rules", rel)),
			Content: body,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(artifacts, func(a, b pluginmodel.Artifact) int { return strings.Compare(a.RelPath, b.RelPath) })
	return artifacts, nil
}
