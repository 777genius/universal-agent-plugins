package agentpluginscli

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// outcomeStatusReason is the single mapping from one binding outcome to the
// status word and reason that Skills, management and update presentations share.
// Position never matters: the outcome carries its own binding.
func outcomeStatusReason(outcome environment.BindingOutcome) (status, reason string) {
	switch outcome.Status {
	case environment.BindingUnchanged:
		return "unchanged", ""
	case environment.BindingNotAttempted:
		return "not_attempted", errorText(outcome.Err)
	}
	status = "failed"
	if outcome.Result != nil {
		if outcome.Result.Outcome != "" && outcome.Status == environment.BindingApplied {
			status = string(outcome.Result.Outcome)
		}
		reason = outcome.Result.Reason
		if outcome.Status == environment.BindingFailed && outcome.Result.Outcome != "" {
			status = string(outcome.Result.Outcome)
		}
	}
	if reason == "" {
		reason = errorText(outcome.Err)
	}
	return status, reason
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
