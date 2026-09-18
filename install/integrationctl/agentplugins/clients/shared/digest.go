package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
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
	sum := sha256.New()
	dir, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer func() { _ = dir.Close() }()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		return hashSkillWalkEntry(sum, dir, root, path, entry, walkErr)
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(sum.Sum(nil)), nil
}

func hashSkillWalkEntry(sum hash.Hash, dir *os.Root, root, path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
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
	_, _ = sum.Write([]byte{kind, 0})
	_, _ = io.WriteString(sum, filepath.ToSlash(relative))
	_, _ = sum.Write([]byte{0})
	if kind != 'f' {
		return nil
	}
	return hashSkillFile(sum, dir, relative, info)
}

func hashSkillFile(sum hash.Hash, dir *os.Root, relative string, info fs.FileInfo) error {
	if info.Mode().Perm()&0o111 != 0 {
		_, _ = sum.Write([]byte{'x', 0})
	} else {
		_, _ = sum.Write([]byte{'-', 0})
	}
	file, err := dir.Open(filepath.ToSlash(relative))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(sum, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	_, _ = sum.Write([]byte{0})
	return nil
}

// DigestJSONObject is the MCP-server counterpart of DigestSkillDirectory.
func DigestJSONObject(value map[string]any) string {
	body, _ := json.Marshal(value)
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}
