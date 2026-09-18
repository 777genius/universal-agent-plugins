package main

import (
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The Part 12 plan extracts domain, ports and clients into their own Go
// module. See docs/plans/installer-core-part-12-contract-module.md §12.1(c)
// for the dependency snapshot this test enforces and §12.1.G for the guard
// test's specification.
const (
	contractPortsImportPath   = modulePath + "/install/integrationctl/agentplugins/ports"
	contractClientsImportPath = modulePath + "/install/integrationctl/agentplugins/clients"
	nativeconfigImportPath    = modulePath + "/install/integrationctl/agentplugins/adapters/nativeconfig"
	legacyDomainImportPath    = modulePath + "/install/integrationctl/domain"
	atomicfileImportPath      = modulePath + "/install/integrationctl/adapters/atomicfile"
	pathpolicyImportPath      = modulePath + "/install/integrationctl/adapters/pathpolicy"
)

// contractDirs are the three top-level packages Part 12 extracts, keyed by
// import path. Only files directly inside these directories count: per-client
// adapters (clients/<id>), clients/shared, clients/all and the *contracttest
// packages are excluded on purpose, per §12.1.G - none of them travel with
// the future module's non-test/test closure.
var contractDirs = map[string]string{
	domainImportPath:          agentplugins + "/domain",
	contractPortsImportPath:   agentplugins + "/ports",
	contractClientsImportPath: agentplugins + "/clients",
}

// contractNonTestEdges is invariant 1's production half (§12.1.G, exception
// A-1 and A-2): domain has no entry because its production code has zero
// non-stdlib imports today.
var contractNonTestEdges = map[string][]string{
	contractPortsImportPath:   {legacyPortsPath},
	contractClientsImportPath: {nativeconfigImportPath},
}

// contractTestEdges is invariant 1's test half. It exists to catch Находка 4:
// agentplugins/domain has no non-test dependency outside the contract layer,
// but identity_portable_test.go imports adapters/pathpolicy to check that
// domain-generated leaf IDs satisfy the path policy. The edge is allowed to
// stay here and is scheduled to move to the consumer side in Part 12a (plan
// §12.2.D, variant D1); it is deliberately not fixed by this change.
var contractTestEdges = map[string][]string{
	domainImportPath: {pathpolicyImportPath},
}

// contractClosure is invariant 2: the full transitive, in-repository closure
// reachable from the contract layer's allowed edges above.
var contractClosure = []string{
	domainImportPath,
	contractPortsImportPath,
	contractClientsImportPath,
	nativeconfigImportPath,
	legacyPortsPath,
	legacyDomainImportPath,
	atomicfileImportPath,
	pathpolicyImportPath,
}

// contractExternalModules is invariant 3: every non-stdlib, non-repository
// module reachable from the contract layer on any platform in platformSweep,
// normalized to its first three import-path segments (the module root for
// both github.com/<owner>/<repo> and golang.org/x/<pkg> paths).
var contractExternalModules = []string{
	"github.com/tailscale/hujson",
	"golang.org/x/sys",
	"golang.org/x/text",
}

// platformSweep lists every GOOS the closure is recomputed under.
// adapters/nativeconfig gates golang.org/x/sys behind lock_windows.go and
// open_nofollow_windows.go: a single-platform scan sees it only on Windows
// and stays permanently blind to a dependency gated behind any other
// platform's file - exactly the class of drift this test exists to catch.
// adapters/atomicfile is stdlib-only on every GOOS, including
// syncdir_windows.go. GOARCH is fixed at amd64 because none of these
// packages carry architecture-specific files today; CgoEnabled is off
// because none use cgo.
var platformSweep = []string{"linux", "darwin", "windows"}

// TestContractLayerDependencyClosure is the guard test from plan §12.1.G: it
// fails the moment domain, ports, clients, or any package they already reach,
// gains a dependency outside the sets declared above, so drift introduced in
// Part 5-9 is caught on the introducing PR instead of at the start of Part 12.
func TestContractLayerDependencyClosure(t *testing.T) {
	t.Parallel()
	root := testRepoRoot(t)

	gotNonTestEdges := make(map[string][]string, len(contractDirs))
	gotTestEdges := make(map[string][]string, len(contractDirs))
	closure := map[string]bool{}
	external := map[string]bool{}

	for _, goos := range platformSweep {
		ctxt := build.Default
		ctxt.GOOS = goos
		ctxt.GOARCH = "amd64"
		ctxt.CgoEnabled = false

		snapshot, err := computeContractSnapshot(root, ctxt)
		if err != nil {
			t.Fatalf("GOOS=%s: %v", goos, err)
		}
		for source, targets := range snapshot.nonTestEdges {
			gotNonTestEdges[source] = append(gotNonTestEdges[source], targets...)
		}
		for source, targets := range snapshot.testEdges {
			gotTestEdges[source] = append(gotTestEdges[source], targets...)
		}
		markAll(closure, setKeys(snapshot.closure))
		markAll(external, setKeys(snapshot.external))
	}

	assertExactEdges(t, "production", gotNonTestEdges, contractNonTestEdges)
	assertExactEdges(t, "test", gotTestEdges, contractTestEdges)
	assertExactSet(t, "contract layer transitive closure", setKeys(closure), contractClosure)
	assertExactSet(t, "contract layer external modules", setKeys(external), contractExternalModules)
}

// contractSnapshot is one platform's dependency picture of the contract
// layer: its outgoing edges (invariant 1) and the transitive closure and
// external modules reachable through them (invariants 2 and 3).
type contractSnapshot struct {
	nonTestEdges map[string][]string
	testEdges    map[string][]string
	closure      map[string]bool
	external     map[string]bool
}

// computeContractSnapshot walks the contract layer's own files, then expands
// the closure through the allowed edges' production imports - their test
// files are not part of the future module and are skipped on purpose.
func computeContractSnapshot(root string, ctxt build.Context) (contractSnapshot, error) {
	snapshot := contractSnapshot{
		nonTestEdges: make(map[string][]string, len(contractDirs)),
		testEdges:    make(map[string][]string, len(contractDirs)),
		closure:      map[string]bool{},
		external:     map[string]bool{},
	}
	for member := range contractDirs {
		snapshot.closure[member] = true
	}
	var frontier []string
	enqueue := func(targets []string) {
		for _, target := range targets {
			if !snapshot.closure[target] {
				frontier = append(frontier, target)
			}
		}
	}

	for importPath, rel := range contractDirs {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		production, tests, err := packageImports(ctxt, dir, true)
		if err != nil {
			return contractSnapshot{}, err
		}

		repoTargets, externalMods := classifyImports(production)
		snapshot.nonTestEdges[importPath] = repoTargets
		markAll(snapshot.external, externalMods)
		enqueue(repoTargets)

		testRepoTargets, testExternalMods := classifyImports(tests)
		snapshot.testEdges[importPath] = testRepoTargets
		markAll(snapshot.external, testExternalMods)
		enqueue(testRepoTargets)
	}

	for len(frontier) > 0 {
		next := frontier[0]
		frontier = frontier[1:]
		if snapshot.closure[next] {
			continue
		}
		snapshot.closure[next] = true

		dir, err := repoDirForImportPath(root, next)
		if err != nil {
			return contractSnapshot{}, err
		}
		production, _, err := packageImports(ctxt, dir, false)
		if err != nil {
			return contractSnapshot{}, err
		}
		repoTargets, externalMods := classifyImports(production)
		markAll(snapshot.external, externalMods)
		enqueue(repoTargets)
	}

	return snapshot, nil
}

// packageImports parses every immediate (non-recursive) .go file in dir that
// ctxt would actually compile, split into production and _test.go imports.
// Filtering through ctxt.MatchFile matters here: adapters/nativeconfig
// carries GOOS-suffixed files (lock_windows.go pulls in
// golang.org/x/sys/windows). A plain directory walk would report that
// module on every platform; `go list` - and this test, once ctxt sweeps
// GOOS - only reports it for the platform that compiles the file.
func packageImports(ctxt build.Context, dir string, includeTests bool) (production, test []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		isTest := strings.HasSuffix(entry.Name(), "_test.go")
		if isTest && !includeTests {
			continue
		}
		matched, err := ctxt.MatchFile(dir, entry.Name())
		if err != nil {
			return nil, nil, err
		}
		if !matched {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.ImportsOnly|parser.SkipObjectResolution)
		if err != nil {
			return nil, nil, err
		}
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, nil, err
			}
			if isTest {
				test = append(test, value)
			} else {
				production = append(production, value)
			}
		}
	}
	return production, test, nil
}

// classifyImports drops stdlib and in-contract-layer self-references, then
// splits the rest into repository packages outside the contract layer and
// normalized external modules.
func classifyImports(imports []string) (repoTargets, externalModules []string) {
	seenRepo := map[string]bool{}
	seenExternal := map[string]bool{}
	for _, imp := range imports {
		if standardLibrary(imp) {
			continue
		}
		if strings.HasPrefix(imp, modulePath+"/") {
			if _, inContract := contractDirs[imp]; inContract {
				continue
			}
			if !seenRepo[imp] {
				seenRepo[imp] = true
				repoTargets = append(repoTargets, imp)
			}
			continue
		}
		normalized := normalizeExternalModule(imp)
		if !seenExternal[normalized] {
			seenExternal[normalized] = true
			externalModules = append(externalModules, normalized)
		}
	}
	return repoTargets, externalModules
}

// normalizeExternalModule collapses an import path to its first three
// segments, which is the module root for both github.com/<owner>/<repo> and
// golang.org/x/<pkg> paths - the only two shapes the contract layer reaches.
func normalizeExternalModule(importPath string) string {
	segments := strings.SplitN(importPath, "/", 4)
	if len(segments) > 3 {
		segments = segments[:3]
	}
	return strings.Join(segments, "/")
}

// repoDirForImportPath converts a repository import path back to the
// on-disk directory beneath root, mirroring how domainImportPath and
// legacyPortsPath already map to their directories elsewhere in this package.
func repoDirForImportPath(root, importPath string) (string, error) {
	rel := strings.TrimPrefix(importPath, modulePath+"/")
	if rel == importPath {
		return "", fmt.Errorf("%s is not a repository import path under %s", importPath, modulePath)
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

func markAll(set map[string]bool, values []string) {
	for _, v := range values {
		set[v] = true
	}
}

func setKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	return keys
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

func sortedUnique(values []string) []string {
	set := toSet(values)
	result := make([]string, 0, len(set))
	for v := range set {
		result = append(result, v)
	}
	sort.Strings(result)
	return result
}

// assertExactEdges reports both directions of drift with the message shape
// §12.1.G asks for: a new edge names what appeared, a missing edge names what
// the baseline still expects. It walks the union of got's and want's source
// keys, not just got's: a source that vanished from got entirely (contractDirs
// shrinking without a matching update to the expectation maps) must still be
// checked against a non-empty want, or the guard would go quiet instead of
// failing.
func assertExactEdges(t *testing.T, kind string, got, want map[string][]string) {
	t.Helper()
	sources := sortedUnique(append(mapStringKeys(got), mapStringKeys(want)...))
	for _, source := range sources {
		gotSet := toSet(got[source])
		wantSet := toSet(want[source])
		for _, target := range sortedUnique(got[source]) {
			if !wantSet[target] {
				t.Errorf("contract layer: new %s import %s -> %s; if intentional, update §12.1(c) of docs/plans/installer-core-part-12-contract-module.md and this test's baseline, otherwise remove the import", kind, source, target)
			}
		}
		for _, target := range sortedUnique(want[source]) {
			if !gotSet[target] {
				t.Errorf("contract layer: expected %s import %s -> %s is gone; update the plan and this test's baseline if the edge was intentionally removed", kind, source, target)
			}
		}
	}
}

func mapStringKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func assertExactSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	gotSet := toSet(got)
	wantSet := toSet(want)
	for _, member := range sortedUnique(got) {
		if !wantSet[member] {
			t.Errorf("%s: new member %s; if intentional, update §12.1(c) of docs/plans/installer-core-part-12-contract-module.md and this test's baseline, otherwise remove the dependency", label, member)
		}
	}
	for _, member := range sortedUnique(want) {
		if !gotSet[member] {
			t.Errorf("%s: expected member %s is missing; update the plan and this test's baseline if it was intentionally removed", label, member)
		}
	}
}
