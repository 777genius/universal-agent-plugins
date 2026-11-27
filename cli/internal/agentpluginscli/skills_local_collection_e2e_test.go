package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func writeCollectionSkill(t *testing.T, root, folder, name string) {
	t.Helper()
	directory := filepath.Join(root, folder)
	if err := os.MkdirAll(filepath.Join(directory, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: Review local collection content\n---\nRead scripts/check.sh.\n"
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "scripts", "check.sh"), []byte("#!/bin/sh\nexit 97\n"), 0700); err != nil {
		t.Fatal(err)
	}
}

func localCollectionJourney(t *testing.T) (savedJourney, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeCollectionSkill(t, root, "review", "review")
	writeCollectionSkill(t, root, "testing", "testing")
	return newSavedJourney(t, t.TempDir()), root
}

func collectionBinding(t *testing.T, j savedJourney, name, client string) (domain.ClientBinding, string) {
	t.Helper()
	for _, installation := range j.state(t).Installations {
		for _, binding := range installation.Clients {
			if binding.ClientID != client {
				continue
			}
			root := binding.TargetLocator
			if client == "claude" {
				root = filepath.Join(root, ".agentplugins-runtime")
			}
			document := filepath.Join(root, "skills", name, "SKILL.md")
			if _, err := os.Stat(document); err == nil {
				return binding, document
			}
		}
	}
	t.Fatalf("missing %s Skill in %s", name, client)
	return domain.ClientBinding{}, ""
}

func collectionDurableTree(t *testing.T, j savedJourney) map[string]savedJourneyFile {
	t.Helper()
	files := savedJourneyTree(t, j.cli.root)
	lease, err := filepath.Rel(j.cli.root, j.cli.app.EnvironmentLockPath)
	if err != nil {
		t.Fatal(err)
	}
	// Only the known advisory lease may change between identical operations.
	delete(files, filepath.ToSlash(lease))
	return files
}

func TestLocalCollectionListAndDryRunAreReadOnly(t *testing.T) {
	j, root := localCollectionJourney(t)
	before, source := savedJourneyTree(t, j.cli.root), savedJourneyTree(t, root)
	first := j.run(t, "skills", "list", root, "--format", "json")
	if next := j.run(t, "skills", "list", root, "--format", "json"); next != first {
		t.Fatal("local listing is nondeterministic")
	}
	var report struct {
		Data skillsListReport `json:"data"`
	}
	if err := json.Unmarshal([]byte(first), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Data.Complete || report.Data.Revision != "" || len(report.Data.Candidates) != 2 || report.Data.Candidates[0].Name != "review" || report.Data.Candidates[1].Name != "testing" {
		t.Fatal("unexpected local catalog", first)
	}
	j.run(t, "skills", "add", root, "--all", "--target", "codex,claude", "--dry-run", "--format", "json")
	if !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.root)) || !reflect.DeepEqual(source, savedJourneyTree(t, root)) || j.runner.calls != 0 {
		t.Fatal("listing/dry-run changed durable state, source, or invoked native mutation")
	}
}

func TestLocalCollectionCodexClaudeLifecycle(t *testing.T) {
	j, root := localCollectionJourney(t)
	args := []string{"skills", "add", root, "--all", "--target", "codex,claude", "--yes", "--format", "json"}
	j.run(t, args...)
	requirements, locked := j.intent(t)
	if len(requirements.Dependencies) != 2 || len(locked.Packages) != 2 || len(j.state(t).EnvironmentReceipts) != 4 {
		t.Fatal("collection did not save two Skills for two clients", requirements, locked)
	}
	foreign := map[string]string{}
	for _, name := range []string{"review", "testing"} {
		for _, target := range []string{"codex", "claude"} {
			binding, document := collectionBinding(t, j, name, target)
			body, err := os.ReadFile(document)
			if err != nil || !strings.Contains(string(body), "name: "+name) {
				t.Fatal("wrong installed Skill", name, target, err)
			}
			marker := filepath.Join(filepath.Dir(binding.TargetLocator), "foreign-user-marker")
			if err := os.WriteFile(marker, []byte("preserve user bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			foreign[marker] = "preserve user bytes"
		}
	}
	before := collectionDurableTree(t, j)
	j.run(t, args...)
	if after := collectionDurableTree(t, j); !reflect.DeepEqual(before, after) {
		t.Fatal("unchanged collection re-add changed durable files: " + savedJourneyDifferences(before, after))
	}
	writeCollectionSkill(t, root, "later", "later")
	if err := os.WriteFile(filepath.Join(root, "review", "scripts", "check.sh"), []byte("#!/bin/sh\nexit 98\n"), 0700); err != nil {
		t.Fatal(err)
	}
	j.run(t, "update", "--all", "--yes", "--format", "json")
	requirements, updated := j.intent(t)
	if len(requirements.Dependencies) != 2 || len(updated.Packages) != 2 {
		t.Fatal("update enrolled an unselected sibling")
	}
	for _, old := range locked.Packages {
		for _, next := range updated.Packages {
			if next.Alias == old.Alias && (environment.SamePin(old, next) != (old.Alias == "testing")) {
				t.Fatal("update changed the wrong Skill pin", old.Alias)
			}
		}
	}
	binding, _ := collectionBinding(t, j, "review", "codex")
	savedRawSkillDropOwnedFixture(t, j, binding.TargetLocator)
	j.run(t, "repair", "review", "--target", "codex", "--format", "json")
	collectionBinding(t, j, "review", "codex")
	if err := os.RemoveAll(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	j.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	j.run(t, "remove", "review", "--target", "codex", "--format", "json")
	requirements, _ = j.intent(t)
	if slices.Contains(requirements.Targets["codex"], "review") || !slices.Contains(requirements.Targets["claude"], "review") {
		t.Fatal("selective removal changed the other target")
	}
	collectionBinding(t, j, "review", "claude")
	j.run(t, "remove", "review", "--target", "claude", "--format", "json")
	j.run(t, "remove", "testing", "--target", "codex,claude", "--format", "json")
	for marker, want := range foreign {
		if body, err := os.ReadFile(marker); err != nil || string(body) != want {
			t.Fatal("lifecycle changed foreign native bytes", marker, err)
		}
	}
}

func TestLocalCollectionRefusalsAndExactSelection(t *testing.T) {
	for _, scenario := range []string{"implicit-all", "invalid-all", "ambiguous-all", "invalid-root"} {
		t.Run(scenario, func(t *testing.T) {
			j, root := localCollectionJourney(t)
			flags := []string{"--all"}
			switch scenario {
			case "implicit-all":
				flags = nil
			case "invalid-all":
				writeCollectionSkill(t, root, "invalid", "wrong-name")
			case "ambiguous-all":
				writeCollectionSkill(t, root, "nested/review", "review")
			case "invalid-root":
				if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("invalid root"), 0600); err != nil {
					t.Fatal(err)
				}
				flags = nil
			}
			args := append([]string{"skills", "add", root, "--target", "codex,claude", "--yes", "--format", "json"}, flags...)
			if out, _, err := j.cli.execute(false, args...); err == nil {
				t.Fatal("invalid collection accepted", scenario, out)
			}
			manifest, lock := j.cli.app.dependencyPaths()
			for _, p := range []string{manifest, lock} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatal("refusal published intent", p, err)
				}
			}
			if len(j.state(t).Installations) != 0 || j.runner.calls != 0 {
				t.Fatal("refusal installed a partial collection")
			}
		})
	}
	j, root := localCollectionJourney(t)
	writeCollectionSkill(t, root, "invalid", "wrong-name")
	j.run(t, "skills", "add", root, "--skill", "review,testing", "--target", "codex,claude", "--yes", "--format", "json")
	requirements, _ := j.intent(t)
	if len(requirements.Dependencies) != 2 {
		t.Fatal("valid selection did not exclude invalid sibling")
	}
	j, root = localCollectionJourney(t)
	writeCollectionSkill(t, root, "nested/review", "review")
	j.run(t, "skills", "add", root, "--path", "nested/review", "--target", "codex,claude", "--yes", "--format", "json")
	requirements, _ = j.intent(t)
	if len(requirements.Dependencies) != 1 || requirements.Dependencies["review"].Local.Path != filepath.ToSlash(filepath.Join(root, "nested", "review")) {
		t.Fatal("exact path selected another Skill", requirements)
	}
}

func TestLocalSingleSkillRepeatKeepsSavedPinWithoutSource(t *testing.T) {
	for _, missing := range []string{"directory", "root-document"} {
		t.Run(missing, func(t *testing.T) {
			j, root := localSkillJourney(t)
			args := []string{"skills", "add", root, "--target", "codex,claude", "--yes", "--format", "json"}
			j.run(t, args...)
			before := collectionDurableTree(t, j)
			removed := root
			if missing == "root-document" {
				writeCollectionSkill(t, root, "nested/testing", "testing")
				removed = filepath.Join(root, "SKILL.md")
			}
			if err := os.RemoveAll(removed); err != nil {
				t.Fatal(err)
			}
			j.run(t, args...)
			if after := collectionDurableTree(t, j); !reflect.DeepEqual(before, after) {
				t.Fatal("saved single Skill re-add changed durable state: " + savedJourneyDifferences(before, after))
			}
		})
	}
}
