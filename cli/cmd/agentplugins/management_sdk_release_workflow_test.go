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
	managementSDKPublishCommand  = `npm publish "${dir}/${TARBALL}" --registry=https://registry.npmjs.org/ --access public --provenance --tag latest`
)

type managementSDKJob struct {
	needs       []string
	condition   string
	permissions map[string]string
}

// The SDK release publishes to npm with an OIDC token. These are the boundaries
// a refactor of the workflow must not loosen silently.
func managementSDKExpectedJobs() map[string]managementSDKJob {
	const dispatch = "${{ success() && github.event_name == 'workflow_dispatch' && "
	read := map[string]string{"contents": "read"}
	return map[string]managementSDKJob{
		"verify-source": {nil, "${{ github.event_name == 'workflow_dispatch' }}", map[string]string{"actions": "read", "contents": "read"}},
		"qualify":       {[]string{"verify-source"}, dispatch + "needs.verify-source.result == 'success' }}", read},
		"build":         {[]string{"verify-source", "qualify"}, dispatch + "needs.verify-source.result == 'success' && needs.qualify.result == 'success' }}", read},
		"attest":        {[]string{"build"}, dispatch + "needs.build.result == 'success' }}", map[string]string{"contents": "read", "id-token": "write", "attestations": "write"}},
		"publish":       {[]string{"build", "attest"}, dispatch + "inputs.publish == true && needs.build.result == 'success' && needs.attest.result == 'success' }}", map[string]string{"actions": "read", "contents": "read", "id-token": "write", "attestations": "read"}},
		"verify-public": {[]string{"build", "publish"}, dispatch + "inputs.publish == true && needs.publish.result == 'success' }}", read},
	}
}

// Complete script lines each job must keep unchanged, so a guard cannot be
// weakened by a suffix such as `|| true` or a different pipeline tail.
func managementSDKRequiredLines() map[string][]string {
	const (
		absent  = `elif ! grep -q 'E404' "${err}"; then`
		forward = `test "$(printf '%s\n%s\n' "${latest}" "${VERSION}" | sort -V | tail -n 1)" = "${VERSION}"`
		exact   = `printf '%s  %s\n' "${TARBALL_SHA256}" "${dir}/${TARBALL}" | sha256sum --check --strict`
		name    = `test "${PACKAGE_NAME}" = ` + managementSDKPackage
		product = ` management-sdk`
		// The protected-environment guard, in verify-source and again in publish.
		reviewed = `jq -e '.can_admins_bypass == false and .deployment_branch_policy.custom_branch_policies == true and ([.protection_rules[] | select(.type == "required_reviewers" and (.reviewers | length) > 0)] | length) == 1' <<<"${environment}" >/dev/null &&`
		tagOnly  = `jq -e '[.branch_policies[] | {name, type}] == [{"name": "management-sdk-v*", "type": "tag"}]' <<<"${policies}" >/dev/null; then`
	)
	return map[string][]string{
		"verify-source": {
			`test "${GITHUB_REF}" = "refs/tags/${tag}"`, `test "${GITHUB_SHA}" = "${commit}"`,
			`test "$(git rev-list -n 1 "refs/tags/${tag}")" = "${commit}"`,
			`git fetch --no-tags --force origin main:refs/remotes/origin/main`,
			`git merge-base --is-ancestor "${commit}" refs/remotes/origin/main`,
			`test "${package_name}" = ` + managementSDKPackage, absent, forward, reviewed, tagOnly, `protected=true`,
			`if [[ "${protected}" != true && "${PUBLISH}" == true ]]; then`,
			`if [[ "${PUBLISH}" == true ]] && ! npm view "${package_name}" name --registry=https://registry.npmjs.org/ >/dev/null; then`,
		},
		"build": {
			`test "$(git rev-parse HEAD)" = "${COMMIT}"`, `npm run build`,
			`npm run test:consumer -- "--tarball=${out}/${tarball}"`, `sha256sum "${tarball}" > checksums.txt`,
		},
		"attest": {exact},
		"publish": {
			reviewed, tagOnly, `protected=true`, `if [[ "${protected}" != true ]]; then`,
			`for (const [claim, value] of Object.entries(expected)) {`,
			"if (claims[claim] !== value) throw new Error(`OIDC claim ${claim} is ${JSON.stringify(claims[claim])}, expected ${JSON.stringify(value)}`);",
			`environment: "npm-management-sdk", repository: env.GITHUB_REPOSITORY, ref: tag, sha: env.COMMIT,`,
			"workflow_ref: `${env.GITHUB_REPOSITORY}/.github/workflows/" + managementSDKReleaseWorkflow + "@${tag}`,",
			`runner_environment: "github-hosted",`,
			`gh attestation verify "${dir}/${TARBALL}" --repo "${GITHUB_REPOSITORY}" \`,
			`--signer-workflow "github.com/${GITHUB_REPOSITORY}/.github/workflows/` + managementSDKReleaseWorkflow + `" \`,
			`--source-ref "refs/tags/management-sdk-v${VERSION}" --source-digest "${COMMIT}" --deny-self-hosted-runners`, `test -z "${NPM_TOKEN:-}"`, `test -z "${NODE_AUTH_TOKEN:-}"`, name, exact,
			`if ! existing="$(npm view "${PACKAGE_NAME}" name --registry=https://registry.npmjs.org/)" || [[ "${existing}" != "${PACKAGE_NAME}" ]]; then`,
			absent, forward, managementSDKPublishCommand,
		},
		"verify-public": {
			`npm --prefix sdks/typescript run test:consumer -- "--registry-version=${VERSION}"`,
			`node "${contract}" metadata "${root}/metadata.json" "${VERSION}" "${TARBALL_INTEGRITY}" "${TARBALL_SHASUM}"` + product,
			`test "$(npm view "${PACKAGE_NAME}" dist-tags.latest)" = "${VERSION}"`,
			`node "${contract}" audit "${root}/audit.json" "${root}/attestation.json" "${VERSION}"` + product,
			`node "${contract}" attestation "${root}/attestation.json" "${VERSION}" "${TARBALL_INTEGRITY}" "management-sdk-v${VERSION}" "${COMMIT}"` + product,
			`node "${contract}" download "${root}/pack.json" "${root}/packed" "${VERSION}" "${TARBALL_INTEGRITY}" "${TARBALL_SHASUM}"` + product,
			`printf '%s  %s\n' "${TARBALL_SHA256}" "${root}/packed/${PACKAGE_NAME}-${VERSION}.tgz" | sha256sum --check --strict`,
		},
	}
}

func validateManagementSDKReleaseBoundary(workflow releaseWorkflow) error {
	for _, validate := range []func(releaseWorkflow) error{
		validateManagementSDKJobGraph, validateManagementSDKArtifactHandoff,
		validateManagementSDKCommands, validateManagementSDKPublishOrder,
	} {
		if err := validate(workflow); err != nil {
			return err
		}
	}
	return nil
}

func validateManagementSDKJobGraph(workflow releaseWorkflow) error {
	expected := managementSDKExpectedJobs()
	if len(workflow.Jobs) != len(expected) || !reflect.DeepEqual(workflow.Permissions, map[string]string{"contents": "read"}) {
		return fmt.Errorf("job set or workflow permissions")
	}
	for name, want := range expected {
		job, ok := workflow.Jobs[name]
		switch {
		case !ok || !reflect.DeepEqual(releaseNeeds(job.Needs), want.needs):
			return fmt.Errorf("%s dependencies", name)
		case job.If != want.condition:
			return fmt.Errorf("%s reachability condition", name)
		case !reflect.DeepEqual(effectiveReleasePermissions(workflow, name), want.permissions):
			return fmt.Errorf("%s permissions", name)
		case (name == "publish") != (job.Environment == "npm-management-sdk") || name != "publish" && job.Environment != nil:
			return fmt.Errorf("%s protected environment boundary", name)
		case name != "qualify" && job.RunsOn != "ubuntu-24.04":
			// OIDC signing and publishing require a GitHub-hosted runner (docs/CI_RUNNERS.md).
			return fmt.Errorf("%s must run on the GitHub-hosted ubuntu-24.04 runner", name)
		}
		for _, step := range job.Steps {
			//nolint:misspell // the GitHub context function uses the double-l spelling
			if step.Continue || strings.Contains(step.If, "always()") || strings.Contains(step.If, "failure()") || strings.Contains(step.If, "cancelled()") {
				return fmt.Errorf("%s step bypass", name)
			}
			if step.Run != "" && !strings.HasPrefix(step.Run, "set -euo pipefail\n") {
				return fmt.Errorf("%s script must start with set -euo pipefail", name)
			}
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
	const artifact = "management-sdk-${{ needs.build.outputs.version }}"
	checkouts := map[string]string{"build": "${{ needs.verify-source.outputs.commit }}", "verify-public": "${{ needs.build.outputs.commit }}"}
	for name, ref := range checkouts {
		pinned := false
		for _, step := range workflow.Jobs[name].Steps {
			pinned = pinned || strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["ref"] == ref && step.With["persist-credentials"] == false
		}
		if !pinned {
			return fmt.Errorf("%s must check out the verified commit without persisted credentials", name)
		}
	}
	uploaded, attested := false, false
	for _, step := range workflow.Jobs["build"].Steps {
		uploaded = uploaded || strings.HasPrefix(step.Uses, "actions/upload-artifact@") &&
			step.With["name"] == "management-sdk-${{ steps.pack.outputs.version }}" && step.With["if-no-files-found"] == "error"
	}
	// Jobs that can mint an OIDC token handle only the built bytes: no source
	// checkout, and the signing job runs nothing besides download and attest.
	for _, name := range []string{"attest", "publish"} {
		downloaded := false
		for _, step := range workflow.Jobs[name].Steps {
			if strings.HasPrefix(step.Uses, "actions/checkout@") {
				return fmt.Errorf("%s must handle only the built bytes, not a source checkout", name)
			}
			if name == "attest" && step.Uses != "" && !strings.HasPrefix(step.Uses, "actions/download-artifact@") && !strings.HasPrefix(step.Uses, "actions/attest@") {
				return fmt.Errorf("attest runs an unexpected action %s", step.Uses)
			}
			downloaded = downloaded || strings.HasPrefix(step.Uses, "actions/download-artifact@") && step.With["name"] == artifact
			attested = attested || name == "attest" && strings.HasPrefix(step.Uses, "actions/attest@") &&
				step.With["subject-path"] == "${{ runner.temp }}/management-sdk/*.tgz"
		}
		if !downloaded {
			return fmt.Errorf("%s must use the build artifact", name)
		}
	}
	if !uploaded || !attested {
		return fmt.Errorf("build to attest artifact handoff")
	}
	for _, output := range []string{"version", "commit", "package_name", "tarball_file", "tarball_sha256", "tarball_integrity", "tarball_shasum"} {
		if workflow.Jobs["build"].Outputs[output] != "${{ steps.pack.outputs."+output+" }}" {
			return fmt.Errorf("build output %s", output)
		}
	}
	return nil
}

func validateManagementSDKCommands(workflow releaseWorkflow) error {
	for name, lines := range managementSDKRequiredLines() {
		present := map[string]bool{}
		for _, step := range workflow.Jobs[name].Steps {
			for _, line := range strings.Split(step.Run, "\n") {
				present[strings.TrimSpace(line)] = true
			}
		}
		for _, line := range lines {
			if !present[line] {
				return fmt.Errorf("%s lacks the exact line %q", name, line)
			}
		}
	}
	return nil
}

// npm publish must be the last step, after the OIDC identity and attestation checks.
func validateManagementSDKPublishOrder(workflow releaseWorkflow) error {
	steps := workflow.Jobs["publish"].Steps
	identity, attestation := -1, -1
	for i, step := range steps {
		if strings.Contains(step.Run, `environment: "npm-management-sdk"`) {
			identity = i
		}
		if strings.Contains(step.Run, "gh attestation verify") {
			attestation = i
		}
	}
	last := len(steps) - 1
	if last < 0 || !strings.Contains(steps[last].Run, managementSDKPublishCommand) || identity < 0 || identity >= last || attestation < 0 || attestation >= last {
		return fmt.Errorf("publish must verify the OIDC identity and attestation before npm publish")
	}
	for _, step := range steps[:last] {
		if strings.Contains(step.Run, "npm publish") {
			return fmt.Errorf("npm publish before the identity and attestation checks")
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
	if err := yaml.Unmarshal(readManagementSDKWorkflow(t, managementSDKReleaseWorkflow), &raw); err != nil || len(raw.On) != 1 || raw.On["workflow_dispatch"] == nil {
		t.Fatalf("management SDK release must remain dispatch-only: %v", err)
	}
	//nolint:misspell // GitHub reports the double-l job result spelling
	for _, status := range []string{"failure", "cancelled", "skipped", ""} {
		// success() is checked both ways so the explicit result guards are exercised too.
		for _, succeeded := range []bool{false, true} {
			values := map[string]any{
				"github.event_name": "workflow_dispatch", "inputs.publish": true, "success": succeeded,
				"needs.build.result": status, "needs.attest.result": status, "needs.publish.result": status,
			}
			for _, name := range []string{"publish", "verify-public"} {
				if evaluateReleaseCondition(t, workflow.Jobs[name].If, values) {
					t.Fatalf("%s reachable after %q dependency (success()=%v)", name, status, succeeded)
				}
			}
		}
	}
	qualified := map[string]any{
		"github.event_name": "workflow_dispatch", "inputs.publish": false,
		"needs.build.result": "success", "needs.attest.result": "success", "needs.publish.result": "success", "success": true,
	}
	for _, name := range []string{"publish", "verify-public"} {
		if evaluateReleaseCondition(t, workflow.Jobs[name].If, qualified) {
			t.Fatalf("%s reachable without publish=true", name)
		}
	}
}

// Required CI and the release share one qualification on all three systems.
func TestManagementSDKSharedQualification(t *testing.T) {
	var required struct {
		Jobs map[string]struct {
			Uses string `yaml:"uses"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(readManagementSDKWorkflow(t, "ci.yml"), &required); err != nil || required.Jobs["management-sdk"].Uses != managementSDKQualification {
		t.Fatalf("required CI must call the shared management SDK qualification: %v", err)
	}
	var shared struct {
		On   map[string]any `yaml:"on"`
		Jobs map[string]struct {
			Strategy struct {
				Matrix struct {
					Runner []string `yaml:"runner"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct {
				If       string `yaml:"if"`
				Continue bool   `yaml:"continue-on-error"`
				Run      string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(readManagementSDKWorkflow(t, filepath.Base(managementSDKQualification)), &shared); err != nil || len(shared.On) != 1 || len(shared.Jobs) != 1 {
		t.Fatalf("shared qualification must be one workflow_call job: %v", err)
	}
	qualify := shared.Jobs["qualify"]
	runners := qualify.Strategy.Matrix.Runner
	if _, called := shared.On["workflow_call"]; !called || len(runners) != 3 || !strings.Contains(runners[0], "'ubuntu-latest'") || runners[1] != "macos-15" || runners[2] != "windows-2025" {
		t.Fatalf("shared qualification must run on Linux, macOS and Windows: %v", runners)
	}
	scripts := make([]string, 0, len(qualify.Steps))
	for _, step := range qualify.Steps {
		if step.Continue || step.If != "" && step.If != "runner.os == 'Linux'" {
			t.Fatalf("shared qualification step bypass: %q", step.If)
		}
		scripts = append(scripts, strings.Split(step.Run, "\n")...)
	}
	for _, command := range []string{
		"npm ci --prefix sdks/typescript --ignore-scripts --no-audit --no-fund", "node contracts/management/generate.ts --check",
		"npm --prefix sdks/typescript run build", "npm --prefix sdks/typescript run typecheck",
		"npm --prefix sdks/typescript test", "npm --prefix sdks/typescript run test:consumer",
	} {
		if !containsManagementSDKLine(scripts, command) {
			t.Fatalf("shared qualification lacks %q", command)
		}
	}
}

func containsManagementSDKLine(lines []string, want string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

func TestManagementSDKReleaseMutationNegativeControls(t *testing.T) {
	for name, mutate := range managementSDKMutations() {
		workflow := parseReleaseWorkflow(t, managementSDKReleaseWorkflow)
		mutate(&workflow)
		if validateManagementSDKReleaseBoundary(workflow) == nil {
			t.Fatalf("negative control accepted %s mutation", name)
		}
	}
}

func managementSDKMutations() map[string]func(*releaseWorkflow) {
	job := func(name string, change func(*releaseJob)) func(*releaseWorkflow) {
		return func(w *releaseWorkflow) { value := w.Jobs[name]; change(&value); w.Jobs[name] = value }
	}
	run := func(name, old, replacement string) func(*releaseWorkflow) {
		return job(name, func(j *releaseJob) {
			for i := range j.Steps {
				j.Steps[i].Run = strings.ReplaceAll(j.Steps[i].Run, old, replacement)
			}
		})
	}
	return map[string]func(*releaseWorkflow){
		"publish needs":          job("publish", func(j *releaseJob) { j.Needs = nil }),
		"build needs":            job("build", func(j *releaseJob) { j.Needs = []any{"verify-source"} }),
		"publish without attest": job("publish", func(j *releaseJob) { j.Needs = []any{"build"} }),
		"build signing":          job("build", func(j *releaseJob) { j.Permissions = map[string]string{"contents": "read", "id-token": "write"} }),
		"publish permissions":    job("publish", func(j *releaseJob) { j.Permissions = map[string]string{"contents": "write", "id-token": "write"} }),
		"status bypass":          job("publish", func(j *releaseJob) { j.If = "${{ always() }}" }),
		"environment":            job("publish", func(j *releaseJob) { j.Environment = nil }),
		"self-hosted signing":    job("attest", func(j *releaseJob) { j.RunsOn = "self-hosted" }),
		"qualification":          job("qualify", func(j *releaseJob) { j.Uses = "./.github/workflows/other.yml" }),
		"unpinned build":         job("build", func(j *releaseJob) { delete(j.Steps[0].With, "ref") }),
		"attest checkout": job("attest", func(j *releaseJob) {
			j.Steps = append(j.Steps, j.Steps[0])
			j.Steps[len(j.Steps)-1].Uses = "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"
		}),
		"publish order": job("publish", func(j *releaseJob) {
			last := len(j.Steps) - 1
			j.Steps[last-1], j.Steps[last] = j.Steps[last], j.Steps[last-1]
		}),
		"artifact": job("publish", func(j *releaseJob) {
			for i := range j.Steps {
				if strings.HasPrefix(j.Steps[i].Uses, "actions/download-artifact@") {
					j.Steps[i].With["name"] = "mutable"
				}
			}
		}),
		"cancel run":        func(w *releaseWorkflow) { w.Concurrency.Cancel = true },
		"environment claim": run("publish", `environment: "npm-management-sdk"`, `environment: claims.environment`),
		"attestation check": run("publish", "gh attestation verify", "true"),
		"token":             run("publish", `test -z "${NPM_TOKEN:-}"`, "true"),
		"overwrite":         run("publish", `elif ! grep -q 'E404' "${err}"; then`, `elif grep -q 'E404' "${err}" && false; then`),
		"publish suffix":    run("publish", managementSDKPublishCommand, managementSDKPublishCommand+" || true"),
		"provenance":        run("publish", "--provenance --tag latest", "--tag latest"),
		"latest backwards":  run("publish", "sort -V | tail -n 1", "sort -V | head -n 1"),
		"missing package":   run("publish", `[[ "${existing}" != "${PACKAGE_NAME}" ]]`, "false"),
		"no pipefail":       run("publish", "set -euo pipefail", "set -e"),
		"tag binding":       run("verify-source", `"refs/tags/${tag}"`, `"${GITHUB_REF}"`),
		"main ref":          run("verify-source", "origin main:refs/remotes/origin/main", "origin feature:refs/remotes/origin/main"),
		"environment guard": run("verify-source", `"management-sdk-v*", "type": "tag"`, `"*", "type": "branch"`),
		"admin bypass":      run("publish", ".can_admins_bypass == false and", "(.can_admins_bypass == false or true) and"),
		"guard skipped":     run("verify-source", `&& "${PUBLISH}" == true ]]; then`, `&& "${PUBLISH}" == never ]]; then`),
		"guard ignored":     run("publish", `if [[ "${protected}" != true ]]; then`, `if [[ "${protected}" == maybe ]]; then`),
		"claims ignored":    run("publish", "if (claims[claim] !== value) throw", "if (false) throw"),
		"attestation ref":   run("publish", `--source-ref "refs/tags/management-sdk-v${VERSION}" `, ""),
		"skipped consumer":  run("build", `npm run test:consumer -- "--tarball=${out}/${tarball}"`, `: npm run test:consumer`),
		"verification":      run("verify-public", "--registry-version", "--tarball"),
		"contract product":  run("verify-public", `"${COMMIT}" management-sdk`, `"${COMMIT}"`),
	}
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

func readManagementSDKWorkflow(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(managementSDKRepositoryRoot(t), ".github/workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func managementSDKRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(source), "../../..")
}
