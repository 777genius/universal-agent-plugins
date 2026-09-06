package scaffold

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func skillFixture(t *testing.T) (string, SkillPlan, SkillSourceGate) {
	t.Helper()
	root, scratch := t.TempDir(), t.TempDir()
	body := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"fixture"}`)
	if e := os.WriteFile(filepath.Join(root, "plugin.json"), body, 0600); e != nil {
		t.Fatal(e)
	}
	plan, e := BuildSkillPlan(context.Background(), "new-skill", "Use for fixture requests.")
	if e != nil {
		t.Fatal(e)
	}
	gate := func(ctx context.Context, root string) (func(context.Context) error, error) {
		p, e := (project.Service{Scratch: scratch}).Read(ctx, root)
		if e != nil {
			return nil, e
		}
		if !report.Build("validate", "test", p, false).Successful() {
			return nil, errors.New("gate failed")
		}
		core := p.Input.Plugin.Bytes
		return func(ctx context.Context) (err error) {
			l, err := (packageview.Reader{TempDir: scratch}).Open(ctx, root)
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, l.Close()) }()
			if !bytes.Equal(l.Data().Plugin.Bytes, core) {
				return errors.New("source changed")
			}
			return nil
		}, nil
	}
	return root, plan, gate
}
func skillTree(t *testing.T, root string) map[string]string {
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
		v := info.Mode().String()
		if d.Type()&os.ModeSymlink != 0 {
			target, e := os.Readlink(p)
			if e != nil {
				return e
			}
			v += target
		} else if !d.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			v += string(b)
		}
		out[rel] = v
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestSkillPlanProfileAndImmutableBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, desc string
		ok         bool
	}{
		{"café", "Use for café documentation", true}, {"0", "text", true},
		{"bad--name", "text", false}, {"UPPER", "text", false}, {"../outside", "text", false}, {"con", "text", false},
		{"name", "", false}, {"name", strings.Repeat("a", 1025), false}, {"name", "\xff", false},
	} {
		t.Run(tc.name+tc.desc[:min(len(tc.desc), 4)], func(t *testing.T) {
			p, e := BuildSkillPlan(context.Background(), tc.name, tc.desc)
			if e == nil {
				e = sharedSkillValidation(tc.name)(context.Background(), p.File().Bytes)
			}
			if (e == nil) != tc.ok {
				t.Fatalf("expected %t: %v", tc.ok, e)
			}
		})
	}
	root, validPlan, gate := skillFixture(t)
	copyFile := validPlan.File()
	copyFile.Bytes[0] = 'x'
	if bytes.Equal(copyFile.Bytes, validPlan.File().Bytes) {
		t.Fatal("plan file aliases private bytes")
	}
	if r, e := ApplySkill(context.Background(), validPlan, root, gate, nil); e == nil || r.Committed {
		t.Fatal("nil profile validator accepted")
	}
	before := skillTree(t, root)
	if r, e := ApplySkill(context.Background(), SkillPlan{}, root, gate, sharedSkillValidation("new-skill")); e == nil || r.Committed {
		t.Fatal("zero plan accepted")
	}
	if r, e := ApplySkill(context.Background(), SkillPlan{}, root, nil, sharedSkillValidation("new-skill")); e == nil || r.Committed {
		t.Fatal("missing gate accepted")
	}
	if !reflect.DeepEqual(before, skillTree(t, root)) {
		t.Fatal("invalid plan mutated source")
	}
}
func TestSkillAtomicFailureAndStagedValidation(t *testing.T) {
	for _, mode := range []string{"write-failure", "cancel", "invalid-generated", "different-valid-generated", "core-change", "core-link", "collision", "unsupported-rename", "postcommit-cleanup"} {
		t.Run(mode, func(t *testing.T) {
			root, p, gate := skillFixture(t)
			original := skillTree(t, root)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			injected := errors.New("injected I/O failure")
			ops := applyOps{write: func(ctx context.Context, r *os.Root, files []File) error {
				if mode == "write-failure" {
					if e := r.Mkdir("partial", 0700); e != nil {
						return e
					}
					return injected
				}
				if e := writeTree(ctx, r, files); e != nil {
					return e
				}
				switch mode {
				case "cancel":
					cancel()
				case "invalid-generated", "different-valid-generated":
					body := "---\nname: other\ndescription: text\n---\n"
					if mode == "different-valid-generated" {
						body = "---\nname: new-skill\ndescription: different\n---\n"
					}
					if e := r.WriteFile(files[0].Path, []byte(body), 0644); e != nil {
						return e
					}
				case "core-change":
					return os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"changed":true}`), 0600)
				case "core-link":
					if e := os.Mkdir(filepath.Join(root, "plugin"), 0700); e != nil {
						return e
					}
					if e := os.WriteFile(filepath.Join(root, "plugin/plugin.yaml"), []byte("must not be opened"), 0600); e != nil {
						return e
					}
					if e := os.Remove(filepath.Join(root, "plugin.json")); e != nil {
						return e
					}
					return os.Symlink("plugin/plugin.yaml", filepath.Join(root, "plugin.json"))
				case "collision":
					if e := os.Mkdir(filepath.Join(root, "skills"), 0700); e != nil {
						return e
					}
					return os.WriteFile(filepath.Join(root, "skills/winner"), []byte("keep"), 0600)
				}
				return nil
			}, rename: func(from *os.File, old string, to *os.File, new string) error {
				if mode == "unsupported-rename" {
					return injected
				}
				if e := renameExclusive(from, old, to, new); e != nil {
					return e
				}
				if mode == "postcommit-cleanup" {
					return os.WriteFile(filepath.Join(from.Name(), "keep"), []byte("replacement retained"), 0600)
				}
				return nil
			}}
			r, e := applySkill(ctx, p, root, gate, sharedSkillValidation("new-skill"), ops)
			if e == nil {
				t.Fatal("injected failure reported success")
			}
			if r.Committed != (mode == "postcommit-cleanup") {
				t.Fatalf("commit fact wrong: %v %+v", e, r)
			}
			if mode == "postcommit-cleanup" {
				var cleanup *CleanupError
				if !errors.As(e, &cleanup) {
					t.Fatalf("cleanup classification: %v", e)
				}
				if _, e := os.Stat(filepath.Join(root, "skills/new-skill/SKILL.md")); e != nil {
					t.Fatal(e)
				}
			} else {
				after := skillTree(t, root)
				for path := range after {
					if strings.Contains(path, ".authoring-") {
						t.Fatalf("owned stage remains: %s", path)
					}
				}
				switch mode {
				case "core-change", "core-link", "collision": // test's concurrent edits survive
				default:
					if !reflect.DeepEqual(original, after) {
						t.Fatal("source changed on failure")
					}
				}
			}
		})
	}
}
func TestSkillRootAndParentReplacement(t *testing.T) {
	for _, replace := range []string{"source", "skills"} {
		t.Run(replace, func(t *testing.T) {
			base := t.TempDir()
			root, p, gate := skillFixture(t)
			if replace == "skills" {
				if e := os.Mkdir(filepath.Join(root, "skills"), 0700); e != nil {
					t.Fatal(e)
				}
			}
			outside := t.TempDir()
			if e := os.WriteFile(filepath.Join(outside, "keep"), []byte("private"), 0600); e != nil {
				t.Fatal(e)
			}
			before := skillTree(t, outside)
			moved := filepath.Join(base, "moved")
			ops := applyOps{rename: renameExclusive, write: func(ctx context.Context, r *os.Root, files []File) error {
				if e := writeTree(ctx, r, files); e != nil {
					return e
				}
				replaced := root
				if replace == "skills" {
					replaced = filepath.Join(root, "skills")
				}
				if e := os.Rename(replaced, moved); e != nil {
					return e
				}
				return os.Symlink(outside, replaced)
			}}
			result, err := applySkill(context.Background(), p, root, gate, sharedSkillValidation("new-skill"), ops)
			if err == nil || result.Committed {
				t.Fatalf("boundary replacement accepted: %v", err)
			}
			if !reflect.DeepEqual(before, skillTree(t, outside)) {
				t.Fatal("replacement target touched")
			}
			for path := range skillTree(t, moved) {
				if strings.Contains(path, ".authoring-") {
					t.Fatal("owned moved staging was not cleaned")
				}
			}
		})
	}
}

func TestSkillStageReplacementPreservesUnrelatedContent(t *testing.T) {
	root, p, gate := skillFixture(t)
	var replacement, moved string
	ops := applyOps{rename: renameExclusive, write: func(ctx context.Context, r *os.Root, files []File) error {
		if err := writeTree(ctx, r, files); err != nil {
			return err
		}
		replacement = filepath.Dir(r.Name())
		moved = replacement + "-moved"
		if err := os.Rename(replacement, moved); err != nil {
			return err
		}
		if err := os.Mkdir(replacement, 0700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(replacement, "keep"), []byte("unrelated replacement"), 0600)
	}}
	result, err := applySkill(context.Background(), p, root, gate, sharedSkillValidation("new-skill"), ops)
	var cleanup *CleanupError
	if err == nil || result.Committed || !errors.As(err, &cleanup) {
		t.Fatalf("ownership error: %v %+v", err, result)
	}
	b, e := os.ReadFile(filepath.Join(replacement, "keep"))
	if e != nil || string(b) != "unrelated replacement" {
		t.Fatal("replacement was touched")
	}
	entries, e := os.ReadDir(moved)
	if e != nil || len(entries) != 0 {
		t.Fatalf("pinned original payload cleanup: %v %v", entries, e)
	}
}

func sharedSkillValidation(name string) SkillValidation {
	return func(ctx context.Context, body []byte) error {
		f, err := (conformance.Decoder{}).DecodeSkill(ctx, conformance.SkillInput{Directory: name, Document: conformance.NewDocument("skills/"+name+"/SKILL.md", conformance.Present, body)})
		if err != nil {
			return err
		}
		if f.Package == nil || f.Coverage.Skills != conformance.Pass || len(f.Findings) != 0 {
			return errors.New("shared normative validation failed")
		}
		return nil
	}
}

func TestSkillConcurrentUnicodeCaseFoldCollision(t *testing.T) {
	root, _, gate := skillFixture(t)
	if err := os.Mkdir(filepath.Join(root, "skills"), 0700); err != nil {
		t.Fatal(err)
	}
	// Both names are distinct, normative lowercase Unicode, NFC, and EqualFold.
	names := []string{"skill", "ſkill"}
	if !strings.EqualFold(names[0], names[1]) {
		t.Fatal("fixture must collide")
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	first, _ := BuildSkillPlan(context.Background(), names[0], "text")
	second, _ := BuildSkillPlan(context.Background(), names[1], "text")
	wg.Add(1)
	var firstResult Result
	var firstErr error
	go func() {
		defer wg.Done()
		firstResult, firstErr = applySkill(context.Background(), first, root, gate, sharedSkillValidation(names[0]), applyOps{
			rename: renameExclusive, write: func(ctx context.Context, r *os.Root, files []File) error {
				close(entered)
				<-release
				return writeTree(ctx, r, files)
			},
		})
	}()
	<-entered
	secondResult, secondErr := ApplySkill(context.Background(), second, root, gate, sharedSkillValidation(names[1]))
	close(release)
	wg.Wait()
	if !errors.Is(secondErr, os.ErrExist) || secondResult.Committed || firstErr != nil || !firstResult.Committed {
		t.Fatalf("fold collision: %v %v %+v %+v", firstErr, secondErr, firstResult, secondResult)
	}
	for path := range skillTree(t, root) {
		if strings.Contains(path, ".authoring-") {
			t.Fatal("reservation/stage leaked")
		}
	}
}

func TestSkillReservationReplacementIsPreserved(t *testing.T) {
	root, p, gate := skillFixture(t)
	reservation := filepath.Join(root, skillReservation("new-skill"))
	ops := applyOps{rename: renameExclusive, write: func(ctx context.Context, r *os.Root, files []File) error {
		if e := writeTree(ctx, r, files); e != nil {
			return e
		}
		if e := os.Remove(reservation); e != nil {
			return e
		}
		return os.WriteFile(reservation, []byte("replacement"), 0600)
	}}
	result, err := applySkill(context.Background(), p, root, gate, sharedSkillValidation("new-skill"), ops)
	var cleanup *CleanupError
	if result.Committed || !errors.As(err, &cleanup) {
		t.Fatalf("reservation replacement: %v %+v", err, result)
	}
	body, e := os.ReadFile(reservation)
	if e != nil || string(body) != "replacement" {
		t.Fatal("replacement deleted")
	}
}
