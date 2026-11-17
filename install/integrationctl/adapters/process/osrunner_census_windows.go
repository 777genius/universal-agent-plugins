//go:build windows

package process

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Every command runs in a job created with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
// whose only handle belongs to the invocation that started it. The system ends
// the whole job when that invocation ends, however it ends.
func survivingCommands(context.Context) ([]ports.LiveProcess, error) {
	return nil, nil
}
