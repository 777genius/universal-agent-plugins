package ports

import (
	"context"
	"time"
)

type PathInfo struct {
	Exists bool
	IsDir  bool
}

type FileSystem interface {
	ReadFile(context.Context, string) ([]byte, error)
	WriteFileAtomic(context.Context, string, []byte, uint32) error
	MkdirAll(context.Context, string, uint32) error
	Stat(context.Context, string) (PathInfo, error)
	Remove(context.Context, string) error
}

type SafeFileMutationInput struct {
	Path           string
	Mode           uint32
	Build          func(original []byte, exists bool) ([]byte, error)
	ValidateBefore func(next []byte) error
	ValidateAfter  func(context.Context, string, []byte) error
}

type SafeFileMutationResult struct {
	Path        string
	BackupPath  string
	HadOriginal bool
}

type SafeFileMutator interface {
	MutateFile(context.Context, SafeFileMutationInput) (SafeFileMutationResult, error)
}

type Command struct {
	Argv []string
	Env  []string
	Dir  string
	// StdoutLimitBytes bounds captured stdout without terminating the child.
	// Zero keeps the existing unbounded behavior for trusted callers.
	StdoutLimitBytes int
}

type CommandResult struct {
	// NoChildStarted is positive evidence that execution failed before a process
	// was successfully started. Zero PID or an empty result alone is not proof.
	NoChildStarted bool
	// CleanupComplete attests that the supervised leader was reaped, descendant
	// cleanup succeeded, and containment was closed before returning. False is
	// unknown; injected runners must not infer this solely from a timeout.
	CleanupComplete bool
	ExitCode        int
	Stdout          []byte
	Stderr          []byte
}

type ProcessRunner interface {
	Run(context.Context, Command) (CommandResult, error)
}

type Clock interface {
	Now() time.Time
}
