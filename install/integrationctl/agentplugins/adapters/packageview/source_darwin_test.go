//go:build darwin && arm64

package packageview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Every volume is newly created inside t.TempDir, populated while writable,
// then detached and mounted read-only. No environment-provided mount/project is
// accepted. hdiutil is fixture tooling, never production reader behavior.
func readOnlyFixture(t *testing.T, build func(string)) string {
	t.Helper()
	dmg, mount, directory := newMountedFixture(t)
	root := filepath.Join(mount, "source")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	build(root)
	if e := detachFixture(dmg, mount, directory); e != nil {
		t.Fatalf("UNPROVEN APFS fixture prerequisite: %v", e)
	}
	fixtureCommand(t, "/usr/bin/hdiutil", "attach", "-readonly", "-nobrowse", "-noautoopen", "-mountpoint", mount, dmg)
	var fs unix.Statfs_t
	if e := unix.Statfs(mount, &fs); e != nil {
		t.Fatal(e)
	}
	if unix.ByteSliceToString(fs.Mntonname[:]) != mount || unix.ByteSliceToString(fs.Fstypename[:]) != "apfs" || fs.Flags&unix.MNT_RDONLY == 0 {
		t.Fatal("UNPROVEN read-only APFS fixture mount")
	}
	return root
}

func fixtureCommand(t *testing.T, tool string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	b, e := exec.CommandContext(ctx, tool, args...).CombinedOutput()
	if e != nil {
		t.Fatalf("UNPROVEN APFS fixture prerequisite: %s %v: %v\n%s", tool, args, errors.Join(e, ctx.Err()), b)
	}
}

func newMountedFixture(t *testing.T) (image, mount string, directory *fixtureDirectory) {
	t.Helper()
	tmp, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	mount = filepath.Join(tmp, "mount")
	dmg := filepath.Join(tmp, "fixture.sparseimage")
	if e := os.Mkdir(mount, 0700); e != nil {
		t.Fatal(e)
	}
	directory = pinFixtureDirectory(t, mount)
	fixtureCommand(t, "/usr/bin/hdiutil", "create", "-size", "512m", "-fs", "APFS", "-volname", "packageview-disposable", "-type", "SPARSE", dmg)
	t.Cleanup(func() {
		if e := detachFixture(dmg, mount, directory); e != nil {
			t.Errorf("UNPROVEN APFS fixture cleanup: %v", e)
		}
	})
	fixtureCommand(t, "/usr/bin/hdiutil", "attach", "-nobrowse", "-noautoopen", "-mountpoint", mount, dmg)
	return dmg, mount, directory
}

// Pin the real directory and every ancestor before attach. These descriptors
// refer to the underlying parent filesystem, never the subsequently mounted
// image. Keeping them open also prevents inode reuse from masquerading as the
// original directory. Cleanup closes them only after image cleanup.
type fixtureDirectory struct {
	path     string
	files    []*os.File
	parentFS unix.Statfs_t
}

func openFixtureDirectories(path string) ([]*os.File, error) {
	root, e := os.Open("/")
	if e != nil {
		return nil, e
	}
	files := []*os.File{root}
	for _, component := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		fd, e := unix.Openat(int(files[len(files)-1].Fd()), component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			for _, file := range files {
				e = errors.Join(e, file.Close())
			}
			return nil, e
		}
		files = append(files, os.NewFile(uintptr(fd), component))
	}
	return files, nil
}

func pinFixtureDirectory(t *testing.T, mount string) *fixtureDirectory {
	t.Helper()
	files, e := openFixtureDirectories(mount)
	if e != nil {
		t.Fatal(e)
	}
	directory := &fixtureDirectory{path: mount, files: files}
	t.Cleanup(func() {
		for _, file := range files {
			if e := file.Close(); e != nil {
				t.Error(e)
			}
		}
	})
	if e := unix.Fstatfs(int(files[len(files)-2].Fd()), &directory.parentFS); e != nil {
		t.Fatal(e)
	}
	if _, e := directory.filesystem(mount); e != nil {
		t.Fatal("UNPROVEN initial fixture directory:", e)
	}
	return directory
}

func sameFixtureFilesystem(a, b unix.Statfs_t) bool {
	return a.Fsid == b.Fsid && a.Type == b.Type && a.Mntfromname == b.Mntfromname && a.Mntonname == b.Mntonname && a.Fstypename == b.Fstypename
}

func (directory *fixtureDirectory) filesystem(mount string) (fs unix.Statfs_t, err error) {
	if directory == nil || directory.path != mount {
		return fs, fmt.Errorf("expected mount does not match pinned directory")
	}
	// Traverse component by component without following aliases. Stat and
	// Statfs refer to these same opened objects, not separate path lookups.
	files, e := openFixtureDirectories(mount)
	if e != nil {
		return fs, e
	}
	defer func() {
		for _, file := range files {
			err = errors.Join(err, file.Close())
		}
	}()
	last := len(files) - 1
	for i := range last {
		actual, e := files[i].Stat()
		if e != nil {
			return fs, e
		}
		expected, e := directory.files[i].Stat()
		if e != nil {
			return fs, e
		}
		if !os.SameFile(actual, expected) {
			return fs, fmt.Errorf("ancestor identity changed at component %d", i)
		}
	}
	var parent unix.Statfs_t
	if e := unix.Fstatfs(int(files[last-1].Fd()), &parent); e != nil {
		return fs, e
	}
	if !sameFixtureFilesystem(parent, directory.parentFS) {
		return fs, fmt.Errorf("enclosing parent filesystem changed")
	}
	if e := unix.Fstatfs(int(files[last].Fd()), &fs); e != nil {
		return fs, e
	}
	if unix.ByteSliceToString(fs.Mntonname[:]) != mount {
		actual, e := files[last].Stat()
		if e != nil {
			return fs, e
		}
		expected, e := directory.files[last].Stat()
		if e != nil {
			return fs, e
		}
		parentInfo, e := files[last-1].Stat()
		if e != nil {
			return fs, e
		}
		if !os.SameFile(actual, expected) || actual.Sys().(*syscall.Stat_t).Dev != parentInfo.Sys().(*syscall.Stat_t).Dev || !sameFixtureFilesystem(fs, parent) {
			return fs, fmt.Errorf("unmounted directory or parent device/filesystem identity changed")
		}
	}
	return fs, nil
}

type fixtureEntity struct {
	Mount  string `json:"mount-point"`
	Device string `json:"dev-entry"`
	Hint   string `json:"content-hint"`
}

type fixtureImage struct {
	Path     string          `json:"image-path"`
	Entities []fixtureEntity `json:"system-entities"`
}

var fixtureWholeDevice = regexp.MustCompile(`^/dev/disk\d+$`)

// Return no target when fully detached, the exact mount when mounted, or the
// image's inventoried GUID whole device when attached but already unmounted.
func fixtureDetachTarget(ctx context.Context, image, mount string, directory *fixtureDirectory) (string, error) {
	plist, e := exec.CommandContext(ctx, "/usr/bin/hdiutil", "info", "-plist").CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("hdiutil info: %w\n%s", errors.Join(e, ctx.Err()), plist)
	}
	convert := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", "-")
	convert.Stdin = bytes.NewReader(plist)
	data, e := convert.CombinedOutput()
	if e != nil {
		return "", fmt.Errorf("plutil mount state: %w\n%s\n%s", errors.Join(e, ctx.Err()), plist, data)
	}
	var info struct {
		Images []fixtureImage `json:"images"`
	}
	if e := json.Unmarshal(data, &info); e != nil {
		return "", fmt.Errorf("decode mount state: %w\n%s\n%s", e, plist, data)
	}
	target, e := ownedFixtureTarget(info.Images, image, mount, directory)
	if e != nil {
		return "", fmt.Errorf("%w\nhdiutil inventory:\n%s\n%s", e, plist, data)
	}
	return target, nil
}

func ownedFixtureTarget(images []fixtureImage, image, mount string, directory *fixtureDirectory) (string, error) {
	if images == nil || !filepath.IsAbs(image) || !filepath.IsAbs(mount) || filepath.Clean(image) != image || filepath.Clean(mount) != mount {
		return "", fmt.Errorf("unproven fixture inventory or expected paths")
	}
	var own *fixtureImage
	for i := range images {
		attached := &images[i]
		if filepath.Clean(attached.Path) == image {
			if own != nil {
				return "", fmt.Errorf("refusing detach: multiple attachments for %s", image)
			}
			own = attached
		}
		for _, entity := range attached.Entities {
			if entity.Mount != "" && filepath.Clean(entity.Mount) == mount && filepath.Clean(attached.Path) != image {
				return "", fmt.Errorf("refusing detach: %s belongs to another image", mount)
			}
		}
	}
	fs, e := directory.filesystem(mount)
	if e != nil {
		return "", fmt.Errorf("refusing detach: unproven fixture directory: %w", e)
	}
	isMount := unix.ByteSliceToString(fs.Mntonname[:]) == mount
	if own == nil {
		if isMount {
			return "", fmt.Errorf("refusing detach: foreign filesystem at %s", mount)
		}
		return "", nil
	}
	device, mountedDevice := "", ""
	for _, entity := range own.Entities {
		if entity.Hint == "GUID_partition_scheme" {
			if device != "" || !fixtureWholeDevice.MatchString(entity.Device) {
				return "", fmt.Errorf("refusing detach: ambiguous whole device for %s", image)
			}
			device = entity.Device
		}
		if entity.Mount != "" {
			if entity.Mount != mount || mountedDevice != "" || entity.Device == "" {
				return "", fmt.Errorf("refusing detach: unexpected or multiple mounts for %s", image)
			}
			mountedDevice = entity.Device
		}
	}
	if device == "" || (mountedDevice != "") != isMount {
		return "", fmt.Errorf("unproven fixture ownership for %s: device=%q mount=%q filesystem=%t", image, device, mountedDevice, isMount)
	}
	// Neither the whole device nor the mounted volume may be attributed to a
	// second image, even if it is listed at a different mount.
	for i := range images {
		if &images[i] == own {
			continue
		}
		for _, entity := range images[i].Entities {
			if entity.Device == device || (mountedDevice != "" && entity.Device == mountedDevice) {
				return "", fmt.Errorf("refusing detach: device belongs to multiple images")
			}
		}
	}
	if isMount {
		if unix.ByteSliceToString(fs.Mntfromname[:]) != mountedDevice || unix.ByteSliceToString(fs.Fstypename[:]) != "apfs" {
			return "", fmt.Errorf("refusing detach: inconsistent fixture filesystem at %s", mount)
		}
		return mount, nil
	}
	return device, nil
}

// A failed detach may unmount without ejecting. Reconcile inventory before
// every command and after every outcome; never infer ownership from the error.
func detachFixture(image, mount string, directory *fixtureDirectory) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var diagnostics strings.Builder
	for attempt := 1; attempt <= 4; attempt++ {
		target, e := fixtureDetachTarget(ctx, image, mount, directory)
		if e != nil {
			return fmt.Errorf("%sverify fixture before detach: %w", diagnostics.String(), e)
		}
		if target == "" {
			return nil
		}
		out, detachErr := exec.CommandContext(ctx, "/usr/bin/hdiutil", "detach", target).CombinedOutput()
		fmt.Fprintf(&diagnostics, "detach attempt %d at %s: %v\n%s\n", attempt, target, errors.Join(detachErr, ctx.Err()), out)
		target, e = fixtureDetachTarget(ctx, image, mount, directory)
		if e != nil {
			return fmt.Errorf("%sverify fixture after detach: %w", diagnostics.String(), e)
		}
		if target == "" {
			return nil
		}
		if detachErr == nil || !strings.Contains(strings.ToLower(string(out)), "resource busy") || attempt == 4 {
			return fmt.Errorf("%sfixture still attached at %s", diagnostics.String(), mount)
		}
		timer := time.NewTimer(time.Duration(attempt) * 250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%sdetach retry: %w", diagnostics.String(), ctx.Err())
		case <-timer.C:
		}
	}
	return fmt.Errorf("%sdetach attempts exhausted", diagnostics.String())
}

func unmountFixture(t *testing.T, mount string) {
	t.Helper()
	// diskutil's on-host usage documents unmount by MountPoint. Verify that
	// primary help before using the non-forced, filesystem-only operation.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	help, e := exec.CommandContext(ctx, "/usr/sbin/diskutil", "unmount").CombinedOutput()
	if e == nil || ctx.Err() != nil || !strings.Contains(string(help), "MountPoint") || !strings.Contains(string(help), "u[n]mount") {
		t.Fatalf("UNPROVEN diskutil unmount usage: %v\n%s", errors.Join(e, ctx.Err()), help)
	}
	t.Logf("primary diskutil unmount usage:\n%s", help)
	fixtureCommand(t, "/usr/sbin/diskutil", "unmount", mount)
}

func TestDarwinFixtureCleanupAttachedUnmounted(t *testing.T) {
	image, mount, directory := newMountedFixture(t)
	unmountFixture(t, mount)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	target, e := fixtureDetachTarget(ctx, image, mount, directory)
	if e != nil || !fixtureWholeDevice.MatchString(target) {
		t.Fatalf("UNPROVEN attached-unmounted prerequisite: target=%q error=%v", target, e)
	}
	t.Logf("attached-unmounted prerequisite: image=%s mount=%s whole-device=%s", image, mount, target)
	if e := detachFixture(image, mount, directory); e != nil {
		t.Fatal("owned attached-unmounted image cleanup failed:", e)
	}
	if target, e := fixtureDetachTarget(ctx, image, mount, directory); e != nil || target != "" {
		t.Fatalf("fixture image remains after cleanup: target=%q error=%v", target, e)
	}
	if e := detachFixture(image, mount, directory); e != nil {
		t.Fatal("fully detached cleanup is not idempotent:", e)
	}
}

func TestDarwinFixtureCleanupRefusesForeignOwnership(t *testing.T) {
	image, mount, directory := newMountedFixture(t)
	otherImage, otherMount, otherDirectory := newMountedFixture(t)
	for _, m := range []string{mount, otherMount} {
		nativeWrite(t, m, "sentinel", "keep")
	}
	for i, pair := range [][2]string{{image, otherMount}, {otherImage, mount}} {
		if e := detachFixture(pair[0], pair[1], []*fixtureDirectory{directory, otherDirectory}[i]); e == nil || !strings.Contains(e.Error(), "refusing detach") {
			t.Fatalf("foreign expected image/mount was not refused: %v", e)
		}
		// Both genuinely mounted sibling fixtures and their files must survive
		// each refusal. An error alone would not prove absence of effects.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		for j, sibling := range [][2]string{{image, mount}, {otherImage, otherMount}} {
			target, e := fixtureDetachTarget(ctx, sibling[0], sibling[1], []*fixtureDirectory{directory, otherDirectory}[j])
			if e != nil || target != sibling[1] {
				cancel()
				t.Fatalf("refusal touched sibling mount: target=%q error=%v", target, e)
			}
			b, e := os.ReadFile(filepath.Join(sibling[1], "sentinel"))
			if e != nil || string(b) != "keep" {
				cancel()
				t.Fatalf("refusal touched sibling file: data=%q error=%v", b, e)
			}
		}
		cancel()
	}
	// The new whole-device path must also refuse a foreign expected mount,
	// leaving both the unmounted image and the mounted sibling untouched.
	unmountFixture(t, mount)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, e := fixtureDetachTarget(ctx, image, mount, directory)
	if e != nil || !fixtureWholeDevice.MatchString(device) {
		t.Fatalf("UNPROVEN attached-unmounted prerequisite: target=%q error=%v", device, e)
	}
	if e := detachFixture(image, otherMount, directory); e == nil || !strings.Contains(e.Error(), "refusing detach") {
		t.Fatalf("unmounted image with foreign expected mount was not refused: %v", e)
	}
	if target, e := fixtureDetachTarget(ctx, image, mount, directory); e != nil || target != device {
		t.Fatalf("refusal touched attached-unmounted image: target=%q error=%v", target, e)
	}
	if target, e := fixtureDetachTarget(ctx, otherImage, otherMount, otherDirectory); e != nil || target != otherMount {
		t.Fatalf("refusal touched mounted sibling: target=%q error=%v", target, e)
	}
	if b, e := os.ReadFile(filepath.Join(otherMount, "sentinel")); e != nil || string(b) != "keep" {
		t.Fatalf("refusal touched mounted sibling file: data=%q error=%v", b, e)
	}
}

// The path is still clean and absolute and inventory still gives B its real
// mount name. The old mount-name-inequality guard admitted A's detach through
// this alias. Check native inventory and files after refusal, not just an error.
func TestDarwinFixtureCleanupRefusesAliasedForeignMount(t *testing.T) {
	image, mount, directory := newMountedFixture(t)
	otherImage, otherMount, otherDirectory := newMountedFixture(t)
	nativeWrite(t, otherMount, "sentinel", "keep")
	unmountFixture(t, mount)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, e := fixtureDetachTarget(ctx, image, mount, directory)
	if e != nil || !fixtureWholeDevice.MatchString(device) {
		t.Fatalf("UNPROVEN attached-unmounted prerequisite: target=%q error=%v", device, e)
	}
	saved := mount + ".owned"
	if e := os.Rename(mount, saved); e != nil {
		t.Fatal(e)
	}
	// Restore the SAME directory, including its device/inode, before registered
	// image cleanup even if an assertion fails. Recreating it would lose proof.
	t.Cleanup(func() {
		if e := os.Remove(mount); e != nil && !errors.Is(e, os.ErrNotExist) {
			t.Error(e)
			return
		}
		if e := os.Rename(saved, mount); e != nil {
			t.Error(e)
		}
	})
	if e := os.Symlink(otherMount, mount); e != nil {
		t.Fatal(e)
	}
	refusal := detachFixture(image, mount, directory)
	plist, e := exec.CommandContext(ctx, "/usr/bin/hdiutil", "info", "-plist").CombinedOutput()
	if e != nil {
		t.Fatalf("UNPROVEN post-refusal native inventory: %v\n%s", e, plist)
	}
	convert := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", "-")
	convert.Stdin = bytes.NewReader(plist)
	data, e := convert.CombinedOutput()
	if e != nil {
		t.Fatalf("UNPROVEN post-refusal inventory conversion: %v\n%s", e, data)
	}
	var info struct {
		Images []fixtureImage `json:"images"`
	}
	if e := json.Unmarshal(data, &info); e != nil {
		t.Fatal(e)
	}
	t.Logf("post-refusal native inventory:\n%s", data)
	ownAttached, otherMounted := false, false
	for _, attached := range info.Images {
		for _, entity := range attached.Entities {
			if attached.Path == image && entity.Device == device && entity.Hint == "GUID_partition_scheme" {
				ownAttached = true
			}
			if attached.Path == otherImage && entity.Mount == otherMount {
				otherMounted = true
			}
		}
	}
	sentinel, readErr := os.ReadFile(filepath.Join(otherMount, "sentinel"))
	t.Logf("alias refusal: error=%v A-attached=%t B-mounted=%t B-sentinel=%q read-error=%v", refusal, ownAttached, otherMounted, sentinel, readErr)
	if refusal == nil || !strings.Contains(refusal.Error(), "refusing detach") || !ownAttached || !otherMounted || readErr != nil || string(sentinel) != "keep" {
		t.Fatalf("foreign mount alias not safely refused: error=%v A-attached=%t B-mounted=%t B-sentinel=%q read-error=%v", refusal, ownAttached, otherMounted, sentinel, readErr)
	}
	if target, e := fixtureDetachTarget(ctx, otherImage, otherMount, otherDirectory); e != nil || target != otherMount {
		t.Fatalf("refusal changed B's native filesystem: target=%q error=%v", target, e)
	}
}

// A real replacement parent on the SAME filesystem defeats a filesystem-only
// comparison. Fully detached cleanup must still refuse lost ancestor identity.
func TestDarwinFixtureCleanupRefusesReplacedParent(t *testing.T) {
	image, mount, directory := newMountedFixture(t)
	if e := detachFixture(image, mount, directory); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if target, e := fixtureDetachTarget(ctx, image, mount, directory); e != nil || target != "" {
		t.Fatalf("UNPROVEN detached prerequisite: target=%q error=%v", target, e)
	}
	parent := filepath.Dir(mount)
	saved := parent + ".owned"
	if e := os.Rename(parent, saved); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		for _, path := range []string{filepath.Join(mount, "sentinel"), mount, parent} {
			if e := os.Remove(path); e != nil && !errors.Is(e, os.ErrNotExist) {
				t.Error(e)
				return
			}
		}
		if e := os.Rename(saved, parent); e != nil {
			t.Error(e)
		}
	})
	if e := os.Mkdir(parent, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(mount, 0700); e != nil {
		t.Fatal(e)
	}
	var replacementFS, savedFS unix.Statfs_t
	if e := unix.Statfs(parent, &replacementFS); e != nil {
		t.Fatal(e)
	}
	if e := unix.Statfs(saved, &savedFS); e != nil {
		t.Fatal(e)
	}
	if replacementFS.Fsid != savedFS.Fsid || replacementFS.Mntfromname != savedFS.Mntfromname {
		t.Fatal("UNPROVEN same-filesystem parent replacement")
	}
	nativeWrite(t, mount, "sentinel", "keep")
	refusal := detachFixture(image, mount, directory)
	b, readErr := os.ReadFile(filepath.Join(mount, "sentinel"))
	t.Logf("replaced-parent refusal: error=%v sentinel=%q read-error=%v", refusal, b, readErr)
	if refusal == nil || !strings.Contains(refusal.Error(), "refusing detach") || readErr != nil || string(b) != "keep" {
		t.Fatalf("replaced parent not safely refused: error=%v sentinel=%q read-error=%v", refusal, b, readErr)
	}
}

// Ordinary sources are new writable local APFS directories, never user projects.
func nativeFixture(t *testing.T, build func(string)) string {
	t.Helper()
	root := t.TempDir()
	build(root)
	return root
}
func TestDarwinWritableProfileAccepted(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	l, e := (Reader{TempDir: t.TempDir()}).Open(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if string(l.Data().Plugin.Bytes) != "core" {
		t.Fatal("writable source not captured")
	}
	if _, e := l.Capture(context.Background()); e != nil {
		t.Fatal(e)
	}
}
func TestDarwinSpecialMetadataNeverOpened(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", "core")
		if e := unix.Mkfifo(filepath.Join(root, "fifo"), 0600); e != nil {
			t.Fatal(e)
		}
	})
	s := nativeSource(t, root)
	p, e := s.pin("fifo", true)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	if p.info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatal("FIFO type lost")
	}
	if f, e := p.reopen(false); e == nil {
		f.Close()
		t.Fatal("FIFO data-opened")
	}
}
func TestDarwinDeviceMetadataNeverOpened(t *testing.T) {
	for _, kind := range []uint32{unix.S_IFCHR, unix.S_IFBLK} {
		t.Run(map[uint32]string{unix.S_IFCHR: "character", unix.S_IFBLK: "block"}[kind], func(t *testing.T) {
			root := nativeFixture(t, func(root string) {
				nativeWrite(t, root, "plugin.json", "core")
				// Deliberately unassigned device number in a NEW fixture, never /dev data.
				e := unix.Mknod(filepath.Join(root, "device"), kind|0600, int(unix.Mkdev(255, 255)))
				if errors.Is(e, syscall.EPERM) || errors.Is(e, syscall.EACCES) {
					t.Skipf("UNPROVEN mknod permission gate: %v", e)
				}
				if e != nil {
					t.Fatal(e)
				}
			})
			s := nativeSource(t, root)
			p, e := s.pin("device", true)
			if e != nil {
				t.Fatal(e)
			}
			defer p.file.Close()
			if p.info.Mode()&os.ModeDevice == 0 {
				t.Fatal("device type lost")
			}
			if f, e := p.reopen(false); e == nil {
				f.Close()
				t.Fatal("device data-opened")
			}
		})
	}
}
func TestDarwinReplacementDeniedByProfile(t *testing.T) {
	root := readOnlyFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	scratch := t.TempDir()
	attempted := false
	l, e := (Reader{TempDir: scratch}).open(context.Background(), root, &captureHooks{afterNameCheck: func(p string) {
		if p != "plugin.json" {
			return
		}
		attempted = true
		e := os.Rename(filepath.Join(root, p), filepath.Join(root, "old"))
		if !errors.Is(e, syscall.EROFS) {
			t.Fatalf("profile allowed replacement: %v", e)
		}
		e = unix.Mkfifo(filepath.Join(root, p), 0600)
		if !errors.Is(e, syscall.EROFS) && !errors.Is(e, syscall.EEXIST) {
			t.Fatalf("unexpected FIFO substitution result: %v", e)
		}
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if !attempted || l.Data().Plugin.State != Present {
		t.Fatal("replacement gate did not run")
	}
	if _, e = l.Capture(context.Background()); e != nil {
		t.Fatal(e)
	}
}

func TestDarwinHandleCleanupAndTypeChecks(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", "core")
		if e := unix.Mkfifo(filepath.Join(root, "fifo"), 0600); e != nil {
			t.Fatal(e)
		}
	})
	// Check saved descriptor numbers immediately after close, before reuse.
	assertClosed := func(fd uintptr) {
		t.Helper()
		if _, e := unix.FcntlInt(fd, unix.F_GETFD, 0); !errors.Is(e, unix.EBADF) {
			t.Fatalf("descriptor %d: expected EBADF after close, got %v", fd, e)
		}
	}
	for i := 0; i < 10; i++ {
		s, e := openSource(root, GeneratedStaging{})
		if e != nil {
			t.Fatal(e)
		}
		sourceFD := s.anchor.Fd()
		for _, name := range []string{".", "plugin.json", "fifo"} {
			p, e := s.pin(name, true)
			if e != nil {
				s.close()
				t.Fatal(e)
			}
			pinFD := p.file.Fd()
			// Every mismatch is rejected before opening any target data.
			if f, e := p.reopen(!p.info.IsDir()); e == nil {
				f.Close()
				t.Fatal("wrong-kind reopen succeeded")
			}
			if e := p.file.Close(); e != nil {
				t.Fatal(e)
			}
			assertClosed(pinFD)
		}
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
		assertClosed(sourceFD)
		if e := s.close(); e != nil {
			t.Fatal(e)
		}
		assertClosed(sourceFD)
	}
}

func TestDarwinReplacementAtBothReadBoundaries(t *testing.T) {
	root := readOnlyFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before", true: "after"}[after], func(t *testing.T) {
			attempted := false
			swap := func(name string) {
				if name != "plugin.json" || attempted {
					return
				}
				attempted = true
				if e := os.Rename(filepath.Join(root, name), filepath.Join(root, "old")); !errors.Is(e, syscall.EROFS) {
					t.Fatalf("read-only profile allowed replacement: %v", e)
				}
				if e := os.WriteFile(filepath.Join(root, name), []byte("mutation"), 0600); !errors.Is(e, syscall.EROFS) {
					t.Fatalf("read-only profile allowed mutation: %v", e)
				}
			}
			h := &captureHooks{beforeDataOpen: swap}
			if after {
				h = &captureHooks{afterNameCheck: swap}
			}
			l, e := (Reader{TempDir: t.TempDir()}).open(context.Background(), root, h)
			if e != nil {
				t.Fatal(e)
			}
			defer l.Close()
			if !attempted || string(l.Data().Plugin.Bytes) != "core" {
				t.Fatal("read boundary gate did not preserve original object")
			}
		})
	}
}

func TestDarwinRootSelectionTraversalOrder(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "dir/nested/keep", "data")
		nativeLink(t, root, "dir/nested", "a")
	})
	// Root selection follows ancestors in filesystem order. Cleaning a/.. would
	// choose root itself; kernel traversal must choose root/dir instead.
	s := nativeSource(t, root+"/a/..")
	got, e := s.anchor.Stat()
	if e != nil {
		t.Fatal(e)
	}
	want, e := os.Stat(filepath.Join(root, "dir"))
	if e != nil || !os.SameFile(got, want) {
		t.Fatal("root selection was lexically cleaned", e)
	}
}

// A matching proof remains accepted and an unmatched proof still fails closed.
func TestDarwinGeneratedStagingProofAcceptsOwnWritableRoot(t *testing.T) {
	root := t.TempDir()
	nativeWrite(t, root, "plugin.json", "core")
	dir, e := os.OpenRoot(root)
	if e != nil {
		t.Fatal(e)
	}
	defer dir.Close()
	proof, e := NewGeneratedStaging(dir)
	if e != nil {
		t.Fatal(e)
	}
	s, e := openSource(root, proof)
	if e != nil {
		t.Fatal("trusted generated-staging proof rejected on its own writable root:", e)
	}
	defer s.close()
	// The relaxation must still require local APFS and every other check:
	// this exercises the same darwinFS/pin/reopen path the untrusted case uses.
	p, e := s.pin("plugin.json", true)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	f, e := p.reopen(false)
	if e != nil {
		t.Fatal("trusted data reopen failed:", e)
	}
	defer f.Close()
}

// A GeneratedStaging proof built from one directory must never authorize a
// different one: the caller could otherwise mint a proof against a trivially
// creatable writable directory and pass an unrelated path to openSource.
func TestDarwinGeneratedStagingProofMismatchFailsClosed(t *testing.T) {
	a := filepath.Join(t.TempDir(), "a")
	b := filepath.Join(t.TempDir(), "b")
	for _, d := range []string{a, b} {
		if e := os.Mkdir(d, 0700); e != nil {
			t.Fatal(e)
		}
	}
	dirA, e := os.OpenRoot(a)
	if e != nil {
		t.Fatal(e)
	}
	defer dirA.Close()
	proof, e := NewGeneratedStaging(dirA)
	if e != nil {
		t.Fatal(e)
	}
	s, e := openSource(b, proof)
	if e == nil {
		s.close()
		t.Fatal("mismatched generated-staging proof accepted a different directory")
	}
	var safe *Error
	if !errors.As(e, &safe) || safe.Code != "generated_staging_mismatch" {
		t.Fatal(e)
	}
}
func nativeLinkPrivilegeError(e error) bool { return os.IsPermission(e) }
