package process

import "errors"

// ErrOrdinaryForcedCleanup identifies an ordinary command whose live descendants
// required forced cleanup. The command remains failed even after successful reap.
var ErrOrdinaryForcedCleanup = errors.New("process left live descendants that required forced cleanup")

const (
	ordinarySupervisorRequestDecode     = "request_decode"
	ordinarySupervisorSubreaperSetup    = "subreaper_setup"
	ordinarySupervisorPreflight         = "preflight"
	ordinarySupervisorCommandStart      = "command_start"
	ordinarySupervisorDescendantCleanup = "descendant_cleanup"
)

type ordinarySupervisorFailure struct {
	cause  error
	stage  string
	reason string
}

func (failure *ordinarySupervisorFailure) Error() string { return failure.cause.Error() }
func (failure *ordinarySupervisorFailure) Unwrap() error { return failure.cause }

// OrdinarySupervisorFailureClass returns a fixed category for a failure reported
// by the isolated Linux ordinary-command supervisor. It never reads error text,
// command arguments or streams. Unrelated errors on every platform return "".
func OrdinarySupervisorFailureClass(err error) string {
	var failure *ordinarySupervisorFailure
	if errors.As(err, &failure) && failure != nil {
		if failure.reason != "" && failure.stage != ordinarySupervisorDescendantCleanup {
			return ""
		}
		class := ordinarySupervisorStageClass(failure.stage)
		if failure.stage == ordinarySupervisorDescendantCleanup {
			if reason := ordinarySupervisorCleanupReason(failure.reason); reason != "" {
				return class + "_" + reason
			}
		}
		return class
	}
	return ""
}

func ordinarySupervisorStageClass(stage string) string {
	switch stage {
	case ordinarySupervisorRequestDecode:
		return "ordinary_supervisor_request_decode"
	case ordinarySupervisorSubreaperSetup:
		return "ordinary_supervisor_subreaper_setup"
	case ordinarySupervisorPreflight:
		return "ordinary_supervisor_preflight"
	case ordinarySupervisorCommandStart:
		return "ordinary_supervisor_command_start"
	case ordinarySupervisorDescendantCleanup:
		return "ordinary_supervisor_descendant_cleanup"
	default:
		return ""
	}
}

func ordinarySupervisorError(stage, message string) error {
	cause := errors.New(message)
	if ordinarySupervisorStageClass(stage) == "" {
		return cause
	}
	return &ordinarySupervisorFailure{cause: cause, stage: stage}
}

func ordinarySupervisorErrorWithReason(stage, reason, message string) error {
	if reason == "" {
		return ordinarySupervisorError(stage, message)
	}
	cause := errors.New(message)
	if stage != ordinarySupervisorDescendantCleanup {
		return cause
	}
	return &ordinarySupervisorFailure{cause: cause, stage: stage, reason: ordinarySupervisorCleanupReason(reason)}
}

func ordinarySupervisorCleanupReason(reason string) string {
	switch reason {
	case "exit_observation", "task_directory_read", "task_identity_unavailable", "task_children_observation",
		"child_identity_read", "child_identity_changed", "pidfd_open", "pidfd_identity_read", "pidfd_identity_changed",
		"parent_identity_read", "parent_changed", "tracked_identity_read", "tracked_identity_changed",
		"traversal_deadline", "traversal_limit", "descendant_signal", "cleanup_deadline":
		return reason
	default:
		return ""
	}
}
