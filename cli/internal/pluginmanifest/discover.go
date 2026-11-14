package pluginmanifest

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

type discoveredPackage struct {
	layout      authoredLayout
	graph       PackageGraph
	publication publishschema.State
	warnings    []Warning
}

func discoverPackage(root string) (PackageGraph, []Warning, error) {
	state, err := discoverPackageState(root)
	if err != nil {
		return PackageGraph{}, nil, err
	}
	return state.graph, state.warnings, nil
}

func discoverPackageState(root string) (discoveredPackage, error) {
	layout, err := detectAuthoredLayout(root)
	if err != nil {
		return discoveredPackage{}, err
	}
	return discoverPackageStateInLayout(root, layout)
}

func discoverPackageStateInLayout(root string, layout authoredLayout) (discoveredPackage, error) {
	manifest, warnings, err := loadManifestWithWarnings(root)
	if err != nil {
		return discoveredPackage{}, err
	}
	launcher, err := loadLauncherForTargets(root, manifest.EnabledTargets())
	if err != nil {
		return discoveredPackage{}, err
	}
	graph := PackageGraph{
		Manifest: manifest,
		Launcher: launcher,
		Portable: newPortableComponents(),
		Targets:  make(map[string]TargetState, len(manifest.Targets)),
	}
	if err := validateRemovedPortableInputs(root, layout, manifest.EnabledTargets()); err != nil {
		return discoveredPackage{}, err
	}
	sourceSet := map[string]struct{}{layout.Path(FileName): {}}
	if launcher != nil {
		sourceSet[layout.Path(LauncherFileName)] = struct{}{}
	}
	if fileExists(filepath.Join(root, layout.Path("README.md"))) {
		sourceSet[layout.Path("README.md")] = struct{}{}
	}
	publication, err := discoverPublication(root, layout)
	if err != nil {
		return discoveredPackage{}, err
	}
	if err := publication.ValidateTargets(manifest.EnabledTargets()); err != nil {
		return discoveredPackage{}, err
	}
	addSourceFiles(sourceSet, publication.Paths())

	skillPaths := discoverFiles(root, layout.Path(filepath.Join("skills")), func(rel string) bool {
		return strings.HasSuffix(rel, "SKILL.md")
	})
	graph.Portable.Add("skills", skillPaths...)
	addSourceFiles(sourceSet, skillPaths)

	if mcpDoc, ok, err := discoverMCP(root, layout); err != nil {
		return discoveredPackage{}, err
	} else if ok {
		graph.Portable.MCP = mcpDoc
		sourceSet[mcpDoc.Path] = struct{}{}
	}

	for _, target := range manifest.EnabledTargets() {
		state, err := discoverTarget(root, layout, target)
		if err != nil {
			return discoveredPackage{}, err
		}
		graph.Targets[target] = state
		addSourceFiles(sourceSet, targetFiles(state))
	}

	graph.SourceFiles = sortedKeys(sourceSet)
	return discoveredPackage{
		layout:      layout,
		graph:       graph,
		publication: publication,
		warnings:    warnings,
	}, nil
}
