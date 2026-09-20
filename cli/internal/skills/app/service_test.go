package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
)

func TestServiceInitGoCommand(t *testing.T) {
	t.Parallel()
	svc := Service{}
	root := t.TempDir()
	out, err := svc.Init(InitOptions{
		Name:      "lint-repo",
		Template:  filesystem.TemplateGoCommand,
		OutputDir: root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, filepath.Join("skills", "lint-repo")) {
		t.Fatalf("out = %q", out)
	}
	for _, rel := range []string{
		filepath.Join("skills", "lint-repo", "SKILL.md"),
		filepath.Join("cmd", "lint-repo", "main.go"),
		filepath.Join("cmd", "lint-repo", "main_test.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestServiceValidateAndRender(t *testing.T) {
	t.Parallel()
	svc := Service{}
	root := t.TempDir()
	if _, err := svc.Init(InitOptions{
		Name:      "lint-repo",
		Template:  filesystem.TemplateGoCommand,
		OutputDir: root,
	}); err != nil {
		t.Fatal(err)
	}
	report, err := svc.Validate(ValidateOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	result, err := svc.Generate(RenderOptions{Root: root, Target: "all"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := result.Artifacts
	if len(artifacts) == 0 {
		t.Fatal("expected artifacts")
	}
	if err := svc.WriteArtifacts(root, artifacts); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		filepath.Join("generated", "skills", "claude", "lint-repo", "SKILL.md"),
		filepath.Join("generated", "skills", "codex", "lint-repo", "SKILL.md"),
		filepath.Join("commands", "lint-repo.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}

func TestServiceRenderDocsOnlySkipsCommandDoc(t *testing.T) {
	t.Parallel()
	svc := Service{}
	root := t.TempDir()
	if _, err := svc.Init(InitOptions{
		Name:      "playbook",
		Template:  filesystem.TemplateDocsOnly,
		OutputDir: root,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := svc.Generate(RenderOptions{Root: root, Target: "all"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := result.Artifacts
	for _, artifact := range artifacts {
		if artifact.RelPath == filepath.Join("commands", "playbook.md") {
			t.Fatalf("unexpected command doc for docs-only skill: %s", artifact.RelPath)
		}
	}
}

func TestServiceRenderRespectsSupportedAgents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "claude-only"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: claude-only
description: claude only skill
execution_mode: docs_only
supported_agents:
  - claude
---

# Claude Only

## What it does

x

## When to use

y

## How to run

z

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "claude-only", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{}
	result, err := svc.Generate(RenderOptions{Root: root, Target: "all"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := result.Artifacts
	for _, artifact := range artifacts {
		if strings.Contains(artifact.RelPath, filepath.Join("generated", "skills", "codex")) {
			t.Fatalf("unexpected codex artifact for claude-only skill: %s", artifact.RelPath)
		}
	}
}

func TestServiceGenerateRejectsUnknownTarget(t *testing.T) {
	t.Parallel()
	svc := Service{}
	_, err := svc.Generate(RenderOptions{Root: t.TempDir(), Target: "typo"})
	if err == nil || !strings.Contains(err.Error(), `unknown generate target "typo"`) {
		t.Fatalf("Generate unknown target error = %v", err)
	}
}

func TestServiceRenderCommandDocQuotesArgs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "quoted"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: quoted
description: quoted args skill
execution_mode: command
supported_agents:
  - claude
command: tool
args:
  - --message
  - hello world
runtime: external
---

# Quoted

## What it does

x

## When to use

y

## How to run

Run the command.

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "quoted", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{}
	result, err := svc.Generate(RenderOptions{Root: root, Target: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := result.Artifacts
	var commandDoc string
	for _, artifact := range artifacts {
		if artifact.RelPath == filepath.Join("commands", "quoted.md") {
			commandDoc = string(artifact.Content)
		}
	}
	if !strings.Contains(commandDoc, "`tool --message 'hello world'`") {
		t.Fatalf("unexpected command doc:\n%s", commandDoc)
	}
}

func TestServiceRenderCommandDocOmitsUnsetExecutionNotes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "minimal"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: minimal
description: minimal command skill
execution_mode: command
supported_agents:
  - claude
command: tool
runtime: external
---

# Minimal

## What it does

x

## When to use

y

## How to run

Run the command.

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "minimal", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{}
	result, err := svc.Generate(RenderOptions{Root: root, Target: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts := result.Artifacts
	var commandDoc string
	for _, artifact := range artifacts {
		if artifact.RelPath == filepath.Join("commands", "minimal.md") {
			commandDoc = string(artifact.Content)
		}
	}
	for _, forbidden := range []string{"Safe to retry:", "Writes files:", "Produces JSON:"} {
		if strings.Contains(commandDoc, forbidden) {
			t.Fatalf("unexpected default execution note %q in command doc:\n%s", forbidden, commandDoc)
		}
	}
}

func TestServiceValidateReportsMissingSection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "bad"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: bad
description: bad skill
execution_mode: docs_only
supported_agents:
  - claude
---

# bad

## What it does

something
`
	if err := os.WriteFile(filepath.Join(root, "skills", "bad", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{}
	report, err := svc.Validate(ValidateOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) == 0 {
		t.Fatal("expected failures")
	}
}

func TestServiceValidateRejectsDocsOnlyCommandFieldsAndDuplicates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "bad-docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	yes := `true`
	body := `---
name: bad-docs
description: docs only with command fields
execution_mode: docs_only
supported_agents:
  - claude
  - claude
allowed_tools:
  - bash
  - bash
command: tool
args:
  - --flag
runtime: external
working_dir: subdir
timeout: 5s
safe_to_retry: ` + yes + `
writes_files: false
produces_json: false
---

# bad-docs

## What it does

x

## When to use

y

## How to run

z

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "bad-docs", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{}
	report, err := svc.Validate(ValidateOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, failure := range report.Failures {
		got = append(got, failure.Message)
	}
	for _, want := range []string{
		`supported_agents contains duplicate value "claude"`,
		`allowed_tools contains duplicate value "bash"`,
		"execution_mode=docs_only must not define command",
		"execution_mode=docs_only must not define args",
		"execution_mode=docs_only must not define runtime",
		"execution_mode=docs_only must not define working_dir",
		"execution_mode=docs_only must not define timeout",
		"execution_mode=docs_only must not define safe_to_retry",
		"execution_mode=docs_only must not define writes_files",
		"execution_mode=docs_only must not define produces_json",
	} {
		found := false
		for _, msg := range got {
			if strings.Contains(msg, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing validation failure %q in %+v", want, got)
		}
	}
}

func TestServiceValidateRejectsInvalidOptionalMetadata(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "badmeta"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: badmeta
description: bad metadata skill
execution_mode: command
supported_agents:
  - claude
allowed_tools:
  - bash
inputs:
  - ""
outputs:
  - ""
command: tool
runtime: external
working_dir: /tmp
timeout: not-a-duration
compatibility:
  requires:
    - ""
  supported_os:
    - ""
  notes:
    - ""
agent_hints:
  codex:
    notes:
      - ""
  typo:
    notes:
      - x
---

# badmeta

## What it does

x

## When to use

y

## How to run

z

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "badmeta", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{}
	report, err := svc.Validate(ValidateOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, failure := range report.Failures {
		got = append(got, failure.Message)
	}
	for _, want := range []string{
		"inputs cannot contain empty values",
		"outputs cannot contain empty values",
		"compatibility.requires cannot contain empty values",
		"compatibility.supported_os cannot contain empty values",
		"compatibility.notes cannot contain empty values",
		`agent_hints.codex requires "codex" in supported_agents`,
		"agent_hints.codex.notes cannot contain empty values",
		`unsupported agent_hints key "typo"`,
		"working_dir must stay within the skill root",
		"timeout must be a valid duration:",
	} {
		found := false
		for _, msg := range got {
			if strings.Contains(msg, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing validation failure %q in %+v", want, got)
		}
	}
}

func TestServiceValidateRejectsMissingWorkingDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "missingwd"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: missingwd
description: missing working dir skill
execution_mode: command
supported_agents:
  - claude
command: tool
runtime: external
working_dir: scripts
---

# missingwd

## What it does

x

## When to use

y

## How to run

z

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "missingwd", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := Service{}
	report, err := svc.Validate(ValidateOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "working_dir must reference an existing directory under the skill root") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected working_dir existence failure, got %+v", report.Failures)
	}
}

func TestServiceRenderReportsStaleArtifactsWhenShapeShrinks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	svc := Service{}
	if _, err := svc.Init(InitOptions{
		Name:      "shrink",
		Template:  filesystem.TemplateGoCommand,
		OutputDir: root,
	}); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Generate(RenderOptions{Root: root, Target: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteArtifacts(root, first.Artifacts); err != nil {
		t.Fatal(err)
	}
	body := `---
name: shrink
description: now docs only and claude only
execution_mode: docs_only
supported_agents:
  - claude
allowed_tools: []
---

# shrink

## What it does

x

## When to use

y

## How to run

z

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "shrink", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Generate(RenderOptions{Root: root, Target: "all"})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{
		filepath.Join("commands", "shrink.md"):                               false,
		filepath.Join("generated", "skills", "codex", "shrink", "SKILL.md"):  false,
		filepath.Join("generated", "skills", "claude", "shrink", "SKILL.md"): true,
	}
	for _, stale := range second.StalePaths {
		if _, ok := expected[stale]; ok {
			expected[stale] = true
		}
	}
	for path, seen := range expected {
		if !seen {
			t.Fatalf("missing managed/stale path %s in %+v", path, second.StalePaths)
		}
	}
}

func TestServiceRenderReportsDeletedSkillArtifactsAsStale(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	svc := Service{}
	if _, err := svc.Init(InitOptions{
		Name:      "ghost",
		Template:  filesystem.TemplateGoCommand,
		OutputDir: root,
	}); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Generate(RenderOptions{Root: root, Target: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteArtifacts(root, first.Artifacts); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "skills", "ghost")); err != nil {
		t.Fatal(err)
	}
	second, err := svc.Generate(RenderOptions{Root: root, Target: "all"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		filepath.Join("commands", "ghost.md"),
		filepath.Join("generated", "skills", "claude", "ghost", "SKILL.md"),
		filepath.Join("generated", "skills", "codex", "ghost", "SKILL.md"),
	} {
		found := false
		for _, stale := range second.StalePaths {
			if stale == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing stale deleted artifact %s in %+v", want, second.StalePaths)
		}
	}
	if err := svc.RemoveArtifacts(root, second.StalePaths); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		filepath.Join("generated", "skills", "claude", "ghost"),
		filepath.Join("generated", "skills", "codex", "ghost"),
		filepath.Join("commands"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected empty directory pruned: %s err=%v", rel, err)
		}
	}
}

func TestServiceInitEscapesYAMLScalars(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	svc := Service{}
	command := `python3 -c "print('a: b # c')"`
	description := `format: repo #1`
	if _, err := svc.Init(InitOptions{
		Name:        "quoted-init",
		Description: description,
		Template:    filesystem.TemplateCLIWrapper,
		OutputDir:   root,
		Command:     command,
	}); err != nil {
		t.Fatal(err)
	}
	report, err := svc.Validate(ValidateOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("unexpected validation failures: %+v", report.Failures)
	}
	doc, err := svc.Repo.LoadSkill(root, "quoted-init")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Spec.Description != description {
		t.Fatalf("description mismatch: got %q want %q", doc.Spec.Description, description)
	}
	if doc.Spec.Command != command {
		t.Fatalf("command mismatch: got %q want %q", doc.Spec.Command, command)
	}
}
