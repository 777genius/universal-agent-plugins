package agentpluginscli

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/installlog"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// installDiagnostic observes recording only. A write error never changes the
// installation outcome and is never formatted as terminal/provider output.
type installDiagnostic struct {
	Path     string
	WriteErr error
}

// recordInstallRun retains both the saved path and recording failure. An empty
// directory disables recording; successful attempts create no diagnostic file.
func (app App) recordInstallRun(command string, attempt int, entries []installlog.Entry) installDiagnostic {
	if app.InstallLogDir == "" {
		return installDiagnostic{}
	}
	run := installlog.Run{Command: command, Version: app.Version, Scope: string(app.scope()), Attempt: attempt, Entries: entries}
	if !run.Failed() {
		return installDiagnostic{}
	}
	path, err := installlog.Write(app.InstallLogDir, run, time.Now())
	return installDiagnostic{Path: path, WriteErr: err}
}

// renderInstallDiagnostic uses fixed classified facts when there is no private
// file. Heuristic credential redaction cannot establish that raw errors are safe
// to publish, so neither the installation detail nor the write error is printed.
func renderInstallDiagnostic(writer io.Writer, diagnostic installDiagnostic, reports []agentReport) error {
	if len(failedAgents(reports)) == 0 {
		return nil
	}
	writer = &planWriter{writer: writer}
	if diagnostic.Path != "" {
		_, err := fmt.Fprintf(writer, "Technical details: %s\n", prompt.SafeText(diagnostic.Path))
		return err
	}
	if _, err := fmt.Fprintln(writer, diagnosticUnavailableMessage(diagnostic)); err != nil {
		return err
	}
	// Codes remain useful when an unfamiliar provider failure cannot safely be
	// explained in more detail. They come from the existing classification only.
	codes, seen := []string{}, map[failureKind]bool{}
	for _, report := range reports {
		for _, problem := range report.Problems {
			kind := problem.Failure.Kind
			if !seen[kind] {
				seen[kind] = true
				codes = append(codes, string(kind))
			}
		}
	}
	_, err := fmt.Fprintln(writer, "Failure codes:", strings.Join(codes, ", "))
	return err
}

// Group and single-add tables retain their established presentation. If no
// technical file exists, their notice also supplies the safe classified cause.
func renderInstallRecord(writer io.Writer, outcome *installOutcome) error {
	if outcome.LogFile == "" {
		if err := renderAgentAttention(writer, outcome.Agents, "", ""); err != nil {
			return err
		}
	}
	return renderInstallDiagnostic(writer, installDiagnostic{Path: outcome.LogFile, WriteErr: outcome.logWriteErr}, outcome.Agents)
}

func diagnosticUnavailableMessage(diagnostic installDiagnostic) string {
	if diagnostic.WriteErr == nil {
		return "Technical details were not recorded for this attempt. Follow the per-agent results and next steps."
	}
	switch failureKindOf(diagnostic.WriteErr) {
	case failurePermission:
		return "Technical details could not be saved because UAP cannot write to the log folder."
	case failureDiskFull:
		return "Technical details could not be saved because there is not enough free disk space."
	case failureReadOnly:
		return "Technical details could not be saved because the log folder is read-only."
	}
	return "Technical details could not be saved. Follow the per-agent results and next steps."
}

// Human apply errors point to results whose detail is safe even when a log was
// unavailable. JSON and callers without reported binding failures keep their
// existing error. The original typed cause always remains available to Unwrap.
func humanInstallApplyError(format string, outcome *installOutcome, cause error) error {
	if format != "human" || cause == nil || outcome == nil || len(outcome.failedAgents()) == 0 {
		return cause
	}
	return installFailureWithCause{summary: "Installation did not complete. Follow the per-agent results and next steps.", cause: cause}
}

// A broken result stream must not start an automatic retry whose consequences
// could no longer be shown. The underlying output error remains classifiable.
var errInstallOutput = errors.New("installation results could not be fully displayed")

func installOutputError(err error) error {
	if err == nil {
		return nil
	}
	return installFailureWithCause{summary: errInstallOutput.Error(), cause: errors.Join(errInstallOutput, err)}
}

// Aggregate group errors remain the outward classification. Preserve output
// failure and native causes behind that summary when a failed stream joins them.
func retainGroupApplyCause(summary, cause error) error {
	if errors.Is(cause, errInstallOutput) {
		return installFailureWithCause{summary: summary.Error(), cause: cause}
	}
	return retainUnclosedInstallCause(summary, cause)
}

// A failed native result may copy the provider error into its reason, next
// actions and manual instructions. With no private log, its detached human view
// uses fixed classification only. JSON/native facts and healthy peers survive.
func dependencyHumanResult(opts *options, result dependencyResultDTO) dependencyResultDTO {
	outcome := opts.installResult
	if outcome != nil && outcome.LogFile != "" {
		return result
	}
	if result.bindingStatus == environment.BindingApplied || result.bindingStatus == environment.BindingUnchanged {
		return result
	}
	failure := result.bindingFailure
	if result.bindingStatus == "" {
		if outcome == nil {
			return result
		}
		failure = pairedDependencyFailure(outcome, result)
		failed := result.Outcome != string(installer.OutcomeCompleted) && result.Outcome != string(installer.OutcomeUnchanged)
		if failure == nil && !failed {
			return result
		}
	}
	if failure == nil {
		kind := failureKindOfOutcome(installer.Outcome(result.Outcome))
		failure = &installFailure{Kind: kind}
		failure.Summary, failure.Next, failure.Retryable = applyFailureWording(kind)
	}
	result.Reason = failure.Summary
	if result.Outcome == string(installer.OutcomeCompleted) || result.Outcome == string(installer.OutcomeUnchanged) {
		result.Outcome = "failed"
	}
	result.NextActions, result.ManualActions = nil, nil
	return result
}

// Missing or ambiguous native identity cannot choose another binding's cause.
func pairedDependencyFailure(outcome *installOutcome, result dependencyResultDTO) *installFailure {
	if result.InstallationID == "" || result.ClientID == "" {
		return nil
	}
	var failure *installFailure
	matches := 0
	anyFailed := false
	for _, item := range outcome.Items {
		if item.installationID == result.InstallationID && item.ClientID == result.ClientID {
			matches++
			failure = normalizeAgentItem(item).failure
			anyFailed = anyFailed || failure != nil
		}
	}
	if matches == 1 {
		return failure
	}
	if anyFailed {
		return describeSyncFailure(nil)
	}
	return nil
}

// A missing private log must not expose the same provider chain through the
// group's existing table. Keep stage identity and successful/manual peers;
// replace only failed human presentation fields after recording was attempted.
func protectUnrecordedGroupDetails(combined *addMultiResult, items []agentItem) {
	for index, item := range items {
		if item.failure == nil {
			continue
		}
		target := &combined.Targets[index]
		failure := usecase.GroupTargetFailure{}
		if target.Error != nil {
			failure = *target.Error
		}
		failure.Message = item.failure.Summary + " " + item.failure.Next
		target.Error = &failure
		target.Output.Result.Failure = &failure
		target.NextAction, target.Output.NextAction = item.failure.Next, item.failure.Next
	}
}
