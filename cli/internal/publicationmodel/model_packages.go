package publicationmodel

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
	"github.com/777genius/plugin-kit-ai/cli/internal/targetcontracts"
)

func buildPackage(graph pluginmodel.PackageGraph, publication publishschema.State, target string) (Package, bool) {
	family, channels := packageFamilies(target)
	if family == "" {
		return Package{}, false
	}
	entry, ok := targetcontracts.Lookup(target)
	if !ok {
		return Package{}, false
	}
	state, ok := graph.Targets[target]
	if !ok {
		return Package{}, false
	}
	authoredSet := map[string]struct{}{
		sourceFilePath(graph.SourceFiles, pluginmodel.FileName): {},
	}
	if graph.Launcher != nil && entry.LauncherRequirement == "required" {
		authoredSet[sourceFilePath(graph.SourceFiles, pluginmodel.LauncherFileName)] = struct{}{}
	}
	authoredDocs := map[string]string{}
	for kind, path := range state.Docs {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		authoredDocs[kind] = path
		authoredSet[path] = struct{}{}
	}
	for _, paths := range state.Components {
		for _, path := range paths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			authoredSet[path] = struct{}{}
		}
	}
	for _, path := range graph.Portable.Paths("skills") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		authoredSet[filepath.ToSlash(path)] = struct{}{}
	}
	if graph.Portable.MCP != nil && strings.TrimSpace(graph.Portable.MCP.Path) != "" {
		authoredSet[filepath.ToSlash(graph.Portable.MCP.Path)] = struct{}{}
	}
	for _, path := range publicationPathsForTarget(publication, target) {
		authoredSet[path] = struct{}{}
	}
	return Package{
		Target:           target,
		PackageFamily:    family,
		ChannelFamilies:  cloneStrings(channels),
		TargetClass:      entry.TargetClass,
		InstallModel:     entry.InstallModel,
		AuthoredInputs:   sortedKeys(authoredSet),
		AuthoredDocs:     cloneStringMap(authoredDocs),
		ManagedArtifacts: cloneStrings(entry.ManagedArtifacts),
	}, true
}

func sourceFilePath(paths []string, base string) string {
	base = strings.TrimSpace(base)
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if filepath.Base(path) == base {
			return filepath.ToSlash(path)
		}
	}
	return base
}

func packageFamilies(target string) (string, []string) {
	switch strings.TrimSpace(target) {
	case "codex-package":
		return "codex-plugin", []string{"codex-marketplace"}
	case "claude":
		return "claude-plugin", []string{"claude-marketplace"}
	case "gemini":
		return "gemini-extension", []string{"gemini-gallery"}
	default:
		return "", nil
	}
}

func publicationPathsForTarget(publication publishschema.State, target string) []string {
	switch strings.TrimSpace(target) {
	case "codex-package":
		if publication.Codex != nil {
			return []string{publication.Codex.Path}
		}
	case "claude":
		if publication.Claude != nil {
			return []string{publication.Claude.Path}
		}
	case "gemini":
		if publication.Gemini != nil {
			return []string{publication.Gemini.Path}
		}
	}
	return nil
}
