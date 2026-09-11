//go:build windows && (amd64 || arm64)

package packageview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// This is an independent, bounded diagnostic, not a retry of Reader.Open.
// Acquisition calls the production resolver with metadata-only observations.
// Sanitized errors are reported as such; optional native overlays supply deeper
// causality without maintaining a second resolver in this test.
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

// Observe the actual production resolver. A copied walker can silently diverge
// from source-root role and namespace protection, invalidating this diagnostic.
func windowsFinalCleanupSource(t *testing.T, role, name string) (*source, error) {
	open := openSourceWithMetadataStage
	if role == "scratch" {
		open = openTrustedScratchWithMetadataStage
	}
	s, err := open(name, func(f *os.File, stage string) {
		snapshot, e := winMeta(f)
		t.Logf("stage=%s.%s observation=%+v err=%v (not internal comparison evidence)", role, stage, snapshot, e)
	})
	return s, windowsFinalCleanupStage(t, role+".openSource", err)
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
