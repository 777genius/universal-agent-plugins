package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Pull request path filters keep documentation work off the shared runner
// pool (docs/CI_RUNNERS.md). They stay safe only while release evidence keeps
// coming from unfiltered pushes to main, every documentation-only filter names
// the same surface, and the shared classifier fails open.

type pathPolicyWorkflow struct {
	On   map[string]any `yaml:"on"`
	Jobs map[string]struct {
		If    string `yaml:"if"`
		Needs any    `yaml:"needs"`
		Uses  string `yaml:"uses"`
		Steps []struct {
			ID  string `yaml:"id"`
			Run string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// documentationOnly is the one documentation and web surface that pull request
// filters and the shared classifier's authoring class may skip.
var documentationOnly = []any{"*.md", "docs/**", "website/**", "landing/**", "assets/**"}

func readPathPolicyWorkflow(t *testing.T, root, name string) pathPolicyWorkflow {
	t.Helper()
	var workflow pathPolicyWorkflow
	if err := yaml.Unmarshal([]byte(readRepoFile(t, root, ".github", "workflows", name)), &workflow); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return workflow
}

func triggerSettings(t *testing.T, root, name, event string) map[string]any {
	t.Helper()
	trigger, ok := readPathPolicyWorkflow(t, root, name).On[event]
	if !ok {
		t.Fatalf("%s must keep its %s trigger", name, event)
	}
	settings, _ := trigger.(map[string]any)
	return settings
}

// requireUnfilteredTrigger fails unless the event exists and has no path filter.
func requireUnfilteredTrigger(t *testing.T, root, name, event string) {
	t.Helper()
	settings := triggerSettings(t, root, name, event)
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
	// Milestone A release admission binds a dispatch run with exactly its jobs.
	requireUnfilteredTrigger(t, root, "authoring-milestone-a-e2e.yml", "workflow_dispatch")

	ci := readPathPolicyWorkflow(t, root, "ci.yml")
	if test := ci.Jobs["test"]; test.Needs != nil || strings.Contains(test.If, "changes") {
		t.Fatal("Required / test must not depend on the path classifier")
	}
	if ci.Jobs["changes"].Uses != "./.github/workflows/pr-changes.yml" {
		t.Fatal("Required must use the shared pull request classifier")
	}
	for job, class := range map[string]string{"lint": "lint", "management-sdk": "management_sdk"} {
		condition := ci.Jobs[job].If
		//nolint:misspell // the GitHub context function uses the double-l spelling
		if ci.Jobs[job].Needs != "changes" || !strings.Contains(condition, "!cancelled()") ||
			!strings.Contains(condition, "needs.changes.result != 'success'") ||
			!strings.Contains(condition, "needs.changes.outputs."+class+" != 'false'") {
			t.Fatalf("%s must skip only on an explicit false classification: %q", job, condition)
		}
	}
}

func TestCIPathPolicy_DocumentationFiltersNameOneSurface(t *testing.T) {
	root := RepoRoot(t)
	for _, name := range []string{"polyglot-smoke.yml", "terminal-ui.yml", "coverage.yml", "skills-picker.yml", "authoring-milestone-a-e2e.yml"} {
		settings := triggerSettings(t, root, name, "pull_request")
		if _, ok := settings["paths"]; ok || !reflect.DeepEqual(settings["paths-ignore"], documentationOnly) {
			t.Fatalf("%s must ignore exactly %v on pull requests, got %v", name, documentationOnly, settings["paths-ignore"])
		}
	}
}

func classifierScript(t *testing.T) string {
	t.Helper()
	for _, step := range readPathPolicyWorkflow(t, RepoRoot(t), "pr-changes.yml").Jobs["classify"].Steps {
		if step.ID == "classify" {
			return step.Run
		}
	}
	t.Fatal("pr-changes.yml must keep the classify step")
	return ""
}

// fakeGitHubCLI replays the already-flattened file list that the real --jq
// expression produces, and fails when asked to.
func fakeGitHubCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$FAKE_GH_ARGS\"\n" +
		"[ -n \"$FAKE_GH_FAIL\" ] && exit 1\ncat \"$FAKE_GH_FILES\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

type classifierCase struct {
	name, changed, fail  string
	files                []string
	lint, sdk, authoring string
}

var classifierCases = []classifierCase{
	{name: "readme only", files: []string{"README.md"}, lint: "false", sdk: "false", authoring: "false"},
	{name: "docs and web", files: []string{"docs/CI_RUNNERS.md", "landing/nuxt.config.ts", "landing/pages/plugins/[slug].vue", "website/source/en/index.md", "assets/logo.svg"}, lint: "false", sdk: "false", authoring: "false"},
	{name: "nested markdown is not documentation only", files: []string{"cli/README.md"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "embedded markdown", files: []string{"install/integrationctl/agentplugins/conformance/profiles/README.md"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "look-alike module file", files: []string{"docs/go.mod.md", "docsx/go.mod.txt"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "unrelated workflow", files: []string{".github/workflows/landing.yml"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "examples", files: []string{"examples/skills/review/SKILL.md"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "go source", files: []string{"cli/internal/app/service.go"}, lint: "true", sdk: "false", authoring: "true"},
	{name: "root module", files: []string{"go.mod"}, lint: "true", sdk: "true", authoring: "true"},
	{name: "nested checksum", files: []string{"install/integrationctl/go.sum"}, lint: "true", sdk: "false", authoring: "true"},
	{name: "workspace checksum", files: []string{"go.work.sum"}, lint: "true", sdk: "false", authoring: "true"},
	{name: "lint config", files: []string{".golangci.yml"}, lint: "true", sdk: "false", authoring: "true"},
	{name: "lint baseline", files: []string{"scripts/lint-baseline-renames.txt"}, lint: "true", sdk: "false", authoring: "true"},
	{name: "lint workflow", files: []string{".github/workflows/lint.yml"}, lint: "true", sdk: "false", authoring: "true"},
	{name: "sdk source", files: []string{"sdks/typescript/src/index.ts"}, lint: "false", sdk: "true", authoring: "true"},
	{name: "management contract", files: []string{"contracts/management/openapi.yaml"}, lint: "false", sdk: "true", authoring: "true"},
	{name: "generated dto", files: []string{"cli/internal/agentpluginscli/management/apitypes.gen.go"}, lint: "true", sdk: "true", authoring: "true"},
	{name: "sdk workflow", files: []string{".github/workflows/management-sdk.yml"}, lint: "false", sdk: "true", authoring: "true"},
	{name: "required workflow", files: []string{".github/workflows/ci.yml"}, lint: "true", sdk: "true", authoring: "true"},
	{name: "classifier workflow", files: []string{".github/workflows/pr-changes.yml"}, lint: "true", sdk: "true", authoring: "true"},
	{name: "rename out of go source", files: []string{"docs/archive/service.txt", "cli/internal/app/service.go"}, lint: "true", sdk: "false", authoring: "true"},
	{name: "windows reserved character", files: []string{"docs/a:b.md"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "windows trailing dot", files: []string{"docs/notes./a.md"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "windows device name", files: []string{"website/source/en/Aux.md"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "windows path length", files: []string{"docs/" + strings.Repeat("a", 200) + ".md"}, lint: "false", sdk: "false", authoring: "true"},
	{name: "empty diff", changed: "0", lint: "false", sdk: "false", authoring: "false"},
	{name: "no paths for a non-empty diff", lint: "true", sdk: "true", authoring: "true"},
	{name: "control character", files: []string{"docs/a\tb.md"}, lint: "true", sdk: "true", authoring: "true"},
	{name: "api unavailable", files: []string{"README.md"}, fail: "1", lint: "true", sdk: "true", authoring: "true"},
	{name: "beyond api limit", files: []string{"README.md"}, changed: "3000", lint: "true", sdk: "true", authoring: "true"},
}

func TestCIPathPolicy_SharedClassifier(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the classifier runs on a Linux runner")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}
	script, fakeBin := classifierScript(t), fakeGitHubCLI(t)
	for _, tc := range classifierCases {
		t.Run(tc.name, func(t *testing.T) {
			got := runClassifier(t, bash, script, fakeBin, tc)
			want := map[string]string{"lint": tc.lint, "management_sdk": tc.sdk, "authoring": tc.authoring}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("outputs = %v, want %v", got, want)
			}
		})
	}
}

func runClassifier(t *testing.T, bash, script, fakeBin string, tc classifierCase) map[string]string {
	t.Helper()
	dir := t.TempDir()
	filesPath, outputPath, argsPath := filepath.Join(dir, "files"), filepath.Join(dir, "output"), filepath.Join(dir, "args")
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
		"GITHUB_OUTPUT="+outputPath, "GITHUB_REPOSITORY=owner/repo", "GH_TOKEN=token", "PR_NUMBER=7",
		"CHANGED_FILES="+changed, "FAKE_GH_FILES="+filesPath, "FAKE_GH_ARGS="+argsPath, "FAKE_GH_FAIL="+tc.fail,
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
	return got
}
