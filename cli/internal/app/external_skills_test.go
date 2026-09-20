package app

import (
	"context"
	"reflect"
	"testing"
)

type fakeExternalSkillsRunner struct {
	invocations []ExternalSkillsInvocation
	err         error
}

func (f *fakeExternalSkillsRunner) Run(ctx context.Context, invocation ExternalSkillsInvocation) error {
	f.invocations = append(f.invocations, invocation)
	return f.err
}

func TestExternalSkillsInstallBuildsNpxInvocationWithAllOptions(t *testing.T) {
	t.Parallel()
	runner := &fakeExternalSkillsRunner{}
	svc := SkillsService{ExternalRunner: runner}
	err := svc.InstallExternal(context.Background(), ExternalSkillsInstallOptions{
		Source:    "flutter/skills",
		Version:   "2.0.0",
		Global:    true,
		Agents:    []string{"codex", "claude-code"},
		Skills:    []string{"flutter-use-http-package", "*"},
		List:      true,
		Yes:       true,
		Copy:      true,
		All:       true,
		FullDepth: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertExternalSkillsInvocation(t, runner, []string{
		"-y", "skills@2.0.0", "add", "flutter/skills",
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
}

func TestExternalSkillsInstallDefaultsToPinnedVersion(t *testing.T) {
	t.Parallel()
	runner := &fakeExternalSkillsRunner{}
	svc := SkillsService{ExternalRunner: runner}
	if err := svc.InstallExternal(context.Background(), ExternalSkillsInstallOptions{Source: "dart-lang/skills"}); err != nil {
		t.Fatal(err)
	}
	assertExternalSkillsInvocation(t, runner, []string{"-y", "skills@1.5.5", "add", "dart-lang/skills"})
}

func TestExternalSkillsInstallLeavesAgentValidationToUpstream(t *testing.T) {
	t.Parallel()
	runner := &fakeExternalSkillsRunner{}
	svc := SkillsService{ExternalRunner: runner}
	if err := svc.InstallExternal(context.Background(), ExternalSkillsInstallOptions{
		Source: "dart-lang/skills",
		Agents: []string{"not-a-real-agent"},
	}); err != nil {
		t.Fatal(err)
	}
	assertExternalSkillsInvocation(t, runner, []string{
		"-y", "skills@1.5.5", "add", "dart-lang/skills",
		"--agent", "not-a-real-agent",
	})
}

func TestExternalSkillsInstallRejectsEmptySource(t *testing.T) {
	t.Parallel()
	runner := &fakeExternalSkillsRunner{}
	svc := SkillsService{ExternalRunner: runner}
	if err := svc.InstallExternal(context.Background(), ExternalSkillsInstallOptions{Source: "   "}); err == nil {
		t.Fatal("expected error")
	}
	if len(runner.invocations) != 0 {
		t.Fatalf("invocations = %d", len(runner.invocations))
	}
}

func TestExternalSkillsListBuildsNpxInvocation(t *testing.T) {
	t.Parallel()
	runner := &fakeExternalSkillsRunner{}
	svc := SkillsService{ExternalRunner: runner}
	if err := svc.ListExternal(context.Background(), ExternalSkillsListOptions{
		Version: "skills@latest",
		Global:  true,
		Agents:  []string{"codex", "gemini-cli"},
		JSON:    true,
	}); err != nil {
		t.Fatal(err)
	}
	assertExternalSkillsInvocation(t, runner, []string{
		"-y", "skills@latest", "list",
		"--agent", "codex",
		"--agent", "gemini-cli",
		"--global",
		"--json",
	})
}

func TestExternalSkillsUpdateBuildsNpxInvocation(t *testing.T) {
	t.Parallel()
	runner := &fakeExternalSkillsRunner{}
	svc := SkillsService{ExternalRunner: runner}
	if err := svc.UpdateExternal(context.Background(), ExternalSkillsUpdateOptions{
		Skills:  []string{"dart-run-static-analysis", "flutter-fix-layout-issues"},
		Global:  true,
		Project: true,
		Yes:     true,
	}); err != nil {
		t.Fatal(err)
	}
	assertExternalSkillsInvocation(t, runner, []string{
		"-y", "skills@1.5.5", "update",
		"dart-run-static-analysis",
		"flutter-fix-layout-issues",
		"--global",
		"--project",
		"--yes",
	})
}

func TestExternalSkillsRemoveBuildsNpxInvocation(t *testing.T) {
	t.Parallel()
	runner := &fakeExternalSkillsRunner{}
	svc := SkillsService{ExternalRunner: runner}
	if err := svc.RemoveExternal(context.Background(), ExternalSkillsRemoveOptions{
		Skills: []string{"dart-add-unit-test"},
		Global: true,
		Agents: []string{"claude-code"},
		Filter: []string{"*"},
		Yes:    true,
		All:    true,
	}); err != nil {
		t.Fatal(err)
	}
	assertExternalSkillsInvocation(t, runner, []string{
		"-y", "skills@1.5.5", "remove",
		"dart-add-unit-test",
		"--agent", "claude-code",
		"--skill", "*",
		"--global",
		"--yes",
		"--all",
	})
}

func assertExternalSkillsInvocation(t *testing.T, runner *fakeExternalSkillsRunner, wantArgs []string) {
	t.Helper()
	if len(runner.invocations) != 1 {
		t.Fatalf("invocations = %d", len(runner.invocations))
	}
	got := runner.invocations[0]
	if got.Command != "npx" {
		t.Fatalf("command = %q", got.Command)
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("args = %#v\nwant %#v", got.Args, wantArgs)
	}
}
