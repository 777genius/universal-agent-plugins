//go:build windows

package managedstdio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
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

func testSessionContextCancellation(t *testing.T, terminationFailure bool, expectedExtra ...error) {
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
		for _, expected := range expectedExtra {
			if !errors.Is(err, expected) {
				t.Fatalf("cancellation failed to preserve %v: %v", expected, err)
			}
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

// A failed TerminateJobObject followed by a failed last-handle close must
// still reap the direct child rather than hanging forever in cmd.Wait.
// The outer job handle closes during deferred cleanup and reaps descendants.
func TestSessionContextCancellationAfterFailedJobClose(t *testing.T) {
	originalTerminate, originalClose := terminateSessionJob, closeSessionJobAfterTerminationFailure
	terminateSessionJob = func(windows.Handle, uint32) error { return windows.ERROR_ACCESS_DENIED }
	closeSessionJobAfterTerminationFailure = func(windows.Handle) error { return windows.ERROR_INVALID_HANDLE }
	t.Cleanup(func() {
		terminateSessionJob, closeSessionJobAfterTerminationFailure = originalTerminate, originalClose
	})
	testSessionContextCancellation(t, true, windows.ERROR_INVALID_HANDLE)
}

// Cancellation during suspended-process setup must not resume the child.
func TestSessionContextCancellationBeforeResume(t *testing.T) {
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
	for _, beforeResume := range []bool{true, false} {
		if !t.Run(fmt.Sprint(beforeResume), func(t *testing.T) {
			testSessionResumeBoundary(t, fixture, dir, beforeResume)
		}) {
			return // Never launch another fixture after an unsettled teardown.
		}
	}
}

func testSessionResumeBoundary(t *testing.T, fixture, dir string, beforeResume bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	owned, done := make(chan [2]windows.Handle, 1), make(chan struct{})
	var result error
	var resumes atomic.Int32
	original, originalResume := assignProcessToJobObject, resumeSessionChild
	assignProcessToJobObject = func(job, process windows.Handle) error {
		var handles [2]windows.Handle
		self := windows.CurrentProcess()
		for index, handle := range []windows.Handle{job, process} {
			if err := windows.DuplicateHandle(self, handle, self, &handles[index], 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
				if handles[0] != 0 {
					err = errors.Join(err, windows.CloseHandle(handles[0]))
				}
				return err
			}
		}
		owned <- handles // Independent handles exist before assignment can fail or block.
		if err := original(job, process); err != nil {
			return err
		}
		if beforeResume {
			cancel() // Exact boundary: job owns a still-suspended child.
		}
		return nil
	}
	resumeSessionChild = func(pid int) error {
		resumes.Add(1)
		err := originalResume(pid)
		cancel() // Positive control reaches the real resume before cancellation.
		return err
	}
	t.Cleanup(func() {
		cancel()
		if cleanupSessionResumeFixture(t, owned, done) {
			assignProcessToJobObject, resumeSessionChild = original, originalResume
		}
	})
	go func() {
		result = StartSessionContext(ctx, fixture,
			[]string{fixture, "-test.run=^TestWindowsJobFixture$", "pre-resume", dir}, dir,
			append(os.Environ(), "UAP_WINDOWS_JOB_FIXTURE=1"))
		close(done)
	}()
	select {
	case <-done:
		if !errors.Is(result, context.Canceled) {
			t.Fatalf("canceled suspended session: %v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled suspended session was not reaped")
	}
	want := int32(1)
	if beforeResume {
		want = 0
	}
	if got := resumes.Load(); got != want {
		t.Fatalf("native resume calls = %d, want %d", got, want)
	}
}

func cleanupSessionResumeFixture(t *testing.T, owned <-chan [2]windows.Handle, done <-chan struct{}) bool {
	t.Helper()
	var handles [2]windows.Handle
	select {
	case handles = <-owned:
	case <-done:
		select {
		case handles = <-owned:
		default:
		}
	case <-time.After(5 * time.Second):
		t.Error("session setup did not publish ownership or finish")
		return false
	}
	if handles[0] != 0 {
		if status, err := windows.WaitForSingleObject(handles[1], 0); err != nil || status != windows.WAIT_OBJECT_0 {
			t.Errorf("session returned or timed out before owned process settlement: status=%d err=%v", status, err)
		}
		if err := windows.TerminateJobObject(handles[0], 92); err != nil {
			t.Errorf("fixture job termination: %v", err)
		}
		if status, err := windows.WaitForSingleObject(handles[1], 5000); err != nil || status != windows.WAIT_OBJECT_0 {
			t.Errorf("fixture process wait: status=%d err=%v", status, err)
			if err := windows.TerminateProcess(handles[1], 92); err != nil {
				t.Errorf("fixture process fallback termination: %v", err)
			}
			if status, err := windows.WaitForSingleObject(handles[1], 5000); err != nil || status != windows.WAIT_OBJECT_0 {
				t.Errorf("fixture process fallback wait: status=%d err=%v", status, err)
			}
		}
	}
	joined := false
	select {
	case <-done:
		joined = true
	case <-time.After(5 * time.Second):
		t.Error("fixture session goroutine did not finish after cleanup")
	}
	for _, handle := range handles {
		if handle != 0 {
			if err := windows.CloseHandle(handle); err != nil {
				t.Errorf("fixture retained handle close: %v", err)
			}
		}
	}
	return joined
}

// The managed Cline session can select a project on a network share. The
// process-layer guard must agree with StartSessionContext's filepath.IsAbs,
// rather than recognizing only drive-letter absolute paths.
func TestSessionWindowsUNCAbsolutePaths(t *testing.T) {
	for _, value := range []string{
		`C:\project`,
		`\\server\share\project`,
		`\\?\C:\project`,
		`\\?\UNC\server\share\project`,
	} {
		if !filepathIsAbs(value) {
			t.Errorf("valid Windows absolute directory refused: %q", value)
		}
	}
	for _, value := range []string{"", "project", `C:project`, `\project`} {
		if filepathIsAbs(value) {
			t.Errorf("relative Windows directory accepted: %q", value)
		}
	}
}
