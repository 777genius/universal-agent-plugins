//go:build darwin && arm64

package directoryidentity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Regression: wrong bitmap count/field layout emits EINVAL or silently requests
// another attribute. Packed Linux-host decoder vectors cannot prove native Attrlist ABI.
func TestDarwinNativeRequestABI(t *testing.T) {
	a := unix.Attrlist{Bitmapcount: 5, Commonattr: darwinReturnedAttrs, Volattr: darwinVolumeInfo | darwinVolumeUUID}
	if unsafe.Sizeof(a) != 24 || unsafe.Offsetof(a.Commonattr) != 4 || unsafe.Offsetof(a.Volattr) != 8 || a.Bitmapcount != 5 || a.Commonattr != 0x80000000 || a.Volattr != 0x80040000 {
		t.Fatalf("request layout: %#v size=%d offsets=%d/%d", a, unsafe.Sizeof(a), unsafe.Offsetof(a.Commonattr), unsafe.Offsetof(a.Volattr))
	}
	root := t.TempDir()
	f, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := canonicalize(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCanonicalName(f, resolved); err != nil {
		t.Fatalf("native held canonical spelling: %v", err)
	}
	if err := verifyCanonicalName(f, resolved+"-alias"); err == nil {
		t.Fatal("different native spelling accepted")
	}
	_, _, observeErr := directoryFacts(f)
	if observeErr != nil && !errors.Is(observeErr, ErrUnsupported) {
		t.Fatalf("native fgetattrlist (failure, not unsupported): %v", observeErr)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := directoryFacts(f); err == nil || errors.Is(err, ErrUnsupported) {
		t.Fatalf("closed fd classified as supported/unsupported: %v", err)
	}
	t.Logf("ACTUAL_DARWIN_REQUEST=%v; native APFS floor still requires host tuple evidence", observeErr)
}

// Regression: Openat follows a replaced link instead of refusing traversal.
// Cross-building the directory primitive proves no native APFS path behavior.
func TestDarwinAnchoredNoFollow(t *testing.T) {
	root := fixtureDirectory(t)
	parent, err := openDirectory(nil, filepath.Dir(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"-old", root); err != nil {
		t.Fatal(err)
	}
	f, err := openDirectory(parent, "profile")
	if err == nil {
		_ = f.Close()
		t.Fatal("native nofollow followed link")
	}
}
