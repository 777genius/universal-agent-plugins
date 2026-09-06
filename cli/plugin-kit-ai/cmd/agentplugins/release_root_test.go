package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
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
