package installer

import (
    "go/parser"
    "go/token"
    "os"
    "strconv"
    "strings"
    "testing"
)

// The installer is an application composition root; CLI and UI concerns must
// consume it through public contracts, never become its dependencies.
func TestInstallerDependencyDirection(t *testing.T) {
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
            if strings.Contains(path, "/cli/") || strings.Contains(path, "/landing/") || strings.Contains(path, "/website/") {
                t.Errorf("%s: installer must not depend on a presentation layer: %s", entry.Name(), path)
            }
        }
    }
}
