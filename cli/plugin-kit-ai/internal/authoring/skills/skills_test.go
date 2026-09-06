package skills_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const secret = "fixture-canary"

func put(t *testing.T, root, path, body string) {
	t.Helper()
	dest := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func setup(t *testing.T) (skills.Service, string) {
	t.Helper()
	root := t.TempDir()
	put(t, root, "plugin.json", `{"$schema":"`+domain.PluginSchemaV1+`","name":"fixture"}`)
	return skills.Service{Projects: project.Service{Scratch: t.TempDir()}, Revision: "skills-test"}, root
}
func skill(name, desc string) string {
	return "---\nname: " + name + "\ndescription: " + desc + "\n---\n"
}
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(root, p)
		info, e := d.Info()
		if e != nil {
			return e
		}
		value := info.Mode().String()
		if d.Type()&os.ModeSymlink != 0 {
			target, e := os.Readlink(p)
			if e != nil {
				return e
			}
			value += target
		} else if !d.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			value += string(b)
		}
		out[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func clean(t *testing.T, s skills.Service, root string) {
	t.Helper()
	entries, err := os.ReadDir(s.Projects.Scratch)
	if err != nil || len(entries) != 0 {
		t.Fatalf("scratch: %v %v", entries, err)
	}
	for path := range snapshot(t, root) {
		if strings.Contains(path, ".authoring-") {
			t.Fatalf("stage residue: %s", path)
		}
	}
}
func TestGeneratedSkillWholePackage(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent-parent", true: "existing-parent"}[parent], func(t *testing.T) {
			s, root := setup(t)
			put(t, root, "unrelated/data", secret)
			if parent {
				put(t, root, "skills/keep/references/data", secret)
			}
			before := snapshot(t, root)
			r, err := s.Init(context.Background(), skills.Request{Root: root, Name: "café", Description: "Use for \"documentation\" requests.\n" + secret})
			if err != nil || !r.Committed || !r.Successful() {
				t.Fatalf("init: %v %+v", err, r)
			}
			after := snapshot(t, root)
			for p, b := range before {
				if after[p] != b {
					t.Fatalf("source changed %s", p)
				}
			}
			if !reflect.DeepEqual(r.Paths, []string{"skills/café/SKILL.md"}) {
				t.Fatal(r.Paths)
			}
			for _, command := range []string{"validate", "test"} {
				p, err := s.Projects.Read(context.Background(), root)
				if err != nil {
					t.Fatal(err)
				}
				r := report.Build(command, s.Revision, p, false)
				if !r.Successful() || r.Runtime.Status != report.NotEvaluated {
					t.Fatalf("%s: %+v", command, r)
				}
				profiles := conformance.ProfileIdentities()
				if len(profiles) != len(r.Profiles) {
					t.Fatal("profiles missing")
				}
				for i, p := range profiles {
					if r.Profiles[i].ID != p.ID || r.Profiles[i].Revision != p.Revision || r.Profiles[i].Digest != p.Digest {
						t.Fatal("profile drift")
					}
				}
				b, _ := json.Marshal(r)
				if strings.Contains(string(b), secret) || strings.Contains(string(b), root) {
					t.Fatal("private input leaked")
				}
			}
			clean(t, s, root)
		})
	}
}
func TestValidateBoundaries(t *testing.T) {
	cases := []struct {
		name, body, code string
		success          bool
	}{
		{"valid-alias", "---\nname: &name bad\ndescription: *name\n---\n", "", true},
		{"valid-merge", "---\ndefaults: &d {name: bad, description: text}\n<<: *d\n---\n", "", true},
		{"unknown-field", skill("bad", "text")[:len(skill("bad", "text"))-4] + "opaque: [true, 1]\n---\n", "", true},
		{"invalid-yaml", "---\nname: [\n---\n", "skill_yaml_invalid", false},
		{"duplicate", skill("bad", "text")[:len(skill("bad", "text"))-4] + "name: bad\n---\n", "yaml_duplicate_key", false},
		{"cycle", "---\nname: bad\ndescription: &cycle [*cycle]\n---\n", "yaml_alias_cycle", false},
		{"depth", "---\nname: bad\ndescription: " + strings.Repeat("[", 70) + "text" + strings.Repeat("]", 70) + "\n---\n", "yaml_depth_limit", false},
		{"frontmatter-size", skill("bad", strings.Repeat("x", 66000)), "frontmatter_bytes_limit", false},
		{"document-size", skill("bad", "text") + strings.Repeat("x", (1<<20)+1), "document_bytes_limit", false},
		{"mismatch", skill("other", "text"), "skill_name_mismatch", false},
		{"invalid-utf8", skill("bad", "text") + "\xff", "document_utf8_invalid", false},
		{"tools-metadata", "---\nname: bad\ndescription: text\nallowed-tools: 'shell arbitrary-command'\nmetadata: {note: fixture-canary}\n---\n", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, root := setup(t)
			put(t, root, "skills/bad/SKILL.md", tc.body)
			put(t, root, "skills/good/SKILL.md", skill("good", "A valid sibling"))
			put(t, root, "skills/good/references/nested/SKILL.md", "not another component")
			before := snapshot(t, root)
			r, err := s.Validate(context.Background(), skills.Request{Root: root})
			if (err == nil) != tc.success || r.Successful() != tc.success {
				t.Fatalf("success=%v: %v %+v", tc.success, err, r)
			}
			passed := 0
			for _, c := range r.Components {
				if c.Type == "skill" && c.Status == report.Pass {
					passed++
				}
			}
			if tc.name == "document-size" {
				// The shared acquisition ceiling invalidates the lease, so no sibling
				// bytes remain authoritative. Preserve explicit unavailable coverage.
				var readErr *packageview.Error
				if !errors.As(err, &readErr) || readErr.Code != "byte_limit" || r.Coverage.FactsComplete || r.Readiness.Status == report.Pass {
					t.Fatalf("oversize acquisition: %v %+v", err, r)
				}
				clean(t, s, root)
				return
			}
			if passed < 1 || passed > 2 {
				t.Fatalf("sibling discovery lost: %+v", r.Components)
			}
			if tc.code != "" {
				found := false
				for _, f := range r.Findings {
					found = found || f.Code == tc.code
				}
				if !found {
					t.Fatalf("missing %s: %+v", tc.code, r.Findings)
				}
			}
			b, _ := json.Marshal(r)
			if strings.Contains(string(b), secret) || strings.Contains(string(b), "arbitrary-command") {
				t.Fatal("value leak")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("validation mutated input")
			}
			clean(t, s, root)
		})
	}
}
func TestSkillContainmentAndSourceGate(t *testing.T) {
	for _, kind := range []string{"root-link", "parent-link", "script-escape", "reference-escape", "asset-escape", "legacy-only", "legacy-alias", "unsupported-core", "invalid-core", "incomplete"} {
		t.Run(kind, func(t *testing.T) {
			s, root := setup(t)
			outside := t.TempDir()
			put(t, outside, "data", secret)
			selected := root
			switch kind {
			case "root-link":
				selected = filepath.Join(t.TempDir(), "link")
				if e := os.Symlink(root, selected); e != nil {
					t.Fatal(e)
				}
			case "parent-link":
				if e := os.Symlink(outside, filepath.Join(root, "skills")); e != nil {
					t.Fatal(e)
				}
			case "script-escape", "reference-escape", "asset-escape":
				put(t, root, "skills/good/SKILL.md", skill("good", "text"))
				dir := map[string]string{"script-escape": "scripts", "reference-escape": "references", "asset-escape": "assets"}[kind]
				if e := os.Symlink(outside, filepath.Join(root, "skills/good", dir)); e != nil {
					t.Fatal(e)
				}
			case "legacy-only":
				if e := os.Remove(filepath.Join(root, "plugin.json")); e != nil {
					t.Fatal(e)
				}
				put(t, root, "plugin/plugin.yaml", secret)
			case "legacy-alias":
				if e := os.Remove(filepath.Join(root, "plugin.json")); e != nil {
					t.Fatal(e)
				}
				put(t, root, "plugin/plugin.yaml", secret)
				if e := os.Link(filepath.Join(root, "plugin/plugin.yaml"), filepath.Join(root, "plugin.json")); e != nil {
					t.Fatal(e)
				}
			case "unsupported-core":
				put(t, root, "plugin.json", `{"$schema":"https://agent-plugins.org/schemas/99/plugin.schema.json","name":"fixture"}`)
			case "invalid-core":
				put(t, root, "plugin.json", `{"$schema":"`+domain.PluginSchemaV1+`","name":"fixture","unknown":true}`)
			case "incomplete":
				s.Projects.Limits = packageview.Limits{Entries: 1}
				put(t, root, "more/file", "data")
			}
			before, outBefore := snapshot(t, root), snapshot(t, outside)
			r, err := s.Init(context.Background(), skills.Request{Root: selected, Name: "new-skill", Description: "text"})
			if err == nil || r.Committed {
				t.Fatalf("source gate authorized %s: %v %+v", kind, err, r)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(outBefore, snapshot(t, outside)) {
				t.Fatal("source or outside changed")
			}
			clean(t, s, root)
		})
	}
}
func TestInitCollisionCancelAndConcurrent(t *testing.T) {
	for _, kind := range []string{"empty", "nonempty", "case", "cancel", "concurrent-absent", "concurrent-existing"} {
		t.Run(kind, func(t *testing.T) {
			s, root := setup(t)
			req := skills.Request{Root: root, Name: "new-skill", Description: "text"}
			switch kind {
			case "empty":
				if e := os.MkdirAll(filepath.Join(root, "skills/new-skill"), 0700); e != nil {
					t.Fatal(e)
				}
			case "nonempty":
				put(t, root, "skills/new-skill/keep", secret)
			case "case":
				put(t, root, "skills/NEW-SKILL/keep", secret)
			case "concurrent-existing":
				put(t, root, "skills/keep/data", secret)
			}
			before := snapshot(t, root)
			if strings.HasPrefix(kind, "concurrent") {
				start := make(chan struct{})
				var wg sync.WaitGroup
				results := make(chan report.Report, 2)
				for i := 0; i < 2; i++ {
					wg.Add(1)
					go func() { defer wg.Done(); <-start; r, _ := s.Init(context.Background(), req); results <- r }()
				}
				close(start)
				wg.Wait()
				close(results)
				committed := 0
				for r := range results {
					if r.Committed {
						committed++
					}
				}
				if committed != 1 {
					t.Fatalf("expected one commit, got %d", committed)
				}
			} else {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				if kind == "cancel" {
					cancel()
				}
				r, err := s.Init(ctx, req)
				if err == nil || r.Committed {
					t.Fatalf("collision/cancel: %v %+v", err, r)
				}
				if !reflect.DeepEqual(before, snapshot(t, root)) {
					t.Fatal("existing source changed")
				}
			}
			clean(t, s, root)
		})
	}
}
