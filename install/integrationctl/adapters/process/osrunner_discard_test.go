package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestOSRunnerDiscardsStdoutAndPreservesNonzeroExit(t *testing.T) {
	if os.Getenv("UAP_DISCARD_STDOUT_HELPER") == "1" {
		_, _ = fmt.Fprint(os.Stdout, strings.Repeat("discarded output", 100000))
		_, _ = fmt.Fprint(os.Stderr, "retained diagnostic")
		os.Exit(7)
	}
	requireOrdinaryPreflight(t)
	result, err := (OS{}).Run(t.Context(), ports.Command{
		Argv: []string{os.Args[0], "-test.run=^TestOSRunnerDiscardsStdoutAndPreservesNonzeroExit$"},
		Env:  append(os.Environ(), "UAP_DISCARD_STDOUT_HELPER=1"),
		// Discarding must keep draining, even when a capture limit is present.
		DiscardStdout: true, StdoutLimitBytes: 1,
	})
	if err != nil || result.ExitCode != 7 || len(result.Stdout) != 0 ||
		string(result.Stderr) != "retained diagnostic" || !result.CleanupComplete {
		t.Fatalf("discarded command result = %+v, error = %v", result, err)
	}
}

func TestOSRunnerDiscardStdoutPreservesCancellationAndCleanup(t *testing.T) {
	if ready := os.Getenv("UAP_DISCARD_CANCEL_READY"); ready != "" {
		_, _ = fmt.Fprint(os.Stdout, strings.Repeat("discarded output", 100000))
		if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
			os.Exit(81)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	requireOrdinaryPreflight(t)
	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type outcome struct {
		result ports.CommandResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := (OS{}).Run(ctx, ports.Command{
			Argv: []string{os.Args[0], "-test.run=^TestOSRunnerDiscardStdoutPreservesCancellationAndCleanup$"},
			Env:  append(os.Environ(), "UAP_DISCARD_CANCEL_READY="+ready), DiscardStdout: true,
		})
		done <- outcome{result, err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("discarded producer did not become ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case got := <-done:
		if !errors.Is(got.err, context.Canceled) || len(got.result.Stdout) != 0 || !got.result.CleanupComplete {
			t.Fatalf("discarded canceled result = %+v, error = %v", got.result, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("discarded command cancellation did not stop within its cleanup bound")
	}
}
