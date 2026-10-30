//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

func windCycleParent(old, now windProfileRecord, start, end time.Time) bool {
	return old.Kind == "directory" && now.Kind == "directory" && old.Device == now.Device && old.Inode == now.Inode && old.Inode > 0 && old.Mode == now.Mode && old.UID == now.UID && old.GID == now.GID && now.UID == uint32(os.Getuid()) && now.GID == uint32(os.Getgid()) && now.Size >= 0 && now.Size <= 1<<20 && old.SHA256 == "" && now.SHA256 == "" && windRuntimeCommandTimes(now, start, end)
}

func windCycleLock(path string, old, now windProfileRecord, start, end time.Time) error {
	refused := fmt.Errorf("Windsurf selected mutation lock authority")
	if now.Kind != "file" || now.Mode != 0100600 || now.UID != uint32(os.Getuid()) || now.GID != uint32(os.Getgid()) || now.Inode == 0 || now.Links != 1 || now.Size <= 0 || now.Size > 4096 || !windRuntimeCommandTimes(now, start, end) {
		return refused
	}
	if old.Kind != "" && !windRuntimeAuthority(old, now) {
		return refused
	}
	raw, err := projectNativeRuntimeRead(path, 4096)
	if err != nil || int64(len(raw)) != now.Size || clineAttemptSHA(raw) != now.SHA256 {
		return refused
	}
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "pid acquired_at", "") != nil {
		return refused
	}
	pid, ok := row["pid"].(json.Number)
	if !ok {
		return refused
	}
	n, err := pid.Int64()
	if err != nil || n <= 0 {
		return refused
	}
	stamp, ok := row["acquired_at"].(string)
	if !ok {
		return refused
	}
	instant, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil || !strings.HasSuffix(stamp, "Z") || instant.Before(start) || instant.After(end) {
		return refused
	}
	canonical, _ := json.Marshal(struct {
		PID      int64  `json:"pid"`
		Acquired string `json:"acquired_at"`
	}{n, stamp})
	return windCycleRequired(string(raw) == string(canonical)+"\n", "canonical mutation lock")
}

func (c *windProjectCycle) frames(label string, r *windComponentRecipe, frames map[string]map[string]windProfileRecord, start, end time.Time) error {
	w := c.w
	if label == "repeat-remove-a" {
		for root, after := range frames {
			if !reflect.DeepEqual(w.trees[root], after) {
				return fmt.Errorf("Windsurf repeated removal changed held frame")
			}
		}
	}
	unselected := map[string]map[string]windProfileRecord{}
	for path, frame := range w.trees {
		if _, selected := frames[path]; !selected {
			unselected[path] = frame
		}
	}
	if err := windFrozenTrees(w.f.ctx, unselected); err != nil {
		return err
	}
	if err := c.profileFrame(label, r, frames[r.Profile], start, end); err != nil {
		return err
	}
	if err := w.projectClosure(r, frames[r.Project]); err != nil {
		return err
	}
	global := w.recipes["user"].StateRoot
	if r.StateRoot != global {
		if err := c.globalFrame(label, w.trees[global], frames[global], start, end); err != nil {
			return err
		}
	}
	return c.selectedFrame(label, r, frames[r.StateRoot], start, end)
}

func (c *windProjectCycle) globalFrame(label string, before, after map[string]windProfileRecord, start, end time.Time) error {
	for name, old := range before {
		now, ok := after[name]
		if ok && old == now {
			continue
		}
		if windCycleCold(label) && name == windAssessmentDirectory {
			continue
		}
		if name == "." && !strings.Contains(label, "remove") && windSourceScratchRoot(old, now, start, end) {
			continue
		}
		return fmt.Errorf("Windsurf changed unrelated global state")
	}
	for name := range after {
		if _, old := before[name]; !old && (!windCycleCold(label) || !c.cacheKeys[name]) {
			return fmt.Errorf("Windsurf unknown global state entry")
		}
	}
	return nil
}

func (c *windProjectCycle) selectedFrame(label string, r *windComponentRecipe, after map[string]windProfileRecord, start, end time.Time) error {
	before := c.w.trees[r.StateRoot]
	active, _ := filepath.Rel(r.StateRoot, r.Active)
	data, _ := filepath.Rel(r.StateRoot, r.Data)
	parents := map[string]bool{".": true, "operations-v2": true}
	for _, leaf := range []string{active, data} {
		for p := filepath.Dir(leaf); p != "."; p = filepath.Dir(p) {
			parents[p] = true
		}
	}
	mutates := windCycleMutated(label)
	for name, now := range after {
		old, had := before[name]
		if had && old == now {
			continue
		}
		if r.Scope == "user" && windCycleCold(label) && (name == windAssessmentDirectory || c.cacheKeys[name]) {
			continue
		}
		switch {
		case name == "mutation.lock":
			if err := windCycleLock(filepath.Join(r.StateRoot, name), old, now, start, end); err != nil {
				return err
			}
		case parents[name]:
			if now.Kind != "directory" || now.Mode != 040700 || now.UID != uint32(os.Getuid()) || now.GID != uint32(os.Getgid()) || now.Size < 0 || now.Size > 1<<20 || had && !windCycleParent(old, now, start, end) {
				return fmt.Errorf("Windsurf selected parent authority")
			}
		case name == "state-v2.json":
			if !mutates || now.Kind != "file" || now.Mode != 0100600 || now.UID != uint32(os.Getuid()) || now.GID != uint32(os.Getgid()) || now.Links != 1 {
				return fmt.Errorf("Windsurf selected state authority")
			}
		case name == active || strings.HasPrefix(name, active+"/"):
			if !mutates || c.removed[r.Scope] {
				return fmt.Errorf("Windsurf unexpected payload mutation")
			}
		case name == data || name == data+"/.agentplugins-data-owner.json":
			if r.Installation.InstallationID != "" || now.UID != uint32(os.Getuid()) || now.GID != uint32(os.Getgid()) || name == data && (now.Kind != "directory" || now.Mode != 040700) || name != data && (now.Kind != "file" || now.Mode != 0100600 || now.Links != 1) {
				return fmt.Errorf("Windsurf owned data changed")
			}
		default:
			return fmt.Errorf("Windsurf lifecycle escaped selected state")
		}
	}
	for name := range before {
		if _, exists := after[name]; !exists && (!c.removed[r.Scope] || name != active && !strings.HasPrefix(name, active+"/")) {
			return fmt.Errorf("Windsurf unrelated selected state deletion")
		}
	}
	if c.removed[r.Scope] {
		if _, exists := after[active]; exists {
			return fmt.Errorf("Windsurf removed payload remains")
		}
	}
	return nil
}

func (c *windProjectCycle) profileFrame(label string, r *windComponentRecipe, after map[string]windProfileRecord, start, end time.Time) error {
	before := c.w.trees[r.Profile]
	owned := "skills/" + windComponentName
	removed := c.removed[r.Scope]
	for name, old := range before {
		now, has := after[name]
		if has && old == now {
			continue
		}
		if name == "." || name == "skills" {
			if !has || !windCycleParent(old, now, start, end) {
				return fmt.Errorf("Windsurf native parent authority")
			}
			continue
		}
		if name == "mcp_config.json" {
			if !windCycleMutated(label) || !has || now.Kind != "file" || now.Mode != 0100600 || now.Links != 1 {
				return fmt.Errorf("Windsurf native config mutation")
			}
			continue
		}
		if r.Scope != "user" && (name == owned || name == owned+"/SKILL.md") && windCycleMutated(label) {
			continue
		}
		return fmt.Errorf("Windsurf native foreign entry changed")
	}
	for name, now := range after {
		if _, had := before[name]; had {
			continue
		}
		if r.Scope == "user" || removed || name != owned && name != owned+"/SKILL.md" {
			return fmt.Errorf("Windsurf native unexpected entry")
		}
		if name == owned && (now.Kind != "directory" || now.Mode != 040755) || name != owned && (now.Kind != "file" || now.Mode != 0100644 || now.SHA256 != clineAttemptSHA(r.Authored[owned+"/SKILL.md"])) {
			return fmt.Errorf("Windsurf native skill authority")
		}
	}
	raw, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil {
		return err
	}
	servers := map[string]any{"foreign-" + r.Scope: windForeign(r.Scope)}
	if !removed {
		servers[windComponentName] = c.entry(r)
	}
	expected, _ := json.Marshal(map[string]any{"mcpServers": servers})
	actual, aerr := projectNativeRuntimeJSON(raw)
	want, werr := projectNativeRuntimeJSON(expected)
	if aerr != nil || werr != nil || !reflect.DeepEqual(actual, want) {
		return fmt.Errorf("Windsurf native complete selected/foreign config")
	}
	if r.Scope != "user" {
		_, has := after[owned]
		if has == removed {
			return fmt.Errorf("Windsurf selected skill presence")
		}
		if has && (after[owned].Kind != "directory" || after[owned].Mode != 040755 || after[owned+"/SKILL.md"].Kind != "file" || after[owned+"/SKILL.md"].Mode != 0100644 || after[owned+"/SKILL.md"].Links != 1 || after[owned+"/SKILL.md"].SHA256 != clineAttemptSHA(r.Authored[owned+"/SKILL.md"])) {
			return fmt.Errorf("Windsurf selected skill exact bytes")
		}
	}
	return nil
}
