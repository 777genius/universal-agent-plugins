//go:build windows && (amd64 || arm64)

package packageview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// This is an independent, bounded diagnostic, not a retry of Reader.Open.
// Stage acquisition uses production primitives before openSource/scratchParent
// sanitize their errors. The fixture has only literal directory components;
// this helper is not a replacement resolver or an alternate reader profile.
// Native codes are logged only when the called primitive actually returns one.
func TestWindowsFinalCleanupAcquisitionStages(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	for i := 1; i <= 10; i++ {
		t.Run(fmt.Sprintf("iteration-%02d", i), func(t *testing.T) {
			before := winRecordCount()
			scratch := t.TempDir()
			t.Logf("independent staged attempt=%d/10 source=%q scratch=%q source_chars=%d scratch_chars=%d", i, root, scratch, len(root), len(scratch))
			err := windowsFinalCleanupAttempt(t, root, scratch)
			// The attempt has already unwound all ownership, even on an early
			// stage error. Keep leak checks independent of the expected failure.
			if got := winRecordCount(); got != before {
				t.Errorf("metadata records before=%d after=%d", before, got)
			}
			entries, readErr := os.ReadDir(scratch)
			if readErr != nil || len(entries) != 0 {
				t.Errorf("scratch cleanup entries=%d err=%v", len(entries), readErr)
			}
			for _, name := range []string{scratch, root} {
				if e := os.Rename(name, name+"-moved"); e != nil {
					t.Errorf("released directory rename %q: %v", name, e)
				} else if e := os.Rename(name+"-moved", name); e != nil {
					t.Fatalf("restore fixture directory %q: %v", name, e)
				}
			}
			var safe *Error
			if !errors.As(err, &safe) || safe.Code != "byte_limit" || safe.CleanupFailed {
				t.Fatalf("staged attempt expected byte_limit with successful cleanup: %v", err)
			}
		})
	}
}

func windowsFinalCleanupStage(t *testing.T, stage string, err error) error {
	t.Helper()
	if err == nil {
		t.Logf("stage=%s PASS", stage)
		return nil
	}
	var status windows.NTStatus
	var errno syscall.Errno
	switch {
	case errors.As(err, &status):
		t.Logf("stage=%s FAIL type=%T NTSTATUS=0x%08x Win32=%d error=%v", stage, err, uint32(status), uint32(status.Errno()), err)
	case errors.As(err, &errno):
		t.Logf("stage=%s FAIL type=%T Win32=%d (0x%08x) error=%v", stage, err, uint32(errno), uint32(errno), err)
	default:
		t.Logf("stage=%s FAIL type=%T native_code=not_exposed error=%v", stage, err, err)
	}
	return fmt.Errorf("%s: %w", stage, err)
}

func windowsFinalCleanupSource(t *testing.T, role, name string) (_ *source, err error) {
	name = strings.ReplaceAll(name, "/", `\`)
	if len(name) < 3 || name[1] != ':' || name[2] != '\\' || !((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
		return nil, windowsFinalCleanupStage(t, role+".literal-drive", fail("root_unreadable"))
	}
	u, e := windows.UTF16PtrFromString(name[:3])
	if err = windowsFinalCleanupStage(t, role+".drive-utf16", e); err != nil {
		return nil, err
	}
	kind := windows.GetDriveType(u)
	t.Logf("stage=%s.drive-type kind=%d", role, kind)
	if kind != windows.DRIVE_FIXED {
		return nil, windowsFinalCleanupStage(t, role+".drive-type", fail("filesystem_unavailable"))
	}
	f, e := winOpen(0, `\??\`+name[:3], true)
	if err = windowsFinalCleanupStage(t, role+".bootstrap.winOpen", e); err != nil {
		return nil, err
	}
	s := &source{anchor: f, records: make(map[winSnapshot]*winObservation)}
	defer func() {
		if err != nil {
			if e := s.close(); e != nil {
				t.Errorf("%s acquisition cleanup: %v", role, e)
			}
		}
	}()
	var fs [32]uint16
	e = windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, &s.volume, nil, nil, &fs[0], uint32(len(fs)))
	if err = windowsFinalCleanupStage(t, role+".volume-information", e); err != nil {
		return nil, err
	}
	t.Logf("stage=%s.filesystem type=%q volume=%08x", role, windows.UTF16ToString(fs[:]), s.volume)
	if windows.UTF16ToString(fs[:]) != "NTFS" {
		return nil, windowsFinalCleanupStage(t, role+".filesystem", fail("filesystem_unavailable"))
	}
	p, err := windowsFinalCleanupRemember(t, role+".bootstrap.remember", s, f)
	if err != nil {
		return nil, err
	}
	if e = winCheckDirectory(p.file); e != nil {
		p.file.Close()
		return nil, windowsFinalCleanupStage(t, role+".bootstrap.protected-directory", e)
	}
	s.anchor.Close()
	s.anchor = p.file
	rest := strings.TrimRight(name[3:], `\`)
	if rest == "" {
		return s, nil
	}
	parts, e := winParts(rest)
	if err = windowsFinalCleanupStage(t, role+".parts", e); err != nil {
		return nil, err
	}
	for i, part := range parts {
		// Only the fresh fixture's simple components may use staged walking.
		if part == "" || part == "." || part == ".." {
			return nil, windowsFinalCleanupStage(t, role+".fixture-component", syscall.EXDEV)
		}
		stage := fmt.Sprintf("%s.component[%d]=%q", role, i, part)
		// Match walk's root-selection validation for literal fixture names.
		if !filepath.IsLocal(part) || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || strings.ContainsAny(part, "<>|") {
			return nil, windowsFinalCleanupStage(t, stage+".name", syscall.EXDEV)
		}
		f, e := winOpen(windows.Handle(s.anchor.Fd()), part, false)
		if err = windowsFinalCleanupStage(t, stage+".winOpen", e); err != nil {
			return nil, err
		}
		p, e = windowsFinalCleanupRemember(t, stage+".remember", s, f)
		f.Close()
		if e != nil {
			return nil, e
		}
		if p.info.Mode()&os.ModeSymlink != 0 || !p.info.IsDir() {
			p.file.Close()
			return nil, windowsFinalCleanupStage(t, stage+".directory", fail("root_wrong_kind"))
		}
		s.anchor.Close()
		s.anchor = p.file
	}
	return s, nil
}

// These snapshots surround the real remember call. They are NOT its internal
// snapshots and do not establish which comparison failed. No path is reopened
// to obtain them, and the diagnostic never retries remember after failure.
func windowsFinalCleanupRemember(t *testing.T, stage string, s *source, f *os.File) (*pinned, error) {
	before, beforeErr := winMeta(f)
	p, err := s.rememberMustDuplicate(f)
	if err != nil {
		after, afterErr := winMeta(f)
		t.Logf("stage=%s surrounding snapshots before=%+v err=%v after=%+v err=%v (not internal comparison evidence)", stage, before, beforeErr, after, afterErr)
	}
	return p, windowsFinalCleanupStage(t, stage, err)
}

func windowsFinalCleanupAttempt(t *testing.T, root, scratch string) (err error) {
	s, err := windowsFinalCleanupSource(t, "source", root)
	if err != nil {
		return err
	}
	limits, err := (Limits{PluginBytes: 1}).bounded()
	if err != nil {
		s.close()
		return err
	}
	l := &Lease{source: s, limits: limits, observations: map[string]os.FileInfo{}, linkInfos: map[string]os.FileInfo{}, directoryEntries: map[string][]string{}, contents: map[string][]byte{}}
	defer l.finish(&err)
	tmp, e := winResolveScratch(scratch)
	if err = windowsFinalCleanupStage(t, "scratch.resolve", e); err != nil {
		return err
	}
	t.Logf("resolved scratch=%q chars=%d", tmp, len(tmp))
	held, err := windowsFinalCleanupSource(t, "scratch", tmp)
	if err != nil {
		return err
	}
	l.scratchClose = held.close
	// Use the actual identity/overlap primitive. It sanitizes its own native
	// queries: do not report a later identity query as that call's raw error.
	if err = windowsFinalCleanupStage(t, "scratch.physical-disjoint", winScratchDisjoint(s.anchor, held.anchor)); err != nil {
		return err
	}
	l.private, e = os.MkdirTemp(tmp, "packageview-*")
	if err = windowsFinalCleanupStage(t, "scratch.MkdirTemp", e); err != nil {
		return err
	}
	l.privateInfo, e = os.Lstat(l.private)
	if err = windowsFinalCleanupStage(t, "scratch.private.Lstat", e); err != nil {
		return err
	}
	l.input = Input{Plugin: Document{Path: "plugin.json", State: Blocked}, MCP: Document{Path: "mcp.json", State: Blocked}, SkillsRoot: Blocked}
	l.input.Legacy, e = l.legacy()
	if err = windowsFinalCleanupStage(t, "source.legacy", e); err != nil {
		return err
	}
	l.input.Plugin, e = l.document(context.Background(), "plugin.json", limits.PluginBytes)
	if e == nil {
		e = fmt.Errorf("unexpected success with PluginBytes=1 and four-byte fixture")
	}
	return windowsFinalCleanupStage(t, "source.plugin.document", e)
}
