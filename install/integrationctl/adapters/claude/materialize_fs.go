package claude

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func copyNativeClaudePackage(sourceRoot, destRoot string) error {
	for _, path := range []string{
		filepath.Join(sourceRoot, ".claude-plugin"),
		filepath.Join(sourceRoot, ".mcp.json"),
		filepath.Join(sourceRoot, "settings.json"),
		filepath.Join(sourceRoot, ".lsp.json"),
		filepath.Join(sourceRoot, "hooks"),
		filepath.Join(sourceRoot, "skills"),
		filepath.Join(sourceRoot, "commands"),
		filepath.Join(sourceRoot, "agents"),
	} {
		if _, err := copyPathIfExists(path, filepath.Join(destRoot, filepath.Base(path))); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy native Claude package", err)
		}
	}
	return nil
}

func copyPathIfExists(src, dest string) (bool, error) {
	return filetree.CopyPathIfExists(src, dest)
}

func copyFile(src, dest string) error {
	return filetree.CopyFile(src, dest)
}

func marshalJSON(value any) ([]byte, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
