//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type cursorCycleProbeRow struct {
	Method, Nonce, Session, Timestamp string
	PID                               int
	Facts                             any
}

func (c *cursorMCPProjectCycle) beforeProbe(t *testing.T) {
	t.Helper()
	raw, err := projectNativeRuntimeRead(c.f.base.Events, 1<<20)
	info, statErr := os.Lstat(c.f.base.Events)
	if err != nil || statErr != nil || !bytes.Equal(raw, c.probe) || !os.SameFile(info, c.probeInfo) || !projectNativeRuntimeStableStat(info.Sys(), c.probeInfo.Sys()) {
		t.Fatal("Cursor original probe prefix/authority changed")
	}
}

func cursorCycleProbeDelta(raw []byte, nonce string, start, end time.Time, previous map[string]bool) ([]string, error) {
	if len(raw) == 0 || len(raw) > 1<<20 || raw[len(raw)-1] != '\n' || end.Before(start) || end.Sub(start) > 35*time.Second {
		return nil, fmt.Errorf("Cursor probe frame/budget differs")
	}
	type progress struct{ pid, lists int }
	sessions, pids := map[string]progress{}, map[int]string{}
	last := start
	for _, line := range bytes.Split(raw[:len(raw)-1], []byte{'\n'}) {
		value, err := projectNativeRuntimeJSON(line)
		object, ok := value.(map[string]any)
		if err != nil || !ok || projectNativeRuntimeKeys(object, "method nonce pid session timestamp facts", "") != nil {
			return nil, fmt.Errorf("Cursor closed probe event differs")
		}
		var row cursorCycleProbeRow
		if json.Unmarshal(line, &row) != nil || row.Facts != nil || row.Nonce != nonce || row.PID <= 0 || row.Session == "" || previous[row.Session] {
			return nil, fmt.Errorf("Cursor probe challenge/session differs")
		}
		parts := strings.Split(row.Session, "-")
		if len(parts) != 2 || parts[0] != strconv.Itoa(row.PID) || parts[1] == "" {
			return nil, fmt.Errorf("Cursor intrinsic probe identity differs")
		}
		serial, serialErr := strconv.ParseUint(parts[1], 10, 64)
		if serialErr != nil || serial == 0 || strconv.FormatUint(serial, 10) != parts[1] {
			return nil, fmt.Errorf("Cursor intrinsic probe identity differs")
		}
		for _, digit := range parts[1] {
			if digit < '0' || digit > '9' {
				return nil, fmt.Errorf("Cursor intrinsic probe identity differs")
			}
		}
		when, err := time.Parse(time.RFC3339Nano, row.Timestamp)
		if err != nil || !strings.HasSuffix(row.Timestamp, "Z") || when.Before(last) || when.After(end) {
			return nil, fmt.Errorf("Cursor probe command time differs")
		}
		last = when
		state, exists := sessions[row.Session]
		if row.Method == "initialize" {
			if exists || pids[row.PID] != "" {
				return nil, fmt.Errorf("Cursor probe session initialized twice")
			}
			sessions[row.Session], pids[row.PID] = progress{pid: row.PID}, row.Session
		} else if row.Method == "tools/list" && exists && state.pid == row.PID {
			state.lists++
			sessions[row.Session] = state
		} else {
			return nil, fmt.Errorf("Cursor probe must only initialize then list tools")
		}
	}
	ids := []string{}
	for id, state := range sessions {
		if state.lists == 0 {
			return nil, fmt.Errorf("Cursor probe lacks tools discovery")
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("Cursor command lacks fresh MCP session")
	}
	return ids, nil
}

func (c *cursorMCPProjectCycle) observeProbe(t *testing.T, step cursorCycleStep, start, end time.Time) {
	t.Helper()
	raw, err := projectNativeRuntimeRead(c.f.base.Events, 1<<20)
	info, statErr := os.Lstat(c.f.base.Events)
	if err != nil || statErr != nil || !bytes.HasPrefix(raw, c.probe) || !cursorCycleDirectorySame(c.probeInfo, info) {
		t.Fatal("Cursor probe original prefix/identity changed")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || info.Mode() != 0600 || info.Size() != int64(len(raw)) {
		t.Fatal("Cursor probe file authority differs")
	}
	delta := raw[len(c.probe):]
	c.append(t, cursorCycleMembers[2], step.label, map[string]any{"before_bytes": len(c.probe), "before_sha256": clineAttemptSHA(c.probe), "after_bytes": len(raw), "after_sha256": clineAttemptSHA(raw), "delta": c.stream(t, delta)})
	if step.verb == "list-tools" {
		ids, err := cursorCycleProbeDelta(delta, c.recipes[step.scope].Nonce, start, end, c.sessions)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			c.sessions[id] = true
		}
	} else if len(delta) != 0 || !projectNativeRuntimeStableStat(c.probeInfo.Sys(), info.Sys()) {
		t.Fatal("Cursor non-tools command changed probe events")
	}
	c.probe, c.probeInfo = bytes.Clone(raw), info
}
