package platformexec

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendGeminiGeneratedArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, entrypoint string, artifacts []pluginmodel.Artifact) ([]pluginmodel.Artifact, error) {
	skillArtifacts, err := renderPortableSkills(root, graph.Portable.Paths("skills"), "skills")
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, skillArtifacts...)
	hookArtifacts, err := renderGeminiHookArtifacts(root, graph, state, entrypoint)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, hookArtifacts...)
	copied, err := copyArtifactDirs(root,
		artifactDir{src: authoredComponentDir(state, "commands", filepath.Join("targets", "gemini", "commands")), dst: "commands"},
		artifactDir{src: authoredComponentDir(state, "policies", filepath.Join("targets", "gemini", "policies")), dst: "policies"},
	)
	if err != nil {
		return nil, err
	}
	return append(artifacts, copied...), nil
}

func renderGeminiHookArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, entrypoint string) ([]pluginmodel.Artifact, error) {
	if hookPaths := state.ComponentPaths("hooks"); len(hookPaths) > 0 {
		return copyArtifacts(root, authoredComponentDir(state, "hooks", filepath.Join("targets", "gemini", "hooks")), "hooks")
	}
	if geminiUsesGeneratedHooks(graph, state) {
		return []pluginmodel.Artifact{{
			RelPath: filepath.Join("hooks", "hooks.json"),
			Content: defaultGeminiHooks(entrypoint),
		}}, nil
	}
	return nil, nil
}
