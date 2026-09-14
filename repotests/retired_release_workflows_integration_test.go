package pluginkitairepo_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type retiredWorkflowContract struct {
	Jobs map[string]retiredWorkflowJob `yaml:"jobs"`
}

type retiredWorkflowJob struct {
	If          string            `yaml:"if"`
	Needs       any               `yaml:"needs"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []retiredStep     `yaml:"steps"`
}

type retiredStep struct {
	Name  string `yaml:"name"`
	Shell string `yaml:"shell"`
	Run   string `yaml:"run"`
}

func TestRetiredReleaseWorkflowGraphPermissionsAndFailureIsolationArePreserved(t *testing.T) {
	root := RepoRoot(t)
	history := filepath.Join(root, "docs", "history", "plugin-kit-ai-release-workflows")
	workflows := map[string]retiredWorkflowContract{}
	bodies := map[string]string{}
	jobs := map[string]string{
		"release-assets.yml":                  "goreleaser",
		"release-preflight.yml":               "preflight",
		"homebrew-tap.yml":                    "update-homebrew-tap",
		"npm-publish.yml":                     "publish-npm",
		"pypi-publish.yml":                    "publish-pypi",
		"agentplugins-paired-release.yml":     "paired-sign-and-promote",
		"agentplugins-paired-npm-publish.yml": "paired_publish",
	}
	for name, requiredJob := range jobs {
		body := readRepoFile(t, history, name)
		var workflow retiredWorkflowContract
		if err := yaml.Unmarshal([]byte(body), &workflow); err != nil {
			t.Fatalf("parse retired workflow %s: %v", name, err)
		}
		if _, ok := workflow.Jobs[requiredJob]; !ok {
			t.Fatalf("retired workflow %s lost job %q", name, requiredJob)
		}
		bodies[name], workflows[name] = body, workflow
	}

	for _, name := range []string{"homebrew-tap.yml", "npm-publish.yml", "pypi-publish.yml"} {
		body, job := bodies[name], workflows[name].Jobs[jobs[name]]
		mustContain(t, body, "workflow_run:\n    workflows: [\"Release Assets\"]\n    types: [completed]")
		mustContain(t, body, "workflow_dispatch:")
		mustContain(t, job.If, "github.event_name == 'workflow_dispatch'")
		mustContain(t, job.If, "github.event.workflow_run.conclusion == 'success'")
	}
	mustContain(t, workflows["npm-publish.yml"].Jobs["publish-npm"].If, "vars.NPM_PUBLISH_READY == 'true'")
	mustContain(t, workflows["pypi-publish.yml"].Jobs["publish-pypi"].If, "vars.PYPI_TRUSTED_PUBLISHING_READY == 'true'")

	assertPermissions(t, "release-assets.yml/goreleaser", workflows["release-assets.yml"].Jobs["goreleaser"].Permissions,
		map[string]string{"contents": "write", "id-token": "write", "attestations": "write", "artifact-metadata": "write"})
	assertPermissions(t, "release-preflight.yml/preflight", workflows["release-preflight.yml"].Jobs["preflight"].Permissions,
		map[string]string{"contents": "read"})
	for _, item := range []struct{ file, job string }{{"npm-publish.yml", "publish-npm"}, {"pypi-publish.yml", "publish-pypi"}} {
		assertPermissions(t, item.file+"/"+item.job, workflows[item.file].Jobs[item.job].Permissions,
			map[string]string{"contents": "read", "id-token": "write"})
	}

	pairedRelease := workflows["agentplugins-paired-release.yml"]
	for admission, contract := range map[string]struct {
		signer      string
		mode        string
		permissions map[string]string
	}{
		"paired-promotion-admission": {"paired-sign-and-promote", "paired-promotion", map[string]string{
			"contents": "write", "actions": "read", "id-token": "write", "attestations": "write", "artifact-metadata": "write",
		}},
		"milestone-a-promotion-admission": {"milestone-a-sign-and-promote", "milestone-a-paired-promotion", map[string]string{
			"contents": "write", "actions": "read", "id-token": "write", "attestations": "write", "artifact-metadata": "write",
		}},
		"paired_input_admission": {"paired_input_attestation", "paired-input-provenance", map[string]string{
			"contents": "read", "actions": "read", "id-token": "write", "attestations": "write",
		}},
	} {
		assertNeeds(t, contract.signer, pairedRelease.Jobs[contract.signer].Needs, admission)
		mustContain(t, pairedRelease.Jobs[contract.signer].If, "workflow_dispatch")
		mustContain(t, pairedRelease.Jobs[contract.signer].If, "inputs.producer_mode == '"+contract.mode+"'")
		assertPermissions(t, contract.signer, pairedRelease.Jobs[contract.signer].Permissions, contract.permissions)
		assertPermissions(t, admission, pairedRelease.Jobs[admission].Permissions,
			map[string]string{"contents": "read", "actions": "read"})
	}
	pairedNPM := workflows["agentplugins-paired-npm-publish.yml"]
	assertNeeds(t, "paired_stage_attestation", pairedNPM.Jobs["paired_stage_attestation"].Needs, "paired_stage")
	mustContain(t, pairedNPM.Jobs["paired_stage_attestation"].If, "needs.paired_stage.result == 'success'")
	assertPermissions(t, "paired_stage", pairedNPM.Jobs["paired_stage"].Permissions,
		map[string]string{"contents": "read", "actions": "read", "attestations": "read"})
	assertPermissions(t, "paired_stage_attestation", pairedNPM.Jobs["paired_stage_attestation"].Permissions,
		map[string]string{"contents": "read", "actions": "read", "id-token": "write", "attestations": "write"})
	assertNeeds(t, "paired_publish_prepare", pairedNPM.Jobs["paired_publish_prepare"].Needs, "dispatch_contract")
	assertNeeds(t, "paired_publish", pairedNPM.Jobs["paired_publish"].Needs, "paired_publish_prepare")
	mustContain(t, pairedNPM.Jobs["paired_publish"].If, "needs.paired_publish_prepare.result == 'success'")
	assertPermissions(t, "paired_publish_prepare", pairedNPM.Jobs["paired_publish_prepare"].Permissions,
		map[string]string{"contents": "read", "actions": "read", "attestations": "read"})
	assertPermissions(t, "paired_publish", pairedNPM.Jobs["paired_publish"].Permissions,
		map[string]string{"contents": "read", "actions": "read", "attestations": "read", "id-token": "write"})
	mustContain(t, bodies["agentplugins-paired-npm-publish.yml"], "fail-fast: false")
	mustContain(t, bodies["agentplugins-paired-npm-publish.yml"], "environment: npm-agentplugins")
	mustContain(t, bodies["agentplugins-paired-npm-publish.yml"], "environment: npm-plugin-kit-ai")
}

func TestRetiredReleaseWorkflowShellSyntaxDigestAndAttestationArePreserved(t *testing.T) {
	root := RepoRoot(t)
	history := filepath.Join(root, "docs", "history", "plugin-kit-ai-release-workflows")
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is required to syntax-check preserved workflow shell")
	}
	for _, name := range []string{"release-assets.yml", "release-preflight.yml", "homebrew-tap.yml", "npm-publish.yml", "pypi-publish.yml", "agentplugins-paired-release.yml", "agentplugins-paired-npm-publish.yml"} {
		body := readRepoFile(t, history, name)
		var workflow retiredWorkflowContract
		if err := yaml.Unmarshal([]byte(body), &workflow); err != nil {
			t.Fatalf("parse retired workflow %s: %v", name, err)
		}
		for jobName, job := range workflow.Jobs {
			for index, step := range job.Steps {
				if step.Run == "" || (step.Shell != "" && step.Shell != "bash") {
					continue
				}
				file := filepath.Join(t.TempDir(), fmt.Sprintf("%s-%d.bash", jobName, index))
				if err := os.WriteFile(file, []byte(step.Run), 0o600); err != nil {
					t.Fatal(err)
				}
				if output, err := exec.Command(bash, "-n", file).CombinedOutput(); err != nil {
					t.Fatalf("retired workflow %s job %s step %q has invalid shell syntax: %v\n%s", name, jobName, step.Name, err, output)
				}
			}
		}
	}

	releaseAssets := readRepoFile(t, history, "release-assets.yml")
	mustAppearBefore(t, releaseAssets, "goreleaser/goreleaser-action@v7", "actions/attest@v4")
	for _, subject := range []string{"dist/plugin-kit-ai_*", "dist/checksums.txt"} {
		mustContain(t, releaseAssets, subject)
	}
	for _, name := range []string{"npm-publish.yml", "pypi-publish.yml"} {
		body := readRepoFile(t, history, name)
		mustAppearBefore(t, body, "Validate published release assets", map[string]string{
			"npm-publish.yml": "Publish npm package", "pypi-publish.yml": "Publish to PyPI",
		}[name])
		mustContain(t, body, `curl -fsSL "${base}/checksums.txt" -o checksums.txt`)
		for _, target := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64", "windows_amd64", "windows_arm64"} {
			mustContain(t, body, "plugin-kit-ai_${version}_"+target)
		}
		mustContain(t, body, `grep -q " ${asset}\$" checksums.txt`)
	}
	pairedRelease := readRepoFile(t, history, "agentplugins-paired-release.yml")
	mustAppearBefore(t, pairedRelease, "paired-promotion-admission:", "paired-sign-and-promote:")
	mustAppearBefore(t, pairedRelease, "milestone-a-promotion-admission:", "milestone-a-sign-and-promote:")
	mustAppearBefore(t, pairedRelease, "paired_input_admission:", "paired_input_attestation:")
	mustContain(t, pairedRelease, "actions/attest@")
	pairedNPM := readRepoFile(t, history, "agentplugins-paired-npm-publish.yml")
	mustAppearBefore(t, pairedNPM, "paired_stage:", "paired_stage_attestation:")
	mustAppearBefore(t, pairedNPM, "paired_publish_prepare:", "paired_publish:")
	mustContain(t, pairedNPM, "artifact_digest: ${{ steps.upload.outputs.artifact-digest }}")
	mustContain(t, pairedNPM, `[[ "${actual}" =~ ^sha256:[0-9a-f]{64}$ ]]`)
	mustContain(t, pairedNPM, "actions/attest@")
}

func assertPermissions(t *testing.T, label string, actual, expected map[string]string) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("%s permissions = %#v, want exactly %#v", label, actual, expected)
	}
	for permission, access := range expected {
		if actual[permission] != access {
			t.Fatalf("%s permission %s = %q, want %q", label, permission, actual[permission], access)
		}
	}
}

func assertNeeds(t *testing.T, label string, actual any, expected string) {
	t.Helper()
	switch value := actual.(type) {
	case string:
		if value == expected {
			return
		}
	case []any:
		for _, item := range value {
			if item == expected {
				return
			}
		}
	}
	t.Fatalf("%s needs = %#v, want dependency %q", label, actual, expected)
}
