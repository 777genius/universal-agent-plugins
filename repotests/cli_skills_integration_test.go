package pluginkitairepo_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAISkillsInitValidateRender(t *testing.T) {
	bin := buildPluginKitAI(t)
	cases := []struct {
		name         string
		skillName    string
		template     string
		command      string
		mustExist    []string
		mustNotExist []string
	}{
		{
			name:      "go-command",
			skillName: "lint-repo",
			template:  "go-command",
			mustExist: []string{
				filepath.Join("skills", "lint-repo", "SKILL.md"),
				filepath.Join("cmd", "lint-repo", "main.go"),
				filepath.Join("generated", "skills", "claude", "lint-repo", "SKILL.md"),
				filepath.Join("generated", "skills", "codex", "lint-repo", "SKILL.md"),
				filepath.Join("commands", "lint-repo.md"),
			},
		},
		{
			name:      "cli-wrapper",
			skillName: "format-changed",
			template:  "cli-wrapper",
			command:   "npx prettier@3.4.2 --write .",
			mustExist: []string{
				filepath.Join("skills", "format-changed", "SKILL.md"),
				filepath.Join("skills", "format-changed", "scripts", ".keep"),
				filepath.Join("generated", "skills", "claude", "format-changed", "SKILL.md"),
				filepath.Join("generated", "skills", "codex", "format-changed", "SKILL.md"),
				filepath.Join("commands", "format-changed.md"),
			},
		},
		{
			name:      "docs-only",
			skillName: "review-checklist",
			template:  "docs-only",
			mustExist: []string{
				filepath.Join("skills", "review-checklist", "SKILL.md"),
				filepath.Join("skills", "review-checklist", "references", ".keep"),
				filepath.Join("generated", "skills", "claude", "review-checklist", "SKILL.md"),
				filepath.Join("generated", "skills", "codex", "review-checklist", "SKILL.md"),
			},
			mustNotExist: []string{
				filepath.Join("commands", "review-checklist.md"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			args := []string{"skills", "init", tc.skillName, "--output", root, "--template", tc.template}
			if tc.command != "" {
				args = append(args, "--command", tc.command)
			}
			initCmd := exec.Command(bin, args...)
			if out, err := initCmd.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai skills init: %v\n%s", err, out)
			}

			validateCmd := exec.Command(bin, "skills", "validate", root)
			if out, err := validateCmd.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai skills validate: %v\n%s", err, out)
			}

			generateCmd := exec.Command(bin, "skills", "generate", root, "--target", "all")
			if out, err := generateCmd.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai skills generate: %v\n%s", err, out)
			}

			for _, rel := range tc.mustExist {
				if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
					t.Fatalf("missing %s: %v", rel, err)
				}
			}
			for _, rel := range tc.mustNotExist {
				if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
					t.Fatalf("unexpected %s", rel)
				}
			}
		})
	}
}

func TestPluginKitAISkillsValidateReportsMultipleProblems(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
description: broken skill
execution_mode: nope
supported_agents:
  - claude
  - invalid-agent
allowed_tools:
  - ""
command: echo hi
runtime: nope
---

# Broken Skill

## What it does

Broken on purpose.
`
	if err := os.WriteFile(filepath.Join(root, "skills", "broken", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	validateCmd := exec.Command(bin, "skills", "validate", root)
	out, err := validateCmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected validation to fail")
	}
	text := string(out)
	for _, want := range []string{
		"Skill validation found",
		filepath.Join("skills", "broken", "SKILL.md") + ": missing frontmatter field: name",
		"invalid execution_mode",
		"unsupported agent",
		"allowed_tools cannot contain empty values",
		"missing section: When to use",
		"missing section: How to run",
		"missing section: Constraints",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("validation output missing %q:\n%s", want, text)
		}
	}
}

func TestPluginKitAISkillsRenderRejectsInvalidSkill(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: broken
description: broken skill
execution_mode: nope
supported_agents:
  - claude
---

# Broken Skill

## What it does

Broken on purpose.

## When to use

When you want a failure.

## How to run

Do not run it.

## Constraints

- Invalid on purpose.
`
	if err := os.WriteFile(filepath.Join(root, "skills", "broken", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	generateCmd := exec.Command(bin, "skills", "generate", root, "--target", "all")
	out, err := generateCmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected generate to fail")
	}
	text := string(out)
	if !strings.Contains(text, "cannot generate invalid skills") || !strings.Contains(text, "invalid execution_mode") {
		t.Fatalf("unexpected generate error:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(root, "generated")); !os.IsNotExist(err) {
		t.Fatalf("expected no generated output, got err=%v", err)
	}
}

func TestPluginKitAISkillsValidateRejectsNameDirectoryMismatch(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: bar
description: mismatched skill
execution_mode: docs_only
supported_agents:
  - claude
---

# Mismatch

## What it does

x

## When to use

y

## How to run

z

## Constraints

- c
`
	if err := os.WriteFile(filepath.Join(root, "skills", "foo", "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	validateCmd := exec.Command(bin, "skills", "validate", root)
	out, err := validateCmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected validate to fail")
	}
	if !strings.Contains(string(out), `frontmatter name "bar" must match skill directory "foo"`) {
		t.Fatalf("unexpected validation output:\n%s", out)
	}
}

func TestPluginKitAISkillsHandwrittenCompatibility(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := t.TempDir()

	mustWriteFile(t, filepath.Join(root, "skills", "review-guide", "SKILL.md"), `---
name: review-guide
description: Handwritten review checklist for Claude only.
execution_mode: docs_only
supported_agents:
  - claude
allowed_tools: []
compatibility:
  repo_required: true
  notes:
    - Keep findings concrete and tied to changed files.
---

# Review Guide

## What it does

Provides a repeatable review checklist.

## When to use

Use this before handoff or merge.

## How to run

Read the checklist and inspect the changed files.

## Constraints

- This skill is instructional only.
`)
	mustWriteFile(t, filepath.Join(root, "skills", "review-guide", "references", "merge.md"), "Prefer exact file references.\n")
	mustWriteFile(t, filepath.Join(root, "skills", "review-guide", "agents", "openai.yaml"), "name: review-guide\n")

	mustWriteFile(t, filepath.Join(root, "skills", "format-staged", "SKILL.md"), `---
name: format-staged
description: Handwritten shell wrapper around a local formatter script.
execution_mode: command
supported_agents:
  - claude
  - codex
allowed_tools:
  - bash
command: ./scripts/format.sh
runtime: shell
compatibility:
  repo_required: true
  notes:
    - Runs a repository-local shell wrapper from the skill root.
safe_to_retry: true
writes_files: true
produces_json: false
---

# Format Staged Files

## What it does

Runs a local script wrapper for formatting.

## When to use

Use this when the repo already has its own formatting wrapper.

## How to run

Run the shell wrapper non-interactively.

## Constraints

- Review the diff after formatting.
`)
	mustWriteFile(t, filepath.Join(root, "skills", "format-staged", "scripts", "format.sh"), "#!/bin/sh\nexit 0\n")
	mustWriteFile(t, filepath.Join(root, "skills", "format-staged", "assets", "note.txt"), "formatter wrapper\n")

	mustWriteFile(t, filepath.Join(root, "skills", "python-fix", "SKILL.md"), `---
name: python-fix
description: Handwritten external CLI skill for Codex only.
execution_mode: command
supported_agents:
  - codex
allowed_tools:
  - python
command: uvx ruff@0.8.0 format .
runtime: external
compatibility:
  requires:
    - uv
  supported_os:
    - darwin
    - linux
  repo_required: true
  network_required: true
  notes:
    - The first run may download the pinned package.
safe_to_retry: true
writes_files: true
produces_json: false
---

# Python Fix

## What it does

Formats Python files through a pinned external CLI.

## When to use

Use this when the repository standardizes on Ruff.

## How to run

Run the pinned uvx command from the repository root.

## Constraints

- This command may download dependencies on the first run.
`)
	mustWriteFile(t, filepath.Join(root, "skills", "python-fix", "references", "style.md"), "Use the pinned version.\n")

	validateCmd := exec.Command(bin, "skills", "validate", root)
	if out, err := validateCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills validate: %v\n%s", err, out)
	}
	generateCmd := exec.Command(bin, "skills", "generate", root, "--target", "all")
	if out, err := generateCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills generate: %v\n%s", err, out)
	}

	for _, rel := range []string{
		filepath.Join("generated", "skills", "claude", "review-guide", "SKILL.md"),
		filepath.Join("generated", "skills", "claude", "format-staged", "SKILL.md"),
		filepath.Join("generated", "skills", "codex", "format-staged", "SKILL.md"),
		filepath.Join("generated", "skills", "codex", "python-fix", "SKILL.md"),
		filepath.Join("commands", "format-staged.md"),
		filepath.Join("commands", "python-fix.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing handwritten compatibility artifact %s: %v", rel, err)
		}
	}

	for _, rel := range []string{
		filepath.Join("generated", "skills", "codex", "review-guide", "SKILL.md"),
		filepath.Join("generated", "skills", "claude", "python-fix", "SKILL.md"),
		filepath.Join("commands", "review-guide.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected handwritten compatibility artifact %s: err=%v", rel, err)
		}
	}

	for _, rel := range []string{
		filepath.Join("skills", "review-guide", "references", "merge.md"),
		filepath.Join("skills", "review-guide", "agents", "openai.yaml"),
		filepath.Join("skills", "format-staged", "scripts", "format.sh"),
		filepath.Join("skills", "format-staged", "assets", "note.txt"),
		filepath.Join("skills", "python-fix", "references", "style.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected handwritten compatibility side file to remain: %s: %v", rel, err)
		}
	}
}

func TestPluginKitAISkillsExamplesValidateAndRender(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := RepoRoot(t)
	examples := []struct {
		root  string
		files []string
	}{
		{
			root: filepath.Join(root, "examples", "skills", "go-command-lint"),
			files: []string{
				filepath.Join("generated", "skills", "claude", "lint-repo", "SKILL.md"),
				filepath.Join("generated", "skills", "codex", "lint-repo", "SKILL.md"),
				filepath.Join("commands", "lint-repo.md"),
			},
		},
		{
			root: filepath.Join(root, "examples", "skills", "cli-wrapper-formatter"),
			files: []string{
				filepath.Join("generated", "skills", "claude", "format-changed", "SKILL.md"),
				filepath.Join("generated", "skills", "codex", "format-changed", "SKILL.md"),
				filepath.Join("commands", "format-changed.md"),
			},
		},
		{
			root: filepath.Join(root, "examples", "skills", "docs-only-review"),
			files: []string{
				filepath.Join("generated", "skills", "claude", "review-checklist", "SKILL.md"),
				filepath.Join("generated", "skills", "codex", "review-checklist", "SKILL.md"),
			},
		},
	}
	for _, example := range examples {
		t.Run(filepath.Base(example.root), func(t *testing.T) {
			before := make(map[string][]byte, len(example.files))
			for _, rel := range example.files {
				full := filepath.Join(example.root, rel)
				body, err := os.ReadFile(full)
				if err != nil {
					t.Fatalf("read committed example artifact %s: %v", rel, err)
				}
				before[rel] = body
			}
			validateCmd := exec.Command(bin, "skills", "validate", example.root)
			if out, err := validateCmd.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai skills validate: %v\n%s", err, out)
			}
			generateCmd := exec.Command(bin, "skills", "generate", example.root, "--target", "all")
			if out, err := generateCmd.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai skills generate: %v\n%s", err, out)
			}
			for _, rel := range example.files {
				full := filepath.Join(example.root, rel)
				if _, err := os.Stat(full); err != nil {
					t.Fatalf("missing example artifact %s: %v", rel, err)
				}
				after, err := os.ReadFile(full)
				if err != nil {
					t.Fatalf("read generated example artifact %s: %v", rel, err)
				}
				if !bytes.Equal(before[rel], after) {
					t.Fatalf("example artifact drift after generate: %s", rel)
				}
			}
		})
	}
}

func TestPluginKitAISkillsExamplesArtifactsTracked(t *testing.T) {
	root := RepoRoot(t)
	paths := []string{
		"examples/skills/go-command-lint/generated/skills/codex/lint-repo/SKILL.md",
		"examples/skills/cli-wrapper-formatter/generated/skills/codex/format-changed/SKILL.md",
		"examples/skills/docs-only-review/generated/skills/codex/review-checklist/SKILL.md",
	}
	args := append([]string{"ls-files", "--error-unmatch"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("expected example artifacts to be tracked: %v\n%s", err, out)
	}
}

func TestPluginKitAISkillsRenderRemovesStaleArtifacts(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := t.TempDir()

	initCmd := exec.Command(bin, "skills", "init", "shrink", "--output", root, "--template", "go-command")
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills init: %v\n%s", err, out)
	}
	generateCmd := exec.Command(bin, "skills", "generate", root, "--target", "all")
	if out, err := generateCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills generate: %v\n%s", err, out)
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

	generateCmd = exec.Command(bin, "skills", "generate", root, "--target", "all")
	if out, err := generateCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills generate after shrink: %v\n%s", err, out)
	}

	for _, rel := range []string{
		filepath.Join("commands", "shrink.md"),
		filepath.Join("generated", "skills", "codex", "shrink", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected stale artifact removed: %s err=%v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "generated", "skills", "claude", "shrink", "SKILL.md")); err != nil {
		t.Fatalf("expected current claude artifact: %v", err)
	}
}

func TestPluginKitAISkillsRenderRemovesDeletedSkillArtifacts(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := t.TempDir()

	initCmd := exec.Command(bin, "skills", "init", "ghost", "--output", root, "--template", "go-command")
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills init: %v\n%s", err, out)
	}
	generateCmd := exec.Command(bin, "skills", "generate", root, "--target", "all")
	if out, err := generateCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills generate: %v\n%s", err, out)
	}
	if err := os.RemoveAll(filepath.Join(root, "skills", "ghost")); err != nil {
		t.Fatal(err)
	}
	generateCmd = exec.Command(bin, "skills", "generate", root, "--target", "all")
	if out, err := generateCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills generate after delete: %v\n%s", err, out)
	}
	for _, rel := range []string{
		filepath.Join("commands", "ghost.md"),
		filepath.Join("generated", "skills", "claude", "ghost", "SKILL.md"),
		filepath.Join("generated", "skills", "codex", "ghost", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected deleted artifact removed: %s err=%v", rel, err)
		}
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

func TestPluginKitAISkillsInitEscapesManifestValues(t *testing.T) {
	bin := buildPluginKitAI(t)
	root := t.TempDir()

	command := `python3 -c "print('a: b # c')"`
	description := `format: repo #1`
	initCmd := exec.Command(
		bin,
		"skills", "init", "quoted-init",
		"--output", root,
		"--template", "cli-wrapper",
		"--description", description,
		"--command", command,
	)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills init: %v\n%s", err, out)
	}
	validateCmd := exec.Command(bin, "skills", "validate", root)
	if out, err := validateCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai skills validate: %v\n%s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(root, "skills", "quoted-init", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	descriptionEscaped := strings.Contains(text, `description: "format: repo #1"`) || strings.Contains(text, `description: 'format: repo #1'`)
	commandEscaped := strings.Contains(text, `command: "python3 -c \"print('a: b # c')\""`) || strings.Contains(text, `command: 'python3 -c "print(''a: b # c'')"'`)
	if !descriptionEscaped {
		t.Fatalf("generated SKILL.md missing escaped description:\n%s", text)
	}
	if !commandEscaped {
		t.Fatalf("generated SKILL.md missing escaped command:\n%s", text)
	}
}

func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
