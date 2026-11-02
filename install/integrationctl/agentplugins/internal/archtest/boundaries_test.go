package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const agentplugins = "install/integrationctl/agentplugins"

// boundary mirrors one depguard rule from .golangci.yml as an executable test,
// so the layering still holds in a plain `go test ./...` without the linter.
type boundary struct {
	// pkg is the repository-relative directory tree the rule covers.
	pkg string
	// tests reports whether the rule also covers _test.go files, matching the
	// presence or absence of "!$test" in the depguard file selector.
	tests bool
	// allow, when non-empty, is an exhaustive list of permitted non-stdlib
	// imports. Otherwise deny lists forbidden import prefixes.
	allow []string
	deny  []string
	// clientImports applies the namespace policy shared by generic dispatchers
	// and client peers, without enumerating supported client identities.
	clientImports bool
}

func layerBoundaries() []boundary {
	// A library is handed a registry and never assembles the default one:
	// importing clients/all would link every adapter into any binary that
	// merely detects, plans or reads a project. Only a main may name the set.
	injectedRegistry := []string{modulePath + "/install/integrationctl/agentplugins/clients/all"}
	return []boundary{
		{pkg: agentplugins + "/domain", allow: nil},
		{pkg: agentplugins + "/ports", tests: true, allow: []string{
			modulePath + "/install/integrationctl/agentplugins/domain",
			// ports/contracttest verifies the port interfaces, so it names them.
			modulePath + "/install/integrationctl/agentplugins/ports",
			legacyPortsPath,
		}},
		// The command format is shared by the loader, the authoring reader,
		// the stager and the security scan; it stays pure so all of them can.
		{pkg: agentplugins + "/commandext", allow: []string{
			modulePath + "/install/integrationctl/agentplugins/domain",
			modulePath + "/install/integrationctl/agentplugins/conformance",
			"gopkg.in/yaml.v3",
		}},
		{pkg: agentplugins + "/usecase", deny: []string{
			modulePath + "/install/integrationctl/adapters",
			modulePath + "/install/integrationctl/agentplugins/adapters",
			modulePath + "/install/integrationctl/agentplugins/planner",
			modulePath + "/install/integrationctl/agentplugins/providers",
			modulePath + "/install/integrationctl/agentplugins/clients",
		}},
		// The client contract and its adapters are depended upon and never
		// depend back. Tests are covered too: a test that reaches for the
		// planner makes the package impossible to build standalone just as
		// surely as production code would.
		{pkg: agentplugins + "/clients", tests: true, deny: []string{
			modulePath + "/install/integrationctl/agentplugins/providers",
			modulePath + "/install/integrationctl/agentplugins/planner",
			modulePath + "/install/integrationctl/agentplugins/usecase",
			modulePath + "/install/integrationctl/agentplugins/adapters/clientdetect",
		}},
		{pkg: agentplugins + "/clients", tests: true, clientImports: true},
		{pkg: agentplugins + "/providers", clientImports: true},
		{pkg: agentplugins + "/planner", clientImports: true},
		{pkg: agentplugins + "/adapters/clientdetect", clientImports: true},
		{pkg: agentplugins + "/installer", deny: injectedRegistry},
		// The CLI libraries are consumers of the same contract. The authoring
		// readiness package reached for the default registry here and pulled
		// every adapter into the authoring binary, which is the cost this rule
		// exists to keep visible in the mains that choose to pay it.
		{pkg: "cli/internal", deny: injectedRegistry},
		{pkg: "cli/internal/agentpluginscli", deny: []string{
			modulePath + "/install/integrationctl/agentplugins/providers",
			modulePath + "/install/integrationctl/adapters/pathpolicy",
		}},
	}
}

func TestLayerBoundaries(t *testing.T) {
	t.Parallel()
	root := testRepoRoot(t)
	for _, rule := range layerBoundaries() {
		name := rule.pkg
		if rule.clientImports {
			name += "/client-imports"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, violation := range boundaryViolations(t, root, rule) {
				t.Error(violation)
			}
		})
	}
}

func boundaryViolations(t *testing.T, root string, rule boundary) []string {
	t.Helper()
	var violations []string
	for _, file := range goFiles(t, filepath.Join(root, filepath.FromSlash(rule.pkg)), rule.tests) {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range fileImports(t, file) {
			if importForbidden(rule, filepath.ToSlash(rel), imported) {
				violations = append(violations, fmt.Sprintf("%s imports %s, which the layer forbids", filepath.ToSlash(rel), imported))
			}
		}
	}
	return violations
}

func importForbidden(rule boundary, file, imported string) bool {
	if standardLibrary(imported) {
		return false
	}
	if rule.clientImports {
		return clientImportForbidden(file, imported)
	}
	if len(rule.deny) > 0 {
		for _, denied := range rule.deny {
			if imported == denied || strings.HasPrefix(imported, denied+"/") {
				return true
			}
		}
		return false
	}
	for _, allowed := range rule.allow {
		if imported == allowed || strings.HasPrefix(imported, allowed+"/") {
			return false
		}
	}
	return true
}

// clientImportForbidden classifies the namespace rather than the domain registry:
// a new or not-yet-registered adapter must not escape the boundary. Only shared
// code and contract-test helpers are structural subpackages. Unknown names,
// nested implementations and clients/all are otherwise private to composition.
func clientImportForbidden(file, imported string) bool {
	const clientRoot = modulePath + "/" + agentplugins + "/clients/"
	rel, clientPackage := strings.CutPrefix(imported, clientRoot)
	if !clientPackage {
		return false
	}
	leaf, _, _ := strings.Cut(rel, "/")
	if leaf == "shared" || leaf == "contracttest" {
		return false
	}
	const clientDir = agentplugins + "/clients/"
	if strings.HasPrefix(file, clientDir+"all/") {
		return false
	}
	// Preserve the existing external Gemini consumer tests' own-adapter
	// exception. A peer or nested concrete import remains forbidden there.
	if path.Dir(file) == clientDir+"gemini" && strings.HasSuffix(file, "_test.go") && imported == clientRoot+"gemini" {
		return false
	}
	return true
}

// standardLibrary uses the same rule the go tool does: only standard library
// paths have no dot in their first segment.
func standardLibrary(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

func goFiles(t *testing.T, dir string, includeTests bool) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		if !includeTests && strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		files = append(files, current)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no Go files under %s", dir)
	}
	return files
}

func fileImports(t *testing.T, filename string) []string {
	t.Helper()
	contents, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), filename, contents, parser.ImportsOnly|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		imports = append(imports, value)
	}
	return imports
}
