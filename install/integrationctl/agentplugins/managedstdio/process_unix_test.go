//go:build darwin || linux

package managedstdio

import (
	"bufio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestExecKeepsPIDAndControlledSignalExit(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	exe, _ := os.Executable()
	body, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "child"), body, 0700); err != nil {
		t.Fatal(err)
	}
	cmd := fixtureCommand(t, Arguments(root, data, root, pathcontract.Plugin, "./child", []string{"-test.run=^TestSubprocess$", "fixture", "child", "sleep"}))
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(pipe).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("child readiness %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child did not start")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		status := cmd.ProcessState.Sys().(syscall.WaitStatus)
		if !status.Signaled() || status.Signal() != syscall.SIGTERM {
			t.Fatalf("lost native signal status: %v", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("controlled signal failed to stop transport PID")
	}
}
