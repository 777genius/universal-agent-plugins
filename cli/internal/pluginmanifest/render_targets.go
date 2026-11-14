package pluginmanifest

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/platformexec"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationexec"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
	"github.com/777genius/plugin-kit-ai/cli/internal/targetcontracts"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func renderTargetArtifacts(root string, graph PackageGraph, target string) ([]Artifact, error) {
	tc := graph.Targets[target]
	adapter, ok := platformexec.Lookup(target)
	if !ok {
		return nil, fmt.Errorf("unsupported target %q", target)
	}
	return adapter.Generate(root, graph, tc)
}

func expectedManagedPaths(root string, layout authoredLayout, graph PackageGraph, publication publishschema.State, selected []string) []string {
	seen := map[string]struct{}{}
	for _, target := range selected {
		profile, ok := platformmeta.Lookup(target)
		if !ok {
			continue
		}
		tc := graph.Targets[target]
		for _, spec := range profile.ManagedArtifacts {
			switch spec.Kind {
			case platformmeta.ManagedArtifactStatic:
				seen[spec.Path] = struct{}{}
			case platformmeta.ManagedArtifactPortableMCP:
				if graph.Portable.MCP != nil {
					seen[spec.Path] = struct{}{}
				}
			case platformmeta.ManagedArtifactPortableSkills:
				sourceRoot := pluginmodel.RebaseAuthoredPath(spec.SourceRoot, layout.Path(""))
				if sourceRoot == "" {
					sourceRoot = layout.Path("skills")
				}
				addManagedCopies(seen, graph.Portable.Paths("skills"), sourceRoot, spec.OutputRoot)
			case platformmeta.ManagedArtifactMirror:
				if spec.OutputRoot == "" {
					rel := filepath.ToSlash(strings.TrimSpace(tc.DocPath(spec.ComponentKind)))
					if rel == "" {
						continue
					}
					relPath, err := filepath.Rel(pluginmodel.RebaseAuthoredPath(spec.SourceRoot, layout.Path("")), rel)
					if err != nil {
						continue
					}
					seen[filepath.ToSlash(filepath.Join(spec.OutputRoot, relPath))] = struct{}{}
					continue
				}
				addManagedCopies(seen, tc.ComponentPaths(spec.ComponentKind), pluginmodel.RebaseAuthoredPath(spec.SourceRoot, layout.Path("")), spec.OutputRoot)
			case platformmeta.ManagedArtifactSelectedContext:
				continue
			}
		}
		if adapter, ok := platformexec.Lookup(target); ok {
			extraPaths, err := adapter.ManagedPaths(root, graph, tc)
			if err == nil {
				for _, path := range extraPaths {
					seen[path] = struct{}{}
				}
			}
		}
	}
	for _, path := range publicationexec.ManagedPaths(publication, selected) {
		seen[path] = struct{}{}
	}
	if strings.TrimSpace(layout.Path("")) != "" {
		seen["CLAUDE.md"] = struct{}{}
		seen["AGENTS.md"] = struct{}{}
		seen["GENERATED.md"] = struct{}{}
		if managesReadme, err := shouldManageRootReadme(root); err == nil && managesReadme && fileExists(filepath.Join(root, layout.Path("README.md"))) {
			seen["README.md"] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func discoveredTargetKinds(tc TargetComponents) []string {
	var kinds []string
	for kind, path := range tc.Docs {
		if strings.TrimSpace(path) != "" {
			kinds = append(kinds, kind)
		}
	}
	for kind, paths := range tc.Components {
		if len(paths) > 0 {
			kinds = append(kinds, kind)
		}
	}
	slices.Sort(kinds)
	return kinds
}

func unsupportedKinds(entry targetcontracts.Entry, graph PackageGraph, tc TargetComponents) []string {
	supportedPortable := setOf(entry.PortableComponentKinds)
	var unsupported []string
	if len(graph.Portable.Paths("skills")) > 0 && !supportedPortable["skills"] {
		unsupported = append(unsupported, "skills")
	}
	if graph.Portable.MCP != nil && !supportedPortable["mcp_servers"] {
		unsupported = append(unsupported, "mcp_servers")
	}
	supportedNative := setOf(entry.TargetComponentKinds)
	for _, kind := range discoveredTargetKinds(tc) {
		if !supportedNative[kind] {
			unsupported = append(unsupported, kind)
		}
	}
	slices.Sort(unsupported)
	return slices.Compact(unsupported)
}

func targetFiles(tc TargetComponents) []string {
	var out []string
	for _, path := range tc.Docs {
		if strings.TrimSpace(path) != "" {
			out = append(out, path)
		}
	}
	for _, paths := range tc.Components {
		out = append(out, paths...)
	}
	slices.Sort(out)
	return out
}
