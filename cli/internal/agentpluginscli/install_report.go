package agentpluginscli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/installlog"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// agentReport is everything a person needs to know about one agent after an
// installation: whether it worked, why not in plain words, and what to do next.
type agentReport struct {
	ClientID  domain.ClientID
	Label     string
	Succeeded []string
	Problems  []agentProblem
}

// agentProblem groups the items of one agent that share the same explanation.
type agentProblem struct {
	Failure installFailure
	Items   []string
}

func (report agentReport) Failed() bool { return len(report.Problems) > 0 }

// Retryable reports whether trying this agent again can help.
func (report agentReport) Retryable() bool {
	for _, problem := range report.Problems {
		if !problem.Failure.Retryable {
			return false
		}
	}
	return report.Failed()
}

// agentItem is one package on one agent, the neutral input of every report.
type agentItem struct {
	Alias, ClientID, Status string
	failure                 *installFailure
}

func skillsAgentItems(items []SkillsBatchItem) []agentItem {
	out := make([]agentItem, len(items))
	for index, item := range items {
		out[index] = agentItem{Alias: item.Alias, ClientID: item.ClientID, Status: item.Status, failure: item.failure}
	}
	return out
}

func outcomeAgentItems(outcomes []environment.BindingOutcome) []agentItem {
	out := make([]agentItem, len(outcomes))
	for index, outcome := range outcomes {
		status, _ := outcomeStatusReason(outcome)
		out[index] = agentItem{Alias: outcome.Binding.Alias, ClientID: outcome.Binding.ClientID, Status: status, failure: describeBindingFailure(outcome)}
	}
	return out
}

// reportByAgent folds per-binding results into one entry per agent, keeping the
// order in which agents first appear.
func reportByAgent(items []agentItem) []agentReport {
	var reports []agentReport
	index := map[string]int{}
	for _, item := range items {
		position, seen := index[item.ClientID]
		if !seen {
			position = len(reports)
			index[item.ClientID] = position
			id := domain.ClientID(item.ClientID)
			reports = append(reports, agentReport{ClientID: id, Label: reviewClientName(id)})
		}
		report := &reports[position]
		if item.failure == nil {
			report.Succeeded = append(report.Succeeded, item.Alias)
			continue
		}
		report.addProblem(*item.failure, item.Alias)
	}
	return reports
}

func (report *agentReport) addProblem(failure installFailure, item string) {
	for index := range report.Problems {
		if report.Problems[index].Failure.Kind == failure.Kind {
			report.Problems[index].Items = append(report.Problems[index].Items, item)
			return
		}
	}
	report.Problems = append(report.Problems, agentProblem{Failure: failure, Items: []string{item}})
}

func failedAgents(reports []agentReport) []agentReport {
	var failed []agentReport
	for _, report := range reports {
		if report.Failed() {
			failed = append(failed, report)
		}
	}
	return failed
}

// retryableAgents lists the agents a retry can help, in report order.
func retryableAgents(reports []agentReport) []agentReport {
	var retryable []agentReport
	for _, report := range reports {
		for _, problem := range report.Problems {
			if problem.Failure.Kind == failureProcessCleanup {
				return nil // The whole run must wait for process cleanup.
			}
		}
		if report.Retryable() {
			retryable = append(retryable, report)
		}
	}
	return retryable
}

// renderAgentAttention prints the plain-language summary of agents that did not
// succeed. Technical detail is only pointed to, never printed.
func renderAgentAttention(writer io.Writer, reports []agentReport, logPath, retryHint string) error {
	failed := failedAgents(reports)
	if len(failed) == 0 {
		return nil
	}
	theme := terminaltheme.For(writer)
	if _, err := fmt.Fprintf(writer, "\n%s\n", theme.Text(terminaltheme.Warning, "Needs attention")); err != nil {
		return err
	}
	for _, report := range failed {
		if _, err := fmt.Fprintf(writer, "\n%s\n", theme.Text(terminaltheme.Label, prompt.SafeText(report.Label))); err != nil {
			return err
		}
		for _, problem := range report.Problems {
			if _, err := fmt.Fprintf(writer, "  %s\n", prompt.SafeText(problem.Failure.Summary+itemsSuffix(problem.Items))); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "  Next: %s\n", prompt.SafeText(problem.Failure.Next)); err != nil {
				return err
			}
		}
	}
	if retryHint != "" {
		if _, err := fmt.Fprintf(writer, "\n%s\n", prompt.SafeText(retryHint)); err != nil {
			return err
		}
	}
	if logPath != "" {
		if _, err := fmt.Fprintf(writer, "Technical details: %s\n", prompt.SafeText(logPath)); err != nil {
			return err
		}
	}
	return nil
}

func itemsSuffix(items []string) string {
	if len(items) == 0 || len(items) > 3 {
		return ""
	}
	return " (" + strings.Join(items, ", ") + ")"
}

// retryTargetHint tells a non-interactive caller how to retry only the agents
// a retry can help; empty when none can be helped.
func retryTargetHint(reports []agentReport) string {
	ids := make([]string, 0, len(reports))
	for _, report := range retryableAgents(reports) {
		ids = append(ids, publicCLITargetName(report.ClientID))
	}
	if len(ids) == 0 {
		return ""
	}
	return "To retry only these agents, run the same command with --target " + strings.Join(ids, ",")
}

// retryHintFor words the non-interactive retry advice for the running command:
// `add` can narrow to failed agents, other verbs converge when run again.
func retryHintFor(cmd *cobra.Command, reports []agentReport) string {
	if len(retryableAgents(reports)) == 0 {
		return ""
	}
	if cmd.Name() == "add" {
		return retryTargetHint(reports)
	}
	return "To retry the agents that failed, run the same command again."
}

// installOutcome is the finished result of one apply, kept on the command's
// options so renderers and the retry step share it.
type installOutcome struct {
	Agents  []agentReport
	Items   []agentItem
	LogFile string
}

// bindingJSON is the machine-readable per-binding result, so a consumer can tell
// which package failed on which agent without reading the log.
type bindingJSON struct {
	Alias    string `json:"alias"`
	ClientID string `json:"client_id"`
	Status   string `json:"status"`
	Code     string `json:"code,omitempty"`
}

func (outcome *installOutcome) bindingsJSON() []bindingJSON {
	out := make([]bindingJSON, 0, len(outcome.Items))
	for _, item := range outcome.Items {
		entry := bindingJSON{Alias: item.Alias, ClientID: item.ClientID, Status: item.Status}
		if item.failure != nil {
			entry.Code = string(item.failure.Kind)
		}
		out = append(out, entry)
	}
	return out
}

func (outcome *installOutcome) failedAgents() []agentReport {
	if outcome == nil {
		return nil
	}
	return failedAgents(outcome.Agents)
}

// finishInstall folds binding outcomes into per-agent reports and records the
// technical log when anything did not succeed.
func (app App) finishInstall(opts *options, command string, items []agentItem) *installOutcome {
	outcome := &installOutcome{Agents: reportByAgent(items), Items: items}
	outcome.LogFile = app.recordInstallRun(command, 1+opts.retryAttempt, agentLogEntries(items))
	opts.installResult = outcome
	return outcome
}

// recordInstallRun saves the technical record of a run that did not fully
// succeed and returns its path. A logging problem must never change an
// installation's result, so failures here are swallowed.
func (app App) recordInstallRun(command string, attempt int, entries []installlog.Entry) string {
	if app.InstallLogDir == "" {
		return ""
	}
	run := installlog.Run{Command: command, Version: app.Version, Scope: string(app.scope()), Attempt: attempt, Entries: entries}
	if !run.Failed() {
		return ""
	}
	path, err := installlog.Write(app.InstallLogDir, run, time.Now())
	if err != nil {
		return ""
	}
	return path
}

func agentLogEntries(items []agentItem) []installlog.Entry {
	entries := make([]installlog.Entry, 0, len(items))
	for _, item := range items {
		entry := installlog.Entry{Alias: item.Alias, Client: item.ClientID, Status: item.Status}
		if item.failure != nil {
			entry.Code, entry.Summary, entry.Detail = string(item.failure.Kind), item.failure.Summary, item.failure.Detail
		}
		entries = append(entries, entry)
	}
	return entries
}

// groupAgentItems turns a plugin group's per-agent results into report items.
// Presentation kinds that a retry cannot fix (setup or sign-in required,
// user-attested) are successes of the installation itself.
func groupAgentItems(alias string, targets []addTargetResult) []agentItem {
	items := make([]agentItem, 0, len(targets))
	for _, target := range targets {
		item := agentItem{Alias: alias, ClientID: target.Target, Status: "completed"}
		switch classifyBatchPresentation(target) {
		case batchPresentationFailed, batchPresentationRolledBack:
			item.Status = "failed"
			item.failure = groupFailure(target, failureUnknown)
		case batchPresentationNotCompleted:
			item.Status = "not_attempted"
			item.failure = groupFailure(target, failureSkipped)
		}
		items = append(items, item)
	}
	return items
}

func groupFailure(target addTargetResult, kind failureKind) *installFailure {
	failure := installFailure{Kind: kind}
	if target.Error != nil {
		failure.Detail = target.Error.Stage + ": " + target.Error.Message
		// A state-save failure needs recovery; retrying blindly could compound it.
		if batchFailureNeedsDoctor(target) {
			failure.Kind = failureRecovery
		}
		if target.Error.Stage == "cleanup" {
			failure.Kind = failureProcessCleanup
		}
	}
	failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	return &failure
}

// recordGroupOutcome stores a plugin group's per-agent result for the retry
// step and the technical log. The group's own table stays the visible summary.
func (app App) recordGroupOutcome(cmd *cobra.Command, opts *options, combined *addMultiResult, cause error) error {
	items := groupAgentItems(combined.Plugin, combined.Targets)
	unclosed := domain.IsUnclosedClientProbe(cause)
	if unclosed {
		// An unconfirmed process stops retries for this whole failed group.
		// Preserve the original detail for the log before making the public
		// per-target messages safe; successful targets remain unchanged.
		for index := range items {
			item := &items[index]
			if item.failure == nil {
				continue
			}
			status := environment.BindingFailed
			if item.Status == "not_attempted" {
				status = environment.BindingNotAttempted
			}
			detail := item.failure.Detail
			item.failure = describeBindingFailure(environment.BindingOutcome{Status: status, Err: cause})
			if detail != "" {
				item.failure.Detail = detail + "\n" + item.failure.Detail
			}
			target := &combined.Targets[index]
			target.Error = &usecase.GroupTargetFailure{Stage: "cleanup", Message: item.failure.Summary + " " + item.failure.Next}
			target.Output.Result.Failure = target.Error
			target.RetryCommand = ""
			target.NextAction = item.failure.Next
			target.Output.NextAction = item.failure.Next
		}
		cause = installFailureWithCause{summary: describeSyncFailure(cause).Summary, cause: cause}
	}
	// A group refused before any client was touched has nothing to retry: the
	// same preflight would refuse again, and the person is told why already.
	switch combined.Status {
	case string(usecase.GroupPhasePlanned), string(usecase.GroupPhasePreparationFailed):
		return cause
	}
	outcome := app.finishInstall(opts, "add", items)
	if outcome.LogFile != "" && opts.format == "human" {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Technical details: %s\n", prompt.SafeText(outcome.LogFile))
	}
	return cause
}

// repairOwnedDependencies repairs one owned installation and, in an interactive
// terminal, offers to run the repair again when some agents failed. Each run
// plans from current state, so a retry never replays stale actions.
func repairOwnedDependencies(cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) (bool, error) {
	var handled bool
	var run func() error
	run = func() error {
		opts.installResult = nil
		attemptHandled, err := repairOwnedDependenciesOnce(cmd, app, opts, selector, targets)
		if handled && !attemptHandled && err == nil {
			return installer.ErrPlanChanged
		}
		handled = handled || attemptHandled
		return retryConverge(cmd, app, opts, err, run)
	}
	err := run()
	return handled, err
}
