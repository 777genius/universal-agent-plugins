package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Ordinary machine problems must be explained in plain words, from the typed
// cause that survives wrapping, and must never leak technical detail.
func TestEnvironmentFailuresGetPlainCauses(t *testing.T) {
	t.Parallel()
	secretPath := "/Users/someone/project/.claude/skills"
	cases := map[string]struct {
		err   error
		kind  failureKind
		words string
	}{
		"permission":  {&fs.PathError{Op: "mkdir", Path: secretPath, Err: syscall.EACCES}, failurePermission, "not allowed to write"},
		"os-sentinel": {fmt.Errorf("create client target root: %w", os.ErrPermission), failurePermission, "not allowed to write"},
		"disk full":   {fmt.Errorf("write: %w", &fs.PathError{Op: "write", Path: secretPath, Err: syscall.ENOSPC}), failureDiskFull, "free disk space"},
		"read only":   {fmt.Errorf("write: %w", &fs.PathError{Op: "mkdir", Path: secretPath, Err: syscall.EROFS}), failureReadOnly, "read-only"},
		"network":     {fmt.Errorf("fetch: %w", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("no route to host")}), failureNetwork, "network is unavailable"},
		"no program":  {fmt.Errorf("probe: %w", &exec.Error{Name: "claude", Err: exec.ErrNotFound}), failureAgentMissing, "not found"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			failure := describeBindingFailure(environment.BindingOutcome{Status: environment.BindingFailed, Err: tc.err})
			if failure.Kind != tc.kind || !failure.Retryable || !strings.Contains(failure.Summary, tc.words) {
				t.Fatalf("got %+v", failure)
			}
			if strings.Contains(failure.Summary+failure.Next, secretPath) {
				t.Fatalf("the terminal wording must not carry technical detail: %+v", failure)
			}
		})
	}
}

// A real cancellation or deadline keeps its own wording even when the wrapped
// chain also contains a network error.
func TestEnvironmentClassificationDoesNotOverrideTimeoutOrCancel(t *testing.T) {
	t.Parallel()
	timeoutDial := &net.OpError{Op: "dial", Err: context.DeadlineExceeded}
	if got := failureKindOf(fmt.Errorf("x: %w", timeoutDial)); got != failureTimeout {
		t.Fatalf("deadline must win over network: %s", got)
	}
	if got := failureKindOf(fmt.Errorf("x: %w", context.Canceled)); got != failureCanceled {
		t.Fatalf("cancel must win: %s", got)
	}
	if got := failureKindOf(errors.New("something else")); got != failureUnknown {
		t.Fatalf("unrelated errors stay generic: %s", got)
	}
}
