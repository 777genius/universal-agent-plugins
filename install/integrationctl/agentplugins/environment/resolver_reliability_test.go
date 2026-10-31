package environment

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"testing"
)

// These bounded property seeds also run during ordinary "go test"; scheduled
// Go fuzzing explores more candidates without real network or accounts.
func FuzzVersionSelectionOrderAndPinReplay(f *testing.F) {
	for _, seed := range [][3]byte{{0, 0, 0}, {2, 7, 1}, {15, 25, 2}, {255, 255, 255}} {
		f.Add(seed[0], seed[1], seed[2])
	}
	f.Fuzz(func(t *testing.T, minorByte, patchByte, orderByte byte) {
		high := fmt.Sprintf("1.%d.%d", int(minorByte%15)+3, int(patchByte%24))
		candidates := []Candidate{
			{Version: "1.2.0", ReleaseSequence: 100},
			{Version: high, ReleaseSequence: 1},
			{Version: high, ReleaseSequence: 4},
		}
		switch orderByte % 3 {
		case 1:
			candidates[0], candidates[2] = candidates[2], candidates[0]
		case 2:
			candidates[0], candidates[1] = candidates[1], candidates[0]
		}
		resolver := &versionSelectionResolver{candidates: map[string][]Candidate{"acme/a": candidates}}
		requirements := versionSelectionRequirements()
		coordinator := Coordinator{Resolver: resolver}
		first, err := coordinator.Resolve(context.Background(), requirements, Lock{}, "")
		if err != nil {
			t.Fatal(err)
		}
		if err := first.Validate(requirements); err != nil {
			t.Fatalf("selected graph invalid: %v", err)
		}
		pin, ok := first.Package("a")
		if !ok || len(first.Packages) != 1 || pin.Version != high || pin.Source.Registry == nil || pin.Source.Registry.ReleaseSequence != 4 {
			t.Fatalf("selection depends on candidate ordering: %+v, want %s #4", first, high)
		}
		enumerated := resolver.enumerated
		replay, err := coordinator.Resolve(context.Background(), requirements, first, "")
		if err != nil || !reflect.DeepEqual(first, replay) || resolver.enumerated != enumerated {
			t.Fatalf("compatible pin replay changed: %+v vs %+v, enumerations %d->%d, error %v",
				first, replay, enumerated, resolver.enumerated, err)
		}
	})
}

// Compare the production fixed-point closure with an independent BFS oracle.
// Include cycles, shared children, orphaned nodes and arbitrary package order.
func FuzzDependencyClosureMatchesTraversal(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5}, byte(0))
	f.Add([]byte{255}, byte(7))
	f.Add([]byte{}, byte(2))
	f.Add([]byte{129, 77, 6, 203}, byte(4))
	f.Fuzz(func(t *testing.T, data []byte, root byte) {
		n := 2 + int(root%7)
		aliases := make([]string, n)
		for i := range aliases {
			aliases[i] = string(rune('a' + i))
		}
		packages := make([]LockedPackage, n)
		adjacency := make(map[string][]string, n)
		for child := range packages {
			packages[child].Alias = aliases[child]
			for parent := range packages {
				index := child*n + parent
				if len(data) == 0 || data[index%len(data)]&(byte(1)<<uint(index%8)) == 0 {
					continue
				}
				packages[child].RequiredBy = append(packages[child].RequiredBy, RequiredBy{Alias: aliases[parent]})
				adjacency[aliases[parent]] = append(adjacency[aliases[parent]], aliases[child])
			}
		}
		start := aliases[int(root)%n]
		seen := map[string]bool{start: true}
		queue := []string{start}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, child := range adjacency[current] {
				if !seen[child] {
					seen[child] = true
					queue = append(queue, child)
				}
			}
		}
		expected := make([]string, 0, len(seen))
		for alias := range seen {
			expected = append(expected, alias)
		}
		sort.Strings(expected)
		lock := Lock{Packages: packages}
		if actual := lock.ClosureAliases([]string{start}); !slices.Equal(actual, expected) {
			t.Fatalf("closure %v != expected BFS %v for %v", actual, expected, packages)
		}
		slices.Reverse(packages)
		if actual := lock.ClosureAliases([]string{start}); !slices.Equal(actual, expected) {
			t.Fatalf("permuted package order changed closure: %v != %v", actual, expected)
		}
	})
}

// Compare graph-solver allocation and latency trends without hard-coded wall
// time thresholds that fluctuate on GitHub-hosted runners.
func BenchmarkVersionSelectionSharedGraph(b *testing.B) {
	for _, width := range []int{8, 24, 48} {
		b.Run(fmt.Sprint(width), func(b *testing.B) {
			requirements := versionSelectionRequirements()
			children := make(map[string]string, width)
			aliases := make(map[string]string, width)
			candidates := map[string][]Candidate{
				"acme/a": {{Version: "1.3.0", ReleaseSequence: 2}},
			}
			for i := range width {
				name := fmt.Sprintf("acme/c%02d", i)
				aliases[name] = fmt.Sprintf("c%02d", i)
				children[name] = "^1.2"
				candidates[name] = []Candidate{{Version: "1.2.0", ReleaseSequence: 1}}
			}
			resolver := &versionSelectionResolver{
				candidates: candidates,
				dependencies: map[string]map[string]string{"acme/a@1.3.0": children},
				manifestAliases: aliases,
			}
			coordinator := Coordinator{Resolver: resolver}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				lock, err := coordinator.Resolve(context.Background(), requirements, Lock{}, "")
				if err != nil {
					b.Fatal(err)
				}
				if len(lock.Packages) != width+1 {
					b.Fatalf("shared graph selected %d packages, want %d", len(lock.Packages), width+1)
				}
			}
		})
	}
}
