package pluginmanifest

import (
	"bytes"
	"os"
	"path/filepath"
	"unicode/utf8"
)

func writeArtifacts(root string, artifacts []Artifact) error {
	for _, artifact := range artifacts {
		full := filepath.Join(root, filepath.FromSlash(artifact.RelPath))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, artifact.Content, 0o644); err != nil {
			return err
		}
	}
	return nil
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
