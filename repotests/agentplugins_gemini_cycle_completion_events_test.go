//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"testing"
	"time"
)

// The public Project completer runs skills list, then mcp list. The pinned
// inventory commands respectively initialize/list and only initialize each
// selected server. Neither phase receives model or tools/call authority.
// Repeated add/update perform two separate inventory probes before those
// completion phases.
func geminiCyclePublicInventoryPhases(label string) []int {
	switch label {
	case "add-a", "add-b", "update-a", "intact-repair-a", "missing-repair-a":
		return []int{2, 1}
	case "repeat-add-a", "unchanged-update-a":
		return []int{2, 2, 2, 1}
	default:
		return nil
	}
}
func geminiCyclePublicInventory(segment []byte, label string, allowed map[string]*geminiCycleRecipe, home string, start, end time.Time, priorSessions map[string]bool, priorPIDs map[int64]bool) (map[string]bool, map[int64]bool, error) {
	phases := geminiCyclePublicInventoryPhases(label)
	if len(phases) == 0 {
		if len(segment) != 0 {
			return nil, nil, fmt.Errorf("Gemini unexpected public inventory")
		}
		return maps.Clone(priorSessions), maps.Clone(priorPIDs), nil
	}
	sessions, pids := maps.Clone(priorSessions), maps.Clone(priorPIDs)
	if sessions == nil {
		sessions = map[string]bool{}
	}
	if pids == nil {
		pids = map[int64]bool{}
	}
	rows := bytes.SplitAfter(segment, []byte{'\n'})
	count := 0
	for _, methods := range phases {
		count += methods * len(allowed)
	}
	if len(allowed) != 2 || len(rows) != count+1 || len(rows[count]) != 0 {
		return nil, nil, fmt.Errorf("Gemini exact public inventory phase count")
	}
	offset := 0
	phaseStart := start
	for _, methods := range phases {
		next := offset + methods*len(allowed)
		groups, err := geminiCycleProbeMethods(bytes.Join(rows[offset:next], nil), allowed, nil, home, phaseStart, end, sessions, pids, methods == 1)
		if err != nil || len(groups) != len(allowed) {
			return nil, nil, fmt.Errorf("Gemini exact public inventory exchange")
		}
		for _, g := range groups {
			pid, _ := g.pid.Int64()
			sessions[g.session] = true
			pids[pid] = true
		}
		for _, raw := range rows[offset:next] {
			var row struct {
				Timestamp string `json:"timestamp"`
			}
			if json.Unmarshal(raw, &row) != nil {
				return nil, nil, fmt.Errorf("Gemini public event timestamp")
			}
			stamp, err := geminiCompleteTime(row.Timestamp)
			if err != nil {
				return nil, nil, err
			}
			if stamp.After(phaseStart) {
				phaseStart = stamp
			}
		}
		offset = next
	}
	return sessions, pids, nil
}
func (c *geminiProjectCycle) publicInventory(t *testing.T, label string, before []byte, start, end time.Time, providerTotal int) (map[string]any, error) {
	t.Helper()
	after, err := c.eventLog()
	if err != nil || !bytes.HasPrefix(after, before) {
		t.Fatal("Gemini public append-only probe stream")
	}
	allowed := map[string]*geminiCycleRecipe{}
	if len(geminiCyclePublicInventoryPhases(label)) > 0 {
		for _, scope := range []string{"user", geminiCycleScope(label)} {
			r := c.recipes[scope]
			allowed[r.Nonce] = r
		}
	}
	sessions, pids, err := geminiCyclePublicInventory(after[len(before):], label, allowed, c.f.base.Home, start, end, c.probeSessions, c.probePIDs)
	c.owner.mu.Lock()
	providerDelta := c.owner.total - providerTotal
	unchanged := providerDelta == 0 && c.owner.busy == 0 && !c.owner.failure
	c.owner.mu.Unlock()
	inventory := map[string]any{"scope": geminiCycleScope(label), "started_at": start.UTC().Format(time.RFC3339Nano), "finished_at": end.UTC().Format(time.RFC3339Nano), "provider_requests": providerDelta, "stream": c.stream(t, after[len(before):])}
	if err != nil {
		return inventory, fmt.Errorf("Gemini public completion native/model authority: %w", err)
	}
	if !unchanged {
		return inventory, fmt.Errorf("Gemini public completion changed model authority")
	}
	c.eventsAccepted = bytes.Clone(after)
	c.probeSessions, c.probePIDs = sessions, pids
	c.openEvents = false
	return inventory, nil
}
