package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Cold native profiles can start a Git HTTPS marketplace lookup. Allow that
// observed helper to finish naturally within the caller's required deadline;
// the cap prevents an unusually large caller deadline extending settlement.
const completionExitGrace = 30 * time.Second

// RunDuplexToCompletion exchanges bounded input/output with a trusted native
// command, then closes stdin and requires a successful natural exit. It uses
// the ordinary command supervisor and its existing platform prerequisites;
// it does not weaken the separate long-lived ACP containment contract.
func (OS) RunDuplexToCompletion(ctx context.Context, command ports.Command, exchange func(io.Writer, io.Reader) error) error {
	if _, bounded := ctx.Deadline(); !bounded {
		return fmt.Errorf("finite native discovery requires a context deadline")
	}
	pipes := defaultDuplexPipeFactory
	pipes.closure = &duplexClosureProof{}
	pipes.exitGrace = completionExitGrace
	var settlementErr error
	pipes.containment = func(cmd *exec.Cmd) (duplexCommandContainment, error) {
		containment, err := newCompletionCommandContainment(cmd, completionExitGrace)
		if err != nil {
			return nil, err
		}
		return &completionContainment{commandContainment: containment, ctx: ctx, settlementErr: &settlementErr}, nil
	}
	err := runDuplexMode(ctx, command, exchange, pipes, false)
	return errors.Join(err, settlementErr)
}

// A trusted finite client can leave an observed helper finishing natural exit
// after its leader stops. Keep the unreaped leader's identity anchored while
// allowing the ordinary supervisor's bounded settlement, before any forced
// cleanup. This does not change strict ACP containment or accept forced exit.
type completionContainment struct {
	*commandContainment
	ctx           context.Context
	settlementErr *error
}

func (containment *completionContainment) terminate() terminationResult {
	var settleErr error
	if stopped, err := containment.exited(); stopped && err == nil && containment.ctx.Err() == nil {
		_, settleErr = containment.settleNaturalExit(containment.ctx, completionExitGrace)
	}
	termination := containment.commandContainment.terminate()
	*containment.settlementErr = errors.Join(settleErr, containment.ctx.Err())
	return termination
}
