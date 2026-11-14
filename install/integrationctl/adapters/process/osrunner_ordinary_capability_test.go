package process

import (
	"errors"
	"testing"
)

func requireOrdinaryPreflight(t *testing.T) {
	t.Helper()
	if err := (OS{}).OrdinaryPreflight(); err != nil {
		if errors.Is(err, ErrOrdinaryContainmentUnsupported) {
			t.Skipf("ordinary containment runtime is not qualified on this host: %v", err)
		}
		t.Fatalf("ordinary containment preflight failed: %v", err)
	}
}
