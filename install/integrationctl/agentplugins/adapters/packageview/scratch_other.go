//go:build !windows || !amd64

package packageview

import "path/filepath"

// Preserve the existing Linux/Darwin path and error semantics.
func scratchParent(_ *source, exactRoot, tempDir string) (string, func() error, error) {
	// Resolve scratch parents only to reject overlap; source I/O remains rooted.
	src, err := filepath.EvalSymlinks(exactRoot)
	if err != nil {
		return "", nil, fail("root_unreadable")
	}
	tmp, err := filepath.EvalSymlinks(tempDir)
	if err != nil {
		return "", nil, fail("scratch_unavailable")
	}
	src, err = filepath.Abs(src)
	if err != nil {
		return "", nil, fail("root_unreadable")
	}
	tmp, err = filepath.Abs(tmp)
	if err != nil {
		return "", nil, fail("scratch_unavailable")
	}
	rel, err := filepath.Rel(src, tmp)
	if err != nil || rel == "." || (rel != ".." && !isParentRelative(rel)) {
		return "", nil, fail("scratch_overlaps_source")
	}
	return tmp, nil, nil
}
