package sourceacquisition

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
)

// This decorator belongs only to the native transport test. Operation labels
// come from its existing call boundaries, never from command arguments.
type nativeTransportDiagnosticRunner struct {
	delegate  Runner
	operation string
	report    func(string, string, string)
	failures  atomic.Uint32
}

func (runner *nativeTransportDiagnosticRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	body, err := runner.delegate.Run(ctx, command)
	if err != nil && runner.failures.Add(1) <= 4 {
		operation := runner.operation
		if operation != "resolve" && operation != "acquire" {
			operation = "unknown"
		}
		class, status := nativeTransportDiagnosticClass(err)
		runner.report(operation, class, status)
	}
	return body, err
}

func nativeTransportDiagnosticClass(err error) (string, string) {
	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled", "unavailable"
	case errors.Is(err, context.DeadlineExceeded):
		return "context_deadline", "unavailable"
	case errors.Is(err, processadapter.ErrOrdinaryContainmentUnsupported):
		return "ordinary_unsupported", "unavailable"
	case errors.Is(err, processadapter.ErrStdoutLimitExceeded):
		return "stdout_limit", "unavailable"
	case errors.Is(err, processadapter.ErrOrdinaryForcedCleanup):
		return "ordinary_forced_cleanup", "unavailable"
	}
	if class := processadapter.OrdinarySupervisorFailureClass(err); class != "" {
		return class, "unavailable"
	}
	var exit interface{ GitTransportExitCode() int }
	if errors.As(err, &exit) {
		return "git_exit", strconv.Itoa(exit.GitTransportExitCode())
	}
	return "other_git_or_process", "unavailable"
}
