package main

import (
	"sort"
	"testing"
)

// TestPathPolicyHasOneImplementation is the safety half of inverting the path
// policy into a port. The interface makes a permissive stand-in possible for
// the first time, and a stand-in that quietly accepts a symlinked or escaping
// path removes the last check before a destructive operation. Any candidate
// implementation belongs in adapters/pathpolicy and has to pass
// ports/contracttest.RunPathPolicy.
func TestPathPolicyHasOneImplementation(t *testing.T) {
	t.Parallel()
	implementations, err := pathPolicyImplementations(testRepoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(implementations) == 0 {
		t.Fatal("no PathPolicy method found at all; the rule stopped measuring anything")
	}
	names := make([]string, 0, len(implementations))
	for name := range implementations {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if implementations[name] != pathPolicyOwner {
			t.Errorf("%s declares a ports.PathPolicy method outside %s; containment has one implementation on purpose, and one overriding method on an embedded Policy is enough to remove a check", name, pathPolicyOwner)
		}
	}
}
