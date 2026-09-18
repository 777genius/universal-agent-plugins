package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// DigestSkillDirectory hashes a skill tree the same way every native-config
// client records ManagedDigest. The algorithm lived next to Kiro only because
// Kiro was the first caller; Gemini, OpenCode and Cline use it too, and they
// cannot import clients/kiro.
func DigestSkillDirectory(root string) (string, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return appendSkillDigest(hash, root, path, entry)
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func appendSkillDigest(hash io.Writer, root, path string, entry fs.DirEntry) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	if entry.Type()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink is not allowed: %s", relative)
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	kind := byte('d')
	if !entry.IsDir() {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file is not allowed: %s", relative)
		}
		kind = 'f'
	}
	_, _ = hash.Write([]byte{kind, 0})
	_, _ = io.WriteString(hash, filepath.ToSlash(relative))
	_, _ = hash.Write([]byte{0})
	if kind != 'f' {
		return nil
	}
	if info.Mode().Perm()&0o111 != 0 {
		_, _ = hash.Write([]byte{'x', 0})
	} else {
		_, _ = hash.Write([]byte{'-', 0})
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	_, _ = hash.Write([]byte{0})
	return nil
}

// DigestJSONObject is the MCP-server counterpart of DigestSkillDirectory.
func DigestJSONObject(value map[string]any) string {
	body, _ := json.Marshal(value)
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}
