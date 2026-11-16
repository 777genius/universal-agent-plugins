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

// RunDuplexToCompletion exchanges bounded input/output with a trusted native
// command, then closes stdin and requires a successful natural exit within the
// caller's deadline. The deadline covers both the exchange and native shutdown.
// It uses the ordinary command supervisor and its existing platform prerequisites;
// it does not weaken the separate long-lived ACP containment contract.
func (OS) RunDuplexToCompletion(ctx context.Context, command ports.Command, exchange func(io.Writer, io.Reader) error) error {
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return fmt.Errorf("finite native discovery requires a context deadline")
	}
	pipes := defaultDuplexPipeFactory
	pipes.closure = &duplexClosureProof{}
	pipes.exitGrace = time.Until(deadline)
	var settlementErr error
	pipes.containment = func(cmd *exec.Cmd) (duplexCommandContainment, error) {
		// Linux settles owned helpers inside the supervisor after the native
		// leader exits. Give that natural settlement the caller's remaining
		// budget too; forced cleanup retains its separate processReapTimeout.
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, context.DeadlineExceeded
		}
		containment, err := newCompletionCommandContainment(cmd, remaining)
		if err != nil {
			return nil, err
		}
		return &completionContainment{commandContainment: containment, ctx: ctx, deadline: deadline, settlementErr: &settlementErr}, nil
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
	deadline      time.Time
	settlementErr *error
}

func (containment *completionContainment) terminate() terminationResult {
	var settleErr error
	if stopped, err := containment.exited(); stopped && err == nil && containment.ctx.Err() == nil {
		remaining := time.Until(containment.deadline)
		if remaining <= 0 {
			settleErr = context.DeadlineExceeded
		} else {
			_, settleErr = containment.settleNaturalExit(containment.ctx, remaining)
		}
	}
	termination := containment.commandContainment.terminate()
	*containment.settlementErr = errors.Join(settleErr, containment.ctx.Err())
	return termination
}
