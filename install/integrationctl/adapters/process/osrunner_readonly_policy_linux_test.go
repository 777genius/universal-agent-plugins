//go:build linux

package process

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func readonlyPolicyFixture() readonlyPolicy {
	stamp := syscall.Timespec{Sec: 1, Nsec: 2}
	directory := readonlyObjectFacts{Device: unix.Mkdev(8, 2), Inode: 101, Links: 2,
		Mode: unix.S_IFDIR | 0700, UID: 1000, GID: 1000, Size: 4096, Mtime: stamp, Ctime: stamp}
	native := directory
	native.Inode, native.Links, native.Mode, native.Size = 102, 1, unix.S_IFREG|0700, 4096
	system := native
	system.UID, system.GID, system.Mode, system.Inode = 0, 0, unix.S_IFREG|0755, 103
	now, _ := readonlyMonotonic()
	policy := readonlyPolicy{Version: 1, Nonce: strings.Repeat("a", 32), Deadline: now + int64(time.Second),
		UID: 1000, GID: 1000, OverflowUID: 65534, OverflowGID: 65534, Groups: []uint32{1000},
		Namespaces: [7]uint64{100, 101, 102, 103, 104, 105, 106},
		Home:       readonlyPolicyObject{Path: "/home/native", Facts: directory},
		Worktree:   readonlyPolicyObject{Path: "/work/tree", Facts: directory},
		Project:    readonlyPolicyObject{Path: "/work/tree/project", Facts: directory},
		Executable: readonlyPolicyObject{Path: "/opt/native/grok", Facts: native},
		Self:       native, Primitive: system, SelfSHA: strings.Repeat("b", 64),
		PrimitiveSHA: strings.Repeat("c", 64), NativeSHA: strings.Repeat("d", 64),
		Input:        readonlyCensusProof{Count: 3, SHA256: strings.Repeat("e", 64)},
		RuntimeProof: readonlyCensusProof{Count: 4, SHA256: strings.Repeat("f", 64)},
		Argv:         []string{"/opt/native/grok", "--agent", "acp"}, Dir: "/work/tree/project",
		Env: []string{"HOME=/home/native", "XDG_CONFIG_HOME=/home/native/.config", "XDG_DATA_HOME=/home/native/.local/share",
			"XDG_CACHE_HOME=/home/native/.cache", "XDG_STATE_HOME=/home/native/.local/state"}}
	for i, path := range []string{"/usr", "/usr/bin", "/usr/lib", "/usr/lib64"} {
		facts := directory
		facts.UID, facts.GID, facts.Mode, facts.Inode = 0, 0, unix.S_IFDIR|0755, uint64(200+i)
		policy.Runtime[i] = readonlyPolicyObject{Path: path, Facts: facts}
	}
	for i, number := range [6][2]uint32{{1, 3}, {1, 5}, {1, 7}, {1, 8}, {1, 9}, {5, 0}} {
		facts := system
		facts.Mode, facts.Size, facts.Device, facts.Inode = unix.S_IFCHR|0666, 0, unix.Mkdev(0, 5), uint64(300+i)
		policy.Devices[i] = readonlyPolicyDevice{Facts: facts, Rdev: unix.Mkdev(number[0], number[1])}
	}
	return policy
}

func readonlyPolicyBodyFile(t *testing.T, body []byte, sealed bool) *os.File {
	t.Helper()
	fd, err := unix.MemfdCreate("readonly-policy-contract", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(fd), "readonly-policy-contract")
	t.Cleanup(func() { _ = file.Close() })
	if n, err := file.Write(body); err != nil || n != len(body) {
		t.Fatalf("write private policy: %d %v", n, err)
	}
	if sealed {
		if _, err := unix.FcntlInt(file.Fd(), unix.F_ADD_SEALS, readonlyPolicySeals); err != nil {
			t.Fatal(err)
		}
	}
	return file
}

func TestReadonlyPolicyIsSealedCanonicalAndKeepsReadOffset(t *testing.T) {
	want := readonlyPolicyFixture()
	file, digest, err := sealReadonlyPolicy(want)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.Seek(7, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	actual, actualDigest, err := readReadonlyPolicy(file)
	if err != nil || !reflect.DeepEqual(actual, want) || actualDigest != digest {
		t.Fatalf("sealed policy did not join: %+v %q %v", actual, actualDigest, err)
	}
	if offset, err := file.Seek(0, io.SeekCurrent); err != nil || offset != 7 {
		t.Fatalf("private policy read moved its open description: %d %v", offset, err)
	}
	if _, err := file.WriteAt([]byte("changed"), 0); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("sealed original policy was writable: %v", err)
	}
	if err := file.Truncate(0); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("sealed original policy was resizable: %v", err)
	}
}

func TestReadonlyPolicyRejectsUnsealedDuplicateAndForeignAuthority(t *testing.T) {
	body, err := json.Marshal(readonlyPolicyFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := readReadonlyPolicy(readonlyPolicyBodyFile(t, body, false)); err == nil {
		t.Fatal("accepted an unsealed policy")
	}
	for name, changed := range map[string][]byte{
		"duplicate field": bytes.Replace(body, []byte(`"Version":1`), []byte(`"Version":1,"Version":1`), 1),
		"unknown field":   bytes.Replace(body, []byte(`"Version":1`), []byte(`"Version":1,"Mounts":["/"]`), 1),
		"noncanonical":    append(append([]byte(nil), body...), '\n'),
		"foreign HOME":    bytes.Replace(body, []byte(`"HOME=/home/native"`), []byte(`"HOME=/other"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := readReadonlyPolicy(readonlyPolicyBodyFile(t, changed, true)); err == nil {
				t.Fatal("accepted malformed or unjoined sealed authority")
			}
		})
	}
}

func TestReadonlyHeaderPreservesNativeTailAndBindsTheOriginalPolicy(t *testing.T) {
	policy := readonlyPolicyFixture()
	digest := strings.Repeat("e", 64)
	tail := []byte("{\"jsonrpc\":\"2.0\",\"id\":1}\n")
	stream := bytes.NewReader(append([]byte(readonlyHeader(policy, digest)), tail...))
	if err := readReadonlyHeader(context.Background(), stream, policy, digest); err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(stream)
	if err != nil || !bytes.Equal(actual, tail) {
		t.Fatalf("private bootstrap consumed native output: %q %v", actual, err)
	}
	for _, body := range []string{readonlyHeader(policy, strings.Repeat("f", 64)), "agentplugins-readonly-v1:"} {
		if err := readReadonlyHeader(context.Background(), strings.NewReader(body), policy, digest); err == nil {
			t.Fatal("accepted a foreign or incomplete private view")
		}
	}
	policy.Deadline = 1
	untouched := bytes.NewReader(tail)
	if err := readReadonlyHeader(context.Background(), untouched, policy, digest); err == nil || untouched.Len() != len(tail) {
		t.Fatal("expired original cutoff consumed input")
	}
}

func TestReadonlyDeadlineRequiresTheExistingFiniteContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cutoff, err := readonlyOriginalDeadline(ctx)
	if err != nil || cutoff <= 0 {
		t.Fatalf("original deadline refused: %d %v", cutoff, err)
	}
	cancel()
	if _, err := readonlyOriginalDeadline(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled original context lost: %v", err)
	}
	if _, err := readonlyOriginalDeadline(context.Background()); err == nil {
		t.Fatal("invented a new deadline for an unbounded caller")
	}
	long, stop := context.WithTimeout(context.Background(), time.Minute)
	defer stop()
	if _, err := readonlyOriginalDeadline(long); err == nil {
		t.Fatal("accepted a deadline beyond the existing native bound")
	}
}
