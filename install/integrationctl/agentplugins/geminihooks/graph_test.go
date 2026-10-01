package geminihooks_test

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Red condition: the neutral production planner acquires a CLI/YAML/installer
// dependency, or directly imports filesystem/environment/network APIs.
func TestNeutralProductionDependencyGraph(t *testing.T) {
	goexe, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("Go toolchain required on PATH:", err)
	}
	cmd := testCommand(t, goexe, "list", "-deps", "-json", ".")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal("list actual production graph:", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg struct {
			ImportPath string
			Standard   bool
			GoFiles    []string
			Dir        string
		}
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !pkg.Standard && pkg.ImportPath != "github.com/tailscale/hujson" && !strings.HasSuffix(pkg.ImportPath, "/agentplugins/geminihooks") {
			t.Fatalf("unexpected neutral dependency %s", pkg.ImportPath)
		}
		if !strings.HasSuffix(pkg.ImportPath, "/agentplugins/geminihooks") {
			continue
		}
		for _, name := range pkg.GoFiles {
			source, err := os.ReadFile(filepath.Join(pkg.Dir, name))
			if err != nil {
				t.Fatal(err)
			}
			file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if path == "os" || strings.HasPrefix(path, "os/") || path == "io/fs" || path == "syscall" || strings.HasPrefix(path, "net") || strings.HasPrefix(path, "golang.org/x/sys") {
					t.Fatalf("pure planner imports %s", path)
				}
			}
		}
	}
}
