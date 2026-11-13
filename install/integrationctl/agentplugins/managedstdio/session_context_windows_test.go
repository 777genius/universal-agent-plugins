//go:build windows

package managedstdio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Cancel the session context itself, without killing its owner or relying on
// console signals. The existing fixture creates three real generations.
func TestSessionContextCancellationReapsOwnedTree(t *testing.T) {
	for _, terminationFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(terminationFailure), func(t *testing.T) {
			if terminationFailure {
				original := terminateSessionJob
				terminateSessionJob = func(windows.Handle, uint32) error { return windows.ERROR_ACCESS_DENIED }
				t.Cleanup(func() { terminateSessionJob = original })
			}
			testSessionContextCancellation(t, terminationFailure)
		})
	}
}

func testSessionContextCancellation(t *testing.T, terminationFailure bool) {
	t.Helper()
	dir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "fixture.exe")
	if err := os.WriteFile(fixture, body, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- StartSessionContext(ctx, fixture,
			[]string{fixture, "-test.run=^TestWindowsJobFixture$", "root", dir}, dir,
			append(os.Environ(), "UAP_WINDOWS_JOB_FIXTURE=1"))
	}()
	handles := make([]windows.Handle, 0, 3)
	for _, role := range []string{"root", "child", "grandchild"} {
		handles = append(handles, windowsFixtureHandle(t, dir, role))
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("session cancellation: %v", err)
		}
		if terminationFailure && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Fatalf("termination failure lost: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session cancellation did not reap native process")
	}
	for index, handle := range handles {
		status, err := windows.WaitForSingleObject(handle, 5000)
		if err != nil || status != windows.WAIT_OBJECT_0 {
			t.Errorf("generation %d survives: status=%d err=%v", index, status, err)
		}
	}
}
