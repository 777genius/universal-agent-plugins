package pluginmanifest

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

func validateRemovedPortableInputs(root string, layout authoredLayout, targets []string) error {
	if removedPortableInputExists(root, layout, "agents") && !looksLikeManagedAgentsOutput(root, layout, targets) {
		return errors.New(rootAgentsMigrationMessage(targets))
	}
	if removedPortableInputExists(root, layout, "contexts") && !looksLikeManagedContextsOutput(root, layout, targets) {
		return errors.New(rootContextsMigrationMessage(targets))
	}
	return nil
}

func removedPortableInputExists(root string, layout authoredLayout, rel string) bool {
	candidates := []string{filepath.ToSlash(rel)}
	if canonical := layout.Path(rel); canonical != rel {
		candidates = append(candidates, filepath.ToSlash(canonical))
	}
	for _, candidate := range candidates {
		if authoredInputExists(root, candidate) {
			return true
		}
	}
	return false
}

func looksLikeManagedAgentsOutput(root string, layout authoredLayout, targets []string) bool {
	targetSet := setOf(targets)
	if !targetSet["claude"] {
		return false
	}
	return len(discoverFiles(root, layout.Path(filepath.Join("targets", "claude", "agents")), nil)) > 0
}

func looksLikeManagedContextsOutput(root string, layout authoredLayout, targets []string) bool {
	targetSet := setOf(targets)
	if targetSet["gemini"] && len(discoverFiles(root, layout.Path(filepath.Join("targets", "gemini", "contexts")), nil)) > 0 {
		return true
	}
	if targetSet["codex-runtime"] && len(discoverFiles(root, layout.Path(filepath.Join("targets", "codex-runtime", "contexts")), nil)) > 0 {
		return true
	}
	return false
}

func rootAgentsMigrationMessage(targets []string) string {
	targetSet := setOf(targets)
	switch {
	case targetSet["claude"]:
		return "portable agents were removed: move repo-root agents/ into targets/claude/agents/; Gemini agents remain preview-only and Codex lanes do not support agents"
	case targetSet["gemini"]:
		return "portable agents were removed: repo-root agents/ is no longer supported; Gemini agents remain preview-only in this wave"
	default:
		return "portable agents were removed: repo-root agents/ is no longer a canonical authored input"
	}
}

func rootContextsMigrationMessage(targets []string) string {
	targetSet := setOf(targets)
	var destinations []string
	if targetSet["gemini"] {
		destinations = append(destinations, "targets/gemini/contexts/")
	}
	if targetSet["codex-runtime"] {
		destinations = append(destinations, "targets/codex-runtime/contexts/")
	}
	if len(destinations) > 0 {
		return fmt.Sprintf("portable contexts were removed: move repo-root contexts/ into %s", strings.Join(destinations, " and/or "))
	}
	return "portable contexts were removed: repo-root contexts/ is no longer supported for these targets"
}
