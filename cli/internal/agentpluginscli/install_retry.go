package agentpluginscli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Interactive retries preserve the first reviewed package and the original
// failure. An unclosed native process stops every retry route before prompting.
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

func declinedInstallResult(opts *options, renderErr error) error {
	if opts.retryAttempt > 0 {
		return errors.Join(renderErr, installer.ErrCancelled)
	}
	return renderErr
}

func runSingleTargetAdd(app App, opts *options, loaded loadedPackage, target domain.ClientID, completing bool, run func(*options) error) (bool, error) {
	selected := *opts
	selected.target = string(target)
	if !completing && !hasSingleTargetBinding(app, loaded, target) {
		return false, nil
	}
	err := run(&selected)
	opts.installResult = selected.installResult
	return true, err
}

// This probe selects the existing one-agent path. If state cannot be observed,
// the ordinary group path retains responsibility for reporting the load error.
func hasSingleTargetBinding(app App, loaded loadedPackage, target domain.ClientID) bool {
	state, err := app.StateStore.Load()
	if err != nil {
		return false
	}
	installation, ok := locallyMatchedInstallation(state, loaded.envelope.Manifest.Name)
	return ok && installationHasTarget(installation, target, string(app.scope()))
}

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

func retryAllowed(cmd *cobra.Command, app App, opts *options, err error) bool {
	return err != nil && !unclosedInstallProbe(err) && app.Terminal && opts.format == "human" && !opts.yes && !opts.dryRun &&
		opts.retryAttempt < maxSkillsRetries && cmd.Context().Err() == nil && opts.installResult != nil
}

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
