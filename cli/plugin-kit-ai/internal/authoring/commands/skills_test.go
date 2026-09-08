package commands_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
)

func TestSkillsFactoriesAndEntrypointParity(t *testing.T) {
	a := commands.App{Projects: project.Service{Scratch: t.TempDir()}, Revision: "skills-factories"}
	roots := []string{t.TempDir(), t.TempDir()}
	for _, root := range roots {
		write(t, root, "plugin.json", plugin(""))
		write(t, root, "keep", marker)
	}
	var initial report.Report
	for i, root := range roots {
		r, code, _ := execute(t, a, []string{"skills", "init", "docs-helper", root, "--description", marker, "--format=json"}, i == 1)
		if code != 0 || !r.Committed || !r.Successful() {
			t.Fatalf("init %d: %+v", code, r)
		}
		if i == 0 {
			initial = r
		} else if !reflect.DeepEqual(initial, r) {
			t.Fatalf("init parity:\n%+v\n%+v", initial, r)
		}
	}
	for _, verb := range []string{"skills", "validate", "test"} {
		args := []string{verb, roots[0], "--format=json"}
		if verb == "skills" {
			args = []string{"skills", "validate", roots[0], "--format=json"}
		}
		r, code, body := execute(t, a, args, false)
		_, other, second := execute(t, a, args, true)
		if code != 0 || other != code || !bytes.Equal(body, second) || r.Runtime.Status != report.NotEvaluated {
			t.Fatalf("static parity: %d %d", code, other)
		}
		found := false
		for _, p := range r.Profiles {
			if p.ID == conformance.ProfileIdentities()[1].ID && p.Digest == conformance.ProfileIdentities()[1].Digest {
				found = true
			}
		}
		if !found {
			t.Fatal("embedded skill provenance missing")
		}
	}
	for _, args := range [][]string{
		{"skills", "init", "next", roots[0]},
		{"skills", "init", "../" + marker, roots[0], "--description", marker},
		{"skills", "init", "docs-helper", roots[0], "--description", marker},
		{"skills", "install", marker}, {"skills", "generate", marker}, {"skills", "remove", marker},
		{"skills", "validate", roots[0], "--profile", marker},
		{"skills", "init", "new", roots[0], "--description", marker, "--output", marker},
		{"skills", "init", "new", roots[0], "--description", marker, "--dry-run"},
		{"skills", "validate", roots[0], "--target", marker},
	} {
		args = append(args, "--format=json")
		before := tree(t, roots[0])
		_, code, body := execute(t, a, args, false)
		_, other, second := execute(t, a, args, true)
		if code == 0 || code != other || !bytes.Equal(body, second) {
			t.Fatalf("failure parity %v: %d %d\n%s\n%s", args, code, other, body, second)
		}
		if !reflect.DeepEqual(before, tree(t, roots[0])) {
			t.Fatal("rejected request mutated source")
		}
	}
	// Shared App never retains failed/inherited flags or description values.
	for _, mount := range []bool{false, true} {
		_, code, _ := execute(t, a, []string{"skills", "validate", roots[0], "--format=json"}, mount)
		if code != 0 {
			t.Fatal("flags leaked")
		}
		var help bytes.Buffer
		build := commands.RootBuilder(authoringcli.NewPluginKitRoot)
		args := []string{"skills", "init", "--help"}
		if mount {
			build = mounted
			args = append([]string{"author"}, args...)
		}
		if err := a.Execute(context.Background(), args, authoringcli.Streams{Out: &help, Err: &help}, build); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"[package-path]", "current directory", "--description", "No ancestor search"} {
			if !strings.Contains(help.String(), want) {
				t.Fatalf("help omits %s: %s", want, help.String())
			}
		}
	}
	c, code, _ := execute(t, a, []string{"capabilities", "--format=json"}, false)
	if code != 0 || !reflect.DeepEqual(c.Capabilities.Commands, []string{"capabilities", "compat", "doctor", "init", "inspect", "skills", "test", "validate"}) {
		t.Fatalf("actual registration metadata: %+v", c.Capabilities)
	}
}
func TestSkillsExactCWD(t *testing.T) {
	root := t.TempDir()
	write(t, root, "plugin.json", plugin(""))
	child := filepath.Join(root, "child")
	if e := os.Mkdir(child, 0700); e != nil {
		t.Fatal(e)
	}
	a := commands.App{Projects: project.Service{Scratch: t.TempDir()}}
	t.Chdir(filepath.Dir(root))
	r, code, _ := execute(t, a, []string{"skills", "init", "relative", "./" + filepath.Base(root), "--description", "text", "--format=json"}, false)
	if code != 0 || !r.Committed {
		t.Fatalf("documented relative package path failed: %d %+v", code, r)
	}
	t.Chdir(root)
	for _, mount := range []bool{false, true} {
		r, code, _ := execute(t, a, []string{"skills", "validate", "--format=json"}, mount)
		if code != 0 || !r.Successful() {
			t.Fatal("exact cwd not used")
		}
	}
	t.Chdir(child)
	before := tree(t, root)
	for _, mount := range []bool{false, true} {
		r, code, _ := execute(t, a, []string{"skills", "init", "new", "--description", "text", "--format=json"}, mount)
		if code == 0 || r.Committed {
			t.Fatal("ancestor package discovered")
		}
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("ancestor mutated")
	}
}

func TestSkillsHumanAllowlist(t *testing.T) {
	root := t.TempDir()
	write(t, root, "plugin.json", plugin(""))
	write(t, root, "skills/bad/SKILL.md", "---\nname: bad\ndescription: ["+marker+"]\n---\n")
	a := commands.App{Projects: project.Service{Scratch: t.TempDir()}}
	for _, mount := range []bool{false, true} {
		var out bytes.Buffer
		args := []string{"skills", "validate", root}
		build := commands.RootBuilder(authoringcli.NewPluginKitRoot)
		if mount {
			build = mounted
			args = append([]string{"author"}, args...)
		}
		err := a.Execute(context.Background(), args, authoringcli.Streams{Out: &out, Err: &out}, build)
		if err == nil || !strings.Contains(out.String(), "conformance fail") || !strings.Contains(out.String(), "host pass") || strings.Contains(out.String(), marker) || strings.Contains(out.String(), root) {
			t.Fatalf("human contract: %v %s", err, out.String())
		}
	}
}

func TestSkillsPostCommitErrorIsExplicit(t *testing.T) {
	for _, mount := range []bool{false, true} {
		for _, jsonOutput := range []bool{false, true} {
			root := t.TempDir()
			write(t, root, "plugin.json", plugin(""))
			a := commands.App{Projects: project.Service{Scratch: t.TempDir(), Limits: packageview.Limits{Entries: 1}}}
			args := []string{"skills", "init", "new", "--description", "text", root}
			var out bytes.Buffer
			build := commands.RootBuilder(authoringcli.NewPluginKitRoot)
			if mount {
				build = mounted
				args = append([]string{"author"}, args...)
			}
			if jsonOutput {
				args = append(args, "--format=json")
			}
			err := a.Execute(context.Background(), args, authoringcli.Streams{Out: &out, Err: &out}, build)
			if err == nil || !strings.Contains(out.String(), "The Skill was committed") {
				t.Fatalf("postcommit error missing: %v %s", err, out.String())
			}
			if _, e := os.Stat(filepath.Join(root, "skills/new/SKILL.md")); e != nil {
				t.Fatal("lost committed file", e)
			}
			if jsonOutput {
				r := decodeReport(t, out.Bytes())
				if !r.Committed || r.Error == nil {
					t.Fatal("false rollback claim")
				}
			}
		}
	}
}
