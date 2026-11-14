package ports

import (
	"context"
	"io"
	"time"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// CommandRunner executes one client command. Command and CommandResult stay in
// install/integrationctl/ports: they are plain data with no behavior, and
// duplicating them into domain would create a second source of truth and a
// conversion on every call. See docs/ARCHITECTURE.md.
type CommandRunner interface {
	Run(context.Context, legacyports.Command) (legacyports.CommandResult, error)
}

// TreeCommandRunner is an opt-in capability: the runner contains the whole
// process tree and reports the exit only after descendants are gone, so an
// observation is never based on descendant sampling.
type TreeCommandRunner interface {
	RunWithTreeExitGrace(context.Context, legacyports.Command, time.Duration) (legacyports.CommandResult, error)
}

// DuplexCommandRunner is an opt-in capability for protocols that must write to
// stdin and read stdout of the same live process, then shut it down by plan.
type DuplexCommandRunner interface {
	RunDuplexWithPlannedShutdown(context.Context, legacyports.Command, func(io.Writer, io.Reader) error) error
}

// DuplexCapabilityRunner reports whether duplex execution is actually available
// on this platform before an observation is attempted.
type DuplexCapabilityRunner interface {
	DuplexCommandRunner
	DuplexCapability() error
}
