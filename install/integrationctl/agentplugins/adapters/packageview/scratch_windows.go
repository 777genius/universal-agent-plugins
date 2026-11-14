//go:build windows && (amd64 || arm64)

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
	tmp, e := winResolveScratch(tempDir)
	if e != nil {
		return "", nil, fail("scratch_unavailable")
	}
	scratch, e := openTrustedScratchWithMetadataStage(tmp, nil)
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

// Only trusted scratch configuration may select this purpose. The ordinary
// source APIs remain strict even when a source happens to live in a temp path.
func openTrustedScratchWithMetadataStage(name string, stage func(*os.File, string)) (*source, error) {
	return openWindowsRoot(name, winTrustedScratch, stage)
}

// winResolveScratch is only for the trusted, cleanable scratch configuration.
// Go 1.23+ reports junctions as ModeIrregular, so EvalSymlinks leaves a leaf
// junction unresolved (and rejects one in an ancestor). Acquisition must still
// reject those reparse points. Resolve known links here using metadata only,
// then acquire and protect the entire resolved directory ancestry.
// Never use this resolver for a source path or as physical identity evidence.
func winResolveScratch(name string) (string, error) {
	name, e := filepath.Abs(name)
	if e != nil {
		return "", e
	}
	steps := 0
	for links := 0; links <= 255; links++ {
		// Stay within the existing local drive scratch boundary. In particular,
		// Readlink returning a UNC, volume GUID or device target is not permission
		// to acquire that namespace, even though scratch itself is trusted.
		if len(name) < 3 || name[1] != ':' || name[2] != '\\' || !((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
			return "", fail("scratch_unavailable")
		}
		parts, e := winParts(name[3:])
		if name[3:] == "" {
			return name, nil
		}
		if e != nil {
			return "", e
		}
		resolved := name[:3]
		linked := false
		for i, part := range parts {
			steps++
			if steps > 8192 {
				return "", fail("scratch_unavailable")
			}
			next := filepath.Join(resolved, part)
			info, e := os.Lstat(next)
			if e != nil {
				return "", e
			}
			if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				// Readlink opens with no data access and OPEN_REPARSE_POINT;
				// it accepts only SYMLINK/MOUNT_POINT tags, not unknown reparses.
				target, e := os.Readlink(next)
				if e != nil {
					return "", e
				}
				if filepath.VolumeName(target) == "" {
					if strings.HasPrefix(target, `\`) || strings.HasPrefix(target, "/") {
						target = name[:2] + target
					} else {
						target = filepath.Join(resolved, target)
					}
				}
				name = filepath.Join(append([]string{target}, parts[i+1:]...)...)
				linked = true
				break
			}
			if !info.IsDir() {
				return "", fail("scratch_unavailable")
			}
			resolved = next
		}
		if !linked {
			return resolved, nil
		}
	}
	return "", fail("scratch_unavailable")
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
