//go:build !windows

package main

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

const hangupChildEnv = "UAP_TEST_INSTALLER_HANGUP_CHILD_20261116"

// runHangupChild reports how installer cancellation reacts to a real SIGHUP
// delivered to this process: exit 0 when the context is canceled, exit 4 when
// it keeps running. A hangup that is not handled terminates the child by signal.
func runHangupChild() {
	os.Exit(hangupChildExitCode())
}

func hangupChildExitCode() int {
	ctx, stop := installerSignalContext(context.Background())
	defer stop()
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		return 3
	}
	select {
	case <-ctx.Done():
		return 0
	case <-time.After(2 * time.Second):
		return 4
	}
}

// Closing the terminal or SSH session sends SIGHUP. It must take the same
// cancellation path as Ctrl+C so the installer records native effects it has
// already made, instead of dying with an unresolved native activation attempt.
// A caller that deliberately ignores hangups (nohup) keeps running.
func TestInstallerSignalContextHandlesTerminalHangup(t *testing.T) {
	if os.Getenv(hangupChildEnv) == "1" {
		runHangupChild()
		return
	}
	for _, test := range []struct {
		name     string
		argv     []string
		wantExit int
	}{
		{"hangup cancels", []string{os.Args[0]}, 0},
		// The shell leaves SIGHUP ignored across exec, exactly like nohup.
		{"ignored hangup is preserved", []string{"/bin/sh", "-c", `trap "" HUP; exec "$0" "$@"`, os.Args[0]}, 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A test runner started under nohup passes ignored SIGHUP to the child.
			if test.wantExit == 0 && signal.Ignored(syscall.SIGHUP) {
				t.Skip("SIGHUP is ignored by the test process, as under nohup")
			}
			argv := append(append([]string{}, test.argv...), "-test.run=^TestInstallerSignalContextHandlesTerminalHangup$", "-test.count=1")
			child := exec.CommandContext(t.Context(), argv[0], argv[1:]...)
			child.Env = append(os.Environ(), hangupChildEnv+"=1")
			output, err := child.CombinedOutput()
			state := child.ProcessState
			if state == nil {
				t.Fatalf("child did not run: %v\n%s", err, output)
			}
			if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				t.Fatalf("child was terminated by %v instead of handling the hangup\n%s", status.Signal(), output)
			}
			if state.ExitCode() != test.wantExit {
				t.Fatalf("child exit = %d, want %d: %v\n%s", state.ExitCode(), test.wantExit, err, output)
			}
		})
	}
}
