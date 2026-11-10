package exampleclient

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
)

func TestExampleClientInjectsWithoutGenericImports(t *testing.T) {
	t.Parallel()
	registry, err := clients.NewRegistry(New())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Lookup(New().ID()); !ok {
		t.Fatal("example adapter did not register")
	}
	contracttest.RunAdapter(t, New())
	contracttest.RunHostDetector(t, New())
	assertDoesNotImport(t, "planner/planner.go")
	assertDoesNotImport(t, "providers/activator.go")
	assertPackageDoesNotImport(t, "usecase")
	assertDoesNotImport(t, "adapters/clientdetect/detector.go")
}

const exampleImport = "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/internal/exampleclient"

func assertDoesNotImport(t *testing.T, rel string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	path := filepath.Join(root, filepath.FromSlash(rel))
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		if strings.Trim(spec.Path.Value, `"`) == exampleImport {
			t.Errorf("%s imports %s; generic packages must not name the example adapter", rel, exampleImport)
		}
	}
}

// Check every production compilation unit, not one historical service.go path.
// A new usecase file must not silently import the example test client.
func assertPackageDoesNotImport(t *testing.T, rel string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		assertDoesNotImport(t, filepath.ToSlash(filepath.Join(rel, entry.Name())))
	}
}
