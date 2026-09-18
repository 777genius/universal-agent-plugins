package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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
}

func TestLayerBoundaries(t *testing.T) {
	t.Parallel()
	root := testRepoRoot(t)
	boundaries := []boundary{
		{pkg: agentplugins + "/domain", allow: nil},
		{pkg: agentplugins + "/ports", tests: true, allow: []string{
			modulePath + "/install/integrationctl/agentplugins/domain",
			// ports/contracttest verifies the port interfaces, so it names them.
			modulePath + "/install/integrationctl/agentplugins/ports",
			legacyPortsPath,
		}},
		{pkg: agentplugins + "/usecase", deny: []string{
			modulePath + "/install/integrationctl/adapters",
			modulePath + "/install/integrationctl/agentplugins/adapters",
			modulePath + "/install/integrationctl/agentplugins/planner",
			modulePath + "/install/integrationctl/agentplugins/providers",
			modulePath + "/install/integrationctl/agentplugins/clients",
		}},
	}
	for _, rule := range boundaries {
		t.Run(rule.pkg, func(t *testing.T) {
			t.Parallel()
			for _, file := range goFiles(t, filepath.Join(root, filepath.FromSlash(rule.pkg)), rule.tests) {
				for _, imported := range fileImports(t, file) {
					checkImport(t, rule, strings.TrimPrefix(file, root+string(filepath.Separator)), imported)
				}
			}
		})
	}
}

func checkImport(t *testing.T, rule boundary, file, imported string) {
	t.Helper()
	if standardLibrary(imported) {
		return
	}
	if len(rule.deny) > 0 {
		for _, denied := range rule.deny {
			if imported == denied || strings.HasPrefix(imported, denied+"/") {
				t.Errorf("%s imports %s, which the layer forbids", file, imported)
			}
		}
		return
	}
	for _, allowed := range rule.allow {
		if imported == allowed || strings.HasPrefix(imported, allowed+"/") {
			return
		}
	}
	t.Errorf("%s imports %s, which the layer does not allow", file, imported)
}

// standardLibrary uses the same rule the go tool does: only standard library
// paths have no dot in their first segment.
func standardLibrary(path string) bool {
	first, _, _ := strings.Cut(path, "/")
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

func fileImports(t *testing.T, path string) []string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, contents, parser.ImportsOnly|parser.SkipObjectResolution)
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
