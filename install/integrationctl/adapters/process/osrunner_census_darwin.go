//go:build darwin

package process

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func survivingCommands(ctx context.Context) ([]ports.LiveProcess, error) {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, fmt.Errorf("list live processes: %w", err)
	}
	parents := make(map[int]int, len(processes))
	for _, process := range processes {
		parents[int(process.Proc.P_pid)] = int(process.Eproc.Ppid)
	}
	ancestry := ownAncestry(parents)
	uid := os.Getuid()
	const zombieState = 5
	var live []ports.LiveProcess
	for _, process := range processes {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pid := int(process.Proc.P_pid)
		owned := int(process.Eproc.Pcred.P_ruid) == uid || int(process.Eproc.Ucred.Uid) == uid
		if !owned || ancestry[pid] || process.Proc.P_stat == zombieState {
			continue
		}
		live = append(live, darwinLiveProcess(pid, process.Proc.P_comm))
	}
	return live, nil
}

// darwinLiveProcess reads the executable path, arguments and environment the
// kernel keeps for pid. A process that hides them or exits meanwhile keeps its
// short command name, so a caller can still recognize it conservatively.
func darwinLiveProcess(pid int, comm [17]byte) ports.LiveProcess {
	process := ports.LiveProcess{PID: pid, Executable: unix.ByteSliceToString(comm[:])}
	raw, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return process
	}
	if executable, argv, env, ok := parseProcargs2(raw); ok {
		process.Executable, process.Argv, process.Env = executable, argv, env
	}
	return process
}

// parseProcargs2 decodes KERN_PROCARGS2: argc, the executable path, alignment
// padding, then argc arguments and the environment, each NUL-terminated.
func parseProcargs2(raw []byte) (string, []string, []string, bool) {
	if len(raw) < 4 {
		return "", nil, nil, false
	}
	argc := int(binary.NativeEndian.Uint32(raw[:4]))
	executable, rest, found := bytes.Cut(raw[4:], []byte{0})
	fields := nulFields(bytes.TrimLeft(rest, "\x00"))
	if !found || argc > len(fields) {
		return "", nil, nil, false
	}
	env := fields[argc:]
	for i, entry := range env {
		if entry == "" {
			env = env[:i]
			break
		}
	}
	return string(executable), fields[:argc], env, true
}
