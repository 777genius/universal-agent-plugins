package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const maxFiles = 64
const maxBytes = 1 << 20

// This is a deliberately bounded portable output policy, not a second schema
// or a claim that every standard-valid package name is safe on every host.
func portableLeaf(s string) error {
	if s == "" || s == "." || s == ".." || len(s) > 128 || strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
		return fmt.Errorf("unsafe portable path component %q", s)
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-') {
			return fmt.Errorf("unsafe portable path component %q", s)
		}
	}
	base := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return fmt.Errorf("Windows-reserved path component %q", s)
	}
	return nil
}
func validateFiles(files []File) error {
	if len(files) == 0 || len(files) > maxFiles {
		return fmt.Errorf("plan must contain 1–%d files", maxFiles)
	}
	seen := map[string]string{}
	kinds := map[string]bool{}
	total := 0
	manifest := false
	for _, f := range files {
		if len(f.Path) > 512 || !fs.ValidPath(f.Path) || strings.Contains(f.Path, "\\") || len(strings.Split(f.Path, "/")) > 8 {
			return fmt.Errorf("unsafe plan path %q", f.Path)
		}
		if f.Mode != 0644 && f.Mode != 0755 {
			return fmt.Errorf("unsupported file mode for %q", f.Path)
		}
		total += len(f.Bytes)
		if total > maxBytes {
			return fmt.Errorf("plan exceeds byte limit")
		}
		pieces := strings.Split(f.Path, "/")
		switch strings.ToLower(pieces[0]) {
		case "plugin", ".codex-plugin", "hooks", ".git", ".mcp.json", ".app.json":
			return fmt.Errorf("forbidden portable output %q", f.Path)
		}
		for i, piece := range pieces {
			if err := portableLeaf(piece); err != nil {
				return err
			}
			name := strings.Join(pieces[:i+1], "/")
			key := strings.ToLower(name)
			isFile := i == len(pieces)-1
			if prev, ok := seen[key]; ok {
				if prev != name || kinds[key] || isFile {
					return fmt.Errorf("path or case collision at %q", name)
				}
			} else {
				seen[key] = name
				kinds[key] = isFile
			}
		}
		manifest = manifest || f.Path == "plugin.json"
	}
	if !manifest {
		return fmt.Errorf("plan lacks exact root plugin.json")
	}
	return nil
}

// openParent walks existing real directories using anchored roots and verifies
// each opened identity. Missing parents and symlink ancestors are refused.
// The caller supplies an absolute clean path; no source directories are created.
func openParent(abs string) (*os.Root, error) {
	volume := filepath.VolumeName(abs)
	anchor := volume + string(filepath.Separator)
	r, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, err
	}
	rest := strings.TrimPrefix(abs, anchor)
	if rest == "" {
		return r, nil
	}
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		info, e := r.Lstat(part)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			r.Close()
			return nil, fmt.Errorf("parent must be an existing real directory: %s: %w", part, errOr(e, fs.ErrInvalid))
		}
		next, e := r.OpenRoot(part)
		if e != nil {
			r.Close()
			return nil, e
		}
		current, e := next.Stat(".")
		if e != nil || !os.SameFile(info, current) {
			next.Close()
			r.Close()
			return nil, fmt.Errorf("parent changed while opening: %w", errOr(e, fs.ErrInvalid))
		}
		if e = r.Close(); e != nil {
			next.Close()
			return nil, e
		}
		r = next
	}
	return r, nil
}
func errOr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
func contained(a, b string) bool { // fold conservatively even on case-sensitive hosts
	a = strings.ToLower(filepath.Clean(a))
	b = strings.ToLower(filepath.Clean(b))
	rel, err := filepath.Rel(a, b)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func validateDestination(destination string, sources []string) (string, error) {
	if destination == "" || len(destination) > 4096 || strings.ContainsRune(destination, 0) || !filepath.IsAbs(destination) || filepath.Clean(destination) != destination {
		return "", fmt.Errorf("destination must be a clean absolute path of at most 4096 bytes")
	}
	if err := portableLeaf(filepath.Base(destination)); err != nil {
		return "", err
	}
	if len(sources) > 64 {
		return "", fmt.Errorf("too many protected source roots")
	}
	for _, source := range sources {
		if !filepath.IsAbs(source) || filepath.Clean(source) != source || len(source) > 4096 {
			return "", fmt.Errorf("source roots must be bounded clean absolute paths")
		}
		if contained(source, destination) || contained(destination, source) {
			return "", fmt.Errorf("source and output overlap")
		}
		// Resolve declared sources as well to reject aliases into the output parent.
		resolved, err := filepath.EvalSymlinks(source)
		if err != nil {
			return "", fmt.Errorf("resolve protected source: %w", err)
		}
		if contained(resolved, destination) || contained(destination, resolved) {
			return "", fmt.Errorf("source and output overlap")
		}
	}
	return filepath.Dir(destination), nil
}
func planDirectories(files []File) []string {
	set := map[string]bool{}
	for _, f := range files {
		for d := path.Dir(f.Path); d != "."; d = path.Dir(d) {
			set[d] = true
		}
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	// Lexical order always puts a parent before its children.
	sort.Strings(dirs)
	return dirs
}
