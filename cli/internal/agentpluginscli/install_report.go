package agentpluginscli

import (
	"errors"
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
	}
	failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	return &failure
}

// retryFailedAgents offers to run the same command again for only the agents
// that failed and a retry can help. It never runs without a person present. The
// retried command resolves its source exactly as typing it again would and
// repeats its normal review and confirmation.
//
// A retry can fix only part of a run, so the result stays an error while any
// agent from an earlier attempt is still unresolved (not retryable, or
// deselected): success of the last attempt alone must not turn into exit 0.
func retryFailedAgents(cmd *cobra.Command, app App, opts *options, err error, rerun func(target string, unresolved []string) error) error {
	if !retryAllowed(cmd, app, opts, err) {
		return err
	}
	targets, askErr := askRetryTargets(cmd, app, opts, opts.installResult.Agents)
	if errors.Is(askErr, prompt.ErrPromptCanceled) {
		return err
	}
	if askErr != nil {
		return errors.Join(err, askErr)
	}
	if len(targets) == 0 {
		return err
	}
	public := make([]string, len(targets))
	selected := make(map[domain.ClientID]bool, len(targets))
	for index, target := range targets {
		public[index] = publicCLITargetName(domain.ClientID(target))
		selected[domain.ClientID(target)] = true
	}
	pending := append([]string(nil), opts.unresolved...)
	for _, report := range failedAgents(opts.installResult.Agents) {
		if !selected[report.ClientID] {
			pending = append(pending, report.Label)
		}
	}
	if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "Retrying %d agent(s)…\n", len(targets)); writeErr != nil {
		return errors.Join(err, writeErr)
	}
	rerr := rerun(strings.Join(public, ","), pending)
	if rerr != nil {
		return errors.Join(err, rerr)
	}
	if len(pending) > 0 {
		return fmt.Errorf("%s still need attention; see the technical log", strings.Join(pending, ", "))
	}
	return nil
}

// declinedInstallResult distinguishes abandoning a retry from a completed
// retry. The outer retry boundary keeps the original installation failure.
func declinedInstallResult(opts *options, renderErr error) error {
	if opts.retryAttempt > 0 {
		return errors.Join(renderErr, installer.ErrCancelled)
	}
	return renderErr
}

// recordGroupOutcome stores a plugin group's per-agent result for the retry
// step and the technical log. The group's own table stays the visible summary.
func (app App) recordGroupOutcome(cmd *cobra.Command, opts *options, combined addMultiResult) {
	outcome := app.finishInstall(opts, "add", groupAgentItems(combined.Plugin, combined.Targets))
	if outcome.LogFile != "" && opts.format == "human" {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Technical details: %s\n", prompt.SafeText(outcome.LogFile))
	}
}

// runSingleTargetAdd dispatches an existing binding or explicit completion to
// the one-agent path and hands its result back, so the retry step sees it.
func runSingleTargetAdd(app App, opts *options, loaded loadedPackage, target domain.ClientID, completing bool, run func(*options) error) (bool, error) {
	selected := *opts
	selected.target = string(target)
	if !completing {
		state, err := app.StateStore.Load()
		if err != nil {
			return false, nil
		}
		installation, ok := locallyMatchedInstallation(state, loaded.envelope.Manifest.Name)
		if !ok || !installationHasTarget(installation, target, string(app.scope())) {
			return false, nil
		}
	}
	err := run(&selected)
	opts.installResult = selected.installResult
	return true, err
}

// recordSingleAdd stores the result of a one-agent add for the retry step and
// the technical log. Success needs no record.
func (app App) recordSingleAdd(cmd *cobra.Command, opts *options, client domain.ClientID, alias string, err error) {
	if err == nil {
		return
	}
	item := agentItem{Alias: alias, ClientID: string(client), Status: "failed", failure: describeSyncFailure(err)}
	outcome := app.finishInstall(opts, "add", []agentItem{item})
	if outcome.LogFile != "" && opts.format == "human" {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Technical details: %s\n", prompt.SafeText(outcome.LogFile))
	}
}

// retryAllowed gates every interactive retry: only with a failure, a person
// present, human output and the retry budget left.
func retryAllowed(cmd *cobra.Command, app App, opts *options, err error) bool {
	return err != nil && app.Terminal && opts.format == "human" && !opts.yes && !opts.dryRun &&
		opts.retryAttempt < maxSkillsRetries && cmd.Context().Err() == nil && opts.installResult != nil
}

// retryConverge offers to run a converging command (install, repair) again. It
// has no per-agent narrowing: every outstanding binding is retried together, so
// nothing from an earlier attempt can stay unresolved behind a green result.
func retryConverge(cmd *cobra.Command, app App, opts *options, err error, rerun func() error) error {
	if !retryAllowed(cmd, app, opts, err) {
		return err
	}
	retryable := retryableAgents(opts.installResult.Agents)
	if len(retryable) == 0 {
		return err
	}
	app, promptErr := withPrompter(cmd, app, opts)
	if errors.Is(promptErr, prompt.ErrPromptCanceled) {
		return err
	}
	if promptErr != nil {
		return errors.Join(err, promptErr)
	}
	labels := make([]string, len(retryable))
	for index, report := range retryable {
		labels[index] = report.Label
	}
	answer, askErr := app.Prompter.Confirm(cmd.Context(), prompt.ConfirmationRequest{Title: "Retry " + strings.Join(labels, ", ") + "?", Default: true})
	if errors.Is(askErr, prompt.ErrPromptCanceled) {
		return err
	}
	if askErr != nil {
		return errors.Join(err, askErr)
	}
	if !answer.Accepted {
		return err
	}
	opts.retryAttempt++
	if _, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "Retrying %d agent(s)…\n", len(retryable)); writeErr != nil {
		return errors.Join(err, writeErr)
	}
	if retryErr := rerun(); retryErr != nil {
		return errors.Join(err, retryErr)
	}
	return nil
}

// pinRetryPackage remembers the package the first attempt resolved and refuses a
// retry that would install different bytes, such as a local folder edited in
// between: the person reviewed the first package, not the changed one.
func pinRetryPackage(opts *options, loaded loadedPackage) error {
	digest := loaded.envelope.TreeDigest
	if digest == "" {
		return nil
	}
	if opts.firstTreeDigest == "" {
		opts.firstTreeDigest = digest
		return nil
	}
	if opts.firstTreeDigest != digest {
		return fmt.Errorf("the package changed since the first attempt; run the command again to review the new version")
	}
	return nil
}
