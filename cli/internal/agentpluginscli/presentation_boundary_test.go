package agentpluginscli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Files named *_presentation.go render results. They can consume usecase
// result types but must not import mutation infrastructure or construct a
// usecase service; helpers outside this naming convention are not covered.
func TestPresentationDependencyDirection(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_presentation.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		usecaseAlias := ""
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(path, "/agentplugins/installer") ||
				strings.Contains(path, "/agentplugins/transaction") ||
				strings.Contains(path, "/agentplugins/adapters/") {
				t.Errorf("%s: presentation must not depend on mutation infrastructure: %s", entry.Name(), path)
			}
			if strings.HasSuffix(path, "/agentplugins/usecase") {
				usecaseAlias = "usecase"
				if spec.Name != nil {
					usecaseAlias = spec.Name.Name
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && id.Name == usecaseAlias && usecaseAlias != "" && n.Sel.Name == "Service" {
					t.Errorf("%s: presentation must not construct or depend on usecase.Service", entry.Name())
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "lifecycleService" {
					t.Errorf("%s: presentation must not construct lifecycleService", entry.Name())
				}
			}
			return true
		})
	}
}
