package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	managementSDKReleaseWorkflow = "management-sdk-release.yml"
	managementSDKQualification   = "./.github/workflows/management-sdk.yml"
	managementSDKPackage         = "universal-agent-plugins-management-sdk"
)

// The SDK release publishes to npm with an OIDC token. These are the boundaries
// a refactor of the workflow must not loosen silently.
func validateManagementSDKReleaseBoundary(workflow releaseWorkflow) error {
	if err := validateManagementSDKJobGraph(workflow); err != nil {
		return err
	}
	if err := validateManagementSDKArtifactHandoff(workflow); err != nil {
		return err
	}
	return validateManagementSDKCommands(workflow)
}

func validateManagementSDKJobGraph(workflow releaseWorkflow) error {
	const dispatch = "success() && github.event_name == 'workflow_dispatch' && "
	expected := map[string]struct {
		needs       []string
		condition   string
		permissions map[string]string
	}{
		"verify-source":    {nil, "${{ github.event_name == 'workflow_dispatch' }}", map[string]string{"contents": "read"}},
		"qualify":          {[]string{"verify-source"}, "${{ " + dispatch + "needs.verify-source.result == 'success' }}", map[string]string{"contents": "read"}},
		"release-artifact": {[]string{"verify-source", "qualify"}, "${{ " + dispatch + "needs.verify-source.result == 'success' && needs.qualify.result == 'success' }}", map[string]string{"contents": "read", "id-token": "write", "attestations": "write"}},
		"publish":          {[]string{"release-artifact"}, "${{ " + dispatch + "inputs.publish == true && needs.release-artifact.result == 'success' }}", map[string]string{"contents": "read", "id-token": "write"}},
		"verify-public":    {[]string{"release-artifact", "publish"}, "${{ " + dispatch + "inputs.publish == true && needs.publish.result == 'success' }}", map[string]string{"contents": "read"}},
	}
	if len(workflow.Jobs) != len(expected) || !reflect.DeepEqual(workflow.Permissions, map[string]string{"contents": "read"}) {
		return fmt.Errorf("job set or workflow permissions")
	}
	for name, want := range expected {
		job, ok := workflow.Jobs[name]
		if !ok || !reflect.DeepEqual(releaseNeeds(job.Needs), want.needs) {
			return fmt.Errorf("%s dependencies", name)
		}
		if job.If != want.condition {
			return fmt.Errorf("%s reachability condition", name)
		}
		if !reflect.DeepEqual(effectiveReleasePermissions(workflow, name), want.permissions) {
			return fmt.Errorf("%s permissions", name)
		}
		for _, step := range job.Steps {
			//nolint:misspell // the GitHub context function uses the double-l spelling
			if step.Continue || strings.Contains(step.If, "always()") || strings.Contains(step.If, "failure()") || strings.Contains(step.If, "cancelled()") {
				return fmt.Errorf("%s step bypass", name)
			}
		}
		if wantEnvironment := name == "publish"; wantEnvironment != (job.Environment == "npm-management-sdk") || (!wantEnvironment && job.Environment != nil) {
			return fmt.Errorf("%s protected environment boundary", name)
		}
	}
	if workflow.Jobs["qualify"].Uses != managementSDKQualification {
		return fmt.Errorf("qualify must call the shared required-CI qualification")
	}
	if workflow.Concurrency.Group != "management-sdk-release-${{ inputs.version }}" || workflow.Concurrency.Cancel {
		return fmt.Errorf("release concurrency must be version-keyed and never cancel a run")
	}
	return nil
}

func validateManagementSDKArtifactHandoff(workflow releaseWorkflow) error {
	attested, uploaded, downloaded := false, false, false
	for _, step := range workflow.Jobs["release-artifact"].Steps {
		attested = attested || strings.HasPrefix(step.Uses, "actions/attest@") && step.With["subject-path"] == "${{ runner.temp }}/management-sdk/*.tgz"
		uploaded = uploaded || strings.HasPrefix(step.Uses, "actions/upload-artifact@") &&
			step.With["name"] == "management-sdk-${{ steps.pack.outputs.version }}" && step.With["if-no-files-found"] == "error"
	}
	for _, step := range workflow.Jobs["publish"].Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") {
			return fmt.Errorf("publish must handle only the attested bytes, not a source checkout")
		}
		downloaded = downloaded || strings.HasPrefix(step.Uses, "actions/download-artifact@") &&
			step.With["name"] == "management-sdk-${{ needs.release-artifact.outputs.version }}"
	}
	if !attested || !uploaded || !downloaded {
		return fmt.Errorf("attested release-artifact to publish handoff")
	}
	for _, output := range []string{"version", "commit", "package_name", "tarball_file", "tarball_sha256", "tarball_integrity", "tarball_shasum"} {
		if workflow.Jobs["release-artifact"].Outputs[output] != "${{ steps.pack.outputs."+output+" }}" {
			return fmt.Errorf("release-artifact output %s", output)
		}
	}
	return nil
}

func validateManagementSDKCommands(workflow releaseWorkflow) error {
	required := map[string][]string{
		"verify-source": {
			`test "${GITHUB_REF}" = "refs/tags/${tag}"`, `test "${GITHUB_SHA}" = "${commit}"`,
			`git merge-base --is-ancestor "${commit}" refs/remotes/origin/main`,
			`test "${package_name}" = ` + managementSDKPackage, `grep -q 'E404'`, `sort -V`,
		},
		"release-artifact": {`npm run test:consumer -- "--tarball=${out}/${tarball}"`, `sha256sum "${tarball}" > checksums.txt`},
		"publish": {
			`test -z "${NPM_TOKEN:-}"`, `test -z "${NODE_AUTH_TOKEN:-}"`, `test "${PACKAGE_NAME}" = ` + managementSDKPackage,
			`sha256sum --check --strict`, `grep -q 'E404'`,
			`npm publish "${dir}/${TARBALL}" --registry=https://registry.npmjs.org/ --access public --provenance --tag latest`,
		},
		"verify-public": {
			`npm run test:consumer -- "--registry-version=${VERSION}"`, `node scripts/verify-registry.ts`,
			`--tag "management-sdk-v${VERSION}" --commit "${COMMIT}"`, `--workflow .github/workflows/` + managementSDKReleaseWorkflow,
		},
	}
	for name, commands := range required {
		var scripts strings.Builder
		for _, step := range workflow.Jobs[name].Steps {
			scripts.WriteString(step.Run)
		}
		for _, command := range commands {
			if !strings.Contains(scripts.String(), command) {
				return fmt.Errorf("%s lacks %q", name, command)
			}
		}
	}
	return nil
}

func TestManagementSDKReleaseBoundary(t *testing.T) {
	workflow := parseReleaseWorkflow(t, managementSDKReleaseWorkflow)
	if err := validateManagementSDKReleaseBoundary(workflow); err != nil {
		t.Fatal(err)
	}
	var raw struct {
		On map[string]any `yaml:"on"`
	}
	body, err := os.ReadFile(filepath.Join(managementSDKRepositoryRoot(t), ".github/workflows", managementSDKReleaseWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(body, &raw); err != nil || len(raw.On) != 1 || raw.On["workflow_dispatch"] == nil {
		t.Fatalf("management SDK release must remain dispatch-only: %v", err)
	}
	//nolint:misspell // GitHub reports the double-l job result spelling
	for _, status := range []string{"failure", "cancelled", "skipped", ""} {
		values := map[string]any{
			"github.event_name": "workflow_dispatch", "inputs.publish": true,
			"needs.release-artifact.result": status, "needs.publish.result": status,
			//nolint:misspell // GitHub reports the double-l job result spelling
			"success": false, "failure": status == "failure", "cancelled": status == "cancelled", "always": true,
		}
		for _, name := range []string{"publish", "verify-public"} {
			if evaluateReleaseCondition(t, workflow.Jobs[name].If, values) {
				t.Fatalf("%s reachable after %q dependency", name, status)
			}
		}
	}
	qualified := map[string]any{
		"github.event_name": "workflow_dispatch", "inputs.publish": false,
		"needs.release-artifact.result": "success", "needs.publish.result": "success", "success": true,
	}
	for _, name := range []string{"publish", "verify-public"} {
		if evaluateReleaseCondition(t, workflow.Jobs[name].If, qualified) {
			t.Fatalf("%s reachable without publish=true", name)
		}
	}
	var required struct {
		Jobs map[string]struct {
			Uses string `yaml:"uses"`
		} `yaml:"jobs"`
	}
	body, err = os.ReadFile(filepath.Join(managementSDKRepositoryRoot(t), ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(body, &required); err != nil || required.Jobs["management-sdk"].Uses != managementSDKQualification {
		t.Fatalf("required CI and the release must share one management SDK qualification: %v", err)
	}
}

func TestManagementSDKReleaseMutationNegativeControls(t *testing.T) {
	mutations := map[string]func(*releaseWorkflow){
		"publish needs": func(w *releaseWorkflow) { job := w.Jobs["publish"]; job.Needs = nil; w.Jobs["publish"] = job },
		"artifact needs": func(w *releaseWorkflow) {
			job := w.Jobs["release-artifact"]
			job.Needs = []any{"verify-source"}
			w.Jobs["release-artifact"] = job
		},
		"status bypass": func(w *releaseWorkflow) {
			job := w.Jobs["publish"]
			job.If = "${{ always() }}"
			w.Jobs["publish"] = job
		},
		"permissions": func(w *releaseWorkflow) {
			job := w.Jobs["publish"]
			job.Permissions = map[string]string{"contents": "write", "id-token": "write"}
			w.Jobs["publish"] = job
		},
		"environment": func(w *releaseWorkflow) { job := w.Jobs["publish"]; job.Environment = nil; w.Jobs["publish"] = job },
		"qualification": func(w *releaseWorkflow) {
			job := w.Jobs["qualify"]
			job.Uses = "./.github/workflows/other.yml"
			w.Jobs["qualify"] = job
		},
		"cancel run": func(w *releaseWorkflow) { w.Concurrency.Cancel = true },
		"token":      func(w *releaseWorkflow) { replaceManagementSDKRun(w, "publish", `test -z "${NPM_TOKEN:-}"`, "true") },
		"overwrite":  func(w *releaseWorkflow) { replaceManagementSDKRun(w, "publish", `grep -q 'E404'`, "true") },
		"tag binding": func(w *releaseWorkflow) {
			replaceManagementSDKRun(w, "verify-source", `"refs/tags/${tag}"`, `"${GITHUB_REF}"`)
		},
		"provenance": func(w *releaseWorkflow) {
			replaceManagementSDKRun(w, "publish", "--provenance --tag latest", "--tag latest")
		},
		"verification": func(w *releaseWorkflow) {
			replaceManagementSDKRun(w, "verify-public", "--registry-version", "--tarball")
		},
		"artifact": func(w *releaseWorkflow) {
			job := w.Jobs["publish"]
			for i := range job.Steps {
				if strings.HasPrefix(job.Steps[i].Uses, "actions/download-artifact@") {
					job.Steps[i].With["name"] = "mutable"
				}
			}
			w.Jobs["publish"] = job
		},
	}
	for name, mutate := range mutations {
		workflow := parseReleaseWorkflow(t, managementSDKReleaseWorkflow)
		mutate(&workflow)
		if validateManagementSDKReleaseBoundary(workflow) == nil {
			t.Fatalf("negative control accepted %s mutation", name)
		}
	}
}

func replaceManagementSDKRun(workflow *releaseWorkflow, name, old, replacement string) {
	job := workflow.Jobs[name]
	for i := range job.Steps {
		job.Steps[i].Run = strings.ReplaceAll(job.Steps[i].Run, old, replacement)
	}
	workflow.Jobs[name] = job
}

// The workflow refuses any other package name, so the manifest must agree, and
// the checked-in version must be an exact stable release, never a placeholder.
func TestManagementSDKManifestMatchesReleaseWorkflow(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(managementSDKRepositoryRoot(t), "sdks/typescript/package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Name          string            `json:"name"`
		Version       string            `json:"version"`
		Files         []string          `json:"files"`
		PublishConfig map[string]any    `json:"publishConfig"`
		Repository    map[string]string `json:"repository"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Name != managementSDKPackage || !regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`).MatchString(manifest.Version) {
		t.Fatalf("management SDK manifest identity %s@%s", manifest.Name, manifest.Version)
	}
	if !reflect.DeepEqual(manifest.Files, []string{"dist", "README.md", "LICENSE"}) ||
		!reflect.DeepEqual(manifest.PublishConfig, map[string]any{"access": "public", "provenance": true}) {
		t.Fatal("management SDK must publish only its build output with public provenance")
	}
	// npm provenance requires repository.url to match the publishing repository.
	if !reflect.DeepEqual(manifest.Repository, map[string]string{"type": "git", "url": "git+https://github.com/777genius/universal-agent-plugins.git", "directory": "sdks/typescript"}) {
		t.Fatal("management SDK repository metadata must match the publishing repository")
	}
}

func managementSDKRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(source), "../../..")
}
