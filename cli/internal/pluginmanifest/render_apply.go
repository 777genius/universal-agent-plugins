package pluginmanifest

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func writeArtifacts(root string, artifacts []Artifact) error {
	for _, artifact := range artifacts {
		full := filepath.Join(root, filepath.FromSlash(artifact.RelPath))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		mode := generatedArtifactMode(artifact.RelPath)
		if err := os.WriteFile(full, artifact.Content, mode); err != nil {
			return err
		}
		// WriteFile keeps the mode of an existing file, so generate must chmod
		// too: hook runtimes under bin/ need their executable bit to survive
		// regeneration and be reproducible from a clean checkout.
		if err := os.Chmod(full, mode); err != nil {
			return err
		}
	}
	return nil
}

// generatedArtifactMode follows the scaffold write policy: files under a bin/
// tree or with .sh/.cmd suffixes are executable so hook runtimes and launcher
// scripts work right after generate.
func generatedArtifactMode(rel string) fs.FileMode {
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "bin/") || strings.HasSuffix(rel, ".sh") || strings.HasSuffix(rel, ".cmd") {
		return 0o755
	}
	return 0o644
}

func removeArtifacts(root string, relPaths []string) error {
	for _, relPath := range relPaths {
		full := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func artifactContentEqual(actual, expected []byte) bool {
	if bytes.Equal(actual, expected) {
		return true
	}
	if !looksLikeText(actual) || !looksLikeText(expected) {
		return false
	}
	return bytes.Equal(normalizeTextNewlines(actual), normalizeTextNewlines(expected))
}

func looksLikeText(body []byte) bool {
	return utf8.Valid(body) && !bytes.Contains(body, []byte{0})
}

func normalizeTextNewlines(body []byte) []byte {
	body = bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n"))
	body = bytes.ReplaceAll(body, []byte("\r"), []byte("\n"))
	return body
}
