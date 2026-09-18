// Package contracttest holds the executable contracts a port implementation has
// to satisfy. RunPathPolicy is mandatory for any candidate PathPolicy: the port
// is the only thing standing between persisted metadata and a destructive
// filesystem operation, so "it compiles" is not evidence that it is safe.
package contracttest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// RunPathPolicy asserts both directions: the rejections that make the port a
// containment guarantee, and the acceptances that stop an implementation from
// passing by refusing everything.
func RunPathPolicy(t *testing.T, policy ports.PathPolicy) {
	t.Helper()
	runLeafIDContract(t, policy)
	runContainmentContract(t, policy)
	runExactPathContract(t, policy)
}

func runLeafIDContract(t *testing.T, policy ports.PathPolicy) {
	t.Helper()
	for _, accepted := range []string{"demo-0123456789ab", "a", "demo_pkg.v2"} {
		if err := policy.ValidateLeafID(accepted); err != nil {
			t.Errorf("ValidateLeafID(%q) rejected a portable leaf name: %v", accepted, err)
		}
	}
	rejected := map[string]string{
		"empty":          "",
		"dot":            ".",
		"parent":         "..",
		"slash":          "a/b",
		"backslash":      `a\b`,
		"absolute":       string(filepath.Separator) + "abs",
		"dot_segment":    "demo..pkg",
		"trailing_dot":   "demo.",
		"windows_device": "CON",
		"null_byte":      "demo\x00pkg",
		"too_long":       strings.Repeat("a", 65),
	}
	for name, value := range rejected {
		if err := policy.ValidateLeafID(value); err == nil {
			t.Errorf("ValidateLeafID rejects nothing for case %s (%q)", name, value)
		}
	}
}

func runContainmentContract(t *testing.T, policy ports.PathPolicy) {
	t.Helper()
	base := t.TempDir()
	child := filepath.Join(base, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := policy.RequireContainedChild(base, child); err != nil {
		t.Errorf("RequireContainedChild rejected a real strict child: %v", err)
	}
	if err := policy.RequireContainedChild(base, base); err == nil {
		t.Error("RequireContainedChild accepted the base itself, which is not a strict child")
	}
	if err := policy.RequireContainedChild(base, filepath.Join(base, "..", "escape")); err == nil {
		t.Error("RequireContainedChild accepted a parent escape")
	}
	if err := policy.RequireContainedChild(base, filepath.Dir(base)); err == nil {
		t.Error("RequireContainedChild accepted an ancestor of the base")
	}
	if err := policy.RequireContainedChild("", child); err == nil {
		t.Error("RequireContainedChild accepted an empty base")
	}
	if link, ok := symlinkedBranch(t, base); ok {
		if err := policy.RequireContainedChild(base, filepath.Join(link, "payload")); err == nil {
			t.Error("RequireContainedChild accepted a path whose ancestor is a symlink")
		}
	}
}

func runExactPathContract(t *testing.T, policy ports.PathPolicy) {
	t.Helper()
	base := t.TempDir()
	expected := filepath.Join(base, "managed", "active")
	if err := os.MkdirAll(expected, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := policy.RequireExactPath(expected, expected); err != nil {
		t.Errorf("RequireExactPath rejected the exact managed path: %v", err)
	}
	if err := policy.RequireExactPath(expected, filepath.Join(base, "managed", "other")); err == nil {
		t.Error("RequireExactPath accepted a sibling of the managed path")
	}
	if err := policy.RequireExactPath(expected, "active"); err == nil {
		t.Error("RequireExactPath accepted a relative path where the managed path is absolute")
	}
	if err := policy.RequireExactPath(expected, ""); err == nil {
		t.Error("RequireExactPath accepted an empty candidate")
	}
	if link, ok := symlinkedBranch(t, filepath.Join(base, "managed")); ok {
		payload := filepath.Join(link, "payload")
		if err := policy.RequireExactPath(payload, payload); err == nil {
			t.Error("RequireExactPath accepted a path whose ancestor is a symlink")
		}
	}
}

// symlinkedBranch reports a path under base whose parent is a symlink. Creating
// one needs a privilege Windows does not always grant, so the caller skips the
// symlink assertions instead of failing on the host's policy.
func symlinkedBranch(t *testing.T, base string) (string, bool) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(filepath.Join(target, "payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Logf("skipping the symlink assertions: %v", err)
		return "", false
	}
	return link, true
}
