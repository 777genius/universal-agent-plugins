package domain

import (
    "go/parser"
    "go/token"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "testing"
)

// The domain owns facts and pure policy; adapters, CLI and orchestration must
// depend on it, never the other way around. This remains valid as clients grow.
func TestDomainImportDirection(t *testing.T) {
    entries, err := os.ReadDir(".")
    if err != nil { t.Fatal(err) }
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
            continue
        }
        source, err := parser.ParseFile(token.NewFileSet(), filepath.Clean(entry.Name()), nil, parser.ImportsOnly)
        if err != nil { t.Fatalf("%s: %v", entry.Name(), err) }
        for _, imported := range source.Imports {
            target, err := strconv.Unquote(imported.Path.Value)
            if err != nil { t.Fatal(err) }
            if strings.HasPrefix(target, "github.com/777genius/plugin-kit-ai/") {
                t.Errorf("%s: domain must not import internal implementation %q", entry.Name(), target)
            }
        }
    }
}
