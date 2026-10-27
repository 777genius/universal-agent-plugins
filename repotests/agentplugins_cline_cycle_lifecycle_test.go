//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func (c *clineProjectCycle) apply(t *testing.T, label, scope string, mutated bool) {
	t.Helper()
	r := c.recipes[scope]
	c.f.check(t)
	stable := map[string]map[string]string{}
	if label == "repeat-add-a" || label == "unchanged-update-a" || label == "repeat-remove-a" {
		for _, path := range []string{c.statePath(r), r.Config, r.SkillPath} {
			body, err := projectNativeRuntimeRead(path, 1<<20)
			if os.IsNotExist(err) && r.Removed {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			stable[path] = map[string]string{"sha256": clineAttemptSHA(body)}
		}
	}
	// Suspend only these independently selected receipt paths, for one public
	// operation. Foreign and sibling paths and source recipes stay frozen.
	for _, path := range []string{c.statePath(r), r.SkillPath} {
		delete(c.f.protected, path)
	}
	if r.Scope != "user" {
		delete(c.f.protected, r.Config)
	}
	if r.Active != "" {
		delete(c.f.trees, r.Active)
	}
	delete(c.f.trees, filepath.Dir(filepath.Dir(r.SkillPath)))
	before := r.Installation
	if label == "remove-user" {
		path, err := c.userUnallocatedData(r, r.Installation, r.Binding)
		if err != nil {
			t.Fatal(err)
		}
		// Default final-binding removal alone may establish this exact owned
		// allocation. The resulting receipt/marker/tree is independently checked.
		delete(c.f.absent, path)
	}
	result := c.command(t, label, scope)
	for path, want := range stable {
		body, err := projectNativeRuntimeRead(path, 1<<20)
		if err != nil || clineAttemptSHA(body) != want["sha256"] {
			t.Fatal("Cline unchanged/repeat removal changed exact selected bytes")
		}
	}
	if label == "repeat-remove-a" {
		raw, err := projectNativeRuntimeRead(c.statePath(r), 1<<20)
		state, decodeErr := clineCycleState(raw)
		if err != nil || decodeErr != nil || len(state.Installations) != 1 || !reflect.DeepEqual(before, state.Installations[0]) {
			t.Fatal("Cline repeat removal changed retained state")
		}
	} else {
		if err := clineCyclePublicJSON(result.Stdout, mutated); err != nil {
			t.Fatal(err)
		}
		if string(result.Stderr) != clineCycleProgress(label) {
			t.Fatal("Cline exact public progress differs")
		}
	}
	if strings.HasPrefix(label, "remove-") {
		r.Removed = true
	}
	c.proof(t, r, label)
}
func clineCycleProgress(label string) string {
	switch label {
	case "add-user", "add-a", "add-b":
		return "Resolving and validating one Agent Plugin package for every selected target...\nInstalling " + clineCycleName + " for 1 selected clients...\n"
	case "repeat-add-a":
		return "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
	case "unchanged-update-a", "update-a":
		return "Resolving and validating one updated Agent Plugin package for every selected target...\n"
	case "intact-repair-a", "missing-repair-a":
		return "Resolving and validating each unique exact installed package revision once...\n"
	default:
		return ""
	}
}
func clineCyclePublicJSON(raw []byte, mutated bool) error {
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		return err
	}
	if err := nativeValidateInstallerJSON(raw); err != nil {
		return err
	}
	count := 0
	var visit func(any) error
	visit = func(value any) error {
		switch node := value.(type) {
		case map[string]any:
			for key, item := range node {
				if key == "mutated" {
					count++
					if item != mutated {
						return fmt.Errorf("Cline public mutation truth differs")
					}
				}
				if key == "no_change" && item == true && mutated {
					return fmt.Errorf("Cline public mutation reported no_change")
				}
				if err := visit(item); err != nil {
					return err
				}
			}
		case []any:
			for _, item := range node {
				if err := visit(item); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(value); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("Cline public mutation receipt absent")
	}
	return nil
}
func (c *clineProjectCycle) reviseA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.f.check(t)
	c.proof(t, r, "before-source-revision-a")
	delete(c.f.trees, r.Source)
	r.Revision = "a-two"
	c.author(t, r, true)
	c.f.check(t)
}
func (c *clineProjectCycle) missingA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.proof(t, r, "before-missing-a")
	c.f.check(t)
	// The held ordinary single-link source-bound leaf is the sole injected fault.
	if _, err := projectNativeRuntimeRead(r.SkillPath, 1<<20); err != nil {
		t.Fatal(err)
	}
	delete(c.f.protected, r.SkillPath)
	delete(c.f.trees, filepath.Dir(filepath.Dir(r.SkillPath)))
	if err := os.Remove(r.SkillPath); err != nil {
		t.Fatal(err)
	}
}
func TestAgentpluginsClineNativeProjectCompleteCycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLINE_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine hosted complete Cline cycle capture")
	}
	f := newClineProjectAttempt(t)
	// Own finalization before source authoring, listener setup or any child.
	var c *clineProjectCycle
	defer func() {
		if c == nil {
			f.finish(t)
		} else {
			c.finish(t)
		}
	}()
	c = newClineProjectCycle(t, f)
	c.prepare(t)
	c.startProvider(t)
	for _, scope := range []string{"a", "b"} {
		result := c.command(t, "git-"+scope, scope)
		if len(result.Stdout) != 0 || len(result.Stderr) != 0 {
			t.Fatal("Cline cycle Git setup diagnostics")
		}
	}
	version := c.command(t, "version", "user")
	if string(version.Stdout) != "3.0.73\n" || len(version.Stderr) != 0 {
		t.Fatal("Cline cycle exact native version")
	}
	for _, scope := range []string{"a", "b"} {
		c.listing(t, "empty-"+scope, scope, nil)
		c.native(t, "empty-"+scope, scope)
	}
	for _, scope := range []string{"user", "a", "b"} {
		c.apply(t, "add-"+scope, scope, true)
	}
	c.listing(t, "installed-user", "user", []string{"user"})
	for _, scope := range []string{"a", "b"} {
		c.listing(t, "installed-"+scope, scope, []string{"user", scope})
		c.native(t, "installed-"+scope, scope)
	}
	c.apply(t, "repeat-add-a", "a", false)
	c.apply(t, "unchanged-update-a", "a", false)
	c.apply(t, "intact-repair-a", "a", true)
	c.listing(t, "stable-a", "a", []string{"user", "a"})
	c.reviseA(t)
	c.apply(t, "update-a", "a", true)
	c.listing(t, "updated-a", "a", []string{"user", "a"})
	c.native(t, "updated-a", "a")
	c.missingA(t)
	c.apply(t, "missing-repair-a", "a", true)
	c.listing(t, "repaired-a", "a", []string{"user", "a"})
	c.native(t, "repaired-a", "a")
	c.apply(t, "remove-a", "a", true)
	c.listing(t, "removed-a", "a", []string{"user"})
	c.native(t, "removed-a", "a")
	c.listing(t, "remaining-b", "b", []string{"user", "b"})
	c.native(t, "remaining-b", "b")
	c.apply(t, "repeat-remove-a", "a", false)
	c.apply(t, "remove-b", "b", true)
	c.listing(t, "removed-b", "b", []string{"user"})
	c.apply(t, "remove-user", "user", true)
	c.listing(t, "removed-user", "user", nil)
	for _, scope := range []string{"a", "b"} {
		c.listing(t, "final-"+scope, scope, nil)
		c.native(t, "final-"+scope, scope)
	}
	if len(f.used) != len(clineCyclePublic) || !reflect.DeepEqual(c.listings, clineCycleListings) || !reflect.DeepEqual(c.sessions, clineCycleACP) {
		t.Fatal("Cline complete cycle stage closure")
	}
	f.evidence["cycle_observed_complete"] = true
	f.evidence["attempt_stage"] = "complete-cycle-capture-review"
	// First empty mode is intentionally unqualified until genuine raw review.
	t.Fatal("complete Cline cycle captured; fresh empty grammar and complete authority cycle require independent raw review before a separate completion amendment")
}
