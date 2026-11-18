//go:build !darwin && !linux && !windows

package process

import (
	"context"
	"fmt"
	"runtime"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func survivingCommands(context.Context) ([]ports.LiveProcess, error) {
	return nil, fmt.Errorf("listing live processes is not supported on %s", runtime.GOOS)
}
