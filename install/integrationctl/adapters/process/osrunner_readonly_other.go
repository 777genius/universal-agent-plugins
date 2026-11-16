//go:build !linux

package process

import (
	"context"
	"errors"
	"io"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (OS) RunReadonlyDuplexWithPlannedShutdown(ctx context.Context, _ ports.Command, _ ports.ReadonlyDuplexInputs, _ func(io.Writer, io.Reader) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.New("readonly native duplex transport is unavailable on this platform")
}
