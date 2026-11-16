package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type faultedClosureContainment struct {
	duplexCommandContainment
	attachErr, terminateErr, closeErr error
}

func (c *faultedClosureContainment) attach(cmd *exec.Cmd) error {
	if err := c.duplexCommandContainment.attach(cmd); err != nil {
		return err
	}
	return c.attachErr
}
func (c *faultedClosureContainment) terminate() terminationResult {
	result := c.duplexCommandContainment.terminate()
	result.err = errors.Join(result.err, c.terminateErr)
	return result
}
func (c *faultedClosureContainment) close() error {
	return errors.Join(c.duplexCommandContainment.close(), c.closeErr)
}

// A real EOF-exiting child must not turn uncertain containment cleanup into a
// normal failed observation. Conversely, a fully closed protocol failure must
// not acquire the unclosed marker merely because its exchange failed.
func TestCompletionClosureFailureCannotBecomeOrdinaryProbeFailure(t *testing.T) {
	if os.Getenv("UAP_CLOSURE_FAILURE_HELPER") == "1" {
		if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
			os.Exit(31)
		}
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			os.Exit(32)
		}
		os.Exit(0)
	}
	requireOrdinaryPreflight(t)
	sentinel := errors.New("injected closure boundary failure")
	for _, mode := range []string{"cleanup", "cleanup_joined_done", "close", "attach", "closed_protocol"} {
		t.Run(mode, func(t *testing.T) {
			pipes := defaultDuplexPipeFactory
			pipes.closure = &duplexClosureProof{}
			pipes.containment = func(cmd *exec.Cmd) (duplexCommandContainment, error) {
				base, err := newCompletionCommandContainment(cmd, time.Second)
				if err != nil {
					return nil, err
				}
				fault := &faultedClosureContainment{duplexCommandContainment: base}
				switch mode {
				case "cleanup":
					fault.terminateErr = sentinel
				case "cleanup_joined_done":
					fault.terminateErr = errors.Join(sentinel, os.ErrProcessDone)
				case "close":
					fault.closeErr = sentinel
				case "attach":
					fault.attachErr = sentinel
					fault.terminateErr = sentinel
				}
				return fault, nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			err := runDuplexMode(ctx, ports.Command{Argv: []string{os.Args[0], "-test.run=^TestCompletionClosureFailureCannotBecomeOrdinaryProbeFailure$"}, Env: append(os.Environ(), "UAP_CLOSURE_FAILURE_HELPER=1"), Dir: t.TempDir()}, func(_ io.Writer, r io.Reader) error {
				if _, err := bufio.NewReader(r).ReadString('\n'); err != nil {
					return err
				}
				if mode == "closed_protocol" {
					return sentinel
				}
				return nil
			}, pipes, false)
			var unclosed *ports.ProcessClosureError
			got := errors.As(err, &unclosed)
			if got != (mode != "closed_protocol") {
				t.Fatalf("mode %s unclosed=%v error=%v", mode, got, err)
			}
			if !errors.Is(err, sentinel) {
				t.Fatalf("original cause lost: %v", err)
			}
		})
	}
}

func TestAttachCleanupReapUncertaintyIsTyped(t *testing.T) {
	cause := errors.New("attach failed")
	release := make(chan struct{})
	defer close(release)
	err := cleanupAttachFailureWithin(cause, func() terminationResult { return terminationResult{leaderStopped: true} }, func() error { t.Fatal("unexpected fallback kill"); return nil }, func() (bool, error) { return true, nil }, func() error { return nil }, func() error { <-release; return nil }, time.Millisecond)
	var unclosed *ports.ProcessClosureError
	if !errors.As(err, &unclosed) || !errors.Is(err, cause) {
		t.Fatalf("unproved reap lost closure marker or original cause: %v", err)
	}
}
