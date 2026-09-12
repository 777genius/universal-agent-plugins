//go:build darwin && arm64

package packageview

import "strings"

// Resolve scratch through the same directory-relative local APFS admission.
// Compare against the selected root identity, not a newly resolved source argv.
func scratchParent(s *source, _ string, tempDir string) (string, func() error, error) {
	var hooks *captureHooks
	if s.hooks != nil {
		hooks = &captureHooks{nativeOpen: s.hooks.scratchOpen}
	}
	scratch, e := openSelectedDirectory(s.ctx, tempDir, false, hooks)
	if e != nil {
		return "", nil, acquisitionError(e, "scratch_unavailable")
	}
	// Own the anchor until successful transfer to the lease. In particular,
	// source binding replay below can panic before scratchClose is assigned.
	transferred := false
	defer func() {
		if !transferred {
			_ = scratch.close()
		}
	}()
	release := func() error { return scratch.close() }
	if e := s.verifyBindings(); e != nil {
		return "", nil, e
	}
	for _, info := range scratch.bindings {
		if sameIdentity(info, s.bindings[s.physical]) {
			return "", nil, fail("scratch_overlaps_source")
		}
	}
	if scratch.physical == s.physical || strings.HasPrefix(scratch.physical, s.physical+"/") {
		return "", nil, fail("scratch_overlaps_source")
	}
	transferred = true
	return scratch.physical, release, nil
}
