package agentpluginscli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
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
	if !slices.Equal(savedTargetActionClients(t, out), []string{"codex"}) {
		t.Fatalf("unselected target in review: %s", out)
	}
	out = j.run(t, "add", source, "--target", "codex", "--format", "json")
	if !slices.Equal(savedTargetActionClients(t, out), []string{"codex"}) {
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

func TestSavedRemoveSelectedTargetLeavesUnselectedDrift(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	j.run(t, "add", "github:acme/demo@"+sha, "--target", "codex,claude", "--format", "json")
	before := savedJourneyBinding(t, j.state(t), "claude")
	view, err := j.cli.app.Installer.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var driftPath string
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			if binding.ClientID == "claude" {
				driftPath = filepath.Join(binding.TargetPath, "user-edit.txt")
			}
		}
	}
	if driftPath == "" {
		t.Fatal("missing Claude profile")
	}
	dependencyWrite(t, driftPath, "preserve unselected drift")
	out := j.run(t, "remove", "demo", "--target", "codex", "--external-uninstalled", "--format", "json")
	if !slices.Equal(savedTargetActionClients(t, out), []string{"codex"}) {
		t.Fatalf("unselected target action: %s", out)
	}
	if after := savedJourneyBinding(t, j.state(t), "claude"); !reflect.DeepEqual(before, after) {
		t.Fatal("unselected binding changed")
	}
	if dependencyRead(t, driftPath) != "preserve unselected drift" {
		t.Fatal("unselected edits changed")
	}
	req, _ := j.intent(t)
	if len(req.Targets["codex"]) != 0 || len(req.Targets["claude"]) != 1 {
		t.Fatalf("wrong removal intent: %+v", req.Targets)
	}
}

func TestSavedUpdateStillRequiresAllTargets(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	j.run(t, "add", "github:acme/demo@"+sha, "--target", "codex,claude", "--format", "json")
	before := savedJourneyTree(t, j.cli.app.DependenciesRoot)
	_, _, err := j.cli.execute(false, "update", "demo", "--target", "codex", "--dry-run", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "must select every target") {
		t.Fatalf("subset update accepted: %v", err)
	}
	if !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)) {
		t.Fatal("refused update changed saved files")
	}
	for _, args := range [][]string{{"update", "demo", "--dry-run", "--format", "json"}, {"update", "demo", "--target", "codex,claude", "--dry-run", "--format", "json"}} {
		out := j.run(t, args...)
		if !slices.Equal(savedTargetActionClients(t, out), []string{"claude", "codex"}) {
			t.Fatalf("full update omitted saved target: %s", out)
		}
	}
}

func savedTargetActionClients(t *testing.T, output string) []string {
	t.Helper()
	var envelope struct{ Data json.RawMessage }
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatal(err)
	}
	body := []byte(output)
	if len(envelope.Data) > 0 {
		body = envelope.Data
	}
	var result struct {
		Actions []struct {
			ClientID string `json:"client_id"`
		}
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	clients := make([]string, 0, len(result.Actions))
	for _, action := range result.Actions {
		clients = append(clients, action.ClientID)
	}
	slices.Sort(clients)
	return clients
}
