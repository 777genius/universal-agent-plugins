package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	skillsapp "github.com/777genius/plugin-kit-ai/cli/internal/skills/app"
	"github.com/spf13/cobra"
)

type fakeSkillsRunner struct {
	installOpts app.ExternalSkillsInstallOptions
	listOpts    app.ExternalSkillsListOptions
	updateOpts  app.ExternalSkillsUpdateOptions
	removeOpts  app.ExternalSkillsRemoveOptions
	called      string
}

func (f *fakeSkillsRunner) Init(app.SkillsInitOptions) (string, error) {
	return "", nil
}

func (f *fakeSkillsRunner) Validate(app.SkillsValidateOptions) (skillsapp.ValidationReport, error) {
	return skillsapp.ValidationReport{}, nil
}

func (f *fakeSkillsRunner) Generate(app.SkillsGenerateOptions) ([]string, error) {
	return nil, nil
}

func (f *fakeSkillsRunner) InstallExternal(ctx context.Context, opts app.ExternalSkillsInstallOptions) error {
	f.called = "install"
	f.installOpts = opts
	return nil
}

func (f *fakeSkillsRunner) ListExternal(ctx context.Context, opts app.ExternalSkillsListOptions) error {
	f.called = "list"
	f.listOpts = opts
	return nil
}

func (f *fakeSkillsRunner) UpdateExternal(ctx context.Context, opts app.ExternalSkillsUpdateOptions) error {
	f.called = "update"
	f.updateOpts = opts
	return nil
}

func (f *fakeSkillsRunner) RemoveExternal(ctx context.Context, opts app.ExternalSkillsRemoveOptions) error {
	f.called = "remove"
	f.removeOpts = opts
	return nil
}

func TestSkillsExternalInstallForwardsAllFlags(t *testing.T) {
	t.Parallel()
	runner := &fakeSkillsRunner{}
	cmd := newSkillsCmd(runner)
	executeSkillsCommand(t, cmd, []string{
		"install", "flutter/skills",
		"--skills-cli-version", "2.0.0",
		"--global",
		"--agent", "codex",
		"--agent", "claude-code",
		"--skill", "flutter-use-http-package",
		"--skill", "*",
		"--list",
		"--yes",
		"--copy",
		"--all",
		"--full-depth",
	})
	if runner.called != "install" {
		t.Fatalf("called = %q", runner.called)
	}
	opts := runner.installOpts
	if opts.Source != "flutter/skills" || opts.Version != "2.0.0" {
		t.Fatalf("source/version = %q/%q", opts.Source, opts.Version)
	}
	if !opts.Global || !opts.List || !opts.Yes || !opts.Copy || !opts.All || !opts.FullDepth {
		t.Fatalf("bool flags = %+v", opts)
	}
	if !reflect.DeepEqual(opts.Agents, []string{"codex", "claude-code"}) {
		t.Fatalf("agents = %#v", opts.Agents)
	}
	if !reflect.DeepEqual(opts.Skills, []string{"flutter-use-http-package", "*"}) {
		t.Fatalf("skills = %#v", opts.Skills)
	}
	if opts.Stdout == nil || opts.Stderr == nil {
		t.Fatalf("expected stdout/stderr to be wired")
	}
}

func TestSkillsExternalInstallAliasUsesPinnedDefaultVersion(t *testing.T) {
	t.Parallel()
	runner := &fakeSkillsRunner{}
	cmd := newSkillsCmd(runner)
	executeSkillsCommand(t, cmd, []string{"add", "dart-lang/skills"})
	if runner.called != "install" {
		t.Fatalf("called = %q", runner.called)
	}
	if runner.installOpts.Version != app.DefaultExternalSkillsCLIVersion {
		t.Fatalf("version = %q", runner.installOpts.Version)
	}
}

func TestSkillsExternalListForwardsFlags(t *testing.T) {
	t.Parallel()
	runner := &fakeSkillsRunner{}
	cmd := newSkillsCmd(runner)
	executeSkillsCommand(t, cmd, []string{
		"ls",
		"--skills-cli-version", "latest",
		"--global",
		"--agent", "codex",
		"--agent", "gemini-cli",
		"--json",
	})
	if runner.called != "list" {
		t.Fatalf("called = %q", runner.called)
	}
	opts := runner.listOpts
	if opts.Version != "latest" || !opts.Global || !opts.JSON {
		t.Fatalf("opts = %+v", opts)
	}
	if !reflect.DeepEqual(opts.Agents, []string{"codex", "gemini-cli"}) {
		t.Fatalf("agents = %#v", opts.Agents)
	}
}

func TestSkillsExternalUpdateForwardsFlags(t *testing.T) {
	t.Parallel()
	runner := &fakeSkillsRunner{}
	cmd := newSkillsCmd(runner)
	executeSkillsCommand(t, cmd, []string{
		"upgrade",
		"dart-run-static-analysis",
		"flutter-fix-layout-issues",
		"--global",
		"--project",
		"--yes",
	})
	if runner.called != "update" {
		t.Fatalf("called = %q", runner.called)
	}
	opts := runner.updateOpts
	if !opts.Global || !opts.Project || !opts.Yes {
		t.Fatalf("opts = %+v", opts)
	}
	if !reflect.DeepEqual(opts.Skills, []string{"dart-run-static-analysis", "flutter-fix-layout-issues"}) {
		t.Fatalf("skills = %#v", opts.Skills)
	}
}

func TestSkillsExternalRemoveForwardsFlags(t *testing.T) {
	t.Parallel()
	runner := &fakeSkillsRunner{}
	cmd := newSkillsCmd(runner)
	executeSkillsCommand(t, cmd, []string{
		"rm",
		"dart-add-unit-test",
		"--global",
		"--agent", "claude-code",
		"--skill", "*",
		"--yes",
		"--all",
	})
	if runner.called != "remove" {
		t.Fatalf("called = %q", runner.called)
	}
	opts := runner.removeOpts
	if !opts.Global || !opts.Yes || !opts.All {
		t.Fatalf("opts = %+v", opts)
	}
	if !reflect.DeepEqual(opts.Skills, []string{"dart-add-unit-test"}) {
		t.Fatalf("positional skills = %#v", opts.Skills)
	}
	if !reflect.DeepEqual(opts.Agents, []string{"claude-code"}) {
		t.Fatalf("agents = %#v", opts.Agents)
	}
	if !reflect.DeepEqual(opts.Filter, []string{"*"}) {
		t.Fatalf("filter = %#v", opts.Filter)
	}
}

func executeSkillsCommand(t *testing.T, cmd *cobra.Command, args []string) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v): %v\n%s", args, err, buf.String())
	}
}
