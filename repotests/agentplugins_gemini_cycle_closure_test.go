//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func geminiCycleKeys(before, after map[string]geminiCycleRecord) map[string]bool {
	keys := map[string]bool{}
	for name := range before {
		keys[name] = true
	}
	for name := range after {
		keys[name] = true
	}
	return keys
}
func geminiCycleParent(old, now geminiCycleRecord, had, has bool) bool {
	if !has || now.Mode != 0040000|0700 {
		return false
	}
	return !had || geminiCycleSameDirectory(old, now)
}
func geminiCycleCWDClosure(before, after map[string]geminiCycleRecord, project bool) error {
	if before == nil || after == nil {
		return fmt.Errorf("Gemini complete cwd maps required")
	}
	for name := range geminiCycleKeys(before, after) {
		a, had := before[name]
		b, has := after[name]
		if had && has && a == b {
			continue
		}
		if project && (name == ".uap" || strings.HasPrefix(name, ".uap/") || name == ".gemini" || strings.HasPrefix(name, ".gemini/")) {
			continue
		}
		return fmt.Errorf("Gemini selected public mutation escaped cwd closure")
	}
	return nil
}
func geminiCycleStateClosure(before, after map[string]geminiCycleRecord, r *geminiCycleRecipe) error {
	if before == nil || after == nil {
		return fmt.Errorf("Gemini complete state maps required")
	}
	active, _ := filepath.Rel(r.StateRoot, r.Active)
	data, _ := filepath.Rel(r.StateRoot, r.Data)
	parents := map[string]bool{".": true, "operations-v2": true}
	for _, name := range []string{active, data} {
		for parent := filepath.Dir(name); parent != "."; parent = filepath.Dir(parent) {
			parents[parent] = true
		}
	}
	for name := range geminiCycleKeys(before, after) {
		a, had := before[name]
		b, has := after[name]
		if had && has && a == b {
			continue
		}
		switch {
		case name == "state-v2.json":
			if !has || b.Mode != 0100000|0600 {
				return fmt.Errorf("Gemini state leaf mode")
			}
		case name == "mutation.lock":
			if !has || b.Mode != 0100000|0600 || geminiCycleMutationLock(filepath.Join(r.StateRoot, name)) != nil {
				return fmt.Errorf("Gemini exact public mutation lock")
			}
		case parents[name]:
			if !geminiCycleParent(a, b, had, has) {
				return fmt.Errorf("Gemini selected state parent identity")
			}
		case name == active || strings.HasPrefix(name, active+"/"):
			if r.Removed && has {
				return fmt.Errorf("Gemini removed payload retained child")
			}
		case name == data || strings.HasPrefix(name, data+"/"):
			if had {
				return fmt.Errorf("Gemini public command changed retained owned data")
			}
			if name == data && has && b.Mode == 0040000|0700 {
				continue
			}
			if name == data+"/.agentplugins-data-owner.json" && has && b.Mode == 0100000|0600 {
				continue
			}
			return fmt.Errorf("Gemini unproved new data child")
		default:
			return fmt.Errorf("Gemini public mutation escaped exact state closure")
		}
	}
	return nil
}
func geminiCycleMutationLock(path string) error {
	raw, err := projectNativeRuntimeRead(path, 4096)
	if err != nil {
		return err
	}
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "pid acquired_at", "") != nil {
		return fmt.Errorf("Gemini mutation lock object")
	}
	pid, ok := row["pid"].(json.Number)
	if !ok {
		return fmt.Errorf("Gemini mutation lock pid")
	}
	number, err := pid.Int64()
	if err != nil || number <= 0 {
		return fmt.Errorf("Gemini mutation lock pid")
	}
	stamp, ok := row["acquired_at"].(string)
	if !ok {
		return fmt.Errorf("Gemini mutation lock stamp")
	}
	_, err = time.Parse(time.RFC3339Nano, stamp)
	return err
}
func geminiCycleProfileClosure(before, after map[string]geminiCycleRecord, r *geminiCycleRecipe) error {
	owned := "skills/" + r.Name
	for name := range geminiCycleKeys(before, after) {
		a, had := before[name]
		b, has := after[name]
		if had && has && a == b {
			continue
		}
		switch {
		case name == "." || name == "skills":
			if !had || !has || !geminiCycleSameDirectory(a, b) {
				return fmt.Errorf("Gemini native profile parent changed")
			}
		case name == "settings.json":
			if !has || b.Mode != 0100000|0600 {
				return fmt.Errorf("Gemini native settings mode")
			}
		case name == "settings.json.agentplugins.lock":
			if had || !has || b.Mode != 0100000|0600 || b.Size != 0 || b.SHA256 != geminiCycleSHA(nil) {
				return fmt.Errorf("Gemini native writer lock changed")
			}
		case name == owned || strings.HasPrefix(name, owned+"/"):
			if r.Removed && has {
				return fmt.Errorf("Gemini removed owned skill remains")
			}
		default:
			return fmt.Errorf("Gemini public command changed foreign original profile member")
		}
	}
	return nil
}
