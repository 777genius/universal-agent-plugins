package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Docs Fixture", "GIT_AUTHOR_EMAIL=docs@example.invalid", "GIT_COMMITTER_NAME=Docs Fixture", "GIT_COMMITTER_EMAIL=docs@example.invalid")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v: %s", args, e, b)
	}
	return strings.TrimSpace(string(b))
}
func checkoutSHA(t *testing.T, dir string) string {
	t.Helper()
	return fixtureGit(t, dir, "rev-parse", "HEAD")
}
func writeFixture(t *testing.T, dir, name string, body []byte) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}
func commitFixture(t *testing.T, dir string) string {
	t.Helper()
	fixtureGit(t, dir, "add", ".")
	fixtureGit(t, dir, "-c", "commit.gpgsign=false", "commit", "-qm", "docs fixture")
	return checkoutSHA(t, dir)
}

// Mutations use disposable Git metadata only. This minimal source-contract
// fixture is supplemental; DOCS_TEST_CHECKOUT makes the disk tests use the full
// clean integrated checkout on which the focused suite is being built.
func committedFixture(t *testing.T) string {
	t.Helper()
	if dir := os.Getenv("DOCS_TEST_CHECKOUT"); dir != "" {
		return dir
	}
	return newSourceFixture(t)
}
func newSourceFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	paths := []string{"docs/PHASE6_CLI_EXPORTER_PREPARATION.md"}
	for _, pin := range factoryPins {
		paths = append(paths, pin.Path)
	}
	for _, name := range adapterFiles {
		paths = append(paths, "cli/plugin-kit-ai/tools/authoring-docs/"+name)
	}
	for _, path := range paths {
		b, e := os.ReadFile(filepath.Join(root, path))
		if e != nil {
			t.Fatal(e)
		}
		writeFixture(t, dir, path, b)
	}
	fixtureGit(t, dir, "init", "-q")
	commitFixture(t, dir)
	return dir
}
func rejectedBeforeOutput(t *testing.T, dir, sha, want string, load func() ([]*cobra.Command, error)) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "output")
	err := exportTrees(dir, sha, out, load)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want %q, got %v", want, err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("rejected export created output")
	}
}
func TestCommittedSourceAndDocsOnlyCommit(t *testing.T) {
	dir := newSourceFixture(t)
	first := checkoutSHA(t, dir)
	a := filepath.Join(t.TempDir(), "a")
	if err := export(dir, first, a); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, dir, "docs/later.md", []byte("docs only\n"))
	second := commitFixture(t, dir)
	b := filepath.Join(t.TempDir(), "b")
	if err := export(dir, second, b); err != nil {
		t.Fatal(err)
	}
	for _, base := range []struct{ path, sha string }{{a, first}, {b, second}} {
		body, err := os.ReadFile(filepath.Join(base.path, namespace, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m manifest
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatal(err)
		}
		if m.SourceSHA != base.sha || m.FactoryBaseline != factoryBaselineSHA || m.Released || m.Status != "prepared-not-release" {
			t.Fatal("provenance/status")
		}
		count := 0
		for _, s := range m.Surfaces {
			for _, e := range s.Commands {
				count++
				if _, err := os.Stat(filepath.Join(base.path, e.FileName)); err != nil {
					t.Fatal(err)
				}
			}
		}
		if count != 34 {
			t.Fatal(count)
		}
	}
	err := filepath.WalkDir(a, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(a, path)
		x, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		y, err := os.ReadFile(filepath.Join(b, rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(bytes.ReplaceAll(x, []byte(first), []byte("SHA")), bytes.ReplaceAll(y, []byte(second), []byte("SHA"))) {
			t.Fatalf("non-provenance difference: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestSourceDriftRejections(t *testing.T) {
	cases := []struct {
		name, path, body, want string
		remove, stage, commit  bool
	}{
		{name: "unstaged", path: "docs/PHASE6_CLI_EXPORTER_PREPARATION.md", body: "dirty", want: "tracked checkout"},
		{name: "staged", path: "docs/PHASE6_CLI_EXPORTER_PREPARATION.md", body: "dirty", want: "tracked checkout", stage: true},
		{name: "factory-root", path: "cli/plugin-kit-ai/internal/agentpluginscli/root.go", body: "package agentpluginscli\n", want: "source input mismatch", commit: true},
		{name: "factory-flags", path: "cli/plugin-kit-ai/internal/authoringcli/flags.go", body: "package authoringcli\n", want: "source input mismatch", commit: true},
		{name: "target-helper", path: "cli/plugin-kit-ai/internal/agentpluginscli/target_batch.go", body: "package agentpluginscli\n", want: "source input mismatch", commit: true},
		{name: "client-registry", path: "install/integrationctl/agentplugins/domain/clients.go", body: "package domain\n", want: "source input mismatch", commit: true},
		{name: "workspace-replace", path: "go.work", body: "go 1.25.0\nuse ./cli/plugin-kit-ai\nreplace github.com/spf13/pflag => ./override\n", want: "source input mismatch", commit: true},
		{name: "workspace-module-dependency", path: "install/integrationctl/go.mod", body: "module github.com/777genius/plugin-kit-ai/install/integrationctl\ngo 1.25.0\nrequire github.com/spf13/pflag v1.0.8\n", want: "source input mismatch", commit: true},
		{name: "missing-factory", path: "cli/plugin-kit-ai/internal/agentpluginscli/target_batch.go", remove: true, want: "source input unavailable", commit: true},
		{name: "new-committed-go", path: "cli/plugin-kit-ai/internal/authoringcli/new.go", body: "package authoringcli\nfunc init() {}\n", want: "unexpected Go input", commit: true},
		{name: "untracked-go", path: "cli/plugin-kit-ai/internal/authoringcli/new.go", body: "package authoringcli\n", want: "unexpected Go input"},
		{name: "ignored-go", path: "cli/plugin-kit-ai/internal/authoringcli/ignored.go", body: "package authoringcli\n", want: "unexpected Go input"},
		{name: "adapter-go", path: "cli/plugin-kit-ai/tools/authoring-docs/new.go", body: "package main\n", want: "unexpected Go input"},
		{name: "ignored-adapter-go", path: "cli/plugin-kit-ai/tools/authoring-docs/ignored.go", body: "package main\n", want: "unexpected Go input"},
		{name: "missing-adapter", path: "cli/plugin-kit-ai/tools/authoring-docs/main.go", remove: true, want: "adapter input unavailable", commit: true},
		{name: "nested-workspace", path: "cli/plugin-kit-ai/go.work", body: "go 1.25.0\nuse .\n", want: "unexpected dependency control"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := newSourceFixture(t)
			if strings.Contains(c.name, "ignored") {
				writeFixture(t, dir, ".git/info/exclude", []byte("ignored.go\n"))
			}
			if c.remove {
				if err := os.Remove(filepath.Join(dir, c.path)); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFixture(t, dir, c.path, []byte(c.body))
			}
			if c.stage {
				fixtureGit(t, dir, "add", c.path)
			}
			if c.commit {
				commitFixture(t, dir)
			}
			rejectedBeforeOutput(t, dir, checkoutSHA(t, dir), c.want, trees)
		})
	}
	t.Run("unrelated-untracked", func(t *testing.T) {
		dir := newSourceFixture(t)
		writeFixture(t, dir, "unrelated/new.go", []byte("unrelated"))
		if _, err := validateSource(dir, checkoutSHA(t, dir)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("empty-checkout", func(t *testing.T) { rejectedBeforeOutput(t, "", factoryBaselineSHA, "--checkout is required", trees) })
}
func TestLoadedProjectionMismatch(t *testing.T) {
	dir := newSourceFixture(t)
	for _, fact := range []string{"help", "flag", "markdown"} {
		t.Run(fact, func(t *testing.T) {
			rejectedBeforeOutput(t, dir, checkoutSHA(t, dir), "loaded documentation projection mismatch", func() ([]*cobra.Command, error) {
				roots, err := trees()
				if err != nil {
					return nil, err
				}
				switch fact {
				case "help":
					roots[0].Short += " changed"
				case "flag":
					roots[1].Parent().PersistentFlags().Lookup("target").Usage += " changed"
				case "markdown":
					roots[0].Run = func(*cobra.Command, []string) { t.Fatal("executed") }
				}
				return roots, nil
			})
		})
	}
}

// Exercise Go's effective selection, not just arbitrary changed control bytes.
// Only module metadata is queried; no product compilation or dependency fetch.
func TestEffectiveDependencyDrift(t *testing.T) {
	for _, control := range []string{"go.work", "install/integrationctl/go.mod"} {
		t.Run(control, func(t *testing.T) {
			dir := newSourceFixture(t)
			replacement := filepath.Join(filepath.Dir(control), "docs-pflag-replacement")
			writeFixture(t, dir, filepath.ToSlash(filepath.Join(replacement, "go.mod")), []byte("module github.com/spf13/pflag\n\ngo 1.22\n"))
			path := filepath.Join(dir, control)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			body = append(body, []byte("\nreplace github.com/spf13/pflag => ./docs-pflag-replacement\n")...)
			writeFixture(t, dir, control, body)
			sha := commitFixture(t, dir)
			cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-m", "-json", "github.com/spf13/pflag")
			cmd.Dir = filepath.Join(dir, "cli/plugin-kit-ai")
			cmd.Env = append(os.Environ(), "GOWORK="+filepath.Join(dir, "go.work"), "GOFLAGS=", "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOMAXPROCS=2")
			selected, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("effective dependency query: %v: %s", err, selected)
			}
			var module struct{ Replace *struct{ Dir string } }
			if err := json.Unmarshal(selected, &module); err != nil {
				t.Fatal(err)
			}
			if module.Replace == nil || module.Replace.Dir != filepath.Join(dir, replacement) {
				t.Fatalf("replacement did not become effective: %s", selected)
			}
			rejectedBeforeOutput(t, dir, sha, "source input mismatch", trees)
		})
	}
}
