package agentpluginscli

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSavedAddSelectedTargetsPreserveIntent(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	source := "github:acme/demo@" + sha
	j.run(t, "add", source, "--target", "codex,claude", "--format", "json")
	before := savedJourneyTree(t, j.cli.app.DependenciesRoot)
	out := j.run(t, "add", source, "--target", "codex", "--format", "json", "--dry-run")
	if strings.Contains(out, `"client_id": "claude"`) || strings.Contains(out, `"target": "claude"`) {
		t.Fatalf("unselected target in review: %s", out)
	}
	out = j.run(t, "add", source, "--target", "codex", "--format", "json")
	if strings.Contains(out, `"client_id": "claude"`) || strings.Contains(out, `"target": "claude"`) {
		t.Fatalf("unselected target applied: %s", out)
	}
	if !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)) {
		t.Fatalf("re-add changed saved environment: %s", savedJourneyDifferences(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)))
	}
	req, _ := j.intent(t)
	if len(req.Targets["claude"]) != 1 {
		t.Fatal("unselected saved intent removed")
	}
}

func TestSkillsSelectedTargetsPreserveIntent(t *testing.T) {
	j, _, sha := skillJourney(t)
	ctx := context.Background()
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf"}}, TargetIDs: []string{"codex", "claude"}}
	plan, err := j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = plan.Apply(ctx); err != nil {
		plan.Close()
		t.Fatal(err)
	}
	plan.Close()
	before := j.state(t)
	request.TargetIDs = []string{"codex"}
	plan, err = j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if warnings := plan.Warnings(); len(warnings) == 0 || !strings.Contains(warnings[0], "Saved targets claude are preserved") {
		t.Fatalf("missing target notice: %v", warnings)
	}
	if actions := plan.Actions(); len(actions) != 1 || actions[0].ClientID != "codex" {
		t.Fatalf("review includes unselected targets: %+v", actions)
	}
	if err = plan.Check(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := plan.Apply(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].ClientID != "codex" {
		t.Fatalf("apply includes unselected targets: %+v", result)
	}
	if !reflect.DeepEqual(before, j.state(t)) {
		t.Fatal("re-add changed installed bindings")
	}
	req, _ := j.intent(t)
	if len(req.Targets["claude"]) != 1 {
		t.Fatal("unselected saved intent removed")
	}
}
