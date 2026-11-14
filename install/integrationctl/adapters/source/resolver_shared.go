package source

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"golang.org/x/text/unicode/norm"
)

const (
	maxPackageSubdirBytes = 1_024
	maxTreeDepth          = 64
	maxTreeFiles          = 10_000
	maxTreeFileBytes      = int64(64 << 20)
	maxTreeTotalBytes     = int64(256 << 20)
)

var errUnsafePackageSubdir = errors.New("unsafe package subdir")

func hashLocalTree(root string) (string, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return "", fmt.Errorf("source root must be a real directory")
	}

	hasher := sha256.New()
	seenPortablePaths := make(map[string]string)
	files := 0
	var totalBytes int64
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed in source tree: %s", rel)
		}
		if err := validatePortableRelativePath(rel); err != nil {
			return fmt.Errorf("unsafe source path %q: %w", rel, err)
		}
		portableKey := strings.ToLower(norm.NFC.String(rel))
		if previous, exists := seenPortablePaths[portableKey]; exists && previous != rel {
			return fmt.Errorf("portable path collision between %q and %q", previous, rel)
		}
		seenPortablePaths[portableKey] = rel
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file is not allowed in source tree: %s", rel)
		}
		files++
		if files > maxTreeFiles {
			return fmt.Errorf("source tree exceeds %d files", maxTreeFiles)
		}
		if info.Size() > maxTreeFileBytes {
			return fmt.Errorf("source file exceeds %d bytes: %s", maxTreeFileBytes, rel)
		}
		totalBytes += info.Size()
		if totalBytes > maxTreeTotalBytes {
			return fmt.Errorf("source tree exceeds %d bytes", maxTreeTotalBytes)
		}

		writeDigestField(hasher, []byte(rel))
		if info.Mode().Perm()&0o111 != 0 {
			writeDigestField(hasher, []byte("executable"))
		} else {
			writeDigestField(hasher, []byte("regular"))
		}
		writeDigestUint64(hasher, uint64(info.Size()))
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		copied, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if copied != info.Size() {
			return fmt.Errorf("source file changed while hashing: %s", rel)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeDigestField(writer io.Writer, value []byte) {
	writeDigestUint64(writer, uint64(len(value)))
	_, _ = writer.Write(value)
}

func writeDigestUint64(writer io.Writer, value uint64) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], value)
	_, _ = writer.Write(length[:])
}

func validateContainedDirectory(root, candidate string) error {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	cleanCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cleanRoot, cleanCandidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errUnsafePackageSubdir
	}
	current := cleanRoot
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink path component is not allowed")
		}
	}
	return nil
}

func validatePortableRelativePath(rel string) error {
	if rel == "" || !utf8.ValidString(rel) || !norm.NFC.IsNormalString(rel) || strings.ContainsAny(rel, "\\\x00") {
		return errors.New("path must be valid NFC UTF-8 without backslashes")
	}
	parts := strings.Split(rel, "/")
	if len(parts) > maxTreeDepth {
		return fmt.Errorf("path exceeds depth limit %d", maxTreeDepth)
	}
	for _, part := range parts {
		if err := validatePortablePathSegment(part); err != nil {
			return err
		}
	}
	return nil
}

func validatePortablePathSegment(part string) error {
	return pathpolicy.ValidatePortablePathSegment(part)
}

func hasWindowsVolumePrefix(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

func isCommandNotFound(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist)
}

func commandOutput(result ports.CommandResult) string {
	if text := strings.TrimSpace(string(result.Stderr)); text != "" {
		return text
	}
	return strings.TrimSpace(string(result.Stdout))
}
