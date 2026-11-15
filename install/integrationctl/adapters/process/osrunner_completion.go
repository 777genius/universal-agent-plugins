package process

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// RunDuplexToCompletion exchanges bounded input/output with a trusted native
// command, then closes stdin and requires a successful natural exit. It uses
// the ordinary command supervisor and its existing platform prerequisites;
// it does not weaken the separate long-lived ACP containment contract.
func (OS) RunDuplexToCompletion(ctx context.Context, command ports.Command, exchange func(io.Writer, io.Reader) error) error {
	if _, bounded := ctx.Deadline(); !bounded {
		return fmt.Errorf("finite native discovery requires a context deadline")
	}
	pipes := defaultDuplexPipeFactory
	pipes.containment = func(cmd *exec.Cmd) (duplexCommandContainment, error) {
		return newCompletionCommandContainment(cmd, duplexExitGrace)
	}
	return runDuplexMode(ctx, command, exchange, pipes, false)
}
