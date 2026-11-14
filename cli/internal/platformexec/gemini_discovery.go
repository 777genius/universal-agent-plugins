package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (geminiAdapter) DetectNative(root string) bool {
	return fileExists(filepath.Join(root, "gemini-extension.json"))
}

func (geminiAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	if err := refineGeminiPackageMetadata(root, state); err != nil {
		return err
	}
	if err := refineGeminiHooksLayout(state); err != nil {
		return err
	}
	return refineGeminiYAMLArtifacts(state)
}

func refineGeminiPackageMetadata(root string, state *pluginmodel.TargetState) error {
	if rel := state.DocPath("package_metadata"); strings.TrimSpace(rel) != "" {
		if _, ok, err := readYAMLDoc[geminiPackageMeta](root, rel); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		} else if !ok {
			return nil
		}
	}
	return nil
}

func refineGeminiHooksLayout(state *pluginmodel.TargetState) error {
	expectedSuffix := filepath.ToSlash(filepath.Join("targets", "gemini", "hooks", "hooks.json"))
	for _, rel := range state.ComponentPaths("hooks") {
		if strings.HasSuffix(filepath.ToSlash(rel), expectedSuffix) {
			continue
		}
		return fmt.Errorf("unsupported Gemini hooks layout: use only %s", filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, expectedSuffix)))
	}
	return nil
}

func refineGeminiYAMLArtifacts(state *pluginmodel.TargetState) error {
	for _, rel := range append(append([]string{}, state.ComponentPaths("settings")...), state.ComponentPaths("themes")...) {
		if geminiYAMLFileRe.MatchString(rel) {
			continue
		}
		kind := "theme"
		if strings.Contains(rel, "/settings/") {
			kind = "setting"
		}
		return fmt.Errorf("unsupported Gemini %s file %s: use .yaml or .yml", kind, rel)
	}
	return nil
}
