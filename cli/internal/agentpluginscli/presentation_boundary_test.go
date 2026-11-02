package agentpluginscli

import (
    "go/parser"
    "go/token"
    "os"
    "strconv"
    "strings"
    "testing"
)

// Presentation files render the command result; they must not acquire source
// state, invoke installer internals or implement persistent transactions.
func TestPresentationDependencyDirection(t *testing.T) {
    entries, err := os.ReadDir(".")
    if err != nil {
        t.Fatal(err)
    }
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_presentation.go") {
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
            if strings.Contains(path, "/agentplugins/installer") ||
                strings.Contains(path, "/agentplugins/transaction") ||
                strings.Contains(path, "/agentplugins/adapters/") {
                t.Errorf("%s: presentation must not depend on mutation infrastructure: %s", entry.Name(), path)
            }
        }
    }
}
