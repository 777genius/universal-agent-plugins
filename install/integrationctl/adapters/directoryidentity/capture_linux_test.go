//go:build linux && (amd64 || arm64)

package directoryidentity

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Regression: ioctl issued on O_PATH, wrong request/length, or a device fallback
// fabricates success. Legacy fstat tests do not query the actual TEST filesystem.
func TestLinuxActualReadableDirectoryFD(t *testing.T) {
	root := t.TempDir()
	f, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	var stat unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &stat); err != nil {
		t.Fatal(err)
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(int(f.Fd()), &fs); err != nil {
		t.Fatal(err)
	}
	var kernel unix.Utsname
	if err := unix.Uname(&kernel); err != nil {
		t.Fatal(err)
	}
	t.Logf("ACTUAL_TUPLE kernel=%s arch=%s fs_magic=%#x inode=%d TEST_root=%s", unix.ByteSliceToString(kernel.Release[:]), runtime.GOARCH, fs.Type, stat.Ino, root)
	// Independent hard-coded request and direct syscall, not the production decoder.
	var raw [17]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), 0x80111500, uintptr(unsafe.Pointer(&raw[0])))
	runtime.KeepAlive(f)
	volume, inode, err := linuxReadableFacts(int(f.Fd()), stat)
	if errno == unix.ENOTTY {
		if !errors.Is(err, ErrUnsupported) || !errors.Is(err, unix.ENOTTY) || volume != "" || inode != "" {
			t.Fatalf("unsupported result: %q %q %v", volume, inode, err)
		}
		t.Logf("ACTUAL_IOCTL=ENOTTY; QUALIFICATION=UNSUPPORTED; raw=%x; no durable grant", raw)
		return
	}
	if errno != 0 {
		t.Fatalf("ACTUAL_IOCTL=%v (failure, not unsupported); production=%v", errno, err)
	}
	if raw[0] < 1 || raw[0] > 16 {
		t.Fatalf("kernel malformed length %d", raw[0])
	}
	expected := hex.EncodeToString(raw[1 : 1+int(raw[0])])
	if err != nil || volume != expected || inode != strconv.FormatUint(stat.Ino, 10) {
		t.Fatalf("metadata mismatch raw=%x volume=%q inode=%q err=%v", raw, volume, inode, err)
	}
	t.Logf("ACTUAL_IOCTL=SUCCESS; fd_tuple=SUPPORTED; volume=%s inode=%s (full namespace requires every transition)", volume, inode)
}

// Regression: an O_PATH ioctl or recycled/closed fd is silently downgraded to
// unsupported. These real-fd errors were absent from legacy identity fixtures.
func TestLinuxPinUpgradeAndClosedFD(t *testing.T) {
	root := t.TempDir()
	pin, err := openDirectory(nil, root)
	if err != nil {
		t.Fatal(err)
	}
	var raw [17]byte
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, pin.Fd(), 0x80111500, uintptr(unsafe.Pointer(&raw)))
	runtime.KeepAlive(pin)
	if errno != unix.EBADF {
		t.Fatalf("O_PATH ioctl: %v", errno)
	}
	_, _, observeErr := directoryFacts(pin)
	if observeErr != nil && !errors.Is(observeErr, ErrUnsupported) {
		t.Fatalf("readable upgrade failed: %v", observeErr)
	}
	if err := pin.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, err = directoryFacts(pin)
	if err == nil || errors.Is(err, ErrUnsupported) {
		t.Fatalf("closed pin: %v", err)
	}
	t.Logf("closed_pin=%v; readable_upgrade=%v", err, observeErr)
}

// Regression: pathname reopen of a child follows its new alias. The public
// canonical alias contract requires frozen spelling, not resumed alias resolution.
func TestUnixAliasFrozenAndMissingAlias(t *testing.T) {
	root := fixtureDirectory(t)
	alias := filepath.Join(filepath.Dir(filepath.Dir(root)), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalize(alias)
	if err != nil || canonical != root {
		t.Fatalf("canonical alias: %q %v", canonical, err)
	}
	original, qualified := captureQualified(t, root)
	viaAlias, err := Capture(context.Background(), alias)
	if !qualified {
		if !errors.Is(err, ErrUnsupported) || !viaAlias.IsZero() {
			t.Fatalf("alias unsupported: %v", err)
		}
		return
	}
	if err != nil || !original.Equal(viaAlias) {
		t.Fatalf("alias differs: %v", err)
	}
	second := t.TempDir()
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, alias); err != nil {
		t.Fatal(err)
	}
	if err := Revalidate(context.Background(), viaAlias); err != nil {
		t.Fatalf("frozen authority followed retarget: %v", err)
	}
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"-old", root); err != nil {
		t.Fatal(err)
	}
	if err := Revalidate(context.Background(), original); err == nil {
		t.Fatal("recorded path now symlink accepted")
	}
}

// Regression: an anchored open follows a substituted symlink, or verifies only
// old held children. Test the real held-edge seam even on unsupported UUID filesystems.
func TestLinuxAnchoredDirectoryComponents(t *testing.T) {
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
	child, err := openDirectory(parent, "profile")
	if err != nil {
		t.Fatal(err)
	}
	before, err := child.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"-old", root); err != nil {
		t.Fatal(err)
	}
	followed, err := openDirectory(parent, "profile")
	if err == nil {
		_ = followed.Close()
		t.Fatal("anchored child symlink followed")
	}
	after, err := child.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("held child unexpectedly changed")
	}
	if err := child.Close(); err != nil {
		t.Fatal(err)
	}
}

// Regression: failure exits leak the O_PATH or readable-upgrade fd, exhausting
// a later capture. The closed-pin test checks use-after-close, not repeated cleanup.
func TestLinuxCaptureClosesHandles(t *testing.T) {
	if root := os.Getenv("P1_TEST_FD_LIMIT_ROOT"); root != "" {
		if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 64, Max: 64}); err != nil {
			t.Fatal(err)
		}
		for range 160 {
			a, err := Capture(context.Background(), root)
			if err != nil && !errors.Is(err, ErrUnsupported) {
				t.Fatalf("capture exhausted descriptors or failed: %v", err)
			}
			if err == nil && a.IsZero() {
				t.Fatal("successful capture zero")
			}
		}
		t.Log("BOUNDED_FD_REPEAT_COMPLETE")
		return
	}
	root := fixtureDirectory(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestLinuxCaptureClosesHandles$", "-test.v")
	cmd.Env = []string{"P1_TEST_FD_LIMIT_ROOT=" + root, "HOME=" + filepath.Dir(root), "TMPDIR=" + filepath.Dir(root)}
	out, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(out, []byte("BOUNDED_FD_REPEAT_COMPLETE")) {
		t.Fatalf("limited fresh process: %v\n%s", err, out)
	}
	t.Logf("child-only RLIMIT_NOFILE=64; 160 capture attempts\n%s", out)
}

// Regression: extraction changes historical decimal dev:inode into hex or adds
// UUID requirements. Publication fixtures compare identities internally, not a golden encoding.
func TestLegacyUnixDecimalIdentity(t *testing.T) {
	root := t.TempDir()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	var stat unix.Stat_t
	if err := unix.Stat(root, &stat); err != nil {
		t.Fatal(err)
	}
	actual, err := LegacyIdentity(root, info)
	expected := strconv.FormatUint(stat.Dev, 10) + ":" + strconv.FormatUint(stat.Ino, 10)
	if err != nil || actual != expected {
		t.Fatalf("legacy bytes=%q want=%q err=%v", actual, expected, err)
	}
	t.Logf("LEGACY_DECIMAL=%s", actual)
}

// Regression: treating EINVAL/permission/closed-fd failures as unsupported masks
// malformed ABI or acquisition defects. Historical dev:inode never queried UUIDs.
func TestLinuxUUIDErrorClassification(t *testing.T) {
	for _, cause := range []error{syscall.ENOTTY, syscall.EINVAL, syscall.EACCES, syscall.EPERM, syscall.EIO, syscall.EBADF, syscall.ENOSYS, syscall.EINTR} {
		err := linuxUUIDError(cause)
		if errors.Is(err, ErrUnsupported) != errors.Is(cause, syscall.ENOTTY) || !errors.Is(err, cause) {
			t.Fatalf("classification of %v: %v", cause, err)
		}
	}
	if linuxUUIDError(nil) != nil {
		t.Fatal("nil classified as failure")
	}
}

// Regression: an incorrect request size/type silently looks unsupported on old
// kernels. Actual ENOTTY alone cannot distinguish a wrong ABI request there.
func TestLinuxRequestABI(t *testing.T) {
	const independentReadDirection = 2
	const independentPayloadBytes = 17
	const independentIoctlType = 0x15
	expected := uint(independentReadDirection<<30 | independentPayloadBytes<<16 | independentIoctlType<<8)
	if uint(linuxGetFSUUID) != expected || unsafe.Sizeof([17]byte{}) != independentPayloadBytes {
		t.Fatalf("request ABI: %#x want %#x, payload size %d", linuxGetFSUUID, expected, unsafe.Sizeof([17]byte{}))
	}
}

// Regression: case-folding Linux directory aliases produce distinct canonical
// tokens for one physical profile. Symlink-only tests miss this inode flag.
func TestLinuxDirectoryCasefoldQualification(t *testing.T) {
	for _, flags := range []int{0, 0x10, 0x80000} {
		if err := checkLinuxDirectoryFlags(flags); err != nil {
			t.Fatalf("ordinary flags %#x: %v", flags, err)
		}
	}
	for _, flags := range []int{0x40000000, 0x40000010} {
		if err := checkLinuxDirectoryFlags(flags); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("casefold flags %#x: %v", flags, err)
		}
	}
	f, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Genuine fresh TEST descriptor, not a fabricated filesystem success.
	err = linuxCaseSensitive(int(f.Fd()))
	if err != nil {
		t.Fatalf("actual TEST flags query: %v", err)
	}
	t.Log("ACTUAL_DIRECTORY_FLAGS=CASE_SENSITIVE; UUID qualification remains separate")
}
