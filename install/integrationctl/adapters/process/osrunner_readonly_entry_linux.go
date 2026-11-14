//go:build linux

package process

import (
	"context"
	"errors"
	"io"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// RunReadonlyDuplexWithPlannedShutdown retains the existing process owner and
// sole Wait. It has no ordinary transport fallback. Caller-owned input handles
// and any native locks remain borrowed through the final original-view check.
func (OS) RunReadonlyDuplexWithPlannedShutdown(ctx context.Context, command ports.Command, originals ports.ReadonlyDuplexInputs, exchange func(io.Writer, io.Reader) error) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if exchange == nil {
		return errors.New("readonly native exchange is required")
	}
	if _, err := readonlyOriginalDeadline(ctx); err != nil {
		return err
	}
	if err := errors.Join(duplexContainmentPreflight(), ctx.Err()); err != nil {
		return err
	}
	command.Argv = append([]string(nil), command.Argv...)
	command.Env = append([]string(nil), command.Env...)
	view, err := prepareReadonlyView(ctx, command, originals)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, view.system.close()) }()
	pipes := readonlyDuplexFactory(ctx, defaultDuplexPipeFactory, view.prepare)
	err = runDuplexMode(ctx, command, func(stdin io.Writer, stdout io.Reader) error {
		if err := readReadonlyHeader(ctx, stdout, view.policy, view.digest); err != nil {
			return err
		}
		// No buffering wrapper: existing native tail capture receives the very
		// same original stdout file and owns its established EOF protocol.
		return exchange(stdin, stdout)
	}, pipes, true)
	return errors.Join(err, view.revalidate(ctx, true), ctx.Err())
}
