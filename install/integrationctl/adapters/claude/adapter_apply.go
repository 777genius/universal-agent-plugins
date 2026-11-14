package claude

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) runClaude(ctx context.Context, argv []string, dir string) error {
	result, err := a.runner().Run(ctx, ports.Command{Argv: argv, Dir: dir})
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "run Claude CLI", err)
	}
	if result.ExitCode != 0 {
		msg := strings.TrimSpace(string(result.Stderr))
		if msg == "" {
			msg = strings.TrimSpace(string(result.Stdout))
		}
		if msg == "" {
			msg = "Claude CLI command failed"
		}
		return domain.NewError(domain.ErrMutationApply, msg, nil)
	}
	return nil
}

func scopedPluginArgv(base []string, scope string) []string {
	argv := append([]string{}, base...)
	if scope = strings.TrimSpace(scope); scope != "" {
		argv = append(argv, "--scope", scope)
	}
	return argv
}
