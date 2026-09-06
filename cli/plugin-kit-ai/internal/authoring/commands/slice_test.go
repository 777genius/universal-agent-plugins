package commands_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

const baseline = "4eda915a77f0ca50d4b9211a8e8a0ad0d84c7e72"
const marker = "ordinary-fixture-marker"

func mounted(f ...authoringcli.Factory) (*cobra.Command, error) {
	root := agentpluginscli.NewRoot(agentpluginscli.App{})
	author, e := authoringcli.NewAuthorCommand(f...)
	if e != nil {
		return nil, e
	}
	root.AddCommand(author)
	return root, nil
}
func decodeReport(t *testing.T, b []byte) report.Report {
	t.Helper()
	var r report.Report
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if e := dec.Decode(&r); e != nil {
		t.Fatalf("invalid report: %v\n%s", e, b)
	}
	var extra any
	if e := dec.Decode(&extra); e != io.EOF {
		t.Fatalf("extra JSON/output: %v\n%s", e, b)
	}
	if bytes.Contains(b, []byte(marker)) {
		t.Fatalf("marker leaked: %s", b)
	}
	ids := map[string]bool{}
	for _, f := range r.Findings {
		if ids[f.ID] {
			t.Fatal("duplicate finding")
		}
		ids[f.ID] = true
	}
	for _, a := range []report.Assessment{r.Loadability, r.Conformance, r.HostSafety, r.Readiness, r.Release, r.Runtime} {
		for _, id := range a.FindingIDs {
			if !ids[id] {
				t.Fatalf("dangling finding %s", id)
			}
		}
	}
	return r
}
func execute(t *testing.T, a commands.App, args []string, mount bool) (report.Report, int, []byte) {
	t.Helper()
	var out, errout bytes.Buffer
	builder := commands.RootBuilder(authoringcli.NewPluginKitRoot)
	if mount {
		builder = mounted
		args = append([]string{"author"}, args...)
	}
	e := a.Execute(context.Background(), args, authoringcli.Streams{Out: &out, Err: &errout}, builder)
	if errout.Len() != 0 {
		t.Fatalf("duplicate stderr %q", errout.String())
	}
	code := 0
	if e != nil {
		code = exitx.Code(e)
	}
	return decodeReport(t, out.Bytes()), code, out.Bytes()
}
func write(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(path))
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
}
func plugin(extra string) string {
	return `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"` + extra + `}`
}
func mcp(servers string) string {
	return `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":` + servers + `}`
}
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
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
			value += ":" + target
		} else if !d.IsDir() {
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			sum := sha256.Sum256(b)
			value += ":" + hex.EncodeToString(sum[:])
		}
		out[rel] = value
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func TestReportsAndFreshFactory(t *testing.T) {
	if runtime.GOOS != "linux" && !(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Skip("writable native authoring requires Linux or Windows amd64")
	}
	scratch := t.TempDir()
	a := commands.App{Projects: project.Service{Scratch: scratch}, Revision: baseline}
	cases := []struct {
		name, core, mc, skill string
		load, norm, host      report.State
	}{
		{"plain", plugin(""), "", "", report.Pass, report.Pass, report.Pass},
		{"non-semver", plugin(`,"version":"banana"`), "", "", report.Pass, report.Pass, report.Pass},
		{"unknown", plugin(`,"ordinary-fixture-marker":{"value":"ordinary-fixture-marker"}`), "", "", report.Pass, report.Fail, report.Pass},
		{"extensions", plugin(`,"extensions":17`), "", "", report.Pass, report.Fail, report.Pass},
		{"unsafe-name", strings.Replace(plugin(""), `"demo"`, `"con"`, 1), "", "", report.Pass, report.Pass, report.Fail},
		{"bad-core", `{"name":`, mcp(`{"good":{"type":"stdio","command":"missing-fixture-command"}}`), "", report.Fail, report.Fail, report.NotEvaluated},
		{"wrong-type", plugin(`,"version":17`), "", "", report.Fail, report.Fail, report.NotEvaluated},
		{"duplicate", plugin(`,"name":"ordinary-fixture-marker"`), "", "", report.Fail, report.NotEvaluated, report.Fail},
		{"unsupported-schema", strings.Replace(plugin(""), "1.0.0", "9.0.0", 1), "", "", report.Fail, report.NotEvaluated, report.Fail},
		{"invalid-utf8", string([]byte{'{', 0xff, '}'}), "", "", report.Fail, report.NotEvaluated, report.Fail},
		{"mcp-siblings", plugin(""), mcp(`{"good":{"type":"stdio","command":"missing-fixture-command","env":{"FIXTURE_NOTE":"ordinary-fixture-marker"}},"ordinary-fixture-marker":{"type":"stdio","command":15}}`), "", report.Pass, report.Fail, report.Pass},
		{"remote-redaction", plugin(`,"extensions":{"example.fixture":{"note":"ordinary-fixture-marker"}}`), mcp(`{"remote":{"type":"streamable-http","url":"https://example.invalid/mcp?note=ordinary-fixture-marker","headers":{"X-Fixture-Note":"ordinary-fixture-marker"}}}`), "", report.Pass, report.Pass, report.Pass},
		{"skill-siblings", plugin(""), "", "---\nname: bad\ndescription: 7\n---\n", report.Pass, report.Fail, report.Pass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "plugin.json", tc.core)
			if tc.mc != "" {
				write(t, root, "mcp.json", tc.mc)
			}
			if tc.skill != "" {
				write(t, root, "skills/bad/SKILL.md", tc.skill)
				write(t, root, "skills/good/SKILL.md", "---\nname: good\ndescription: A good fixture.\n---\nFollow the fixture.\n")
			}
			before := tree(t, root)
			for _, command := range []string{"validate", "inspect", "test"} {
				args := []string{command, root, "--format=json"}
				r, c, b := execute(t, a, args, false)
				r2, c2, b2 := execute(t, a, args, true)
				if c != c2 || !bytes.Equal(b, b2) || !reflect.DeepEqual(r, r2) {
					t.Fatalf("entrypoint mismatch: %s / %s", b, b2)
				}
				if r.Loadability.Status != tc.load || r.Conformance.Status != tc.norm || r.HostSafety.Status != tc.host {
					t.Fatalf("wrong assessment: %s", b)
				}
				if r.Runtime.Status != report.NotEvaluated {
					t.Fatal("invented runtime evidence")
				}
				if tc.norm != report.Pass || tc.host != report.Pass {
					if c == 0 {
						t.Fatal("failed checks succeeded")
					}
				} else if c != 0 {
					t.Fatalf("valid checks failed: %s", b)
				}
				if tc.load == report.Fail && r.Coverage.ComponentsRequested {
					t.Fatal("fatal core authorized components")
				}
				if strings.HasSuffix(tc.name, "siblings") {
					good, bad := 0, 0
					for _, v := range r.Components {
						if v.Status == report.Pass {
							good++
						}
						if v.Status == report.Fail {
							bad++
						}
					}
					if good != 1 || bad != 1 {
						t.Fatalf("sibling lost: %+v", r.Components)
					}
				}
				if command == "test" && len(r.Checks) != 5 {
					t.Fatalf("missing real static composition: %+v", r.Checks)
				}
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("source changed")
			}
			entries, e := os.ReadDir(scratch)
			if e != nil || len(entries) != 0 {
				t.Fatalf("scratch leaked: %v %v", entries, e)
			}
		})
	}
	root := t.TempDir()
	write(t, root, "plugin.json", plugin(""))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, c, _ := execute(t, a, []string{"validate", root, "--format=json"}, i%2 == 0)
			if c != 0 || r.Conformance.Status != report.Pass {
				t.Error("concurrent factory failed")
			}
		}(i)
	}
	wg.Wait()
}

func TestArgumentFailuresBeforeEffects(t *testing.T) {
	a := commands.App{Projects: project.Service{Scratch: t.TempDir()}, Revision: baseline}
	parent := t.TempDir()
	destination := filepath.Join(parent, "new")
	for _, args := range [][]string{
		{"init", destination, "--scope=" + marker}, {"init", destination, "--accept-security-risk=false"}, {"init", destination, "--security-details=false"},
		{"init", destination, "--target=" + marker}, {"init", destination, "--dry-run=false"}, {"init", destination, "--force"},
		{"init", destination, "--runtime-test"}, {"test", destination, "--runtime"}, {"test", destination, "--timeout=" + marker},
		{"validate"}, {"validate", destination, marker}, {"validate", destination, "--include-root=" + marker},
		{"compat", destination}, {"bootstrap", destination}, {"init", destination, "--name=" + marker, "--unknown"},
	} {
		for _, mount := range []bool{false, true} {
			r, c, _ := execute(t, a, append(args, "--format=json"), mount)
			if c != 2 || r.Error == nil {
				t.Fatalf("arguments accepted: %v => %+v (%d)", args, r, c)
			}
		}
	}
	if entries, _ := os.ReadDir(parent); len(entries) != 0 {
		t.Fatal("argument failure created destination")
	}
}

func TestInitValidationAndFailurePolicy(t *testing.T) {
	if runtime.GOOS != "linux" && !(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Skip("writable native authoring requires Linux or Windows amd64")
	}
	scratch := t.TempDir()
	a := commands.App{Projects: project.Service{Scratch: scratch}, Revision: baseline}
	parent := t.TempDir()
	dest := filepath.Join(parent, "demo")
	baseArgs := []string{"init", dest, "--template=skill", "--name=demo", "--description=A fixture.", "--format=json"}
	r, c, _ := execute(t, a, baseArgs, false)
	if c != 0 || !r.Committed || len(r.Paths) == 0 {
		t.Fatalf("init: %+v", r)
	}
	validated, code, _ := execute(t, a, []string{"validate", dest, "--format=json"}, true)
	if code != 0 || !reflect.DeepEqual(r.Identity, validated.Identity) {
		t.Fatalf("staging/public validation differ: %+v / %+v", r, validated)
	}
	before := tree(t, dest)
	r, c, _ = execute(t, a, baseArgs, true)
	if c == 0 || r.Committed || r.Error.Code != "destination_exists" {
		t.Fatalf("overwrite: %+v", r)
	}
	if !reflect.DeepEqual(before, tree(t, dest)) {
		t.Fatal("existing destination changed")
	}
	empty := filepath.Join(parent, "empty")
	if e := os.Mkdir(empty, 0700); e != nil {
		t.Fatal(e)
	}
	args := append([]string{}, baseArgs...)
	args[1] = empty
	r, c, _ = execute(t, a, args, false)
	if c == 0 || r.Error.Code != "destination_exists" {
		t.Fatalf("empty overwrite: %+v", r)
	}
	// Real required callback fails closed when acquisition is unavailable. A no-op
	// validator would commit this otherwise perfectly valid generated plan.
	a.Projects.Scratch = filepath.Join(parent, "unavailable-scratch")
	args[1] = filepath.Join(parent, "rejected")
	r, c, _ = execute(t, a, args, false)
	if c == 0 || r.Committed {
		t.Fatalf("fake staging validation: %+v", r)
	}
	if _, e := os.Stat(args[1]); !os.IsNotExist(e) {
		t.Fatal("failed validation committed")
	}
	for _, p := range []string{parent, scratch} {
		entries, _ := os.ReadDir(p)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".authoring-") || strings.HasPrefix(e.Name(), "packageview-") {
				t.Fatal("owned staging leak")
			}
		}
	}
}

func TestConcurrentInitAndCanceledInvocation(t *testing.T) {
	if runtime.GOOS != "linux" && !(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Skip("writable native authoring requires Linux or Windows amd64")
	}
	parent, scratch := t.TempDir(), t.TempDir()
	a := commands.App{Projects: project.Service{Scratch: scratch}, Revision: baseline}
	dest := filepath.Join(parent, "demo")
	args := []string{"init", dest, "--template=skill", "--name=demo", "--description=A fixture.", "--format=json"}
	start := make(chan struct{})
	results := make(chan report.Report, 2)
	for i := 0; i < 2; i++ {
		go func(mount bool) {
			<-start
			r, _, _ := execute(t, a, args, mount)
			results <- r
		}(i == 1)
	}
	close(start)
	wins := 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.Committed {
			wins++
		} else if r.Error == nil || r.Error.Code != "destination_exists" {
			t.Fatalf("unexpected race failure: %+v", r)
		}
	}
	if wins != 1 {
		t.Fatalf("exclusive init winners: %d", wins)
	}
	if _, code, b := execute(t, a, []string{"validate", dest, "--format=json"}, false); code != 0 {
		t.Fatalf("winning tree invalid: %s", b)
	}
	before := tree(t, parent)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, errout bytes.Buffer
	args[1] = filepath.Join(parent, "canceled")
	e := a.Execute(ctx, args, authoringcli.Streams{Out: &out, Err: &errout}, authoringcli.NewPluginKitRoot)
	r := decodeReport(t, out.Bytes())
	if e == nil || exitx.Code(e) != 1 || r.Error == nil || r.Error.Code != "canceled" || r.Committed || errout.Len() != 0 {
		t.Fatalf("canceled invocation: %s (%v)", out.Bytes(), e)
	}
	if !reflect.DeepEqual(before, tree(t, parent)) {
		t.Fatal("canceled command changed source/destination")
	}
	if entries, e := os.ReadDir(scratch); e != nil || len(entries) != 0 {
		t.Fatalf("race/cancellation scratch: %v %v", entries, e)
	}
}

type faultContext struct {
	context.Context
	check func()
}

func (c faultContext) Err() error { c.check(); return c.Context.Err() }

func TestCleanupFailureSurvivesCancellation(t *testing.T) {
	if runtime.GOOS != "linux" && !(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Skip("writable native authoring requires Linux or Windows amd64")
	}
	root, scratch := t.TempDir(), t.TempDir()
	write(t, root, "plugin.json", plugin(""))
	a := commands.App{Projects: project.Service{Scratch: scratch}, Revision: baseline}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	replacement := ""
	fault := faultContext{Context: ctx, check: func() {
		if replacement != "" {
			return
		}
		entries, e := os.ReadDir(scratch)
		if e != nil {
			t.Fatal(e)
		}
		if len(entries) == 0 {
			return
		}
		replacement = filepath.Join(scratch, entries[0].Name())
		if e := os.Rename(replacement, replacement+"-displaced"); e != nil {
			t.Fatal(e)
		}
		write(t, replacement, "preserve", marker)
		cancel()
	}}
	var out, errout bytes.Buffer
	e := a.Execute(fault, []string{"test", root, "--release-policy", "--format=json"}, authoringcli.Streams{Out: &out, Err: &errout}, authoringcli.NewPluginKitRoot)
	r := decodeReport(t, out.Bytes())
	if e == nil || r.Error == nil || r.Error.Code != "private_cleanup_failed" || r.Readiness.Status != report.Fail || errout.Len() != 0 {
		t.Fatalf("cleanup failure hidden: %s (%v)", out.Bytes(), e)
	}
	if r.Release.Status != report.Fail || len(r.Checks) != 5 || !reflect.DeepEqual(r.Checks[1].Assessment, r.HostSafety) {
		t.Fatalf("cleanup failure lost in release/static projection: %s", out.Bytes())
	}
	if replacement == "" {
		t.Fatal("cleanup fault did not run")
	}
	if b, e := os.ReadFile(filepath.Join(replacement, "preserve")); e != nil || string(b) != marker {
		t.Fatal("reader removed unowned replacement")
	}
}

func TestNativeBinaryVerticalSlice(t *testing.T) {
	if runtime.GOOS != "linux" && !(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") {
		t.Skip("writable native authoring requires Linux or Windows amd64")
	}
	_, file, _, _ := runtime.Caller(0)
	module := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	bin := os.Getenv("AUTHORING_NATIVE_BIN_DIR")
	supplied := bin != ""
	revision := baseline + "+vertical-slice-worktree"
	if supplied {
		revision = os.Getenv("EXPECTED_HEAD")
		if len(revision) != 40 {
			t.Fatal("supplied native binaries require EXPECTED_HEAD")
		}
	}
	if !supplied {
		bin = t.TempDir()
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	names := []string{"plugin-kit-ai", "agentplugins"}
	binaries := []string{filepath.Join(bin, names[0]+suffix), filepath.Join(bin, names[1]+suffix)}
	hashes := make([]string, len(binaries))
	prefix := "github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	for i, name := range names {
		if !supplied {
			build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"+suffix), "build", "-p", "2", "-ldflags", "-X "+prefix+".Enabled=vertical-slice-v1 -X "+prefix+".Revision="+revision, "-o", binaries[i], "./cmd/"+name)
			build.Dir = module
			if out, e := build.CombinedOutput(); e != nil {
				t.Fatalf("build actual %s: %v\n%s", name, e, out)
			}
		}
		body, e := os.ReadFile(binaries[i])
		if e != nil {
			t.Fatal(e)
		}
		hashes[i] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	homes := []string{t.TempDir(), t.TempDir()}
	roots := []string{t.TempDir(), t.TempDir()}
	scratch := []string{t.TempDir(), t.TempDir()}
	traps := t.TempDir()
	effect := filepath.Join(traps, "effect")
	trapBody := []byte("#!/bin/sh\nprintf invoked > '" + strings.ReplaceAll(effect, "'", "'\"'\"'") + "'\nexit 79\n")
	if runtime.GOOS == "windows" {
		// A real PE trap catches direct CreateProcess/LookPath execution; a .cmd
		// or shebang fixture alone cannot establish this on Windows.
		source := filepath.Join(t.TempDir(), "trap.go")
		write(t, filepath.Dir(source), filepath.Base(source), "package main\nimport (\"os\")\nfunc main(){os.WriteFile(os.Getenv(\"AUTHORING_TRAP_EFFECT\"), []byte(\"invoked\"), 0600); os.Exit(79)}\n")
		trap := filepath.Join(t.TempDir(), "trap.exe")
		build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-p", "2", "-o", trap, source)
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build Windows execution trap: %v: %s", err, out)
		}
		var err error
		trapBody, err = os.ReadFile(trap)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"node", "npm", "npx", "python", "go", "agent", "claude", "codex", "missing-fixture-command"} {
		if e := os.WriteFile(filepath.Join(traps, name+suffix), trapBody, 0700); e != nil {
			t.Fatal(e)
		}
	}
	// Prove the harness trap works before testing the products, then clear its
	// control effect. This utility is outside every generated package root.
	control := exec.Command(filepath.Join(traps, "node"+suffix))
	control.Env = append(nativeEnvironment(homes[0], scratch[0], traps), "AUTHORING_TRAP_EFFECT="+effect)
	if err := control.Run(); err == nil {
		t.Fatal("execution trap did not fail")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 79 {
		t.Fatalf("execution trap cannot run: %v", err)
	}
	if body, err := os.ReadFile(effect); err != nil || string(body) != "invoked" {
		t.Fatal("execution trap did not record control effect", err)
	}
	if err := os.Remove(effect); err != nil {
		t.Fatal(err)
	}
	runExact := func(i int, args ...string) (report.Report, int, []byte) {
		t.Helper()
		cmd := exec.Command(binaries[i], args...)
		cmd.Dir = homes[i]
		cmd.Env = append(nativeEnvironment(homes[i], scratch[i], traps), "AUTHORING_TRAP_EFFECT="+effect, "AGENTPLUGINS_DIRECTORY_ORIGIN=ordinary-fixture-marker", "AGENTPLUGINS_SECURITY_ORIGIN=ordinary-fixture-marker")
		var out, errout bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errout
		e := cmd.Run()
		code := 0
		if e != nil {
			if ee, ok := e.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				t.Fatal(e)
			}
		}
		if errout.Len() != 0 {
			t.Fatalf("native duplicate/raw stderr: %s", errout.Bytes())
		}
		r := decodeReport(t, out.Bytes())
		if strings.Contains(out.String(), homes[i]) || strings.Contains(out.String(), scratch[i]) {
			t.Fatalf("implicit root disclosure: %s", out.Bytes())
		}
		if r.Revision != revision {
			t.Fatalf("wrong engine revision: %s", r.Revision)
		}
		return r, code, out.Bytes()
	}
	run := func(i int, args ...string) (report.Report, int, []byte) {
		t.Helper()
		if i == 1 {
			args = append([]string{"author"}, args...)
		}
		return runExact(i, append(args, "--format=json")...)
	}
	readProfile := ""
	sdk := map[string]string{}
	for _, tc := range []struct {
		lane  string
		flags []string
	}{
		{"skill", nil}, {"mcp-remote", []string{"--url=https://example.invalid/mcp"}},
		{"mcp-stdio", []string{"--runtime=node"}}, {"hybrid", []string{"--mcp=mcp-stdio", "--runtime=node"}},
		{"hybrid-remote", []string{"--mcp=mcp-remote", "--url=https://example.invalid/mcp"}},
	} {
		t.Run(tc.lane, func(t *testing.T) {
			destinations := []string{filepath.Join(roots[0], tc.lane), filepath.Join(roots[1], tc.lane)}
			template := tc.lane
			if template == "hybrid-remote" {
				template = "hybrid"
			}
			var first []byte
			for i := range binaries {
				args := append([]string{"init", destinations[i], "--template=" + template, "--name=demo", "--description=A disposable fixture."}, tc.flags...)
				r, c, b := run(i, args...)
				if c != 0 || !r.Committed || r.Readiness.Status != report.Pass {
					t.Fatalf("native init failed (%d): %s", c, b)
				}
				if i == 0 {
					first = append([]byte{}, b...)
				} else if !bytes.Equal(first, b) {
					t.Fatalf("native init parity: %s / %s", first, b)
				}
				if _, e := os.Stat(filepath.Join(destinations[i], "plugin", "plugin.yaml")); !os.IsNotExist(e) {
					t.Fatal("legacy scaffold")
				}
			}
			if !reflect.DeepEqual(tree(t, destinations[0]), tree(t, destinations[1])) {
				t.Fatal("native generated bytes/modes differ")
			}
			before := tree(t, destinations[0])
			if tc.lane == "mcp-stdio" {
				lock, err := os.ReadFile(filepath.Join(destinations[0], "package-lock.json"))
				if err != nil {
					t.Fatal(err)
				}
				var manifest struct {
					Packages map[string]struct {
						Version string `json:"version"`
					} `json:"packages"`
				}
				if err := json.Unmarshal(lock, &manifest); err != nil {
					t.Fatal(err)
				}
				sdk["@modelcontextprotocol/sdk"] = manifest.Packages["node_modules/@modelcontextprotocol/sdk"].Version
				sdk["package-lock-sha256"] = fmt.Sprintf("%x", sha256.Sum256(lock))
				if sdk["@modelcontextprotocol/sdk"] == "" {
					t.Fatal("missing generated SDK pin")
				}
			}
			for _, command := range []string{"validate", "inspect", "test"} {
				r, c, b := run(0, command, destinations[0])
				r2, c2, b2 := run(1, command, destinations[1])
				if c != 0 || c2 != 0 || !bytes.Equal(b, b2) {
					t.Fatalf("native %s parity codes %d/%d: %s / %s", command, c, c2, b, b2)
				}
				if r.Runtime.Status != report.NotEvaluated || r2.Runtime.Status != report.NotEvaluated {
					t.Fatal("static runtime evidence fabricated")
				}
				if r.Identity.TreeDigest == "" || r.Identity.ScopeDigest == "" || r.Identity.ScopeDigest == r.Identity.TreeDigest {
					t.Fatal("missing or conflated identity")
				}
				if r.Identity.ReadProfile != "packageview-local-"+runtime.GOOS+"-v1" {
					t.Fatalf("unexpected native filesystem read profile: %s", r.Identity.ReadProfile)
				}
				readProfile = r.Identity.ReadProfile
				t.Logf("native journey: template=%s command=%s exit=0 scope=%s tree=%s boundary=static-only", tc.lane, command, r.Identity.ScopeDigest, r.Identity.TreeDigest)
			}
			if !reflect.DeepEqual(before, tree(t, destinations[0])) {
				t.Fatal("native read changed source")
			}
		})
	}
	// Native failure parity, including raw flag values and an unresolvable binary.
	for _, tc := range []struct{ name, body, mc string }{
		{"unknown", plugin(`,"ordinary-fixture-marker":true`), ""},
		{"extensions", plugin(`,"extensions":17`), ""},
		{"wrong-type", plugin(`,"version":17`), ""},
		{"invalid-utf8", string([]byte{'{', 0xff, '}'}), ""},
		{"duplicate", plugin(`,"name":"ordinary-fixture-marker"`), ""},
		{"bad-schema", strings.Replace(plugin(""), "1.0.0", "7.0.0", 1), ""},
		{"bad-sibling", plugin(""), mcp(`{"good":{"type":"stdio","command":"missing-fixture-command"},"bad":{"type":"stdio","command":4}}`)},
		{"unsafe", strings.Replace(plugin(""), `"demo"`, `"con"`, 1), ""},
		{"missing-executable", plugin(""), mcp(`{"good":{"type":"stdio","command":"absent-fixture-executable"}}`)},
		{"legacy-only", "", ""},
		{"native-only", "", ""},
		{"coexisting", plugin(""), ""},
	} {
		var first []byte
		firstCode := 0
		for i := range binaries {
			root := filepath.Join(roots[i], "negative-"+tc.name)
			if tc.body != "" {
				write(t, root, "plugin.json", tc.body)
			}
			if tc.name == "legacy-only" || tc.name == "coexisting" {
				write(t, root, "plugin/plugin.yaml", marker)
			}
			if tc.name == "native-only" {
				write(t, root, ".codex-plugin/plugin.json", plugin(""))
			}
			if tc.mc != "" {
				write(t, root, "mcp.json", tc.mc)
			}
			before := tree(t, root)
			r, c, b := run(i, "test", root)
			if i == 0 {
				first = b
				firstCode = c
			} else if c != firstCode || !bytes.Equal(first, b) {
				t.Fatalf("native failure parity %s", tc.name)
			}
			if tc.name == "missing-executable" && c != 0 {
				t.Fatalf("static config resolved PATH: %s", b)
			}
			if tc.name != "missing-executable" && c == 0 {
				t.Fatalf("native negative accepted: %s", b)
			}
			if tc.name == "coexisting" {
				if r.Conformance.Status != report.Pass || r.Readiness.Status == report.Pass || r.Identity.TreeDigest != "" {
					t.Fatalf("coexisting metadata policy/identity: %s", b)
				}
				strict, code, _ := run(i, "validate", root, "--release-policy")
				if code == 0 || strict.Release.Status != report.Fail {
					t.Fatal("release hygiene accepted legacy metadata")
				}
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("native negative changed source")
			}
		}
	}
	for _, flags := range [][]string{{"--scope=" + marker}, {"--accept-security-risk=false"}, {"--security-details=false"}, {"--include-root=" + marker}, {"--force"}, {"--runtime-test"}} {
		var first []byte
		for i := range binaries {
			dest := filepath.Join(roots[i], "must-not-exist")
			args := append([]string{"init", dest}, flags...)
			_, c, b := run(i, args...)
			if c != 2 {
				t.Fatalf("native flag failure exit %d: %s", c, b)
			}
			if i == 0 {
				first = b
			} else if !bytes.Equal(first, b) {
				t.Fatal("native argument parity")
			}
			if _, e := os.Stat(dest); !os.IsNotExist(e) {
				t.Fatal("native flag caused effects")
			}
		}
	}
	for i := range homes {
		for _, args := range [][]string{{"test", roots[i], "--runtime"}, {"bootstrap", roots[i]}, {"compat", roots[i]}} {
			if _, code, b := run(i, args...); code != 2 {
				t.Fatalf("unsupported action accepted: %s", b)
			}
		}
		if entries, _ := os.ReadDir(homes[i]); len(entries) != 0 {
			t.Fatalf("user home changed: %v", entries)
		}
		if entries, _ := os.ReadDir(scratch[i]); len(entries) != 0 {
			t.Fatalf("scratch leaked: %v", entries)
		}
	}
	// Installer-only flags before the author group must use the same sanitized
	// author boundary, before production installer dependency construction.
	for _, flag := range []string{"--scope=" + marker, "--target=" + marker, "--accept-security-risk=false"} {
		if _, code, b := runExact(1, flag, "author", "init", filepath.Join(roots[1], "never"), "--format=json"); code != 2 {
			t.Fatalf("enclosing flag escaped author routing: %s", b)
		}
	}
	for _, flags := range [][]string{{"--ordinary-review-option", "ordinary-review-value"}, {"--ordinary-review-option=ordinary-review-value"}} {
		var first []byte
		for i := range binaries {
			args := append([]string{}, flags...)
			if i == 1 {
				args = append(args, "author")
			}
			args = append(args, "validate", filepath.Join(roots[i], "absent"), "--format=json")
			r, code, b := runExact(i, args...)
			if code != 2 || r.Error == nil || r.Error.Code != "arguments_invalid" {
				t.Fatalf("unknown flag escaped author routing: %s", b)
			}
			if i == 0 {
				first = b
			} else if !bytes.Equal(first, b) {
				t.Fatal("unknown flag native parity")
			}
		}
	}
	if _, e := os.Stat(effect); !os.IsNotExist(e) {
		t.Fatal("static authoring executed a process")
	}
	if t.Failed() {
		return
	}
	for i := range names {
		body, err := os.ReadFile(binaries[i])
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(body)) != hashes[i] {
			t.Fatal("native binary changed during flow")
		}
	}
	for i, name := range names {
		evidence, err := json.Marshal(struct {
			Entrypoint  string            `json:"entrypoint"`
			SHA256      string            `json:"sha256"`
			Revision    string            `json:"revision"`
			ReadProfile string            `json:"read_profile"`
			Templates   []string          `json:"templates"`
			SDK         map[string]string `json:"sdk"`
		}{name, hashes[i], revision, readProfile, []string{"skill", "mcp-remote", "mcp-stdio", "hybrid", "hybrid-remote"}, sdk})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("AUTHORING_NATIVE_E2E %s", evidence)
	}

}

// Only disposable profile/config/temp paths reach the binaries under test.
// SystemRoot is required by Windows process startup and is not a user profile.
func nativeEnvironment(home, scratch, path string) []string {
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + filepath.Join(home, "appdata"), "LOCALAPPDATA=" + filepath.Join(home, "localappdata"),
		"XDG_CONFIG_HOME=" + home, "XDG_CACHE_HOME=" + filepath.Join(home, "cache"), "XDG_DATA_HOME=" + filepath.Join(home, "data"), "XDG_STATE_HOME=" + filepath.Join(home, "state"),
		"TMPDIR=" + scratch, "TMP=" + scratch, "TEMP=" + scratch, "PATH=" + path, "GIT_CONFIG_GLOBAL=" + filepath.Join(home, "gitconfig"), "GIT_CONFIG_NOSYSTEM=1"}
	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(home)
		env = append(env, "SystemRoot="+os.Getenv("SystemRoot"), "HOMEDRIVE="+volume, "HOMEPATH="+strings.TrimPrefix(home, volume), "PATHEXT=.EXE")
	}
	return env
}
