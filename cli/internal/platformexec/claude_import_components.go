package platformexec

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func importClaudeComponentRefs(root, kind, dstRoot string, overridden bool, refs []string) ([]pluginmodel.Artifact, []pluginmodel.Warning, error) {
	if !overridden {
		if !fileExists(filepath.Join(root, kind)) {
			return nil, nil, nil
		}
		refs = []string{kind}
	}
	if len(refs) == 0 {
		return nil, nil, nil
	}
	artifacts, err := copyArtifactsFromRefs(root, refs, dstRoot)
	if err != nil {
		return nil, nil, err
	}
	if !overridden {
		return artifacts, nil, nil
	}
	return artifacts, []pluginmodel.Warning{{
		Kind:    pluginmodel.WarningFidelity,
		Path:    filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
		Message: fmt.Sprintf("custom Claude %s paths were normalized into canonical package-standard layout", kind),
	}}, nil
}

func importClaudePortableSkills(root string, manifest importedClaudePluginManifest) ([]pluginmodel.Artifact, []pluginmodel.Warning, error) {
	if !manifest.SkillsOverride {
		if !fileExists(filepath.Join(root, "skills")) {
			return nil, nil, nil
		}
		artifacts, err := copyArtifactsFromRefs(root, []string{"skills"}, "skills")
		if err != nil {
			return nil, nil, err
		}
		return artifacts, nil, nil
	}
	if len(manifest.SkillsRefs) == 0 {
		return nil, nil, nil
	}
	artifacts, err := copyArtifactsFromRefs(root, manifest.SkillsRefs, "skills")
	if err != nil {
		return nil, nil, err
	}
	return artifacts, []pluginmodel.Warning{{
		Kind:    pluginmodel.WarningFidelity,
		Path:    filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json")),
		Message: "custom Claude skills paths were normalized into canonical package-standard layout",
	}}, nil
}
