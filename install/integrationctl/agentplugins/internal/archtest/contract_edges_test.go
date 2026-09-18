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
// module reachable from the contract layer, normalized to its first three
// import-path segments (the module root for both github.com/<owner>/<repo>
// and golang.org/x/<pkg> paths).
var contractExternalModules = []string{
	"github.com/tailscale/hujson",
	"golang.org/x/text",
}

// TestContractLayerDependencyClosure is the guard test from plan §12.1.G: it
// fails the moment domain, ports, clients, or any package they already reach,
// gains a dependency outside the sets declared above, so drift introduced in
// Part 5-9 is caught on the introducing PR instead of at the start of Part 12.
func TestContractLayerDependencyClosure(t *testing.T) {
	t.Parallel()
	root := testRepoRoot(t)

	closure := map[string]bool{}
	for member := range contractDirs {
		closure[member] = true
	}
	external := map[string]bool{}
	var frontier []string
	enqueue := func(targets []string) {
		for _, target := range targets {
			if !closure[target] {
				frontier = append(frontier, target)
			}
		}
	}

	gotNonTestEdges := make(map[string][]string, len(contractDirs))
	gotTestEdges := make(map[string][]string, len(contractDirs))
	for importPath, rel := range contractDirs {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		production, tests, err := packageImports(dir, true)
		if err != nil {
			t.Fatal(err)
		}

		repoTargets, externalMods := classifyImports(production)
		gotNonTestEdges[importPath] = repoTargets
		markAll(external, externalMods)
		enqueue(repoTargets)

		testRepoTargets, testExternalMods := classifyImports(tests)
		gotTestEdges[importPath] = testRepoTargets
		markAll(external, testExternalMods)
		enqueue(testRepoTargets)
	}

	assertExactEdges(t, "production", gotNonTestEdges, contractNonTestEdges)
	assertExactEdges(t, "test", gotTestEdges, contractTestEdges)

	// Invariant 2/3 expand the closure through the already-allowed edges'
	// own production imports. Their test files are not part of the future
	// module and are skipped on purpose.
	for len(frontier) > 0 {
		next := frontier[0]
		frontier = frontier[1:]
		if closure[next] {
			continue
		}
		closure[next] = true

		dir, err := repoDirForImportPath(root, next)
		if err != nil {
			t.Fatal(err)
		}
		production, _, err := packageImports(dir, false)
		if err != nil {
			t.Fatal(err)
		}
		repoTargets, externalMods := classifyImports(production)
		markAll(external, externalMods)
		enqueue(repoTargets)
	}

	assertExactSet(t, "contract layer transitive closure", setKeys(closure), contractClosure)
	assertExactSet(t, "contract layer external modules", setKeys(external), contractExternalModules)
}

// packageImports parses every immediate (non-recursive) .go file in dir that
// the current build context would actually compile, split into production
// and _test.go imports. Filtering through build.Default.MatchFile matters
// here: adapters/nativeconfig and adapters/atomicfile carry GOOS-suffixed
// files (lock_windows.go pulls in golang.org/x/sys/windows), and a plain
// directory walk would report that module on every platform except the one
// building it, whereas `go list` - and this test - only report it on
// Windows.
func packageImports(dir string, includeTests bool) (production, test []string, err error) {
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
		matched, err := build.Default.MatchFile(dir, entry.Name())
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
// the baseline still expects.
func assertExactEdges(t *testing.T, kind string, got, want map[string][]string) {
	t.Helper()
	for source, targets := range got {
		gotSet := toSet(targets)
		wantSet := toSet(want[source])
		for _, target := range sortedUnique(targets) {
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
