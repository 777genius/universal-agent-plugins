package agentpluginscli

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func enterprisePolicyAgentFailure(err error) (AgentFailure, bool) {
	var denied *domain.InstallPolicyError
	var config *enterprisepolicy.ConfigError
	switch {
	case errors.As(err, &denied):
		return AgentFailure{Code: "POLICY_DENIED", Message: "Installation policy denied this operation.",
			Violations: safeEnterprisePolicyViolations(denied.Violations),
			NextActions: []AgentAction{{Code: "inspect_policy", Actor: "agent", Message: "Run agentplugins policy status with the same scope and project root to inspect the named rules."},
				{Code: "resolve_policy", Actor: "human", Message: "Choose an allowed package or ask the policy owner to update the rule. Confirmation and security override flags do not bypass installation policy."}}}, true
	case errors.As(err, &config):
		return AgentFailure{Code: "POLICY_INVALID", Message: "Installation policy is unavailable, unsafe or invalid. Repair the named file before retrying.",
			PolicyPath: safeEnterprisePolicyText(config.Path), PolicyLayer: safeEnterprisePolicyText(config.Layer),
			NextActions: []AgentAction{{Code: "validate_policy", Actor: "agent", Message: "Run agentplugins policy validate for the named file, then policy status with the same scope and project root."}}}, true
	case errors.Is(err, domain.ErrInstallPolicyChanged):
		return AgentFailure{Code: "POLICY_CHANGED", Message: "Installation policy changed after inspection. Prepare a fresh operation against the current rules.", Retryable: true,
			NextActions: []AgentAction{{Code: "review_policy", Actor: "agent", Message: "Run agentplugins policy status and prepare a new review before applying the operation again."}}}, true
	default:
		return AgentFailure{}, false
	}
}

// A host may inject its own checker. Only bounded labels and application-owned
// guidance are public; custom checker messages can contain transport secrets.
func safeEnterprisePolicyViolations(violations []domain.InstallPolicyViolation) []domain.InstallPolicyViolation {
	result := make([]domain.InstallPolicyViolation, 0, len(violations))
	for _, violation := range violations {
		result = append(result, domain.InstallPolicyViolation{
			Code: safeEnterprisePolicyCode(violation.Code), Layer: safeEnterprisePolicyText(violation.Layer),
			PolicyPath: safeEnterprisePolicyText(violation.PolicyPath), Subject: safeEnterprisePolicyText(violation.Subject),
			Message: "Blocked by the named installation policy rule.",
		})
	}
	return result
}

func safeEnterprisePolicyCode(value string) string {
	if value == "" || len(value) > 128 {
		return "policy_rule"
	}
	for _, ch := range value {
		allowed := ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '.' || ch == '_' || ch == '-'
		if !allowed {
			return "policy_rule"
		}
	}
	return value
}

func safeEnterprisePolicyText(value string) string {
	value = domain.PolicySafeText(safeDiagnosticText(value))
	if strings.Contains(value, "://") {
		return "[redacted URL]"
	}
	return value
}

func enterprisePolicyFailureText(failure AgentFailure) string {
	message := failure.Message
	if len(failure.Violations) > 0 {
		first := failure.Violations[0]
		message += " Rule: " + first.Code + "."
		if first.PolicyPath != "" {
			message += " Policy: " + first.PolicyPath + "."
		}
		if len(failure.Violations) > 1 {
			message += fmt.Sprintf(" %d rule violations were reported.", len(failure.Violations))
		}
	} else if failure.PolicyPath != "" {
		message += " Policy: " + failure.PolicyPath + "."
	}
	return message + " Run agentplugins policy status with the same scope and project root."
}

type enterprisePolicyPublicError struct {
	refusal *management.Refusal
	cause   error
}

func (err *enterprisePolicyPublicError) Error() string   { return err.refusal.Message }
func (err *enterprisePolicyPublicError) Unwrap() []error { return []error{err.refusal, err.cause} }

func managementEnterprisePolicyError(err error) error {
	failure, ok := enterprisePolicyAgentFailure(err)
	if !ok {
		return err
	}
	status := http.StatusUnprocessableEntity
	if failure.Retryable {
		status = http.StatusConflict
	}
	return &enterprisePolicyPublicError{cause: err, refusal: &management.Refusal{
		Code: failure.Code, Message: enterprisePolicyFailureText(failure), Retryable: failure.Retryable, Status: status,
	}}
}

func managementEnterprisePolicyOutcome(out *management.Outcome, err error) bool {
	failure, ok := enterprisePolicyAgentFailure(err)
	if !ok {
		return false
	}
	out.Result.Message = enterprisePolicyFailureText(failure)
	out.Diagnostics = append(out.Diagnostics, management.Diagnostic{Code: failure.Code, Message: out.Result.Message, Severity: management.DiagnosticSeverityError})
	for _, action := range failure.NextActions {
		out.Result.NextActions = append(out.Result.NextActions, management.NextAction{Kind: action.Code, Reason: action.Message})
	}
	return true
}
