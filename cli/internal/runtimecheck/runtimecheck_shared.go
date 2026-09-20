package runtimecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func (p Project) ProjectLine() string {
	manager := "none"
	switch p.Runtime {
	case "python":
		manager = p.Python.ManagerDisplay()
	case "node":
		manager = p.Node.ManagerDisplay()
	case "":
		manager = "n/a"
	}
	return fmt.Sprintf("Project: lane=%s runtime=%s manager=%s", p.Lane, valueOrDefault(p.Runtime, "none"), manager)
}

func ValidateCommand(targets []string) string {
	return validateCommand(targets)
}

func laneSummary(targets []string) string {
	if len(targets) == 0 {
		return "none"
	}
	return strings.Join(targets, ",")
}

func validateCommand(targets []string) string {
	if len(targets) == 1 {
		return fmt.Sprintf("plugin-kit-ai validate . --platform %s --strict", targets[0])
	}
	return "plugin-kit-ai validate . --strict"
}

func firstExisting(root string, names ...string) string {
	for _, name := range names {
		if fileExists(filepath.Join(root, name)) {
			return name
		}
	}
	return ""
}

func firstAvailableBinary(names []string) string {
	for _, name := range names {
		if lookupBinary(name) {
			return name
		}
	}
	return ""
}

func lookupBinary(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	_, err := LookPath(name)
	return err == nil
}

func launcherPath(root, entrypoint string) string {
	rel := strings.TrimPrefix(filepath.Clean(entrypoint), "./")
	full := filepath.Join(root, rel)
	if runtime.GOOS == "windows" {
		if _, err := os.Stat(full + ".cmd"); err == nil {
			return full + ".cmd"
		}
	}
	return full
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
