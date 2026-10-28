//go:build linux

package pluginkitairepo_test

import (
	"os"
	"reflect"
	"testing"
)

func TestAgentpluginsGeminiNativeProjectCompleteCycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_GEMINI_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine hosted Gemini complete lifecycle first capture")
	}
	f := newGeminiProjectAttempt(t)
	c := newGeminiProjectCycle(f)
	heldSource := f.evidence["source_identity"]
	defer func() { c.finish(t, heldSource) }()
	heldSource = geminiCycleCompletionSource(t, heldSource)
	c.prepare(t)
	c.startProvider(t)
	for _, scope := range []string{"a", "b", "user"} {
		c.git(t, scope)
	}
	version := c.command(t, "version")
	if string(version.Stdout) != "0.65.0\n" || len(version.Stderr) != 0 {
		t.Fatal("Gemini pinned native version")
	}
	for _, scope := range []string{"a", "b", "user"} {
		c.listing(t, "baseline-"+scope, scope)
	}
	c.native(t, "baseline-a", "a")
	for _, scope := range []string{"user", "a", "b"} {
		c.apply(t, "add-"+scope, scope, true)
		c.listing(t, "installed-"+scope, scope)
		c.native(t, "installed-"+scope, scope)
	}
	c.apply(t, "repeat-add-a", "a", false)
	c.listing(t, "repeat-add-a", "a")
	c.apply(t, "unchanged-update-a", "a", false)
	c.listing(t, "unchanged-update-a", "a")
	c.apply(t, "intact-repair-a", "a", true)
	c.listing(t, "intact-repair-a", "a")
	c.native(t, "restarted-a", "a")
	c.reviseA(t)
	c.apply(t, "update-a", "a", true)
	c.listing(t, "updated-a", "a")
	c.native(t, "updated-a", "a")
	c.missingA(t)
	c.apply(t, "missing-repair-a", "a", true)
	c.listing(t, "repaired-a", "a")
	c.native(t, "repaired-a", "a")
	c.apply(t, "remove-a", "a", true)
	c.listing(t, "removed-a", "a")
	c.native(t, "removed-a", "a")
	c.apply(t, "repeat-remove-a", "a", false)
	c.apply(t, "remove-b", "b", true)
	c.listing(t, "removed-b", "b")
	c.native(t, "removed-b", "b")
	c.apply(t, "remove-user", "user", true)
	c.listing(t, "removed-user", "user")
	c.native(t, "removed-user", "user")
	c.check(t)
	if !reflect.DeepEqual(c.public, geminiCyclePublic) || !reflect.DeepEqual(c.listings, geminiCycleListings) || !reflect.DeepEqual(c.natives, geminiCycleNative) {
		t.Fatal("Gemini complete finite cycle matrix")
	}
	c.observed = true
	c.stage = "complete-cycle-capture-review"
	// The deferred finalizer still owns ordinary closure, held authority checks,
	// raw semantic admission and the fixed completion decision.
}
