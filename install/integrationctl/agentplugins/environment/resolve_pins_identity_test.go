package environment

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPinnedContractionRejectsChangedImmutableFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*LockedPackage)
	}{
		{"alias", func(pin *LockedPackage) { pin.Alias = "different" }},
		{"version", func(pin *LockedPackage) { pin.Version = "1.1.0" }},
		{"source key", func(pin *LockedPackage) { pin.SourceKey = "directory:acme/different" }},
		{"source release", func(pin *LockedPackage) { pin.Source.Registry.ReleaseSequence++ }},
		{"source commit", func(pin *LockedPackage) { pin.Source.Registry.Commit = strings.Repeat("f", 40) }},
		{"source product", func(pin *LockedPackage) { pin.Source.Registry.ProductID = "different" }},
		{"digest algorithm", func(pin *LockedPackage) { pin.DigestAlgorithm = "different" }},
		{"tree digest", func(pin *LockedPackage) { pin.TreeDigest = "sha256:" + strings.Repeat("f", 64) }},
		{"manifest digest", func(pin *LockedPackage) { pin.ManifestDigest = "sha256:" + strings.Repeat("f", 64) }},
		{"executable inventory", func(pin *LockedPackage) { pin.ExecutableFiles = []string{"run.sh"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "independent", 1)
			resolver.describe = func(_ context.Context, pin LockedPackage) (ResolvedPackage, error) {
				source := *pin.Source.Registry
				pin.Source.Registry = &source
				tc.change(&pin)
				return ResolvedPackage{Package: pin}, nil
			}
			solver := newGraphSolver(resolver, requirements, previous, nil)
			choices, nodes, complete, err := solver.tryPinnedContraction(t.Context())
			if err == nil || !strings.Contains(err.Error(), "changed immutable identity") || errors.Is(err, ErrDependencyConflict) || complete || choices != nil || nodes != nil || len(solver.descriptions) != 0 {
				t.Fatalf("immutable change was accepted, retried or cached: %v %v", complete, err)
			}
			assertContractionCallbacks(t, resolver, 1)
		})
	}
}

func TestPinnedContractionHydratesOnlyAbsentVersion(t *testing.T) {
	requirements, previous, resolver := contractionFixture(t, "independent", 1)
	root := requirements.Dependencies["node0000"]
	root.Registry.Selector = Selector{Kind: SelectorRelease, Value: "1"}
	requirements.Dependencies["node0000"] = root
	setRequested(&previous.Packages[0], root)
	previous.Packages[0].Version = ""
	if err := previous.ValidateCompatiblePins(requirements); err != nil {
		t.Fatal(err)
	}
	resolver.describe = func(_ context.Context, pin LockedPackage) (ResolvedPackage, error) {
		pin.Version = "1.0.0"
		// Graph intent returned by the provider is never immutable authority.
		pin.Targets = []string{"claude"}
		pin.RequiredBy = []RequiredBy{{Alias: "unrelated"}}
		source := *pin.Source.Registry
		source.Requested = Selector{Kind: SelectorVersion, Value: "1.0.0"}
		pin.Source.Registry = &source
		return ResolvedPackage{Package: pin}, nil
	}
	solver := newGraphSolver(resolver, requirements, previous, nil)
	choices, _, complete, err := solver.tryPinnedContraction(t.Context())
	if err != nil || !complete {
		t.Fatalf("legacy hydration refused: %v %v", complete, err)
	}
	selected := choices[previous.Packages[0].SourceKey].Package
	expected := previous.Packages[0]
	expected.Version = "1.0.0"
	if !SamePin(expected, selected) || selected.Source.Registry.Requested != root.Registry.Selector || len(selected.RequiredBy) != 0 {
		t.Fatalf("hydration changed other historical facts: %+v", selected)
	}
}

func TestPinnedContractionKeepsStableAliasAfterRootRename(t *testing.T) {
	requirements, previous, resolver := contractionFixture(t, "independent", 1)
	req := requirements.Dependencies["node0000"]
	req.Alias = "logical-renamed"
	delete(requirements.Dependencies, "node0000")
	requirements.Dependencies[req.Alias] = req
	requirements.Targets["codex"] = []string{req.Alias}
	solver := newGraphSolver(resolver, requirements, previous, nil)
	choices, nodes, complete, err := solver.tryPinnedContraction(t.Context())
	if err != nil || !complete || len(nodes) != 1 {
		t.Fatalf("logical rename refused: %v %v", complete, err)
	}
	key := previous.Packages[0].SourceKey
	if nodes[key].requirement.Alias != "node0000" || !SamePin(previous.Packages[0], choices[key].Package) {
		t.Fatal("logical rename changed ownership identity")
	}
}

func TestPinnedContractionDescriptionFailuresAreTerminal(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "operational failure", true: "malformed declaration"}[malformed], func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "independent", 1)
			failure := errors.New("retained snapshot cleanup failed")
			resolver.describe = func(_ context.Context, pin LockedPackage) (ResolvedPackage, error) {
				if !malformed {
					return ResolvedPackage{}, failure
				}
				return ResolvedPackage{Package: pin, Dependencies: map[string]Requirement{"wrong-key": conflictRequirement("child", "^1.0")}}, nil
			}
			solver := newGraphSolver(resolver, requirements, previous, nil)
			choices, nodes, complete, err := solver.tryPinnedContraction(t.Context())
			if err == nil || errors.Is(err, ErrDependencyConflict) || complete || choices != nil || nodes != nil || len(solver.descriptions) != 0 {
				t.Fatalf("failed immutable description was retained: %v %v", complete, err)
			}
			if !malformed && !errors.Is(err, failure) {
				t.Fatalf("operational error identity lost: %v", err)
			}
			assertContractionCallbacks(t, resolver, 1)
		})
	}
}
