package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Write creates the plugin tree at root (must exist or be creatable).
func Write(root string, d Data, force bool) error {
	plan, err := BuildPlan(d)
	if err != nil {
		return err
	}
	return Apply(root, plan, force)
}

func Apply(root string, plan ProjectPlan, force bool) error {
	info, err := os.Stat(root)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if mkErr := os.MkdirAll(root, 0o755); mkErr != nil {
			return mkErr
		}
	} else if !info.IsDir() {
		return fmt.Errorf("output path %q is not a directory", root)
	} else {
		entries, rerr := os.ReadDir(root)
		if rerr != nil {
			return rerr
		}
		if len(entries) > 0 && !force {
			return fmt.Errorf("directory %q is not empty (use --force to overwrite files)", root)
		}
	}

	for _, file := range plan.Files {
		if err := writeOne(root, file.RelPath, file.Template, plan.Data, force); err != nil {
			return err
		}
	}
	return nil
}

func writeOne(root, rel, tplName string, d Data, force bool) error {
	body, mode, err := RenderTemplate(tplName, d)
	if err != nil {
		return err
	}
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(full); err == nil && !force {
		return fmt.Errorf("refusing to overwrite existing file %s (use --force)", rel)
	}
	return os.WriteFile(full, body, modeForPath(rel, tplName, mode))
}

func modeForPath(rel, tplName string, defaultMode fs.FileMode) fs.FileMode {
	if strings.HasPrefix(filepath.ToSlash(rel), "bin/") || strings.HasSuffix(rel, ".sh") || strings.HasSuffix(rel, ".cmd") ||
		strings.HasSuffix(tplName, ".sh.tmpl") || strings.HasSuffix(tplName, ".cmd.tmpl") {
		return 0o755
	}
	return defaultMode
}
