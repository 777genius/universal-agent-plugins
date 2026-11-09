package commands

import (
	"errors"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
)

func TestDevFailurePendingDeduplicatesIdenticalCodeAndRetainsLatestEvidence(t *testing.T) {
	start := time.Unix(100, 0)
	first, second := project.Result{}, project.Result{}
	first.Input.Identity.TreeDigest = "first"
	second.Input.Identity.TreeDigest = "second"
	errFirst := errors.New("initial read error")
	errSecond := errors.New("updated read error")
	var pending devFailurePending
	pending.observe(first, errFirst, "runtime_source_identity_unavailable", start)
	pending.observe(second, errSecond, "runtime_source_identity_unavailable", start.Add(devDebounce - time.Millisecond))
	if pending.ready(start.Add(devDebounce - time.Millisecond)) {
		t.Fatal("identical failure code was emitted before the quiet deadline")
	}
	if !pending.ready(start.Add(devDebounce)) {
		t.Fatal("identical failure code incorrectly restarted the quiet deadline")
	}
	got, gotCode, gotErr := pending.take()
	if got.Input.Identity.TreeDigest != "second" || !errors.Is(gotErr, errSecond) ||
		gotCode != "runtime_source_identity_unavailable" || pending.set {
		t.Fatalf("latest failure evidence lost or pending window not reset: %s %v %q %+v",
			got.Input.Identity.TreeDigest, gotErr, gotCode, pending)
	}
	if pending.ready(start.Add(devMaxCoalesce)) {
		t.Fatal("consumed failure window remained ready")
	}
}

func TestDevFailurePendingNewCodeRestartsQuietDeadlineButNotHardLimit(t *testing.T) {
	start := time.Unix(200, 0)
	var pending devFailurePending
	pending.observe(project.Result{}, errors.New("read"), "read_failed", start)
	pending.observe(project.Result{}, errors.New("identity"), "identity_failed", start.Add(devDebounce - time.Millisecond))
	if pending.ready(start.Add(devDebounce)) {
		t.Fatal("new failure code did not restart the quiet deadline")
	}
	if !pending.ready(start.Add(2 * devDebounce)) {
		t.Fatal("new failure code never became ready after the quiet deadline")
	}

	pending = devFailurePending{}
	for elapsed, index := time.Duration(0), 0; elapsed < devMaxCoalesce; elapsed, index = elapsed+devDebounce/2, index+1 {
		code := "read_failed"
		if index%2 != 0 {
			code = "identity_failed"
		}
		pending.observe(project.Result{}, errors.New(code), code, start.Add(elapsed))
	}
	if !pending.ready(start.Add(devMaxCoalesce)) {
		t.Fatal("continuous code changes exceeded the hard coalescing deadline")
	}
}
