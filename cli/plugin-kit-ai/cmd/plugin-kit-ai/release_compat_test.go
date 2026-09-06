package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func releaseProbe(t *testing.T, args []string, want int) (report.Public, string) {
	t.Helper()
	a := commands.App{Revision: "fixture-revision", PublicContract: true, Release: &commands.ReleaseOptions{Product: "plugin-kit-ai", Version: "2.0.0", Reject: rejectV1}}
	var out, stderr bytes.Buffer
	calls := 0
	err := a.Execute(context.Background(), args, authoringcli.Streams{Out: &out, Err: &stderr}, func(fs ...authoringcli.Factory) (*cobra.Command, error) {
		root, err := newReleaseRoot(fs...)
		if err != nil {
			return nil, err
		}
		var trap func(*cobra.Command)
		trap = func(c *cobra.Command) {
			if c.RunE != nil && c.Name() != "version" {
				c.RunE = func(*cobra.Command, []string) error { calls++; return errors.New("service trap") }
			}
			for _, child := range c.Commands() {
				trap(child)
			}
		}
		trap(root)
		return root, nil
	})
	code := 0
	if err != nil {
		code = exitx.Code(err)
	}
	if code != want || calls != 0 || stderr.Len() != 0 {
		t.Fatalf("status=%d want=%d services=%d stderr=%d output=%s", code, want, calls, stderr.Len(), out.String())
	}
	if strings.Contains(out.String(), "ghp_") || strings.Contains(out.String(), "credential-fixture") {
		t.Fatal("unsafe argv projection")
	}
	var env struct {
		Schema          int `json:"schema_version"`
		Command, Result string
		Data            report.Public
	}
	if strings.HasPrefix(out.String(), "{") {
		d := json.NewDecoder(&out)
		if err := d.Decode(&env); err != nil {
			t.Fatal(err)
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			t.Fatal("multiple documents")
		}
		result := "success"
		if want != 0 {
			result = "failure"
		}
		if env.Schema != 1 || env.Result != result || env.Data.Effects.Attempted || env.Data.Effects.Committed || len(env.Data.Paths) != 0 {
			t.Fatalf("effect/envelope: %+v", env)
		}
		if env.Data.Requested.Operation != env.Command || env.Data.Conformance.Status != "not_evaluated" {
			t.Fatalf("operation/policy: %+v", env)
		}
		return env.Data, ""
	}
	return env.Data, out.String()
}

// Compare EVERY real registration, including aliases and hidden auxiliary init
// registrations, to the bounded disposition table. A new v1 flag fails here.
func TestReleaseV1InventoryCoverage(t *testing.T) {
	expected := map[string]v1Disposition{}
	for _, row := range v1Dispositions {
		expected[row.path] = row
	}
	seen := map[string]bool{}
	var walk func(*cobra.Command, string)
	walk = func(c *cobra.Command, prefix string) {
		if c != rootCmd {
			path := strings.TrimSpace(prefix + " " + c.Name())
			names := append([]string{c.Name()}, c.Aliases...)
			for _, name := range names {
				key := strings.TrimSpace(prefix + " " + name)
				row, ok := expected[key]
				if !ok {
					t.Errorf("unclassified command/alias %s", key)
					continue
				}
				seen[key] = true
				flags := map[string]string{}
				for _, spec := range strings.Fields(row.flags) {
					n, s, b := flagParts(spec)
					flags[n] = s
					if b {
						flags[n] += "!"
					}
				}
				c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
					if f.Name == "help" {
						return
					}
					got := f.Shorthand
					if f.NoOptDefVal != "" {
						got += "!"
					}
					want, ok := flags[f.Name]
					if !ok || got != want {
						t.Errorf("unclassified/changed %s --%s shorthand/arity=%q want=%q", key, f.Name, got, want)
					}
					delete(flags, f.Name)
				})
				if len(flags) > 0 {
					t.Errorf("stale flags %s: %v", key, flags)
				}
			}
			prefix = path
		}
		for _, child := range c.Commands() {
			if child.Name() == "help" || child.Name() == "completion" || strings.HasPrefix(child.Name(), "__complete") {
				continue
			}
			walk(child, prefix)
		}
	}
	walk(rootCmd, "")
	for name := range expected {
		if !seen[name] {
			t.Errorf("stale disposition %s", name)
		}
	}
}

func TestReleaseEveryRemovedVerbAliasFlag(t *testing.T) {
	for _, row := range v1Dispositions {
		if row.retained {
			continue
		}
		t.Run(row.path, func(t *testing.T) {
			base := strings.Fields(row.path)
			variants := [][]string{nil, {"--help"}, {"-h"}, {"arbitrary", "--unknown=credential-fixture"}, {"--unknown=credential-fixture", "--help"}, {"--", "--format=human", "credential-fixture"}}
			for _, spec := range strings.Fields(row.flags) {
				name, short, boolean := flagParts(spec)
				value := "credential-fixture"
				if boolean {
					value = "false"
				}
				variants = append(variants, []string{"--" + name + "=" + value}, []string{"--" + name + "=" + value, "--help"})
				if !boolean {
					variants = append(variants, []string{"--" + name, value})
				} else {
					variants = append(variants, []string{"--" + name})
				}
				if short != "" {
					variants = append(variants, []string{"-" + short + "=" + value}, []string{"-h" + short + "=" + value})
				}
			}
			for _, tail := range variants {
				args := append([]string{"--format=json"}, base...)
				args = append(args, tail...)
				p, human := releaseProbe(t, args, 2)
				if !strings.HasPrefix(row.path, "__docs") && !strings.Contains(human, legacyGuidance(row.path)) && (p.Error == nil || !strings.Contains(p.Error.Action, legacyGuidance(row.path))) {
					t.Fatalf("missing exact guidance for %s: %+v", row.path, p.Error)
				}
			}
		})
	}
}
func TestReleaseSupportedNameLegacyFlags(t *testing.T) {
	retained := map[string]string{"init": "template runtime", "validate": "format", "inspect": "target format", "compat": "target format", "test": "format", "capabilities": "format", "skills init": "description"}
	for _, row := range v1Dispositions {
		if !row.retained {
			continue
		}
		for _, spec := range strings.Fields(row.flags) {
			name, short, boolean := flagParts(spec)
			if strings.Contains(" "+retained[row.path]+" ", " "+name+" ") {
				continue
			}
			value := "credential-fixture"
			if boolean {
				value = "false"
			}
			variants := [][]string{{"--" + name + "=" + value}, {"--unknown=credential-fixture", "--" + name + "=" + value, "--help"}}
			if short != "" {
				variants = append(variants, []string{"-h" + short + "=" + value})
			}
			for _, tail := range variants {
				args := append(strings.Fields(row.path), "--format=json")
				releaseProbe(t, append(args, tail...), 2)
			}
		}
	}
	for _, args := range [][]string{
		{"init", "--template=online-service"}, {"init", "--template=local-tool"}, {"init", "--template=custom-logic"},
		{"init", "--runtime=go"}, {"init", "--runtime=python"}, {"init", "--runtime=shell"}, {"init", "--runtime=node", "--template=skill"},
		{"inspect", "--target=all"}, {"compat", "--target=codex-package"}, {"inspect", "--target=codex-runtime"}, {"compat", "--target=cursor-workspace"},
		{"validate", "--format=text"}, {"capabilities", "--format=table"},
	} {
		releaseProbe(t, append(args, "--help"), 2)
	}
}
func TestReleaseJSONSelectionAndClosure(t *testing.T) {
	for _, args := range [][]string{
		{"skills", "list", "--json"}, {"skills", "ls", "--json=true"},
		{"init", "--force=false", "--format=human", "--format", "json"},
		{"init", "--unknown=credential-fixture", "-hf=false", "--format=json"},
		{"init", "-zhf=false", "--format=json"},
		{"init", "--format=json", "--output"},
		{"skills", "install", "-gly", "--format=json"},
		{"bundle", "fetch", "--github-token", "--format=human", "--format=json"},
	} {
		p, _ := releaseProbe(t, args, 2)
		if p.Error == nil {
			t.Fatal("JSON not selected")
		}
	}
	for _, args := range [][]string{
		{"skills", "list", "--json=false"}, {"skills", "list", "--", "--json"},
		{"init", "--force", "--format=json", "--format=human"},
		{"bundle", "fetch", "--github-token", "--format=json"},
	} {
		_, human := releaseProbe(t, args, 2)
		if human == "" {
			t.Fatal("flag value/delimiter became JSON")
		}
	}
	for _, args := range [][]string{nil, {"--help"}, {"help", "skills", "init"}, {"version"}, {"skills"}} {
		releaseProbe(t, append(args, "--format=json"), 0)
	}
	for _, args := range [][]string{{"credential-fixture"}, {"init", "--unknown=credential-fixture"}, {"skills", "init", "--description"}, {"--", "init"}} {
		releaseProbe(t, append(args, "--format=json"), 2)
	}
}

func TestReleaseCompletionUsesVisibleTrees(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		_, out := releaseProbe(t, []string{"completion", shell}, 0)
		for _, forbidden := range []string{"bootstrap", "integrations", "publication", "__docs", "skills-cli-version", "runtime-package"} {
			if strings.Contains(out, forbidden) {
				t.Fatalf("%s leaked %s", shell, forbidden)
			}
		}
		if !strings.Contains(out, "plugin-kit-ai") {
			t.Fatal("missing executable")
		}
	}
	p, _ := releaseProbe(t, []string{"--help", "--format=json"}, 0)
	if len(p.Surface) != 10 || p.Help.Use != "plugin-kit-ai <command>" {
		t.Fatal(p)
	}
	for _, args := range [][]string{{"help", "bundle"}, {"help", "skills", "add"}, {"bundle", "inspect", "--help"}} {
		releaseProbe(t, append(args, "--format=json"), 2)
	}
}

func TestReleaseActualLegacyFlagDefaults(t *testing.T) {
	for _, row := range v1Dispositions {
		c, _, err := rootCmd.Find(strings.Fields(row.path))
		if err != nil {
			t.Fatal(err)
		}
		c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" {
				return
			}
			if row.retained {
				// These have a retained standard meaning; their values are validated by
				// the shared factory. The other legacy defaults must still reject.
				switch f.Name {
				case "format", "target", "description":
					return
				}
			}
			args := append(strings.Fields(row.path), "--"+f.Name+"="+f.DefValue, "--help", "--format=json")
			releaseProbe(t, args, 2)
		})
	}
}
func TestReleaseManagerGuidancePreservesIntent(t *testing.T) {
	for _, tc := range []struct {
		args         []string
		want, absent string
	}{
		{[]string{"integrations", "add"}, "adding --dry-run", ""},
		{[]string{"add"}, "agentplugins add <name-or-source>", "adding --dry-run"},
		{[]string{"integrations", "add", "--dry-run=false"}, "agentplugins add <name-or-source>", "adding --dry-run"},
		{[]string{"add", "--auto-update=false"}, legacyGuidance("add"), "agentplugins add <name-or-source>"},
		{[]string{"add", "--scope=project"}, legacyGuidance("add"), "agentplugins add <name-or-source>"},
		{[]string{"repair", "--target=codex-runtime"}, legacyGuidance("repair"), "agentplugins repair"},
		{[]string{"update"}, "omitted name does not imply --all", "Use agentplugins update --all"},
		{[]string{"update", "--all=true", "--dry-run=true"}, "Use agentplugins update --all. Preserve plan intent by adding --dry-run.", ""},
	} {
		p, _ := releaseProbe(t, append(tc.args, "--format=json"), 2)
		if p.Error == nil || !strings.Contains(p.Error.Action, tc.want) || tc.absent != "" && strings.Contains(p.Error.Action, tc.absent) {
			t.Fatal(tc.args, p.Error)
		}
	}
}

func TestReleaseCompletionHelpAncestry(t *testing.T) {
	p, _ := releaseProbe(t, []string{"completion", "bash", "--help", "--format=json"}, 0)
	if p.Help == nil || p.Help.Use != "plugin-kit-ai completion bash" {
		t.Fatal(p.Help)
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		releaseProbe(t, []string{"completion", shell, "--no-descriptions"}, 0)
	}
}
