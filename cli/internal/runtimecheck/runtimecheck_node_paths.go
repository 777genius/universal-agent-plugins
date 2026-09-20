package runtimecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func detectNodeManager(root string) NodeManager {
	switch {
	case fileExists(filepath.Join(root, "bun.lock")) || fileExists(filepath.Join(root, "bun.lockb")):
		return NodeManagerBun
	case fileExists(filepath.Join(root, "pnpm-lock.yaml")):
		return NodeManagerPNPM
	case fileExists(filepath.Join(root, "yarn.lock")):
		return NodeManagerYarn
	default:
		return NodeManagerNPM
	}
}

func parseTSOutDir(root string) string {
	body, err := os.ReadFile(filepath.Join(root, "tsconfig.json"))
	if err != nil {
		return ""
	}
	var cfg tsConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return ""
	}
	outDir := strings.TrimSpace(filepath.ToSlash(cfg.CompilerOptions.OutDir))
	if outDir == "" {
		return ""
	}
	return strings.TrimPrefix(outDir, "./")
}

func isBuiltOutputTarget(target, outDir string) bool {
	target = filepath.ToSlash(strings.TrimSpace(target))
	if outDir != "" && strings.HasPrefix(target, strings.Trim(strings.TrimSpace(outDir), "/")+"/") {
		return true
	}
	return strings.HasPrefix(target, "dist/") || strings.HasPrefix(target, "build/")
}

func nodeInstallStatePresent(root string) bool {
	if dirExists(filepath.Join(root, "node_modules")) {
		return true
	}
	return fileExists(filepath.Join(root, ".pnp.cjs")) || fileExists(filepath.Join(root, ".pnp.loader.mjs"))
}

func detectNodeRuntimeTarget(root, entrypoint string) string {
	body, err := os.ReadFile(launcherPath(root, entrypoint))
	if err != nil {
		return defaultNodeRuntimeTarget
	}
	text := filepath.ToSlash(string(body))
	for _, pattern := range launcherTargetPatterns {
		matches := pattern.FindStringSubmatch(text)
		if len(matches) == 2 {
			return matches[1]
		}
	}
	return defaultNodeRuntimeTarget
}
