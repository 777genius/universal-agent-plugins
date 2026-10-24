package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// auditedAncestorFixtureTrees models the audited ancestor facts for tests of the
// historical source fixture. It is not a rebuilt historical binary and is never
// a production exporter loader. Only the two reviewed live ancestor differences
// are admitted; the unchanged projection golden rejects every other visible fact.
func auditedAncestorFixtureTrees() ([]*cobra.Command, error) {
	roots, err := trees()
	if err != nil {
		return nil, err
	}
	return modelAuditedAncestorFixture(roots)
}

func modelAuditedAncestorFixture(roots []*cobra.Command) ([]*cobra.Command, error) {
	if err := modelAuditedAncestorFacts(roots); err != nil {
		return nil, err
	}
	if err := validateProjection(roots); err != nil {
		return nil, err
	}
	return roots, nil
}

// This step does not render or inspect callbacks, allowing the hook-trap test
// to install every trap before its first render. Other fixture callers use the
// loader above, which also requires the unchanged full projection golden.
func modelAuditedAncestorFacts(roots []*cobra.Command) error {
	var installer *cobra.Command
	for _, root := range roots {
		if root.CommandPath() == "agentplugins author" {
			installer = root.Parent()
		}
	}
	if installer == nil || installer.CommandPath() != "agentplugins" {
		return fmt.Errorf("audited ancestor fixture requires the attached installer ancestor")
	}
	live := map[string]flagFact{}
	for _, fact := range flags(installer.PersistentFlags()) {
		live[fact.Name] = fact
	}
	for _, want := range []flagFact{
		{Name: "scope", Type: "string", Default: "user", Usage: "installation scope: user or project"},
		{Name: "project-root", Type: "string", Default: "", Usage: "existing project directory (project scope only; default: current directory)"},
	} {
		if got := live[want.Name]; got != want {
			return fmt.Errorf("audited ancestor fixture does not admit changed %s facts", want.Name)
		}
	}
	installer.PersistentFlags().Lookup("scope").Usage = "installation scope (user only in this release)"
	installer.PersistentFlags().Lookup("project-root").Hidden = true
	return nil
}

func rendered(t *testing.T) (map[string][]byte, manifest) {
	t.Helper()
	roots, err := trees()
	if err != nil {
		t.Fatal(err)
	}
	if err := modelAuditedAncestorFacts(roots); err != nil {
		t.Fatal(err)
	}
	// Every executable hook is a trap: generation must only inspect definitions.
	var trap func(*cobra.Command)
	trap = func(c *cobra.Command) {
		c.Run = func(*cobra.Command, []string) { t.Fatal("Run called") }
		c.RunE = func(*cobra.Command, []string) error { t.Fatal("RunE called"); return nil }
		c.Args = func(*cobra.Command, []string) error { t.Fatal("Args called"); return nil }
		c.PreRunE = c.RunE
		c.PersistentPreRunE = c.RunE
		c.PostRunE = c.RunE
		c.PersistentPostRunE = c.RunE
		c.ValidArgsFunction = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			t.Fatal("completion called")
			return nil, 0
		}
		for _, ch := range c.Commands() {
			trap(ch)
		}
	}
	for _, root := range roots {
		trap(root)
	}
	files, err := render(factoryBaselineSHA, factoryPins, roots)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(files[namespace+"/manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	return files, m
}

func TestReviewedProjectionMatchesAuditedAncestorFixture(t *testing.T) {
	roots, err := auditedAncestorFixtureTrees()
	if err != nil {
		t.Fatal(err)
	}
	if err := validateProjection(roots); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentProjectAncestorRejectsHistoricalExportBeforeOutput(t *testing.T) {
	checkout := newSourceFixture(t)
	out := filepath.Join(t.TempDir(), "output")
	err := export(checkout, checkoutSHA(t, checkout), out)
	if err == nil || !strings.Contains(err.Error(), "loaded documentation projection mismatch") {
		t.Fatalf("current construction accepted historical source facts: %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("mismatched live construction created output")
	}
}

func TestAuditedAncestorFixtureRejectsAdditionalDrift(t *testing.T) {
	for _, drift := range []struct {
		name   string
		change func([]*cobra.Command)
	}{
		{"scope-usage", func(roots []*cobra.Command) { roots[1].Parent().PersistentFlags().Lookup("scope").Usage += " changed" }},
		{"project-root-default", func(roots []*cobra.Command) {
			roots[1].Parent().PersistentFlags().Lookup("project-root").DefValue = "/other"
		}},
		{"project-root-hidden", func(roots []*cobra.Command) { roots[1].Parent().PersistentFlags().Lookup("project-root").Hidden = true }},
		{"additional-inherited-flag", func(roots []*cobra.Command) {
			roots[1].Parent().PersistentFlags().Bool("unexpected", false, "unreviewed inherited flag")
		}},
		{"other-inherited-help", func(roots []*cobra.Command) { roots[1].Parent().PersistentFlags().Lookup("target").Usage += " changed" }},
		{"author-help", func(roots []*cobra.Command) { roots[1].Short += " changed" }},
		{"plugin-command", func(roots []*cobra.Command) {
			roots[0].AddCommand(&cobra.Command{Use: "unexpected", Short: "unreviewed command"})
		}},
	} {
		t.Run(drift.name, func(t *testing.T) {
			roots, err := trees()
			if err != nil {
				t.Fatal(err)
			}
			drift.change(roots)
			if _, err = modelAuditedAncestorFixture(roots); err == nil {
				t.Fatal("fixture accepted additional construction drift")
			}
		})
	}
}

func TestDeterministicSurfacesAndNoActions(t *testing.T) {
	sandbox := t.TempDir()
	t.Setenv("HOME", sandbox)
	t.Setenv("USERPROFILE", sandbox)
	t.Setenv("TMPDIR", sandbox)
	first, m := rendered(t)
	second, _ := rendered(t)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("fresh exports differ")
	}
	if m.Released || m.Status != "prepared-not-release" || m.SourceSHA != factoryBaselineSHA || m.Namespace != namespace {
		t.Fatal("identity")
	}
	if len(m.Surfaces) != 2 || m.Surfaces[0].CommandPath != "plugin-kit-ai" || m.Surfaces[1].CommandPath != "agentplugins author" {
		t.Fatal("surfaces")
	}
	authorNames := []string{"", "capabilities", "compat", "dev", "doctor", "init", "inspect", "skills", "skills init", "skills validate", "help", "skills help", "test", "validate", "version"}
	for i, s := range m.Surfaces {
		want := map[string]bool{}
		for _, n := range authorNames {
			want[strings.TrimSpace(s.CommandPath+" "+n)] = true
		}
		if i == 0 {
			for _, n := range []string{"completion", "completion bash", "completion fish", "completion help", "completion powershell", "completion zsh"} {
				want[s.CommandPath+" "+n] = true
			}
		} else {
			for _, n := range []string{"import", "import help", "import native", "normalize"} {
				want[s.CommandPath+" "+n] = true
			}
		}
		for _, e := range s.Commands {
			if !want[e.CommandPath] {
				t.Fatalf("unexpected command %s", e.CommandPath)
			}
			delete(want, e.CommandPath)
			body, ok := first[e.FileName]
			if !ok || !bytes.Contains(body, []byte(e.Short)) {
				t.Fatalf("missing Markdown/help %s", e.CommandPath)
			}
			if !strings.HasPrefix(e.Identity, namespace+":") {
				t.Fatal("v1 identity reuse")
			}
		}
		if len(want) > 0 {
			t.Fatalf("missing commands: %v", want)
		}
	}
	for _, body := range first {
		for _, bad := range []string{"Auto generated by", sandbox, "__docs", "__complete", "plugin-kit-ai generate", "skills install"} {
			if bytes.Contains(body, []byte(bad)) {
				t.Fatalf("unwanted output %q", bad)
			}
		}
	}
	entries, err := os.ReadDir(sandbox)
	if err != nil || len(entries) != 0 {
		t.Fatalf("generation mutated sandbox: %v %v", entries, err)
	}
}

func TestFlagFidelity(t *testing.T) {
	files, m := rendered(t)
	for _, s := range m.Surfaces {
		var init entry
		for _, e := range s.Commands {
			if e.CommandPath == s.CommandPath+" init" {
				init = e
			}
		}
		local := map[string]flagFact{}
		inherited := map[string]flagFact{}
		for _, f := range init.LocalFlags {
			local[f.Name] = f
		}
		for _, f := range init.InheritedFlags {
			inherited[f.Name] = f
		}
		if init.Use != "init <path>" || local["mcp-template"].Usage != "hybrid MCP choice: mcp-remote or mcp-stdio" || local["runtime"].Usage != "stdio template runtime: node" {
			t.Fatal("local facts lost")
		}
		for _, bad := range []string{"mcp", "platform", "strict", "force", "output"} {
			if _, ok := local[bad]; ok {
				t.Fatal("retired flag", bad)
			}
		}
		if inherited["format"].Default != "human" || inherited["no-color"].Type != "bool" || inherited["no-color"].NoOptDefault != "true" {
			t.Fatal("inherited facts lost")
		}
		if s.CommandPath == "agentplugins author" {
			if inherited["scope"].Default != "user" || inherited["accept-security-risk"].Type != "bool" || !strings.Contains(init.Long, "are rejected") {
				t.Fatal("installer inheritance lost")
			}
		} else if _, ok := inherited["scope"]; ok {
			t.Fatal("installer flags leaked")
		}
		for _, f := range append(init.LocalFlags, init.InheritedFlags...) {
			if !bytes.Contains(files[init.FileName], []byte("--"+f.Name)) {
				t.Fatalf("Markdown omitted flag %s", f.Name)
			}
		}
	}
}

func TestExclusionPrunesLinksAndDescendants(t *testing.T) {
	root := &cobra.Command{Use: "probe"}
	for _, c := range []*cobra.Command{
		{Use: "hidden", Hidden: true},
		{Use: "shim", Annotations: map[string]string{"authoringcli.v1-rejection": "v1"}},
		{Use: "__internal"},
	} {
		c.AddCommand(&cobra.Command{Use: "visible-child", Run: func(*cobra.Command, []string) { t.Fatal("executed") }})
		root.AddCommand(c)
	}
	root.Flags().String("secret", "", "hidden flag")
	_ = root.Flags().MarkHidden("secret")
	files, err := render(factoryBaselineSHA, nil, []*cobra.Command{root})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range files {
		for _, bad := range []string{"visible-child", "shim", "__internal", "secret"} {
			if bytes.Contains(body, []byte(bad)) {
				t.Fatal("exclusion leaked", bad)
			}
		}
	}
}

func TestSourceIdentityAndNoOverwrite(t *testing.T) {
	checkout := committedFixture(t)
	sha := checkoutSHA(t, checkout)
	if _, err := validateSource(checkout, sha); err != nil {
		t.Fatal(err)
	}
	for _, sha := range []string{"", "070663e", strings.Repeat("0", 40), sha + "\n"} {
		dest := filepath.Join(t.TempDir(), "output")
		if err := export(checkout, sha, dest); err == nil {
			t.Fatal("accepted bad source", sha)
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatal("wrote output for bad source")
		}
	}
	dest := t.TempDir()
	marker := filepath.Join(dest, "v1.md")
	if err := os.WriteFile(marker, []byte("keep v1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := exportTrees(checkout, sha, dest, auditedAncestorFixtureTrees); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing output was not refused at the no-overwrite gate: %v", err)
	}
	body, _ := os.ReadFile(marker)
	if string(body) != "keep v1" {
		t.Fatal("overwrote v1")
	}
	if _, err := validateSource(t.TempDir(), sha); err == nil {
		t.Fatal("accepted non-checkout")
	}
}

func TestExportBytesAndLinks(t *testing.T) {
	checkout := committedFixture(t)
	sha := checkoutSHA(t, checkout)
	base := t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	for _, dest := range []string{a, b} {
		if err := exportTrees(checkout, sha, dest, auditedAncestorFixtureTrees); err != nil {
			t.Fatal(err)
		}
	}
	roots, err := auditedAncestorFixtureTrees()
	if err != nil {
		t.Fatal(err)
	}
	expected, err := render(sha, factoryPins, roots)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(expected[namespace+"/manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Surfaces) != 2 || len(m.Surfaces[0].Commands) != 21 || len(m.Surfaces[1].Commands) != 19 || m.FactoryBaseline != factoryBaselineSHA || m.SourceSHA != sha || m.Released || m.Status != "prepared-not-release" {
		t.Fatal("audited fixture tree inventory/provenance")
	}
	for _, surface := range m.Surfaces {
		for _, entry := range surface.Commands {
			if _, err := os.Stat(filepath.Join(a, entry.FileName)); err != nil {
				t.Fatal(err)
			}
		}
	}
	seen := 0
	err = filepath.WalkDir(a, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(a, path)
		first, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		seen++
		if !bytes.Equal(first, expected[filepath.ToSlash(rel)]) {
			t.Fatalf("disk differs from audited fixture tree: %s", rel)
		}
		second, err := os.ReadFile(filepath.Join(b, rel))
		if err != nil {
			return err
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("nondeterministic file %s", rel)
		}
		if strings.HasSuffix(path, ".md") {
			for _, match := range regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`).FindAllStringSubmatch(string(first), -1) {
				target := match[1]
				if strings.HasPrefix(target, "https://github.com/777genius/universal-agent-plugins/blob/"+sha+"/") {
					continue
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), target)); err != nil {
					t.Fatalf("broken generated link %s -> %s", rel, target)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 41 {
		t.Fatalf("disk file count: %d", seen)
	}
}

func TestCheckoutHEADMismatch(t *testing.T) {
	// Minimal disposable Git metadata; no commit, checkout, or real profile edits.
	checkout := t.TempDir()
	for _, name := range []string{".git/objects", ".git/refs"} {
		if err := os.MkdirAll(filepath.Join(checkout, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(checkout, ".git/HEAD"), []byte(strings.Repeat("1", 40)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := validateSource(checkout, factoryBaselineSHA)
	if err == nil || !strings.Contains(err.Error(), "differs from checkout HEAD") {
		t.Fatalf("wrong mismatch result: %v", err)
	}
}
