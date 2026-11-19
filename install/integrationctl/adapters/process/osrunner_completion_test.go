package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestOSRunnerFiniteProtocolRequiresCleanEOFExit(t *testing.T) {
	if mode := os.Getenv("UAP_FINITE_PROTOCOL_HELPER"); mode != "" {
		if mode == "delayed-helper-child" {
			time.Sleep(3 * time.Second)
			if err := os.WriteFile(os.Getenv("UAP_FINITE_HELPER_MARKER"), []byte("natural exit"), 0600); err != nil {
				os.Exit(18)
			}
			os.Exit(0)
		}
		reader := bufio.NewReader(os.Stdin)
		if line, err := reader.ReadString('\n'); err != nil || line != "inspect\n" {
			os.Exit(19)
		}
		_, _ = fmt.Fprintln(os.Stdout, "observed")
		_, _ = io.Copy(io.Discard, reader)
		if mode == "delayed-helper" {
			// The native leader exits first. Linux must allow its owned helper
			// to finish after the ordinary two-second grace, within this call.
			child := exec.CommandContext(context.Background(), os.Args[0], "-test.run=^TestOSRunnerFiniteProtocolRequiresCleanEOFExit$")
			child.Env = append(os.Environ(), "UAP_FINITE_PROTOCOL_HELPER=delayed-helper-child")
			if err := child.Start(); err != nil {
				os.Exit(18)
			}
			os.Exit(0)
		}
		if mode == "delayed" {
			// A finite native server can flush its state after stdin closes.
			// This exceeds the ordinary duplex grace but remains within the
			// caller's explicitly bounded completion deadline.
			time.Sleep(3 * time.Second)
		}
		if mode == "nonzero" {
			os.Exit(17)
		}
		if mode == "stalled" {
			for {
				time.Sleep(time.Hour)
			}
		}
		os.Exit(0)
	}
	requireOrdinaryPreflight(t)
	modes := []string{"success", "delayed", "nonzero", "stalled"}
	if runtime.GOOS == "linux" {
		modes = append(modes, "delayed-helper")
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			timeout := 8 * time.Second
			if mode == "stalled" {
				timeout = time.Second
			}
			marker := filepath.Join(t.TempDir(), "helper-natural-exit")
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := (OS{}).RunDuplexToCompletion(ctx, ports.Command{
				Argv: []string{os.Args[0], "-test.run=^TestOSRunnerFiniteProtocolRequiresCleanEOFExit$"},
				Env:  append(os.Environ(), "UAP_FINITE_PROTOCOL_HELPER="+mode, "UAP_FINITE_HELPER_MARKER="+marker),
			}, func(input io.Writer, output io.Reader) error {
				if _, err := io.WriteString(input, "inspect\n"); err != nil {
					return err
				}
				line, err := bufio.NewReader(output).ReadString('\n')
				if err != nil {
					return err
				}
				if line != "observed\n" {
					return fmt.Errorf("wrong response: %q", line)
				}
				return nil
			})
			success := mode == "success" || mode == "delayed" || mode == "delayed-helper"
			if success && err != nil || !success && err == nil {
				t.Fatalf("mode=%s result=%v", mode, err)
			}
			if mode == "delayed-helper" {
				body, readErr := os.ReadFile(marker)
				if readErr != nil || string(body) != "natural exit" {
					t.Fatalf("returned before the owned helper naturally exited: %q %v", body, readErr)
				}
			}
			if mode == "nonzero" && !strings.Contains(err.Error(), "17") {
				t.Fatalf("native exit status was hidden: %v", err)
			}
			if mode == "stalled" && !strings.Contains(err.Error(), "forced") {
				t.Fatalf("forced cleanup was accepted as natural completion: %v", err)
			}
			var unclosed *ports.ProcessClosureError
			if errors.As(err, &unclosed) {
				t.Fatalf("fully cleaned command was mislabeled unclosed: %v", err)
			}
			if mode == "stalled" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("completion deadline was not preserved: %v", err)
			}
		})
	}
}

func TestOSRunnerFiniteProtocolValidatesAfterNaturalEOF(t *testing.T) {
	if os.Getenv("UAP_FINITE_EOF_VALIDATION_HELPER") == "1" {
		if line, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil || line != "inspect\n" {
			os.Exit(19)
		}
		_, _ = fmt.Fprintln(os.Stdout, "observed")
		os.Exit(0)
	}
	requireOrdinaryPreflight(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	validated := false
	err := (OS{}).RunDuplexToCompletion(ctx, ports.Command{
		Argv: []string{os.Args[0], "-test.run=^TestOSRunnerFiniteProtocolValidatesAfterNaturalEOF$"},
		Env:  append(os.Environ(), "UAP_FINITE_EOF_VALIDATION_HELPER=1"),
	}, func(input io.Writer, output io.Reader) error {
		if _, err := io.WriteString(input, "inspect\n"); err != nil {
			return err
		}
		body, err := io.ReadAll(output)
		if err != nil || string(body) != "observed\n" {
			return fmt.Errorf("finite EOF response differs: %q: %w", body, err)
		}
		// A finite server exits after its shutdown acknowledgement. Metadata
		// validation after EOF must finish before the supervisor closes the pipe.
		// This bounded work spans the supervisor's existing 5 ms exit poll.
		time.Sleep(100 * time.Millisecond)
		validated = true
		return nil
	})
	if err != nil || !validated {
		t.Fatalf("natural EOF validation did not complete: validated=%t err=%v", validated, err)
	}
}

type finiteEOFValidationContainment struct {
	eof, observed chan struct{}
	once          sync.Once
	termination   terminationResult
}

func (*finiteEOFValidationContainment) attach(*exec.Cmd) error { return nil }
func (*finiteEOFValidationContainment) close() error           { return nil }
func (c *finiteEOFValidationContainment) terminate() terminationResult {
	return c.termination
}
func (c *finiteEOFValidationContainment) exited() (bool, error) {
	select {
	case <-c.eof:
		c.once.Do(func() { close(c.observed) })
		return true, nil
	default:
		return false, nil
	}
}

func TestOSRunnerFiniteEOFExitBeforeCallbackReturn(t *testing.T) {
	if mode := os.Getenv("UAP_FINITE_EOF_ORDER_HELPER"); mode != "" {
		_, _ = fmt.Fprintln(os.Stdout, "observed")
		if mode == "nonzero" {
			os.Exit(17)
		}
		os.Exit(0)
	}
	for _, mode := range []string{"success", "protocol-error", "nonzero", "canceled", "deadline", "forced-descendants", "cleanup-failure", "interactive"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			containment := &finiteEOFValidationContainment{eof: make(chan struct{}), observed: make(chan struct{})}
			containment.termination = terminationResult{leaderStopped: true}
			if mode == "forced-descendants" {
				containment.termination.forcedMembers = true
			}
			if mode == "cleanup-failure" {
				containment.termination.err = errors.New("inert containment cleanup refused")
			}
			pipes := defaultDuplexPipeFactory
			pipes.containment = func(*exec.Cmd) (duplexCommandContainment, error) { return containment, nil }
			pipes.closure = &duplexClosureProof{}
			pipes.finishExchangeOnExit = mode != "interactive"
			validated := false
			want := errors.New("finite response refused")
			err := runDuplexMode(ctx, ports.Command{
				Argv: []string{os.Args[0], "-test.run=^TestOSRunnerFiniteEOFExitBeforeCallbackReturn$"},
				Env:  append(os.Environ(), "UAP_FINITE_EOF_ORDER_HELPER="+mode),
			}, func(_ io.Writer, output io.Reader) error {
				body, readErr := io.ReadAll(output)
				close(containment.eof)
				select {
				case <-containment.observed:
				case <-ctx.Done():
					return ctx.Err()
				}
				if readErr != nil || string(body) != "observed\n" {
					return errors.Join(readErr, want)
				}
				validated = true
				if mode == "deadline" {
					<-ctx.Done()
					return ctx.Err()
				}
				if mode == "protocol-error" {
					return want
				}
				if mode == "canceled" {
					cancel()
				}
				return nil
			}, pipes, false)
			if !validated || (err == nil) != (mode == "success") {
				t.Fatalf("EOF ordering mode=%s validated=%t err=%v", mode, validated, err)
			}
			if mode == "protocol-error" && !errors.Is(err, want) || mode == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("original finite failure lost: %s %v", mode, err)
			}
			if mode == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("finite callback deadline lost: %v", err)
			}
			if mode == "nonzero" && !strings.Contains(err.Error(), "17") || mode == "interactive" && !strings.Contains(err.Error(), "exited before the exchange completed") {
				t.Fatalf("native or interactive refusal lost: %s %v", mode, err)
			}
		})
	}
}

func TestOSRunnerFiniteProtocolCancellationAndExchangeFailure(t *testing.T) {
	if os.Getenv("UAP_FINITE_WAIT_HELPER") == "1" {
		_, _ = fmt.Fprintln(os.Stdout, "waiting")
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	requireOrdinaryPreflight(t)
	for _, cancelExchange := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		want := errors.New("invalid native observation")
		if cancelExchange {
			want = context.Canceled
		}
		err := (OS{}).RunDuplexToCompletion(ctx, ports.Command{
			Argv: []string{os.Args[0], "-test.run=^TestOSRunnerFiniteProtocolCancellationAndExchangeFailure$"},
			Env:  append(os.Environ(), "UAP_FINITE_WAIT_HELPER=1"),
		}, func(_ io.Writer, output io.Reader) error {
			if _, err := bufio.NewReader(output).ReadString('\n'); err != nil {
				return err
			}
			if cancelExchange {
				cancel()
			}
			return want
		})
		cancel()
		if !errors.Is(err, want) {
			t.Fatalf("original failure/cancellation was hidden: %v", err)
		}
	}
}

func TestOSRunnerFiniteProtocolRefusesUnboundedOrCanceledStart(t *testing.T) {
	called := false
	exchange := func(io.Writer, io.Reader) error { called = true; return nil }
	command := ports.Command{Argv: []string{"must-not-start"}}
	if err := (OS{}).RunDuplexToCompletion(context.Background(), command, exchange); err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("unbounded observation was accepted: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	cancel()
	if err := (OS{}).RunDuplexToCompletion(ctx, command, exchange); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled observation was accepted: %v", err)
	}
	if called {
		t.Fatal("refused observation entered exchange")
	}
}
