//go:build linux

package process

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func survivingCommands(ctx context.Context) ([]ports.LiveProcess, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("list live processes: %w", err)
	}
	uid := os.Getuid()
	parents := map[int]int{}
	var owned []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		parent, state, owner, ok := linuxProcessStatus(pid)
		if !ok {
			continue
		}
		parents[pid] = parent
		if int(owner) == uid && state != 'Z' && state != 'X' {
			owned = append(owned, pid)
		}
	}
	ancestry := ownAncestry(parents)
	var live []ports.LiveProcess
	for _, pid := range owned {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !ancestry[pid] {
			live = append(live, linuxLiveProcess(pid))
		}
	}
	return live, nil
}

// linuxProcessStatus reads the parent and state from /proc/<pid>/stat and the
// owner of the process directory. The command name in parentheses may contain
// spaces and parentheses, so the fields are taken after the last ')'.
func linuxProcessStatus(pid int) (int, byte, uint32, bool) {
	root := "/proc/" + strconv.Itoa(pid)
	info, err := os.Stat(root)
	if err != nil {
		return 0, 0, 0, false
	}
	owner, isStat := info.Sys().(*syscall.Stat_t)
	body, err := os.ReadFile(root + "/stat")
	end := bytes.LastIndexByte(body, ')')
	if err != nil || !isStat || end < 0 {
		return 0, 0, 0, false
	}
	fields := strings.Fields(string(body[end+1:]))
	if len(fields) < 2 || fields[0] == "" {
		return 0, 0, 0, false
	}
	parent, err := strconv.Atoi(fields[1])
	return parent, fields[0][0], owner.Uid, err == nil
}

// linuxLiveProcess reads what the kernel exposes for pid. A process that exits
// meanwhile or hides its arguments keeps its short command name.
func linuxLiveProcess(pid int) ports.LiveProcess {
	root := "/proc/" + strconv.Itoa(pid)
	process := ports.LiveProcess{PID: pid}
	process.Executable, _ = os.Readlink(root + "/exe")
	if body, err := os.ReadFile(root + "/cmdline"); err == nil {
		process.Argv = nulFields(body)
	}
	if body, err := os.ReadFile(root + "/environ"); err == nil {
		process.Env = nulFields(body)
	}
	if process.Executable == "" {
		if body, err := os.ReadFile(root + "/comm"); err == nil {
			process.Executable = strings.TrimSpace(string(body))
		}
	}
	return process
}
