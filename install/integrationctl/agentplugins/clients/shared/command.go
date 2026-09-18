package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// RunClientCommand runs one client CLI invocation and turns a missing runner, a
// start failure and a non-zero exit into errors named after the client, so an
// adapter never has to decide what "failed" means. The result is returned even
// on a non-zero exit: the output is often the only evidence available.
func RunClientCommand(ctx context.Context, runner ports.CommandRunner, client, executable string, args ...string) (legacyports.CommandResult, error) {
	if runner == nil {
		return legacyports.CommandResult{}, fmt.Errorf("%s runner is unavailable", client)
	}
	result, err := runner.Run(ctx, legacyports.Command{Argv: append([]string{executable}, args...)})
	if err != nil {
		return result, fmt.Errorf("start %s: %w", client, err)
	}
	if result.ExitCode != 0 {
		return result, fmt.Errorf("%s command failed with exit code %d", client, result.ExitCode)
	}
	return result, nil
}

// CommandOutputContains reports whether a fragment appears anywhere in the
// combined output, case-insensitively.
func CommandOutputContains(result legacyports.CommandResult, fragment string) bool {
	output := string(result.Stdout) + "\n" + string(result.Stderr)
	return strings.Contains(strings.ToLower(output), strings.ToLower(fragment))
}

// CLIAutomaticallyActivates is the predicate shared by clients whose
// Activate path is a managed CLI: a runner and an executable are both
// required, and nothing else.
func CLIAutomaticallyActivates(runner ports.CommandRunner, executable string) bool {
	return runner != nil && strings.TrimSpace(executable) != ""
}

// CLIVerifierAvailable is the matching read-only predicate: the client's
// own listing command is what proves install state, so an empty executable
// cannot verify.
func CLIVerifierAvailable(backendExecutable string) bool {
	return strings.TrimSpace(backendExecutable) != ""
}
