package platformexec

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendOpenCodeArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, artifacts []pluginmodel.Artifact) ([]pluginmodel.Artifact, error) {
	skillArtifacts, err := renderPortableSkills(root, graph.Portable.Paths("skills"), ".opencode/skills")
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, skillArtifacts...)

	copied, err := copyArtifactDirs(root,
		artifactDir{src: authoredComponentDir(state, "commands", filepath.Join("targets", "opencode", "commands")), dst: filepath.Join(".opencode", "commands")},
		artifactDir{src: authoredComponentDir(state, "agents", filepath.Join("targets", "opencode", "agents")), dst: filepath.Join(".opencode", "agents")},
		artifactDir{src: authoredComponentDir(state, "themes", filepath.Join("targets", "opencode", "themes")), dst: filepath.Join(".opencode", "themes")},
		artifactDir{src: authoredComponentDir(state, "tools", filepath.Join("targets", "opencode", "tools")), dst: filepath.Join(".opencode", "tools")},
		artifactDir{src: authoredOpenCodePluginDir(root, state), dst: filepath.Join(".opencode", "plugins")},
	)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, copied...)

	packageArtifacts, err := copySingleArtifactIfExists(root, state.DocPath("local_plugin_dependencies"), filepath.Join(".opencode", "package.json"))
	if err != nil {
		return nil, err
	}
	return append(artifacts, packageArtifacts...), nil
}

func authoredOpenCodePluginDir(root string, state pluginmodel.TargetState) string {
	if paths := state.ComponentPaths("local_plugin_code"); len(paths) > 0 {
		dir := filepath.ToSlash(filepath.Dir(paths[0]))
		if dir != "." {
			return dir
		}
	}
	for _, candidate := range []string{
		filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "plugins"),
	} {
		if _, err := os.Stat(filepath.Join(root, candidate)); err == nil {
			return filepath.ToSlash(candidate)
		}
	}
	return filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "plugins"))
}
