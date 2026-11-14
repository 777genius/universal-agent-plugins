package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

func FindRepoRoot(start string) (string, error) {
	dir := start
	for {
		mod := filepath.Join(dir, "go.mod")
		body, err := os.ReadFile(mod)
		if err == nil && bytes.HasPrefix(body, []byte("module github.com/777genius/plugin-kit-ai\n")) {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", fmt.Errorf("repo root not found from %s", start)
		}
		dir = next
	}
}
