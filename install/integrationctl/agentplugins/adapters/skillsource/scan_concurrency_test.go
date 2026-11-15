package skillsource

import (
	"context"
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
	if runner.peak.Load() != 2 || runner.reads.Load() != 3 {
		t.Fatalf("metadata reads=%d peak=%d; want 3 reads with 2 concurrent", runner.reads.Load(), runner.peak.Load())
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
