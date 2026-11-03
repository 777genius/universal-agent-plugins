package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

func validRoot(p string) bool {
	if p == "" || !utf8.ValidString(p) || len(p) > 4096 || !filepath.IsAbs(p) || filepath.Clean(p) != p || strings.ContainsRune(p, 0) {
		return false
	}
	volume := filepath.VolumeName(p)
	return !strings.HasPrefix(volume, `\\`) && !strings.HasPrefix(volume, `//`)
}

// overlappingRoots reports whether two owned paths are the same location or
// nested, including case, symlink, and Unicode aliases when the filesystem
// presents them as the same identity.
func overlappingRoots(a, b string) bool {
	left, right := canonicalRoot(a), canonicalRoot(b)
	if left != "" && right != "" {
		if left == right || containedRoot(left, right) || containedRoot(right, left) {
			return true
		}
	}
	return identityOverlap(a, b)
}

func identityOverlap(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	for cur := filepath.Clean(a); ; {
		if sameExistingFile(cur, b) {
			return true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	for cur := filepath.Clean(b); ; {
		if sameExistingFile(cur, a) {
			return true
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return false
}

func sameExistingFile(a, b string) bool {
	left, err := os.Stat(a)
	if err != nil {
		return false
	}
	right, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(left, right)
}

func containedRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func canonicalRoot(p string) string {
	p = filepath.Clean(p)
	if eval, err := filepath.EvalSymlinks(p); err == nil {
		p = eval
	}
	switch runtime.GOOS {
	case "windows", "darwin":
		return strings.ToLower(p)
	default:
		return p
	}
}
