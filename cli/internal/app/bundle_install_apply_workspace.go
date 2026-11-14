package app

import (
	"os"
	"path/filepath"
)

func prepareBundleInstallWorkspace(dest string) (string, func(), error) {
	dest = filepath.Clean(dest)
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, err
	}
	tmp, err := os.MkdirTemp(parent, ".plugin-kit-ai-bundle-*")
	if err != nil {
		return "", nil, err
	}
	return tmp, func() { _ = os.RemoveAll(tmp) }, nil
}
