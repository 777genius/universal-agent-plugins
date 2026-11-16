package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
)

type skillsSelectionPrompter struct {
	prompt.Prompter
	selectSkills func(prompt.SkillsSelectionRequest) (prompt.SkillsSelectionResult, error)
}

func (p skillsSelectionPrompter) SelectSkills(_ context.Context, request prompt.SkillsSelectionRequest) (prompt.SkillsSelectionResult, error) {
	return p.selectSkills(request)
}

func TestSkillsCatalogUsesRichCapabilityAndCanonicalSelection(t *testing.T) {
	available := []skillsource.Candidate{{ID: "first", Name: "Review", Path: "engineering/review", Description: "Long description belongs in review, not the menu"}, {ID: "last", Name: "Docs", Path: "docs/review"}}
	session, catalog := richCatalogFixture(available)
	called := false
	session.app.Prompter = skillsSelectionPrompter{selectSkills: func(request prompt.SkillsSelectionRequest) (prompt.SkillsSelectionResult, error) {
		called = true
		if len(request.Choices) != 2 || !strings.Contains(request.Choices[0].Label, "engineering/review") || strings.Contains(request.Choices[0].Label, "Long description") {
			t.Fatalf("rich catalog lost path identity: %+v", request)
		}
		return prompt.SkillsSelectionResult{IDs: []string{"last", "first"}}, nil
	}}
	command := &cobra.Command{}
	selected, err := promptSkillsCatalog(command, session, catalog, &options{format: "human"})
	if err != nil || !called || !reflect.DeepEqual(selected, available) {
		t.Fatalf("rich selection routing/canonical order: %+v %v called=%t", selected, err, called)
	}
}

func TestSkillsRichCancellationNeverFallsBackToNumericInput(t *testing.T) {
	session, catalog := richCatalogFixture([]skillsource.Candidate{{ID: "first", Name: "Review"}})
	session.app.Prompter = skillsSelectionPrompter{selectSkills: func(prompt.SkillsSelectionRequest) (prompt.SkillsSelectionResult, error) {
		return prompt.SkillsSelectionResult{}, prompt.ErrPromptCanceled
	}}
	input := strings.NewReader("1\ndone\n")
	command := &cobra.Command{}
	command.SetIn(input)
	command.SetOut(&bytes.Buffer{})
	_, err := promptSkillsCatalog(command, session, catalog, &options{format: "human"})
	if !errors.Is(err, prompt.ErrPromptCanceled) || input.Len() != len("1\ndone\n") {
		t.Fatalf("cancellation retried as a different prompt: %v unread=%d", err, input.Len())
	}
}

func TestSkillsPlainExplicitlyKeepsNumericCatalog(t *testing.T) {
	available := []skillsource.Candidate{{ID: "first", Name: "Review"}}
	session, catalog := richCatalogFixture(available)
	session.app.Prompter = skillsSelectionPrompter{selectSkills: func(prompt.SkillsSelectionRequest) (prompt.SkillsSelectionResult, error) {
		t.Fatal("--plain invoked rich selection")
		return prompt.SkillsSelectionResult{}, nil
	}}
	command := &cobra.Command{}
	command.SetIn(strings.NewReader("1\ndone\n"))
	command.SetOut(&bytes.Buffer{})
	selected, err := promptSkillsCatalog(command, session, catalog, &options{format: "human", plain: true})
	if err != nil || !reflect.DeepEqual(selected, available) {
		t.Fatalf("plain catalog changed: %+v %v", selected, err)
	}
}

func richCatalogFixture(available []skillsource.Candidate) (*skillsCatalogSession, skillspicker.Catalog) {
	session := &skillsCatalogSession{candidates: map[string]skillsource.Candidate{}}
	catalog := skillspicker.Catalog{}
	for _, candidate := range available {
		session.candidates[candidate.ID] = candidate
		catalog.Candidates = append(catalog.Candidates, skillspicker.Candidate{Id: candidate.ID, Status: skillspicker.Available})
	}
	return session, catalog
}
