package platformexec

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func importClaudeStructuredDoc(root, rootPath, targetPath string, inlineProvided bool, inline map[string]any, inlineLabel string) ([]pluginmodel.Artifact, string, error) {
	if body, err := os.ReadFile(filepath.Join(root, rootPath)); err == nil {
		return []pluginmodel.Artifact{{RelPath: targetPath, Content: body}}, "", nil
	} else if !os.IsNotExist(err) {
		return nil, "", err
	}
	if inlineProvided {
		return []pluginmodel.Artifact{{RelPath: targetPath, Content: mustJSON(inline)}}, fmt.Sprintf("%s was normalized into %s", inlineLabel, filepath.ToSlash(targetPath)), nil
	}
	return nil, "", nil
}
