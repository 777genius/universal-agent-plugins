package agentpluginscli

import (
	"context"
	"errors"
	"io"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (runner *unchangedSkillsRunner) RunDuplexToCompletion(_ context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	runner.observations++
	if runner.unavailable {
		return errors.New("native inventory is unavailable")
	}
	return savedCodexDiscovery(command, runner.inner.inner.codexIDs, exchange)
}
