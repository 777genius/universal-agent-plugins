package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Do not normalize ambiguous caller input: ownership checks and subprocesses
// must use the very same explicit profile spelling.
func validateProfile(root, planned string) error {
	if !cleanAbsolute(root) || root == filepath.VolumeName(root)+string(filepath.Separator) {
		return fmt.Errorf("Codex config root must be an explicit clean absolute directory")
	}
	if planned != "" && planned != root {
		return fmt.Errorf("Codex planned registry root differs from client config root")
	}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		return fmt.Errorf("Codex config root is not a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect Codex config root: %w", err)
	}
	return nil
}

func cleanAbsolute(path string) bool {
	return path != "" && strings.TrimSpace(path) == path && !strings.ContainsAny(path, "\x00\r\n") &&
		filepath.IsAbs(path) && filepath.Clean(path) == path
}

func codexCommand(root, executable string, ambient []string, args ...string) (legacyports.Command, error) {
	if err := validateProfile(root, ""); err != nil {
		return legacyports.Command{}, err
	}
	if !cleanAbsolute(executable) {
		return legacyports.Command{}, fmt.Errorf("Codex executable must be an explicit clean absolute path")
	}
	environment, err := codexEnvironment(root, ambient)
	if err != nil {
		return legacyports.Command{}, err
	}
	return legacyports.Command{Argv: append([]string{executable}, args...), Env: environment}, nil
}

// Keep only process basics needed by the native executable and its downloads.
// In particular, do not inherit loader, language-runtime, proxy or Codex knobs.
// Validate even discarded entries, with case-folding to reject ambiguity on
// Windows as well as Unix. Sorting makes the child environment deterministic.
func codexEnvironment(root string, ambient []string) ([]string, error) {
	if err := validateProfile(root, ""); err != nil {
		return nil, err
	}
	keep := map[string]bool{"HOME": true, "USERPROFILE": true, "PATH": true, "SYSTEMROOT": true, "WINDIR": true, "TEMP": true, "TMP": true, "TMPDIR": true}
	seen := map[string]bool{}
	result := []string{"CODEX_HOME=" + root}
	size := 0
	for _, entry := range ambient {
		size += len(entry)
		if size > 128*1024 || len(ambient) > 4096 {
			return nil, fmt.Errorf("Codex ambient environment exceeds limit")
		}
		key, value, ok := strings.Cut(entry, "=")
		canonical := strings.ToUpper(key)
		if !ok || key == "" || strings.ContainsRune(entry, 0) || seen[canonical] {
			return nil, fmt.Errorf("malformed or duplicate Codex environment entry")
		}
		for i, ch := range key {
			if !(ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch == '_' || i > 0 && ch >= '0' && ch <= '9') {
				return nil, fmt.Errorf("malformed Codex environment name")
			}
		}
		seen[canonical] = true
		if keep[canonical] {
			result = append(result, canonical+"="+value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func runCodex(ctx context.Context, env clients.Env, root, executable string, args ...string) (legacyports.CommandResult, error) {
	command, err := codexCommand(root, executable, os.Environ(), args...)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	if env.Runner == nil {
		return legacyports.CommandResult{}, fmt.Errorf("%s runner is unavailable", codexCLIName)
	}
	result, err := env.Runner.Run(ctx, command)
	if err != nil {
		return result, fmt.Errorf("start %s: %w", codexCLIName, err)
	}
	if result.ExitCode != 0 {
		return result, fmt.Errorf("%s command failed with exit code %d", codexCLIName, result.ExitCode)
	}
	return result, nil
}
