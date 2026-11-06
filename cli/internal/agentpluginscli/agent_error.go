package agentpluginscli

import (
	"context"
	"errors"
	"io"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// AgentAction is actionable guidance, not proof of native activation.
type AgentAction struct {
	Code    string `json:"code"`
	Actor   string `json:"actor"`
	Message string `json:"message"`
}

// CLIUsageError carries a fixed safe diagnostic without re-parsing stderr.
type CLIUsageError struct {
	Code    string
	Message string
}

func (e CLIUsageError) Error() string { return e.Message }

// AgentFailure deliberately never serializes raw native/transport errors.
type AgentFailure struct {
	Code        string        `json:"code"`
	Message     string        `json:"message"`
	Retryable   bool          `json:"retryable"`
	NextActions []AgentAction `json:"next_actions,omitempty"`
}

func classifyAgentFailure(err error) AgentFailure {
	var usage CLIUsageError
	if errors.As(err, &usage) {
		return AgentFailure{Code: usage.Code, Message: usage.Message}
	}
	var issue skillsSelectionError
	if errors.As(err, &issue) {
		return AgentFailure{Code: issue.Code, Message: issue.Message}
	}
	switch {
	case errors.Is(err, context.Canceled):
		return AgentFailure{Code: "CANCELED", Message: "The operation was canceled. Inspect the selected environment before retrying."}
	case errors.Is(err, context.DeadlineExceeded):
		return AgentFailure{Code: "TIMEOUT", Message: "A source or client operation timed out; inspect its availability before retrying.", Retryable: true}
	case errors.Is(err, installer.ErrRecoveryRequired):
		return AgentFailure{Code: "RECOVERY_REQUIRED", Message: "Inspect the selected dependency environment and its pending recovery before another mutation.", NextActions: []AgentAction{{Code: "inspect_recovery", Actor: "agent", Message: "Run agentplugins deps status with the same --scope and --project-root."}}}
	case errors.Is(err, installer.ErrAssessmentRejected):
		return AgentFailure{Code: "SECURITY_REVIEW_BLOCKED", Message: "Automated security assessment rejected the candidate; inspect the findings rather than bypassing them."}
	case errors.Is(err, installer.ErrForeignSkillDestination):
		return AgentFailure{Code: "FOREIGN_DESTINATION", Message: "An unmanaged destination conflicts with installation; preserve or manually resolve the foreign entry."}
	case errors.Is(err, installer.ErrPlanChanged), errors.Is(err, ErrSkillsPlanStale):
		return AgentFailure{Code: "PLAN_CHANGED", Message: "The reviewed environment changed; obtain a new dry-run plan.", Retryable: true}
	case errors.Is(err, installer.ErrTargetFactsUnavailable):
		return AgentFailure{Code: "TARGET_UNAVAILABLE", Message: "The selected client could not be verified; inspect client detection before applying."}
	case errors.Is(err, installer.ErrUnsupported), errors.Is(err, installer.ErrIncomplete):
		return AgentFailure{Code: "TARGET_UNSUPPORTED", Message: "The selected client cannot safely install the required components."}
	default:
		return AgentFailure{Code: "COMMAND_FAILED", Message: "The command did not complete. Inspect stderr and run agentplugins doctor for the same scope."}
	}
}

// ShouldWriteAgentFailure opts in only early usage errors. Source selection
// errors already emit their own JSON; old native refusals require empty stdout.
func ShouldWriteAgentFailure(err error) bool {
	var usage CLIUsageError
	return errors.As(err, &usage)
}

// WriteAgentFailure is called only when the command has not already emitted a
// JSON document. The caller must keep the nonzero exit status and stderr.
func WriteAgentFailure(writer io.Writer, err error) error {
	return writeJSONResult(writer, "agentplugins", outputResultFailure, map[string]any{
		"error": classifyAgentFailure(err),
	})
}
