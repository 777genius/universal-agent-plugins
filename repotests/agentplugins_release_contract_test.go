package pluginkitairepo_test

import (
	"strings"
	"testing"
)

func TestAgentpluginsReleaseContractsStayFailClosed(t *testing.T) {
	root := RepoRoot(t)
	makefile := readRepoFile(t, root, "Makefile")
	releaseWorkflow := readRepoFile(t, root, ".github", "workflows", "agentplugins-release.yml")
	npmWorkflow := readRepoFile(t, root, ".github", "workflows", "agentplugins-npm-publish.yml")
	platformWorkflow := readRepoFile(t, root, ".github", "workflows", "agentplugins-platform-proof.yml")
	platformPrepareJob := yamlJob(t, platformWorkflow, "prepare")
	platformNativeJob := yamlJob(t, platformWorkflow, "native-runtime")
	platformCompleteJob := yamlJob(t, platformWorkflow, "proof-complete")
	releaseDraftJob := yamlJob(t, releaseWorkflow, "stage-draft")
	releaseProofJob := yamlJob(t, releaseWorkflow, "platform-proof")
	releasePromoteJob := yamlJob(t, releaseWorkflow, "promote-release")
	npmPrepareJob := yamlJob(t, npmWorkflow, "prepare")
	npmPublishJob := yamlJob(t, npmWorkflow, "publish")
	npmPublicVerifyJob := yamlJob(t, npmWorkflow, "verify-public")
	removedBoundaryScript := readRepoFile(t, root, "scripts", "check-removed-contract-boundary.sh")
	runbook := readRepoFile(t, root, "docs", "agentplugins-release.md")

	for _, want := range []string{
		"go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./...",
		"go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./cli/plugin-kit-ai/...",
		"go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./install/integrationctl/...",
		"go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./install/plugininstall/...",
		"go test -count=1 -timeout=$(REQUIRED_TEST_TIMEOUT) ./sdk/...",
		"cd npm/agentplugins && npm test && npm pack --dry-run --ignore-scripts",
		"cd install/integrationctl && go vet ./...",
	} {
		mustContain(t, makefile, want)
	}

	for _, want := range []string{
		"producer_mode:",
		"default: binary-only",
		"options:\n          - binary-only",
		"PRODUCER_MODE: ${{ inputs.producer_mode }}",
		"unsupported agentplugins producer mode",
	} {
		mustContain(t, releaseWorkflow, want)
	}
	mustContain(t, releaseWorkflow, "actions/attest@")
	mustContain(t, releaseWorkflow, "gh release create")
	mustAppearBefore(t, releaseWorkflow, "actions/attest@", "gh release create")
	mustContain(t, releaseWorkflow, "^agentplugins-v[0-9]+\\.[0-9]+\\.[0-9]+$")
	mustNotContain(t, releaseWorkflow, "--prerelease")
	mustContain(t, releaseDraftJob, "exact resumable draft; refusing to overwrite immutable assets")
	mustContain(t, releaseDraftJob, "--draft")
	mustContain(t, releaseDraftJob, "existing draft asset differs from the frozen build")
	mustContain(t, releaseDraftJob, "gh api graphql")
	mustContain(t, releaseDraftJob, ".data.repository.release")
	mustContain(t, releaseDraftJob, "Share frozen draft assets with the read-only proof workflow")
	mustContain(t, releaseDraftJob, "assets_artifact=agentplugins-draft-assets-${FROZEN_COMMIT}")
	mustNotContain(t, releaseDraftJob, "target_commitish")
	mustContain(t, releaseProofJob, "needs: [validate, stage-draft]")
	mustContain(t, releaseProofJob, "contents: read")
	mustContain(t, releaseProofJob, "require_draft: true")
	mustContain(t, releaseProofJob, "expected_asset_set_digest: ${{ needs.stage-draft.outputs.asset_set_digest }}")
	mustContain(t, releaseProofJob, "release_assets_artifact: ${{ needs.stage-draft.outputs.assets_artifact }}")
	mustContain(t, releasePromoteJob, "needs: [validate, stage-draft, platform-proof]")
	mustContain(t, releasePromoteJob, "EXPECTED_ASSET_SET_DIGEST: ${{ needs.stage-draft.outputs.asset_set_digest }}")
	mustContain(t, releasePromoteJob, "gh release edit \"${TAG}\"")
	mustContain(t, releasePromoteJob, "--draft=false")
	mustContain(t, releasePromoteJob, `git fetch --force origin "refs/tags/${TAG}:refs/tags/${TAG}"`)
	mustContain(t, releasePromoteJob, `git rev-list -n 1 "refs/tags/${TAG}"`)
	mustContain(t, releasePromoteJob, "gh api graphql")
	mustNotContain(t, releasePromoteJob, "target_commitish")
	mustAppearBefore(t, releaseWorkflow, "gh release create", "gh release edit")
	mustAppearBefore(t, releaseWorkflow, "uses: ./.github/workflows/agentplugins-platform-proof.yml", "gh release edit")
	mustContain(t, releaseWorkflow, "uses: ./.github/workflows/agentplugins-platform-proof.yml")
	mustContain(t, releaseWorkflow, "expected_commit: ${{ needs.validate.outputs.commit }}")
	mustContain(t, releaseWorkflow, `test "${WORKFLOW_REF}" = "refs/heads/main"`)
	mustContain(t, releaseWorkflow, "release workflow source does not match the exact tagged main commit")
	mustContain(t, releaseWorkflow, "commits/${COMMIT}/pulls")
	mustContain(t, releaseWorkflow, "release commit must come from a merged pull request into main")
	mustContain(t, releaseWorkflow, "check-runs?filter=latest&per_page=100")
	for _, name := range []string{
		"dependency-review",
		"test",
		"docs-check",
		"polyglot-smoke (ubuntu-latest)",
		"polyglot-smoke (windows-latest)",
		"analyze (go)",
		"analyze (javascript-typescript)",
		"govulncheck (root)",
		"govulncheck (cli)",
		"govulncheck (integrationctl)",
		"govulncheck (plugininstall)",
		"govulncheck (sdk)",
	} {
		mustContain(t, releaseWorkflow, name)
	}

	for _, want := range []string{
		"name: Agentplugins NPM Publish",
		"tag:",
		"publish:",
		"default: false",
		"ref: ${{ inputs.tag }}",
		"agentplugins-v[0-9]+",
		"Verify exact public release identity and attestations",
		"release-assets.js verify",
		"stage-release.js",
		"npm-public-contract.js stage-outputs",
		"agentplugins-npm-${{ steps.stage.outputs.version }}",
	} {
		mustContain(t, npmWorkflow, want)
	}
	for _, want := range []string{
		"Verify exact public release identity and attestations",
		"release-assets.js verify",
		"stage-release.js",
		"npm-public-contract.js stage-outputs",
	} {
		mustContain(t, npmPrepareJob, want)
	}
	for _, want := range []string{
		"needs: prepare",
		"environment: npm-agentplugins",
		"id-token: write",
		"npm publish",
		"--provenance",
		"Refuse overwrite",
		"test -z \"${NPM_TOKEN:-}\"",
		"test -z \"${NODE_AUTH_TOKEN:-}\"",
	} {
		mustContain(t, npmPublishJob, want)
	}
	for _, want := range []string{
		"needs: [prepare, publish]",
		"npm audit signatures --json --include-attestations",
		"npm/agentplugins/scripts/npm-public-contract.js\" metadata",
		"npm/agentplugins/scripts/npm-public-contract.js\" audit",
		"npm/agentplugins/scripts/npm-public-contract.js\" attestation",
		"npm/agentplugins/scripts/npm-public-contract.js\" download",
		"run_agentplugins()",
		"--target codex,cursor,kiro",
		"run_agentplugins add",
		"run_agentplugins update",
		"run_agentplugins remove",
		"data.succeeded == 3",
	} {
		mustContain(t, npmPublicVerifyJob, want)
	}
	for _, unwanted := range []string{
		"verify_only",
		"only historical verification is supported",
		"--tag beta",
		"bootstrap_publish",
		"NPM_AGENTPLUGINS_PUBLISH_READY",
	} {
		mustNotContain(t, npmWorkflow, unwanted)
	}
	mustAppearBefore(t, npmWorkflow, "release-assets.js verify", "stage-release.js")
	mustAppearBefore(t, npmWorkflow, "npm publish", "Verify metadata, signatures, provenance, and isolated lifecycle")
	mustNotContain(t, npmPublicVerifyJob, "NPM_TOKEN")
	mustNotContain(t, npmPublicVerifyJob, "NODE_AUTH_TOKEN")

	for _, want := range []string{
		"workflow_call:",
		"allow_legacy_manifest:",
		"Audit a schema-v1 historical release; never eligible for the strict producer contract",
		"native runtime E2E (${{ matrix.target }})",
		"target: darwin-amd64",
		"target: darwin-arm64",
		"target: linux-amd64",
		"target: linux-arm64",
		"target: windows-amd64",
		"target: windows-arm64",
		"macos-15-intel",
		"ubuntu-24.04-arm",
		"windows-11-arm",
		"platform-proof.js",
		"verified-release.json",
		"Aggregate all six native platform proofs",
		"draft proof requires caller-staged release assets",
		"Download caller-staged draft assets",
		"bootstrap_mode:",
		"local_frozen_asset",
		"public_release_download",
		"platform proof workflow source does not match the frozen release commit",
		"historical audit must be dispatched with --ref ${TAG}",
		".isDraft == false and .isPrerelease == false and .tagName == $tag",
		`git -C release-source fetch --force origin "refs/tags/${TAG}:refs/tags/${TAG}"`,
		`git -C release-source rev-list -n 1 "refs/tags/${TAG}"`,
	} {
		mustContain(t, platformWorkflow, want)
	}
	for _, want := range []string{
		"contents: read",
		"attestations: read",
		"caller-staged assets are only valid for draft proof",
		`[[ ! "${EXPECTED_COMMIT}" =~ ^[0-9a-f]{40}$ ]]`,
		`if [[ "${commit}" != "${EXPECTED_COMMIT}" ]]`,
		"release tag commit does not match the caller's frozen commit",
		"ref: ${{ inputs.expected_commit }}",
		"agentplugins-proof-bundle",
		"release-assets",
		"git -C release-source diff --quiet HEAD --",
		"docs/AGENTPLUGINS_CLIENT_E2E.md",
		"docs/evidence/agentplugins-client-e2e-2026-08-30.json",
		`--evidence-root "${GITHUB_WORKSPACE}/release-source/docs"`,
	} {
		mustContain(t, platformPrepareJob, want)
	}
	mustAppearBefore(t, platformPrepareJob, "git -C release-source diff --quiet HEAD --", "stage-release.js")
	mustAppearBefore(t, platformPrepareJob, "stage-release.js", "npm test && npm pack --dry-run --ignore-scripts")
	for _, want := range []string{
		"ref: ${{ inputs.expected_commit }}",
		`test "$(git rev-parse HEAD)" = "${{ inputs.expected_commit }}"`,
		"needs.prepare.outputs.bootstrap_mode",
		"release_assets=\"-\"",
		"npm/agentplugins/scripts/platform-proof.js",
	} {
		mustContain(t, platformNativeJob, want)
	}
	for _, want := range []string{
		"Download all native platform proofs",
		"expected exactly six machine-readable platform proofs",
		"expected exactly one machine-readable proof for ${target}",
		".schema_version == 1",
		".release_version == $version",
		".proofs.isolated_add_info_update_remove == $lifecycle",
		`.bootstrap_source == $bootstrap_mode`,
		`.proofs.local_frozen_asset_bootstrap == ($bootstrap_mode == "local_frozen_asset")`,
		`.proofs.anonymous_public_release_download == ($bootstrap_mode == "public_release_download")`,
		".proofs.warm_cache_without_proof_source == true",
	} {
		mustContain(t, platformCompleteJob, want)
	}
	mustNotContain(t, platformWorkflow, "target_commitish")
	mustNotContain(t, platformWorkflow, `releases/tags/${TAG}`)
	mustContain(t, readRepoFile(t, root, "npm", "agentplugins", "scripts", "platform-proof.js"), "assertPublicJSONPathFree")
	mustNotContain(t, npmWorkflow, "add context7")

	mustContain(t, removedBoundaryScript, "if command -v rg")
	mustContain(t, removedBoundaryScript, "git grep --untracked --exclude-standard")
	mustContain(t, removedBoundaryScript, "removed-contract scan failed")

	for _, want := range []string{
		"attests all six binaries, `checksums.txt`, and",
		"`release-manifest.json` before creating the non-public draft",
		"promotes that exact draft only after all six native platform proofs succeed",
		"requires a merged pull request into",
		"Repository settings are not treated as the release proof",
		"short-lived bootstrap",
		"disallow bypass-2FA tokens",
		"Do not add a bootstrap token back",
		"required `binary-only` producer mode",
		"same six assets",
		"npm facade is staged from",
		"manual trusted publisher",
		"publish=true",
		"protected `npm-agentplugins` environment",
		"requires GitHub OIDC",
		"npm audit signatures",
		"add/info/update/remove lifecycle",
		"With `publish=false`",
		"never recreates an old version",
	} {
		mustContain(t, runbook, want)
	}
}

func TestAgentpluginsReadmesUseUnversionedNpxExamples(t *testing.T) {
	root := RepoRoot(t)
	for _, path := range [][]string{
		{"README.md"},
		{"npm", "agentplugins", "README.md"},
	} {
		readme := readRepoFile(t, root, path...)
		mustContain(t, readme, "npx universal-agent-plugins add context7")
		mustNotContain(t, readme, "npx universal-agent-plugins@")
	}
}

func TestAgentpluginsPublicNpmRepairFixtureIsIsolatedAndUnambiguous(t *testing.T) {
	workflow := readRepoFile(t, RepoRoot(t), ".github", "workflows", "agentplugins-npm-publish.yml")
	job := yamlJob(t, workflow, "verify-public")
	const configHome = `XDG_CONFIG_HOME="${root}/config"`
	const surface = `mkdir -p "${XDG_CONFIG_HOME}/opencode"`
	const add = `run_agentplugins add "${repair_source}" --target opencode`
	mustContain(t, job, configHome)
	mustContain(t, job, surface)
	mustContain(t, job, add)
	mustAppearBefore(t, job, configHome, surface)
	mustAppearBefore(t, job, surface, add)
	mustContain(t, job, `[.installations[] | select(.declared_name == "context7") | .clients[] | select(.client_id == "opencode") | .native_objects[] | select(.kind == "opencode_global_mcp_server") | .path] | if length == 1 and (.[0] | type == "string" and length > 0) then .[0] else error("expected exactly one Context7 OpenCode repair path") end`)
	mustNotContain(t, job, `.installations[0]`)
	mustAppearBefore(t, job, add, `repair_file="$(jq -er`)
}

func TestAgentpluginsNativeInstallSurfaceIsChecksumAndVersionBound(t *testing.T) {
	root := RepoRoot(t)
	unixInstaller := readRepoFile(t, root, "install.sh")
	windowsInstaller := readRepoFile(t, root, "install.ps1")
	workflow := readRepoFile(t, root, ".github", "workflows", "agentplugins-native-install.yml")
	guide := readRepoFile(t, root, "docs", "NATIVE_INSTALL.md")

	for _, want := range []string{
		"checksums.txt must contain exactly one entry",
		"checksum mismatch",
		"OBSERVED_VERSION",
		"agentplugins $VERSION",
		"mktemp \"$BIN_DIR/.agentplugins.XXXXXX\"",
		"mv -f \"$INSTALL_TEMP\" \"$DEST_PATH\"",
	} {
		mustContain(t, unixInstaller, want)
	}
	for _, want := range []string{
		"Get-FileHash",
		"checksums.txt must contain exactly one valid entry",
		"ObservedVersion",
		"[System.IO.File]::Replace($InstallTemp, $Destination, $null, $true)",
		"[System.IO.File]::Move($InstallTemp, $Destination)",
	} {
		mustContain(t, windowsInstaller, want)
	}
	for _, target := range []string{
		"darwin-amd64",
		"darwin-arm64",
		"linux-amd64",
		"linux-arm64",
		"windows-amd64",
		"windows-arm64",
	} {
		mustContain(t, workflow, target)
	}
	mustContain(t, workflow, "Install the published native binary without Node.js")
	mustContain(t, guide, "The native installation path does not require")
	mustContain(t, guide, "atomically replaces the destination only after all checks pass")
}

func TestAgentpluginsHomebrewTapOwnershipIsDocumented(t *testing.T) {
	root := RepoRoot(t)
	guide := readRepoFile(t, root, "docs", "NATIVE_INSTALL.md")
	runbook := readRepoFile(t, root, "docs", "agentplugins-release.md")

	for _, want := range []string{
		"777genius/homebrew-agentplugins",
		"checks the latest stable release every hour",
		"release-manifest.json",
		"artifact attestation",
		"short-lived `GITHUB_TOKEN`",
		"long-lived cross-repository PAT",
	} {
		mustContain(t, runbook, want)
	}
	mustContain(t, guide, "brew install 777genius/agentplugins/agentplugins")
}

func mustAppearBefore(t *testing.T, text, first, second string) {
	t.Helper()
	firstIndex := strings.Index(text, first)
	secondIndex := strings.Index(text, second)
	if firstIndex < 0 || secondIndex < 0 || firstIndex >= secondIndex {
		t.Fatalf("expected %q to appear before %q", first, second)
	}
}

func yamlJob(t *testing.T, workflow, name string) string {
	t.Helper()
	marker := "\n  " + name + ":\n"
	start := strings.Index(workflow, marker)
	if start < 0 {
		t.Fatalf("missing workflow job %q", name)
	}

	lines := strings.SplitAfter(workflow[start+1:], "\n")
	var job strings.Builder
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if index > 0 && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
			break
		}
		job.WriteString(line)
	}
	return job.String()
}
