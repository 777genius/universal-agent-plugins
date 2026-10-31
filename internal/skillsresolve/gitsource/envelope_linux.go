//go:build linux

package gitsource

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func checkRuntimeEnvelope(tempDir string) error {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(tempDir, &fs); err != nil {
		return fmt.Errorf("resolver scratch bounds unavailable")
	}
	if fs.Type != 0x01021994 || fs.Bsize <= 0 || fs.Blocks > (256<<20)/uint64(fs.Bsize) {
		return fmt.Errorf("resolver requires a temporary filesystem bounded to 256 MiB")
	}
	group, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return fmt.Errorf("resolver process bounds unavailable")
	}
	return checkCgroupBounds("/sys/fs/cgroup", string(group))
}

func checkCgroupBounds(root, membership string) error {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("resolver cgroup directory unavailable")
	}
	defer func() { _ = directory.Close() }()
	for _, line := range strings.Split(membership, "\n") {
		if !strings.HasPrefix(line, "0::/") {
			continue
		}
		relative := strings.TrimPrefix(line, "0::/")
		if filepath.Clean("/"+relative) != "/"+relative || strings.Contains(relative, "..") {
			return fmt.Errorf("resolver cgroup identity unavailable")
		}
		if !boundedFile(directory, filepath.Join(relative, "memory.max"), 512<<20) || !boundedFile(directory, filepath.Join(relative, "pids.max"), 64) {
			return fmt.Errorf("resolver requires memory and task limits")
		}
		return nil
	}
	return fmt.Errorf("resolver requires cgroup v2 process limits")
}

func boundedFile(root *os.Root, path string, ceiling uint64) bool {
	body, err := root.ReadFile(path)
	if err != nil || len(body) > 64 {
		return false
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(body)), 10, 64)
	return err == nil && value > 0 && value <= ceiling
}
