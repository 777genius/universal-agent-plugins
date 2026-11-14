package platformexec

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func importedOpenCodeInlineMarkdownArtifacts(field string, raw map[string]any, configPath string, dstKind string, normalize openCodeInlineNormalizer) ([]pluginmodel.Artifact, map[string]any, []pluginmodel.Warning, error) {
	if len(raw) == 0 {
		return nil, nil, nil, nil
	}
	var (
		artifacts []pluginmodel.Artifact
		warnings  []pluginmodel.Warning
		remaining = map[string]any{}
	)
	for name, value := range raw {
		spec, ok := value.(map[string]any)
		if !ok {
			remaining[name] = value
			warnings = append(warnings, inlineOpenCodePreservationWarning(field, name, configPath, dstKind))
			continue
		}
		frontmatter, body, ok := normalize(name, spec)
		if !ok {
			remaining[name] = value
			warnings = append(warnings, inlineOpenCodePreservationWarning(field, name, configPath, dstKind))
			continue
		}
		relPath, ok := canonicalOpenCodeNamedMarkdownPath(dstKind, name)
		if !ok {
			remaining[name] = value
			warnings = append(warnings, pluginmodel.Warning{
				Kind:    pluginmodel.WarningFidelity,
				Path:    configPath,
				Message: fmt.Sprintf("preserved OpenCode inline %s %q in targets/opencode/config.extra.json because its name cannot be normalized into a canonical markdown file path", field, name),
			})
			continue
		}
		content, err := marshalOpenCodeMarkdown(frontmatter, body)
		if err != nil {
			return nil, nil, nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{RelPath: relPath, Content: content})
	}
	return compactArtifacts(artifacts), remaining, warnings, nil
}

func inlineOpenCodePreservationWarning(field, name, configPath, dstKind string) pluginmodel.Warning {
	return pluginmodel.Warning{
		Kind:    pluginmodel.WarningFidelity,
		Path:    configPath,
		Message: fmt.Sprintf("preserved OpenCode inline %s %q in targets/opencode/config.extra.json because it is not representable as targets/opencode/%s/*.md", field, name, dstKind),
	}
}

func marshalOpenCodeMarkdown(frontmatter map[string]any, body string) ([]byte, error) {
	fm := strings.TrimSpace(string(mustYAML(frontmatter)))
	text := "---\n" + fm + "\n---\n"
	if strings.TrimSpace(body) != "" {
		text += "\n" + strings.TrimSpace(body) + "\n"
	}
	return []byte(text), nil
}

func canonicalOpenCodeNamedMarkdownPath(kind, name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		return "", false
	}
	return filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", kind, name+".md")), true
}

func sortedArtifactKeys(artifacts map[string]pluginmodel.Artifact) []string {
	out := make([]string, 0, len(artifacts))
	for rel := range artifacts {
		out = append(out, rel)
	}
	slices.Sort(out)
	return out
}
