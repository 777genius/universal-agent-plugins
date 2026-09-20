package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendImportedGeminiHooks(root string, result *ImportResult) error {
	hooksPath := filepath.Join("hooks", "hooks.json")
	hooksBody, hookBodyErr := os.ReadFile(filepath.Join(root, hooksPath))
	copied, err := copySingleArtifactIfExists(root, hooksPath, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "hooks", "hooks.json"))
	if err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, copied...)
	if hookBodyErr != nil {
		return nil
	}
	if _, err := parseGeminiHooks(hooksBody); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.ToSlash(hooksPath), err)
	}
	if entrypoint, ok := inferGeminiEntrypoint(hooksBody); ok {
		applyImportedGeminiEntrypoint(entrypoint, result)
	}
	return nil
}

func applyImportedGeminiEntrypoint(entrypoint string, result *ImportResult) {
	if result.Launcher == nil {
		result.Launcher = &pluginmodel.Launcher{
			Runtime:    "go",
			Entrypoint: entrypoint,
		}
		return
	}
	result.Launcher.Entrypoint = entrypoint
}

func appendImportedGeminiDirs(root string, result *ImportResult) error {
	copied, err := copyArtifactDirs(root,
		artifactDir{src: "skills", dst: "skills"},
		artifactDir{src: "commands", dst: filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "commands")},
		artifactDir{src: "policies", dst: filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "policies")},
		artifactDir{src: "contexts", dst: filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts")},
	)
	if err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, copied...)
	return nil
}
