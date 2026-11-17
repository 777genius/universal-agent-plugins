package process

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// SurvivingCommands lists live processes of the current user, except this
// process and its ancestors, that a command started by an earlier invocation
// could have left running after that invocation was killed. Unix process
// groups outlive their killed owner, so every such process is reported with
// the arguments and environment the system exposes. Windows commands run in a
// kill-on-close job, so none can outlive the invocation that started it.
func (OS) SurvivingCommands(ctx context.Context) ([]ports.LiveProcess, error) {
	return survivingCommands(ctx)
}
