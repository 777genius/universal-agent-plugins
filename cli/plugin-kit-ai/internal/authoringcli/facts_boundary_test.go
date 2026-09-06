package authoringcli

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const repositoryModule = "github.com/777genius/plugin-kit-ai"

// Canonical facts may use standard-library data types and schema registry ports.
// This is an import boundary, not a proof of purity of individual functions.
func forbiddenFactsImport(dep string) bool {
	for _, prefix := range []string{
		"github.com/spf13/cobra", "github.com/spf13/pflag",
		"os", "syscall", "unsafe", "plugin", "net/http", "net/rpc",
		"golang.org/x/sys", "golang.org/x/net/http2",
		repositoryModule + "/cli", repositoryModule + "/plugininstall",
	} {
		if dep == prefix || strings.HasPrefix(dep, prefix+"/") {
			return true
		}
	}
	if dep == "net" {
		return true
	} // net/url is a legitimate data parser.
	if dep == repositoryModule || strings.HasPrefix(dep, repositoryModule+"/") {
		for _, part := range strings.Split(strings.TrimPrefix(dep, repositoryModule+"/"), "/") {
			switch part {
			case "client", "clients", "planner", "provider", "providers", "state", "mutation", "publication", "publicationmodel", "publish", "transaction", "usecase", "adapters", "cmd", "app", "pluginmanifest", "pluginmodel", "legacyimport":
				return true
			}
		}
	}
	return false
}

// Scan every production Go file regardless of filename platform suffix or build
// constraints. Follow local imports even through helpers outside the fact roots.
func checkFactsBoundary(repo string) error {
	modules := map[string]string{repositoryModule + "/cli": "cli/plugin-kit-ai", repositoryModule + "/install/integrationctl": "install/integrationctl", repositoryModule + "/plugininstall": "install/plugininstall", repositoryModule + "/sdk": "sdk", repositoryModule: "."}
	resolve := func(dep string) string {
		best, dir := "", ""
		for prefix, local := range modules {
			if (dep == prefix || strings.HasPrefix(dep, prefix+"/")) && len(prefix) > len(best) {
				best = prefix
				dir = filepath.Join(repo, local, strings.TrimPrefix(dep, prefix))
			}
		}
		return dir
	}
	seen := map[string]bool{}
	var scan func(string) error
	scan = func(dir string) error {
		if seen[dir] {
			return nil
		}
		seen[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file := filepath.Join(dir, entry.Name())
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range parsed.Imports {
				dep, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if forbiddenFactsImport(dep) {
					return fmt.Errorf("canonical facts boundary: %s imports forbidden %s", file, dep)
				}
				if local := resolve(dep); local != "" {
					if err := scan(local); err != nil {
						return fmt.Errorf("%s -> %s: %w", file, dep, err)
					}
				}
			}
		}
		return nil
	}
	for _, name := range []string{"domain", "conformance"} {
		root := filepath.Join(repo, "install/integrationctl/agentplugins", name)
		if _, err := os.Stat(root); os.IsNotExist(err) && name == "conformance" {
			continue
		} // Extraction is separately owned; scan it as soon as present.
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return scan(path)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func TestFactsBoundaryMutations(t *testing.T) {
	const local = repositoryModule + "/install/integrationctl/agentplugins/"
	for _, root := range []string{"domain", "conformance"} {
		for _, viaHelper := range []bool{false, true} {
			for _, dep := range []string{"github.com/spf13/cobra", "github.com/spf13/pflag", "os/exec", "os", "net/http", local + "planner", local + "providers/cursor", local + "state", local + "transaction", local + "adapters/client", repositoryModule + "/cli/internal/authoringcli", repositoryModule + "/cli/internal/publicationmodel", repositoryModule + "/sdk/publication", repositoryModule + "/sdk/clients", repositoryModule + "/sdk/mutation"} {
				t.Run(fmt.Sprintf("%s/helper=%v/%s", root, viaHelper, dep), func(t *testing.T) {
					repo := t.TempDir()
					write := func(path, body string) {
						t.Helper()
						path = filepath.Join(repo, path)
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(body), 0600); err != nil {
							t.Fatal(err)
						}
					}
					prefix := "install/integrationctl/agentplugins/"
					write(prefix+"domain/data.go", "package domain\nimport (\"encoding/json\";\"time\";\"io\";\"net/url\")\n")
					write(prefix+"ports/registry.go", "package ports\nimport _ "+strconv.Quote(local+"domain")+"\n")
					write(prefix+"conformance/data.go", "package conformance\nimport _ "+strconv.Quote(local+"ports")+"\n")
					if err := checkFactsBoundary(repo); err != nil {
						t.Fatalf("legitimate data/registry ports rejected: %v", err)
					}
					route := dep
					if viaHelper {
						route = repositoryModule + "/sdk/facthelper"
						write("sdk/facthelper/edge_plan9.go", "//go:build plan9 && boundary_canary\n\npackage facthelper\nimport _ "+strconv.Quote(dep)+"\n")
					}
					write(prefix+root+"/edge_plan9.go", "//go:build plan9 && boundary_canary\n\npackage "+root+"\nimport _ "+strconv.Quote(route)+"\n")
					if err := checkFactsBoundary(repo); err == nil || !strings.Contains(err.Error(), "imports forbidden "+dep) {
						t.Fatalf("mutation escaped: %v", err)
					}
				})
			}
		}
	}
}
