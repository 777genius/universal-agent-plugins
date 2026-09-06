//go:build windows && amd64

package packageview

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"
)

func scratchParent(s *source, _ string, tempDir string) (_ string, release func() error, err error) {
	// Scratch is trusted configuration. Resolve its aliases, then hold the same
	// no-reparse directory ancestry as source acquisition, through private cleanup.
	// Source identity comes only from the already acquired handle, never a second
	// source pathname lookup. Neither identity query requests file data or writes.
	tmp, e := filepath.EvalSymlinks(tempDir)
	if e != nil {
		return "", nil, fail("scratch_unavailable")
	}
	tmp, e = filepath.Abs(tmp)
	if e != nil {
		return "", nil, fail("scratch_unavailable")
	}
	scratch, e := openSource(tmp)
	if e != nil {
		return "", nil, fail("scratch_unavailable")
	}
	defer func() {
		if err != nil {
			_ = scratch.close()
		}
	}()
	if e = winScratchDisjoint(s.anchor, scratch.anchor); e != nil {
		return "", nil, e
	}
	// Use the ordinary trusted scratch spelling for writes. GUID names are
	// identity evidence only, never an alternate namespace acquisition route.
	return tmp, scratch.close, nil
}

func winScratchDisjoint(source, scratch *os.File) error {
	sv, sp, e := winDirectoryIdentity(source)
	if e != nil {
		return fail("root_unreadable")
	}
	tv, tp, e := winDirectoryIdentity(scratch)
	if e != nil {
		return fail("scratch_unavailable")
	}
	if !strings.EqualFold(sv, tv) {
		return nil // distinct kernel-resolved volume GUIDs, not drive letters/serials
	}
	// On one volume, canonical long names share held, immovable ancestry. Fold
	// case conservatively (including case-sensitive NTFS directories). A Rel
	// failure never proves disjointness. Scratch must not equal or descend source.
	rel, e := filepath.Rel(strings.ToUpper(sp), strings.ToUpper(tp))
	if e != nil || rel == "." || (rel != ".." && !isParentRelative(rel)) {
		return fail("scratch_overlaps_source")
	}
	return nil
}

func winDirectoryIdentity(f *os.File) (volume, path string, err error) {
	defer runtime.KeepAlive(f)
	if f == nil {
		return "", "", fail("directory_identity_unavailable")
	}
	m, e := winMeta(f)
	if e != nil || m.Attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || m.Attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", "", fail("directory_identity_unavailable")
	}
	// FILE_NAME_NORMALIZED (0) | VOLUME_NAME_GUID (1). A fixed NT path bound
	// prevents unbounded allocation; no DOS/NT namespace fallback on failure.
	buf := make([]uint16, 32768)
	n, e := windows.GetFinalPathNameByHandle(windows.Handle(f.Fd()), &buf[0], uint32(len(buf)), 1)
	if e != nil || n == 0 || n >= uint32(len(buf)) {
		return "", "", fail("directory_identity_unavailable")
	}
	name := windows.UTF16ToString(buf[:n])
	// Require exactly \\?\Volume{GUID}\ followed by a volume-rooted path.
	const prefix = `\\?\Volume`
	if len(name) < len(prefix)+39 || !strings.EqualFold(name[:len(prefix)], prefix) || name[len(prefix)+38] != '\\' {
		return "", "", fail("directory_identity_unavailable")
	}
	guid := name[len(prefix) : len(prefix)+38]
	if _, e := windows.GUIDFromString(guid); e != nil {
		return "", "", fail("directory_identity_unavailable")
	}
	return guid, name[len(prefix)+38:], nil
}
