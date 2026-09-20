//go:build windows && (amd64 || arm64)

package commands_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// A junction between new fixture directories needs no symlink privilege and
// changes no system mount or drive mapping. It is never a special endpoint.
func pathFixAlias(t *testing.T, path, target string) {
	t.Helper()
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	u, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(u, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	targetName, err := windows.UTF16FromString(`\??\` + target)
	if err != nil {
		t.Fatal(err)
	}
	// Both names need in-buffer NULs, including the empty print name whose
	// offset follows the substitute name's terminator (as in nativeJunction).
	buf := make([]byte, 16+2*(len(targetName)+1))
	binary.LittleEndian.PutUint32(buf, windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buf[4:], uint16(len(buf)-8))
	binary.LittleEndian.PutUint16(buf[10:], uint16(2*(len(targetName)-1)))
	binary.LittleEndian.PutUint16(buf[12:], uint16(2*len(targetName)))
	for i, c := range targetName {
		binary.LittleEndian.PutUint16(buf[16+2*i:], c)
	}
	var n uint32
	if err := windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &buf[0], uint32(len(buf)), nil, 0, &n, nil); err != nil {
		t.Fatal("new-directory junction fixture unavailable", err)
	}
}

func pathFixFixtureVolume(t *testing.T, path string) string {
	t.Helper()
	u, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(u, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	var info windows.ByHandleFileInformation
	var fs [32]uint16
	if windows.GetFileInformationByHandle(h, &info) != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || windows.GetVolumeInformationByHandle(h, nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))) != nil || windows.UTF16ToString(fs[:]) != "NTFS" {
		t.Fatal("fresh fixture is not an ordinary NTFS directory")
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)), 1)
	if err != nil || n == 0 || n >= uint32(len(buf)) {
		t.Fatal("fixture physical identity unavailable", err)
	}
	name := windows.UTF16ToString(buf[:n])
	const prefix = `\\?\Volume`
	if !strings.HasPrefix(name, prefix) || len(name) < len(prefix)+39 || name[len(prefix)+38] != '\\' {
		t.Fatalf("fixture did not yield a volume GUID path: %q", name)
	}
	volume := name[len(prefix) : len(prefix)+38]
	if _, err := windows.GUIDFromString(volume); err != nil {
		t.Fatal(err)
	}
	return volume
}

func pathFixWindowsContracts(t *testing.T, b pathFixBinaries) {
	t.Run("namespace-rejection", func(t *testing.T) {
		parent, scratch := t.TempDir(), t.TempDir()
		write(t, parent, "demo/plugin.json", plugin(""))
		// All controls hit the prefix gate before any namespace API. No endpoint
		// is created or opened, and no ambient drive-relative cwd is consulted.
		for _, root := range []string{`C:demo`, `C:`, `\demo`, `/demo`, `\\server\share`, `\\?\C:\demo`, `\\.\pipe\fixture`} {
			for i := range b.paths {
				r, code, out := b.run(t, i, parent, scratch, "validate", root)
				if code == 0 || r.Error == nil || r.Error.Code != "root_unreadable" {
					t.Fatalf("namespace passed absolute gate: %q %d %s", root, code, out)
				}
			}
		}
		pathFixNoScratch(t, scratch)
	})
	t.Run("same-volume-overlapping-alias", func(t *testing.T) {
		parent := t.TempDir()
		root := filepath.Join(parent, "demo")
		write(t, root, "plugin.json", plugin(""))
		write(t, root, "nested/keep", "inert")
		before := tree(t, root)
		for _, target := range []string{root, filepath.Join(root, "nested")} {
			alias := filepath.Join(t.TempDir(), "scratch-alias")
			pathFixAlias(t, alias, target)
			if !strings.EqualFold(pathFixFixtureVolume(t, root), pathFixFixtureVolume(t, target)) {
				t.Fatal("overlap control must be on one physical volume")
			}
			for _, command := range []string{"validate", "inspect", "test"} {
				for i := range b.paths {
					r, code, out := b.run(t, i, parent, alias, command, "demo")
					if code == 0 || r.Error == nil || r.Error.Code != "scratch_overlaps_source" || r.Identity.ScopeDigest != "" {
						t.Fatalf("overlapping scratch alias accepted: %d %s", code, out)
					}
				}
			}
			pathFixNoScratch(t, target)
		}
		if !reflect.DeepEqual(tree(t, root), before) {
			t.Fatal("overlap rejection changed source")
		}
	})
	t.Run("two-physical-ntfs-volumes", func(t *testing.T) {
		scratch := t.TempDir()
		volume := pathFixFixtureVolume(t, scratch)
		mask, err := windows.GetLogicalDrives()
		if err != nil {
			t.Fatal(err)
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
			fixture, err := os.MkdirTemp(drive, "uap-path-fix-binaries-*")
			if err != nil {
				t.Logf("fresh fixture unavailable on %s: %v", drive, err)
				continue
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(fixture); err != nil {
					t.Error(err)
				}
			})
			otherVolume := pathFixFixtureVolume(t, fixture)
			if strings.EqualFold(volume, otherVolume) {
				continue
			}
			t.Logf("binary journey physical NTFS volumes source=%s scratch=%s", otherVolume, volume)
			b.journey(t, fixture, scratch)
			cwd := filepath.Join(fixture, "entry-0")
			root := filepath.Join(cwd, "demo")
			// Cwd can itself be a drive root (already separator-terminated). The
			// selected package and every write still belong to fresh fixture dirs.
			for i := range b.paths {
				r, code, out := b.run(t, i, drive, scratch, "validate", filepath.Join(filepath.Base(fixture), "entry-0", "demo"))
				if code != 0 || r.Identity.ScopeDigest == "" {
					t.Fatalf("drive-root cwd prefix failed: %d %s", code, out)
				}
			}
			alias := filepath.Join(t.TempDir(), "cross-volume-scratch-alias")
			pathFixAlias(t, alias, root)
			before := tree(t, root)
			for i := range b.paths {
				r, code, out := b.run(t, i, cwd, alias, "validate", "demo")
				if code == 0 || r.Error == nil || r.Error.Code != "scratch_overlaps_source" {
					t.Fatalf("cross-volume spelling hid overlapping alias: %d %s", code, out)
				}
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("cross-volume alias rejection changed source")
			}
			return
		}
		t.Skip("UNPROVEN: two writable, physically distinct local fixed NTFS volumes required")
	})
}
