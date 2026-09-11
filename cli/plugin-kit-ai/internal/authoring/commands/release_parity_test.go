package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

func releaseExecute(t *testing.T, args []string, mount bool) ([]byte, int) {
	t.Helper()
	a := publicApp(t)
	product, version := "plugin-kit-ai", "2.0.0"
	build := commands.RootBuilder(authoringcli.NewReleasePluginKitRoot)
	if mount {
		product, version = "agentplugins", "0.1.91"
		args = append([]string{"author"}, args...)
		build = func(fs ...authoringcli.Factory) (*cobra.Command, error) {
			root := agentpluginscli.NewRoot(agentpluginscli.App{Version: version})
			c, err := authoringcli.NewReleaseAuthorCommand(fs...)
			if err != nil {
				return nil, err
			}
			root.AddCommand(c)
			return root, nil
		}
	}
	a.Release = &commands.ReleaseOptions{Product: product, Version: version}
	var out, errout bytes.Buffer
	err := a.Execute(context.Background(), args, authoringcli.Streams{Out: &out, Err: &errout}, build)
	if errout.Len() > 0 {
		t.Fatal("unexpected stderr")
	}
	code := 0
	if err != nil {
		code = exitx.Code(err)
	}
	return out.Bytes(), code
}
func TestReleaseVersionParityAndFreshHelp(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--help"}, {"help", "skills", "init"}, {"skills"}, {"version", "--help"}, {"capabilities"}} {
		var pair [2]map[string]any
		for i := range pair {
			raw, code := releaseExecute(t, append(args, "--format=json"), i == 1)
			if code != 0 {
				t.Fatal(code, string(raw))
			}
			d := json.NewDecoder(bytes.NewReader(raw))
			if err := d.Decode(&pair[i]); err != nil {
				t.Fatal(err)
			}
			var extra any
			if d.Decode(&extra) != io.EOF {
				t.Fatal("multiple documents")
			}
			data := pair[i]["data"].(map[string]any)
			if data["engine_version"] != "standard-first-slice/1" || data["revision"] != publicRevision {
				t.Fatal(data)
			}
			if product, ok := data["product"]; ok {
				wantProduct, wantVersion := "plugin-kit-ai", "2.0.0"
				if i == 1 {
					wantProduct, wantVersion = "agentplugins", "0.1.91"
				}
				if product != wantProduct || data["product_version"] != wantVersion || pair[i]["command"] != "author.version" {
					t.Fatal(data)
				}
				delete(data, "product")
				delete(data, "product_version")
			}
			if help, ok := data["help"].(map[string]any); ok {
				prefix := "plugin-kit-ai"
				if i == 1 {
					prefix = "agentplugins author"
				}
				if !strings.HasPrefix(help["use"].(string), prefix) {
					t.Fatal(help)
				}
				help["use"] = strings.Replace(help["use"].(string), prefix, "<author>", 1)
			}
		}
		a, _ := json.Marshal(pair[0])
		b, _ := json.Marshal(pair[1])
		if !bytes.Equal(a, b) {
			t.Fatalf("parity:\n%s\n%s", a, b)
		}
	}
}
func TestReleaseParserAndProtocolClosure(t *testing.T) {
	for _, args := range [][]string{
		{"version", "--target=cursor"}, {"version", "--unknown=credential-fixture"}, {"version", "extra"},
		{"init", "--name=inspect", "--template=skill", "--unknown=credential-fixture"},
		{"skills", "init", "--description=version"}, {"--format=json", "--", "version"},
		{"completion", "bash", "--unknown=credential-fixture"},
	} {
		for _, mount := range []bool{false, true} {
			raw, code := releaseExecute(t, append(args, "--format=json"), mount)
			if code != 2 || strings.Contains(string(raw), "credential-fixture") {
				t.Fatal(code, string(raw))
			}
		}
	}
	for _, args := range [][]string{{"__complete", ""}, {"__completeNoDesc", "skills", ""}} {
		raw, code := releaseExecute(t, args, false)
		if code != 0 || !strings.Contains(string(raw), ":") {
			t.Fatal(code, string(raw))
		}
	}
}
func TestReleaseBuildModeClosed(t *testing.T) {
	old := commands.Enabled
	defer func() { commands.Enabled = old }()
	for _, value := range []string{"", "vertical-slice-v1", commands.ReleaseMode, "true", "enabled"} {
		commands.Enabled = value
		if commands.IsRelease() != (value == commands.ReleaseMode) || commands.IsEnabled() != (value == commands.ReleaseMode || value == "vertical-slice-v1") {
			t.Fatal(value)
		}
	}
}
