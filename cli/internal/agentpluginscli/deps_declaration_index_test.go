package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDependencyDeclarationIndexPreservesEdgeEvidence(t *testing.T) {
	declared := environment.Selector{Kind: environment.SelectorRange, Value: "^1.0"}
	other := environment.Selector{Kind: environment.SelectorRange, Value: "^1.2"}
	requirement := environment.Requirement{Alias: "child", Registry: &environment.RegistrySource{Distribution: "acme/child", Selector: declared}}
	declarations := map[string]environment.Requirement{"child": requirement}
	for _, tc := range []struct {
		name         string
		edges        []environment.RequiredBy
		declarations map[string]environment.Requirement
		want         string
	}{
		{name: "matching", edges: []environment.RequiredBy{{Alias: "parent", Selector: declared}}, declarations: declarations},
		{name: "identical_duplicates", edges: []environment.RequiredBy{{Alias: "parent", Selector: declared}, {Alias: "parent", Selector: declared}}, declarations: declarations, want: "unexpected"},
		{name: "distinct_selectors_declared_first", edges: []environment.RequiredBy{{Alias: "parent", Selector: declared}, {Alias: "parent", Selector: other}}, declarations: declarations, want: "unexpected"},
		{name: "distinct_selectors_declared_last", edges: []environment.RequiredBy{{Alias: "parent", Selector: other}, {Alias: "parent", Selector: declared}}, declarations: declarations, want: "unexpected"},
		{name: "incompatible_selector", edges: []environment.RequiredBy{{Alias: "parent", Selector: other}}, declarations: declarations, want: "unexpected"},
		{name: "shared_parents", edges: []environment.RequiredBy{{Alias: "other", Selector: other}, {Alias: "parent", Selector: declared}}, declarations: declarations},
		{name: "empty"},
		{name: "empty_with_recorded_edge", edges: []environment.RequiredBy{{Alias: "parent", Selector: declared}}, want: "unexpected"},
		{name: "missing_edge", declarations: declarations, want: "missing"},
		{name: "duplicate_source_declarations", edges: []environment.RequiredBy{{Alias: "parent", Selector: declared}}, declarations: map[string]environment.Requirement{"child": requirement, "same": requirement}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index, err := indexDependencyDeclarations(t.Context(), environment.Lock{Packages: []environment.LockedPackage{{Alias: "child", SourceKey: "directory:acme/child", RequiredBy: tc.edges}}})
			if err != nil {
				t.Fatal(err)
			}
			err = index.verifyDeclaredEdges(t.Context(), "parent", tc.declarations)
			want := ""
			switch tc.want {
			case "unexpected":
				want = "parent has an unexpected dependency edge to child; run agentplugins deps lock"
			case "missing":
				want = "parent pinned manifest declares missing dependency edges; run agentplugins deps lock"
			}
			if want == "" && err != nil || want != "" && (err == nil || err.Error() != want) {
				t.Fatalf("edge evidence: got %v, want %q", err, want)
			}
			if tc.name == "shared_parents" {
				requirement.Registry = &environment.RegistrySource{Distribution: "acme/child", Selector: other}
				if err := index.verifyDeclaredEdges(t.Context(), "other", map[string]environment.Requirement{"child": requirement}); err != nil {
					t.Fatalf("shared parent's distinct selector: %v", err)
				}
			}
		})
	}
	index, err := indexDependencyDeclarations(t.Context(), environment.Lock{})
	if err != nil {
		t.Fatal(err)
	}
	if err := index.verifyDeclaredEdges(t.Context(), "parent", declarations); err == nil || !strings.Contains(err.Error(), "missing dependency edges") {
		t.Fatalf("absent child source: %v", err)
	}
}

func TestDependencyCheckDistinctSelectorEvidenceCLI(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		t.Run(fmt.Sprintf("reversed=%t", reversed), func(t *testing.T) {
			f := newDependencyCheckFixture(t)
			parent := seedDependencyCheckPackage(t, f, "parent", "schema_version = 1\n[dependencies]\n\"acme/child\" = \"^1.0\"\n")
			child := seedDependencyCheckPackage(t, f, "child", "")
			child.SourceKey = "directory:acme/child"
			child.Source = environment.LockedSource{Registry: &environment.LockedRegistrySource{Distribution: "acme/child", DistributionID: "acme/child", ProductID: "child", ReleaseSequence: 1, Requested: environment.Selector{Kind: environment.SelectorRelease, Value: "1"}, Repository: "acme/child", Commit: strings.Repeat("a", 40)}}
			child.RequiredBy = []environment.RequiredBy{{Alias: "parent", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.0"}}, {Alias: "parent", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.0.0"}}}
			if reversed {
				child.RequiredBy[0], child.RequiredBy[1] = child.RequiredBy[1], child.RequiredBy[0]
			}
			writeDependencyCheckAgreement(t, f, []environment.LockedPackage{parent, child}, "parent")
			before := observeDependencyCheckFiles(t, f.root)
			report, _ := executeDependencyCheck(t, f, dependencyCheckInvalid)
			requireDependencyCheckCode(t, report, "graph_declarations_mismatch")
			for _, check := range report.Checks {
				if check.Code != "" && check.Code != "graph_declarations_mismatch" {
					t.Fatalf("selector evidence failed before declaration comparison: %+v", report)
				}
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

type dependencyIndexCancelContext struct {
	context.Context
	remaining int
}

func (ctx *dependencyIndexCancelContext) Err() error {
	ctx.remaining--
	if ctx.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestDependencyDeclarationIndexCancellation(t *testing.T) {
	lock := environment.Lock{Packages: []environment.LockedPackage{{Alias: "parent", SourceKey: "directory:acme/parent"}}}
	declarations := make(map[string]environment.Requirement, 12000)
	selector := environment.Selector{Kind: environment.SelectorRange, Value: "^1.0"}
	for i := range 12000 {
		alias := fmt.Sprintf("child%d", i)
		lock.Packages = append(lock.Packages, environment.LockedPackage{Alias: alias, SourceKey: "directory:acme/" + alias, RequiredBy: []environment.RequiredBy{{Alias: "parent", Selector: selector}}})
		declarations[alias] = environment.Requirement{Alias: alias, Registry: &environment.RegistrySource{Distribution: "acme/" + alias, Selector: selector}}
	}
	ctx := &dependencyIndexCancelContext{Context: t.Context(), remaining: 50}
	if _, err := indexDependencyDeclarations(ctx, lock); !errors.Is(err, context.Canceled) {
		t.Fatalf("large index ignored cancellation: %v", err)
	}
	index, err := indexDependencyDeclarations(t.Context(), lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.verifyDeclaredEdges(t.Context(), "parent", declarations); err != nil {
		t.Fatalf("admitted graph inherited a fresh-search ceiling: %v", err)
	}
	for _, remaining := range []int{50, len(declarations) + 50} {
		ctx = &dependencyIndexCancelContext{Context: t.Context(), remaining: remaining}
		if err := index.verifyDeclaredEdges(ctx, "parent", declarations); !errors.Is(err, context.Canceled) {
			t.Fatalf("large declaration/edge comparison ignored cancellation: %v", err)
		}
	}
}
