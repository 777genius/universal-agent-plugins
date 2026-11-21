package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type releaseActionStatusNames struct {
	Success  string
	Always   string
	Failure  string
	Canceled string
	Skipped  string
}

func releaseActionStatuses(t *testing.T) releaseActionStatusNames {
	t.Helper()
	body, err := os.ReadFile("testdata/github-actions-status.yml")
	if err != nil {
		t.Fatal(err)
	}
	var status releaseActionStatusNames
	if err := yaml.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, name := range []string{status.Success, status.Always, status.Failure, status.Canceled, status.Skipped} {
		if name == "" || seen[name] {
			t.Fatal("Actions status vocabulary must contain distinct nonempty names")
		}
		seen[name] = true
	}
	return status
}

func TestStableReleaseRequiresVerifiedReproducibleBootstrapBeforeBuild(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate release workflow test")
	}
	workflowPath := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", ".github", "workflows", "agentplugins-release.yml"))
	body, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	gateIndex, stageIndex := strings.Index(workflow, "Verify exact release-bound Directory bootstrap"), strings.Index(workflow, "\n  stage-draft:\n")
	if gateIndex < 0 || stageIndex < 0 || gateIndex > stageIndex {
		t.Fatal("Directory bootstrap gate must run in qualify-go before draft mutation")
	}
	for _, required := range []string{
		"go-version: 1.26.11",
		"node-version: \"22.21.1\"",
		"GOWORK: \"off\"",
		"GOTOOLCHAIN: local",
		"GOFLAGS: -buildvcs=false -p=1",
		"go run ./cmd/agentplugins/bootstrapgen",
		"(cd install/integrationctl/agentplugins && go test -count=1 ./...)",
		"(cd install/integrationctl && go test -count=1 ./adapters/dirswap ./adapters/source)",
		"(cd cli && go test -count=1 ./internal/agentpluginscli ./cmd/agentplugins/...)",
		"Prove production binary excludes Directory conformance overrides",
		"go test ./cmd/agentplugins -run '^TestReleaseBuiltBinaryHasNoConformanceEnvironmentOverride$' -count=1",
		"-snapshot cmd/agentplugins/directory_bootstrap_inputs/snapshot.json",
		"-envelope cmd/agentplugins/directory_bootstrap_inputs/envelope.json",
		"-trust cmd/agentplugins/directory_bootstrap_inputs/trusted-keys.json",
		"-check cmd/agentplugins/directory_bootstrap_generated.go",
		"-expected-key-id " + defaultDirectoryKeyID,
		"-expected-public-key " + defaultDirectoryPublicKey,
		"-release-at \"${release_at}\"",
		"CGO_ENABLED=0 go build -trimpath -ldflags=\"-s -w -X main.version=${version} -X github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands.Enabled=release-cli-contract-v1 -X github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands.Revision=${SOURCE_COMMIT}\" -o \"${RUNNER_TEMP}/${asset}\" ./cmd/agentplugins",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("agentplugins stable release lacks %q", required)
		}
	}
	if strings.Count(workflow, "node-version: \"22.21.1\"") != 4 {
		t.Fatal("agentplugins stable release must pin Node 22.21.1 on qualify-npm, verified-draft, reuse-verified-draft, and promote")
	}
	if strings.Contains(workflow, "node-version: 22\n") || strings.Contains(workflow, "node-version: 22\r") {
		t.Fatal("agentplugins stable release must not use an unpinned Node 22")
	}
	negativeTestIndex := strings.Index(workflow, "TestReleaseBuiltBinaryHasNoConformanceEnvironmentOverride")
	if negativeTestIndex < 0 || negativeTestIndex > stageIndex {
		t.Fatal("negative release-binary conformance test must run in qualify-go before draft mutation")
	}
	parsed := parseReleaseWorkflow(t, "agentplugins-release.yml")
	for _, name := range []string{"qualify-npm", "verified-draft", "reuse-verified-draft", "promote-release"} {
		setupNode := 0
		for _, step := range parsed.Jobs[name].Steps {
			if strings.HasPrefix(step.Uses, "actions/setup-node@") {
				setupNode++
				if step.With["node-version"] != "22.21.1" {
					t.Fatalf("%s must pin Node 22.21.1", name)
				}
			}
		}
		if setupNode != 1 {
			t.Fatalf("%s must have exactly one setup-node step", name)
		}
	}

	binaryTestPath := filepath.Join(filepath.Dir(source), "release_binary_test.go")
	binaryTestBody, err := os.ReadFile(binaryTestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(binaryTestBody), "forbiddenProductionDirectoryVariables") {
		t.Fatal("release binary test does not inspect the complete forbidden conformance variable set")
	}
}

func TestPlatformProofPinsNodeAndRunsHermeticStagedPackageTests(t *testing.T) {
	workflow := parseReleaseWorkflow(t, "agentplugins-platform-proof.yml")
	for _, jobName := range []string{"prepare", "native-runtime"} {
		job, ok := workflow.Jobs[jobName]
		if !ok {
			t.Fatalf("platform proof lacks %s job", jobName)
		}
		setupNode := 0
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/setup-node@") {
				setupNode++
				if step.With["node-version"] != "22.21.1" {
					t.Fatalf("platform proof %s job must pin Node 22.21.1, got %v", jobName, step.With["node-version"])
				}
			}
		}
		if setupNode != 1 {
			t.Fatalf("platform proof %s job must have exactly one setup-node step", jobName)
		}
	}

	var prepareRun string
	for _, step := range workflow.Jobs["prepare"].Steps {
		if step.ID == "prepare" {
			prepareRun = step.Run
			break
		}
	}
	stagedTest := regexp.MustCompile(`(?m)AGENTPLUGINS_STAGED_TEST_CHILD=1\s*\\\s*\n\s*AGENTPLUGINS_DETACHED_ASSERT_ROOT="\$\{stage\}"\s*\\\s*\n\s*npm test`)
	if !stagedTest.MatchString(prepareRun) {
		t.Fatal("platform proof prepare step must apply both detached-package variables to staged npm test")
	}
}

func TestNPMPublishRunsHermeticStagedPackageTests(t *testing.T) {
	workflow := parseReleaseWorkflow(t, "agentplugins-npm-publish.yml")
	expectedNodeByJob := map[string]string{
		"prepare":       "22.21.1",
		"publish":       "22.23.2",
		"verify-public": "22.23.2",
	}
	for _, jobName := range []string{"prepare", "publish", "verify-public"} {
		job, ok := workflow.Jobs[jobName]
		if !ok {
			t.Fatalf("npm publish workflow lacks %s job", jobName)
		}
		setupNode := 0
		for _, step := range job.Steps {
			if strings.HasPrefix(step.Uses, "actions/setup-node@") {
				setupNode++
				if step.With["node-version"] != expectedNodeByJob[jobName] {
					t.Fatalf("npm publish %s job must pin Node %s, got %v", jobName, expectedNodeByJob[jobName], step.With["node-version"])
				}
			}
		}
		if setupNode != 1 {
			t.Fatalf("npm publish %s job must have exactly one setup-node step", jobName)
		}
	}

	prepare := workflow.Jobs["prepare"]

	var stageRun string
	for _, step := range prepare.Steps {
		if step.ID == "stage" {
			stageRun = step.Run
			break
		}
	}
	for _, required := range []string{
		"python3 scripts/reuse-agentplugins-npm-artifact.py",
		`--repository "${GITHUB_REPOSITORY}" --tag "${TAG}" --commit "$(git rev-parse HEAD)"`,
		`--run-id "${QUALIFIED_RUN_ID}" --run-attempt "${QUALIFIED_RUN_ATTEMPT}"`,
		`--source "${GITHUB_WORKSPACE}" --output-dir "${RUNNER_TEMP}/npm-tarball"`,
		"npm-package.json", "tarball_integrity", "tarball_shasum",
	} {
		if !strings.Contains(stageRun, required) {
			t.Fatalf("npm prepare must reuse the exact qualified tarball: missing %q", required)
		}
	}
	for _, step := range prepare.Steps {
		if strings.Contains(step.Run, "npm test") || strings.Contains(step.Run, "npm pack") || strings.Contains(step.Run, "stage-package") {
			t.Fatal("npm prepare must preserve qualified bytes without staging, retesting or repacking")
		}
		if step.ID == "stage" && (step.Env["QUALIFIED_RUN_ID"] != "${{ inputs.qualified_run_id }}" || step.Env["QUALIFIED_RUN_ATTEMPT"] != "${{ inputs.qualified_run_attempt }}" || step.Env["TAG"] != "${{ inputs.tag }}") {
			t.Fatal("npm reuse must bind the exact qualification run, attempt and tag")
		}
	}

	var publicVerificationRun string
	for _, step := range workflow.Jobs["verify-public"].Steps {
		if strings.Contains(step.Name, "Verify metadata, signatures, provenance") {
			publicVerificationRun = step.Run
			break
		}
	}
	for _, required := range []string{
		"for attempt in {1..12}; do",
		`if [[ "${attempt}" == 12 ]]; then`,
		`sleep "$((attempt * 5))"`,
	} {
		if !strings.Contains(publicVerificationRun, required) {
			t.Fatalf("npm public verification lacks bounded registry propagation contract %q", required)
		}
	}
}

// releaseWorkflow is intentionally small: it parses only the reusable security
// boundary that these tests own. Product-specific staging internals are tested
// by the scripts that implement them.
type releaseWorkflow struct {
	Concurrency struct {
		Group  string `yaml:"group"`
		Cancel bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	On struct {
		Run struct {
			Workflows []string `yaml:"workflows"`
		} `yaml:"workflow_run"`
		Dispatch any `yaml:"workflow_dispatch"`
		Schedule []struct {
			Cron string `yaml:"cron"`
		} `yaml:"schedule"`
	} `yaml:"on"`
	Permissions map[string]string     `yaml:"permissions"`
	Jobs        map[string]releaseJob `yaml:"jobs"`
}

type releaseJob struct {
	If          string            `yaml:"if"`
	Continue    bool              `yaml:"continue-on-error"`
	Uses        string            `yaml:"uses"`
	RunsOn      any               `yaml:"runs-on"`
	Needs       any               `yaml:"needs"`
	Environment any               `yaml:"environment"`
	Env         map[string]string `yaml:"env"`
	Outputs     map[string]string `yaml:"outputs"`
	With        map[string]any    `yaml:"with"`
	Permissions map[string]string `yaml:"permissions"`
	Steps       []struct {
		ID       string            `yaml:"id"`
		Name     string            `yaml:"name"`
		If       string            `yaml:"if"`
		Continue bool              `yaml:"continue-on-error"`
		Uses     string            `yaml:"uses"`
		Run      string            `yaml:"run"`
		With     map[string]any    `yaml:"with"`
		Env      map[string]string `yaml:"env"`
	} `yaml:"steps"`
}

func parseReleaseWorkflow(t *testing.T, name string) releaseWorkflow {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../../.github/workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var workflow releaseWorkflow
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func releaseNeeds(value any) []string {
	switch needs := value.(type) {
	case nil:
		return nil
	case string:
		return []string{needs}
	case []any:
		result := make([]string, 0, len(needs))
		for _, need := range needs {
			result = append(result, need.(string))
		}
		return result
	default:
		panic("unknown workflow needs shape")
	}
}

func effectiveReleasePermissions(workflow releaseWorkflow, job string) map[string]string {
	if workflow.Jobs[job].Permissions != nil {
		return workflow.Jobs[job].Permissions
	}
	return workflow.Permissions
}

// GitHub accepts runner context in step env, but rejects it in job env before
// a runner is allocated. Keep this generic so future release jobs are covered.
func TestCurrentReleaseWorkflowsRejectRunnerContextAtJobEnv(t *testing.T) {
	runnerExpression := regexp.MustCompile(`\$\{\{[^}]*\brunner\s*[.\[]`)
	for _, file := range []string{"agentplugins-release.yml", "agentplugins-npm-publish.yml"} {
		workflow := parseReleaseWorkflow(t, file)
		for name, job := range workflow.Jobs {
			for key, value := range job.Env {
				if runnerExpression.MatchString(value) {
					t.Errorf("%s jobs.%s.env.%s uses unavailable runner context", file, name, key)
				}
			}
		}
	}
}

func TestCurrentReleaseGraphsAndConcurrency(t *testing.T) {
	release := parseReleaseWorkflow(t, "agentplugins-release.yml")
	if !reflect.DeepEqual(releaseNeeds(release.Jobs["build"].Needs), []string{"validate"}) {
		t.Fatal("build must depend on validate")
	}
	if !reflect.DeepEqual(releaseNeeds(release.Jobs["stage-draft"].Needs), []string{"validate", "qualify-go", "qualify-npm", "build", "sign-macos", "verify-macos"}) {
		t.Fatal("stage-draft must require validate, both qualifications, all builds and macOS signing")
	}
	if release.Concurrency.Group != "agentplugins-release-${{ inputs.tag }}" || release.Concurrency.Cancel {
		t.Fatal("agentplugins release concurrency must be tag-keyed and non-cancelling")
	}

	npm := parseReleaseWorkflow(t, "agentplugins-npm-publish.yml")
	if npm.On.Dispatch == nil || len(npm.On.Run.Workflows) != 0 {
		t.Fatal("agentplugins npm publication must remain dispatch-only")
	}
	if npm.Concurrency.Group != "agentplugins-npm-${{ inputs.tag }}" || npm.Concurrency.Cancel {
		t.Fatal("agentplugins npm concurrency must be tag-keyed and non-cancelling")
	}
}

func validateNativeReleaseBoundary(workflow releaseWorkflow) error {
	expectedNeeds := map[string][]string{
		"validate":                 nil,
		"build":                    {"validate"},
		"qualify-go":               {"validate"},
		"qualify-npm":              {"validate"},
		"sign-macos":               {"validate", "build"},
		"verify-macos":             {"validate", "sign-macos"},
		"stage-draft":              {"validate", "qualify-go", "qualify-npm", "build", "sign-macos", "verify-macos"},
		"platform-proof":           {"validate", "stage-draft"},
		"hermes-native-proof":      {"validate", "stage-draft"},
		"antigravity-native-proof": {"validate", "stage-draft"},
		"verified-draft":           {"validate", "stage-draft", "platform-proof", "hermes-native-proof", "antigravity-native-proof"},
		"reuse-verified-draft":     {"validate"},
		"promote-release":          {"validate", "reuse-verified-draft"},
		// Dispatched once every proof passed, beside verified-draft. The
		// promotion run then waits in the tag-keyed concurrency group and
		// authenticates the completed qualification run and its receipt.
		"dispatch-promotion": {"validate", "stage-draft", "platform-proof", "hermes-native-proof", "antigravity-native-proof"},
	}
	expectedPermissions := map[string]map[string]string{
		"validate":                 {"checks": "read", "contents": "read", "pull-requests": "read"},
		"build":                    {"contents": "read"},
		"qualify-go":               {"contents": "read"},
		"qualify-npm":              {"contents": "read"},
		"sign-macos":               {"contents": "read"},
		"verify-macos":             {"contents": "read"},
		"stage-draft":              {"contents": "write", "id-token": "write", "attestations": "write", "artifact-metadata": "write"},
		"platform-proof":           {"contents": "read", "attestations": "read"},
		"hermes-native-proof":      {"contents": "read"},
		"antigravity-native-proof": {"contents": "read"},
		"verified-draft":           {"contents": "write", "actions": "read", "attestations": "read"},
		"reuse-verified-draft":     {"contents": "write", "actions": "read", "attestations": "read"},
		"promote-release":          {"contents": "write", "actions": "write", "attestations": "read"},
		"dispatch-promotion":       {"actions": "write"},
	}
	expectedConditions := map[string]string{
		"validate":                 "",
		"build":                    "${{ inputs.publish_release != true }}",
		"qualify-go":               "${{ inputs.publish_release != true }}",
		"qualify-npm":              "${{ inputs.publish_release != true }}",
		"sign-macos":               "${{ inputs.publish_release != true }}",
		"verify-macos":             "${{ inputs.publish_release != true }}",
		"stage-draft":              "${{ inputs.publish_release != true }}",
		"platform-proof":           "${{ inputs.publish_release != true }}",
		"hermes-native-proof":      "${{ inputs.publish_release != true }}",
		"antigravity-native-proof": "${{ inputs.publish_release != true }}",
		"verified-draft":           "${{ inputs.publish_release != true }}",
		"reuse-verified-draft":     "${{ inputs.publish_release == true }}",
		"promote-release":          "${{ inputs.publish_release == true }}",
		"dispatch-promotion":       "${{ inputs.publish_release != true && inputs.auto_publish == true }}",
	}
	for name, needs := range expectedNeeds {
		job, ok := workflow.Jobs[name]
		if !ok || !reflect.DeepEqual(releaseNeeds(job.Needs), needs) {
			return fmt.Errorf("%s dependencies", name)
		}
		if !reflect.DeepEqual(effectiveReleasePermissions(workflow, name), expectedPermissions[name]) {
			return fmt.Errorf("%s permissions", name)
		}
		if job.If != expectedConditions[name] {
			return fmt.Errorf("%s two-mode reachability condition", name)
		}
		if job.Continue {
			return fmt.Errorf("%s job must not suppress failure", name)
		}
		for _, step := range job.Steps {
			if step.Continue || strings.Contains(step.If, "always()") || strings.Contains(step.If, "failure()") || strings.Contains(step.If, "cancelled()") {
				return fmt.Errorf("%s step bypass", name)
			}
		}
	}
	if workflow.Jobs["stage-draft"].Environment != "agentplugins-release" || workflow.Jobs["promote-release"].Environment != "agentplugins-release" || workflow.Jobs["reuse-verified-draft"].Environment != nil {
		return fmt.Errorf("native protected environments")
	}
	if err := validateMacOSSigningBoundary(workflow); err != nil {
		return err
	}
	promoteIndex, dispatchIndex := -1, -1
	for i, step := range workflow.Jobs["promote-release"].Steps {
		if step.Name == "Reverify exact draft and promote only after all native proofs" {
			promoteIndex = i
		}
		if step.Name == "Dispatch trusted npm publish for the promoted release" {
			dispatchIndex = i
			if !strings.Contains(step.Run, "gh workflow run agentplugins-npm-publish.yml") ||
				!strings.Contains(step.Run, `--ref "${TAG}"`) ||
				!strings.Contains(step.Run, `-f "tag=${TAG}"`) ||
				!strings.Contains(step.Run, "-f publish=true") ||
				!strings.Contains(step.Run, `-f "qualified_run_id=${QUALIFIED_RUN_ID}"`) ||
				!strings.Contains(step.Run, `-f "qualified_run_attempt=${QUALIFIED_RUN_ATTEMPT}"`) {
				return fmt.Errorf("npm dispatch step must run agentplugins-npm-publish.yml from the exact tag with publish=true")
			}
			if step.Env["TAG"] != "${{ inputs.tag }}" || step.Env["QUALIFIED_RUN_ID"] != "${{ inputs.qualified_run_id }}" || step.Env["QUALIFIED_RUN_ATTEMPT"] != "${{ inputs.qualified_run_attempt }}" {
				return fmt.Errorf("npm dispatch must bind the exact release tag, qualification run and attempt")
			}
		}
	}
	if promoteIndex < 0 || dispatchIndex <= promoteIndex {
		return fmt.Errorf("promote-release must dispatch the trusted npm publish after the release goes public")
	}
	for _, name := range []string{"qualify-go", "qualify-npm"} {
		checkoutBound := false
		for _, step := range workflow.Jobs[name].Steps {
			if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["ref"] == "${{ needs.validate.outputs.commit }}" && step.With["persist-credentials"] == false {
				checkoutBound = true
			}
		}
		if !checkoutBound {
			return fmt.Errorf("%s must qualify the frozen validated commit without persisted credentials", name)
		}
	}

	stageUpload := false
	for _, step := range workflow.Jobs["stage-draft"].Steps {
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") && step.With["name"] == "${{ steps.draft.outputs.assets_artifact }}" && step.With["if-no-files-found"] == "error" {
			stageUpload = true
		}
	}
	if !stageUpload || workflow.Jobs["platform-proof"].With["release_assets_artifact"] != "${{ needs.stage-draft.outputs.assets_artifact }}" {
		return fmt.Errorf("native artifact handoff")
	}
	for _, client := range []string{"hermes", "antigravity"} {
		job := workflow.Jobs[client+"-native-proof"]
		if job.Uses != "./.github/workflows/"+client+"-native-e2e.yml" || job.RunsOn != nil || len(job.Steps) != 0 || job.Environment != nil {
			return fmt.Errorf("%s reusable native proof", client)
		}
		if !reflect.DeepEqual(job.With, map[string]any{
			"release_tag":               "${{ inputs.tag }}",
			"frozen_commit":             "${{ needs.validate.outputs.commit }}",
			"release_assets_artifact":   "${{ needs.stage-draft.outputs.assets_artifact }}",
			"expected_checksums_sha256": "${{ needs.stage-draft.outputs.asset_set_digest }}",
		}) {
			return fmt.Errorf("%s frozen native artifact handoff", client)
		}
	}
	return nil
}

// The macOS signing secrets stay in one job on a hosted runner, and staging
// can hash and attest only the macOS bytes that job signed and notarized.
func validateMacOSSigningBoundary(workflow releaseWorkflow) error {
	sign := workflow.Jobs["sign-macos"]
	if sign.Environment != "macos-signing" || sign.RunsOn != "macos-15" || sign.Uses != "" {
		return fmt.Errorf("sign-macos protected hosted runner")
	}
	verify := workflow.Jobs["verify-macos"]
	if verify.Environment != nil || verify.RunsOn != "macos-15" || verify.Uses != "" {
		return fmt.Errorf("verify-macos must use a separate runner without the signing environment")
	}
	verifiedRun := false
	for _, step := range verify.Steps {
		if strings.Contains(step.Run, "bash scripts/verify-agentplugins-macos.sh") &&
			step.Env["SIGNED_DARWIN_AMD64_SHA256"] == "${{ needs.sign-macos.outputs.darwin_amd64_sha256 }}" &&
			step.Env["SIGNED_DARWIN_ARM64_SHA256"] == "${{ needs.sign-macos.outputs.darwin_arm64_sha256 }}" {
			verifiedRun = true
		}
	}
	if !verifiedRun {
		return fmt.Errorf("verify-macos must run against the exact signing outputs")
	}
	secretNames := []string{"APPLE_CERTIFICATE", "APPLE_CERTIFICATE_PASSWORD", "APPLE_TEAM_ID", "APPLE_API_KEY_BASE64", "APPLE_API_KEY_ID", "APPLE_API_ISSUER"}
	signStep := false
	for name, job := range workflow.Jobs {
		values := []string{}
		for _, value := range job.Env {
			values = append(values, value)
		}
		for _, value := range job.With {
			values = append(values, fmt.Sprint(value))
		}
		for _, step := range job.Steps {
			values = append(values, step.Run)
			for _, value := range step.Env {
				values = append(values, value)
			}
			for _, value := range step.With {
				values = append(values, fmt.Sprint(value))
			}
			if name == "sign-macos" && step.ID == "sign" {
				if step.If != "" {
					return fmt.Errorf("signing step must not be skipped")
				}
				for _, secret := range secretNames {
					if step.Env[secret] != "${{ secrets."+secret+" }}" {
						return fmt.Errorf("sign-macos must bind %s from the protected environment", secret)
					}
				}
				signStep = strings.Contains(step.Run, "bash scripts/sign-agentplugins-macos.sh")
			}
		}
		if name != "sign-macos" && strings.Contains(strings.Join(values, "\n"), "secrets.APPLE_") {
			return fmt.Errorf("%s must not receive macOS signing secrets", name)
		}
	}
	if !signStep {
		return fmt.Errorf("sign-macos must run the signing script")
	}
	for _, target := range []string{"amd64", "arm64"} {
		if sign.Outputs["darwin_"+target+"_sha256"] != "${{ steps.sign.outputs.darwin_"+target+"_sha256 }}" {
			return fmt.Errorf("sign-macos %s digest output", target)
		}
	}
	artifactSteps := func(job, action string) map[string]string {
		names := map[string]string{}
		for _, step := range workflow.Jobs[job].Steps {
			if strings.HasPrefix(step.Uses, "actions/"+action+"-artifact@") {
				names[fmt.Sprint(step.With["name"])] = fmt.Sprint(step.With["path"]) + fmt.Sprint(step.With["pattern"])
			}
		}
		return names
	}
	built := artifactSteps("build", "upload")
	if !reflect.DeepEqual(built, map[string]string{
		"agentplugins-binaries":    "${{ runner.temp }}/agentplugins_*_linux_*\n${{ runner.temp }}/agentplugins_*_windows_*\n<nil>",
		"unsigned-darwin-binaries": "${{ runner.temp }}/agentplugins_*_darwin_*<nil>",
	}) {
		return fmt.Errorf("build must hand unsigned macOS binaries only to sign-macos")
	}
	if !reflect.DeepEqual(artifactSteps("sign-macos", "download"), map[string]string{"unsigned-darwin-binaries": "${{ runner.temp }}/agentplugins-darwin<nil>"}) {
		return fmt.Errorf("sign-macos input artifact")
	}
	if _, ok := artifactSteps("sign-macos", "upload")["agentplugins-darwin-signed"]; !ok {
		return fmt.Errorf("sign-macos signed artifact")
	}
	if !reflect.DeepEqual(artifactSteps("verify-macos", "download"), map[string]string{"agentplugins-darwin-signed": "${{ runner.temp }}/agentplugins-darwin<nil>"}) {
		return fmt.Errorf("verify-macos signed artifact")
	}
	staged := artifactSteps("stage-draft", "download")
	if len(staged) != 2 || staged["agentplugins-binaries"] != "${{ runner.temp }}/agentplugins-assets<nil>" || staged["agentplugins-darwin-signed"] != "${{ runner.temp }}/agentplugins-assets<nil>" {
		return fmt.Errorf("stage-draft must download exactly the non-macOS build and the signed macOS binaries")
	}
	for _, step := range workflow.Jobs["stage-draft"].Steps {
		if step.ID != "draft" {
			continue
		}
		bound := step.Env["SIGNED_DARWIN_AMD64_SHA256"] == "${{ needs.sign-macos.outputs.darwin_amd64_sha256 }}" &&
			step.Env["SIGNED_DARWIN_ARM64_SHA256"] == "${{ needs.sign-macos.outputs.darwin_arm64_sha256 }}"
		check, prepare := strings.Index(step.Run, `sha256sum -- "${asset}"`), strings.Index(step.Run, `prepare "${PWD}" "${TAG}" "${FROZEN_COMMIT}"`)
		if !bound || check < 0 || prepare < 0 || check > prepare {
			return fmt.Errorf("stage-draft must bind signed macOS digests before hashing assets")
		}
		return nil
	}
	return fmt.Errorf("stage-draft prepare step")
}

func TestNativeReleaseFailureReachabilityAndMutationControls(t *testing.T) {
	statusNames := releaseActionStatuses(t)
	workflow := parseReleaseWorkflow(t, "agentplugins-release.yml")
	if err := validateNativeReleaseBoundary(workflow); err != nil {
		t.Fatal(err)
	}
	assertNativeReleaseReachability(t, workflow)
	mutations := map[string]func(*releaseWorkflow){
		"build needs": func(w *releaseWorkflow) { job := w.Jobs["build"]; job.Needs = nil; w.Jobs["build"] = job },
		"stage needs": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			job.Needs = []any{"build"}
			w.Jobs["stage-draft"] = job
		},
		"stage qualification bypass": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			job.Needs = []any{"validate", "build"}
			w.Jobs["stage-draft"] = job
		},
		"qualification promotion bypass": func(w *releaseWorkflow) {
			job := w.Jobs["qualify-go"]
			job.If = ""
			w.Jobs["qualify-go"] = job
		},
		"status override": func(w *releaseWorkflow) {
			job := w.Jobs["verified-draft"]
			job.If = "${{ always() }}"
			w.Jobs["verified-draft"] = job
		},
		"build promotion bypass": func(w *releaseWorkflow) {
			job := w.Jobs["build"]
			job.If = ""
			w.Jobs["build"] = job
		},
		"reuse needs": func(w *releaseWorkflow) {
			job := w.Jobs["reuse-verified-draft"]
			job.Needs = nil
			w.Jobs["reuse-verified-draft"] = job
		},
		"promote proof bypass": func(w *releaseWorkflow) {
			job := w.Jobs["promote-release"]
			job.Needs = []any{"validate"}
			w.Jobs["promote-release"] = job
		},
		"permissions": func(w *releaseWorkflow) {
			job := w.Jobs["platform-proof"]
			job.Permissions = map[string]string{"contents": "write"}
			w.Jobs["platform-proof"] = job
		},
		"environment": func(w *releaseWorkflow) {
			job := w.Jobs["promote-release"]
			job.Environment = nil
			w.Jobs["promote-release"] = job
		},
		"npm dispatch": func(w *releaseWorkflow) {
			job := w.Jobs["promote-release"]
			kept := job.Steps[:0:0]
			for _, step := range job.Steps {
				if step.Name != "Dispatch trusted npm publish for the promoted release" {
					kept = append(kept, step)
				}
			}
			job.Steps = kept
			w.Jobs["promote-release"] = job
		},
		"artifact": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			for i := range job.Steps {
				if strings.HasPrefix(job.Steps[i].Uses, "actions/upload-artifact@") {
					job.Steps[i].With["name"] = "mutable"
				}
			}
			w.Jobs["stage-draft"] = job
		},
		"sign-macos missing":   func(w *releaseWorkflow) { delete(w.Jobs, "sign-macos") },
		"verify-macos missing": func(w *releaseWorkflow) { delete(w.Jobs, "verify-macos") },
		"verify-macos receives signing environment": func(w *releaseWorkflow) {
			job := w.Jobs["verify-macos"]
			job.Environment = "macos-signing"
			w.Jobs["verify-macos"] = job
		},
		"stage without macOS signing": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			job.Needs = []any{"validate", "qualify-go", "qualify-npm", "build"}
			w.Jobs["stage-draft"] = job
		},
		"sign-macos without build": func(w *releaseWorkflow) {
			job := w.Jobs["sign-macos"]
			job.Needs = []any{"validate"}
			w.Jobs["sign-macos"] = job
		},
		"sign-macos unprotected": func(w *releaseWorkflow) {
			job := w.Jobs["sign-macos"]
			job.Environment = nil
			w.Jobs["sign-macos"] = job
		},
		"sign-macos self-hosted": func(w *releaseWorkflow) {
			job := w.Jobs["sign-macos"]
			job.RunsOn = "self-hosted"
			w.Jobs["sign-macos"] = job
		},
		"sign-macos write permission": func(w *releaseWorkflow) {
			job := w.Jobs["sign-macos"]
			job.Permissions = map[string]string{"contents": "write"}
			w.Jobs["sign-macos"] = job
		},
		"sign-macos promotion mode bypass": func(w *releaseWorkflow) { job := w.Jobs["sign-macos"]; job.If = ""; w.Jobs["sign-macos"] = job },
		"sign-macos status override": func(w *releaseWorkflow) {
			job := w.Jobs["sign-macos"]
			job.If = "${{ always() }}"
			w.Jobs["sign-macos"] = job
		},
		"sign-macos digest output dropped": func(w *releaseWorkflow) {
			job := w.Jobs["sign-macos"]
			delete(job.Outputs, "darwin_arm64_sha256")
			w.Jobs["sign-macos"] = job
		},
		"sign-macos step bypass": func(w *releaseWorkflow) {
			job := w.Jobs["sign-macos"]
			job.Steps[len(job.Steps)-1].Continue = true
			w.Jobs["sign-macos"] = job
		},
		"signing secret leaks to stage-draft": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			job.Steps[len(job.Steps)-1].Env = map[string]string{"KEY": "${{ secrets.APPLE_API_KEY_BASE64 }}"}
			w.Jobs["stage-draft"] = job
		},
		"stage-draft macOS digest binding gone": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			for i := range job.Steps {
				if job.Steps[i].ID == "draft" {
					delete(job.Steps[i].Env, "SIGNED_DARWIN_ARM64_SHA256")
				}
			}
			w.Jobs["stage-draft"] = job
		},
		"stage-draft merges every artifact": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			job.Steps[1].With = map[string]any{"pattern": "agentplugins-*", "merge-multiple": true, "path": "${{ runner.temp }}/agentplugins-assets"}
			w.Jobs["stage-draft"] = job
		},
		"stage-draft takes unsigned macOS binaries": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			job.Steps[2].With["name"] = "unsigned-darwin-binaries"
			w.Jobs["stage-draft"] = job
		},
		"build hands macOS binaries to staging": func(w *releaseWorkflow) {
			job := w.Jobs["build"]
			for i := range job.Steps {
				if job.Steps[i].With["name"] == "agentplugins-binaries" {
					job.Steps[i].With["path"] = "${{ runner.temp }}/agentplugins_*"
				}
			}
			w.Jobs["build"] = job
		},
	}
	for _, client := range []string{"hermes", "antigravity"} {
		name := client + "-native-proof"
		mutations[name+" missing"] = func(w *releaseWorkflow) { delete(w.Jobs, name) }
		mutations[name+" bypassed by verified draft"] = func(w *releaseWorkflow) {
			job := w.Jobs["verified-draft"]
			kept := []any{}
			for _, need := range releaseNeeds(job.Needs) {
				if need != name {
					kept = append(kept, need)
				}
			}
			job.Needs = kept
			w.Jobs["verified-draft"] = job
		}
		for _, dependency := range []string{"validate", "stage-draft"} {
			mutations[name+" missing "+dependency+" edge"] = func(w *releaseWorkflow) {
				job := w.Jobs[name]
				kept := []any{}
				for _, need := range releaseNeeds(job.Needs) {
					if need != dependency {
						kept = append(kept, need)
					}
				}
				job.Needs = kept
				w.Jobs[name] = job
			}
		}
		mutations[name+" permissions"] = func(w *releaseWorkflow) {
			job := w.Jobs[name]
			job.Permissions = map[string]string{"contents": "write"}
			w.Jobs[name] = job
		}
		mutations[name+" promotion mode bypass"] = func(w *releaseWorkflow) {
			job := w.Jobs[name]
			job.If = ""
			w.Jobs[name] = job
		}
		for _, status := range []string{statusNames.Always, statusNames.Failure, statusNames.Canceled} {
			mutations[name+" "+status+" override"] = func(w *releaseWorkflow) {
				job := w.Jobs[name]
				job.If = "${{ " + status + "() }}"
				w.Jobs[name] = job
			}
		}
		mutations[name+" workflow substitution"] = func(w *releaseWorkflow) {
			job := w.Jobs[name]
			job.Uses = "./.github/workflows/agentplugins-platform-proof.yml"
			w.Jobs[name] = job
		}
		for _, input := range []string{"release_tag", "frozen_commit", "release_assets_artifact", "expected_checksums_sha256"} {
			mutations[name+" changed "+input] = func(w *releaseWorkflow) {
				job := w.Jobs[name]
				job.With[input] = "unqualified"
				w.Jobs[name] = job
			}
		}
	}
	for _, dependency := range []string{"validate", "stage-draft", "platform-proof", "hermes-native-proof", "antigravity-native-proof"} {
		mutations["dispatch-promotion missing "+dependency+" edge"] = func(w *releaseWorkflow) {
			job := w.Jobs["dispatch-promotion"]
			kept := []any{}
			for _, need := range releaseNeeds(job.Needs) {
				if need != dependency {
					kept = append(kept, need)
				}
			}
			job.Needs = kept
			w.Jobs["dispatch-promotion"] = job
		}
	}
	mutations["dispatch-promotion without auto_publish opt-in"] = func(w *releaseWorkflow) {
		job := w.Jobs["dispatch-promotion"]
		job.If = "${{ inputs.publish_release != true }}"
		w.Jobs["dispatch-promotion"] = job
	}
	mutations["dispatch-promotion broader permissions"] = func(w *releaseWorkflow) {
		job := w.Jobs["dispatch-promotion"]
		job.Permissions = map[string]string{"actions": "write", "contents": "write"}
		w.Jobs["dispatch-promotion"] = job
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			mutated := parseReleaseWorkflow(t, "agentplugins-release.yml")
			mutate(&mutated)
			if validateNativeReleaseBoundary(mutated) == nil {
				t.Fatalf("negative control accepted %s mutation", name)
			}
		})
	}
}

func assertNativeReleaseReachability(t *testing.T, workflow releaseWorkflow) {
	t.Helper()
	statusNames := releaseActionStatuses(t)
	for _, name := range []string{"qualify-go", "qualify-npm", "build", "sign-macos", "verify-macos", "stage-draft", "platform-proof", "hermes-native-proof", "antigravity-native-proof", "verified-draft", "reuse-verified-draft", "promote-release"} {
		job := workflow.Jobs[name]
		promotion := name == "reuse-verified-draft" || name == "promote-release"
		results := map[string]string{}
		for _, dependency := range releaseNeeds(job.Needs) {
			results[dependency] = "success"
		}
		if !evaluateNativeReleaseJob(t, job, results, promotion) {
			t.Fatalf("%s unreachable after successful prerequisites in its intended mode", name)
		}
		if evaluateNativeReleaseJob(t, job, results, !promotion) {
			t.Fatalf("%s reachable in the wrong release mode", name)
		}
		for _, need := range releaseNeeds(job.Needs) {
			for _, status := range []string{statusNames.Failure, statusNames.Canceled, statusNames.Skipped, ""} {
				if status == "" {
					delete(results, need)
				} else {
					results[need] = status
				}
				if evaluateNativeReleaseJob(t, job, results, promotion) {
					t.Fatalf("%s reachable after %s prerequisite %q", name, need, status)
				}
			}
			results[need] = "success"
		}
	}
}

// Model the Actions default status check against the parsed job's real needs
// and condition. An explicit status function replaces implicit success().
func evaluateNativeReleaseJob(t *testing.T, job releaseJob, results map[string]string, publish bool) bool {
	t.Helper()
	statusNames := releaseActionStatuses(t)
	values := map[string]any{statusNames.Success: true, statusNames.Failure: false, statusNames.Canceled: false, statusNames.Always: true, "inputs.publish_release": publish}
	for _, need := range releaseNeeds(job.Needs) {
		status := results[need]
		values["needs."+need+".result"] = status
		if status != "success" {
			values["success"] = false
		}
		if status == statusNames.Failure || status == statusNames.Canceled {
			values[status] = true
		}
	}
	condition := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(job.If, "${{"), "}}"))
	if condition == "" {
		condition = "true"
	}
	functions := []string{statusNames.Success, statusNames.Always, statusNames.Failure, statusNames.Canceled}
	if !regexp.MustCompile(`\b(` + strings.Join(functions, "|") + `)\s*\(`).MatchString(condition) {
		condition = "success() && (" + condition + ")"
	}
	return evaluateReleaseCondition(t, condition, values)
}

func validateNPMReleaseBoundary(workflow releaseWorkflow) error {
	expectedNeeds := map[string][]string{
		"prepare": nil, "publish": {"prepare"}, "verify-public": {"prepare", "publish"},
	}
	expectedPermissions := map[string]map[string]string{
		"prepare":       {"contents": "read", "attestations": "read", "actions": "read"},
		"publish":       {"contents": "read", "id-token": "write"},
		"verify-public": {"contents": "read", "attestations": "read"},
	}
	expectedConditions := map[string]string{
		"prepare":       "${{ github.event_name == 'workflow_dispatch' }}",
		"publish":       "${{ success() && github.event_name == 'workflow_dispatch' && inputs.publish == true && needs.prepare.result == 'success' }}",
		"verify-public": "${{ success() && github.event_name == 'workflow_dispatch' && inputs.publish == true && needs.publish.result == 'success' }}",
	}
	for _, name := range []string{"prepare", "publish", "verify-public"} {
		job, ok := workflow.Jobs[name]
		if !ok || !reflect.DeepEqual(releaseNeeds(job.Needs), expectedNeeds[name]) {
			return fmt.Errorf("%s dependencies", name)
		}
		if job.If != expectedConditions[name] {
			return fmt.Errorf("%s reachability condition", name)
		}
		if !reflect.DeepEqual(effectiveReleasePermissions(workflow, name), expectedPermissions[name]) {
			return fmt.Errorf("%s permissions", name)
		}
		for _, step := range job.Steps {
			if step.Continue || strings.Contains(step.If, "always()") || strings.Contains(step.If, "failure()") || strings.Contains(step.If, "cancelled()") {
				return fmt.Errorf("%s step bypass", name)
			}
		}
	}
	if workflow.Jobs["publish"].Environment != "npm-agentplugins" || workflow.Jobs["prepare"].Environment != nil || workflow.Jobs["verify-public"].Environment != nil {
		return fmt.Errorf("protected environment boundary")
	}
	prepareUpload, publishDownload := false, false
	for _, step := range workflow.Jobs["prepare"].Steps {
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") && step.With["name"] == "agentplugins-npm-${{ steps.stage.outputs.version }}" && step.With["if-no-files-found"] == "error" {
			prepareUpload = true
		}
	}
	for _, step := range workflow.Jobs["publish"].Steps {
		if strings.HasPrefix(step.Uses, "actions/download-artifact@") && step.With["name"] == "agentplugins-npm-${{ needs.prepare.outputs.version }}" {
			publishDownload = true
		}
	}
	if !prepareUpload || !publishDownload {
		return fmt.Errorf("immutable prepare-to-publish artifact handoff")
	}
	requiredOutputs := []string{"version", "commit", "package_name", "tarball_file", "tarball_integrity", "tarball_shasum"}
	for _, output := range requiredOutputs {
		if workflow.Jobs["prepare"].Outputs[output] == "" {
			return fmt.Errorf("prepare output %s", output)
		}
	}
	verifyBindings := map[string]string{
		"PACKAGE_NAME": "${{ needs.prepare.outputs.package_name }}", "VERSION": "${{ needs.prepare.outputs.version }}",
		"COMMIT": "${{ needs.prepare.outputs.commit }}", "TARBALL_INTEGRITY": "${{ needs.prepare.outputs.tarball_integrity }}",
		"TARBALL_SHASUM": "${{ needs.prepare.outputs.tarball_shasum }}",
	}
	foundBindings := false
	for _, step := range workflow.Jobs["verify-public"].Steps {
		if reflect.DeepEqual(step.Env, verifyBindings) {
			foundBindings = true
		}
	}
	if !foundBindings {
		return fmt.Errorf("prepare-to-verify output handoff")
	}
	return nil
}

func TestNPMPreparePublishVerifyBoundary(t *testing.T) {
	workflow := parseReleaseWorkflow(t, "agentplugins-npm-publish.yml")
	if err := validateNPMReleaseBoundary(workflow); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"failure", "cancelled", "skipped", ""} {
		values := map[string]any{
			"github.event_name": "workflow_dispatch", "inputs.publish": true,
			"needs.prepare.result": status, "needs.publish.result": status,
			"success": false, "failure": status == "failure", "cancelled": status == "cancelled", "always": true,
		}
		for _, name := range []string{"publish", "verify-public"} {
			if evaluateReleaseCondition(t, workflow.Jobs[name].If, values) {
				t.Fatalf("%s reachable after %q dependency", name, status)
			}
		}
	}
}

func TestNPMReleaseBoundaryMutationNegativeControls(t *testing.T) {
	mutations := map[string]func(*releaseWorkflow){
		"publish needs": func(w *releaseWorkflow) { job := w.Jobs["publish"]; job.Needs = nil; w.Jobs["publish"] = job },
		"verify needs": func(w *releaseWorkflow) {
			job := w.Jobs["verify-public"]
			job.Needs = []any{"publish"}
			w.Jobs["verify-public"] = job
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
		"artifact": func(w *releaseWorkflow) {
			job := w.Jobs["publish"]
			for i := range job.Steps {
				if strings.HasPrefix(job.Steps[i].Uses, "actions/download-artifact@") {
					job.Steps[i].With["name"] = "mutable"
				}
			}
			w.Jobs["publish"] = job
		},
		"continue": func(w *releaseWorkflow) {
			job := w.Jobs["verify-public"]
			job.Steps[0].Continue = true
			w.Jobs["verify-public"] = job
		},
	}
	for name, mutate := range mutations {
		workflow := parseReleaseWorkflow(t, "agentplugins-npm-publish.yml")
		mutate(&workflow)
		if validateNPMReleaseBoundary(workflow) == nil {
			t.Fatalf("negative control accepted %s mutation", name)
		}
	}
}

func evaluateReleaseCondition(t *testing.T, expression string, values map[string]any) bool {
	t.Helper()
	expression = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(expression), "${{"), "}}"))
	statusNames := releaseActionStatuses(t)
	for _, fn := range []string{statusNames.Success, statusNames.Always, statusNames.Failure, statusNames.Canceled} {
		if value, ok := values[fn]; ok {
			expression = strings.ReplaceAll(expression, fn+"()", strconv.FormatBool(value.(bool)))
		}
	}
	// Job IDs such as needs.verify-source contain hyphens; GitHub expressions
	// have no subtraction operator, so a hyphen is always part of a name.
	words := regexp.MustCompile(`'[^']*'|[a-zA-Z_][a-zA-Z0-9_.-]*`)
	expression = words.ReplaceAllStringFunc(expression, func(word string) string {
		if strings.HasPrefix(word, "'") {
			return strconv.Quote(word[1 : len(word)-1])
		}
		if value, ok := values[word]; ok {
			switch typed := value.(type) {
			case string:
				return strconv.Quote(typed)
			case bool:
				return strconv.FormatBool(typed)
			}
		}
		if word == "true" || word == "false" {
			return word
		}
		t.Fatalf("unreviewed workflow context %q", word)
		return "false"
	})
	tree, err := parser.ParseExpr(expression)
	if err != nil {
		t.Fatal(err)
	}
	var eval func(ast.Expr) any
	eval = func(node ast.Expr) any {
		switch value := node.(type) {
		case *ast.ParenExpr:
			return eval(value.X)
		case *ast.BasicLit:
			decoded, err := strconv.Unquote(value.Value)
			if err != nil {
				t.Fatal(err)
			}
			return decoded
		case *ast.Ident:
			return value.Name == "true"
		case *ast.BinaryExpr:
			left, right := eval(value.X), eval(value.Y)
			switch value.Op {
			case token.EQL:
				return left == right
			case token.NEQ:
				return left != right
			case token.LAND:
				return left.(bool) && right.(bool)
			case token.LOR:
				return left.(bool) || right.(bool)
			}
		}
		t.Fatal("unsupported workflow condition syntax")
		return false
	}
	return eval(tree).(bool)
}

func TestNpmDriftGuardScheduledReadOnlyAndDetectsDrift(t *testing.T) {
	workflow := parseReleaseWorkflow(t, "agentplugins-npm-drift.yml")
	if len(workflow.On.Schedule) != 1 || workflow.On.Schedule[0].Cron == "" {
		t.Fatal("npm drift guard must run on a schedule")
	}
	if !reflect.DeepEqual(workflow.Permissions, map[string]string{"contents": "read"}) {
		t.Fatalf("npm drift guard must be read-only, got %v", workflow.Permissions)
	}
	for name, job := range workflow.Jobs {
		if !reflect.DeepEqual(effectiveReleasePermissions(workflow, name), map[string]string{"contents": "read"}) {
			t.Fatalf("npm drift guard job %s must be read-only", name)
		}
		for _, step := range job.Steps {
			//nolint:misspell // the GitHub context function uses the double-l spelling
			if step.Continue || strings.Contains(step.If, "always()") || strings.Contains(step.If, "failure()") || strings.Contains(step.If, "cancelled()") {
				t.Fatalf("npm drift guard job %s step bypass", name)
			}
		}
	}
	job, ok := workflow.Jobs["npm-drift"]
	if !ok {
		t.Fatal("npm drift guard lacks the npm-drift job")
	}
	found := false
	for _, step := range job.Steps {
		if !strings.Contains(step.Run, "dist-tags.latest") {
			continue
		}
		found = true
		for _, required := range []string{
			"exit 1", "npm view", ".published_at", "dist-tags.latest",
			".draft == false", ".prerelease == false", "^agentplugins-v", "age_minutes < 360",
		} {
			if !strings.Contains(step.Run, required) {
				t.Fatalf("npm drift check lacks %q", required)
			}
		}
	}
	if !found {
		t.Fatal("npm drift guard lacks the registry parity check")
	}
}
