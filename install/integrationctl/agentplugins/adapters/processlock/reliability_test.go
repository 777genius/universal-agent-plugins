package processlock

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLockConcurrentContendersAndRetry(t *testing.T) {
	const writers = 16
	lock := Lock{Path: filepath.Join(t.TempDir(), "mutation.lock")}
	started := make(chan struct{})
	results := make(chan struct {
		unlock func() error
		err    error
	}, writers)
	var group sync.WaitGroup
	for range writers {
		group.Add(1)
		go func() {
			defer group.Done()
			<-started
			unlock, err := lock.Acquire(context.Background())
			results <- struct {
				unlock func() error
				err    error
			}{unlock: unlock, err: err}
		}()
	}
	close(started)
	group.Wait()
	close(results)
	var winner func() error
	successful := 0
	for outcome := range results {
		if outcome.err == nil {
			successful++
			winner = outcome.unlock
		} else if !errors.Is(outcome.err, ErrActive) {
			t.Fatalf("concurrent writer returned unexpected error: %v", outcome.err)
		}
	}
	if successful != 1 || winner == nil {
		t.Fatalf("exactly one exclusive writer required, got %d", successful)
	}
	if err := winner(); err != nil {
		t.Fatal(err)
	}
	retry, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("released lock did not permit a new writer: %v", err)
	}
	if err := retry(); err != nil {
		t.Fatal(err)
	}
}

// A child *process* holds the real OS lock. A forced kill, without any unlock
// call, must release the kernel-owned handle before the next mutation.
func TestLockReleasedAfterForcedProcessDeath(t *testing.T) {
	if os.Getenv("UAP_PROCESSLOCK_KILL_CHILD") == "1" {
		release, err := (Lock{Path: os.Getenv("UAP_PROCESSLOCK_KILL_PATH")}).Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintln(os.Stdout, "UAP_PROCESSLOCK_ACQUIRED"); err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, os.Stdin)
		_ = release()
		return
	}
	path := filepath.Join(t.TempDir(), "mutation.lock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLockReleasedAfterForcedProcessDeath$")
	child.Env = append(os.Environ(), "UAP_PROCESSLOCK_KILL_CHILD=1", "UAP_PROCESSLOCK_KILL_PATH="+path)
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	child.Stderr = io.Discard
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = stdin.Close()
		if !waited {
			_ = child.Wait()
		}
	})
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err == nil && !strings.Contains(line, "UAP_PROCESSLOCK_ACQUIRED") {
			err = fmt.Errorf("child did not report its kernel lock: %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("child lock handshake: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("child never acquired OS lock")
	}
	lock := Lock{Path: path}
	if _, err := lock.Acquire(context.Background()); !errors.Is(err, ErrActive) {
		t.Fatalf("foreign process was not excluded: %v", err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("child exited cleanly; expected forced termination")
	}
	waited = true
	_ = stdin.Close()
	release, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("lock remained held after process death: %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}
