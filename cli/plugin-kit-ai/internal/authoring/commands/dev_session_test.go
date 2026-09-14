package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
)

func TestDevSessionLockConflictsDeterministicallyAndReleases(t *testing.T) {
	scratch, root := t.TempDir(), t.TempDir()
	release, err := acquireDevSession(context.Background(), scratch, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireDevSession(context.Background(), scratch, filepath.Join(root, ".")); runtimeErrorCode(err) != "runtime_dev_session_conflict" {
		t.Fatalf("second session diagnostic = %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("release session: %v", err)
	}
	releaseAgain, err := acquireDevSession(context.Background(), scratch, root)
	if err != nil {
		t.Fatalf("released session remained locked: %v", err)
	}
	if err := releaseAgain(); err != nil {
		t.Fatalf("release reacquired session: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(scratch, "agentplugins-author-dev-locks"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one stable private lock file: %v %v", entries, err)
	}
}

func TestDevPendingDebouncesAndBoundsContinuousChanges(t *testing.T) {
	start := time.Unix(100, 0)
	first := project.Result{}
	first.Input.Identity.TreeDigest = "first"
	second := project.Result{}
	second.Input.Identity.TreeDigest = "second"
	var pending devPending
	pending.observe(first, start)
	if pending.ready(start.Add(devDebounce - time.Millisecond)) {
		t.Fatal("change became ready before the quiet debounce")
	}
	pending.observe(second, start.Add(devDebounce-time.Millisecond))
	if pending.ready(start.Add(2*devDebounce - 2*time.Millisecond)) {
		t.Fatal("second change was not coalesced")
	}
	if !pending.ready(start.Add(2 * devDebounce)) {
		t.Fatal("quiet coalesced change never became ready")
	}
	if got := pending.take().Input.Identity.TreeDigest; got != "second" || pending.set {
		t.Fatalf("coalesced digest=%q pending=%+v", got, pending)
	}
	for elapsed := time.Duration(0); elapsed < devMaxCoalesce; elapsed += devDebounce / 2 {
		pending.observe(second, start.Add(elapsed))
	}
	if !pending.ready(start.Add(devMaxCoalesce)) {
		t.Fatal("continuous edits exceeded the maximum coalescing window")
	}
}
