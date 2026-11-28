package agentpluginscli

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

func TestSkillsAllStrictCatalog(t *testing.T) {
	valid := []skillsource.Candidate{{ID: "b", Name: "testing", Path: "z/testing", Status: "available"}, {ID: "a", Name: "review", Path: "a/review", Status: "available"}}
	for _, test := range []struct {
		name     string
		entries  []skillsource.Candidate
		complete bool
		code     string
	}{
		{"valid", valid, true, ""}, {"empty", nil, true, "SELECTION_REQUIRED"}, {"incomplete", valid, false, "CATALOG_INCOMPLETE"},
		{"invalid", []skillsource.Candidate{{ID: "a", Name: "review", Status: "invalid"}}, true, "SKILL_UNAVAILABLE"},
		{"unavailable", []skillsource.Candidate{{ID: "a", Name: "review", Status: "unavailable"}}, true, "SKILL_UNAVAILABLE"},
		{"ambiguous", []skillsource.Candidate{{ID: "a", Name: "review", Status: "available"}, {ID: "b", Name: "review", Status: "available"}}, true, "SKILL_AMBIGUOUS"},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := skillspicker.Catalog{Complete: test.complete}
			candidates := map[string]skillsource.Candidate{}
			for _, c := range test.entries {
				catalog.Candidates = append(catalog.Candidates, wireSkillCandidate(c))
				candidates[c.ID] = c
			}
			got, err := explicitSkillCandidates(catalog, candidates, nil, true)
			if test.code != "" {
				var issue skillsSelectionError
				if !errors.As(err, &issue) || issue.Code != test.code {
					t.Fatalf("got %v, want %s", err, test.code)
				}
				return
			}
			if err != nil || len(got) != 2 || got[0].Name != "review" {
				t.Fatalf("unexpected selection: %+v %v", got, err)
			}
		})
	}
}

func TestSkillsAllFlagConflictsBeforeSource(t *testing.T) {
	for _, flags := range [][]string{{"--skill", "review"}, {"--skill", ""}, {"--path", ""}, {"--ui"}} {
		j := newSavedJourney(t, t.TempDir())
		args := append([]string{"skills", "add", "acme/does-not-exist", "--all", "--target", "codex", "--yes"}, flags...)
		_, _, err := j.cli.execute(false, args...)
		if err == nil || !strings.Contains(err.Error(), "--all cannot combine") {
			t.Fatalf("flags %v: %v", flags, err)
		}
		if j.runner.calls != 0 {
			t.Fatal("conflict ran native commands")
		}
	}
	selection := skillsAddOptions{all: true, yes: true}
	if selection.useBrowserPicker(true, &options{}, false, false) || selection.requiresInteraction(&options{target: "codex"}, false) {
		t.Fatal("all-selection requested catalog interaction")
	}
}

type allWebsiteCatalogAcquirer struct {
	sourceacquisition.Acquirer
	catalog sourceacquisition.HTTPSkillCatalog
}

func (a allWebsiteCatalogAcquirer) ListHTTPSkills(context.Context, string) (sourceacquisition.HTTPSkillCatalog, error) {
	return a.catalog, nil
}
func TestSkillsAllWebsiteCarriesIndexExpectation(t *testing.T) {
	app := App{SourceAcquirer: allWebsiteCatalogAcquirer{catalog: sourceacquisition.HTTPSkillCatalog{URL: "https://publisher.example", IndexSHA256: strings.Repeat("a", 64), Candidates: []sourceacquisition.HTTPSkillCandidate{{Name: "review", Available: true}, {Name: "testing", Available: true}}}}}
	got, err := allWebsiteSkills(context.Background(), app, "https://publisher.example")
	want := []SkillsSelection{{Website: "https://publisher.example", Name: "review", ExpectedIndexSHA256: strings.Repeat("a", 64)}, {Website: "https://publisher.example", Name: "testing", ExpectedIndexSHA256: strings.Repeat("a", 64)}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestSkillsAllGitSourceScope(t *testing.T) {
	for _, scope := range []string{"engineering", "engineering/review"} {
		t.Run(scope, func(t *testing.T) {
			journey, _, _ := skillsFullFlowFixture(t)
			input := "https://github.com/acme/skills/tree/main/" + scope
			journey.run(t, "skills", "add", input, "--all", "--target", "codex,claude", "--yes", "--format", "json")
			requirements, locked := journey.intent(t)
			if len(locked.Packages) != 1 || locked.Packages[0].Source.Git.Path != "engineering/review" || len(requirements.Targets) != 2 {
				t.Fatalf("scope escaped: %+v %+v", requirements, locked)
			}
		})
	}
}

func TestSkillsAllRejectsAmbiguousGitCollectionBeforeEffects(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	out, _, err := journey.cli.execute(false, "skills", "add", "acme/skills", "--all", "--target", "codex,claude", "--yes", "--format", "json")
	if err == nil || !strings.Contains(out, "SKILL_AMBIGUOUS") || journey.runner.calls != 0 {
		t.Fatalf("ambiguous all selection: %s %v native=%d", out, err, journey.runner.calls)
	}
}
