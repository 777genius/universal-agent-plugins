package agentpluginscli

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

func selectSkillCandidates(cmd *cobra.Command, app App, opts *options, catalog skillspicker.Catalog, candidates map[string]skillsource.Candidate, names []string, all, interactive bool) ([]skillsource.Candidate, error) {
	if all || len(names) > 0 {
		selected, err := explicitSkillCandidates(catalog, candidates, names, all)
		if err != nil {
			return nil, renderSkillsCommandIssue(cmd, opts, err)
		}
		return selected, nil
	}
	if !interactive {
		return nil, skillsMissingSelection(cmd, opts, "SELECTION_REQUIRED", "Use --skill, --path or --all with --target in a non-interactive session", catalog.Candidates)
	}
	return promptSkillCandidates(cmd, app, catalog, candidates, opts)
}

func explicitSkillCandidates(catalog skillspicker.Catalog, candidates map[string]skillsource.Candidate, names []string, all bool) ([]skillsource.Candidate, error) {
	issue := func(code, message string) error {
		return skillsSelectionError{Code: code, Message: message, Candidates: catalog.Candidates}
	}
	if !catalog.Complete {
		return nil, issue("CATALOG_INCOMPLETE", "The catalog is incomplete; narrow the source to an exact folder")
	}
	if all {
		if len(catalog.Candidates) == 0 {
			return nil, issue("SELECTION_REQUIRED", "The catalog contains no Skills")
		}
		names = make([]string, 0, len(catalog.Candidates))
		for _, wire := range catalog.Candidates {
			candidate, ok := candidates[wire.Id]
			if !ok || wire.Status != skillspicker.Available || candidate.Status != "available" {
				return nil, issue("SKILL_UNAVAILABLE", "--all requires every Skill in the catalog to be available")
			}
			names = append(names, candidate.Name)
		}
	}
	selected := []skillsource.Candidate{}
	seen := map[string]bool{}
	for _, name := range names {
		matches := []skillsource.Candidate{}
		for _, candidate := range candidates {
			if candidate.Name == name {
				matches = append(matches, candidate)
			}
		}
		if err := namedSkillIssue(name, matches, catalog.Candidates); err != nil {
			return nil, *err
		}
		if name == "" || seen[name] {
			return nil, issue("INVALID_SELECTION", "Choose unique non-empty Skill names")
		}
		seen[name] = true
		selected = append(selected, matches[0])
	}
	if all {
		sort.Slice(selected, func(i, j int) bool { return selected[i].Path < selected[j].Path })
	}
	return selected, nil
}

func allWebsiteSkills(ctx context.Context, app App, endpoint string) ([]SkillsSelection, error) {
	lister, ok := app.SourceAcquirer.(interface {
		ListHTTPSkills(context.Context, string) (sourceacquisition.HTTPSkillCatalog, error)
	})
	if !ok {
		return nil, errors.New("HTTPS publisher catalog is unavailable")
	}
	index, err := lister.ListHTTPSkills(ctx, endpoint)
	if err != nil {
		return nil, httpsSkillsListFailure(err)
	}
	catalog := skillspicker.Catalog{Complete: true, Candidates: []skillspicker.Candidate{}}
	candidates := map[string]skillsource.Candidate{}
	for i, item := range index.Candidates {
		id := strconv.Itoa(i)
		status := "unavailable"
		if item.Available {
			status = "available"
		}
		candidate := skillsource.Candidate{ID: id, Name: item.Name, Path: item.Name, Status: status}
		candidates[id] = candidate
		catalog.Candidates = append(catalog.Candidates, wireSkillCandidate(candidate))
	}
	selected, err := explicitSkillCandidates(catalog, candidates, nil, true)
	if err != nil {
		return nil, err
	}
	selections := make([]SkillsSelection, 0, len(selected))
	for _, candidate := range selected {
		selections = append(selections, SkillsSelection{Website: index.URL, Name: candidate.Name, ExpectedIndexSHA256: index.IndexSHA256})
	}
	return selections, nil
}
