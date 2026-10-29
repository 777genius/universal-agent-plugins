package copilot

import (
	"context"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Copilot 1.0.103 includes this separate builtin inventory with its user plugin
// listing. Only this observed section is accepted; the shared user-plugin parser
// still validates the complete identity, version, enablement and install path.
const builtinListingSection = "Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\n"

func listingEnv(env clients.Env) clients.Env {
	if env.Runner == nil {
		return env
	}
	runner := listingCommandRunner{env.Runner}
	if tree, ok := env.Runner.(ports.TreeCommandRunner); ok {
		env.Runner = listingTreeRunner{runner, tree}
	} else {
		env.Runner = runner
	}
	return env
}

type listingCommandRunner struct{ ports.CommandRunner }

func (r listingCommandRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	result, err := r.CommandRunner.Run(ctx, command)
	return normalizeListing(command, result, err), err
}

type listingTreeRunner struct {
	listingCommandRunner
	tree ports.TreeCommandRunner
}

func (r listingTreeRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	result, err := r.tree.RunWithTreeExitGrace(ctx, command, grace)
	return normalizeListing(command, result, err), err
}

func normalizeListing(command legacyports.Command, result legacyports.CommandResult, err error) legacyports.CommandResult {
	if err != nil || result.ExitCode != 0 || len(command.Argv) != 3 || command.Argv[1] != "plugin" || command.Argv[2] != "list" {
		return result
	}
	document := strings.ReplaceAll(string(result.Stdout), "\r\n", "\n")
	if strings.Count(document, "Built-in plugins:") != 1 {
		return result
	}
	if strings.HasPrefix(document, builtinListingSection) {
		result.Stdout = []byte(strings.TrimPrefix(document, builtinListingSection))
	} else if strings.HasSuffix(document, "\n"+builtinListingSection) {
		result.Stdout = []byte(strings.TrimSuffix(document, builtinListingSection))
	}
	return result
}
