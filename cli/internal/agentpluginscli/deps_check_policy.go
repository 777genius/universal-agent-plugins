package agentpluginscli

import (
	"context"
	"errors"
	"os"
)

// Extend the same offline integrity report with an opt-in local policy verdict.
// No second resolver, remote scan, or source acquisition is introduced.
func appendLocalPolicyAudit(ctx context.Context, app App, path string, report dependencyCheckReport) dependencyCheckReport {
	if path == "" || report.Outcome != dependencyCheckValid {
		return report
	}
	agreement, checks := inspectDependencyAgreement(ctx, app)
	for _, check := range checks {
		if check.Outcome != dependencyCheckValid {
			report.add(check)
		}
	}
	if report.Outcome != dependencyCheckValid {
		return report
	}
	_, err := evaluateLocalDependencyPolicy(ctx, app, path, agreement.lock)
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		report.add(dependencyCanceledResult("policy"))
	case err != nil:
		outcome, code := dependencyCheckInvalid, "policy_denied"
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
			outcome, code = dependencyCheckUnavailable, "policy_unavailable"
		}
		report.add(dependencyCheckResult{Stage: "policy", Outcome: outcome, Code: code,
			Message: "The explicit local policy did not approve this verified dependency closure. Review its source, pin and MCP rules. No files were changed."})
	default:
		report.add(dependencyCheckResult{Stage: "policy", Outcome: dependencyCheckValid})
	}
	if report.Outcome == dependencyCheckValid {
		if issue := agreement.recheck(ctx); issue != nil {
			report.add(*issue)
		}
	}
	return report
}
