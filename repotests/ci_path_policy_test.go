package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Pull request path filters keep documentation work off the shared runner
// pool (docs/CI_RUNNERS.md). They stay safe only while release evidence keeps
// coming from unfiltered pushes to main and the Required classifier fails open.

type pathPolicyWorkflow struct {
	On   map[string]any `yaml:"on"`
	Jobs map[string]struct {
		If    string `yaml:"if"`
		Needs any    `yaml:"needs"`
		Steps []struct {
			ID  string `yaml:"id"`
			Run string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func readPathPolicyWorkflow(t *testing.T, root, name string) pathPolicyWorkflow {
	t.Helper()
	var workflow pathPolicyWorkflow
	if err := yaml.Unmarshal([]byte(readRepoFile(t, root, ".github", "workflows", name)), &workflow); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return workflow
}

// requireUnfilteredTrigger fails unless the event exists and has no path filter.
func requireUnfilteredTrigger(t *testing.T, root, name, event string) {
	t.Helper()
	trigger, ok := readPathPolicyWorkflow(t, root, name).On[event]
	if !ok {
		t.Fatalf("%s must keep its %s trigger", name, event)
	}
	settings, _ := trigger.(map[string]any)
	for _, filter := range []string{"paths", "paths-ignore"} {
		if _, filtered := settings[filter]; filtered {
			t.Fatalf("%s %s trigger must stay unfiltered for release evidence", name, event)
		}
	}
}

func TestCIPathPolicy_ReleaseEvidenceStaysUnfiltered(t *testing.T) {
	root := RepoRoot(t)
	// Workflows behind the push-triggered checks that agentplugins-release.yml
	// requires on the tagged merge commit.
	for _, name := range []string{"ci.yml", "docs.yml", "polyglot-smoke.yml", "codeql.yml", "govulncheck.yml", "skills-picker.yml"} {
		requireUnfilteredTrigger(t, root, name, "push")
	}
	// Dependency Review only runs on pull requests, and the release accepts its
	// success on the merged pull request head.
	requireUnfilteredTrigger(t, root, "dependency-review.yml", "pull_request")

	ci := readPathPolicyWorkflow(t, root, "ci.yml")
	if test := ci.Jobs["test"]; test.Needs != nil || strings.Contains(test.If, "changes") {
		t.Fatal("Required / test must not depend on the path classifier")
	}
	for _, job := range []string{"lint", "management-sdk"} {
		condition := ci.Jobs[job].If
		//nolint:misspell // the GitHub context function uses the double-l spelling
		if ci.Jobs[job].Needs != "changes" || !strings.Contains(condition, "!cancelled()") ||
			!strings.Contains(condition, "needs.changes.result != 'success'") {
			t.Fatalf("%s must run whenever the path classifier did not succeed: %q", job, condition)
		}
	}
}

func TestCIPathPolicy_RequiredClassifier(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the classifier runs on a Linux runner")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}
	var script string
	for _, step := range readPathPolicyWorkflow(t, RepoRoot(t), "ci.yml").Jobs["changes"].Steps {
		if step.ID == "classify" {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatal("ci.yml changes job must keep the classify step")
	}

	fakeBin := t.TempDir()
	// The fake gh replays the already-flattened file list that the real --jq
	// expression produces, and fails when asked to.
	fakeGH := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$FAKE_GH_ARGS\"\n" +
		"[ -n \"$FAKE_GH_FAIL\" ] && exit 1\ncat \"$FAKE_GH_FILES\"\n"
	if err := os.WriteFile(filepath.Join(fakeBin, "gh"), []byte(fakeGH), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, changed, fail string
		files               []string
		lint, sdk           string
	}{
		{name: "readme only", files: []string{"README.md"}, lint: "false", sdk: "false"},
		{name: "docs and web", files: []string{"docs/CI_RUNNERS.md", "landing/nuxt.config.ts", "website/source/en/index.md", "assets/logo.svg"}, lint: "false", sdk: "false"},
		{name: "embedded markdown", files: []string{"install/integrationctl/agentplugins/conformance/profiles/README.md"}, lint: "false", sdk: "false"},
		{name: "look-alike module file", files: []string{"docs/go.mod.md", "xgo.mod"}, lint: "false", sdk: "false"},
		{name: "unrelated workflow", files: []string{".github/workflows/landing.yml"}, lint: "false", sdk: "false"},
		{name: "go source", files: []string{"cli/internal/app/service.go"}, lint: "true", sdk: "false"},
		{name: "root module", files: []string{"go.mod"}, lint: "true", sdk: "true"},
		{name: "nested checksum", files: []string{"install/integrationctl/go.sum"}, lint: "true", sdk: "false"},
		{name: "workspace checksum", files: []string{"go.work.sum"}, lint: "true", sdk: "false"},
		{name: "lint config", files: []string{".golangci.yml"}, lint: "true", sdk: "false"},
		{name: "lint baseline", files: []string{"scripts/lint-baseline-renames.txt"}, lint: "true", sdk: "false"},
		{name: "lint workflow", files: []string{".github/workflows/lint.yml"}, lint: "true", sdk: "false"},
		{name: "sdk source", files: []string{"sdks/typescript/src/index.ts"}, lint: "false", sdk: "true"},
		{name: "management contract", files: []string{"contracts/management/openapi.yaml"}, lint: "false", sdk: "true"},
		{name: "generated dto", files: []string{"cli/internal/agentpluginscli/management/apitypes.gen.go"}, lint: "true", sdk: "true"},
		{name: "sdk workflow", files: []string{".github/workflows/management-sdk.yml"}, lint: "false", sdk: "true"},
		{name: "required workflow", files: []string{".github/workflows/ci.yml"}, lint: "true", sdk: "true"},
		{name: "rename out of go source", files: []string{"docs/archive/service.txt", "cli/internal/app/service.go"}, lint: "true", sdk: "false"},
		{name: "empty diff", lint: "false", sdk: "false"},
		{name: "api unavailable", files: []string{"README.md"}, fail: "1", lint: "true", sdk: "true"},
		{name: "beyond api limit", files: []string{"README.md"}, changed: "3000", lint: "true", sdk: "true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			filesPath := filepath.Join(dir, "files")
			outputPath := filepath.Join(dir, "output")
			argsPath := filepath.Join(dir, "args")
			list := strings.Join(tc.files, "\n")
			if list != "" {
				list += "\n"
			}
			if err := os.WriteFile(filesPath, []byte(list), 0o600); err != nil {
				t.Fatal(err)
			}
			changed := tc.changed
			if changed == "" {
				changed = "1"
			}
			cmd := exec.CommandContext(t.Context(), bash, "-c", script)
			cmd.Env = append(os.Environ(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"GITHUB_OUTPUT="+outputPath,
				"GITHUB_REPOSITORY=owner/repo",
				"GH_TOKEN=token",
				"PR_NUMBER=7",
				"CHANGED_FILES="+changed,
				"FAKE_GH_FILES="+filesPath,
				"FAKE_GH_ARGS="+argsPath,
				"FAKE_GH_FAIL="+tc.fail,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("classifier failed: %v\n%s", err, out)
			}
			if tc.changed == "" {
				if args, err := os.ReadFile(argsPath); err != nil || !strings.Contains(string(args), "repos/owner/repo/pulls/7/files") {
					t.Fatalf("classifier must list the pull request files: %q %v", args, err)
				}
			}
			output, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				if key, value, ok := strings.Cut(line, "="); ok {
					if _, seen := got[key]; seen {
						t.Fatalf("duplicate output %q in %q", key, output)
					}
					got[key] = value
				}
			}
			if got["lint"] != tc.lint || got["management_sdk"] != tc.sdk || len(got) != 2 {
				t.Fatalf("outputs = %v, want lint=%s management_sdk=%s", got, tc.lint, tc.sdk)
			}
		})
	}
}
