package pluginmanifest

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// containedArtifactRel normalizes a generated artifact path and rejects
// anything that would escape the plugin root when joined with it: artifact
// rel paths originate from imported untrusted bundles.
func containedArtifactRel(rel string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if cleaned == "" || cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" {
		return "", fmt.Errorf("artifact path %q must stay inside the plugin root", rel)
	}
	return cleaned, nil
}

// rejectSymlinkedDest fails when any existing path component under root is a
// symlink, so generated artifacts can never be written or removed through a
// link that points outside the plugin tree.
func rejectSymlinkedDest(root, rel string) error {
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("artifact path %q must not traverse symlinks", rel)
		}
	}
	return nil
}

func writeArtifacts(root string, artifacts []Artifact) error {
	for _, artifact := range artifacts {
		rel, err := containedArtifactRel(artifact.RelPath)
		if err != nil {
			return err
		}
		if err := rejectSymlinkedDest(root, rel); err != nil {
			return err
		}
		full := filepath.Join(root, rel)
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
		rel, err := containedArtifactRel(relPath)
		if err != nil {
			return err
		}
		if err := rejectSymlinkedDest(root, rel); err != nil {
			return err
		}
		full := filepath.Join(root, rel)
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
