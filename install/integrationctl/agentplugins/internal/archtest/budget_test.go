package main

import (
	"testing"
)

// TestClientIDBudgetDoesNotGrow is a ratchet: the refactor removes
// client-specific branching from the generic packages, so a package may only
// ever shrink. Regenerate the baseline with
// `go run ./internal/archtest -update` from install/integrationctl/agentplugins.
func TestClientIDBudgetDoesNotGrow(t *testing.T) {
	t.Parallel()
	root := testRepoRoot(t)
	baseline, err := readBudget(root)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := clientIDBudget(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range sortedKeys(counts) {
		allowed, known := baseline[name]
		if !known {
			t.Errorf("package %s newly branches on client identity (%d references); the refactor only removes them", name, counts[name])
			continue
		}
		if counts[name] > allowed {
			t.Errorf("package %s = %d client identity references, budget is %d", name, counts[name], allowed)
		}
	}
}

// TestClientIDBudgetBaselineIsCurrent keeps the committed numbers honest: a
// package that shrank below its budget must lower the budget in the same
// change, otherwise the ratchet quietly stops protecting it.
func TestClientIDBudgetBaselineIsCurrent(t *testing.T) {
	t.Parallel()
	root := testRepoRoot(t)
	baseline, err := readBudget(root)
	if err != nil {
		t.Fatal(err)
	}
	counts, err := clientIDBudget(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range sortedKeys(baseline) {
		if counts[name] < baseline[name] {
			t.Errorf("package %s = %d client identity references but the budget still claims %d; run archtest -update", name, counts[name], baseline[name])
		}
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}
