package agentpluginscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Exact names must never pick the first duplicate. A repository containing a
// root Skill also must not silently become an explicit non-interactive choice.
func TestSkillsCLIRequiresUnambiguousExplicitSelectionBeforeEffects(t *testing.T) {
	journey, repo, _ := skillsFullFlowFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "SKILL.md"), []byte("---\nname: skills\ndescription: Root fixture Skill\n---\n# Root\n"), 0600); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: disposable root Skill selection")
	before := savedJourneyTree(t, journey.cli.app.DependenciesRoot)
	for _, test := range []struct {
		args []string
		code string
	}{
		{[]string{"skills", "add", "acme/skills", "--skill", "review", "--target", "codex", "--no-ui", "--format", "json"}, "SKILL_AMBIGUOUS"},
		{[]string{"skills", "add", "acme/skills", "--target", "codex", "--no-ui", "--format", "json"}, "SELECTION_REQUIRED"},
		{[]string{"skills", "add", "acme/skills", "--path", "engineering/review", "--target", "codex", "--no-ui", "--format", "json"}, "CONFIRMATION_REQUIRED"},
	} {
		out, _, err := journey.cli.execute(false, test.args...)
		if err == nil || !strings.Contains(out, test.code) {
			t.Fatalf("%v: wanted %s refusal, got %v %s", test.args, test.code, err, out)
		}
		if !reflect.DeepEqual(before, savedJourneyTree(t, journey.cli.app.DependenciesRoot)) || journey.runner.calls != 0 {
			t.Fatalf("refused selection changed the environment: %s", out)
		}
	}
	out, _, err := journey.cli.execute(false, "skills", "add", "acme/skills", "--path", "engineering/review", "--target", "codex", "--no-ui", "--format", "json", "--dry-run")
	if err != nil || !strings.Contains(out, `"actions"`) {
		t.Fatalf("noninteractive dry-run requires no confirmation: %v %s", err, out)
	}
	if !reflect.DeepEqual(before, savedJourneyTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("dry-run changed persistent environment")
	}
}

// The presentation body excludes YAML; raw inspection retains the exact bounded
// source document used by installation, including its frontmatter.
func TestSkillsPreviewKeepsBodyAndExactRawContent(t *testing.T) {
	journey, _, raw := skillsFullFlowFixture(t)
	session, err := journey.cli.app.openSkillsCatalog(context.Background(), SkillsPickerRequest{Input: "acme/skills", Path: "engineering/review", PathSet: true})
	if err != nil {
		t.Fatal(err)
	}
	defer session.cleanup()
	catalog, err := session.scan(context.Background(), "engineering/review")
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Candidates) != 1 {
		t.Fatalf("exact catalog candidates: %d", len(catalog.Candidates))
	}
	preview, err := session.preview(context.Background(), catalog.Candidates[0].Id)
	if err != nil {
		t.Fatal(err)
	}
	if preview.RawContent == nil || *preview.RawContent != string(raw) {
		t.Fatal("raw preview changed source bytes")
	}
	if strings.HasPrefix(preview.Content, "---") || !strings.Contains(preview.Content, "# Review") {
		t.Fatalf("preview did not expose Markdown body: %q", preview.Content)
	}
}

// SDK reason strings can contain native-provider logs. Public operation items
// preserve useful stable reason guidance without copying arbitrary source text.
func TestSkillsOperationOutcomeNeverExposesProviderReasons(t *testing.T) {
	outcome := skillBatchOutcome(SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{{Status: "failed", Reason: "native-provider credential=private-secret"}, {Status: "failed", Reason: "plan_changed"}}})
	body, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "private-secret") || strings.Contains(string(body), "native-provider") {
		t.Fatalf("provider logs crossed public outcome boundary: %s", body)
	}
	if outcome.Items[1].Message == nil || *outcome.Items[1].Message != "Installer facts changed. Prepare a new review before retrying." {
		t.Fatalf("stable reason guidance lost: %s", body)
	}
}
