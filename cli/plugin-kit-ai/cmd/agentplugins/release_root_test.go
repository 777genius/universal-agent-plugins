package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

func TestReleaseRoutesBeforeInstallerSetup(t *testing.T) {
	for _, args := range [][]string{
		nil, {"--help"}, {"help", "author"}, {"help", "author", "skills", "init"},
		{"author"}, {"author", "--help"}, {"author", "version"}, {"version"},
		{"author", "skills", "--help"}, {"author", "inspect", "--help"},
		{"--scope=user", "author", "--help"}, {"author", "--accept-security-risk=false", "--help"},
		{"author", "init", "--unknown=credential-fixture"},
		{"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}, {"completion", "powershell"},
		{"help", "add"}, {"add", "--help"}, {"author", "skills", "--security-details=false"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errout bytes.Buffer
			calls := 0
			err := executeRelease(context.Background(), args, authoringcli.Streams{Out: &out, Err: &errout}, func() error { calls++; return errors.New("installer trap") })
			if calls != 0 || errout.Len() != 0 || strings.Contains(out.String(), "credential-fixture") {
				t.Fatalf("route reached setup or leaked input: calls=%d", calls)
			}
			if len(args) == 0 && (err != nil || !strings.Contains(out.String(), "author")) {
				t.Fatal("author missing from root")
			}
			if strings.HasPrefix(strings.Join(args, " "), "completion") && (err != nil || out.Len() == 0) {
				t.Fatal("completion failed", err)
			}
		})
	}
	for _, args := range [][]string{{"add", "author"}, {"--target", "author", "add", "source"}, {"--target", "cursor", "add", "author"}, {"add", "--", "author"}, {"validate", "author"}} {
		calls := 0
		_ = executeRelease(context.Background(), args, authoringcli.Streams{Out: io.Discard, Err: io.Discard}, func() error { calls++; return nil })
		if calls != 1 {
			t.Fatalf("installer placement/source changed: %v", args)
		}
	}
}

func TestReleaseVersionAndHelpJSON(t *testing.T) {
	old := version
	version = "0.1.91"
	defer func() { version = old }()
	for _, args := range [][]string{{"version"}, {"author", "version"}, {"--help"}, {"help", "author", "skills", "init"}, {"completion", "bash"}} {
		var out bytes.Buffer
		err := executeRelease(context.Background(), append(args, "--format=json"), authoringcli.Streams{Out: &out, Err: io.Discard}, func() error { t.Fatal("installer initialized"); return nil })
		if err != nil {
			t.Fatal(err, out.String())
		}
		var e struct {
			Command, Result string
			Data            map[string]any
		}
		if err = json.Unmarshal(out.Bytes(), &e); err != nil {
			t.Fatal(err, out.String())
		}
		if e.Result != "success" {
			t.Fatal(e)
		}
		switch strings.Join(args, " ") {
		case "completion bash":
			if e.Command != "completion" || !strings.Contains(e.Data["script"].(string), "agentplugins") {
				t.Fatal(e)
			}
		case "version":
			if e.Command != "version" || e.Data["version"] != version {
				t.Fatal(e)
			}
		case "author version":
			if e.Command != "author.version" || e.Data["product_version"] != version || e.Data["engine_version"] != "standard-first-slice/1" {
				t.Fatal(e)
			}
		case "help author skills init":
			if !strings.HasPrefix(e.Data["help"].(map[string]any)["use"].(string), "agentplugins author skills init ") {
				t.Fatal(e)
			}
		}
	}
}
func TestReleaseInstallerFlagsRejectBeforeHelp(t *testing.T) {
	for _, f := range []string{"--scope=user", "--security-details=false", "--accept-security-risk=false", "--dry-run=false"} {
		for _, args := range [][]string{{f, "author", "init", "--help"}, {"author", "init", f, "--help"}} {
			var out bytes.Buffer
			err := executeRelease(context.Background(), append(args, "--format=json"), authoringcli.Streams{Out: &out, Err: io.Discard}, func() error { t.Fatal("installer initialized"); return nil })
			if exitx.Code(err) != 2 || !strings.Contains(out.String(), `"attempted":false`) || !strings.Contains(out.String(), `"result":"failure"`) {
				t.Fatal(err, out.String())
			}
		}
	}
}

func TestReleaseInstallerAliasAndFlagPlacement(t *testing.T) {
	for _, args := range [][]string{
		{"install", "fixture", "--target=cursor"}, {"--target=cursor", "install", "fixture"},
		{"install", "author"}, {"--target=author", "install", "fixture"}, {"install", "--", "author"},
		{"update", "--all"}, {"--all", "update"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			capture := func() (string, error) {
				root := agentpluginscli.NewRoot(agentpluginscli.App{})
				selected := ""
				for _, c := range root.Commands() {
					c.RunE = func(cmd *cobra.Command, _ []string) error { selected = cmd.Name(); return nil }
				}
				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				root.SetArgs(args)
				err := root.Execute()
				return selected, err
			}
			want, baselineErr := capture()
			calls, got := 0, ""
			err := executeRelease(context.Background(), args, authoringcli.Streams{Out: io.Discard, Err: io.Discard}, func() error {
				calls++
				var err error
				got, err = capture()
				return err
			})
			if (err == nil) != (baselineErr == nil) || got != want || baselineErr == nil && calls != 1 {
				t.Fatalf("Cobra dispatch changed: baseline=%q/%v release=%q/%v callbacks=%d", want, baselineErr, got, err, calls)
			}
		})
	}
}

func TestReleaseInstallerErrorRendering(t *testing.T) {
	var out, stderr bytes.Buffer
	failure := errors.New("captured installer failure")
	err := executeRelease(context.Background(), []string{"list"}, authoringcli.Streams{Out: &out, Err: &stderr}, func() error { return failure })
	if exitx.Code(err) != 1 || !errors.Is(err, failure) || out.Len() != 0 || stderr.String() != "agentplugins: captured installer failure\n" {
		t.Fatalf("installer error boundary changed: %v, %q, %q", err, out.String(), stderr.String())
	}
	for _, args := range [][]string{{"author", "--unknown=credential-fixture"}, {"--unknown=credential-fixture"}} {
		out.Reset()
		stderr.Reset()
		err = executeRelease(context.Background(), append(args, "--format=json"), authoringcli.Streams{Out: &out, Err: &stderr}, func() error { t.Fatal("installer callback"); return failure })
		if exitx.Code(err) != 2 || stderr.Len() != 0 || strings.Contains(out.String(), "credential-fixture") || strings.Count(out.String(), `"schema_version"`) != 1 {
			t.Fatal("duplicated or disclosed author/utility failure")
		}
	}
}

// A child process is essential: Cobra CompErrorln bypasses Command.SetErr.
func TestReleaseCompletionProcessStderr(t *testing.T) {
	if os.Getenv("UAP_COMPLETION_PROCESS_FIXTURE") == "1" {
		args := os.Args
		for i, arg := range args {
			if arg == "--" {
				args = args[i+1:]
				break
			}
		}
		streams := authoringcli.Streams{Out: os.Stdout, Err: os.Stderr}
		err := executeRelease(context.Background(), args, streams, func() error { panic("installer callback in completion") })
		if err != nil {
			os.Exit(exitx.Code(err))
		}
		os.Exit(0)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	marker := "glpat-" + strings.Repeat("b", 20)
	for _, protocol := range []string{"__complete", "__completeNoDesc"} {
		cases := []struct {
			args  []string
			want  string
			fails bool
		}{
			{[]string{protocol, ""}, "author", false},
			{append(append([]string{protocol}, []string{"author", "init"}...), "--desc"), "--description", false},
			{append(append([]string{protocol}, []string{"author", "init"}...), "--description", ""), ":", false},
			// Empty flag names fall back to the preceding value flag in Cobra.
			{[]string{protocol, "author", "init", "--description", "--=x"}, ":0\n", false},
			{[]string{protocol, "author", "skills", "init", "--description", "--=x"}, ":0\n", false},
			{[]string{protocol, "author", "skills", "init", "--format", "--=x"}, ":0\n", false},
			{[]string{protocol, "--format", "--=x"}, ":0\n", false},
			// Ordinary partial flag names must still leave the missing value invalid.
			{[]string{protocol, "author", "init", "--description", "--desc"}, "", true},
			{[]string{protocol, "author", "init", "--description", "-"}, "", true},
			{[]string{protocol, "author", "init", "--description", "--"}, "", true},
			{[]string{protocol, "--unknown=" + marker, "--=x"}, "", true},
			{[]string{protocol, "--unknown=" + marker, ""}, "", true},
			{[]string{protocol, "--unknown=" + marker}, "", true},
			{[]string{protocol, "--no-color=" + marker, ""}, "", true},
			{[]string{protocol, "-" + marker, ""}, "", true},
			{[]string{protocol, marker, ""}, "", true},
			{[]string{protocol, "--format", marker, "--unknown", ""}, "", true},
			{[]string{protocol, "--unknown=" + marker, "", "--format=json"}, "", true},
			{[]string{protocol, "--format=json", "--unknown=" + marker, ""}, "", true},
			{[]string{"--format=json", protocol, "--unknown=" + marker, ""}, "", true},
		}
		for i, tc := range cases {
			child := exec.Command(exe, append([]string{"-test.run=^TestReleaseCompletionProcessStderr$", "--"}, tc.args...)...)
			child.Dir = home
			child.Env = []string{"UAP_COMPLETION_PROCESS_FIXTURE=1", "HOME=" + home, "TMPDIR=" + home, "XDG_CONFIG_HOME=" + home, "PATH=" + home, "BASH_COMP_DEBUG_FILE=" + home + "/completion-debug", "GOMAXPROCS=2"}
			var stdout, stderr bytes.Buffer
			child.Stdout, child.Stderr = &stdout, &stderr
			err := child.Run()
			if _, statErr := os.Stat(home + "/completion-debug"); !os.IsNotExist(statErr) {
				t.Fatal("completion wrote a process-global debug file")
			}
			if tc.want == ":0\n" && stdout.String() != tc.want {
				t.Fatalf("protocol %s case %d: want exact completion %q, got %q", protocol, i, tc.want, stdout.String())
			}
			if (err != nil) != tc.fails || stderr.Len() != 0 || strings.Contains(stdout.String(), marker) || !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("protocol %s case %d: exit=%v stderr bytes=%d; completion/containment failed", protocol, i, err, stderr.Len())
			}
		}
	}
}
