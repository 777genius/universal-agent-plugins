package conformance

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestConformanceHasNoEffectDependencies(t *testing.T) {
	entries, e := os.ReadDir(".")
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, e := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if e != nil {
			t.Fatal(e)
		}
		for _, spec := range file.Imports {
			p, e := strconv.Unquote(spec.Path.Value)
			if e != nil {
				t.Fatal(e)
			}
			for _, forbidden := range []string{"os", "os/exec", "net/http", "net", "github.com/spf13/cobra"} {
				if p == forbidden {
					t.Errorf("%s imports %s", entry.Name(), p)
				}
			}
			if strings.Contains(p, "plugin-kit-ai/") && !strings.HasSuffix(p, "/agentplugins/domain") {
				t.Errorf("%s imports an effect/service dependency %s", entry.Name(), p)
			}
		}
	}
}
