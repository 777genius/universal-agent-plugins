package platformexec

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func importCursorAgentsArtifact(root string) (pluginmodel.Artifact, bool, error) {
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return pluginmodel.Artifact{}, false, nil
		}
		return pluginmodel.Artifact{}, false, err
	}
	content := cursorManagedAgentsImportContent(string(body))
	if strings.TrimSpace(content) == "" {
		return pluginmodel.Artifact{}, false, nil
	}
	return pluginmodel.Artifact{
		RelPath: filepath.ToSlash(filepath.Join("targets", "cursor-workspace", "AGENTS.md")),
		Content: append([]byte(content), '\n'),
	}, true, nil
}

func cursorManagedAgentsImportContent(body string) string {
	content := extractCursorManagedAgentsSection(body)
	if strings.TrimSpace(content) == "" &&
		!strings.Contains(body, "<!-- plugin-kit-ai:begin managed-guidance -->") &&
		!strings.Contains(body, "<!-- plugin-kit-ai:end managed-guidance -->") {
		content = strings.TrimSpace(body)
	}
	return content
}
