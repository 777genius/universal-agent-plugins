package environment

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func reconciliationFixture(t *testing.T) (Requirements, Lock) {
	t.Helper()
	req := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"a": {Alias: "a", Registry: &RegistrySource{Distribution: "acme/a", Selector: Selector{Kind: SelectorRange, Value: "^1.2"}}}}, Targets: map[string][]string{"codex": {"a"}}}
	makePin := func(alias string) LockedPackage {
		return LockedPackage{Alias: alias, SourceKey: "directory:acme/" + alias, Version: "1.2.0", DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Targets: []string{"codex"}, Source: LockedSource{Registry: &LockedRegistrySource{Distribution: "acme/" + alias, DistributionID: "acme/" + alias, ReleaseSequence: 1, Requested: Selector{Kind: SelectorRelease, Value: "1"}, Repository: "acme/source", Path: alias, Commit: strings.Repeat("1", 40)}}}
	}
	a, c := makePin("a"), makePin("c")
	a.Source.Registry.Requested = req.Dependencies["a"].Registry.Selector
	c.RequiredBy = []RequiredBy{{Alias: "a", Selector: Selector{Kind: SelectorRange, Value: "^1.2"}}}
	return req, Lock{LockVersion: 1, ManifestDigest: mustDigest(t, req), Packages: []LockedPackage{a, c}}
}

// Changing desired intent must invalidate replay without invalidating the old
// immutable pins as input to a fresh resolution.
func TestReconciliationIntentChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Requirements)
	}{
		{"range", func(r *Requirements) { r.Dependencies["a"].Registry.Selector.Value = "^1.3" }},
		{"targets", func(r *Requirements) { r.Targets["claude"] = []string{"a"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, lock := reconciliationFixture(t)
			original := lock.ManifestDigest
			tc.change(&req)
			if mustDigest(t, req) == original {
				t.Fatal("changed intent retained digest")
			}
			if err := lock.Validate(req); err == nil {
				t.Fatal("changed intent accepted for replay")
			}
			if err := lock.ValidateCompatiblePins(req); err != nil {
				t.Fatalf("old immutable pins rejected before resolution: %v", err)
			}
		})
	}
}

type reconciliationMutator struct{ calls int }

func (m *reconciliationMutator) Prepare(context.Context, PreparedBinding) (*installer.PreparedOperation, error) {
	m.calls++
	return nil, nil
}
func (m *reconciliationMutator) Apply(context.Context, *installer.PreparedOperation) (installer.Result, error) {
	m.calls++
	return installer.Result{}, nil
}

// Malformed graphs must fail at planning, before a mutator can prepare effects.
func TestReconciliationRejectsGraphStructure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Lock)
	}{
		{"missing root", func(l *Lock) { l.Packages = l.Packages[1:] }},
		{"dangling consumer", func(l *Lock) { l.Packages[1].RequiredBy[0].Alias = "absent" }},
		{"duplicate edge", func(l *Lock) {
			l.Packages[1].RequiredBy = append(l.Packages[1].RequiredBy, l.Packages[1].RequiredBy[0])
		}},
		{"orphan", func(l *Lock) { l.Packages[1].RequiredBy = nil }},
		{"missing inherited target", func(l *Lock) { l.Packages[1].Targets = nil }},
		{"extra inherited target", func(l *Lock) { l.Packages[1].Targets = append(l.Packages[1].Targets, "claude") }},
		{"duplicate source", func(l *Lock) { l.Packages[1].SourceKey = l.Packages[0].SourceKey }},
		{"unsatisfied edge", func(l *Lock) { l.Packages[1].RequiredBy[0].Selector.Value = "^2.0" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, lock := reconciliationFixture(t)
			if err := lock.Validate(req); err != nil {
				t.Fatalf("invalid baseline graph: %v", err)
			}
			tc.change(&lock)
			mutator := &reconciliationMutator{}
			_, err := (Coordinator{Mutator: mutator}).PrepareSync(req, lock, nil)
			if err == nil {
				t.Fatal("invalid graph accepted")
			}
			if mutator.calls != 0 {
				t.Fatal("invalid graph reached mutator")
			}
		})
	}
}

// Internal identity is checked independently of the current manifest digest.
func TestReconciliationRejectsPinIdentity(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*LockedPackage)
	}{
		{"source key", func(p *LockedPackage) { p.SourceKey = "directory:acme/other" }},
		{"commit", func(p *LockedPackage) { p.Source.Registry.Commit = "main" }},
		{"release", func(p *LockedPackage) { p.Source.Registry.ReleaseSequence = 0 }},
		{"requested release", func(p *LockedPackage) { p.Source.Registry.Requested = Selector{Kind: SelectorRelease, Value: "2"} }},
		{"version", func(p *LockedPackage) { p.Version = "2.0.0" }},
		{"tree digest", func(p *LockedPackage) { p.TreeDigest = "sha256:bad" }},
		{"manifest digest", func(p *LockedPackage) { p.ManifestDigest = "sha256:bad" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, lock := reconciliationFixture(t)
			if err := lock.Validate(req); err != nil {
				t.Fatalf("invalid baseline graph: %v", err)
			}
			if tc.name == "requested release" {
				tc.change(&lock.Packages[1])
			} else {
				tc.change(&lock.Packages[0])
			}
			if err := lock.Validate(req); err == nil {
				t.Fatal("inconsistent pin identity accepted")
			}
		})
	}
}
