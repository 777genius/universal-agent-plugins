//go:build darwin || linux

package process

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"testing"
	"time"
)

const censusChildMarker = "AGENTPLUGINS_CENSUS_CHILD=1"

// A recovery decides from this census whether a command of a killed run may
// still be changing a client, so the kernel's view of a live process must be
// decoded exactly, and this process and its ancestors must be left out. The
// child is this test binary: Darwin hides the environment of platform
// binaries such as /bin/sleep, never of an installed client.
func TestSurvivingCommandsReportArgumentsAndEnvironment(t *testing.T) {
	if slices.Contains(os.Environ(), censusChildMarker) {
		time.Sleep(30 * time.Second)
		return
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSurvivingCommandsReportArgumentsAndEnvironment$")
	cmd.Env = append(os.Environ(), censusChildMarker)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	live, err := (OS{}).SurvivingCommands(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, process := range live {
		if process.PID == os.Getpid() || process.PID == os.Getppid() {
			t.Fatalf("census reported this process or its parent: %+v", process)
		}
		if process.PID != cmd.Process.Pid {
			continue
		}
		seen = true
		if process.Executable == "" || !reflect.DeepEqual(process.Argv, cmd.Args) || !slices.Contains(process.Env, censusChildMarker) {
			t.Fatalf("census misread the child: %+v", process)
		}
	}
	if !seen {
		t.Fatalf("census missed the live child %d among %d processes", cmd.Process.Pid, len(live))
	}
}
