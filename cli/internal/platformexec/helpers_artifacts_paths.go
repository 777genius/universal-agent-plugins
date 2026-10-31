package platformexec

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// maxBundleFileBytes bounds how much of an untrusted bundle file import and
// validation will read into memory.
const maxBundleFileBytes = 8 << 20

// fileAccessError reports a bundle file access failure by label so host paths
// never leak into user-facing errors. The *fs.PathError shape keeps
// os.IsNotExist and os.IsPermission checks working for callers.
func fileAccessError(err error, label string) error {
	kind := err
	switch {
	case os.IsNotExist(err):
		kind = fs.ErrNotExist
	case os.IsPermission(err):
		kind = fs.ErrPermission
	default:
		if inner := errors.Unwrap(err); inner != nil {
			kind = inner
		}
	}
	return &fs.PathError{Op: "read", Path: label, Err: kind}
}

// readRegularFile reads a bundle file only when it is a regular file: symlinks
// and devices must never pull host file content into generated artifacts. The
// opened handle is re-checked against the earlier Lstat so a path swapped for a
// symlink between the two calls is rejected instead of followed.
func readRegularFile(path, label string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fileAccessError(err, label)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", label)
	}
	if info.Size() > maxBundleFileBytes {
		return nil, fmt.Errorf("%s exceeds the %d-byte bundle file limit", label, maxBundleFileBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fileAccessError(err, label)
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil {
		return nil, fileAccessError(err, label)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("%s must be a regular file", label)
	}
	body, err := io.ReadAll(io.LimitReader(f, maxBundleFileBytes+1))
	if err != nil {
		return nil, fileAccessError(err, label)
	}
	if len(body) > maxBundleFileBytes {
		return nil, fmt.Errorf("%s exceeds the %d-byte bundle file limit", label, maxBundleFileBytes)
	}
	return body, nil
}

// rejectSymlinkComponents fails when any path component under root is a
// symlink: containment must hold on disk, not just lexically, or a bundle can
// smuggle host file content in through a symlinked directory.
func rejectSymlinkComponents(root, rel string) error {
	current := root
	for _, part := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return fileAccessError(err, rel)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s must not traverse symlinks", rel)
		}
	}
	return nil
}

// readBundleFile reads root/rel after lexical containment and a symlink-free
// component walk. Every untrusted bundle read must go through it.
func readBundleFile(root, rel, label string) ([]byte, error) {
	contained, err := resolveRelativeRef(root, rel)
	if err != nil || contained == "" {
		return nil, fmt.Errorf("%s must stay inside the plugin root", label)
	}
	if err := rejectSymlinkComponents(root, contained); err != nil {
		return nil, err
	}
	return readRegularFile(filepath.Join(root, filepath.FromSlash(contained)), label)
}

func discoverFiles(root, dir string, allow func(string) bool) []string {
	full := filepath.Join(root, dir)
	var out []string
	filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if allow != nil && !allow(rel) {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	slices.Sort(out)
	return out
}

func cleanRelativeRef(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return ""
	}
	return path
}

func resolveRelativeRef(root, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}
	if filepath.IsAbs(ref) || filepath.VolumeName(ref) != "" {
		return "", fmt.Errorf("ref %q must stay within the plugin root", ref)
	}
	cleaned := filepath.Clean(ref)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("ref %q must stay within the plugin root", ref)
	}
	cleaned = strings.TrimPrefix(cleaned, "."+string(filepath.Separator))
	cleaned = filepath.ToSlash(cleaned)
	if cleaned == "" || cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("ref %q must stay within the plugin root", ref)
	}
	return cleaned, nil
}
