package nativeimport

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ValidateOutput performs the read-only output checks shared by plan and write.
// Apply repeats all mutation-sensitive checks inside scaffold's pinned staging
// and absence-preserving publication boundary.
func ValidateOutput(p Plan, output string) error {
	if p.source == "" || !filepath.IsAbs(output) || filepath.Clean(output) != output || output == p.source {
		return fail("source_output_overlap")
	}
	if _, err := os.Lstat(output); err == nil {
		return fail("output_exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail("output_unavailable")
	}
	parent := filepath.Dir(output)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fail("output_parent_invalid")
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fail("output_parent_invalid")
	}
	candidate := filepath.Join(resolved, filepath.Base(output))
	if pathContains(candidate, p.source) || pathContains(p.source, candidate) {
		return fail("source_output_overlap")
	}
	return nil
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && rel != "." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
