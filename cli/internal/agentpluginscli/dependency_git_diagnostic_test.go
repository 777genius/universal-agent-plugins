package agentpluginscli

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

// This fixture decorator exposes only fixed labels. OSRunner errors may contain
// argv and streams, so even a disposable repository's error text is never read.
type dependencyGitDiagnosticRunner struct {
	delegate sourceacquisition.Runner
	report   func(string, string, string)
	failures atomic.Uint32
}

func (runner *dependencyGitDiagnosticRunner) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	body, err := runner.delegate.Run(ctx, command)
	if err != nil && runner.failures.Add(1) <= 4 {
		class, status := dependencyGitDiagnosticClass(err)
		runner.report(dependencyGitDiagnosticPhase(command.Args), class, status)
	}
	return body, err
}

func dependencyGitDiagnosticPhase(args []string) string {
	if len(args) > 2 && args[0] == "-C" {
		args = args[2:]
	}
	if len(args) > 0 {
		switch args[0] {
		case "init", "remote", "config", "fetch", "rev-parse", "ls-tree", "sparse-checkout", "checkout", "ls-remote", "show", "check-ref-format":
			return args[0]
		}
	}
	return "unknown"
}

func dependencyGitDiagnosticClass(err error) (string, string) {
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
