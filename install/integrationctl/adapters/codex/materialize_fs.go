package codex

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func copyNativeCodexPackage(sourceRoot, destRoot string) error {
	for _, path := range []string{
		filepath.Join(sourceRoot, ".codex-plugin"),
		filepath.Join(sourceRoot, ".mcp.json"),
		filepath.Join(sourceRoot, ".app.json"),
		filepath.Join(sourceRoot, "skills"),
		filepath.Join(sourceRoot, "assets"),
	} {
		if _, err := copyPathIfExists(path, filepath.Join(destRoot, filepath.Base(path))); err != nil {
			return domain.NewError(domain.ErrMutationApply, "copy native Codex package", err)
		}
	}
	return nil
}

func copyPathIfExists(src, dest string) (bool, error) {
	return filetree.CopyPathIfExists(src, dest)
}

func copyDir(src, dest string) error {
	return filetree.CopyDir(src, dest)
}

func copyFile(src, dest string) error {
	return filetree.CopyFile(src, dest)
}
