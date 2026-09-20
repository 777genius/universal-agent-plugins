package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/geminimanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func readImportedGeminiExtension(root string) (importedGeminiExtension, bool, error) {
	return geminimanifest.ReadImportedExtension(root)
}

func importedGeminiSettingsArtifacts(values []any) []pluginmodel.Artifact {
	used := map[string]int{}
	var artifacts []pluginmodel.Artifact
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		setting := geminiSetting{}
		if name, ok := item["name"].(string); ok {
			setting.Name = name
		}
		if description, ok := item["description"].(string); ok {
			setting.Description = description
		}
		if envVar, ok := item["envVar"].(string); ok {
			setting.EnvVar = envVar
		}
		if sensitive, ok := item["sensitive"].(bool); ok {
			setting.Sensitive = sensitive
		}
		filename := collisionSafeSlug(setting.Name, used) + ".yaml"
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "settings", filename),
			Content: mustYAML(setting),
		})
	}
	return artifacts
}

func importedGeminiThemeArtifacts(values []any) []pluginmodel.Artifact {
	used := map[string]int{}
	var artifacts []pluginmodel.Artifact
	for _, value := range values {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name, _ := item["name"].(string)
		filename := collisionSafeSlug(name, used) + ".yaml"
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "themes", filename),
			Content: mustYAML(item),
		})
	}
	return artifacts
}

func collisionSafeSlug(name string, used map[string]int) string {
	base := strings.TrimSpace(strings.ToLower(name))
	if base == "" {
		base = "item"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "item"
	}
	used[slug]++
	if used[slug] == 1 {
		return slug
	}
	return fmt.Sprintf("%s-%d", slug, used[slug])
}
