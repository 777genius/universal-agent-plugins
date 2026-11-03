package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

var errDependencyCheckFailed = errors.New("local dependency integrity was not established; inspect the check report")

type dependencyCheckReport struct {
	SchemaVersion int                     `json:"schema_version"`
	Scope         string                  `json:"scope"`
	ManifestPath  string                  `json:"manifest_path,omitempty"`
	LockPath      string                  `json:"lock_path,omitempty"`
	Outcome       dependencyCheckOutcome  `json:"outcome"`
	Checks        []dependencyCheckResult `json:"checks"`
}

func newDependencyCheckCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "check", Short: "Check local agreement and cached package integrity without fetching or changing files",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDepsOptions(opts); err != nil {
				return err
			}
			report := dependencyCheckReport{SchemaVersion: 1, Scope: opts.scope, Outcome: dependencyCheckValid, Checks: []dependencyCheckResult{}}
			if cmd.Context().Err() != nil {
				report.add(dependencyCanceledResult("scope"))
				return finishDependencyCheck(cmd, opts, report)
			}
			selected, err := app.forOptions(opts)
			if err != nil {
				report.add(dependencyCheckResult{Stage: "scope", Outcome: dependencyCheckError, Code: "scope_unavailable", Message: "The selected scope is unavailable or unsafe. Check --scope and --project-root before inspecting its agreement."})
				return finishDependencyCheck(cmd, opts, report)
			}
			report = inspectLocalDependencies(cmd.Context(), selected)
			return finishDependencyCheck(cmd, opts, report)
		},
	}
}

func inspectLocalDependencies(ctx context.Context, app App) dependencyCheckReport {
	manifest, lock := app.dependencyPaths()
	report := dependencyCheckReport{SchemaVersion: 1, Scope: string(app.scope()), ManifestPath: manifest, LockPath: lock, Outcome: dependencyCheckValid, Checks: []dependencyCheckResult{}}
	if ctx.Err() != nil {
		report.add(dependencyCanceledResult("scope"))
		return report
	}
	report.add(dependencyCheckResult{Stage: "scope", Outcome: dependencyCheckValid})
	agreement, checks := inspectDependencyAgreement(ctx, app)
	for _, check := range checks {
		report.add(check)
	}
	if report.Outcome != dependencyCheckValid {
		return report
	}
	bySource := make(map[string]environment.LockedPackage, len(agreement.lock.Packages))
	for _, pin := range agreement.lock.Packages {
		bySource[pin.SourceKey] = pin
	}
	packages := append([]environment.LockedPackage(nil), agreement.lock.Packages...)
	sort.Slice(packages, func(i, j int) bool { return packages[i].Alias < packages[j].Alias })
	for _, pin := range packages {
		if ctx.Err() != nil {
			report.add(dependencyCanceledResult("package"))
			break
		}
		// This helper only reads a verified store snapshot. In particular, do not
		// use readReplayDependency, which fetches exact sources on a cache miss.
		resolved, err := describePinnedDependency(ctx, app, pin)
		if err != nil {
			report.add(dependencyPackageCheckFailure(pin.Alias, err))
			continue
		}
		report.add(dependencyCheckResult{Stage: "package", Alias: pin.Alias, Outcome: dependencyCheckValid})
		if err := verifyDeclaredEdges(pin.Alias, resolved.Dependencies, bySource); err != nil {
			report.add(dependencyCheckResult{Stage: "graph", Alias: pin.Alias, Outcome: dependencyCheckInvalid, Code: "graph_declarations_mismatch", Message: "Recorded dependency edges differ from this exact cached package's declarations. Preserve the agreement and review it before explicitly resolving a replacement lock."})
		} else {
			report.add(dependencyCheckResult{Stage: "graph", Alias: pin.Alias, Outcome: dependencyCheckValid})
		}
	}
	if report.Outcome == dependencyCheckValid {
		if issue := agreement.recheck(ctx); issue != nil {
			report.add(*issue)
		}
	}
	return report
}

func dependencyPackageCheckFailure(alias string, err error) dependencyCheckResult {
	issue := dependencyCheckResult{Stage: "package", Alias: alias, Outcome: dependencyCheckInvalid, Code: "package_invalid", Message: "The exact cached package failed local manifest or pin validation. Preserve the agreement and use verified source bytes for explicit recovery."}
	var pathError *os.PathError
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		issue = dependencyCanceledResult("package")
		issue.Alias = alias
	case errors.Is(err, environmentstore.ErrMissing):
		issue.Outcome, issue.Code = dependencyCheckUnavailable, "package_unavailable"
		issue.Message = "Exact package bytes are not cached. This offline check did not fetch them and cannot establish their integrity."
	case errors.Is(err, environmentstore.ErrMismatch):
		issue.Code = "store_mismatch"
		issue.Message = "Cached metadata or payload disagrees with its immutable digest or executable inventory. Inspect it before explicit recovery."
	case errors.As(err, &pathError):
		issue.Outcome, issue.Code = dependencyCheckError, "store_unreadable"
		issue.Message = "Cached package bytes could not be inspected because of a filesystem error; this is not evidence that the package is absent."
	}
	return issue
}

func (report *dependencyCheckReport) add(check dependencyCheckResult) {
	report.Checks = append(report.Checks, check)
	if dependencyCheckRank(check.Outcome) > dependencyCheckRank(report.Outcome) {
		report.Outcome = check.Outcome
	}
}

func dependencyCheckRank(outcome dependencyCheckOutcome) int {
	switch outcome {
	case dependencyCheckCanceled:
		return 4
	case dependencyCheckError:
		return 3
	case dependencyCheckInvalid:
		return 2
	case dependencyCheckUnavailable:
		return 1
	default:
		return 0
	}
}
