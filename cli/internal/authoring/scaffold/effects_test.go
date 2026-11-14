package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Check all platform source files, not just the current host's build. The
// foundation's transitive repository import gate is also run in verification.
func TestPrivateScaffoldEffectBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(".", entry.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if name == "os/exec" || name == "net/http" || strings.Contains(name, "/providers") || strings.Contains(name, "/clientdetect") || strings.Contains(name, "/securityscan") || strings.Contains(name, "/sourceacquisition") || strings.Contains(name, "/internal/app") || strings.Contains(name, "/pluginmanifest") || strings.Contains(name, "/pluginmodel") {
				t.Errorf("%s imports effect/legacy capability %s", entry.Name(), name)
			}
			if strings.HasPrefix(name, "github.com/777genius/") && name != "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain" && name != "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry" {
				t.Errorf("unexpected repository capability %s", name)
			}
		}
	}
}
