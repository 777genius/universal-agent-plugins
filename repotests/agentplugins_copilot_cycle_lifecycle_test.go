//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *copilotProjectCycle) apply(t *testing.T, label, key string, mutated bool) {
	t.Helper()
	r := c.recipes[key]
	c.check(t)
	stable := map[string]string{}
	if !mutated {
		for _, path := range []string{c.statePath(r), r.SkillPath, r.Config} {
			if path == "" {
				continue
			}
			body, err := projectNativeRuntimeRead(path, 1<<20)
			if os.IsNotExist(err) && r.Removed {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			stable[path] = copilotProjectConfigHash(body)
		}
	}
	c.suspend(r)
	if label == "remove-user" {
		delete(c.absent, copilotCycleDataPath(c, r))
	}
	result := c.publicCommand(t, label, key)
	for path, want := range stable {
		body, err := projectNativeRuntimeRead(path, 1<<20)
		if err != nil || copilotProjectConfigHash(body) != want {
			t.Fatal("Copilot repeat/no-op changed selected bytes", err)
		}
	}
	if !strings.HasPrefix(label, "repeat-remove-") {
		verb := strings.Split(label, "-")[0]
		switch label {
		case "repeat-add-a":
			verb = "add"
		case "unchanged-update-a":
			verb = "update"
		case "intact-repair-a", "missing-repair-a":
			verb = "repair"
		}
		if err := copilotCyclePublicJSON(result.Stdout, verb, mutated); err != nil {
			t.Fatal(err)
		}
		if string(result.Stderr) != copilotCycleProgress(label) {
			t.Fatal("Copilot public progress differs")
		}
	}
	if strings.HasPrefix(label, "remove-") {
		r.Removed = true
	}
	c.proof(t, r, label)
	c.registryList(t, label)
}

func copilotCyclePublicJSON(raw []byte, verb string, mutated bool) error {
	if err := clineCyclePublicJSON(raw, mutated); err != nil {
		return err
	}
	value, err := projectNativeRuntimeJSON(raw)
	row, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(row, "schema_version command result data", "") != nil || row["schema_version"] != json.Number("1") || row["command"] != verb || row["result"] != "success" {
		return fmt.Errorf("Copilot exact public envelope differs")
	}
	return nil
}

func copilotCycleProgress(label string) string {
	switch label {
	case "add-user", "add-a", "add-b":
		return "Resolving and validating one Agent Plugin package for every selected target...\nInstalling native-proof for 1 selected clients...\n"
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

func (c *copilotProjectCycle) reviseA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.proof(t, r, "before-source-revision-a")
	c.check(t)
	delete(c.trees, r.Source)
	r.Revision, r.Version = "a-two", "1.1.0"
	c.author(t, r, true)
	c.check(t)
}

func (c *copilotProjectCycle) missingA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.proof(t, r, "before-missing-a")
	c.check(t)
	owned := filepath.Dir(r.SkillPath)
	body := r.Authored["skills/"+r.Skill+"/SKILL.md"]
	digest := "sha256:" + copilotProjectConfigHash(append(append([]byte("f\x00SKILL.md\x00-\x00"), body...), 0))
	want := domain.NativeObjectOwnership{ObjectID: "copilot_project_skill_directory:" + r.Skill, Kind: "copilot_project_skill_directory", LogicalName: r.Skill, Path: owned, SourceRelative: "skills/" + r.Skill, ManagedDigest: digest, ProtectionClass: "managed"}
	count := 0
	for _, object := range r.Binding.NativeObjects {
		if object.Kind == want.Kind {
			if !reflect.DeepEqual(object, want) {
				t.Fatal("Copilot fault differs from independently owned skill")
			}
			count++
		}
	}
	if r.Key != "a" || r.Revision != "a-two" || count != 1 {
		t.Fatal("Copilot fault requires sole frozen A-two skill")
	}
	tree, err := clineAttemptTree(owned)
	if err != nil || !reflect.DeepEqual(tree, map[string]string{".": fmt.Sprintf("directory:%o", os.ModeDir|0755), "SKILL.md": "file:644:" + copilotProjectConfigHash(body)}) {
		t.Fatal("Copilot fault target has unexpected membership", err)
	}
	identity, err := os.Lstat(owned)
	if err != nil || !identity.IsDir() {
		t.Fatal("Copilot fault directory is not ordinary")
	}
	delete(c.f.protected, r.SkillPath)
	delete(c.trees, filepath.Dir(owned))
	if err = os.Remove(r.SkillPath); err != nil {
		t.Fatal(err)
	}
	current, err := os.Lstat(owned)
	if err != nil || !os.SameFile(identity, current) {
		t.Fatal("Copilot held fault directory changed")
	}
	if err = os.Remove(owned); err != nil {
		t.Fatal(err)
	}
	if copilotCycleAbsent(owned) != nil {
		t.Fatal("Copilot exact missing fault did not remove owned skill")
	}
	c.append(t, "cycle-proof.jsonl", "missing-a", map[string]any{"path": owned, "source_digest": r.Digest, "object": want, "before": tree, "absent": true})
	c.check(t)
}

func TestAgentpluginsCopilotNativeProjectCompleteCycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_COPILOT_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine hosted complete Copilot cycle")
	}
	f := newCopilotProjectAttempt(t)
	c := newCopilotProjectCycle(f)
	defer c.finish(t)
	c.prepare(t)
	for _, key := range []string{"a", "b"} {
		result := c.publicCommand(t, "git-"+key, key)
		if len(result.Stdout) != 0 || len(result.Stderr) != 0 {
			t.Fatal("Copilot Git setup diagnostics")
		}
	}
	version := c.publicCommand(t, "version", "user")
	if string(version.Stdout) != "GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n" || len(version.Stderr) != 0 {
		t.Fatal("Copilot exact native version")
	}
	c.registryList(t, "empty-user")
	c.apply(t, "add-user", "user", true)
	c.native(t, "installed-user", "user", "")
	c.native(t, "empty-a", "a", "")
	c.native(t, "empty-b", "b", "")
	c.apply(t, "add-a", "a", true)
	c.native(t, "installed-a", "a", "write")
	c.apply(t, "add-b", "b", true)
	c.native(t, "installed-b", "b", "write")
	c.apply(t, "repeat-add-a", "a", false)
	c.apply(t, "unchanged-update-a", "a", false)
	c.apply(t, "intact-repair-a", "a", true)
	c.native(t, "stable-a", "a", "read")
	c.reviseA(t)
	c.apply(t, "update-a", "a", true)
	c.native(t, "updated-a", "a", "read")
	c.missingA(t)
	c.apply(t, "missing-repair-a", "a", true)
	c.native(t, "repaired-a", "a", "read")
	c.apply(t, "remove-a", "a", true)
	c.native(t, "removed-a", "a", "")
	c.native(t, "remaining-b", "b", "read")
	c.apply(t, "repeat-remove-a", "a", false)
	c.apply(t, "remove-b", "b", true)
	c.native(t, "removed-b", "b", "")
	c.apply(t, "repeat-remove-b", "b", false)
	c.native(t, "remaining-user", "user", "")
	c.apply(t, "remove-user", "user", true)
	c.apply(t, "repeat-remove-user", "user", false)
	c.native(t, "final-a", "a", "")
	c.native(t, "final-b", "b", "")
	if !reflect.DeepEqual(c.public, copilotCyclePublic) || !reflect.DeepEqual(c.registry, copilotCycleRegistry) || !reflect.DeepEqual(c.sessions, copilotCycleNative) || len(c.nativeIDs) != 14 || len(c.probeIDs) != 6 {
		t.Fatal("Copilot complete stage/identity closure")
	}
	c.check(t)
	c.complete = true
	f.evidence["cycle_stage"] = "complete-cycle"
}
