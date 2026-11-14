package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func runBootstrapCommand(ctx context.Context, root, bin string, args ...string) error {
	cmd := bootstrapCommandContext(ctx, bin, args...)
	cmd.Dir = root
	if len(cmd.Env) == 0 {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = append(os.Environ(), cmd.Env...)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bootstrap failed: %s %s: %v\n%s", filepath.Base(bin), strings.Join(args, " "), err, out)
	}
	return nil
}

func runnableVenvPython(root string) string {
	for _, candidate := range pythonCandidates(root) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			cmd := bootstrapCommandContext(context.Background(), candidate, "--version")
			if _, err := cmd.CombinedOutput(); err == nil {
				return candidate
			}
		}
	}
	return ""
}

func hasVenv(root string) bool {
	return fileExists(filepath.Join(root, ".venv")) || dirExists(filepath.Join(root, ".venv"))
}

func pythonCandidates(root string) []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(root, ".venv", "Scripts", "python.exe"),
			filepath.Join(root, ".venv", "bin", "python3"),
		}
	}
	return []string{
		filepath.Join(root, ".venv", "bin", "python3"),
		filepath.Join(root, ".venv", "Scripts", "python.exe"),
	}
}

func pythonPathNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"python", "python3"}
	}
	return []string{"python3", "python"}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
