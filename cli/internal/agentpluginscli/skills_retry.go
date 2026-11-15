package agentpluginscli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

// maxSkillsRetries bounds the interactive retry loop. A person who keeps
// failing the same way is better served by the technical log than a fourth try.
const maxSkillsRetries = 3

// applySkillsWithRecovery applies a reviewed batch, explains agents that did not
// succeed in plain words, records the technical detail in the install log and,
// when a person is present, offers to retry only the agents that failed.
func applySkillsWithRecovery(cmd *cobra.Command, app App, opts *options, prepared *PreparedSkillsBatch, canRetry bool) error {
	reviewed := prepared.Actions()
	current := prepared
	// outstanding tracks agents that failed in any attempt and were not fixed
	// since, so a successful last retry cannot hide an earlier unresolved agent.
	outstanding := map[string]string{}
	for attempt := 1; ; attempt++ {
		result, err := applySkillsTerminalAttempt(cmd, app, opts, current)
		for _, report := range reportByAgent(skillsAgentItems(result.Items)) {
			if report.Failed() {
				outstanding[string(report.ClientID)] = report.Label
			} else {
				delete(outstanding, string(report.ClientID))
			}
		}
		logPath := app.recordInstallRun("skills add", attempt, agentLogEntries(skillsAgentItems(result.Items)))
		err = humanSkillsApplyError(opts.format, result.Items, err, logPath)
		if writeErr := renderSkillsResult(cmd, opts, result, logPath); writeErr != nil {
			return errors.Join(err, writeErr)
		}
		if current != prepared {
			current.Close()
		}
		if err == nil && len(outstanding) > 0 {
			err = fmt.Errorf("%s still need attention; see the technical log", strings.Join(sortedValues(outstanding), ", "))
		}
		if err == nil || !canRetry || attempt > maxSkillsRetries || cmd.Context().Err() != nil || result.Status == "succeeded" {
			return err
		}
		targets, askErr := askRetryTargets(cmd, app, opts, reportByAgent(skillsAgentItems(result.Items)))
		if askErr != nil || len(targets) == 0 {
			return err
		}
		next, prepErr := app.PrepareSkills(cmd.Context(), SkillsBatchRequest{Selections: prepared.selections, TargetIDs: targets})
		if prepErr != nil {
			return errors.Join(err, fmt.Errorf("could not prepare the retry: %w", prepErr))
		}
		defer next.Close()
		if !coveredByReview(reviewed, next.Actions()) {
			// The retry would do something the person never reviewed: show and confirm it.
			if confirmErr := confirmChangedRetry(cmd, app, opts, next); confirmErr != nil {
				next.Close()
				return errors.Join(err, confirmErr)
			}
		}
		current = next
		if _, werr := fmt.Fprintf(cmd.ErrOrStderr(), "Retrying %d agent(s)…\n", len(targets)); werr != nil {
			return errors.Join(err, werr)
		}
	}
}

// Preserve typed causes while the final human error points to the report that
// already explains the failed bindings. JSON and SDK callers keep their errors.
func humanSkillsApplyError(format string, items []SkillsBatchItem, cause error, logPath string) error {
	if format != "human" || cause == nil {
		return cause
	}
	for _, item := range items {
		if item.failure != nil {
			return &skillsReportedFailure{cause: cause, logPath: logPath}
		}
	}
	return cause
}

type skillsReportedFailure struct {
	cause   error
	logPath string
}

func (failure *skillsReportedFailure) Error() string {
	message := "Skills installation did not complete. Follow the per-agent results above."
	if failure.logPath != "" {
		message += " Technical details: " + prompt.SafeText(failure.logPath)
	}
	return message
}

func (failure *skillsReportedFailure) Unwrap() error { return failure.cause }

// askRetryTargets lets the person pick which failed agents to try again. All
// agents that a retry can help start selected; cancelling means no retry.
func askRetryTargets(cmd *cobra.Command, app App, opts *options, reports []agentReport) ([]string, error) {
	retryable := retryableAgents(reports)
	if len(retryable) == 0 {
		return nil, nil
	}
	app, err := withPrompter(cmd, app, opts)
	if err != nil {
		return nil, err
	}
	request := prompt.TargetSelectionRequest{Title: "Retry failed agents? Enter retries the selected ones, Esc skips"}
	for _, report := range retryable {
		request.Choices = append(request.Choices, prompt.TargetChoice{ID: report.ClientID, Label: report.Label})
		request.DefaultIDs = append(request.DefaultIDs, report.ClientID)
	}
	selected, err := app.Prompter.SelectTargets(cmd.Context(), request)
	if errors.Is(err, prompt.ErrPromptCanceled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	validated, err := prompt.ValidateSelection(request, selected.IDs)
	if err != nil {
		return nil, err
	}
	targets := make([]string, len(validated.IDs))
	for index, id := range validated.IDs {
		targets[index] = string(id)
	}
	return targets, nil
}

// coveredByReview reports whether every retry action was already part of the
// plan the person confirmed. Only alias, agent and destination matter: a retry
// may turn an install into an update of the same destination without widening
// what was approved.
func coveredByReview(reviewed, retry []SkillsAction) bool {
	type key struct{ alias, client, path string }
	approved := make(map[key]bool, len(reviewed))
	for _, action := range reviewed {
		approved[key{action.Alias, action.ClientID, action.TargetPath}] = true
	}
	for _, action := range retry {
		if action.Action == "unchanged" {
			continue
		}
		if !approved[key{action.Alias, action.ClientID, action.TargetPath}] {
			return false
		}
	}
	return true
}

func confirmChangedRetry(cmd *cobra.Command, app App, opts *options, prepared *PreparedSkillsBatch) error {
	review := skillsPickerPrepared{prepared}.Review()
	if err := renderHumanSkillsReview(cmd, review); err != nil {
		return err
	}
	app, err := withPrompter(cmd, app, opts)
	if err != nil {
		return err
	}
	answer, err := app.Prompter.Confirm(cmd.Context(), prompt.ConfirmationRequest{Title: "The retry differs from what you reviewed. Apply it?", Default: false})
	if err != nil {
		return err
	}
	if !answer.Accepted {
		return errors.New("retry not applied")
	}
	return nil
}

func sortedValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}
