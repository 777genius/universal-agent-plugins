//go:build windows && amd64

package packageview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsScratchPhysicalAliases(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "source")
	nativeWrite(t, root, "plugin.json", "core")
	nativeWrite(t, root, "nested/keep", "inert")
	sibling := filepath.Join(parent, "source-sibling")
	if e := os.Mkdir(sibling, 0700); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name, target string
		overlap      bool
		ancestorLink bool
	}{
		{"same", root, true, false}, {"descendant", filepath.Join(root, "nested"), true, false},
		{"sibling", sibling, false, false}, {"ancestor", parent, false, false},
		{"ancestor-junction-same", root, true, true},
		{"ancestor-junction-sibling", sibling, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alias := filepath.Join(t.TempDir(), "scratch-alias")
			if tc.ancestorLink {
				nativeJunction(t, alias, parent)
				alias = filepath.Join(alias, filepath.Base(tc.target))
			} else {
				nativeJunction(t, alias, tc.target)
			}
			before := winRecordCount()
			opened := 0
			l, e := (Reader{TempDir: alias}).open(context.Background(), root, &captureHooks{beforeDataOpen: func(string) { opened++ }})
			if tc.overlap {
				if l != nil {
					l.Close()
				}
				var safe *Error
				if !errors.As(e, &safe) || safe.Code != "scratch_overlaps_source" || opened != 0 {
					t.Fatalf("alias overlap reached data: opens=%d err=%v", opened, e)
				}
			} else {
				if e != nil {
					t.Fatal(e)
				}
				if opened != 1 || l.Data().Plugin.State != Present {
					t.Fatal("disjoint alias did not read source")
				}
				if e := l.Close(); e != nil {
					t.Fatal(e)
				}
			}
			if winRecordCount() != before {
				t.Fatal("scratch ancestry leaked")
			}
			entries, e := os.ReadDir(tc.target)
			if e != nil {
				t.Fatal(e)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "packageview-") {
					t.Fatal("overlap wrote scratch or close leaked it")
				}
			}
		})
	}
}

func TestWindowsScratchAliasFailuresStayBeforeData(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	for _, name := range []string{"cycle", "missing-target", "unknown-reparse"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			alias := filepath.Join(parent, "scratch-alias")
			switch name {
			case "cycle":
				nativeJunction(t, alias, alias)
			case "missing-target":
				nativeJunction(t, alias, filepath.Join(parent, "missing"))
			case "unknown-reparse":
				nativeWrite(t, parent, "scratch-alias", "")
				// Inert NTFS LX_FIFO metadata, never a live special endpoint.
				setNativeReparse(t, alias, 0x80000024, nil)
			}
			before := winRecordCount()
			l, e := (Reader{TempDir: alias}).open(context.Background(), root, &captureHooks{beforeDataOpen: func(string) { t.Fatal("invalid scratch reached source data") }})
			if l != nil {
				l.Close()
			}
			var safe *Error
			if !errors.As(e, &safe) || safe.Code != "scratch_unavailable" {
				t.Fatalf("invalid scratch: %v", e)
			}
			if winRecordCount() != before {
				t.Fatal("invalid scratch leaked ancestry")
			}
			entries, e := os.ReadDir(parent)
			if e != nil || len(entries) != 1 || entries[0].Name() != "scratch-alias" {
				t.Fatal("invalid scratch changed fixture parent", e)
			}
		})
	}
}

func TestWindowsScratchIdentityUnavailableFailsClosed(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	s := nativeSource(t, root)
	closed, e := winDup(s.anchor)
	if e != nil {
		t.Fatal(e)
	}
	closed.Close()
	for _, pair := range [][2]*os.File{{nil, s.anchor}, {s.anchor, nil}, {closed, s.anchor}, {s.anchor, closed}} {
		if e := winScratchDisjoint(pair[0], pair[1]); e == nil {
			t.Fatal("unavailable handle identity accepted")
		}
	}
	for _, scratch := range []string{filepath.Join(root, "missing"), filepath.Join(root, "plugin.json")} {
		l, e := (Reader{TempDir: scratch}).open(context.Background(), root, &captureHooks{beforeDataOpen: func(string) { t.Fatal("data before separation") }})
		if l != nil {
			l.Close()
		}
		var safe *Error
		if !errors.As(e, &safe) || safe.Code != "scratch_unavailable" {
			t.Fatalf("unavailable scratch: %v", e)
		}
	}
}

func TestWindowsScratchAncestryHeld(t *testing.T) {
	for _, name := range []string{"direct", "junction"} {
		t.Run(name, func(t *testing.T) {
			root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
			parent := t.TempDir()
			scratch := filepath.Join(parent, "scratch")
			if e := os.Mkdir(scratch, 0700); e != nil {
				t.Fatal(e)
			}
			tempDir := scratch
			if name == "junction" {
				tempDir = filepath.Join(t.TempDir(), "scratch-alias")
				nativeJunction(t, tempDir, scratch)
			}
			l, e := (Reader{TempDir: tempDir}).Open(context.Background(), root)
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			if e := os.Rename(scratch, scratch+"-moved"); !errors.Is(e, windows.ERROR_SHARING_VIOLATION) {
				t.Fatalf("scratch ancestry was not held: %v", e)
			}
			if e := l.Close(); e != nil {
				t.Fatal(e)
			}
			if e := os.Rename(scratch, scratch+"-moved"); e != nil {
				t.Fatal("scratch handle leaked", e)
			}
		})
	}
}

func TestWindowsScratchDistinctNTFSVolumes(t *testing.T) {
	scratch := t.TempDir()
	s := nativeSource(t, scratch)
	volume, _, e := winDirectoryIdentity(s.anchor)
	if e != nil {
		t.Fatal(e)
	}
	// Inspect local fixed disks, and write only uniquely created fixture children.
	// No drive mapping, mount, volume policy, service or privilege changes.
	mask, e := windows.GetLogicalDrives()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		drive := string(rune('A'+i)) + `:\`
		u, _ := windows.UTF16PtrFromString(drive)
		var fs [32]uint16
		if windows.GetDriveType(u) != windows.DRIVE_FIXED || windows.GetVolumeInformation(u, nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))) != nil || windows.UTF16ToString(fs[:]) != "NTFS" {
			continue
		}
		fixture, e := os.MkdirTemp(drive, "uap-path-fix-*")
		if e != nil {
			t.Logf("fresh fixture unavailable on %s: %v", drive, e)
			continue
		}
		t.Cleanup(func() {
			if e := os.RemoveAll(fixture); e != nil {
				t.Error(e)
			}
		})
		other := nativeSource(t, fixture)
		otherVolume, _, e := winDirectoryIdentity(other.anchor)
		if e != nil {
			t.Fatal(e)
		}
		if strings.EqualFold(volume, otherVolume) {
			continue
		}
		t.Logf("physical NTFS volumes source=%s scratch=%s", otherVolume, volume)
		nativeWrite(t, fixture, "demo/plugin.json", "core")
		l, e := (Reader{TempDir: scratch}).Open(context.Background(), filepath.Join(fixture, "demo"))
		if e != nil {
			t.Fatal(e)
		}
		defer l.Close()
		if l.Data().Plugin.State != Present {
			t.Fatal("cross-volume core unavailable")
		}
		if _, e := l.Capture(context.Background()); e != nil {
			t.Fatal(e)
		}
		if e := l.Close(); e != nil {
			t.Fatal(e)
		}
		entries, e := os.ReadDir(scratch)
		if e != nil || len(entries) != 0 {
			t.Fatal("cross-volume scratch leaked", e)
		}
		// The alias spelling lives on the other volume, but its physical target
		// overlaps source. A drive-letter comparison would authorize a source write.
		alias := filepath.Join(t.TempDir(), "cross-volume-scratch-alias")
		nativeJunction(t, alias, filepath.Join(fixture, "demo"))
		l, e = (Reader{TempDir: alias}).open(context.Background(), filepath.Join(fixture, "demo"), &captureHooks{beforeDataOpen: func(string) { t.Fatal("cross-volume alias reached source data") }})
		if l != nil {
			l.Close()
		}
		var safe *Error
		if !errors.As(e, &safe) || safe.Code != "scratch_overlaps_source" {
			t.Fatalf("cross-volume alias was not rejected: %v", e)
		}
		entries, e = os.ReadDir(filepath.Join(fixture, "demo"))
		if e != nil || len(entries) != 1 || entries[0].Name() != "plugin.json" {
			t.Fatal("cross-volume alias wrote into source", e)
		}
		return
	}
	t.Skip("UNPROVEN: two writable, physically distinct local fixed NTFS volumes required")
}
