package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type runtimePublishWorkflow struct {
	On   map[string]any               `yaml:"on"`
	Jobs map[string]runtimePublishJob `yaml:"jobs"`
}

type runtimePublishJob struct {
	If          string                       `yaml:"if"`
	Needs       string                       `yaml:"needs"`
	Environment any                          `yaml:"environment"`
	Permissions map[string]string            `yaml:"permissions"`
	Steps       []runtimePublishWorkflowStep `yaml:"steps"`
}

type runtimePublishWorkflowStep struct {
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

func TestRuntimePublishersBindV1TagsAndWorkflowBeforeEffects(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)

	for _, tc := range []struct {
		file, publishJob, environment, effect string
	}{
		{"npm-runtime-publish.yml", "publish-npm-runtime", "npm-plugin-kit-ai", "Configure npm publish authentication"},
		{"pypi-runtime-publish.yml", "publish-pypi-runtime", "pypi", "Check PyPI trusted publishing readiness"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			body := readRepoFile(t, root, ".github", "workflows", tc.file)
			var workflow runtimePublishWorkflow
			if err := yaml.Unmarshal([]byte(body), &workflow); err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}
			if _, dispatch := workflow.On["workflow_dispatch"]; len(workflow.On) != 1 || !dispatch {
				t.Fatalf("%s triggers = %#v, want dispatch only", tc.file, workflow.On)
			}

			preflight, ok := workflow.Jobs["preflight"]
			if !ok {
				t.Fatalf("%s has no isolated preflight job", tc.file)
			}
			for _, marker := range []string{
				"github.event_name == 'workflow_dispatch'",
				"github.repository == '777genius/universal-agent-plugins'",
				"github.ref == 'refs/heads/main'",
				"@refs/heads/main",
			} {
				mustContain(t, preflight.If, marker)
			}
			if len(preflight.Permissions) != 1 || preflight.Permissions["contents"] != "read" {
				t.Fatalf("%s preflight permissions = %#v, want contents: read only", tc.file, preflight.Permissions)
			}

			publish, ok := workflow.Jobs[tc.publishJob]
			if !ok {
				t.Fatalf("%s has no %s job", tc.file, tc.publishJob)
			}
			if publish.Needs != "preflight" {
				t.Fatalf("%s publish needs = %q, want preflight", tc.file, publish.Needs)
			}
			for _, marker := range []string{"needs.preflight.result == 'success'", "github.event_name == 'workflow_dispatch'"} {
				mustContain(t, publish.If, marker)
			}
			if runtimePublishEnvironmentName(publish.Environment) != tc.environment {
				t.Fatalf("%s environment = %#v, want %q", tc.file, publish.Environment, tc.environment)
			}
			if publish.Permissions["contents"] != "read" || publish.Permissions["id-token"] != "write" {
				t.Fatalf("%s publish permissions = %#v", tc.file, publish.Permissions)
			}

			preflightScript := runtimePublishNamedStep(t, preflight, "Verify approved runtime tag and workflow source").Run
			for _, marker := range []string{
				`^v1\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`,
				`git merge-base --is-ancestor "${tag_commit}" refs/remotes/origin/main`,
				`git merge-base --is-ancestor "${WORKFLOW_SHA}" refs/remotes/origin/main`,
				`git diff --quiet "${WORKFLOW_SHA}" "${tag_commit}" -- "${WORKFLOW_PATH}"`,
			} {
				mustContain(t, preflightScript, marker)
			}

			revalidate := runtimePublishStepIndex(publish, "Revalidate approved revision before effects")
			effect := runtimePublishStepIndex(publish, tc.effect)
			if revalidate < 0 || effect < 0 || revalidate >= effect {
				t.Fatalf("%s does not revalidate before effect-bearing setup: revalidate=%d effect=%d", tc.file, revalidate, effect)
			}
			mustContain(t, publish.Steps[revalidate].Run, `test "${tag_commit}" = "${COMMIT}"`)
			mustContain(t, publish.Steps[revalidate].Run, `git diff --quiet "${WORKFLOW_SHA}" "${COMMIT}" -- "${WORKFLOW_PATH}"`)
		})
	}
}

func TestRuntimePublisherPreflightRejectsUnapprovedInputs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runtime publisher preflight is a bash workflow")
	}
	root := RepoRoot(t)
	for _, file := range []string{"npm-runtime-publish.yml", "pypi-runtime-publish.yml"} {
		t.Run(file, func(t *testing.T) {
			body := readRepoFile(t, root, ".github", "workflows", file)
			var workflow runtimePublishWorkflow
			if err := yaml.Unmarshal([]byte(body), &workflow); err != nil {
				t.Fatal(err)
			}
			script := runtimePublishNamedStep(t, workflow.Jobs["preflight"], "Verify approved runtime tag and workflow source").Run

			for _, tc := range []struct {
				name, fixture, tag, event string
				wantSuccess               bool
			}{
				{"approved-v1", "approved", "v1.2.3", "workflow_dispatch", true},
				{"major-version", "approved", "v2.0.0", "workflow_dispatch", false},
				{"wrong-event", "approved", "v1.2.3", "push", false},
				{"tag-off-main", "off-main", "v1.2.3", "workflow_dispatch", false},
				{"workflow-content-mismatch", "mismatch", "v1.2.3", "workflow_dispatch", false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					checkout, workflowSHA := runtimePublisherGitFixture(t, body, file, tc.fixture)
					output := filepath.Join(t.TempDir(), "github-output")
					cmd := exec.Command("bash", "-c", script)
					cmd.Dir = checkout
					cmd.Env = append(os.Environ(),
						"TAG="+tc.tag,
						"WORKFLOW_PATH=.github/workflows/"+file,
						"WORKFLOW_REF=777genius/universal-agent-plugins/.github/workflows/"+file+"@refs/heads/main",
						"WORKFLOW_SHA="+workflowSHA,
						"GITHUB_EVENT_NAME="+tc.event,
						"GITHUB_REPOSITORY=777genius/universal-agent-plugins",
						"GITHUB_REF=refs/heads/main",
						"GITHUB_OUTPUT="+output,
					)
					out, err := cmd.CombinedOutput()
					if tc.wantSuccess && err != nil {
						t.Fatalf("approved preflight failed: %v\n%s", err, out)
					}
					if !tc.wantSuccess && err == nil {
						t.Fatalf("unapproved preflight succeeded:\n%s", out)
					}
				})
			}
		})
	}
}

func runtimePublishEnvironmentName(value any) string {
	switch environment := value.(type) {
	case string:
		return environment
	case map[string]any:
		name, _ := environment["name"].(string)
		return name
	default:
		return ""
	}
}

func runtimePublishNamedStep(t *testing.T, job runtimePublishJob, name string) runtimePublishWorkflowStep {
	t.Helper()
	for _, step := range job.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("workflow job has no step %q", name)
	return runtimePublishWorkflowStep{}
}

func runtimePublishStepIndex(job runtimePublishJob, name string) int {
	for index, step := range job.Steps {
		if step.Name == name {
			return index
		}
	}
	return -1
}

func runtimePublisherGitFixture(t *testing.T, workflowBody, workflowFile, mode string) (string, string) {
	t.Helper()
	fixture := t.TempDir()
	origin := filepath.Join(fixture, "origin.git")
	work := filepath.Join(fixture, "work")
	checkout := filepath.Join(fixture, "checkout")
	mustRun(t, "", "git", "init", "--bare", origin)
	mustRun(t, "", "git", "init", "-b", "main", work)
	mustRun(t, work, "git", "config", "user.name", "Runtime Publisher Test")
	mustRun(t, work, "git", "config", "user.email", "runtime-publisher@example.invalid")
	mustRun(t, work, "git", "remote", "add", "origin", origin)

	path := filepath.Join(work, ".github", "workflows", workflowFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := workflowBody
	if mode == "mismatch" {
		initial = "name: stale runtime publisher\n"
	}
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, work, "git", "add", ".")
	mustRun(t, work, "git", "commit", "-m", "workflow base")

	switch mode {
	case "approved":
		mustRun(t, work, "git", "tag", "v1.2.3")
	case "mismatch":
		mustRun(t, work, "git", "tag", "v1.2.3")
		if err := os.WriteFile(path, []byte(workflowBody), 0o644); err != nil {
			t.Fatal(err)
		}
		mustRun(t, work, "git", "add", ".")
		mustRun(t, work, "git", "commit", "-m", "change workflow")
	case "off-main":
		mustRun(t, work, "git", "checkout", "-b", "unapproved")
		if err := os.WriteFile(filepath.Join(work, "unapproved.txt"), []byte("not on main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		mustRun(t, work, "git", "add", ".")
		mustRun(t, work, "git", "commit", "-m", "unapproved tag")
		mustRun(t, work, "git", "tag", "v1.2.3")
		mustRun(t, work, "git", "checkout", "main")
	default:
		t.Fatalf("unknown runtime publisher fixture %q", mode)
	}

	mustRun(t, work, "git", "push", "origin", "main")
	mustRun(t, work, "git", "push", "origin", "refs/tags/v1.2.3")
	mustRun(t, "", "git", "clone", "--branch", "main", origin, checkout)
	workflowSHA := strings.TrimSpace(mustRun(t, checkout, "git", "rev-parse", "HEAD"))
	return checkout, workflowSHA
}

func TestPythonRuntimePackageContractFiles(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)

	for _, trackedPath := range []string{
		"python/plugin-kit-ai-runtime/pyproject.toml",
		"python/plugin-kit-ai-runtime/README.md",
		"python/plugin-kit-ai-runtime/src/plugin_kit_ai_runtime/__init__.py",
	} {
		tracked := exec.Command("git", "ls-files", "--error-unmatch", trackedPath)
		tracked.Dir = root
		if out, err := tracked.CombinedOutput(); err != nil {
			t.Fatalf("python runtime package file must be tracked in git (%s): %v\n%s", trackedPath, err, out)
		}
	}

	pyproject := readRepoFile(t, root, "python", "plugin-kit-ai-runtime", "pyproject.toml")
	for _, want := range []string{
		`name = "plugin-kit-ai-runtime"`,
		`requires-python = ">=3.10"`,
		`dynamic = ["version"]`,
		`version = { attr = "plugin_kit_ai_runtime.__version__" }`,
	} {
		mustContain(t, pyproject, want)
	}

	initPy := readRepoFile(t, root, "python", "plugin-kit-ai-runtime", "src", "plugin_kit_ai_runtime", "__init__.py")
	for _, want := range []string{
		`__version__ = "0.0.0.dev0"`,
		`CLAUDE_STABLE_HOOKS = (`,
		`CLAUDE_EXTENDED_HOOKS = (`,
		"class ClaudeApp:",
		"class CodexApp:",
	} {
		mustContain(t, initPy, want)
	}

	readme := readRepoFile(t, root, "python", "plugin-kit-ai-runtime", "README.md")
	for _, want := range []string{
		"pip install plugin-kit-ai-runtime",
		"supported handler-oriented API as a shared dependency",
		"local `plugin/plugin_runtime.py` helper",
		"Go is still the recommended path",
		"stable supported lane",
	} {
		mustContain(t, readme, want)
	}

	workflow := readRepoFile(t, root, ".github", "workflows", "pypi-runtime-publish.yml")
	for _, want := range []string{
		"name: PyPI Runtime Publish",
		"workflow_dispatch:",
		"plugin-kit-ai-runtime",
		"plugin-kit-ai-runtime PyPI prepublish smoke ok",
		"id-token: write",
		"pypa/gh-action-pypi-publish@release/v1",
	} {
		mustContain(t, workflow, want)
	}
	mustNotContain(t, workflow, "workflow_run:")
}

func TestPythonRuntimePackageClaudeAndCodexSmoke(t *testing.T) {
	t.Parallel()
	requirePythonRuntime(t)

	root := RepoRoot(t)
	appDir := t.TempDir()
	pkgSrc := filepath.Join(root, "python", "plugin-kit-ai-runtime", "src")

	codexScript := filepath.Join(appDir, "codex_app.py")
	if err := os.WriteFile(codexScript, []byte(`from plugin_kit_ai_runtime import CodexApp, continue_

app = CodexApp()


@app.on_notify
def on_notify(event):
    if event.get("client") != "codex-tui":
        raise RuntimeError(f"unexpected client: {event}")
    return continue_()


raise SystemExit(app.run())
`), 0o644); err != nil {
		t.Fatal(err)
	}

	codex := exec.Command("python3", codexScript, "notify", `{"client":"codex-tui"}`)
	codex.Env = append(os.Environ(), "PYTHONPATH="+pkgSrc)
	var codexStdout bytes.Buffer
	var codexStderr bytes.Buffer
	codex.Stdout = &codexStdout
	codex.Stderr = &codexStderr
	if err := codex.Run(); err != nil {
		t.Fatalf("python runtime package codex smoke: %v\nstderr=%s", err, codexStderr.String())
	}
	if strings.TrimSpace(codexStdout.String()) != "" {
		t.Fatalf("codex stdout = %q, want empty", codexStdout.String())
	}

	claudeScript := filepath.Join(appDir, "claude_app.py")
	if err := os.WriteFile(claudeScript, []byte(`from plugin_kit_ai_runtime import CLAUDE_STABLE_HOOKS, ClaudeApp, allow

app = ClaudeApp(allowed_hooks=CLAUDE_STABLE_HOOKS, usage="claude_app.py <hook-name>")


@app.on_stop
def on_stop(event):
    if event.get("hook_event_name") != "Stop":
        raise RuntimeError(f"unexpected hook payload: {event}")
    return allow()


raise SystemExit(app.run())
`), 0o644); err != nil {
		t.Fatal(err)
	}

	claude := exec.Command("python3", claudeScript, "Stop")
	claude.Env = append(os.Environ(), "PYTHONPATH="+pkgSrc)
	claude.Stdin = strings.NewReader(`{"hook_event_name":"Stop"}`)
	var claudeStdout bytes.Buffer
	var claudeStderr bytes.Buffer
	claude.Stdout = &claudeStdout
	claude.Stderr = &claudeStderr
	if err := claude.Run(); err != nil {
		t.Fatalf("python runtime package claude smoke: %v\nstderr=%s", err, claudeStderr.String())
	}
	if strings.TrimSpace(claudeStdout.String()) != "{}" {
		t.Fatalf("claude stdout = %q, want {}", claudeStdout.String())
	}
}

func TestPythonRuntimePackageBuildInstallAndUpgradeSmoke(t *testing.T) {
	t.Parallel()
	requirePythonRuntime(t)
	requirePythonBuildBackend(t)

	v1Root := copyPythonRuntimeAuthoringPackageToTemp(t)
	rewritePythonRuntimeAuthoringPackageVersion(t, v1Root, "1.0.6")
	v2Root := copyPythonRuntimeAuthoringPackageToTemp(t)
	rewritePythonRuntimeAuthoringPackageVersion(t, v2Root, "1.1.0")

	venvDir := filepath.Join(t.TempDir(), "venv")
	mustRun(t, "", "python3", "-m", "venv", venvDir)
	pythonBin := venvPythonPath(venvDir)
	if _, err := os.Stat(pythonBin); err != nil {
		t.Fatalf("python venv binary: %v", err)
	}

	mustRun(t, v1Root, "python3", "-m", "pip", "wheel", ".", "--no-deps", "--no-build-isolation", "-w", "dist")
	wheelV1 := firstMatch(t, filepath.Join(v1Root, "dist", "*.whl"))
	mustRun(t, "", pythonBin, "-m", "pip", "install", wheelV1)
	versionOut := mustRun(t, "", pythonBin, "-c", `import importlib.metadata; print(importlib.metadata.version("plugin-kit-ai-runtime"))`)
	if strings.TrimSpace(versionOut) != "1.0.6" {
		t.Fatalf("installed python runtime package version = %q, want 1.0.6", versionOut)
	}
	mustRun(t, "", pythonBin, "-c", `from plugin_kit_ai_runtime import CodexApp, continue_; app = CodexApp(); app.on_notify(lambda event: continue_()); print("python runtime consumer ok")`)

	mustRun(t, v2Root, "python3", "-m", "pip", "wheel", ".", "--no-deps", "--no-build-isolation", "-w", "dist")
	wheelV2 := firstMatch(t, filepath.Join(v2Root, "dist", "*.whl"))
	mustRun(t, "", pythonBin, "-m", "pip", "install", "--upgrade", wheelV2)
	versionOut = mustRun(t, "", pythonBin, "-c", `import importlib.metadata; print(importlib.metadata.version("plugin-kit-ai-runtime"))`)
	if strings.TrimSpace(versionOut) != "1.1.0" {
		t.Fatalf("upgraded python runtime package version = %q, want 1.1.0", versionOut)
	}
}

func TestNPMRuntimePackageContractFiles(t *testing.T) {
	t.Parallel()
	root := RepoRoot(t)

	for _, trackedPath := range []string{
		"npm/plugin-kit-ai-runtime/package.json",
		"npm/plugin-kit-ai-runtime/README.md",
		"npm/plugin-kit-ai-runtime/index.js",
		"npm/plugin-kit-ai-runtime/index.d.ts",
	} {
		tracked := exec.Command("git", "ls-files", "--error-unmatch", trackedPath)
		tracked.Dir = root
		if out, err := tracked.CombinedOutput(); err != nil {
			t.Fatalf("npm runtime package file must be tracked in git (%s): %v\n%s", trackedPath, err, out)
		}
	}

	packageJSON := readRepoFile(t, root, "npm", "plugin-kit-ai-runtime", "package.json")
	for _, want := range []string{
		`"name": "plugin-kit-ai-runtime"`,
		`"version": "0.0.0-development"`,
		`"node": ">=20"`,
		`"types": "./index.d.ts"`,
		`"default": "./index.js"`,
	} {
		mustContain(t, packageJSON, want)
	}

	indexJS := readRepoFile(t, root, "npm", "plugin-kit-ai-runtime", "index.js")
	for _, want := range []string{
		"export const CLAUDE_STABLE_HOOKS",
		"export const CLAUDE_EXTENDED_HOOKS",
		"export class ClaudeApp",
		"export class CodexApp",
	} {
		mustContain(t, indexJS, want)
	}

	indexDTS := readRepoFile(t, root, "npm", "plugin-kit-ai-runtime", "index.d.ts")
	for _, want := range []string{
		"export declare const CLAUDE_STABLE_HOOKS",
		"export declare class ClaudeApp",
		"export declare class CodexApp",
	} {
		mustContain(t, indexDTS, want)
	}

	readme := readRepoFile(t, root, "npm", "plugin-kit-ai-runtime", "README.md")
	for _, want := range []string{
		"npm i plugin-kit-ai-runtime",
		"shared dependency instead of copying a local helper file",
		"Go is still the recommended path",
		"stable supported lane",
	} {
		mustContain(t, readme, want)
	}

	workflow := readRepoFile(t, root, ".github", "workflows", "npm-runtime-publish.yml")
	for _, want := range []string{
		"name: NPM Runtime Publish",
		"workflow_dispatch:",
		"plugin-kit-ai-runtime",
		"plugin-kit-ai-runtime npm prepublish smoke ok",
		"NPM_TOKEN",
		"npm publish --access public",
	} {
		mustContain(t, workflow, want)
	}
	mustNotContain(t, workflow, "workflow_run:")
}

func TestNPMRuntimePackageClaudeAndCodexSmoke(t *testing.T) {
	t.Parallel()
	requireNodeRuntime(t)

	root := RepoRoot(t)
	pkgRoot := filepath.Join(root, "npm", "plugin-kit-ai-runtime")
	appDir := t.TempDir()
	nodeModules := filepath.Join(appDir, "node_modules", "plugin-kit-ai-runtime")
	copyTree(t, pkgRoot, nodeModules)

	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{\"type\":\"module\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	codexScript := filepath.Join(appDir, "codex_app.mjs")
	if err := os.WriteFile(codexScript, []byte(`import { CodexApp, continue_ } from "plugin-kit-ai-runtime";

const app = new CodexApp().onNotify((event) => {
  if (event.client !== "codex-tui") {
    throw new Error(`+"`unexpected client: ${JSON.stringify(event)}`"+`);
  }
  return continue_();
});

process.exit(app.run());
`), 0o644); err != nil {
		t.Fatal(err)
	}

	codex := exec.Command("node", codexScript, "notify", `{"client":"codex-tui"}`)
	codex.Dir = appDir
	var codexStdout bytes.Buffer
	var codexStderr bytes.Buffer
	codex.Stdout = &codexStdout
	codex.Stderr = &codexStderr
	if err := codex.Run(); err != nil {
		t.Fatalf("npm runtime package codex smoke: %v\nstderr=%s", err, codexStderr.String())
	}
	if strings.TrimSpace(codexStdout.String()) != "" {
		t.Fatalf("codex stdout = %q, want empty", codexStdout.String())
	}

	claudeScript := filepath.Join(appDir, "claude_app.mjs")
	if err := os.WriteFile(claudeScript, []byte(`import { CLAUDE_STABLE_HOOKS, ClaudeApp, allow } from "plugin-kit-ai-runtime";

const app = new ClaudeApp({
  allowedHooks: [...CLAUDE_STABLE_HOOKS],
  usage: "claude_app.mjs <hook-name>"
}).onStop((event) => {
  if (event.hook_event_name !== "Stop") {
    throw new Error(`+"`unexpected hook payload: ${JSON.stringify(event)}`"+`);
  }
  return allow();
});

process.exit(app.run());
`), 0o644); err != nil {
		t.Fatal(err)
	}

	claude := exec.Command("node", claudeScript, "Stop")
	claude.Dir = appDir
	claude.Stdin = strings.NewReader(`{"hook_event_name":"Stop"}`)
	var claudeStdout bytes.Buffer
	var claudeStderr bytes.Buffer
	claude.Stdout = &claudeStdout
	claude.Stderr = &claudeStderr
	if err := claude.Run(); err != nil {
		t.Fatalf("npm runtime package claude smoke: %v\nstderr=%s", err, claudeStderr.String())
	}
	if strings.TrimSpace(claudeStdout.String()) != "{}" {
		t.Fatalf("claude stdout = %q, want {}", claudeStdout.String())
	}
}

func TestNPMRuntimePackagePackInstallAndUpgradeSmoke(t *testing.T) {
	t.Parallel()
	requireNodeRuntime(t)

	v1Root := copyNPMRuntimeAuthoringPackageToTemp(t)
	rewriteNPMRuntimeAuthoringPackageVersion(t, v1Root, "1.0.6")
	v2Root := copyNPMRuntimeAuthoringPackageToTemp(t)
	rewriteNPMRuntimeAuthoringPackageVersion(t, v2Root, "1.1.0")

	tgzV1 := strings.TrimSpace(mustRun(t, v1Root, "npm", "pack", "--silent"))
	consumer := t.TempDir()
	if err := os.WriteFile(filepath.Join(consumer, "package.json"), []byte("{\"type\":\"module\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, consumer, "npm", "install", filepath.Join(v1Root, tgzV1))
	versionOut := mustRun(t, consumer, "node", "--input-type=module", "-e", `import fs from "node:fs"; const pkg = JSON.parse(fs.readFileSync("./node_modules/plugin-kit-ai-runtime/package.json", "utf8")); console.log(pkg.version);`)
	if strings.TrimSpace(versionOut) != "1.0.6" {
		t.Fatalf("installed npm runtime package version = %q, want 1.0.6", versionOut)
	}
	mustRun(t, consumer, "node", "--input-type=module", "-e", `import { CodexApp, continue_ } from "plugin-kit-ai-runtime"; const app = new CodexApp().onNotify(() => continue_()); if (typeof app.run !== "function") throw new Error("missing run"); console.log("npm runtime consumer ok");`)

	tgzV2 := strings.TrimSpace(mustRun(t, v2Root, "npm", "pack", "--silent"))
	mustRun(t, consumer, "npm", "install", filepath.Join(v2Root, tgzV2))
	versionOut = mustRun(t, consumer, "node", "--input-type=module", "-e", `import fs from "node:fs"; const pkg = JSON.parse(fs.readFileSync("./node_modules/plugin-kit-ai-runtime/package.json", "utf8")); console.log(pkg.version);`)
	if strings.TrimSpace(versionOut) != "1.1.0" {
		t.Fatalf("upgraded npm runtime package version = %q, want 1.1.0", versionOut)
	}
}

func copyPythonRuntimeAuthoringPackageToTemp(t *testing.T) string {
	t.Helper()
	root := RepoRoot(t)
	dst := filepath.Join(t.TempDir(), "plugin-kit-ai-runtime-pypi")
	copyTree(t, filepath.Join(root, "python", "plugin-kit-ai-runtime"), dst)
	return dst
}

func rewritePythonRuntimeAuthoringPackageVersion(t *testing.T, packageRoot, version string) {
	t.Helper()
	path := filepath.Join(packageRoot, "src", "plugin_kit_ai_runtime", "__init__.py")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), `__version__ = "0.0.0.dev0"`, `__version__ = "`+version+`"`, 1)
	if updated == string(body) {
		t.Fatalf("failed to rewrite package version in %s", path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyNPMRuntimeAuthoringPackageToTemp(t *testing.T) string {
	t.Helper()
	root := RepoRoot(t)
	dst := filepath.Join(t.TempDir(), "plugin-kit-ai-runtime-npm")
	copyTree(t, filepath.Join(root, "npm", "plugin-kit-ai-runtime"), dst)
	return dst
}

func rewriteNPMRuntimeAuthoringPackageVersion(t *testing.T, packageRoot, version string) {
	t.Helper()
	path := filepath.Join(packageRoot, "package.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), `"version": "0.0.0-development"`, fmt.Sprintf(`"version": "%s"`, version), 1)
	if updated == string(body) {
		t.Fatalf("failed to rewrite package version in %s", path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requirePythonBuildBackend(t *testing.T) {
	t.Helper()
	cmd := exec.Command("python3", "-c", "import setuptools.build_meta")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("python build backend unavailable: %v\n%s", err, out)
	}
}

func mustRun(t *testing.T, dir, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", bin, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func firstMatch(t *testing.T, pattern string) string {
	t.Helper()
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("no files matched %s", pattern)
	}
	return matches[0]
}

func venvPythonPath(root string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "Scripts", "python.exe")
	}
	return filepath.Join(root, "bin", "python")
}
