//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestTrustedScannerHeldAncestryRejectsReplacementAndMetadataDrift(t *testing.T) {
	for _, kind := range []string{"directory-replacement", "directory-mode", "directory-alias", "file-replacement", "file-mode", "file-size", "file-hardlink", "wrong-owner", "mount"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			path, mode := filepath.Join(base, "node"), os.FileMode(0700)
			directory := kind == "directory-replacement" || kind == "directory-mode" || kind == "directory-alias"
			if directory {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				mode |= os.ModeDir
			} else if err := os.WriteFile(path, []byte("held original"), 0700); err != nil {
				t.Fatal(err)
			}
			s := &trustedScannerCache{}
			if err := s.hold(path, os.Getuid(), os.Getgid(), mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				for _, n := range s.nodes {
					_ = n.file.Close()
				}
			})
			n := s.node(path)
			if err := n.verify(); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "directory-replacement", "file-replacement", "directory-alias":
				err = os.Rename(path, path+"-held")
				if err == nil {
					switch kind {
					case "directory-replacement":
						err = os.Mkdir(path, 0700)
					case "directory-alias":
						err = os.Symlink(path+"-held", path)
					default:
						err = os.WriteFile(path, []byte("held original"), 0700)
					}
				}
			case "directory-mode", "file-mode":
				err = os.Chmod(path, 0750)
			case "file-size":
				err = os.WriteFile(path, []byte("held original with a different size"), 0700)
			case "file-hardlink":
				err = os.Link(path, path+"-alias")
			case "wrong-owner":
				if err := s.hold(path, os.Getuid()+1, os.Getgid(), mode); err == nil {
					t.Fatal("foreign ownership admitted")
				}
				return
			case "mount":
				n.mount, err = scannerMountID(path)
				n.mount++
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := n.verify(); err == nil {
				t.Fatal("original scanner node drift was adopted", kind)
			}
		})
	}
}

func TestTrustedScannerContentsRejectSameSizeBytesAndInventoryDrift(t *testing.T) {
	for _, kind := range []string{"same-size-bytes", "file-mode", "empty-directory"} {
		t.Run(kind, func(t *testing.T) {
			security := t.TempDir()
			tree := filepath.Join(security, "lintai")
			if err := os.Mkdir(tree, 0700); err != nil {
				t.Fatal(err)
			}
			body := []byte("held original")
			sum := sha256.Sum256(body)
			s := &trustedScannerCache{security: security, binary: filepath.Join(tree, "lintai"), digest: hex.EncodeToString(sum[:])}
			if err := os.WriteFile(s.binary, body, 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			s.inventory, err = trustedInventory(tree)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.verifyContents(); err != nil {
				t.Fatal("original content refused", err)
			}
			want := "sealed scanner subtree differs"
			switch kind {
			case "same-size-bytes":
				before, err := anchoredInfo(s.binary)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(s.binary, []byte("held changed!"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(s.binary, before.ModTime(), before.ModTime()); err != nil {
					t.Fatal(err)
				}
				after, err := anchoredInfo(s.binary)
				if err != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("same-size restored-mtime setup differs", err)
				}
				want = "pinned cached scanner bytes differ"
			case "file-mode":
				err = os.Chmod(s.binary, 0750)
			case "empty-directory":
				err = os.Mkdir(filepath.Join(tree, "unexpected-empty"), 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.verifyContents(); err == nil || err.Error() != want {
				t.Fatalf("content drift %s: got %v, want %q", kind, err, want)
			}
		})
	}
}

func TestTrustedScannerCopyRequiresExactSealedELFAndFreshOrdinaryTarget(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("root-owned scanner-copy contract; hosted UID gate covers privilege separation")
	}
	for _, kind := range []string{"valid", "pin", "not-elf", "source-mode", "source-alias", "source-hardlink", "existing", "target-alias", "target-hardlink", "parent-alias", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			for _, dir := range []string{"executables", "cache"} {
				if err := os.Mkdir(filepath.Join(base, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			body := []byte("\x7fELFexpressly inert source bytes, never executed")
			source, target := filepath.Join(base, "executables", "lintai"), filepath.Join(base, "cache", "lintai")
			if err := os.WriteFile(source, body, 0555); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			c := &controller{work: context.Background(), request: sealedRequest{Base: base, Input: inputRequest{Trusted: &trustedInput{ScannerSHA256: hex.EncodeToString(sum[:])}}}}
			var err error
			switch kind {
			case "pin":
				c.request.Input.Trusted.ScannerSHA256 = "incorrect"
			case "not-elf":
				err = os.Chmod(source, 0700)
				if err == nil {
					err = os.WriteFile(source, []byte("not an ELF"), 0555)
				}
				if err == nil {
					err = os.Chmod(source, 0555)
				}
			case "source-mode":
				err = os.Chmod(source, 0755)
			case "source-alias":
				err = os.Rename(source, source+"-original")
				if err == nil {
					err = os.Symlink(source+"-original", source)
				}
			case "source-hardlink":
				err = os.Link(source, source+"-alias")
			case "existing":
				err = os.WriteFile(target, []byte("must preserve"), 0600)
			case "target-alias":
				err = os.Symlink(source, target)
			case "target-hardlink":
				err = os.Link(source, target)
			case "parent-alias":
				err = os.Rename(filepath.Dir(target), filepath.Dir(target)+"-original")
				if err == nil {
					err = os.Symlink(filepath.Dir(target)+"-original", filepath.Dir(target))
				}
			case "canceled":
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				c.work = ctx
			}
			if err != nil {
				t.Fatal(err)
			}
			err = c.scannerCopy(target)
			if kind != "valid" {
				if err == nil {
					t.Fatal("invalid scanner copy admitted", kind)
				}
				if kind == "existing" {
					got, _ := os.ReadFile(target)
					if string(got) != "must preserve" {
						t.Fatal("existing target replaced")
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := hashFile(target, 128<<20)
			info, statErr := anchoredInfo(target)
			if err != nil || statErr != nil || got != c.request.Input.Trusted.ScannerSHA256 || info.Mode() != 0555 {
				t.Fatal("copied inert scanner identity differs", err, statErr)
			}
		})
	}
}

func TestTrustedScannerExchangeGeometryRejectsNonPermissionFailures(t *testing.T) {
	root := t.TempDir()
	left, right := filepath.Join(root, "left"), filepath.Join(root, "right")
	for _, path := range []string{left, right, filepath.Join(left, "nested")} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := scannerExchangeGeometry(left, right); err != nil {
		t.Fatal("disjoint sibling operands refused", err)
	}
	for _, pair := range [][2]string{{left, left}, {left, filepath.Join(left, "nested")}, {filepath.Join(left, "nested"), left}, {left, "/proc/sys"}} {
		if err := scannerExchangeGeometry(pair[0], pair[1]); err == nil {
			t.Fatal("invalid exchange geometry admitted", pair)
		}
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(right, alias); err != nil {
		t.Fatal(err)
	}
	if err := scannerExchangeGeometry(left, alias); err == nil {
		t.Fatal("exchange alias admitted")
	}
}
