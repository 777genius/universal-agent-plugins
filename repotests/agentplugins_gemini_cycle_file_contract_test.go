//go:build linux

package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiCycleOriginalUserFileMetadataAndSameBytesReplacementStayFrozen(t *testing.T) {
	for _, name := range []string{"settings.json", "trustedFolders.json", "policies/uap-project-proof.toml"} {
		for _, kind := range []string{"mode", "inode", "parent-mode"} {
			t.Run(name+"/"+kind, func(t *testing.T) {
				c := geminiCycleInert(t)
				path := filepath.Join(c.f.base.Home, ".gemini", filepath.FromSlash(name))
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "mode":
					if err := os.Chmod(path, 0644); err != nil {
						t.Fatal(err)
					}
				case "inode":
					file, err := os.CreateTemp(filepath.Dir(path), ".inert-replacement-")
					if err != nil {
						t.Fatal(err)
					}
					_, writeErr := file.Write(body)
					closeErr := file.Close()
					if writeErr != nil || closeErr != nil {
						t.Fatal("owned same-bytes replacement write/close")
					}
					if err := os.Rename(file.Name(), path); err != nil {
						t.Fatal(err)
					}
				case "parent-mode":
					if err := os.Chmod(filepath.Dir(path), 0750); err != nil {
						t.Fatal(err)
					}
				}
				// Byte-only checks continue to match: the new finite held frame at the
				// actual c.check seam must independently reject metadata-only changes.
				c.f.check(t)
				if err := c.originalFiles(); err == nil {
					t.Fatal("same bytes hid original user authority metadata replacement")
				}
				if nativeSHA(t, path) != geminiCycleSHA(body) {
					t.Fatal("negative control unexpectedly changed original bytes")
				}
			})
		}
	}
}
