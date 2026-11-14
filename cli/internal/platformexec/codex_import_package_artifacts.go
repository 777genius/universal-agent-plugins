package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendImportedCodexPackageBundleArtifacts(result *ImportResult, root string, pluginManifest importedCodexPluginManifest, extra map[string]any) error {
	if err := appendImportedCodexInterfaceArtifact(result, pluginManifest); err != nil {
		return err
	}
	if err := appendImportedCodexSkillsArtifacts(result, root, pluginManifest); err != nil {
		return err
	}
	if err := appendImportedCodexAppsArtifact(result, root, pluginManifest); err != nil {
		return err
	}
	if err := appendImportedCodexMCPArtifact(result, root, pluginManifest); err != nil {
		return err
	}
	if err := appendImportedCodexHooksArtifacts(result, root); err != nil {
		return err
	}
	appendImportedCodexExtraArtifact(result, extra)
	return nil
}

func appendImportedCodexInterfaceArtifact(result *ImportResult, pluginManifest importedCodexPluginManifest) error {
	if pluginManifest.Interface == nil {
		return nil
	}
	body, err := marshalJSON(pluginManifest.Interface)
	if err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
		RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "interface.json"),
		Content: body,
	})
	return nil
}

func appendImportedCodexSkillsArtifacts(result *ImportResult, root string, pluginManifest importedCodexPluginManifest) error {
	if ref := strings.TrimSpace(pluginManifest.SkillsPath); ref != "" {
		copied, err := copyArtifactsFromRefs(root, []string{ref}, "skills")
		if err != nil {
			return err
		}
		result.Artifacts = append(result.Artifacts, copied...)
	}
	return nil
}

func appendImportedCodexAppsArtifact(result *ImportResult, root string, pluginManifest importedCodexPluginManifest) error {
	ref := strings.TrimSpace(pluginManifest.AppsRef)
	if ref == "" {
		return nil
	}
	refPath, err := resolveRelativeRef(root, ref)
	if err != nil {
		return err
	}
	appBody, err := os.ReadFile(filepath.Join(root, refPath))
	if err != nil {
		return err
	}
	if _, err := codexmanifest.ParseAppManifestDoc(appBody); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.ToSlash(refPath), err)
	}
	result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
		RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "app.json"),
		Content: append([]byte(nil), appBody...),
	})
	return nil
}

func appendImportedCodexMCPArtifact(result *ImportResult, root string, pluginManifest importedCodexPluginManifest) error {
	ref := strings.TrimSpace(pluginManifest.MCPServersRef)
	if ref == "" {
		return nil
	}
	refPath, err := resolveRelativeRef(root, ref)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(root, refPath))
	if err != nil {
		return err
	}
	servers, err := decodeJSONObject(body, fmt.Sprintf("Codex MCP manifest %s", filepath.ToSlash(refPath)))
	if err != nil {
		return err
	}
	artifact, err := importedPortableMCPArtifact("codex-package", servers)
	if err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, artifact)
	return nil
}

func appendImportedCodexHooksArtifacts(result *ImportResult, root string) error {
	copied, err := copyArtifactDirs(root, artifactDir{
		src: "hooks",
		dst: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "hooks"),
	})
	if err != nil {
		return err
	}
	result.Artifacts = append(result.Artifacts, copied...)
	return nil
}

func appendImportedCodexExtraArtifact(result *ImportResult, extra map[string]any) {
	if len(extra) == 0 {
		return
	}
	result.Artifacts = append(result.Artifacts, pluginmodel.Artifact{
		RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "manifest.extra.json"),
		Content: mustJSON(extra),
	})
}
