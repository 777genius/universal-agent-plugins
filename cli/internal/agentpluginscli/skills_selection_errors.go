package agentpluginscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
)

type skillsSelectionError struct {
	Code       string                   `json:"code"`
	Message    string                   `json:"message"`
	Candidates []skillspicker.Candidate `json:"candidates"`
}

func (e skillsSelectionError) Error() string { return e.Message }
func skillsMissingSelection(cmd *cobra.Command, opts *options, code, message string, candidates []skillspicker.Candidate) error {
	return emitSkillsSelectionIssue(cmd, opts, skillsSelectionError{Code: code, Message: message, Candidates: candidates})
}
func emitSkillsSelectionIssue(cmd *cobra.Command, opts *options, issue skillsSelectionError) error {
	if issue.Candidates == nil {
		issue.Candidates = []skillspicker.Candidate{}
	}
	if opts.format == "json" {
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(issue); err != nil {
			return errors.Join(issue, err)
		}
		return issue
	}
	for _, candidate := range issue.Candidates[:min(len(issue.Candidates), 10)] {
		path := candidate.Path
		if path == "" {
			path = "."
		}
		name := candidate.Name
		if name == "" {
			name = "SKILL.md"
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s — %s (%s)\n", prompt.SafeText(path), prompt.SafeText(name), candidate.Status); err != nil {
			return errors.Join(issue, err)
		}
	}
	if len(issue.Candidates) > 10 {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Showing 10 of %d candidates. Narrow the folder with --path or use --no-ui in a terminal to search.\n", len(issue.Candidates))
	}
	return issue
}
func wireSkillCandidate(candidate skillsource.Candidate) skillspicker.Candidate {
	return skillspicker.Candidate{Id: candidate.ID, Name: candidate.Name, Description: candidate.Description, Path: candidate.Path, Status: skillspicker.CandidateStatus(candidate.Status), DiagnosticCodes: append([]string{}, candidate.Diagnostics...)}
}
func exactSkillIssue(path string, candidate *skillsource.Candidate) skillsSelectionError {
	code, message := "SKILL_UNAVAILABLE", fmt.Sprintf("No readable SKILL.md was found in folder %q. Choose its exact repository-relative folder with --path.", prompt.SafeText(path))
	matches := []skillspicker.Candidate{}
	if candidate != nil {
		matches = append(matches, wireSkillCandidate(*candidate))
		if candidate.Status == "invalid" {
			code = "SKILL_INVALID"
		}
		message = fmt.Sprintf("Skill in folder %q is %s", prompt.SafeText(path), candidate.Status)
		if len(candidate.Diagnostics) > 0 {
			message += ": " + skillDiagnosticGuidance(candidate.Diagnostics)
		}
		if candidate.Status == "invalid" {
			message += ". Choose another folder or correct its metadata before retrying."
		} else {
			message += ". Retry the source or narrow to another exact --path."
		}
	}
	return skillsSelectionError{Code: code, Message: message, Candidates: matches}
}
func namedSkillIssue(name string, matches []skillsource.Candidate, catalog []skillspicker.Candidate) *skillsSelectionError {
	name = prompt.SafeText(name)
	sort.Slice(matches, func(i, j int) bool { return matches[i].Path < matches[j].Path })
	if len(matches) == 0 {
		return &skillsSelectionError{Code: "SKILL_NOT_FOUND", Message: fmt.Sprintf("No Skill named %q exists in this catalog. Names are exact; use --path to choose a folder.", name), Candidates: catalog}
	}
	candidates := make([]skillspicker.Candidate, 0, len(matches))
	for _, candidate := range matches {
		candidates = append(candidates, wireSkillCandidate(candidate))
	}
	if len(matches) > 1 {
		return &skillsSelectionError{Code: "SKILL_AMBIGUOUS", Message: fmt.Sprintf("Skill name %q matches multiple folders. Choose one exact --path.", name), Candidates: candidates}
	}
	if matches[0].Status != "available" {
		issue := exactSkillIssue(matches[0].Path, &matches[0])
		return &issue
	}
	return nil
}
func (s *skillsCatalogSession) rememberTerminalCandidates(candidates []skillsource.Candidate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, candidate := range candidates {
		s.candidates[candidate.ID] = candidate
	}
}

func renderSkillsCommandIssue(cmd *cobra.Command, opts *options, err error) error {
	var issue skillsSelectionError
	if errors.As(err, &issue) {
		return emitSkillsSelectionIssue(cmd, opts, issue)
	}
	return err
}
