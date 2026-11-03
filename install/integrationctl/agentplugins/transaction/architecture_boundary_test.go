package transaction

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Transactions may depend on domain facts and implementation ports, but never
// on callers that orchestrate or present a mutation.
func TestTransactionDependencyDirection(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(path, "/agentplugins/usecase") ||
				strings.Contains(path, "/agentplugins/installer") ||
				strings.Contains(path, "/cli/") {
				t.Errorf("%s: transaction must not import an outward orchestration layer: %s", entry.Name(), path)
			}
		}
	}
}
