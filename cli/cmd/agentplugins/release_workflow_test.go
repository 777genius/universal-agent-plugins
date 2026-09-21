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
	gateIndex, buildIndex := strings.Index(workflow, "Verify exact release-bound Directory bootstrap"), strings.Index(workflow, "\n  build:\n")
	if gateIndex < 0 || buildIndex < 0 || gateIndex > buildIndex {
		t.Fatal("Directory bootstrap gate must be in validate before build")
	}
	for _, required := range []string{
		"go-version: 1.25.13",
		"node-version: \"22.21.1\"",
		"GOWORK: \"off\"",
		"GOTOOLCHAIN: local",
		"GOFLAGS: -buildvcs=false -p=1",
		"go run ./cmd/agentplugins/bootstrapgen",
		"(cd install/integrationctl/agentplugins && go test ./...)",
		"(cd install/integrationctl && go test ./adapters/dirswap ./adapters/source)",
		"(cd cli && go test ./internal/agentpluginscli ./cmd/agentplugins/...)",
		"Prove production binary excludes Directory conformance overrides",
		"go test ./cmd/agentplugins -run '^TestReleaseBuiltBinaryHasNoConformanceEnvironmentOverride$' -count=1",
		"-snapshot cmd/agentplugins/directory_bootstrap_inputs/snapshot.json",
		"-envelope cmd/agentplugins/directory_bootstrap_inputs/envelope.json",
		"-trust cmd/agentplugins/directory_bootstrap_inputs/trusted-keys.json",
		"-check cmd/agentplugins/directory_bootstrap_generated.go",
		"-expected-key-id " + defaultDirectoryKeyID,
		"-expected-public-key " + defaultDirectoryPublicKey,
		"-release-at \"${release_at}\"",
		"CGO_ENABLED=0 go build -trimpath -ldflags=\"-s -w -X main.version=${version}\" -o \"${RUNNER_TEMP}/${asset}\" ./cmd/agentplugins",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("agentplugins stable release lacks %q", required)
		}
	}
	if strings.Count(workflow, "node-version: \"22.21.1\"") != 3 {
		t.Fatal("agentplugins stable release must pin Node 22.21.1 on validate, verified-draft, and promote")
	}
	if strings.Contains(workflow, "node-version: 22\n") || strings.Contains(workflow, "node-version: 22\r") {
		t.Fatal("agentplugins stable release must not use an unpinned Node 22")
	}
	negativeTestIndex := strings.Index(workflow, "TestReleaseBuiltBinaryHasNoConformanceEnvironmentOverride")
	if negativeTestIndex < 0 || negativeTestIndex > buildIndex {
		t.Fatal("negative release-binary conformance test must run in validate before release builds")
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
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate platform proof workflow test")
	}
	workflowPath := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", ".github", "workflows", "agentplugins-platform-proof.yml"))
	body, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(body)
	if strings.Count(workflow, "node-version: \"22.21.1\"") != 2 {
		t.Fatal("platform proof must pin Node 22.21.1 in prepare and native runtime jobs")
	}
	if strings.Contains(workflow, "node-version: 22\n") || strings.Contains(workflow, "node-version: 22\r") {
		t.Fatal("platform proof must not resolve a moving Node 22 release")
	}
	for _, required := range []string{
		"AGENTPLUGINS_STAGED_TEST_CHILD=1",
		"AGENTPLUGINS_DETACHED_ASSERT_ROOT=\"${stage}\"",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("platform proof staged package test lacks %q", required)
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
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		If          string            `yaml:"if"`
		Needs       any               `yaml:"needs"`
		Environment any               `yaml:"environment"`
		Env         map[string]string `yaml:"env"`
		Outputs     map[string]string `yaml:"outputs"`
		With        map[string]any    `yaml:"with"`
		Permissions map[string]string `yaml:"permissions"`
		Steps       []struct {
			If       string            `yaml:"if"`
			Continue bool              `yaml:"continue-on-error"`
			Uses     string            `yaml:"uses"`
			With     map[string]any    `yaml:"with"`
			Env      map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
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
	if !reflect.DeepEqual(releaseNeeds(release.Jobs["stage-draft"].Needs), []string{"validate", "build"}) {
		t.Fatal("stage-draft must retain validate and build dependencies")
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
		"validate":        nil,
		"build":           {"validate"},
		"stage-draft":     {"validate", "build"},
		"platform-proof":  {"validate", "stage-draft"},
		"verified-draft":  {"validate", "stage-draft", "platform-proof"},
		"promote-release": {"validate", "stage-draft", "platform-proof", "verified-draft"},
	}
	expectedPermissions := map[string]map[string]string{
		"validate":        {"checks": "read", "contents": "read", "pull-requests": "read"},
		"build":           {"contents": "read"},
		"stage-draft":     {"contents": "write", "id-token": "write", "attestations": "write", "artifact-metadata": "write"},
		"platform-proof":  {"contents": "read", "attestations": "read"},
		"verified-draft":  {"contents": "write", "attestations": "read"},
		"promote-release": {"contents": "write", "attestations": "read"},
	}
	for name, needs := range expectedNeeds {
		job, ok := workflow.Jobs[name]
		if !ok || !reflect.DeepEqual(releaseNeeds(job.Needs), needs) {
			return fmt.Errorf("%s dependencies", name)
		}
		if !reflect.DeepEqual(effectiveReleasePermissions(workflow, name), expectedPermissions[name]) {
			return fmt.Errorf("%s permissions", name)
		}
		if name == "promote-release" {
			if job.If != "${{ inputs.publish_release == true }}" {
				return fmt.Errorf("promotion condition")
			}
		} else if job.If != "" {
			return fmt.Errorf("%s must use implicit success reachability", name)
		}
		for _, step := range job.Steps {
			if step.Continue || strings.Contains(step.If, "always()") || strings.Contains(step.If, "failure()") || strings.Contains(step.If, "cancelled()") {
				return fmt.Errorf("%s step bypass", name)
			}
		}
	}
	if workflow.Jobs["stage-draft"].Environment != "agentplugins-release" || workflow.Jobs["promote-release"].Environment != "agentplugins-release" {
		return fmt.Errorf("native protected environments")
	}
	stageDownload, stageUpload := false, false
	for _, step := range workflow.Jobs["stage-draft"].Steps {
		if strings.HasPrefix(step.Uses, "actions/download-artifact@") && step.With["pattern"] == "agentplugins-*" && step.With["merge-multiple"] == true {
			stageDownload = true
		}
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") && step.With["name"] == "${{ steps.draft.outputs.assets_artifact }}" && step.With["if-no-files-found"] == "error" {
			stageUpload = true
		}
	}
	if !stageDownload || !stageUpload || workflow.Jobs["platform-proof"].With["release_assets_artifact"] != "${{ needs.stage-draft.outputs.assets_artifact }}" {
		return fmt.Errorf("native artifact handoff")
	}
	return nil
}

func TestNativeReleaseFailureReachabilityAndMutationControls(t *testing.T) {
	workflow := parseReleaseWorkflow(t, "agentplugins-release.yml")
	if err := validateNativeReleaseBoundary(workflow); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"failure", "cancelled", "skipped", ""} {
		for _, name := range []string{"build", "stage-draft", "platform-proof", "verified-draft", "promote-release"} {
			// None of these jobs has a status override. GitHub prepends implicit
			// success(), so any non-success prerequisite keeps it unreachable.
			job := workflow.Jobs[name]
			if status != "success" && (strings.Contains(job.If, "always()") || strings.Contains(job.If, "failure()") || strings.Contains(job.If, "cancelled()")) {
				t.Fatalf("%s reachable after %s", name, status)
			}
		}
	}
	mutations := map[string]func(*releaseWorkflow){
		"build needs": func(w *releaseWorkflow) { job := w.Jobs["build"]; job.Needs = nil; w.Jobs["build"] = job },
		"stage needs": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			job.Needs = []any{"build"}
			w.Jobs["stage-draft"] = job
		},
		"status override": func(w *releaseWorkflow) {
			job := w.Jobs["verified-draft"]
			job.If = "${{ always() }}"
			w.Jobs["verified-draft"] = job
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
		"artifact": func(w *releaseWorkflow) {
			job := w.Jobs["stage-draft"]
			for i := range job.Steps {
				if strings.HasPrefix(job.Steps[i].Uses, "actions/upload-artifact@") {
					job.Steps[i].With["name"] = "mutable"
				}
			}
			w.Jobs["stage-draft"] = job
		},
	}
	for name, mutate := range mutations {
		mutated := parseReleaseWorkflow(t, "agentplugins-release.yml")
		mutate(&mutated)
		if validateNativeReleaseBoundary(mutated) == nil {
			t.Fatalf("negative control accepted %s mutation", name)
		}
	}
}

func validateNPMReleaseBoundary(workflow releaseWorkflow) error {
	expectedNeeds := map[string][]string{
		"prepare": nil, "publish": {"prepare"}, "verify-public": {"prepare", "publish"},
	}
	expectedPermissions := map[string]map[string]string{
		"prepare":       {"contents": "read", "attestations": "read"},
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
	for _, fn := range []string{"success", "always", "failure", "cancelled"} {
		if value, ok := values[fn]; ok {
			expression = strings.ReplaceAll(expression, fn+"()", strconv.FormatBool(value.(bool)))
		}
	}
	words := regexp.MustCompile(`'[^']*'|[a-zA-Z_][a-zA-Z0-9_.]*`)
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
