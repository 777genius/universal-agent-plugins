package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestOSRunnerFiniteProtocolRequiresCleanEOFExit(t *testing.T) {
	if mode := os.Getenv("UAP_FINITE_PROTOCOL_HELPER"); mode != "" {
		reader := bufio.NewReader(os.Stdin)
		if line, err := reader.ReadString('\n'); err != nil || line != "inspect\n" {
			os.Exit(19)
		}
		_, _ = fmt.Fprintln(os.Stdout, "observed")
		_, _ = io.Copy(io.Discard, reader)
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
	for _, mode := range []string{"success", "delayed", "nonzero", "stalled"} {
		t.Run(mode, func(t *testing.T) {
			timeout := 8 * time.Second
			if mode == "stalled" {
				timeout = time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := (OS{}).RunDuplexToCompletion(ctx, ports.Command{
				Argv: []string{os.Args[0], "-test.run=^TestOSRunnerFiniteProtocolRequiresCleanEOFExit$"},
				Env:  append(os.Environ(), "UAP_FINITE_PROTOCOL_HELPER="+mode),
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
			success := mode == "success" || mode == "delayed"
			if success && err != nil || !success && err == nil {
				t.Fatalf("mode=%s result=%v", mode, err)
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
