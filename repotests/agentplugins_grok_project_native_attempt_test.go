//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func TestAgentpluginsGrokNativeProjectRuntimeAttempt(t *testing.T) {
	projectNativeCLIRuntimeAttempt(t, "grok")
}

// This reads candidate inventory only. A disabled Prepared entry is not skill
// loading, native execution, source revision or completed lifecycle evidence.
func projectGrokPreparedInspect(body []byte, project, name, active string) error {
	if !filepath.IsAbs(project) || filepath.Clean(project) != project || pathpolicy.ValidateLeafID(name) != nil || !filepath.IsAbs(active) || filepath.Clean(active) != active || filepath.Dir(active) != filepath.Join(project, ".grok", "plugins") || pathpolicy.ValidateLeafID(filepath.Base(active)) != nil {
		return fmt.Errorf("Grok inspection expectation is outside the selected project")
	}
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return err
	}
	object, ok := value.(map[string]any)
	if !ok || !projectGrokIdentityKeys(object, "grokVersion cwd projectRoot projectTrusted skills plugins") || object["grokVersion"] != "1.0.70" || object["cwd"] != project || (object["projectRoot"] != project && object["projectRoot"] != project+string(filepath.Separator)) || object["projectTrusted"] != false {
		return fmt.Errorf("measured Grok untrusted project identity differs")
	}
	if _, ok := object["skills"].([]any); !ok {
		return fmt.Errorf("Grok skill inventory is not an array")
	}
	plugins, ok := object["plugins"].([]any)
	if !ok {
		return fmt.Errorf("Grok plugin inventory is not an array")
	}
	seen, selected := map[string]bool{}, false
	for _, value := range plugins {
		entry, ok := value.(map[string]any)
		if !ok || !projectGrokIdentityKeys(entry, "name scope path enabled") {
			return fmt.Errorf("Grok plugin identity object differs")
		}
		entryName, n := entry["name"].(string)
		scope, s := entry["scope"].(string)
		path, p := entry["path"].(string)
		enabled, e := entry["enabled"].(bool)
		folded := shared.FoldJSONKey(entryName)
		if !n || strings.TrimSpace(entryName) == "" || seen[folded] || !s || (scope != "project" && scope != "user") || !p || !filepath.IsAbs(path) || filepath.Clean(path) != path || !e {
			return fmt.Errorf("ambiguous or noncanonical Grok plugin identity")
		}
		seen[folded] = true
		if folded == shared.FoldJSONKey(name) {
			if entryName != name || scope != "project" || path != active || enabled {
				return fmt.Errorf("selected Grok disabled Prepared candidate differs")
			}
			selected = true
		}
	}
	if !selected {
		return fmt.Errorf("selected Grok Prepared candidate is absent")
	}
	return nil
}

func projectGrokIdentityKeys(object map[string]any, identity string) bool {
	for key := range object {
		for _, expected := range strings.Fields(identity) {
			if key != expected && shared.FoldJSONKey(key) == shared.FoldJSONKey(expected) {
				return false
			}
		}
	}
	return true
}

// Freeze only the finite selected ordinary paths. No returned native pathname
// can select a file; system ancestors retain their existing no-alias checks.
func projectGrokPreparedAuthority(f *projectNativeCLIAttempt) (func() error, error) {
	project, active := f.base.Project, f.binding.TargetLocator
	profile := filepath.Join(project, ".grok")
	if f.spec.client != "grok" || f.binding.Scope != "project" || f.binding.NativeProfileRoot != profile || pathpolicy.ValidateLeafID(f.name) != nil || pathpolicy.ValidateLeafID(f.binding.PhysicalArtifact) != nil || !filepath.IsAbs(active) || filepath.Clean(active) != active || filepath.Dir(active) != filepath.Join(profile, "plugins") || filepath.Base(active) != f.binding.PhysicalArtifact || f.skill != filepath.Join(active, "skills", f.name, "SKILL.md") {
		return nil, fmt.Errorf("Grok selected Prepared package authority differs")
	}
	directories := []string{project, profile, filepath.Dir(active), active, filepath.Join(active, ".grok-plugin"), filepath.Join(active, "skills"), filepath.Dir(f.skill), f.base.Package, filepath.Join(f.base.Package, "skills"), filepath.Join(f.base.Package, "skills", f.name)}
	dirs := map[string]os.FileInfo{}
	for _, path := range directories {
		if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || stat.Uid != uint32(os.Getuid()) {
			return nil, fmt.Errorf("Grok selected directory must be ordinary and originally owned")
		}
		dirs[path] = info
	}
	skill := []byte("---\nname: " + f.name + "\ndescription: Harmless installed startup marker\n---\nUAP_NATIVE_INSTALLED_" + f.base.Nonce + "\n")
	sourceSkill := filepath.Join(f.base.Package, "skills", f.name, "SKILL.md")
	manifest := filepath.Join(active, ".grok-plugin", "plugin.json")
	files := map[string][]byte{sourceSkill: skill, f.skill: skill, manifest: nil}
	identities := map[string]os.FileInfo{}
	for path, expected := range files {
		before, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		body, err := projectNativeRuntimeRead(path, 1<<20)
		if err != nil {
			return nil, err
		}
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before, after) || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) {
			return nil, fmt.Errorf("Grok selected held-file identity changed")
		}
		if path == manifest {
			value, err := projectNativeRuntimeJSON(body)
			if err != nil || !reflect.DeepEqual(value, map[string]any{"name": f.name, "version": "1.0.0", "description": "Disposable native project startup observation", "skills": "./skills/"}) {
				return nil, fmt.Errorf("Grok selected native manifest differs from authored projection")
			}
		} else if !bytes.Equal(body, expected) {
			return nil, fmt.Errorf("Grok source or delivered skill differs from authored recipe")
		}
		files[path], identities[path] = bytes.Clone(body), before
	}
	return func() error {
		for path, before := range dirs {
			if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
				return err
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Sys().(*syscall.Stat_t).Uid != after.Sys().(*syscall.Stat_t).Uid || before.Sys().(*syscall.Stat_t).Gid != after.Sys().(*syscall.Stat_t).Gid {
				return fmt.Errorf("Grok selected directory authority changed")
			}
		}
		for path, expected := range files {
			body, err := projectNativeRuntimeRead(path, 1<<20)
			after, statErr := os.Lstat(path)
			if err != nil || statErr != nil || !bytes.Equal(body, expected) || !os.SameFile(identities[path], after) || !projectNativeRuntimeStableStat(identities[path].Sys(), after.Sys()) {
				return fmt.Errorf("Grok selected held-file authority changed")
			}
		}
		return nil
	}, nil
}

func (f *projectNativeCLIAttempt) inspectGrokPrepared(t *testing.T) {
	t.Helper()
	verify, err := projectGrokPreparedAuthority(f)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := verify(); err != nil {
			t.Error(err)
		}
	}()
	f.evidence["attempt_stage"] = "native-installed-inspection"
	r := f.command(t, "native-inspect", f.base.Project, []string{f.client, "inspect", "--json"}, 30*time.Second)
	if err := verify(); err != nil {
		t.Fatal(err)
	}
	if len(r.Stderr) != 0 {
		t.Fatal("Grok installed inspection emitted diagnostics")
	}
	if err := projectGrokPreparedInspect(r.Stdout, f.base.Project, f.name, f.binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	f.evidence["native_inspection"] = map[string]any{"name": f.name, "scope": "project", "path": f.binding.TargetLocator, "enabled": false, "project_trusted": false, "interpretation": "visible disabled Prepared candidate only; no skill loading or lifecycle/runtime qualification"}
}
