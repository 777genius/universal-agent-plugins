package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type projectGitDiagnosticDelegate func(context.Context, sourceacquisition.Command) ([]byte, error)

func (f projectGitDiagnosticDelegate) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	return f(ctx, command)
}

type projectGitInspectionTrap struct{}

func (*projectGitInspectionTrap) Error() string {
	panic("argv/stdout/stderr/secret cause must not be inspected")
}

func projectGitDiagnosticFixture(t *testing.T) (projectGitDiagnosticRunner, string, *[]string) {
	t.Helper()
	root := projectJourneyTemp(t)
	dir := filepath.Join(root, "agentplugins-git-inert")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	logs := []string{}
	d := &projectGitDiagnostic{counts: map[string]uint16{}, endpoint: "https://127.0.0.1:12345/repo.git", log: func(record string) { logs = append(logs, record) }}
	return projectGitDiagnosticRunner{recorder: d, root: root}, dir, &logs
}

func TestProjectGitDiagnosticPreservesOneCallInputsAndExactResult(t *testing.T) {
	r, dir, logs := projectGitDiagnosticFixture(t)
	ctx := context.WithValue(t.Context(), struct{}{}, "inert")
	command := sourceacquisition.Command{Dir: dir, Args: []string{"init", "--quiet", filepath.Join(dir, "repository")}, Stdin: []byte("same input"), MaxOutputBytes: 17}
	body, original := []byte("same returned bytes"), &projectGitInspectionTrap{}
	calls := 0
	r.delegate = projectGitDiagnosticDelegate(func(got context.Context, c sourceacquisition.Command) ([]byte, error) {
		calls++
		if got != ctx || !reflect.DeepEqual(c, command) || &c.Args[0] != &command.Args[0] || &c.Stdin[0] != &command.Stdin[0] {
			t.Fatal("decorator changed context/command authority")
		}
		return body, original
	})
	got, err := r.Run(ctx, command)
	if calls != 1 || err != original || &got[0] != &body[0] || len(*logs) != 1 {
		t.Fatal("original call/result/error identity changed")
	}
	if strings.Contains((*logs)[0], "secret") || !strings.Contains((*logs)[0], "phase=init") || !strings.Contains((*logs)[0], "cause=cause_omitted") {
		t.Fatal("nonnative argv/stream-bearing error was inspected")
	}
}

func TestProjectGitDiagnosticOnlyAdmitsNativeKnownFixtureAuthority(t *testing.T) {
	for _, kind := range []string{"explicit", "origin-registered", "origin-unregistered", "unknown-grammar", "external-endpoint", "userinfo", "query", "empty-query", "fragment", "outside-workspace"} {
		t.Run(kind, func(t *testing.T) {
			r, dir, logs := projectGitDiagnosticFixture(t)
			repo := filepath.Join(dir, "repository")
			var returned error = errors.New("read stable proc identity failed")
			r.delegate = projectGitDiagnosticDelegate(func(context.Context, sourceacquisition.Command) ([]byte, error) { return nil, returned })
			if kind == "origin-registered" {
				returned = nil
				_, err := r.Run(t.Context(), sourceacquisition.Command{Dir: dir, Args: []string{"-C", repo, "remote", "add", "origin", r.recorder.endpoint}})
				if err != nil {
					t.Fatal(err)
				}
				returned = errors.New("read stable proc identity failed")
			}
			args := []string{"-C", repo, "ls-remote", "--symref", "--", r.recorder.endpoint, "HEAD"}
			switch kind {
			case "origin-registered", "origin-unregistered":
				args = []string{"-C", repo, "fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", "origin", strings.Repeat("1", 40)}
			case "unknown-grammar":
				args = append(args, "unreviewed")
			case "external-endpoint":
				r.recorder.endpoint = "https://outside.example/repo.git"
				args[5] = r.recorder.endpoint
			case "userinfo":
				r.recorder.endpoint = "https://name:password@127.0.0.1:12345/repo.git"
				args[5] = r.recorder.endpoint
			case "query":
				r.recorder.endpoint += "?key=inert"
				args[5] = r.recorder.endpoint
			case "empty-query":
				r.recorder.endpoint += "?"
				args[5] = r.recorder.endpoint
			case "fragment":
				r.recorder.endpoint += "#"
				args[5] = r.recorder.endpoint
			case "outside-workspace":
				r.root = projectJourneyTemp(t)
			}
			positive := kind == "explicit" || kind == "origin-registered"
			if !positive {
				returned = &projectGitInspectionTrap{}
			}
			_, err := r.Run(t.Context(), sourceacquisition.Command{Dir: dir, Args: args, NativeCredentials: true})
			if err != returned || len(*logs) != 1 {
				t.Fatal("delegate outcome or accounting changed")
			}
			omitted := strings.Contains((*logs)[0], "cause=cause_omitted")
			if omitted == positive {
				t.Fatalf("positiveauthority=%v log=%s", positive, (*logs)[0])
			}
		})
	}
}

func TestProjectGitDiagnosticCauseInputRedactionAndFinalASCIIBytesAreBounded(t *testing.T) {
	for _, input := range []string{strings.Repeat("x", 8193), string([]byte{0xff}), "Authorization: Bearer private", "password=inert", "token:private", "cookie=value"} {
		if got := projectGitDiagnosticCause(errors.New(input), "/known/root"); got != "cause_omitted" {
			t.Fatal("unsafe/oversized input was admitted")
		}
	}
	got := projectGitDiagnosticCause(errors.New("read /known/root/agentplugins-git-inert/repository failed; https://user:private@elsewhere.example/path?q=inert"), "/known/root", "/known/root/agentplugins-git-inert/repository")
	for _, secret := range []string{"/known/root", "private", "elsewhere.example", "https://"} {
		if strings.Contains(got, secret) {
			t.Fatal("URI/locator leaked")
		}
	}
	if !strings.Contains(got, "<redacted-uri>") {
		t.Fatal("URI was not explicitly redacted")
	}
	got = projectGitDiagnosticCause(errors.New(strings.Repeat("é\x00\n", 1500)))
	if len(got) > 2048 || !strings.Contains(got, "truncated") {
		t.Fatalf("final bounded quote differs: %d", len(got))
	}
	for _, value := range []byte(got) {
		if value < 0x20 || value > 0x7e {
			t.Fatal("control or nonASCII output escaped the quote")
		}
	}
}

func TestProjectGitDiagnosticClassesCountsAndUnknownPhaseNeverInferAcquisition(t *testing.T) {
	for _, tc := range []struct {
		err   error
		class string
	}{{context.Canceled, "context_cancelled"}, {context.DeadlineExceeded, "context_deadline"}, {processadapter.ErrOrdinaryContainmentUnsupported, "ordinary_unsupported"}, {processadapter.ErrStdoutLimitExceeded, "stdout_limit"}, {&projectGitInspectionTrap{}, "other_git_or_process"}} {
		if got := projectGitDiagnosticClass(tc.err); got != tc.class {
			t.Fatal(got, tc.class)
		}
	}
	r, dir, logs := projectGitDiagnosticFixture(t)
	var original error
	r.delegate = projectGitDiagnosticDelegate(func(context.Context, sourceacquisition.Command) ([]byte, error) { return []byte("unchanged"), original })
	command := sourceacquisition.Command{Dir: dir, Args: []string{"init", "--quiet", filepath.Join(dir, "repository")}}
	if _, err := r.Run(t.Context(), command); err != nil || len(*logs) != 0 {
		t.Fatal("success emitted diagnostics")
	}
	sequence := r.recorder.failureSequence()
	r.recorder.outsideInvocations(sequence)
	if len(*logs) != 1 || !strings.Contains((*logs)[0], "remaining source phase undetermined") || !strings.Contains((*logs)[0], "init:1") {
		t.Fatal("prior command counts became phase proof")
	}
	r.recorder.counts["init"] = 65535
	original = &projectGitInspectionTrap{}
	for range 8 {
		if _, err := r.Run(t.Context(), command); err != original {
			t.Fatal("diagnostic saturation changed real error")
		}
	}
	if len(*logs) != 4 || r.recorder.counts["init"] != 65535 {
		t.Fatal("diagnostics/counts not saturated")
	}
	r.recorder.outsideInvocations(sequence)
	if len(*logs) != 4 {
		t.Fatal("unknown phase bypassed diagnostic cap")
	}
	r, _, logs = projectGitDiagnosticFixture(t)
	r.recorder.outsideInvocations(0)
	if len(*logs) != 1 || !strings.Contains((*logs)[0], "init:0") || !strings.Contains((*logs)[0], "undetermined") {
		t.Fatal("pre-command failure inferred snapshot")
	}
}

func TestProjectGitDiagnosticOperationGrammarIsClosed(t *testing.T) {
	endpoint, dir, sha := "https://127.0.0.1:12345/repo.git", filepath.Join(string(os.PathSeparator), "inert", "agentplugins-git-1"), strings.Repeat("1", 40)
	repo := filepath.Join(dir, "repository")
	cases := []struct {
		label string
		args  []string
	}{
		{"init", []string{"init", "--quiet", repo}},
		{"remote-add", []string{"remote", "add", "origin", endpoint}},
		{"config", []string{"config", "core.autocrlf", "false"}},
		{"ls-remote", []string{"ls-remote", "--symref", "--", endpoint, "HEAD"}},
		{"ls-remote", []string{"ls-remote", "--refs", "--", endpoint, "refs/heads/main", "refs/tags/v1"}},
		{"check-ref-format", []string{"check-ref-format", "refs/heads/main"}},
		{"fetch", []string{"fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", "origin", sha}},
		{"fetch", []string{"fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", "--", endpoint, sha}},
		{"fetch", []string{"fetch", "--quiet", "--depth=1", "--filter=blob:none", "--no-tags", "--no-recurse-submodules", "origin", sha}},
		{"rev-parse", []string{"rev-parse", "--verify", "FETCH_HEAD^{commit}"}},
		{"ls-tree", []string{"ls-tree", "-r", "-z", sha, "--", "."}},
		{"sparse-init", []string{"sparse-checkout", "init", "--cone"}},
		{"sparse-set", []string{"sparse-checkout", "set", "--cone", "--", "package"}},
		{"checkout", []string{"checkout", "--quiet", "--detach", sha}},
		{"show", []string{"show", sha + ":plugin.json"}},
	}
	for _, tc := range cases {
		args := tc.args
		if tc.label != "init" {
			args = append([]string{"-C", repo}, args...)
		}
		label, known, _ := projectGitDiagnosticOperation(sourceacquisition.Command{Dir: dir, Args: args}, endpoint)
		if !known || label != tc.label {
			t.Fatal(tc.label, label, known)
		}
		label, known, _ = projectGitDiagnosticOperation(sourceacquisition.Command{Dir: dir, Args: append(args, "extra")}, endpoint)
		if known || label != "unknown" {
			t.Fatal("unreviewed extra argument admitted", tc.label)
		}
	}
	for _, args := range [][]string{{"-C", repo, "config", "private", ""}, {"-C", repo, "remote", "set-url", "origin", endpoint}, {"-C", repo, "show", sha + ":../secret"}, {"-C", repo, "checkout", "--quiet", "--detach", "main"}} {
		if label, known, _ := projectGitDiagnosticOperation(sourceacquisition.Command{Dir: dir, Args: args}, endpoint); known || label != "unknown" {
			t.Fatal("unknown grammar admitted", fmt.Sprint(args))
		}
	}
}
