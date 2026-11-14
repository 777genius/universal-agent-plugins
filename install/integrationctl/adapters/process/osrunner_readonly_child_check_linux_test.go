//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReadonlyChildNamespacesRequireSixFreshAndOriginalTime(t *testing.T) {
	original := [7]uint64{100, 101, 102, 103, 104, 105, 106}
	current := [7]uint64{200, 201, 202, 203, 204, 205, 106}
	if err := readonlyChildNamespaces(original, current); err != nil {
		t.Fatal(err)
	}
	for i, name := range readonlyNamespaceNames() {
		t.Run(name, func(t *testing.T) {
			changed := current
			changed[i] = original[i]
			if i == 6 {
				changed[i]++
			}
			if err := readonlyChildNamespaces(original, changed); err == nil {
				t.Fatal("accepted the wrong namespace relation")
			}
			changed[i] = 0
			if err := readonlyChildNamespaces(original, changed); err == nil {
				t.Fatal("accepted an absent namespace identity")
			}
		})
	}
}

func TestReadonlyChildIDMapsBindSelectedIDsToParentZero(t *testing.T) {
	for _, body := range []string{"      1000          0          1\n", "1000\t0\t1\n"} {
		if err := readonlyChildIDMap([]byte(body), 1000); err != nil {
			t.Fatalf("rejected the selected nested mapping: %v", err)
		}
	}
	for _, body := range []string{"0 1000 1\n", "1000 1000 1\n", "1000 0 2\n", "01000 0 1\n",
		"1000 0 1", "1000 0 1\n1001 1 1\n", "1000 0 1\r\n", "1000\v0 1\n", "1000\u00a00 1\n", "1000 0 true\n"} {
		if err := readonlyChildIDMap([]byte(body), 1000); err == nil {
			t.Fatalf("accepted malformed or broader nested mapping %q", body)
		}
	}
	if err := readonlyChildIDMap([]byte("0 0 1\n"), 0); err == nil {
		t.Fatal("accepted root as the selected identity")
	}
}

func readonlyChildStatusFixture() string {
	return "Name:\treadonly-child\nUmask:\t0022\nUid:\t1000\t1000\t1000\t1000\nGid:\t1000\t1000\t1000\t1000\n" +
		"Groups:\t65534 1000 65534 \nCapInh:\t0000000000000000\nCapPrm:\t0000000000000000\n" +
		"CapEff:\t0000000000000000\nCapBnd:\t0000000000000000\nCapAmb:\t0000000000000000\nNoNewPrivs:\t1\n"
}

func TestReadonlyChildStatusRequiresTranslatedGroupsAndNoPrivileges(t *testing.T) {
	policy := readonlyPolicyFixture()
	policy.Groups = []uint32{0, 1000, 44}
	body := readonlyChildStatusFixture()
	if err := readonlyChildStatus([]byte(body), policy); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]string{
		"real uid":                strings.Replace(body, "Uid:\t1000", "Uid:\t0", 1),
		"saved gid":               strings.Replace(body, "Gid:\t1000\t1000\t1000", "Gid:\t1000\t1000\t0", 1),
		"empty groups":            strings.Replace(body, "65534 1000 65534 ", "", 1),
		"unmapped groups":         strings.Replace(body, "65534 1000 65534", "0 1000 44", 1),
		"reordered groups":        strings.Replace(body, "65534 1000 65534", "1000 65534 65534", 1),
		"extra group":             strings.Replace(body, "65534 1000 65534", "65534 1000 65534 65534", 1),
		"new privileges":          strings.Replace(body, "NoNewPrivs:\t1", "NoNewPrivs:\t0", 1),
		"missing privilege field": strings.Replace(body, "NoNewPrivs:\t1\n", "", 1),
		"duplicate uid":           body + "Uid:\t1000\t1000\t1000\t1000\n",
		"duplicate empty groups":  strings.Replace(body, "Groups:\t65534 1000 65534 \n", "Groups:\t\nGroups:\t\n", 1),
		"extra uid":               strings.Replace(body, "Uid:\t1000", "Uid:\t1000\t1000", 1),
	}
	for _, field := range []string{"CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"} {
		mutations[field] = strings.Replace(body, field+":\t0000000000000000", field+":\t0000000000000001", 1)
	}
	for name, changed := range mutations {
		t.Run(name, func(t *testing.T) {
			if err := readonlyChildStatus([]byte(changed), policy); err == nil {
				t.Fatal("accepted changed credentials or privileges")
			}
		})
	}
	policy.Groups = nil
	if err := readonlyChildStatus([]byte(strings.Replace(body, "65534 1000 65534 ", "", 1)), policy); err != nil {
		t.Fatalf("refused genuinely empty original supplementary groups: %v", err)
	}
}

func readonlyChildMountPolicyFixture() readonlyPolicy {
	policy := readonlyPolicyFixture()
	policy.Home.Path, policy.Worktree.Path, policy.Project.Path = "/home/native", "/work/tree", "/work/tree/project"
	policy.Executable.Path, policy.Dir = "/opt/native/grok", policy.Project.Path
	policy.Home.Facts.Device, policy.Worktree.Facts.Device = unix.Mkdev(8, 2), unix.Mkdev(8, 2)
	policy.Project.Facts.Device, policy.Executable.Facts.Device, policy.Self.Device = unix.Mkdev(8, 2), unix.Mkdev(8, 2), unix.Mkdev(8, 2)
	for i := range policy.Runtime {
		policy.Runtime[i].Facts.Device = unix.Mkdev(8, 1)
	}
	for i := range policy.Devices {
		policy.Devices[i].Facts.Device = unix.Mkdev(0, 5)
	}
	return policy
}

// This is a synthetic kernel table, not evidence of a hosted native launch.
// Bind mounts deliberately retain an rw superblock behind their ro mount.
func readonlyChildMountFixture() string {
	return "1 999 0:100 / / ro,nosuid,nodev - tmpfs tmpfs rw\n" +
		"2 1 8:1 /usr /usr ro,nosuid,nodev,relatime - ext4 /dev/root rw\n" +
		"3 1 8:1 /usr/bin /bin ro,nosuid,nodev,relatime - ext4 /dev/root rw\n" +
		"4 1 8:1 /usr/lib /lib ro,nosuid,nodev,relatime - ext4 /dev/root rw\n" +
		"5 1 8:1 /usr/lib64 /lib64 ro,nosuid,nodev,relatime - ext4 /dev/root rw\n" +
		"6 1 8:2 /tmp/self /.agentplugins-readonly-bootstrap ro,nosuid,nodev - ext4 /dev/other rw\n" +
		"7 1 0:101 / /proc ro,nosuid,nodev,noexec - proc proc rw\n" +
		"8 1 0:102 / /dev ro,nosuid,nodev - tmpfs tmpfs rw\n" +
		"9 8 0:103 / /dev/pts ro,nosuid,noexec - devpts devpts rw\n" +
		"10 8 0:5 /null /dev/null ro,nosuid,noexec - devtmpfs udev rw\n" +
		"11 8 0:5 /zero /dev/zero ro,nosuid,noexec - devtmpfs udev rw\n" +
		"12 8 0:5 /full /dev/full ro,nosuid,noexec - devtmpfs udev rw\n" +
		"13 8 0:5 /random /dev/random ro,nosuid,noexec - devtmpfs udev rw\n" +
		"14 8 0:5 /urandom /dev/urandom ro,nosuid,noexec - devtmpfs udev rw\n" +
		"15 8 0:5 /tty /dev/tty ro,nosuid,noexec - devtmpfs udev rw\n" +
		"16 1 8:2 /home/native /home/native ro,nosuid,nodev - ext4 /dev/other rw\n" +
		"17 1 8:2 /work/tree /work/tree ro,nosuid,nodev - ext4 /dev/other rw\n" +
		"18 1 8:2 /opt/native/grok /opt/native/grok ro,nosuid,nodev - ext4 /dev/other rw\n"
}

func readonlyChildWithoutMount(body, path string) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		if !strings.Contains(line, " "+path+" ro,") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n") + "\n"
}

func TestReadonlyChildMountsRequireExactClosedReadonlyView(t *testing.T) {
	policy, body := readonlyChildMountPolicyFixture(), readonlyChildMountFixture()
	if err := readonlyChildMounts([]byte(body), policy); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]string{
		"missing input":              readonlyChildWithoutMount(body, "/home/native"),
		"missing native file":        readonlyChildWithoutMount(body, "/opt/native/grok"),
		"unknown nested input mount": body + "23 16 8:3 / /home/native/foreign ro,nosuid,nodev - ext4 /dev/foreign rw\n",
		"host run mount":             body + "23 1 8:3 /run /run ro,nosuid,nodev - ext4 /dev/foreign rw\n",
		"console device":             body + "23 8 0:5 /console /dev/console ro,nosuid - devtmpfs udev rw\n",
		"writable runtime":           strings.Replace(body, "/usr ro,", "/usr rw,", 1),
		"runtime device effects":     strings.Replace(body, "/usr ro,nosuid,nodev,", "/usr ro,nosuid,", 1),
		"device suid":                strings.Replace(body, "/dev/null ro,nosuid,noexec", "/dev/null ro,noexec", 1),
		"devpts executable":          strings.Replace(body, "/dev/pts ro,nosuid,noexec", "/dev/pts ro,nosuid", 1),
		"foreign device filesystem":  strings.Replace(body, "10 8 0:5", "10 8 0:6", 1),
		"root not fresh tmpfs":       strings.Replace(body, "- tmpfs tmpfs rw", "- ext4 /dev/host rw", 1),
		"host proc subtree":          strings.Replace(body, "7 1 0:101 / /proc", "7 1 0:101 /host /proc", 1),
		"shared propagation":         strings.Replace(body, "relatime - ext4", "relatime shared:5 - ext4", 1),
		"slave propagation":          strings.Replace(body, "relatime - ext4", "relatime master:5 - ext4", 1),
		"duplicate identity":         strings.Replace(body, "18 1 8:2", "2 1 8:2", 1),
		"duplicate point":            strings.Replace(body, "/opt/native/grok /opt/native/grok", "/opt/native/grok /home/native", 1),
		"unknown parent":             strings.Replace(body, "16 1 8:2", "16 999 8:2", 1),
		"self parent":                strings.Replace(body, "16 1 8:2", "16 16 8:2", 1),
		"wrong parent path":          strings.Replace(body, "16 1 8:2", "16 17 8:2", 1),
		"duplicate flag":             strings.Replace(body, "/usr ro,", "/usr ro,ro,", 1),
		"missing final newline":      strings.TrimSuffix(body, "\n"),
	}
	for name, changed := range mutations {
		t.Run(name, func(t *testing.T) {
			if err := readonlyChildMounts([]byte(changed), policy); err == nil {
				t.Fatal("accepted changed view or permission")
			}
		})
	}
}

func TestReadonlyChildMountsKeepOnlyFiniteProcAndMinimalInputBinds(t *testing.T) {
	policy, body := readonlyChildMountPolicyFixture(), readonlyChildMountFixture()
	for i, name := range []string{"sys", "sysrq-trigger", "irq", "bus"} {
		body += fmt.Sprintf("%d 7 0:101 /%s /proc/%s ro,nosuid,nodev,noexec - proc proc rw\n", 19+i, name, name)
	}
	if err := readonlyChildMounts([]byte(body), policy); err != nil {
		t.Fatal(err)
	}
	if err := readonlyChildMounts([]byte(strings.Replace(body, "/proc/sys ro,nosuid,nodev,noexec", "/proc/sys ro,nosuid,noexec", 1)), policy); err == nil {
		t.Fatal("accepted a proc submount without NODEV")
	}
	policy.Worktree.Path, policy.Project.Path = policy.Home.Path+"/tree", policy.Home.Path+"/tree/project"
	coalesced := readonlyChildWithoutMount(readonlyChildMountFixture(), "/work/tree")
	if err := readonlyChildMounts([]byte(coalesced), policy); err != nil {
		t.Fatalf("rejected the single enclosing original HOME bind: %v", err)
	}
	policy.Executable.Path = policy.Home.Path + "/bin/grok"
	coalesced = readonlyChildWithoutMount(coalesced, "/opt/native/grok")
	if err := readonlyChildMounts([]byte(coalesced), policy); err != nil {
		t.Fatalf("required an unnecessary separate native mount: %v", err)
	}
	if err := readonlyChildMounts([]byte(readonlyChildMountFixture()), policy); err == nil {
		t.Fatal("accepted stale extra input and native mounts")
	}
}

func TestReadonlyChildMountRowsUseCanonicalNumbersAndSingleKernelUnescape(t *testing.T) {
	line := "20 1 8:2 /home/native\\040space /home/native\\040space ro,nosuid,nodev - ext4 /dev/other rw"
	row, err := readonlyChildMountLine(line)
	if err != nil || row.path != "/home/native space" {
		t.Fatalf("kernel escaped path mismatch: %v %+v", err, row)
	}
	path, err := readonlyChildMountPath(`/home/literal\134040`)
	if err != nil || path != `/home/literal\040` {
		t.Fatalf("kernel path was recursively unescaped: %q %v", path, err)
	}
	for _, changed := range []string{
		strings.Replace(line, "20 1", "020 1", 1), strings.Replace(line, "20 1", "0 1", 1),
		strings.Replace(line, "8:2", "8:4294967296", 1), strings.Replace(line, "8:2", "8:2:3", 1),
		strings.ReplaceAll(line, `\040`, `\041`), strings.ReplaceAll(line, `\040`, `\012`),
		strings.Replace(line, "/home/native", "/home/../native", 1), strings.Replace(line, "ro,nosuid,nodev", "ro,,nosuid,nodev", 1),
		strings.Replace(line, "20 1", "20  1", 1), strings.Replace(line, " - ext4", " propagate_from:9 - ext4", 1),
	} {
		if _, err := readonlyChildMountLine(changed); err == nil {
			t.Fatalf("accepted malformed kernel row %q", changed)
		}
	}
}

func TestReadonlyChildObjectRefusesReplacementAliasAndAnyOriginalMetadataDrift(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "native")
	body := []byte("original native bytes\n")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := readonlyFacts(info)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readonlyChildObject(context.Background(), path, original); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*readonlyObjectFacts){
		"device": func(f *readonlyObjectFacts) { f.Device++ }, "inode": func(f *readonlyObjectFacts) { f.Inode++ },
		"owner": func(f *readonlyObjectFacts) { f.UID++ }, "group": func(f *readonlyObjectFacts) { f.GID++ },
		"mode": func(f *readonlyObjectFacts) { f.Mode ^= 0001 }, "links": func(f *readonlyObjectFacts) { f.Links++ },
		"size": func(f *readonlyObjectFacts) { f.Size++ }, "mtime": func(f *readonlyObjectFacts) { f.Mtime.Sec++ },
		"ctime": func(f *readonlyObjectFacts) { f.Ctime.Sec++ },
	} {
		t.Run(name, func(t *testing.T) {
			changed := original
			change(&changed)
			if _, err := readonlyChildObject(context.Background(), path, changed); err == nil {
				t.Fatal("adopted changed original metadata")
			}
		})
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := readonlyChildObject(context.Background(), alias, original); err == nil {
		t.Fatal("followed an alias instead of the original leaf")
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacement, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if _, err := readonlyChildObject(context.Background(), path, original); err == nil {
		t.Fatal("accepted a same-byte replacement")
	}
	if actual, err := os.ReadFile(path); err != nil || string(actual) != string(body) {
		t.Fatalf("validation changed bytes: %q %v", actual, err)
	}
}

func TestReadonlyChildCancellationAndOriginalCutoffPrecedeViewReads(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateReadonlyChild(ctx, readonlyPolicy{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost before policy or namespace read: %v", err)
	}
	if _, err := readonlyChildObject(ctx, "/does-not-exist/readonly-original", readonlyObjectFacts{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("object access preceded cancellation: %v", err)
	}
	if err := validateReadonlyChild(context.Background(), readonlyPolicy{Deadline: 1}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("original monotonic cutoff was reset: %v", err)
	}
}
