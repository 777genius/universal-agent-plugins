//go:build linux

package gitsource

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRuntimeEnvelopeFailsClosed(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		memory, pids string
		accepted     bool
	}{
		{"536870912", "64", true}, {"max", "64", false}, {"536870913", "64", false},
		{"536870912", "max", false}, {"536870912", "65", false}, {"0", "64", false},
	} {
		put(t, root, "memory.max", tc.memory)
		put(t, root, "pids.max", tc.pids)
		err := checkCgroupBounds(root, "0::/\n")
		if (err == nil) != tc.accepted {
			t.Fatal("incorrect cgroup admission", tc, err)
		}
	}
	if err := checkCgroupBounds(root, "0::/../../other\n"); err == nil {
		t.Fatal("unsafe cgroup namespace path accepted")
	}
	if _, err := NewHosted(t.TempDir()); err == nil {
		t.Fatal("ordinary scratch lacks required runtime envelope")
	}
}

// The dedicated CI job runs this test binary as UID 65532 in a network-disabled
// scratch container with a real 256 MiB tmpfs, 512 MiB memory and 64 PID limit.
// The ordinary source test lane cannot qualify those host/kernel boundaries.
func TestHostedRuntimeEnvelopeAndDiskExhaustion(t *testing.T) {
	if os.Getenv("UAP_RESOLVE_RUNTIME_PROBE") != "1" {
		t.Skip("executed by the dedicated bounded-runtime CI job")
	}
	scratch := os.Getenv("SKILLS_RESOLVER_TEMP_DIR")
	if scratch == "" {
		scratch = "/tmp"
	}
	if _, err := NewHosted(scratch); err != nil {
		t.Fatal("actual runtime envelope rejected", err)
	}
	file, err := os.CreateTemp(scratch, "resolver-disk-probe-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close(); _ = os.Remove(file.Name()) }()
	chunk := make([]byte, 1<<20)
	var written int64
	for written <= 256<<20 {
		n, writeErr := file.Write(chunk)
		written += int64(n)
		if writeErr != nil {
			if !errors.Is(writeErr, syscall.ENOSPC) {
				t.Fatal("unexpected bounded-filesystem error", writeErr)
			}
			break
		}
	}
	if written <= 0 || written > 256<<20 {
		t.Fatal("temporary filesystem was not hard bounded", written)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(file.Name()); err != nil {
		t.Fatal(err)
	}
	probe, err := os.CreateTemp(filepath.Dir(file.Name()), "after-cleanup-*")
	if err != nil {
		t.Fatal("scratch did not recover after cleanup", err)
	}
	if err = probe.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(probe.Name()); err != nil {
		t.Fatal(err)
	}
	t.Logf("verified unprivileged cgroup limits and ENOSPC after %d temporary bytes", written)
}
