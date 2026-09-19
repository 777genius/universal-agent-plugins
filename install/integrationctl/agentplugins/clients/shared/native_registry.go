package shared

import (
	"context"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const nativeRegistryTreeExitGrace = time.Second

// RunNativeRegistry executes a trusted, short-lived client list command. A
// tree-aware runner is used when available so Linux requires atomic cgroup
// containment and Windows uses a Job Object.
func RunNativeRegistry(ctx context.Context, runner ports.CommandRunner, command legacyports.Command) (legacyports.CommandResult, error) {
	if tree, ok := runner.(ports.TreeCommandRunner); ok {
		return tree.RunWithTreeExitGrace(ctx, command, nativeRegistryTreeExitGrace)
	}
	return runner.Run(ctx, command)
}
