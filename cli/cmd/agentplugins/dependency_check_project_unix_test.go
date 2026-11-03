//go:build unix

package main

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestProductionDependencyCheckDoesNotOpenStateFIFO(t *testing.T) {
	app, _, project, _ := projectCompositionFixture(t)
	state := filepath.Join(project, ".uap", "state-v2.json")
	if err := os.MkdirAll(filepath.Dir(state), 0700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(state, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(state)
	if err != nil {
		t.Fatal(err)
	}
	command, output, agreement := productionProjectCheckCommand(t, app, project)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- command.ExecuteContext(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		// Unblock a regressed reader so failure cannot leak a test goroutine.
		file, err := os.OpenFile(state, os.O_RDWR|syscall.O_NONBLOCK, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_ = file.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("project check remained blocked after releasing state FIFO")
		}
		t.Fatal("project integrity check blocked while opening unrelated installed state")
	}
	assertProductionProjectCheckUnchanged(t, output, agreement, state, before)
}
