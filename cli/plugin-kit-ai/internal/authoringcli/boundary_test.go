package authoringcli

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Inspect all platform/build-tag files, not just the host's compiled graph.
// Follow local production imports so a benign helper cannot hide a legacy edge.
func TestStandardAuthoringImportBoundary(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
	const base = "github.com/777genius/plugin-kit-ai"
	modules := map[string]string{base + "/cli": "cli/plugin-kit-ai", base + "/install/integrationctl": "install/integrationctl", base + "/plugininstall": "install/plugininstall", base + "/sdk": "sdk", base: "."}
	resolve := func(path string) string {
		// Longest prefix wins over the root module.
		best := ""
		dir := ""
		for prefix, local := range modules {
			if (path == prefix || strings.HasPrefix(path, prefix+"/")) && len(prefix) > len(best) {
				best = prefix
				dir = filepath.Join(repo, local, strings.TrimPrefix(path, prefix))
			}
		}
		return dir
	}
	forbidden := func(path string) bool {
		for _, part := range []string{"pluginmanifest", "pluginmodel", "app", "publicationmodel", "legacyimport"} {
			if path == base+"/cli/internal/"+part || strings.HasPrefix(path, base+"/cli/internal/"+part+"/") {
				return true
			}
		}
		return strings.HasPrefix(path, base+"/cli/cmd/")
	}
	visited := map[string]bool{}
	var check func(string)
	check = func(dir string) {
		if visited[dir] {
			return
		}
		visited[dir] = true
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range parsed.Imports {
				dep, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if forbidden(dep) {
					t.Errorf("%s imports forbidden %s", path, dep)
					continue
				}
				if local := resolve(dep); local != "" {
					check(local)
				}
			}
		}
	}
	for _, sub := range []string{"authoringcli", "authoring"} {
		root := filepath.Join(repo, "cli/plugin-kit-ai/internal", sub)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return filepath.SkipDir
				}
				check(path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// Canonical facts and installer policy must never depend on authoring.
	root := filepath.Join(repo, "install/integrationctl/agentplugins")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range parsed.Imports {
			dep, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(dep, base+"/cli/internal/authoring") {
				t.Errorf("installer dependency inversion: %s -> %s", path, dep)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
