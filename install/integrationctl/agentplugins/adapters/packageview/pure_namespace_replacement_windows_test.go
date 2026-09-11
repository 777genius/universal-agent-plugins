//go:build windows && (amd64 || arm64)

package packageview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsPureNamespaceReplacement(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before_name_check", true: "after_name_check"}[after], func(t *testing.T) {
			const body = `{"name":"pure-namespace-fixture"}`
			root := nativeFixture(t, func(root string) {
				nativeWrite(t, root, "plugin.json", body)
				nativeWrite(t, root, "replacement", body)
			})
			scratch := t.TempDir()
			name := filepath.Join(root, "plugin.json")
			moved := filepath.Join(root, "held-original")
			before := winRecordCount()
			attempted, replaced := false, false
			var setupErr error
			replace := func(rel string) {
				if rel != "plugin.json" || attempted {
					return
				}
				attempted = true
				// Only directory entries change. Equal bytes and no forced metadata
				// mutation keep content/readonly checks from standing in for identity.
				if err := nativeSharedRename(name, moved); err != nil {
					setupErr = fmt.Errorf("move pinned original: %w", err)
					return
				}
				if err := nativeSharedRename(filepath.Join(root, "replacement"), name); err != nil {
					setupErr = fmt.Errorf("install ordinary replacement: %w", err)
					return
				}
				if err := nativeDistinctReplacement(moved, name); err != nil {
					setupErr = fmt.Errorf("calibrate distinct replacement identity: %w", err)
					return
				}
				replaced = true
			}
			// Return through Reader's error cleanup; never Fatal/Goexit in a hook.
			hooks := &captureHooks{dataOpenError: func(string) error {
				if setupErr != nil {
					return fail("replacement_setup_failed")
				}
				return nil
			}}
			if after {
				hooks.afterNameCheck = replace
			} else {
				hooks.beforeDataOpen = replace
			}
			// Exercise the read path's name checks. A same-object handle reopen
			// alone may still read the moved original; that separate contract stands.
			l, err := (Reader{TempDir: scratch}).open(context.Background(), root, hooks)
			if l != nil {
				t.Error("replacement returned a lease")
				if closeErr := l.Close(); closeErr != nil {
					t.Errorf("unexpected lease cleanup: %v", closeErr)
				}
			}
			if setupErr != nil {
				t.Errorf("UNPROVEN pure namespace replacement setup: %v", setupErr)
			}
			var safe *Error
			if !attempted || !replaced || !errors.As(err, &safe) || safe.Code != "source_changed" || safe.CleanupFailed {
				t.Errorf("want calibrated replacement and source_changed with safe unwind: attempted=%t replaced=%t error=%v", attempted, replaced, err)
			}
			if got := winRecordCount(); got != before {
				t.Errorf("replacement retained source metadata records: before=%d after=%d", before, got)
			}
			if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
				t.Errorf("replacement leaked private scratch: entries=%v error=%v", entries, err)
			}
			if replaced {
				for _, path := range []string{moved, name} {
					if got, err := os.ReadFile(path); err != nil || string(got) != body {
						t.Errorf("fixture bytes changed at %s: bytes=%q error=%v", filepath.Base(path), got, err)
					}
				}
			}
			// Directory pins deny delete sharing; both roots must move after unwind.
			for _, path := range []string{root, scratch} {
				if err := os.Rename(path, path+"-released"); err != nil {
					t.Errorf("replacement retained directory handle for %s: %v", path, err)
				} else if err := os.Rename(path+"-released", path); err != nil {
					t.Errorf("restore disposable directory: %v", err)
				}
			}
		})
	}
}
