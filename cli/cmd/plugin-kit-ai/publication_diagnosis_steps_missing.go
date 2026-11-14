package main

import (
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func buildMissingPublicationNextSteps(authoredRoot string, missing []publicationmodel.Package) []string {
	authoredRoot = normalizedPublicationAuthoredRoot(authoredRoot)
	stepSet := map[string]struct{}{}
	var steps []string
	for _, pkg := range missing {
		step, ok := missingPublicationStep(authoredRoot, pkg.Target)
		if !ok {
			continue
		}
		if _, seen := stepSet[step]; seen {
			continue
		}
		stepSet[step] = struct{}{}
		steps = append(steps, step)
	}
	slices.Sort(steps)
	return steps
}

func missingPublicationStep(authoredRoot, target string) (string, bool) {
	switch target {
	case "codex-package":
		return "add " + authoredRoot + "/publish/codex/marketplace.yaml, then rerun plugin-kit-ai generate . and plugin-kit-ai validate . --strict", true
	case "claude":
		return "add " + authoredRoot + "/publish/claude/marketplace.yaml, then rerun plugin-kit-ai generate . and plugin-kit-ai validate . --strict", true
	case "gemini":
		return "add " + authoredRoot + "/publish/gemini/gallery.yaml, keep gemini-extension.json in the repository or release root, then rerun plugin-kit-ai validate . --strict", true
	default:
		return "", false
	}
}

func publicationChannelForTarget(authoredRoot, target string) (family string, path string) {
	authoredRoot = normalizedPublicationAuthoredRoot(authoredRoot)
	switch target {
	case "codex-package":
		return "codex-marketplace", authoredRoot + "/publish/codex/marketplace.yaml"
	case "claude":
		return "claude-marketplace", authoredRoot + "/publish/claude/marketplace.yaml"
	case "gemini":
		return "gemini-gallery", authoredRoot + "/publish/gemini/gallery.yaml"
	default:
		return "", ""
	}
}

func normalizedPublicationAuthoredRoot(authoredRoot string) string {
	authoredRoot = strings.TrimSpace(authoredRoot)
	if authoredRoot == "" {
		return pluginmodel.SourceDirName
	}
	return authoredRoot
}
