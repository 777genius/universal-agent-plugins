package commands_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
)

func TestReviewSplitUnknownAuthorFlag(t *testing.T) {
	// A no-effect subtree is sufficient to check Cobra's command selection.
	plain := agentpluginscli.NewRoot(agentpluginscli.App{})
	author, err := authoringcli.NewAuthorCommand(func() (*cobra.Command, error) {
		return &cobra.Command{Use: "validate", RunE: func(*cobra.Command, []string) error { return nil }}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	plain.AddCommand(author)
	args := []string{"--ordinary-review-option", "ordinary-review-value", "author", "validate", filepath.Join(t.TempDir(), "absent"), "--format=json"}
	selected, _, err := plain.Find(args)
	if err != nil {
		t.Fatal(err)
	}
	routed := commands.IsAuthorInvocation(args, agentpluginscli.NewRoot(agentpluginscli.App{}))
	t.Logf("Cobra selects %q; author dispatcher selects author=%t", selected.CommandPath(), routed)

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	binary := filepath.Join(os.Getenv("AUTHORING_NATIVE_BIN_DIR"), "agentplugins"+suffix)
	if os.Getenv("AUTHORING_NATIVE_BIN_DIR") == "" {
		binary = filepath.Join(t.TempDir(), "agentplugins"+suffix)
		_, file, _, _ := runtime.Caller(0)
		module := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
		prefix := "github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
		build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"+suffix), "build", "-p", "2", "-ldflags", "-X "+prefix+".Enabled=vertical-slice-v1 -X "+prefix+".Revision=8d514ba723bf1c564ec1fbf92a3858f51d13e641", "-o", binary, "./cmd/agentplugins")
		build.Dir = module
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
	}
	home, scratch := t.TempDir(), t.TempDir()
	native := func(argv []string) (int, string, string) {
		c := exec.Command(binary, argv...)
		c.Dir = home
		c.Env = append(nativeEnvironment(home, scratch, t.TempDir()), "AGENTPLUGINS_DIRECTORY_ORIGIN=ordinary-review-origin")
		var out, errout bytes.Buffer
		c.Stdout, c.Stderr = &out, &errout
		code := 0
		if err := c.Run(); err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				t.Fatal(err)
			}
		}
		return code, out.String(), errout.String()
	}
	control := append([]string{"--ordinary-review-option=ordinary-review-value"}, args[2:]...)
	code, stdout, stderr := native(control)
	t.Logf("equals control: exit=%d JSON=%t stderr=%q", code, json.Valid([]byte(stdout)), stderr)
	if code != 2 || !json.Valid([]byte(stdout)) || stderr != "" {
		t.Fatal("control failed")
	}
	code, stdout, stderr = native(args)
	t.Logf("split flag: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	if !routed || code != 2 || !json.Valid([]byte(stdout)) || stderr != "" {
		t.Error("split rejected flag escapes author routing and initializes installer dependencies")
	}
}

func TestReviewInitCleanupFailurePrecedence(t *testing.T) {
	// Windows pins prevent replacing the held stage. An unrelated container
	// entry instead makes real, nonrecursive cleanup fail after cancellation.
	// Exercise that fixture on Linux too, alongside the actual replacement.
	faults := []string{"retained-container-entry"}
	if runtime.GOOS != "windows" {
		faults = append(faults, "stage-replacement")
	}
	for _, fixture := range faults {
		for _, throughCommand := range []bool{false, true} {
			boundary := "shared-apply"
			if throughCommand {
				boundary = "command"
			}
			t.Run(fixture+"/"+boundary, func(t *testing.T) {
				parent, scratch := physicalMutationRoot(t), t.TempDir()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				replaced := ""
				var stageInfo os.FileInfo
				fault := faultContext{Context: ctx, check: func() {
					if replaced != "" {
						return
					}
					entries, err := os.ReadDir(parent)
					if err != nil {
						t.Fatal(err)
					}
					for _, entry := range entries {
						if !strings.HasPrefix(entry.Name(), ".authoring-") {
							continue
						}
						replaced = filepath.Join(parent, entry.Name())
						stageInfo, err = os.Stat(replaced)
						if err != nil {
							t.Fatal(err)
						}
						if fixture == "stage-replacement" {
							if err := os.Rename(replaced, replaced+"-displaced"); err != nil {
								t.Fatal(err)
							}
						}
						write(t, replaced, "preserve", "ordinary-review-value")
						cancel()
						return
					}
				}}
				dest := filepath.Join(parent, "demo")
				if !throughCommand {
					plan, err := scaffold.BuildPlan(scaffold.Options{Template: scaffold.Skill, Name: "demo", Description: "A disposable review fixture."})
					if err != nil {
						t.Fatal(err)
					}
					result, err := scaffold.Apply(fault, plan, scaffold.ApplyOptions{Destination: dest, Validate: func(ctx context.Context, stage string, _ *os.Root) error {
						p, err := (project.Service{Scratch: scratch}).Read(ctx, stage)
						if err != nil {
							return err
						}
						if !report.Build("validate", "review", p, false).Successful() {
							return fmt.Errorf("staging validation failed")
						}
						return nil
					}})
					t.Logf("shared Apply: committed=%t err=%v", result.Committed, err)
					var cleanup *scaffold.CleanupError
					if !errors.Is(err, context.Canceled) || !errors.As(err, &cleanup) || result.Committed {
						t.Fatal("joined cleanup/cancellation fault not preserved")
					}
					if fixture == "stage-replacement" {
						if !strings.Contains(err.Error(), "refused sibling cleanup") {
							t.Fatal("replacement ownership refusal missing")
						}
					} else {
						reviewCleanupRequireNonempty(t, cleanup.Err, filepath.Base(replaced))
						if !strings.Contains(cleanup.Error(), "cleanup staging ") || !strings.Contains(cleanup.Error(), fmt.Sprintf("%q", replaced)) {
							t.Fatal("expected private container cleanup cause")
						}
					}
				} else {
					a := commands.App{Projects: project.Service{Scratch: scratch}, Revision: "8d514ba723bf1c564ec1fbf92a3858f51d13e641"}
					var out, errout bytes.Buffer
					err := a.Execute(fault, []string{"init", dest, "--template=skill", "--name=demo", "--description=A disposable review fixture.", "--format=json"}, authoringcli.Streams{Out: &out, Err: &errout}, authoringcli.NewPluginKitRoot)
					var r report.Report
					if e := json.Unmarshal(out.Bytes(), &r); e != nil {
						t.Fatal(e)
					}
					t.Logf("command: committed=%t error=%+v returned=%v", r.Committed, r.Error, err)
					if err == nil || r.Committed || r.Error == nil {
						t.Fatal("unexpected init result")
					}
					if r.Error.Code != "private_cleanup_failed" {
						t.Errorf("scaffold cleanup failure hidden by %q", r.Error.Code)
					}
					if !strings.Contains(err.Error(), "cleanup") || !strings.Contains(r.Error.Action, "staging") || !strings.Contains(r.Error.Action, "committed") {
						t.Error("sanitized recovery evidence missing")
					}
					for _, value := range []string{parent, scratch, ".authoring-", "ordinary-review-value"} {
						if strings.Contains(out.String()+err.Error(), value) {
							t.Error("private cleanup value leaked")
						}
					}
					if errout.Len() != 0 {
						t.Error("raw cleanup stderr")
					}
				}
				if replaced == "" || !errors.Is(ctx.Err(), context.Canceled) {
					t.Fatal("stage fault was not reached")
				}
				if b, err := os.ReadFile(filepath.Join(replaced, "preserve")); err != nil || string(b) != "ordinary-review-value" {
					t.Fatal("unowned fixture content was removed")
				}
				if fixture == "stage-replacement" {
					if _, err := os.Stat(replaced + "-displaced"); err != nil {
						t.Fatal("expected retained displaced container")
					}
				} else {
					reviewCleanupVerifyRetainedEntry(t, replaced, stageInfo)
				}
				if _, err := os.Stat(dest); !os.IsNotExist(err) {
					t.Fatal("unexpected destination")
				}
			})
		}
	}
}

func TestReviewUnavailableComponentProjection(t *testing.T) {
	for _, pending := range []string{`"command":"./missing-review-file"`, `"command":"./bin/good","cwd":"./missing-review-file"`} {
		root, scratch := t.TempDir(), t.TempDir()
		write(t, root, "plugin.json", plugin(""))
		write(t, root, "bin/good", "ordinary review inert file")
		write(t, root, "mcp.json", mcp(`{"good":{"type":"stdio","command":"./bin/good"},"pending":{"type":"stdio",`+pending+`}}`))
		p, err := (project.Service{Scratch: scratch}).Read(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		r := report.Build("inspect", "8d514ba723bf1c564ec1fbf92a3858f51d13e641", p, false)
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("report=%s", b)
		if r.Successful() {
			t.Fatal("unexpected aggregate success")
		}
		ids := map[string]bool{}
		for _, c := range r.Components {
			ids[c.ID] = true
		}
		for _, f := range r.Findings {
			if f.Code == "path_containment_unavailable" {
				if !ids[f.ItemID] {
					t.Errorf("containment finding item_id=%s matches no component ID", f.ItemID)
				}
			}
		}
		wantPending := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("mcp:pending")))
		found := false
		for _, f := range r.Findings {
			if f.Code == "path_containment_unavailable" {
				found = true
				if f.ItemID != wantPending {
					t.Error("finding points to wrong sibling")
				}
			}
		}
		if !found {
			t.Fatal("missing containment finding")
		}
		for _, c := range r.Components {
			if c.ID == wantPending && c.Status != report.NotEvaluated {
				t.Error("unknown component must remain not evaluated")
			}
		}
		passes := 0
		for _, c := range r.Components {
			if c.Status == report.Pass {
				passes++
			}
		}
		if passes != 1 {
			t.Errorf("good and unobserved bundled command both project pass: count=%d", passes)
		}
		for _, marker := range []string{"ordinary-review-executable", "missing-review-file", root, scratch} {
			if strings.Contains(string(b), marker) {
				t.Errorf("report leaked %q", marker)
			}
		}
		if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
			t.Fatalf("scratch not empty: %v %v", entries, err)
		}
	}
}

func TestAuthorSelectionPreservesFlagValuesAndInstaller(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		author bool
	}{
		{[]string{"--target", "author", "add", "ordinary-fixture"}, false},
		{[]string{"--target=author", "add", "ordinary-fixture"}, false},
		{[]string{"add", "author"}, false},
		{[]string{"validate", "author"}, false},
		{[]string{"--target", "ordinary-fixture", "author", "validate", "."}, true},
		{[]string{"--target=ordinary-fixture", "author", "validate", "."}, true},
		{[]string{"--accept-security-risk=false", "author", "validate", "."}, true},
		{[]string{"--ordinary-option", "ordinary-value", "author", "validate", "."}, true},
		{[]string{"--ordinary-option=ordinary-value", "author", "validate", "."}, true},
	} {
		if got := commands.IsAuthorInvocation(tc.args, agentpluginscli.NewRoot(agentpluginscli.App{})); got != tc.author {
			t.Errorf("%v: author=%t", tc.args, got)
		}
	}
}

func TestReviewSkillFindingProjection(t *testing.T) {
	root, scratch := t.TempDir(), t.TempDir()
	write(t, root, "plugin.json", plugin(""))
	names := []string{"ordinary-first", "ordinary-second"}
	for _, name := range names {
		write(t, root, "skills/"+name+"/SKILL.md", "---\nname: "+name+"\ndescription: 7\n---\n")
	}
	write(t, root, "skills/ordinary-valid/SKILL.md", "---\nname: ordinary-valid\ndescription: A disposable fixture.\n---\nFollow the fixture.\n")
	// Same private identity in another boundary must retain its own namespace.
	write(t, root, "mcp.json", mcp(`{"ordinary-first":{"type":"stdio","command":7}}`))
	p, err := (project.Service{Scratch: scratch}).Read(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	hash := func(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
	r := report.Build("inspect", "review", p, false)
	components := map[string]report.State{}
	for _, c := range r.Components {
		components[c.ID] = c.Status
	}
	for _, name := range names {
		id := hash("skill:" + name)
		if components[id] != report.Fail {
			t.Errorf("invalid Skill component missing: %s", id)
		}
		found := false
		for _, f := range r.Findings {
			if f.Location == "skills" && f.ItemID == id {
				found = true
			}
		}
		if !found {
			t.Errorf("invalid Skill sibling diagnostic does not correlate: %s", id)
		}
	}
	if r.Successful() {
		t.Fatal("invalid siblings passed aggregate")
	}
	if components[hash("skill:ordinary-valid")] != report.Pass {
		t.Fatal("good sibling lost")
	}
	// Exercise projection of retained identities without a new parser or reread.
	for _, tc := range []struct {
		code, item string
		boundary   domain.FailureBoundary
		want       string
	}{
		{"retained_skill", hash("ordinary-valid"), domain.BoundarySkill, hash("skill:ordinary-valid")},
		{"missing_skill", "", domain.BoundarySkill, ""},
		{"unknown_skill", hash("ordinary-unknown"), domain.BoundarySkill, hash("ordinary-unknown")},
		{"retained_mcp", hash("ordinary-first"), domain.BoundaryMCPServer, hash("mcp:ordinary-first")},
		{"unknown_mcp", hash("ordinary-valid"), domain.BoundaryMCPServer, hash("ordinary-valid")},
	} {
		p.Facts.Findings = append(p.Facts.Findings, conformance.Finding{Code: tc.code, Item: tc.item, Boundary: tc.boundary, Layer: conformance.HostSafety})
		projected := report.Build("inspect", "review", p, false)
		found := false
		for _, f := range projected.Findings {
			if f.Code == tc.code {
				found = true
				if f.ItemID != tc.want {
					t.Errorf("%s: item=%s want=%s", tc.code, f.ItemID, tc.want)
				}
			}
		}
		if !found {
			t.Fatalf("missing %s", tc.code)
		}
	}
	b, err := json.Marshal(report.Build("inspect", "review", p, false))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"ordinary-first", "ordinary-second", "ordinary-valid", "ordinary-unknown", root, scratch} {
		if strings.Contains(string(b), marker) {
			t.Errorf("projection leaked %q", marker)
		}
	}
	if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
		t.Fatalf("scratch not empty: %v %v", entries, err)
	}
}
