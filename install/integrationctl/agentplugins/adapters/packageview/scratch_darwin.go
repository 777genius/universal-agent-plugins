//go:build darwin && arm64

package packageview

import "strings"

// Resolve scratch through the same directory-relative local APFS admission.
// Compare against the selected root identity, not a newly resolved source argv.
func scratchParent(s *source, _ string, tempDir string) (string, func() error, error) {
	scratch, e := openSourceContext(s.ctx, tempDir)
	if e != nil {
		return "", nil, fail("scratch_unavailable")
	}
	release := func() error { return scratch.close() }
	if e := s.verifyBindings(); e != nil {
		release()
		return "", nil, e
	}
	for _, info := range scratch.bindings {
		if sameIdentity(info, s.bindings[s.physical]) {
			release()
			return "", nil, fail("scratch_overlaps_source")
		}
	}
	if scratch.physical == s.physical || strings.HasPrefix(scratch.physical, s.physical+"/") {
		release()
		return "", nil, fail("scratch_overlaps_source")
	}
	return scratch.physical, release, nil
}
