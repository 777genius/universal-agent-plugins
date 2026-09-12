//go:build windows && (amd64 || arm64)

package packageview

import "os"

// postOpenInfo binds the reopened file to its genuine, lease-owned observation.
// A fresh Windows File.Stat lacks the registered native link/change metadata
// required by legacyGuard. same checks its identity and ordinary metadata and
// revalidates the protected pin's complete native snapshot before we use p.info.
// legacyGuard then rechecks that live snapshot and its link count. Nothing is
// registered here; unknown FileInfo values must continue to fail closed.
func (p *pinned) postOpenInfo(f *os.File) (os.FileInfo, error) {
	opened, err := f.Stat()
	if err != nil || !same(p.info, opened) {
		return nil, fail("source_changed")
	}
	return p.info, nil
}
