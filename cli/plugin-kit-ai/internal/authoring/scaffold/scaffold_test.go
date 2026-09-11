package scaffold

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func fixtureOptions(lane string) Options {
	o := Options{Name: "audit-plugin", Description: "Help review the requested changes.", Template: Template(lane)}
	switch lane {
	case "mcp-remote":
		o.RemoteURL = "https://mcp.example.test/api"
	case "mcp-stdio":
		o.Runtime = "node"
	case "hybrid-remote":
		o.Template = Hybrid
		o.MCPChoice = MCPRemote
		o.RemoteURL = "https://mcp.example.test/api"
	case "hybrid-stdio":
		o.Template = Hybrid
		o.MCPChoice = MCPStdio
		o.Runtime = "node"
	}
	return o
}

var lanes = []string{"skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"}

func tempRoot(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func planFor(t *testing.T, lane string) Plan {
	t.Helper()
	p, err := BuildPlan(fixtureOptions(lane))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// This is the real current loader, not a success stub or a second parser.
// Future composition replaces this test adapter with complete shared conformance
// plus authoring policy. Current loader evidence is deliberately scoped.
func realValidation(t *testing.T) Validate {
	t.Helper()
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, root string, _ *os.Root) error {
		envelope, err := (loader.Loader{Registry: registry}).Load(ctx, domain.LoadInput{SnapshotRoot: root})
		if err != nil {
			return err
		}
		if envelope.FormatID != domain.FormatIDAgentPluginsV1 || len(envelope.Diagnostics) != 0 {
			return fmt.Errorf("unexpected standard facts/diagnostics: %+v", envelope.Diagnostics)
		}
		return nil
	}
}

type goldenFile struct {
	Path   string
	Mode   uint32
	SHA256 string
}

func treeGolden(files []File) []goldenFile {
	out := []goldenFile{}
	for _, f := range files {
		out = append(out, goldenFile{f.Path, uint32(f.Mode), fmt.Sprintf("%x", sha256.Sum256(f.Bytes))})
	}
	return out
}
func TestTemplateGoldenTreesAndCurrentStandardLoader(t *testing.T) {
	body, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden map[string][]goldenFile
	if err = json.Unmarshal(body, &golden); err != nil {
		t.Fatal(err)
	}
	for _, lane := range lanes {
		t.Run(lane, func(t *testing.T) {
			p := planFor(t, lane)
			again := planFor(t, lane)
			if !reflect.DeepEqual(p.Files(), again.Files()) {
				t.Fatal("nondeterministic planning")
			}
			if !reflect.DeepEqual(treeGolden(p.Files()), golden[lane]) {
				t.Fatalf("golden mismatch: %+v", treeGolden(p.Files()))
			}
			root := tempRoot(t)
			dest := filepath.Join(root, "result")
			calls := 0
			validate := realValidation(t)
			result, err := Apply(context.Background(), p, ApplyOptions{Destination: dest, Validate: func(ctx context.Context, s string, dir *os.Root) error {
				calls++
				if filepath.Dir(filepath.Dir(s)) != root || s == dest {
					t.Fatal("not private sibling staging")
				}
				if runtime.GOOS != "windows" {
					i, e := os.Stat(filepath.Dir(s))
					if e != nil || i.Mode().Perm() != 0700 {
						t.Fatalf("nonprivate stage: %v %v", i, e)
					}
				}
				return validate(ctx, s, dir)
			}})
			if err != nil || !result.Committed || calls != 1 {
				t.Fatalf("apply: %+v %v calls=%d", result, err, calls)
			}
			actual := readTree(t, dest)
			if !reflect.DeepEqual(actual, p.Files()) {
				t.Fatalf("output tree differs: %v", treeGolden(actual))
			}
			assertOnly(t, root, "result")
			if err = validate(context.Background(), dest, nil); err != nil {
				t.Fatal(err)
			}
			// Schema validation is independent evidence of static conformance for both
			// JSON documents; loader may otherwise keep components after diagnostics.
			registry, _ := specregistry.New()
			for _, file := range p.Files() {
				if file.Path == "plugin.json" || file.Path == "mcp.json" {
					var doc map[string]any
					if err = json.Unmarshal(file.Bytes, &doc); err != nil {
						t.Fatal(err)
					}
					uri := domain.PluginSchemaV1
					if file.Path == "mcp.json" {
						uri = domain.MCPSchemaV1
					}
					if err = registry.Validate(uri, doc); err != nil {
						t.Fatal(err)
					}
					if file.Path == "plugin.json" {
						if doc["name"] != "audit-plugin" || doc["version"] != "0.1.0" || doc["author"] != nil || doc["license"] != nil {
							t.Fatal(doc)
						}
					}
				}
				if file.Path == "LICENSE" || strings.HasPrefix(file.Path, "plugin/") || strings.HasPrefix(file.Path, "hooks/") || strings.HasPrefix(file.Path, ".codex-plugin/") {
					t.Fatal(file.Path)
				}
			}
		})
	}
}
func readTree(t *testing.T, root string) []File {
	t.Helper()
	var files []File
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		mode := info.Mode().Perm()
		if runtime.GOOS == "windows" {
			mode = 0644
		}
		files = append(files, File{filepath.ToSlash(rel), b, mode})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}
func assertOnly(t *testing.T, root string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := append([]string{}, names...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("entries %v, want %v", got, want)
	}
}
func TestInvalidOptionsAreExplicit(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Options)
	}{
		{"unknown template", func(o *Options) { o.Template = "python" }},
		{"uppercase identity", func(o *Options) { o.Name = "My Plugin" }},
		{"invalid identity", func(o *Options) { o.Name = "a--b" }},
		{"invalid dots", func(o *Options) { o.Name = "a..b" }},
		{"long identity", func(o *Options) { o.Name = strings.Repeat("a", 65) }},
		{"reserved identity", func(o *Options) { o.Name = "con" }},
		{"empty description", func(o *Options) { o.Description = " \n" }},
		{"long description", func(o *Options) { o.Description = strings.Repeat("界", 1025) }},
		{"bad skill", func(o *Options) { o.SkillName = "a.b" }},
		{"missing hybrid choice", func(o *Options) { o.Template = Hybrid }},
		{"bad hybrid choice", func(o *Options) { o.Template = Hybrid; o.MCPChoice = Skill }},
		{"irrelevant hybrid choice", func(o *Options) { o.MCPChoice = MCPRemote }},
		{"missing runtime", func(o *Options) { o.Template = MCPStdio }},
		{"python rejected", func(o *Options) { o.Template = MCPStdio; o.Runtime = "python" }},
		{"go rejected", func(o *Options) { o.Template = MCPStdio; o.Runtime = "go" }},
		{"node version rejected", func(o *Options) { o.Template = MCPStdio; o.Runtime = "node20" }},
		{"irrelevant runtime", func(o *Options) { o.Runtime = "node" }},
		{"missing URL", func(o *Options) { o.Template = MCPRemote }},
		{"irrelevant URL", func(o *Options) { o.RemoteURL = "https://example.test" }},
		{"irrelevant skill", func(o *Options) { o.Template = MCPRemote; o.RemoteURL = "https://example.test"; o.SkillName = "hello" }},
		{"license without holder", func(o *Options) { o.License = "MIT" }},
		{"unimplemented license", func(o *Options) { o.License = "Apache-2.0" }},
		{"copyright without license", func(o *Options) { o.CopyrightYear = "2026" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := fixtureOptions("skill")
			tc.change(&o)
			p, err := BuildPlan(o)
			if err == nil || len(p.Files()) != 0 {
				t.Fatalf("accepted %+v", o)
			}
		})
	}
	o := fixtureOptions("skill")
	o.Name = "My Plugin"
	_, err := BuildPlan(o)
	var invalid *IdentityError
	if !errors.As(err, &invalid) || invalid.Suggestion != "my-plugin" {
		t.Fatalf("missing explicit suggestion: %v", err)
	}
	o.Name = "con"
	_, err = BuildPlan(o)
	if !errors.As(err, &invalid) || invalid.Suggestion != "my-plugin" {
		t.Fatalf("missing host-safe suggestion: %v", err)
	}
	o.Name = "plugin.with.dots"
	o.SkillName = "different-skill"
	p, err := BuildPlan(o)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(tempRoot(t), "result")
	if _, err = Apply(context.Background(), p, ApplyOptions{Destination: dest, Validate: realValidation(t)}); err != nil {
		t.Fatal(err)
	}
}
func TestRemoteURLs(t *testing.T) {
	for _, raw := range []string{"", "/mcp", "mcp.example.test", "ftp://example.test/mcp", "https://", "https://u:p@example.test", "https://example.test/#fragment", "https://example.test/${TOKEN}", "https://example.test/<endpoint>", "https://example.test:bad", "https://example.test:0", "https://example.test:65536", "https://example.test:", "https://example.test/#", "http://example.test/", "https://example.test/\n"} {
		o := fixtureOptions("mcp-remote")
		o.RemoteURL = raw
		if _, err := BuildPlan(o); err == nil {
			t.Errorf("accepted URL %q", raw)
		}
	}
	for _, raw := range []string{"https://example.test/mcp?version=1", "HTTPS://example.test/mcp", "http://localhost/mcp", "http://127.0.0.1:8080/mcp", "http://[::1]:8080/mcp"} {
		o := fixtureOptions("mcp-remote")
		o.RemoteURL = raw
		if _, err := BuildPlan(o); err != nil {
			t.Errorf("%q: %v", raw, err)
		}
	}
}
func TestExplicitLicensesAndNoInference(t *testing.T) {
	for _, id := range []string{"MIT", "ISC"} {
		o := fixtureOptions("skill")
		o.License = id
		o.CopyrightHolder = "Fixture Authors"
		o.CopyrightYear = "2026"
		o.AuthorName = "Explicit Author"
		p, err := BuildPlan(o)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range p.Files() {
			if f.Path == "LICENSE" {
				found = true
				if !bytes.Contains(f.Bytes, []byte("Copyright (c) 2026 Fixture Authors")) || bytes.Contains(f.Bytes, []byte("<")) {
					t.Fatal(string(f.Bytes))
				}
			}
			if f.Path == "plugin.json" {
				var m map[string]any
				json.Unmarshal(f.Bytes, &m)
				if m["license"] != id || m["author"].(map[string]any)["name"] != "Explicit Author" {
					t.Fatal(m)
				}
			}
		}
		if !found {
			t.Fatal("missing license")
		}
	}
	// Planning depends only on explicit values even when inference-like values exist.
	t.Setenv("GIT_AUTHOR_NAME", "Must Not Infer")
	t.Setenv("USER", "Must Not Infer")
	if p := planFor(t, "skill"); bytes.Contains(jsonBytes(p.Files()), []byte("Must Not Infer")) {
		t.Fatal("inferred author")
	}
}
func TestNodeFixtureIntegrityAndStaticServer(t *testing.T) {
	// Original generated fixture hashes pin every byte, including every transitive
	// dependency and integrity value. No npm, Node, or SDK code is invoked here.
	for name, b := range map[string][]byte{"package": nodePackage, "lock": nodeLock} {
		want := map[string]string{"package": "dcf6025f7356e0f09aa50db14395f5efd33ff5934ab2404601eaba7b5f36e053", "lock": "14b7f6537202997b0572ab7e368a3c969ff01b558f7e8b25917c839361cb0f0f"}[name]
		if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != want {
			t.Fatalf("%s fixture changed: %s", name, got)
		}
	}
	for _, name := range []string{"audit-plugin", "other.name"} {
		o := fixtureOptions("mcp-stdio")
		o.Name = name
		p, err := BuildPlan(o)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range p.Files() {
			switch f.Path {
			case "package-lock.json":
				restored := bytes.ReplaceAll(f.Bytes, []byte(`"name": "`+name+`"`), []byte(`"name": "agent-plugin-template"`))
				if !bytes.Equal(restored, nodeLock) {
					t.Fatal("non-root lock bytes changed")
				}
				var doc map[string]any
				json.Unmarshal(f.Bytes, &doc)
				packages := doc["packages"].(map[string]any)
				if doc["name"] != name || packages[""].(map[string]any)["name"] != name || packages["node_modules/@modelcontextprotocol/sdk"].(map[string]any)["version"] != "1.30.0" {
					t.Fatal("lock identity/version mismatch")
				}
			case "package.json":
				var doc map[string]any
				json.Unmarshal(f.Bytes, &doc)
				if doc["name"] != name || doc["scripts"] != nil || doc["engines"].(map[string]any)["node"] != ">=22" || doc["dependencies"].(map[string]any)["@modelcontextprotocol/sdk"] != "1.30.0" {
					t.Fatal(doc)
				}
			case "src/server.mjs":
				for _, token := range []string{"@modelcontextprotocol/sdk/server/mcp.js", "@modelcontextprotocol/sdk/server/stdio.js", "registerTool('hello'", "server.connect(new StdioServerTransport())"} {
					if !bytes.Contains(f.Bytes, []byte(token)) {
						t.Fatal("missing official server structure")
					}
				}
			case "mcp.json":
				var doc struct {
					Servers map[string]struct {
						Command string
						Args    []string
					} `json:"mcpServers"`
				}
				json.Unmarshal(f.Bytes, &doc)
				if doc.Servers[name].Command != "node" || !reflect.DeepEqual(doc.Servers[name].Args, []string{"${PLUGIN_ROOT}/src/server.mjs"}) {
					t.Fatal(doc)
				}
			}
		}
	}
}
func TestPlanReviewIsImmutableAndPathPolicy(t *testing.T) {
	p := planFor(t, "skill")
	before := p.Files()
	review := p.Files()
	review[0].Bytes[0] = '!'
	review[0].Path = "../oops"
	if !reflect.DeepEqual(before, p.Files()) {
		t.Fatal("review mutated plan")
	}
	base := []File{{"plugin.json", []byte("{}"), 0644}}
	for _, paths := range [][]string{{"../escape"}, {"/absolute"}, {"a\\b"}, {"CON.txt"}, {"COM1"}, {"a."}, {"a:b"}, {"é"}, {"A/x", "a/y"}, {"a", "a/b"}, {"a/b", "a"}, {"x", "x"}, {"hooks/x"}, {"plugin/plugin.yaml"}, {".codex-plugin/plugin.json"}, {".git/config"}} {
		files := append([]File{}, base...)
		for _, path := range paths {
			files = append(files, File{path, nil, 0644})
		}
		if err := validateFiles(files); err == nil {
			t.Errorf("accepted %v", paths)
		}
	}
	if err := validateFiles([]File{{"plugin.json", nil, 0777}}); err == nil {
		t.Fatal("unsafe mode")
	}
	if err := validateFiles([]File{{"plugin.json", make([]byte, maxBytes+1), 0644}}); err == nil {
		t.Fatal("unbounded bytes")
	}
}
func TestAllExistingDestinationsPreserved(t *testing.T) {
	for _, kind := range []string{"empty", "nonempty", "file", "symlink", "dangling"} {
		t.Run(kind, func(t *testing.T) {
			root := tempRoot(t)
			dest := filepath.Join(root, "dest")
			switch kind {
			case "empty", "nonempty":
				if err := os.Mkdir(dest, 0755); err != nil {
					t.Fatal(err)
				}
				if kind == "nonempty" {
					os.WriteFile(filepath.Join(dest, "sentinel"), []byte("unchanged"), 0600)
				}
			case "file":
				os.WriteFile(dest, []byte("unchanged"), 0600)
			case "symlink", "dangling":
				target := filepath.Join(root, "target")
				if kind == "symlink" {
					os.Mkdir(target, 0755)
				}
				if err := os.Symlink(target, dest); err != nil {
					t.Skipf("native symlink prerequisite: %v", err)
				}
			}
			before, err := os.Lstat(dest)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			result, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: func(context.Context, string, *os.Root) error { called = true; return errors.New("must not validate") }})
			after, e := os.Lstat(dest)
			if err == nil || e != nil || result.Committed || called || !os.SameFile(before, after) {
				t.Fatalf("existing destination changed: %v %v", result, err)
			}
			if kind == "nonempty" {
				b, _ := os.ReadFile(filepath.Join(dest, "sentinel"))
				if string(b) != "unchanged" {
					t.Fatal("sentinel changed")
				}
			}
			if kind == "file" {
				b, _ := os.ReadFile(dest)
				if string(b) != "unchanged" {
					t.Fatal("file changed")
				}
			}
		})
	}
}
func TestLateRaceUsesKernelNoReplace(t *testing.T) {
	for _, kind := range []string{"empty", "nonempty", "file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root := tempRoot(t)
			dest := filepath.Join(root, "dest")
			var before fs.FileInfo
			ops := applyOps{write: writeTree, rename: func(from *os.File, old string, to *os.File, new string) error {
				// Runs after Apply's final absence check: only the kernel can preserve this.
				switch kind {
				case "empty", "nonempty":
					if err := os.Mkdir(dest, 0755); err != nil {
						return err
					}
					if kind == "nonempty" {
						if err := os.WriteFile(filepath.Join(dest, "sentinel"), []byte("race winner"), 0644); err != nil {
							return err
						}
					}
				case "file":
					if err := os.WriteFile(dest, []byte("race winner"), 0644); err != nil {
						return err
					}
				case "symlink":
					if err := os.Symlink(filepath.Join(root, "absent-target"), dest); err != nil {
						t.Skipf("native symlink prerequisite: %v", err)
					}
				}
				before, _ = os.Lstat(dest)
				return renameExclusive(from, old, to, new)
			}}
			result, err := apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: dest, Validate: realValidation(t)}, ops)
			after, e := os.Lstat(dest)
			if err == nil || result.Committed || e != nil || !os.SameFile(before, after) {
				t.Fatalf("late race replaced: %+v %v", result, err)
			}
			assertOnly(t, root, "dest")
			if kind == "nonempty" {
				b, _ := os.ReadFile(filepath.Join(dest, "sentinel"))
				if string(b) != "race winner" {
					t.Fatal("race winner changed")
				}
			}
		})
	}
}
func TestConcurrentApplyExactlyOneWinner(t *testing.T) {
	root := tempRoot(t)
	dest := filepath.Join(root, "result")
	p := planFor(t, "skill")
	validate := realValidation(t)
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	type outcome struct {
		result Result
		err    error
	}
	results := make(chan outcome, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			arrived := false
			defer func() {
				if !arrived {
					ready <- struct{}{}
				}
			}()
			r, err := Apply(context.Background(), p, ApplyOptions{Destination: dest, Validate: func(ctx context.Context, s string, dir *os.Root) error {
				if err := validate(ctx, s, dir); err != nil {
					return err
				}
				arrived = true
				ready <- struct{}{}
				<-release
				return nil
			}})
			results <- outcome{r, err}
		}()
	}
	<-ready
	<-ready
	close(release)
	wg.Wait()
	close(results)
	winners := 0
	for got := range results {
		t.Logf("concurrent Apply: result=%+v raw_error=%T %v", got.result, got.err, got.err)
		if got.err == nil && got.result.Committed {
			winners++
		} else if got.result.Committed || !errors.Is(got.err, os.ErrExist) {
			t.Errorf("loser must report destination existence: result=%+v raw_error=%T %v", got.result, got.err, got.err)
		}
	}
	if winners != 1 {
		t.Fatalf("%d winners", winners)
	}
	assertOnly(t, root, "result")
}
