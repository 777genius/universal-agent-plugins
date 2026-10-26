//go:build linux

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

func trustedSnapshot(paths ...string) (map[string]map[string]string, error) {
	result := map[string]map[string]string{}
	for _, path := range paths {
		value, err := trustedInventory(path)
		if err != nil {
			return nil, err
		}
		result[path] = value
	}
	return result, nil
}
func trustedUnchanged(before map[string]map[string]string) error {
	for path, want := range before {
		got, err := trustedInventory(path)
		if err != nil || !reflect.DeepEqual(got, want) {
			return fmt.Errorf("selected no-op/retained bytes changed: %s", path)
		}
	}
	return nil
}

func (c *controller) trustedSequence(_ context.Context) (resultErr error) {
	f := c.trustedState
	// The same exact original-user skill is native-discovered in both workspaces
	// before UAP installation. Trust is granted separately through native UI.
	for _, scope := range []string{"a", "b"} {
		if err := trustedSystem("/usr/bin/git"); err != nil {
			return err
		}
		body, err := c.trustedCommand("git-"+scope, f.Projects[scope], []string{"/usr/bin/git", "-c", "init.defaultBranch=main", "init", "--quiet"}, 0)
		if err != nil {
			return err
		}
		if len(body) != 0 {
			return fmt.Errorf("git baseline emitted unexpected diagnostics")
		}
		if err := c.trustedObserve("empty-"+scope, scope, "prompt", "read", false); err != nil {
			return err
		}
	}
	for _, scope := range []string{"user", "a", "b"} {
		if err := c.trustedUAP("add-"+scope, scope, true, false, "add", f.Packages[scope].Source, "--no-save"); err != nil {
			return err
		}
		if err := c.trustedReceipt(scope); err != nil {
			return err
		}
		if scope == "user" {
			if err := c.trustedPreserveUser(); err != nil {
				return err
			}
		} else if err := c.trustedProtect(f.Packages[scope].Binding.TargetLocator); err != nil {
			return err
		}
	}
	for _, scope := range []string{"a", "b"} {
		if err := c.trustedObserve("installed-"+scope, scope, "persisted", "write", true); err != nil {
			return err
		}
	}
	for _, path := range []string{filepath.Join(f.Projects["b"], ".uap"), filepath.Join(f.Projects["b"], ".vscode"), filepath.Join(f.Projects["b"], ".github")} {
		if err := c.trustedProtect(path); err != nil {
			return err
		}
	}
	a := f.Packages["a"]
	aState := filepath.Join(f.Projects["a"], ".uap", "state-v2.json")
	aConfig := filepath.Join(f.Projects["a"], ".vscode", "mcp.json")
	stable, err := trustedSnapshot(aState, aConfig, a.Skill, a.Binding.TargetLocator)
	if err != nil {
		return err
	}
	if err := c.trustedUAP("repeat-add-a", "a", false, false, "add", a.Source, "--no-save"); err != nil {
		return err
	}
	if err := trustedUnchanged(stable); err != nil {
		return err
	}
	if err := c.trustedUAP("unchanged-update-a", "a", false, false, "update", a.Name); err != nil {
		return err
	}
	if err := trustedUnchanged(stable); err != nil {
		return err
	}
	nativeStable, err := trustedSnapshot(aConfig, a.Skill)
	if err != nil {
		return err
	}
	if err := c.trustedUAP("intact-repair-a", "a", true, false, "repair", a.Name); err != nil {
		return err
	}
	if err := trustedUnchanged(nativeStable); err != nil {
		return err
	}
	if err := c.trustedReceipt("a"); err != nil {
		return err
	}
	if err := c.trustedObserve("stable-a", "a", "persisted", "read", true); err != nil {
		return err
	}
	// The only authored source replacement happens while every native child is
	// stopped. Missing repair below keeps this selected local v2 input unchanged;
	// moving-source/recorded-pin proof belongs to the retained TLS Git/ZIP gates.
	old := *f.Packages["a"]
	delete(f.Protected, old.Source)
	if err := c.trustedSource("a", "a-two"); err != nil {
		return err
	}
	// Only the reviewed public update may replace A's managed payload. B and
	// the actual original-user package remain frozen across this operation.
	delete(f.Protected, old.Binding.TargetLocator)
	if err := c.trustedUAP("update-a", "a", true, false, "update", f.Packages["a"].Source); err != nil {
		return err
	}
	if err := c.trustedReceipt("a"); err != nil {
		return err
	}
	if err := c.trustedProtect(f.Packages["a"].Binding.TargetLocator); err != nil {
		return err
	}
	a = f.Packages["a"]
	if a.Installation.Source.TreeDigest == old.Installation.Source.TreeDigest || a.Installation.InstallationID != old.Installation.InstallationID || a.Binding.ClientBindingID != old.Binding.ClientBindingID || a.Data != old.Data {
		return fmt.Errorf("changed source update lost recorded installation/data identity")
	}
	if err := c.trustedObserve("updated-a", "a", "persisted", "read", true); err != nil {
		return err
	}
	if a.Skill != filepath.Join(f.Projects["a"], ".github", "skills", a.Name, "SKILL.md") {
		return fmt.Errorf("missing-component operation outside receipt-owned skill")
	}
	if _, err := readOrdinary(a.Skill, 1<<20); err != nil {
		return err
	}
	if err := os.Remove(a.Skill); err != nil {
		return err
	}
	if err := os.Remove(filepath.Dir(a.Skill)); err != nil {
		return err
	}
	if err := c.trustedUAP("missing-repair-a", "a", true, false, "repair", a.Name); err != nil {
		return err
	}
	if err := c.trustedReceipt("a"); err != nil {
		return err
	}
	if f.Packages["a"].Data != old.Data {
		return fmt.Errorf("repair replaced retained data identity")
	}
	if err := c.trustedObserve("repaired-a", "a", "persisted", "read", true); err != nil {
		return err
	}
	if err := c.trustedObserve("restart-a", "a", "persisted", "read", true); err != nil {
		return err
	}
	retainedData, err := trustedSnapshot(a.Data)
	if err != nil {
		return err
	}
	beforeEvents, err := trustedEvents(filepath.Join(f.Root, "a-events.jsonl"))
	if err != nil {
		return err
	}
	delete(f.Protected, a.Binding.TargetLocator)
	if err := c.trustedUAP("remove-a", "a", true, false, "remove", a.Name); err != nil {
		return err
	}
	for _, path := range []string{filepath.Dir(a.Skill), a.Binding.TargetLocator} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("removed owned package/skill survives")
		}
	}
	if err := c.trustedObserve("removed-a", "a", "persisted", "read", false); err != nil {
		return err
	}
	if err := c.trustedObserve("remaining-b", "b", "persisted", "read", true); err != nil {
		return err
	}
	afterEvents, err := trustedEvents(filepath.Join(f.Root, "a-events.jsonl"))
	if err != nil || !bytes.Equal(afterEvents, beforeEvents) {
		return fmt.Errorf("removed A produced new native events")
	}
	if err := trustedUnchanged(retainedData); err != nil {
		return err
	}
	retainedA, err := trustedSnapshot(filepath.Join(f.Projects["a"], ".uap"), filepath.Join(f.Projects["a"], ".vscode"), filepath.Join(f.Projects["a"], ".github"))
	if err != nil {
		return err
	}
	if err := c.trustedUAP("repeat-remove-a", "a", false, true, "remove", a.Name); err != nil {
		return err
	}
	if err := trustedUnchanged(retainedA); err != nil {
		return err
	}
	if err := c.checkTrustedAssets(); err != nil {
		return err
	}
	for _, scope := range []string{"a", "b"} {
		body, err := trustedEvents(filepath.Join(f.Root, scope+"-events.jsonl"))
		if err != nil {
			return err
		}
		if err := c.keep("probe-"+scope+"-events.log", body); err != nil {
			return err
		}
	}
	f.Complete = true
	return nil
}

func (c *controller) trustedSummary(summary map[string]any, runErr, cleanupErr error) {
	complete := runErr == nil && cleanupErr == nil && c.trustedState != nil && c.trustedState.Complete
	stages := []string{"empty-a", "empty-b", "installed-a", "installed-b", "stable-a", "updated-a", "repaired-a", "restart-a", "removed-a", "remaining-b"}
	if c.trustedState == nil || !reflect.DeepEqual(c.trustedState.Stages, stages) {
		complete = false
	}
	required := map[string]bool{"Xvfb": true, "git-a": true, "git-b": true, "add-user": true, "add-a": true, "add-b": true, "repeat-add-a": true,
		"unchanged-update-a": true, "intact-repair-a": true, "update-a": true, "missing-repair-a": true, "remove-a": true, "repeat-remove-a": true}
	for _, stage := range stages {
		required[stage] = true
	}
	seen := map[string]bool{}
	for _, child := range c.children {
		if !required[child.Label] || seen[child.Label] || !child.TreeReaped || child.Error != "" || child.MergedOverflow {
			complete = false
		}
		seen[child.Label] = true
	}
	if len(seen) != len(required) {
		complete = false
	}
	qualification := map[string]bool{}
	for _, name := range []string{"project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call"} {
		qualification[name] = complete
	}
	summary["evidence_kind"] = "vscode-pinned-trusted-project-gateway-lifecycle"
	summary["mode"], summary["source_tree"] = trustedProjectMode, c.request.Input.SourceTree
	summary["collection_complete"], summary["qualification"] = complete, qualification
	summary["native_project_qualified"], summary["native_mcp_initialization_qualified"], summary["native_runtime_qualified"] = complete, complete, complete
	summary["activation"] = "prepared"
	summary["user_public_state"], summary["user_native_install_qualified"] = "prepared", false
	summary["vendor_authentication_qualified"], summary["real_model_qualified"] = false, false
	summary["boundary"] = "Pinned trusted development host/proposed API; original-user public Prepared package preserved separately from preexisting native user skill. Local unchanged-input repair; recorded-pin/source movement belongs to separate TLS gates. No model/OAuth/released-client claim."
	if c.trustedState != nil {
		summary["completed_stages"] = c.trustedState.Stages
	}
}
