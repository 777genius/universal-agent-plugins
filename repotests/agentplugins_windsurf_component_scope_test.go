//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The public mutation closure is finite and derived from compose.go,
// PluginDataManager, Stager and the single selected native adapter.
func (w *windComponentFixture) scopeClosure(r *windComponentRecipe) error {
	tree, err := windOwnedTree(w.f.ctx, r.StateRoot)
	if err != nil {
		return err
	}
	before := w.trees[r.StateRoot]
	activeRel, err := filepath.Rel(r.StateRoot, r.Active)
	if err != nil {
		return err
	}
	dataRel, err := filepath.Rel(r.StateRoot, r.Data)
	if err != nil {
		return err
	}
	ancestors := map[string]bool{".": true, "operations-v2": true}
	for _, relative := range []string{activeRel, dataRel} {
		for dir := filepath.Dir(relative); dir != "."; dir = filepath.Dir(dir) {
			ancestors[dir] = true
		}
	}
	for name, record := range tree {
		if previous, ok := before[name]; ok && previous == record {
			continue
		}
		switch {
		case name == "state-v2.json":
			if record.Kind != "file" {
				return fmt.Errorf("component state leaf type")
			}
		case name == "mutation.lock":
			if err := windMutationLock(filepath.Join(r.StateRoot, name)); err != nil {
				return err
			}
		case ancestors[name]:
			if record.Kind != "directory" || record.Mode&07777 != 0700 {
				return fmt.Errorf("component public parent mode")
			}
			if previous, ok := before[name]; ok && (previous.Device != record.Device || previous.Inode != record.Inode || previous.Mode != record.Mode || previous.UID != record.UID || previous.GID != record.GID) {
				return fmt.Errorf("component public parent identity")
			}
		case name == activeRel || strings.HasPrefix(name, activeRel+"/"):
			// Complete payload membership/content was independently proven before here.
		case name == dataRel:
			if record.Kind != "directory" || record.Mode&07777 != 0700 {
				return fmt.Errorf("component owned data mode")
			}
		case name == dataRel+"/.agentplugins-data-owner.json":
			if record.Kind != "file" || record.Mode&07777 != 0600 {
				return fmt.Errorf("component data marker mode")
			}
		default:
			return fmt.Errorf("component public mutation escaped selected closure")
		}
	}
	for name := range before {
		if _, ok := tree[name]; !ok {
			return fmt.Errorf("component public mutation removed prior scope entry")
		}
	}
	if _, ok := tree[activeRel]; !ok {
		return fmt.Errorf("component payload absent")
	}
	if _, ok := tree[dataRel+"/.agentplugins-data-owner.json"]; !ok {
		return fmt.Errorf("component data absent")
	}
	return nil
}
func windMutationLock(path string) error {
	raw, err := projectNativeRuntimeRead(path, 4096)
	if err != nil {
		return err
	}
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "pid acquired_at", "") != nil {
		return fmt.Errorf("component mutation lock metadata")
	}
	pid, ok := row["pid"].(json.Number)
	if !ok {
		return fmt.Errorf("component lock pid")
	}
	number, err := pid.Int64()
	if err != nil || number <= 0 {
		return fmt.Errorf("component lock pid range")
	}
	stamp, ok := row["acquired_at"].(string)
	if !ok {
		return fmt.Errorf("component lock timestamp")
	}
	if _, err = time.Parse(time.RFC3339Nano, stamp); err != nil {
		return fmt.Errorf("component lock timestamp")
	}
	return nil
}
func windPublicProfileClosure(before, after map[string]windProfileRecord, scope string, start, end time.Time) error {
	tmp := windPublicTmpRoot(before, after, scope, start, end)
	for key, old := range before {
		current, present := after[key]
		if present && old == current {
			continue
		}
		if key == "tmp\x00." && tmp {
			continue
		}
		if scope != "user" {
			return fmt.Errorf("project add changed original user profile")
		}
		switch key {
		case "home\x00.codeium/windsurf/mcp_config.json":
			if !present || current.Kind != "file" || current.Mode&07777 != 0600 {
				return fmt.Errorf("user add native file mode")
			}
		case "home\x00.codeium/windsurf":
			if !present || current.Kind != "directory" || old.Device != current.Device || old.Inode != current.Inode || old.Mode != current.Mode || old.UID != current.UID || old.GID != current.GID {
				return fmt.Errorf("user add native parent identity")
			}
		default:
			return fmt.Errorf("public add changed unrelated original profile")
		}
	}
	for key := range after {
		if _, ok := before[key]; !ok {
			return fmt.Errorf("public add added unrelated original profile entry")
		}
	}
	return nil
}

func windPublicTmpRoot(before, after map[string]windProfileRecord, scope string, start, end time.Time) bool {
	if scope != "user" && scope != "a" && scope != "b" {
		return false
	}
	for _, frame := range []map[string]windProfileRecord{before, after} {
		if _, present := frame["tmp\x00."]; !present {
			return false
		}
		for key := range frame {
			if strings.HasPrefix(key, "tmp\x00") && key != "tmp\x00." {
				return false
			}
		}
	}
	old, now := before["tmp\x00."], after["tmp\x00."]
	return old.Kind == "directory" && old.Mode&0170000 == 0040000 && old.Mode&07000 == 0 &&
		old.Inode > 0 && old.Links > 0 && old.UID == uint32(os.Getuid()) && old.GID == uint32(os.Getgid()) &&
		windRuntimeAuthority(old, now) && old.Size >= 0 && old.Size == now.Size &&
		old.SHA256 == "" && now.SHA256 == "" && windRuntimeCommandTimes(now, start, end)
}

func (w *windComponentFixture) gitFreeze(label string) error {
	r := w.recipes[strings.TrimPrefix(label, "git-")]
	if (label != "git-a" && label != "git-b") || r == nil {
		return fmt.Errorf("selected initial Git setup required")
	}
	path := filepath.Join(r.Project, ".git")
	if _, frozen := w.trees[path]; frozen {
		return fmt.Errorf("Git authority already frozen")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("ordinary Git directory required")
	}
	tree, err := windOwnedTree(w.f.ctx, path)
	if err != nil {
		return err
	}
	after, err := windOwnedTree(w.f.ctx, r.Project)
	if err != nil || windCWDMutation(w.trees[r.Project], after, "git") != nil {
		return fmt.Errorf("Git setup escaped exact .git closure")
	}
	record, err := windGitExcludePrepare(filepath.Join(path, "info"), tree["info/exclude"])
	if err != nil {
		return err
	}
	tree["info/exclude"] = record
	after[".git/info/exclude"] = record
	if err = windFrozenTrees(w.f.ctx, map[string]map[string]windProfileRecord{path: tree, r.Project: after}); err != nil {
		return err
	}
	w.trees[path] = tree
	w.trees[r.Project] = after
	return nil
}
func windNativeProfileClosure(before, after map[string]windProfileRecord, scope string, skill []byte) error {
	for key, old := range before {
		now, ok := after[key]
		if !ok {
			return fmt.Errorf("native add removed prior profile entry")
		}
		if old == now {
			continue
		}
		switch key {
		case ".", "skills":
			if old.Kind != "directory" || now.Kind != "directory" || old.Device != now.Device || old.Inode != now.Inode || old.Mode != now.Mode || old.UID != now.UID || old.GID != now.GID {
				return fmt.Errorf("native add replaced parent authority")
			}
		case "mcp_config.json":
			if now.Kind != "file" || now.Mode&07777 != 0600 {
				return fmt.Errorf("native add config mode")
			}
		default:
			return fmt.Errorf("native add changed foreign profile entry")
		}
	}
	for key, record := range after {
		if _, ok := before[key]; ok {
			continue
		}
		if scope == "user" {
			return fmt.Errorf("user skill activation fabricated")
		}
		switch key {
		case "skills/" + windComponentName:
			if record.Kind != "directory" || record.Mode&07777 != 0755 {
				return fmt.Errorf("project skill directory mode")
			}
		case "skills/" + windComponentName + "/SKILL.md":
			if record.Kind != "file" || record.Mode&07777 != 0644 || record.SHA256 != windsurfConfigDigest(skill) {
				return fmt.Errorf("project skill leaf differs")
			}
		default:
			return fmt.Errorf("native add unexpected profile entry")
		}
	}
	return nil
}

func (w *windComponentFixture) projectClosure(r *windComponentRecipe) error {
	after, err := windOwnedTree(w.f.ctx, r.Project)
	if err != nil {
		return err
	}
	operation := "user-public"
	if r.Scope != "user" {
		operation = "project-public"
	}
	return windCWDMutation(w.trees[r.Project], after, operation)
}
func windCWDMutation(before, after map[string]windProfileRecord, operation string) error {
	if before == nil || after == nil {
		return fmt.Errorf("complete cwd trees required")
	}
	if operation != "git" && operation != "project-public" && operation != "user-public" {
		return fmt.Errorf("closed cwd operation required")
	}
	if operation == "git" {
		_, existed := before[".git"]
		if existed || after[".git"].Kind != "directory" {
			return fmt.Errorf("fresh ordinary Git directory required")
		}
	}
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	for key := range keys {
		old, had := before[key]
		now, has := after[key]
		if had && has && old == now {
			continue
		}
		if operation == "git" && key == "." {
			if !had || !has || old.Kind != "directory" || now.Kind != "directory" || old.Device != now.Device || old.Inode != now.Inode || old.Mode != now.Mode || old.UID != now.UID || old.GID != now.GID {
				return fmt.Errorf("Git setup replaced cwd authority")
			}
			continue
		}
		if operation == "git" && (key == ".git" || strings.HasPrefix(key, ".git/")) {
			continue
		}
		if operation == "project-public" && (key == ".uap" || strings.HasPrefix(key, ".uap/") || key == ".devin" || strings.HasPrefix(key, ".devin/")) {
			continue
		}
		return fmt.Errorf("child mutation escaped exact selected cwd closure")
	}
	return nil
}
