package skillsource

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type sizeConcurrencyRunner struct {
	active, peak, reads atomic.Int32
	slowObject          string
}

func (r *sizeConcurrencyRunner) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	if len(command.Args) == 3 && command.Args[0] == "cat-file" && command.Args[1] == "-s" {
		r.reads.Add(1)
		active := r.active.Add(1)
		defer r.active.Add(-1)
		for previous := r.peak.Load(); active > previous; previous = r.peak.Load() {
			if r.peak.CompareAndSwap(previous, active) {
				break
			}
		}
		delay := 15 * time.Millisecond
		if command.Args[2] == r.slowObject {
			delay = 80 * time.Millisecond
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return (sourceacquisition.OSRunner{}).Run(ctx, command)
}

func TestScanMetadataConcurrencyPreservesCanonicalAggregateBudget(t *testing.T) {
	root, acquirer := fixture(t)
	for _, name := range []string{"alpha", "bravo", "third"} {
		put(t, root, name+"/SKILL.md", document(name), 0600)
	}
	sha := commitFixture(t, root)
	runner := &sizeConcurrencyRunner{slowObject: git(t, root, "rev-parse", sha+":alpha/SKILL.md")}
	acquirer.Runner = runner
	acquirer.Policy.MaxAggregateBytes = len(document("alpha"))
	session, err := acquirer.Open(t.Context(), Source{Repository: "example/fixture", Commit: sha})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	catalog, err := session.Scan(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	// Three objects fit under the read bound, so all of them are in flight at once.
	if runner.peak.Load() != 3 || runner.reads.Load() != 3 {
		t.Fatalf("metadata reads=%d peak=%d; want 3 reads, all concurrent", runner.reads.Load(), runner.peak.Load())
	}
	if catalog.Complete || len(catalog.Candidates) != 3 || catalog.Candidates[0].Name != "alpha" || catalog.Candidates[0].Status != "available" {
		t.Fatalf("canonical first candidate did not retain the aggregate reservation: %+v", catalog)
	}
	for _, candidate := range catalog.Candidates[1:] {
		if len(candidate.Diagnostics) != 1 || candidate.Diagnostics[0] != "skill_aggregate_bytes_limit" {
			t.Fatalf("later candidate consumed the first candidate's reservation: %+v", candidate)
		}
	}
}

// More objects than the bound never exceed it, so a huge repository cannot
// open an unbounded number of network reads.
func TestScanMetadataConcurrencyNeverExceedsBound(t *testing.T) {
	root, acquirer := fixture(t)
	for i := range documentReadConcurrency + 4 {
		name := fmt.Sprintf("skill%02d", i)
		put(t, root, name+"/SKILL.md", document(name), 0600)
	}
	sha := commitFixture(t, root)
	runner := &sizeConcurrencyRunner{}
	acquirer.Runner = runner
	session, err := acquirer.Open(t.Context(), Source{Repository: "example/fixture", Commit: sha})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := session.Scan(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	if peak := int(runner.peak.Load()); peak > documentReadConcurrency || peak < 2 {
		t.Fatalf("peak concurrent reads=%d, want between 2 and %d", peak, documentReadConcurrency)
	}
}
