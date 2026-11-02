package environmentapp_test

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Keep the application reusable by CLI and local web transports. Its concrete
// filesystem/process/client dependencies belong in the calling adapters.
func TestApplicationImportsOnlyEnvironmentAndInstallerBoundaries(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	const module = "github.com/777genius/plugin-kit-ai/"
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if path == module+"install/integrationctl/agentplugins/environment" || path == module+"install/integrationctl/agentplugins/installer" {
				continue
			}
			if strings.Contains(strings.Split(path, "/")[0], ".") {
				t.Errorf("%s imports concrete/external dependency %s", entry.Name(), path)
			}
			if path == "os" || path == "os/exec" || path == "syscall" || path == "net" || strings.HasPrefix(path, "net/") {
				t.Errorf("%s imports transport/effect dependency %s", entry.Name(), path)
			}
		}
	}
}
