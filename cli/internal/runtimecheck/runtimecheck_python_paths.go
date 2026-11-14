package runtimecheck

import (
	"os"
	"path/filepath"
	"runtime"
)

func hasVenv(root string) bool {
	return fileExists(joinRoot(root, ".venv")) || dirExists(joinRoot(root, ".venv"))
}

func pythonInterpreter(root string) string {
	for _, candidate := range pythonCandidates(root) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func pythonInterpreterInEnv(envRoot string) string {
	for _, candidate := range pythonEnvCandidates(envRoot) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func pythonCandidates(root string) []string {
	return pythonEnvCandidates(joinRoot(root, ".venv"))
}

func pythonEnvCandidates(envRoot string) []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(envRoot, "Scripts", "python.exe"),
			filepath.Join(envRoot, "bin", "python3"),
		}
	}
	return []string{
		filepath.Join(envRoot, "bin", "python3"),
		filepath.Join(envRoot, "Scripts", "python.exe"),
	}
}

func pythonPathNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"python", "python3"}
	}
	return []string{"python3", "python"}
}

func pythonVersion(root, path string) (string, error) {
	return RunCommand(root, path, "--version")
}

func joinRoot(root, rel string) string {
	return filepath.Join(root, rel)
}

func filepathToSlash(path string) string {
	return filepath.ToSlash(path)
}
