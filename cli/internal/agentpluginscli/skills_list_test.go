package agentpluginscli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSkillsListIsDeterministicAndReadOnly(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	before := savedJourneyTree(t, journey.cli.app.DependenciesRoot)
	var first string
	for i := 0; i < 2; i++ {
		out, _, err := journey.cli.execute(false, "skills", "list", "acme/skills", "--format", "json")
		if err != nil {
			t.Fatalf("source listing: %v", err)
		}
		var response struct {
			SchemaVersion int              `json:"schema_version"`
			Command       string           `json:"command"`
			Result        string           `json:"result"`
			Data          skillsListReport `json:"data"`
		}
		if err := json.Unmarshal([]byte(out), &response); err != nil {
			t.Fatal(err)
		}
		if response.SchemaVersion != 1 || response.Command != "skills list" || response.Result != "success" {
			t.Fatalf("invalid listing envelope: %s", out)
		}
		if !response.Data.Complete || len(response.Data.Candidates) != 2 {
			t.Fatalf("expected complete nested catalog, got: %s", out)
		}
		paths := []string{response.Data.Candidates[0].Path, response.Data.Candidates[1].Path}
		if !reflect.DeepEqual(paths, []string{"docs/review", "engineering/review"}) {
			t.Fatalf("lost duplicate Skill names or deterministic paths: %v", paths)
		}
		for _, candidate := range response.Data.Candidates {
			if candidate.Name != "review" || candidate.Status != "available" {
				t.Fatalf("unexpected Skill: %+v", candidate)
			}
		}
		if i == 0 {
			first = out
		} else if first != out {
			t.Fatalf("machine listing contains a transient session identifier:\n%s\n%s", first, out)
		}
	}
	if !reflect.DeepEqual(before, savedJourneyTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("read-only listing changed saved environment")
	}
	if journey.runner.calls != 0 {
		t.Fatal("read-only listing started a native client")
	}
}

func TestSkillsListExactFolderAndInvalidSelectors(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	out, _, err := journey.cli.execute(false, "skills", "list", "acme/skills",
		"--path", "engineering/review", "--format", "json")
	if err != nil || !strings.Contains(out, `"path":"engineering/review"`) {
		t.Fatalf("exact listing: %v %s", err, out)
	}
	var response struct{ Data skillsListReport `json:"data"` }
	if err := json.Unmarshal([]byte(out), &response); err != nil || len(response.Data.Candidates) != 1 {
		t.Fatalf("exact folder listing: %v %s", err, out)
	}
	for _, args := range [][]string{
		{"skills", "list", "acme/skills", "--path", "../escape", "--format", "json"},
		{"skills", "list", "acme/skills", "--target", "codex", "--format", "json"},
	} {
		if _, _, err := journey.cli.execute(false, args...); err == nil {
			t.Fatalf("unsafe or mutation-only selection unexpectedly accepted: %v", args)
		}
	}
}

func TestAgentYesIsExplicitWithoutChangingLegacyConfirmation(t *testing.T) {
	if !mutationConfirmed(App{Terminal: true}, &options{format: "human", yes: true}) {
		t.Fatal("--yes must explicitly confirm an already reviewed plan")
	}
	if mutationConfirmed(App{Terminal: true}, &options{format: "human"}) {
		t.Fatal("ordinary interactive invocation must still ask")
	}
	if !mutationConfirmed(App{Terminal: false}, &options{format: "json"}) {
		t.Fatal("legacy noninteractive add compatibility changed")
	}
}
