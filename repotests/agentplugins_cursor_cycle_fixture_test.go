//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const cursorCycleMode = "cursor-complete-mcp-cycle"
const cursorCycleName = "native-proof"

var cursorCycleMembers = []string{"cursor-cycle-commands.jsonl", "cursor-cycle-proofs.jsonl", "cursor-cycle-probe.jsonl"}

type cursorCycleRecipe struct {
	Scope, Project, Source, Version, Nonce, Revision, Digest string
	Authored                                                 map[string][]byte
	Installation                                             domain.Installation
	Binding                                                  domain.ClientBinding
	Data                                                     domain.DataReceipt
	Active, Config                                           string
	Removed                                                  bool
}

type cursorMCPProjectCycle struct {
	f                        *cursorProjectAttempt
	recipes                  map[string]*cursorCycleRecipe
	aggregates               map[string][]byte
	completed                []string
	proofs                   []string
	approvals                map[string][]string
	priorApproval            map[string]cursorProjectHeld
	probe                    []byte
	probeInfo                os.FileInfo
	sessions                 map[string]bool
	trees                    map[string]map[string][]byte
	dirs                     map[string]os.FileInfo
	installerBody, probeBody []byte
	stage                    string
	complete                 bool
	revising                 bool
}

func newCursorMCPProjectCycle(t *testing.T, f *cursorProjectAttempt) *cursorMCPProjectCycle {
	t.Helper()
	c := &cursorMCPProjectCycle{f: f, recipes: map[string]*cursorCycleRecipe{}, aggregates: map[string][]byte{}, approvals: map[string][]string{}, priorApproval: map[string]cursorProjectHeld{}, sessions: map[string]bool{}, trees: map[string]map[string][]byte{}, dirs: map[string]os.FileInfo{}, completed: []string{}, proofs: []string{}}
	f.cycle = c
	f.evidence["capture_mode"] = cursorCycleMode
	// The new recipe deliberately authors an empty original User document.
	// Keep the earlier failed-only constructor and its receipts unchanged.
	delete(f.evidence, "other_source_absent")
	delete(f.evidence, "approval_controls_initially_absent")
	return c
}

func cursorCycleStream(raw []byte) (clineCycleBytes, error) {
	marker := []byte("uap-cursor-private-")
	budget := clineCyclePrivacyBudget{text: 4 * len(raw), nodes: 16384}
	if len(raw) > 1<<20 || !utf8.Valid(raw) || bytes.Contains(raw, marker) || clineCycleCredentialJSON(raw, marker, 0, &budget) {
		return clineCycleBytes{}, fmt.Errorf("Cursor cycle private/unbounded stream refused")
	}
	return clineCycleBytes{Bytes: len(raw), SHA256: clineAttemptSHA(raw), Body: append([]byte{}, raw...)}, nil
}

func (c *cursorMCPProjectCycle) stream(t *testing.T, raw []byte) clineCycleBytes {
	t.Helper()
	value, err := cursorCycleStream(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func (c *cursorMCPProjectCycle) append(t *testing.T, member, stage string, value any) {
	t.Helper()
	known := false
	for _, name := range cursorCycleMembers {
		known = known || name == member
	}
	raw, err := json.Marshal(map[string]any{"stage": stage, "value": value})
	if err != nil || !known || stage == "" {
		t.Fatal("Cursor cycle closed aggregate/stage")
	}
	raw = append(raw, '\n')
	total := len(raw)
	for _, body := range c.aggregates {
		total += len(body)
	}
	for _, body := range c.f.files {
		total += len(body)
	}
	if len(c.aggregates[member])+len(raw) > 1<<20 || total > 16<<20 {
		t.Fatal("Cursor cycle encoded aggregate bound")
	}
	c.aggregates[member] = append(c.aggregates[member], raw...)
}

func (c *cursorMCPProjectCycle) forget(path string) {
	delete(c.f.held, path)
	c.removeAbsent(path)
}

func (c *cursorMCPProjectCycle) removeAbsent(path string) {
	for i := 0; i < len(c.f.absent); {
		if c.f.absent[i] == path {
			c.f.absent = append(c.f.absent[:i], c.f.absent[i+1:]...)
		} else {
			i++
		}
	}
}

func (c *cursorMCPProjectCycle) proof(t *testing.T, stage string, value any) {
	t.Helper()
	c.append(t, cursorCycleMembers[1], stage, value)
	c.proofs = append(c.proofs, stage)
}

func (c *cursorMCPProjectCycle) statePath(r *cursorCycleRecipe) string {
	return filepath.Join(r.Project, ".uap", "state-v2.json")
}
