package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
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
	workflowPath := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", ".github", "workflows", "agentplugins-release.yml"))
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
		"GOWORK: \"off\"",
		"GOTOOLCHAIN: local",
		"GOFLAGS: -buildvcs=false -p=1",
		"go run ./cmd/agentplugins/bootstrapgen",
		"(cd install/integrationctl && go test ./agentplugins/... ./adapters/dirswap ./adapters/source)",
		"(cd cli/plugin-kit-ai && go test ./internal/agentpluginscli ./cmd/agentplugins/...)",
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

// Parse the job graph as well as executing its preflight shell. A new job or
// dispatch mode must not accidentally inherit the publication permissions.
type producerWorkflow struct {
	Concurrency struct {
		Group  string `yaml:"group"`
		Cancel bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Name string `yaml:"name"`
	On   struct {
		Run struct {
			Workflows []string `yaml:"workflows"`
		} `yaml:"workflow_run"`
		Dispatch struct {
			Inputs map[string]struct {
				Default  string   `yaml:"default"`
				Type     string   `yaml:"type"`
				Required bool     `yaml:"required"`
				Options  []string `yaml:"options"`
			} `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		If          string            `yaml:"if"`
		Name        string            `yaml:"name"`
		Runner      string            `yaml:"runs-on"`
		Timeout     int               `yaml:"timeout-minutes"`
		Outputs     map[string]string `yaml:"outputs"`
		Environment any               `yaml:"environment"`
		Needs       any               `yaml:"needs"`
		Uses        string            `yaml:"uses"`
		Permissions map[string]string `yaml:"permissions"`
		Env         map[string]string `yaml:"env"`
		Steps       []struct {
			Name     string            `yaml:"name"`
			ID       string            `yaml:"id"`
			If       string            `yaml:"if"`
			Continue bool              `yaml:"continue-on-error"`
			Run      string            `yaml:"run"`
			Uses     string            `yaml:"uses"`
			With     map[string]any    `yaml:"with"`
			Env      map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func readProducerWorkflow(t *testing.T, name string) producerWorkflow {
	t.Helper()
	_, source, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../../../.github/workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	var workflow producerWorkflow
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func runProducerPreflight(t *testing.T, script string, values map[string]string, success bool) {
	t.Helper()
	command := exec.Command("/bin/bash", "-c", script)
	command.Dir = t.TempDir()
	command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin"}
	for key, value := range values {
		command.Env = append(command.Env, key+"="+value)
	}
	body, err := command.CombinedOutput()
	if (err == nil) != success {
		t.Fatalf("preflight success=%v, expected %v: %v\n%s", err == nil, success, err, body)
	}
}

// YAML decoding alone accepts runner expressions in job env, but GitHub rejects
// that context there before creating a run. Step env supports runner instead.
func TestReleaseWorkflowRunnerCacheContext(t *testing.T) {
	w := readProducerWorkflow(t, "agentplugins-release.yml")
	runnerExpression := regexp.MustCompile(`\$\{\{[^}]*\brunner\s*[.\[]`)
	for name, job := range w.Jobs {
		for key, value := range job.Env {
			if runnerExpression.MatchString(value) {
				t.Errorf("jobs.%s.env.%s uses unavailable runner context", name, key)
			}
		}
	}
	consumers := map[string]bool{
		"Provision declared Go modules for offline producer":               false,
		"Freeze one full release-contract pair and project verified bytes": false,
	}
	for _, step := range w.Jobs["paired-preparation"].Steps {
		if _, ok := consumers[step.Name]; !ok {
			continue
		}
		consumers[step.Name] = true
		for key, want := range map[string]string{
			"GOMODCACHE": "${{ runner.temp }}/paired-modules",
			"GOCACHE":    "${{ runner.temp }}/paired-cache",
		} {
			if step.Env[key] != want {
				t.Errorf("%s must set step env %s=%q, got %q", step.Name, key, want, step.Env[key])
			}
		}
	}
	for name, found := range consumers {
		if !found {
			t.Errorf("missing cache consumer %q", name)
		}
	}
}

func TestReleasePairedPreparationReadOnlyGraph(t *testing.T) {
	w := readProducerWorkflow(t, "agentplugins-release.yml")
	mode := w.On.Dispatch.Inputs["producer_mode"]
	if mode.Default != "binary-only" || strings.Join(mode.Options, ",") != "binary-only,paired-preparation,paired-promotion,paired-input-provenance" {
		t.Fatal("default binary-only dispatch contract changed")
	}
	if len(w.Permissions) != 1 || w.Permissions["contents"] != "read" {
		t.Fatal("workflow must default to contents-read")
	}
	if len(w.Jobs) != 11 {
		t.Fatal("review every new producer job for preparation reachability")
	}
	for name, job := range w.Jobs {
		if name == "dispatch_contract" || strings.HasPrefix(name, "paired_input_") {
			continue
		}
		if name == "paired-promotion-admission" || name == "paired-sign-and-promote" {
			if job.If != "${{ github.event_name == 'workflow_dispatch' && inputs.producer_mode == 'paired-promotion' }}" {
				t.Fatalf("%s loses explicit promotion isolation", name)
			}
			continue
		}
		if name != "paired-preparation" {
			if job.If != "${{ inputs.producer_mode == 'binary-only' }}" {
				t.Fatalf("%s reachable from paired route", name)
			}
			continue
		}
		if job.If != "${{ inputs.producer_mode == 'paired-preparation' }}" || job.Needs != nil || job.Uses != "" {
			t.Fatal("paired route must be an independent explicit job")
		}
		if len(job.Permissions) != 1 || job.Permissions["contents"] != "read" {
			t.Fatal("paired route has write or attestation permission")
		}
		var scripts strings.Builder
		uploads := 0
		for _, step := range job.Steps {
			scripts.WriteString(step.Run)
			if step.Uses != "" && !strings.HasPrefix(step.Uses, "actions/checkout@") && !strings.HasPrefix(step.Uses, "actions/setup-go@") &&
				!strings.HasPrefix(step.Uses, "actions/setup-node@") && !strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
				t.Fatalf("unreviewed paired action: %s", step.Uses)
			}
			if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
				uploads++
				paths, _ := step.With["path"].(string)
				for _, required := range []string{"/candidate-identity.json", "/candidate/candidate.json", "/pair-prepared.json", "/agentplugins/*", "/plugin-kit-ai/*"} {
					if !strings.Contains(paths, required) {
						t.Fatalf("paired upload lacks %s", required)
					}
				}
			}
		}
		body := scripts.String()
		if uploads != 1 || strings.Count(body, "stageCandidate({") != 1 || strings.Count(body, "prepareAuthoringRelease(options)") != 1 ||
			strings.Count(body, "verifyAuthoringRelease(options)") != 1 {
			t.Fatal("must freeze once, project once and verify before one upload")
		}
		for _, required := range []string{"six-platform-pair", "release-cli-contract-v1", "manifestDigest: staged.manifest_sha256", "commit: process.env.SOURCE_SHA, engine_revision: process.env.SOURCE_SHA"} {
			if !strings.Contains(body, required) {
				t.Fatalf("missing paired binding %q", required)
			}
		}
		for _, forbidden := range []string{"gh ", "goreleaser", "go build", "npm publish", "twine", "homebrew", "git push", "git tag", "workflow_dispatch"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("preparation reaches forbidden effect %q", forbidden)
			}
		}
		if job.Steps[0].Name != "Validate explicit paired identity before checkout" {
			t.Fatal("identity must fail before checkout/setup/build")
		}
		valid := map[string]string{"SOURCE_SHA": strings.Repeat("a", 40), "WORKFLOW_SHA": strings.Repeat("a", 40),
			"TAG": "agentplugins-v0.1.54", "KIT_VERSION": "2.0.0", "GITHUB_REPOSITORY": "777genius/universal-agent-plugins"}
		runProducerPreflight(t, job.Steps[0].Run, valid, true)
		for key, invalid := range map[string]string{"SOURCE_SHA": "latest", "WORKFLOW_SHA": strings.Repeat("b", 40), "TAG": "agentplugins-v01.2.3", "KIT_VERSION": "1.2.4", "GITHUB_REPOSITORY": "777genius/plugin-kit-ai"} {
			values := make(map[string]string)
			for k, v := range valid {
				values[k] = v
			}
			values[key] = invalid
			runProducerPreflight(t, job.Steps[0].Run, values, false)
		}
	}
	// Preserve the existing publication gate dependency chain.
	for name, expected := range map[string]string{"build": "validate", "stage-draft": "validate,build", "platform-proof": "validate,stage-draft", "promote-release": "validate,stage-draft,platform-proof"} {
		var needs []string
		switch value := w.Jobs[name].Needs.(type) {
		case string:
			needs = []string{value}
		case []any:
			for _, item := range value {
				needs = append(needs, item.(string))
			}
		}
		if strings.Join(needs, ",") != expected {
			t.Fatalf("%s lost required gates", name)
		}
	}
}

func TestReleaseLegacyGoReleaserRejectsMajorTwoBeforeEffects(t *testing.T) {
	w := readProducerWorkflow(t, "release-assets.yml")
	job := w.Jobs["goreleaser"]
	if len(job.Steps) == 0 || job.Steps[0].Name != "Reject standard-first versions on legacy producer" || job.Steps[0].Uses != "" {
		t.Fatal("legacy version guard must precede checkout, tooling and publication")
	}
	guard := job.Steps[0].Run
	if !strings.Contains(guard, "paired-preparation") {
		t.Fatal("guard must direct users to paired producer")
	}
	for _, tag := range []string{"v1.0.0", "v1.2.4", "v1.99.100"} {
		runProducerPreflight(t, guard, map[string]string{"RELEASE_TAG": tag}, true)
	}
	for _, tag := range []string{"v2.0.0", "v2.1.0", "v20.0.0", "v2.0.0-rc.1", "v01.2.4", "latest", "", "v1.2.4; touch forbidden"} {
		runProducerPreflight(t, guard, map[string]string{"RELEASE_TAG": tag}, false)
	}
}

func TestReleasePairedRouteCannotTriggerDownstreamPublication(t *testing.T) {
	paired := readProducerWorkflow(t, "agentplugins-release.yml")
	if paired.Name != "Agentplugins Release Assets" {
		t.Fatal("review downstream triggers before renaming producer")
	}
	for _, name := range []string{"npm-publish.yml", "pypi-publish.yml", "homebrew-tap.yml"} {
		w := readProducerWorkflow(t, name)
		if strings.Join(w.On.Run.Workflows, ",") != "Release Assets" {
			t.Fatalf("%s may be triggered by paired producer", name)
		}
		for jobName, job := range w.Jobs {
			// Dispatch-only jobs may be added by B. Every automatic route must
			// reject failed legacy runs; trigger names above isolate paired runs.
			for _, conclusion := range []string{"failure", "cancelled", "skipped", ""} {
				if downstreamCondition(t, job.If, "workflow_run", conclusion) {
					t.Fatalf("%s/%s admits failed legacy run", name, jobName)
				}
			}
			legacy := jobName == "publish-npm" || jobName == "publish-pypi" || jobName == "update-homebrew-tap"
			if legacy && !downstreamCondition(t, job.If, "workflow_run", "success") {
				t.Fatalf("%s lost successful automatic v1 route", name)
			}
			if !legacy && downstreamCondition(t, job.If, "workflow_run", "success") {
				t.Fatalf("%s/%s new paired job must be dispatch-only", name, jobName)
			}
		}
	}
	w := readProducerWorkflow(t, "agentplugins-npm-publish.yml")
	if len(w.On.Run.Workflows) != 0 {
		t.Fatal("agentplugins npm publication must require separate dispatch")
	}
}

// Parse the restricted boolean expression grammar, rejecting unknown syntax.
// Testing a failed event must evaluate the whole OR/AND graph, not find a token.
func downstreamCondition(t *testing.T, expression, event, conclusion string) bool {
	t.Helper()
	return workflowCondition(t, expression, map[string]any{"github.event_name": event, "github.event.workflow_run.conclusion": conclusion,
		"vars.NPM_PUBLISH_READY": "true", "vars.PYPI_TRUSTED_PUBLISHING_READY": "true", "inputs.producer_mode": "paired-promotion"})
}
func workflowCondition(t *testing.T, expression string, values map[string]any) bool {
	t.Helper()
	expression = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(expression), "${{"), "}}"))
	if expression == "" {
		expression = "success()"
	}
	for _, fn := range []string{"success", "always", "failure", "cancelled"} {
		if value, ok := values[fn]; ok {
			expression = strings.ReplaceAll(expression, fn+"()", strconv.FormatBool(value.(bool)))
		}
	}
	words := regexp.MustCompile(`'[^']*'|[a-zA-Z_][a-zA-Z0-9_.]*`)
	expression = words.ReplaceAllStringFunc(expression, func(s string) string {
		if strings.HasPrefix(s, "'") {
			return strconv.Quote(s[1 : len(s)-1])
		}
		if value, ok := values[s]; ok {
			switch v := value.(type) {
			case string:
				return strconv.Quote(v)
			case bool:
				return strconv.FormatBool(v)
			default:
				t.Fatal("unsupported typed context")
			}
		}
		if s == "true" || s == "false" {
			return s
		}
		t.Fatalf("unreviewed downstream expression context %q", s)
		return "false"
	})
	tree, err := parser.ParseExpr(expression)
	if err != nil {
		t.Fatal(err)
	}
	var evaluate func(ast.Expr) any
	evaluate = func(e ast.Expr) any {
		switch v := e.(type) {
		case *ast.ParenExpr:
			return evaluate(v.X)
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				t.Fatal("non-string expression literal")
			}
			value, err := strconv.Unquote(v.Value)
			if err != nil {
				t.Fatal(err)
			}
			return value
		case *ast.Ident:
			if v.Name == "true" {
				return true
			}
			if v.Name == "false" {
				return false
			}
		case *ast.BinaryExpr:
			left, right := evaluate(v.X), evaluate(v.Y)
			switch v.Op {
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
		t.Fatal("unsupported downstream expression syntax")
		return false
	}
	result, ok := evaluate(tree).(bool)
	if !ok {
		t.Fatal("non-boolean job condition")
	}
	return result
}

func TestReleaseDownstreamEventIsolationNegativeControls(t *testing.T) {
	for _, expression := range []string{
		"github.event.workflow_run.conclusion == 'success' || true",
		"github.event_name == 'workflow_run' || github.event.workflow_run.conclusion == 'success'",
	} {
		if !downstreamCondition(t, expression, "workflow_run", "failure") {
			t.Fatal("negative control failed")
		}
	}
	for _, conclusion := range []string{"success", "failure"} {
		if downstreamCondition(t, "${{ github.event_name == 'workflow_dispatch' }}", "workflow_run", conclusion) {
			t.Fatal("dispatch job reachable automatically")
		}
	}
}

func TestReleasePairedPromotionProtectedGraph(t *testing.T) {
	w := readProducerWorkflow(t, "agentplugins-release.yml")
	admission, signing := w.Jobs["paired-promotion-admission"], w.Jobs["paired-sign-and-promote"]
	if w.Concurrency.Group != "agentplugins-release-${{ inputs.tag }}" || w.Concurrency.Cancel {
		t.Fatal("promotion must serialize on the independently selected, record-bound tag")
	}
	for _, job := range []string{"paired-promotion-admission", "paired-sign-and-promote"} {
		for _, step := range w.Jobs[job].Steps {
			if _, ok := step.Env["PROMOTION_RECORD"]; !ok {
				continue
			}
			for key, want := range map[string]string{"TAG": "${{ inputs.tag }}", "WORKFLOW_REF": "${{ github.ref }}", "WORKFLOW_SHA": "${{ github.sha }}", "KIT_VERSION": "${{ inputs.plugin_kit_version }}"} {
				if step.Env[key] != want {
					t.Fatalf("%s loses independent %s binding", step.Name, key)
				}
			}
			binding := strings.Index(step.Run, "Buffer.from(")
			contract := strings.Index(step.Run, "p.checkNativeContracts(record)")
			scratch := strings.Index(step.Run, "fs.mkdtempSync(")
			if contract <= binding || scratch <= contract {
				t.Fatalf("%s must reject unsupported native identifiers before scratch/provider effects", step.Name)
			}
			effect := strings.Index(step.Run, "p.acquirePreparation(")
			if binding < 0 || !strings.Contains(step.Run, ", selected)") || (effect >= 0 && binding > effect) {
				t.Fatalf("%s must bind canonical record before native/provider effects", step.Name)
			}
		}
	}
	if admission.Needs != nil || len(admission.Permissions) != 2 || admission.Permissions["actions"] != "read" || admission.Permissions["contents"] != "read" || signing.Needs != "paired-promotion-admission" {
		t.Fatal("native admission must precede protected promotion")
	}
	if signing.Environment != "agentplugins-release" {
		t.Fatal("paired signing requires protected release environment")
	}
	if signing.Permissions["contents"] != "write" || signing.Permissions["id-token"] != "write" || signing.Permissions["attestations"] != "write" {
		t.Fatal("missing protected signing boundary")
	}
	valid := map[string]string{"SOURCE_SHA": strings.Repeat("a", 40), "WORKFLOW_SHA": strings.Repeat("a", 40),
		"TAG": "agentplugins-v0.1.54", "KIT_VERSION": "2.0.0", "GITHUB_REPOSITORY": "777genius/universal-agent-plugins", "WORKFLOW_REF": "refs/tags/agentplugins-v0.1.54"}
	runProducerPreflight(t, admission.Steps[0].Run, valid, true)
	for _, key := range []string{"SOURCE_SHA", "WORKFLOW_SHA", "TAG", "KIT_VERSION", "GITHUB_REPOSITORY", "WORKFLOW_REF"} {
		values := make(map[string]string)
		for k, v := range valid {
			values[k] = v
		}
		values[key] = "invalid"
		runProducerPreflight(t, admission.Steps[0].Run, values, false)
	}
	scripts := ""
	attestIndex, admitIndex, promoteIndex := -1, -1, -1
	for i, step := range signing.Steps {
		scripts += step.Run
		if strings.Contains(step.Run, "admission=admit") {
			admitIndex = i
		}
		if step.Uses == "actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6" {
			attestIndex = i
		}
		if strings.Contains(step.Run, "case \"${PROMOTION_OPERATION}\" in promote|reconcile)") {
			promoteIndex = i
		}
	}
	if admitIndex < 0 || attestIndex <= admitIndex || promoteIndex <= attestIndex {
		t.Fatal("admit -> pinned attest -> reverify/promote required")
	}
	for _, bad := range []string{"go build", "stageCandidate", "prepareAuthoringRelease", "npm publish", "--clobber", "git push"} {
		if strings.Contains(scripts, bad) {
			t.Fatalf("promotion reaches %s", bad)
		}
	}
	if !strings.Contains(admission.Steps[len(admission.Steps)-1].Run, "requireNativeContracts") {
		t.Fatal("unsupported contracts must reject before protected job")
	}
}

func TestReleasePairedPromotionShellSyntax(t *testing.T) {
	w := readProducerWorkflow(t, "agentplugins-release.yml")
	if len(w.On.Dispatch.Inputs) != 10 {
		t.Fatal("review dispatch input limit and closed input contract")
	}
	for _, name := range []string{"paired-promotion-admission", "paired-sign-and-promote"} {
		for _, step := range w.Jobs[name].Steps {
			if step.Run == "" {
				continue
			}
			cmd := exec.Command("/bin/bash", "-n")
			cmd.Dir = t.TempDir()
			cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin"}
			cmd.Stdin = strings.NewReader(step.Run)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%s: %v %s", step.Name, err, output)
			}
		}
	}
}

func TestFrozenNativeReadOnlyWorkflowContract(t *testing.T) {
	w := readProducerWorkflow(t, "authoring-frozen-native.yml")
	if len(w.Jobs) != 2 || len(w.On.Dispatch.Inputs) != 10 || len(w.On.Run.Workflows) != 0 {
		t.Fatal("N2 requires explicit selected native route, excluded failure route and ten bounded inputs")
	}
	if len(w.Permissions) != 1 || w.Permissions["contents"] != "read" || w.Concurrency.Cancel {
		t.Fatal("native producer must preserve read-only permissions and owned cancellation")
	}
	job, ok := w.Jobs["linux-amd64"]
	if !ok || job.If != "${{ github.event_name == 'workflow_dispatch' && (fromJSON(inputs.host_contract).target == 'linux-amd64' || fromJSON(inputs.host_contract).target == 'linux-arm64') }}" || job.Needs != nil || job.Environment != nil {
		t.Fatal("native route must remain independently dispatched without protected effects")
	}
	if len(job.Permissions) != 2 || job.Permissions["actions"] != "read" || job.Permissions["contents"] != "read" {
		t.Fatal("only contents/actions read is permitted")
	}
	if job.Env["PATH"] != "/usr/local/bin:/usr/bin:/bin" || len(job.Steps) == 0 || job.Steps[0].Uses != "" {
		t.Fatal("guarded PATH and pre-acquisition validation required")
	}
	var scripts strings.Builder
	downloads, uploads := 0, 0
	for _, step := range job.Steps {
		scripts.WriteString(step.Run)
		if step.Run != "" {
			command := exec.Command("/bin/bash", "-n")
			command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin"}
			command.Stdin = strings.NewReader(step.Run)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("%s: %v %s", step.Name, err, output)
			}
		}
		if token, ok := step.Env["GH_TOKEN"]; ok && (step.Name != "Acquire and extract the same checked preparation ZIP" || token != "${{ github.token }}") {
			t.Fatal("read token escaped acquisition")
		}
		if strings.HasPrefix(step.Uses, "actions/download-artifact@") {
			t.Fatal("a second downloader can extract different bytes from the checked ZIP")
		}
		if strings.Contains(step.Run, "p.acquireArtifact(") {
			downloads++
			if !strings.Contains(step.Run, "p.extractArtifact(file, pin, 'preparation', files,") {
				t.Fatal("extract the same acquired file with the independently selected pin")
			}
		}
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			uploads++
			if step.With["path"] != "${{ runner.temp }}/frozen-native-evidence/*" {
				t.Fatal("upload must exclude binaries, client state and compilation caches")
			}
		}
		if step.Uses != "" && !regexp.MustCompile(`^actions/(checkout|setup-node|setup-go|upload-artifact)@[0-9a-f]{40}$`).MatchString(step.Uses) {
			t.Fatalf("unreviewed action %s", step.Uses)
		}
	}
	if downloads != 1 || uploads != 1 {
		t.Fatal("one exact input acquisition and one diagnostic/evidence upload required")
	}
	body := scripts.String()
	for _, forbidden := range []string{"go build", "go test", "go mod", "stageCandidate", "frozenCandidate", "npm install", "npm publish", "attestation verify", "git push", "--auth-complete", "--accept-security-risk", "strace", "ptrace"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("native route reaches forbidden operation %s", forbidden)
		}
	}
	for _, required := range []string{"p.acquireArtifact", "p.extractArtifact", "run_attempt: Number(process.env.PREPARATION_ATTEMPT)", "authoring-native-qualification.js", "go_sha256: process.env.HOST_GO_SHA256", "pair_marker_sha256: process.env.PAIR_SHA256"} {
		if !strings.Contains(body, required) {
			t.Fatalf("native route lacks %s", required)
		}
	}
}

func TestFrozenPreparationReceiptOutsideProjectionBytes(t *testing.T) {
	job := readProducerWorkflow(t, "agentplugins-release.yml").Jobs["paired-preparation"]
	receipts, uploads := 0, 0
	for _, step := range job.Steps {
		if step.Name == "Bind preparation invocation outside unchanged frozen input bytes" {
			receipts++
			for _, required := range []string{"n.writePreparation(root, pins", "workflow_sha: process.env.WORKFLOW_SHA", "run_attempt: Number(process.env.GITHUB_RUN_ATTEMPT)", "deepEqual(after, before)"} {
				if !strings.Contains(step.Run, required) {
					t.Fatalf("receipt does not bind %s", required)
				}
			}
			for _, forbidden := range []string{"qualification_sha256", "tarball_sha256", "attest", "stageCandidate({"} {
				if strings.Contains(step.Run, forbidden) {
					t.Fatalf("provenance-only receipt includes %s", forbidden)
				}
			}
		}
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			uploads++
			paths, _ := step.With["path"].(string)
			if !strings.Contains(paths, "${{ env.PAIRED_OUTPUT }}/preparation-run.json\n") || strings.Contains(paths, "/agentplugins/preparation-run") {
				t.Fatal("receipt must be a sibling of unchanged eight-file projections")
			}
		}
	}
	if receipts != 1 || uploads != 1 {
		t.Fatal("one byte-bound receipt before the existing upload required")
	}
}

func TestN2AdmissionAcquiresEvidenceBeforeOIDC(t *testing.T) {
	w := readProducerWorkflow(t, "agentplugins-release.yml")
	read := w.Jobs["paired-promotion-admission"]
	if read.Environment != nil || len(read.Permissions) != 2 || read.Permissions["actions"] != "read" || read.Permissions["contents"] != "read" {
		t.Fatal("native evidence intake must remain outside protected writes/OIDC")
	}
	step := read.Steps[len(read.Steps)-1]
	selectAt := strings.Index(step.Run, "p.validateSelection(")
	acquireAt := strings.Index(step.Run, "p.admitNativeEvidence(")
	publicAt := strings.Index(step.Run, "p.requireNativeContracts(")
	if selectAt < 0 || acquireAt <= selectAt || publicAt <= acquireAt {
		t.Fatal("syntactic dispatch selection, real native admission, then mandatory public contract required")
	}
	for key, value := range map[string]string{
		"GH_TOKEN": "${{ github.token }}", "PREPARATION_RUN": "${{ inputs.preparation_run }}",
		"PREPARATION_ATTEMPT": "${{ inputs.preparation_attempt }}", "PREPARATION_ARTIFACT": "${{ inputs.preparation_artifact }}",
		"PREPARATION_DIGEST": "${{ inputs.preparation_digest }}",
	} {
		if step.Env[key] != value {
			t.Fatalf("missing independent provider locator %s", key)
		}
	}
	write := w.Jobs["paired-sign-and-promote"]
	if write.Needs != "paired-promotion-admission" {
		t.Fatal("protected job may not bypass read-only admission failure")
	}
	for _, step := range write.Steps {
		if strings.HasPrefix(step.Uses, "actions/download-artifact@") {
			t.Fatal("protected job must never independently download/extract unchecked ZIP bytes")
		}
		if strings.Contains(step.Run, "p.acquirePreparation(") && !strings.Contains(step.Run, "root: acquired.root") {
			t.Fatal("promotion must use the checked extracted preparation root")
		}
	}
}

func TestN2ExcludedNativeExecutionRemainsFailure(t *testing.T) {
	w := readProducerWorkflow(t, "authoring-frozen-native.yml")
	if _, ok := w.On.Dispatch.Inputs["host_contract"]; !ok {
		t.Fatal("explicit target/version/host-tool pins required")
	}
	job := w.Jobs["excluded-native-execution"]
	if !strings.Contains(job.If, "target != 'linux-amd64'") || !strings.Contains(job.If, "target != 'linux-arm64'") ||
		len(job.Permissions) != 1 || job.Permissions["contents"] != "read" || len(job.Steps) != 1 || job.Environment != nil {
		t.Fatal("excluded Windows/writable macOS must retain an explicit non-protected failure route")
	}
	if job.Steps[0].Uses != "" || !strings.Contains(job.Steps[0].Run, "NATIVE_EXECUTION_PENDING") {
		t.Fatal("excluded platforms must not download or execute products")
	}
	command := exec.Command("/bin/bash", "-e", "-c", job.Steps[0].Run)
	command.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin"}
	if output, err := command.CombinedOutput(); err == nil || !strings.Contains(string(output), "No terminal emitted") {
		t.Fatalf("pending execution must fail, not become skipped qualification: %v %s", err, output)
	}
	_, source, _, _ := runtime.Caller(0)
	body, err := os.ReadFile(filepath.Join(filepath.Dir(source), "../../../../.github/workflows/authoring-frozen-native.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"runs-on: windows", "runs-on: macos", "continue-on-error:", "strategy:", "--accept-security-risk"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("excluded native route enables %s", forbidden)
		}
	}
}

// Fixed C1 graph expectations; fixtures prove source reachability, not hosted
// permission enforcement, cryptographic acceptance or genuine provider custody.
func c1Needs(w producerWorkflow, name string) []string {
	switch n := w.Jobs[name].Needs.(type) {
	case nil:
		return nil
	case string:
		return []string{n}
	case []any:
		result := []string{}
		for _, v := range n {
			result = append(result, v.(string))
		}
		return result
	default:
		panic("unknown needs shape")
	}
}
func c1Permissions(w producerWorkflow, name string) map[string]string {
	if w.Jobs[name].Permissions != nil {
		return w.Jobs[name].Permissions
	}
	return w.Permissions
}
func c1Scripts(w producerWorkflow, name string) string {
	var s strings.Builder
	for _, step := range w.Jobs[name].Steps {
		s.WriteString(step.Run)
	}
	return s.String()
}
func c1Contract(w producerWorkflow, name string, stage, signer bool) error {
	job, ok := w.Jobs[name]
	if !ok {
		return fmt.Errorf("missing %s", name)
	}
	need, mode, env, timeout := "dispatch_contract", "paired-input-provenance", "agentplugins-release", 20
	if stage {
		mode, env = "paired-stage", "npm-agentplugins"
		timeout = 30
	}
	if signer {
		timeout = 20
		if stage {
			need = "paired_stage"
		} else {
			need = "paired_input_admission"
		}
	}
	condition := "${{ success() && github.event_name == 'workflow_dispatch' && inputs.producer_mode == '" + mode + "'"
	if stage {
		condition += " && inputs.publish == false"
	}
	condition += " && needs." + need + ".result == 'success' }}"
	if job.If != condition || !reflect.DeepEqual(c1Needs(w, name), []string{need}) {
		return fmt.Errorf("%s mode/status/needs", name)
	}
	permissions := map[string]string{"contents": "read", "actions": "read"}
	if signer {
		permissions["id-token"] = "write"
		permissions["attestations"] = "write"
	} else if stage {
		permissions["attestations"] = "read"
	}
	if !reflect.DeepEqual(c1Permissions(w, name), permissions) {
		return fmt.Errorf("%s effective permissions", name)
	}
	if (signer && job.Environment != env) || (!signer && job.Environment != nil) || job.Runner != "ubuntu-24.04" || job.Timeout != timeout {
		return fmt.Errorf("%s execution boundary", name)
	}
	if len(job.Steps) < 3 || job.Steps[0].Name != "C1 preflight" || job.Steps[0].Uses != "" {
		return fmt.Errorf("%s preflight ordering", name)
	}
	attest, uploads, admission, recheck := -1, 0, -1, -1
	for index, step := range job.Steps {
		if step.Continue || (step.If != "" && step.If != "${{ success() }}") {
			return fmt.Errorf("%s step bypass", name)
		}
		if step.ID == "stage" {
			admission = index
		}
		if step.ID == "recheck" {
			recheck = index
		}
		if strings.HasPrefix(step.Uses, "actions/attest@") {
			attest = index
			if step.Uses != "actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6" || step.With["subject-path"] != "${{ steps.stage.outputs.subjects }}" {
				return fmt.Errorf("exact subject signing")
			}
		}
		if strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			uploads++
			if step.Uses != "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a" || step.ID != "upload" || step.With["if-no-files-found"] != "error" || step.With["retention-days"] != 7 || step.With["overwrite"] != nil {
				return fmt.Errorf("immutable upload")
			}
			id := "stage"
			if signer {
				id = "recheck"
			}
			if step.With["path"] != "${{ steps."+id+".outputs.payload }}" {
				return fmt.Errorf("fixed upload payload")
			}
		}
	}
	if signer && (attest <= admission || admission < 0 || recheck <= attest) {
		return fmt.Errorf("independent admission/sign/recheck order")
	}
	if !signer && attest != -1 {
		return fmt.Errorf("unexpected signing")
	}
	expectedUploads := 0
	if stage != signer {
		expectedUploads = 1
	}
	if uploads != expectedUploads {
		return fmt.Errorf("upload lifecycle")
	}
	body := c1Scripts(w, name)
	for _, forbidden := range []string{"npm publish", "npm install", "--read-stage", "allow_incomplete", "needs.paired_input_admission.outputs", "needs.paired_stage.outputs.accepted"} {
		if strings.Contains(body, forbidden) {
			return fmt.Errorf("forbidden C1 effect or trust substitution: %s", forbidden)
		}
	}
	if signer && (!strings.Contains(body, "assert.equal(previous.root, e.SIGNING_ROOT)") || !strings.Contains(body, "pins(previous.root)") || !strings.Contains(body, "result.root = previous.root") || !strings.Contains(body, "fs.mkdtempSync")) {
		return fmt.Errorf("original signing root recheck")
	}
	return nil
}
func TestC1InputProvenanceWorkflowContract(t *testing.T) {
	w := readProducerWorkflow(t, "agentplugins-release.yml")
	if len(w.Jobs) != 11 || len(w.On.Dispatch.Inputs) != 10 || w.On.Dispatch.Inputs["producer_mode"].Type != "choice" || !w.On.Dispatch.Inputs["producer_mode"].Required {
		t.Fatal("closed release inputs/jobs")
	}
	for _, name := range []string{"paired_input_admission", "paired_input_attestation"} {
		if err := c1Contract(w, name, false, name == "paired_input_attestation"); err != nil {
			t.Fatal(err)
		}
		body := c1Scripts(w, name)
		for _, text := range []string{"--produce-inputs", "preparation:", "workflow_sha: e.SOURCE_SHA", "assert.equal(payload.length, 21)", "'native-inputs.json'", "'preparation-run.json', 'candidate-identity.json'"} {
			if !strings.Contains(body, text) {
				t.Fatalf("%s missing %s", name, text)
			}
		}
	}
	if len(w.Jobs["paired_input_admission"].Outputs) != 1 || len(w.Jobs["paired_input_attestation"].Outputs) != 3 {
		t.Fatal("selector-only outputs")
	}
}
func TestC1PublicStageWorkflowContract(t *testing.T) {
	w := readProducerWorkflow(t, "agentplugins-npm-publish.yml")
	if len(w.Jobs) != 6 || len(w.On.Dispatch.Inputs) != 7 || strings.Join(w.On.Dispatch.Inputs["producer_mode"].Options, ",") != "legacy,paired-stage" || w.On.Dispatch.Inputs["producer_mode"].Default != "legacy" || w.On.Dispatch.Inputs["publish"].Type != "boolean" {
		t.Fatal("closed stage inputs/jobs")
	}
	for _, name := range []string{"paired_stage", "paired_stage_attestation"} {
		if err := c1Contract(w, name, true, name == "paired_stage_attestation"); err != nil {
			t.Fatal(err)
		}
		body := c1Scripts(w, name)
		if !strings.Contains(body, "assert.equal(payload.length, 3)") || !strings.Contains(body, "input_file") || !strings.Contains(body, "Buffer.from(e.NATIVE_INPUTS, 'utf8')") {
			t.Fatal("three retained files and Buffer transport")
		}
	}
	if strings.Count(c1Scripts(w, "paired_stage"), "--stage-prepublication") != 1 || strings.Count(c1Scripts(w, "paired_stage_attestation"), "--validate-unsigned-stage") != 2 {
		t.Fatal("pack once and independent same-run validation")
	}
	for name, expected := range map[string][]string{"prepare": {"dispatch_contract"}, "publish": {"prepare"}, "verify-public": {"prepare", "publish"}} {
		if !reflect.DeepEqual(c1Needs(w, name), expected) {
			t.Fatalf("legacy needs changed %s", name)
		}
	}
	for _, name := range []string{"prepare", "publish", "verify-public"} {
		if !strings.Contains(w.Jobs[name].If, "inputs.producer_mode == 'legacy'") || !strings.Contains(w.Jobs[name].If, "github.event_name == 'workflow_dispatch'") || len(c1Needs(w, name)) == 0 {
			t.Fatal("legacy isolation", name)
		}
	}
}
func TestC1WorkflowFailureReachability(t *testing.T) {
	for _, file := range []string{"agentplugins-release.yml", "agentplugins-npm-publish.yml"} {
		w := readProducerWorkflow(t, file)
		modes := []string{"binary-only", "paired-preparation", "paired-promotion", "paired-input-provenance", "legacy", "paired-stage", "unknown"}
		legacyPermissions := map[string]map[string]string{
			"dispatch_contract": {"contents": "read"}, "validate": {"checks": "read", "contents": "read", "pull-requests": "read"},
			"build": {"contents": "read"}, "stage-draft": {"contents": "write", "id-token": "write", "attestations": "write", "artifact-metadata": "write"},
			"platform-proof": {"contents": "read", "attestations": "read"}, "promote-release": {"contents": "write", "attestations": "read"},
			"paired-preparation": {"contents": "read"}, "paired-promotion-admission": {"contents": "read", "actions": "read"},
			"paired-sign-and-promote": {"contents": "write", "actions": "read", "id-token": "write", "attestations": "write", "artifact-metadata": "write"},
			"prepare":                 {"contents": "read", "attestations": "read"}, "publish": {"contents": "read", "id-token": "write"},
			"verify-public": {"contents": "read", "attestations": "read"},
		}
		for name, job := range w.Jobs {
			if expected, ok := legacyPermissions[name]; ok && !reflect.DeepEqual(c1Permissions(w, name), expected) {
				t.Fatalf("effective legacy permissions %s", name)
			}
			if name == "dispatch_contract" {
				continue
			}
			for _, mode := range modes {
				for _, event := range []string{"workflow_dispatch", "workflow_run"} {
					for _, status := range []string{"success", "failure", "cancelled", "skipped", ""} {
						for _, publish := range []bool{true, false} {
							values := map[string]any{"github.event_name": event, "inputs.producer_mode": mode, "inputs.publish": publish,
								"success": status == "success", "failure": status == "failure", "cancelled": status == "cancelled", "always": true}
							for dependency := range w.Jobs {
								values["needs."+dependency+".result"] = status
							}
							// GitHub's implicit success() applies when no status function is present.
							reachable := workflowCondition(t, job.If, values)
							if !strings.Contains(job.If, "success()") {
								reachable = reachable && status == "success"
							}
							for _, dep := range c1Needs(w, name) {
								reachable = reachable && values["needs."+dep+".result"] == "success"
							}
							if (status != "success") && reachable {
								t.Fatalf("%s reachable after %s", name, status)
							}
							if strings.HasPrefix(name, "paired_input_") || strings.HasPrefix(name, "paired_stage") {
								expected := status == "success" && event == "workflow_dispatch" && ((strings.HasPrefix(name, "paired_input_") && mode == "paired-input-provenance") || (strings.HasPrefix(name, "paired_stage") && mode == "paired-stage" && !publish))
								if reachable != expected {
									t.Fatalf("%s reachability %s %s %s %v", name, mode, event, status, publish)
								}
								for _, step := range job.Steps {
									for _, state := range []string{"failure", "cancelled", "skipped", ""} {
										stopped := map[string]any{"success": false, "failure": state == "failure", "cancelled": state == "cancelled", "always": true}
										if workflowCondition(t, step.If, stopped) {
											t.Fatalf("sensitive step survives %s", state)
										}
									}

									if workflowCondition(t, step.If, values) && reachable && step.Continue {
										t.Fatal("sensitive step tolerates failure")
									}
								}
								if err := c1Contract(w, name, strings.HasPrefix(name, "paired_stage"), strings.HasSuffix(name, "attestation")); err != nil {
									t.Fatal(err)
								}
							}
							if file == "agentplugins-npm-publish.yml" && (name == "prepare" || name == "publish" || name == "verify-public") {
								expected := status == "success" && event == "workflow_dispatch" && mode == "legacy" && (name == "prepare" || publish)
								if reachable != expected {
									t.Fatalf("legacy reachability %s %s %s %s %v", name, mode, event, status, publish)
								}
							}
							if mode == "paired-stage" && (name == "prepare" || name == "publish" || name == "verify-public") && reachable {
								t.Fatal("paired stage reaches legacy")
							}
						}
					}
				}
			}
		}
		name := "paired_input_attestation"
		if file == "agentplugins-npm-publish.yml" {
			name = "paired_stage_attestation"
		}
		original := w.Jobs[name]
		for _, mutation := range []string{"mode", "or true", "always", "needs", "output authorization", "permissions", "continue"} {
			job := original
			job.Steps = append(job.Steps[:0:0], job.Steps...)
			switch mutation {
			case "mode":
				job.If = "${{ success() }}"
			case "or true":
				job.If = strings.TrimSuffix(job.If, " }}") + " || true }}"
			case "always":
				job.If = "${{ always() }}"
			case "needs":
				job.Needs = nil
			case "output authorization":
				job.If = "${{ needs.paired_stage.outputs.accepted == 'true' }}"
			case "permissions":
				job.Permissions = map[string]string{"contents": "write"}
			case "continue":
				job.Steps[0].Continue = true
			}
			w.Jobs[name] = job
			if c1Contract(w, name, file == "agentplugins-npm-publish.yml", true) == nil {
				t.Fatal("mutation accepted", mutation)
			}
		}
		w.Jobs[name] = original
	}
}
func TestC1WorkflowPreflightNoEffects(t *testing.T) {
	for _, file := range []string{"agentplugins-release.yml", "agentplugins-npm-publish.yml"} {
		w := readProducerWorkflow(t, file)
		stage := file == "agentplugins-npm-publish.yml"
		mode := "paired-input-provenance"
		if stage {
			mode = "paired-stage"
		}
		good := map[string]string{"PRODUCER_MODE": mode, "TAG": "agentplugins-v0.1.54", "KIT_VERSION": "2.0.0", "SOURCE_SHA": strings.Repeat("a", 40),
			"GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": "777genius/universal-agent-plugins",
			"GITHUB_SHA": strings.Repeat("a", 40), "GITHUB_WORKFLOW_SHA": strings.Repeat("a", 40), "GITHUB_REF": "refs/tags/agentplugins-v0.1.54",
			"GITHUB_WORKFLOW_REF": "777genius/universal-agent-plugins/.github/workflows/" + file + "@refs/tags/agentplugins-v0.1.54",
			"GITHUB_RUN_ID":       "21", "GITHUB_RUN_ATTEMPT": "2", "PUBLISH": "false", "NATIVE_INPUTS": "{}\n",
			"INPUT_ARTIFACT":  `{"run_id":11,"run_attempt":1,"artifact_id":31,"artifact_sha256":"` + strings.Repeat("b", 64) + `"}`,
			"PREPARATION_RUN": "11", "PREPARATION_ATTEMPT": "1", "PREPARATION_ARTIFACT": "31", "PREPARATION_DIGEST": strings.Repeat("b", 64), "PROMOTION_RECORD": "", "PROMOTION_OPERATION": "promote"}
		names := []string{"dispatch_contract", "paired_input_admission", "paired_input_attestation"}
		if stage {
			names = []string{"dispatch_contract", "paired_stage", "paired_stage_attestation"}
		}
		for _, name := range names {
			good["GITHUB_JOB"] = name
			good["STAGE_ARTIFACT_ID"] = "901"
			good["STAGE_ARTIFACT_SHA256"] = strings.Repeat("c", 64)
			good["STAGE_SHA256"] = strings.Repeat("d", 64)
			job := w.Jobs[name]
			if len(job.Steps) == 0 {
				t.Fatal("missing preflight")
			}
			for _, step := range job.Steps {
				if step.Run != "" {
					cmd := exec.Command("/bin/bash", "-n")
					cmd.Stdin = strings.NewReader(step.Run)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("shell syntax %s: %v %s", name, err, out)
					}
				}
			}
			cases := []map[string]string{{}}
			for _, key := range []string{"PRODUCER_MODE", "TAG", "KIT_VERSION", "SOURCE_SHA", "GITHUB_EVENT_NAME", "GITHUB_ACTIONS", "GITHUB_REPOSITORY", "GITHUB_SHA", "GITHUB_WORKFLOW_SHA", "GITHUB_REF", "GITHUB_WORKFLOW_REF", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT"} {
				for _, bad := range []string{"", "invalid", "$(touch injected)", "bad\nvalue"} {
					cases = append(cases, map[string]string{key: bad})
				}
			}
			if stage {
				for _, bad := range []map[string]string{{"PUBLISH": "true"}, {"INPUT_ARTIFACT": "{}"}, {"INPUT_ARTIFACT": strings.ReplaceAll(good["INPUT_ARTIFACT"], `"run_attempt":1`, `"run_attempt":1001`)}, {"NATIVE_INPUTS": ""}, {"PRODUCER_MODE": "legacy"}} {
					cases = append(cases, bad)
				}
			} else {
				for _, key := range []string{"PREPARATION_RUN", "PREPARATION_ATTEMPT", "PREPARATION_ARTIFACT", "PREPARATION_DIGEST", "PROMOTION_RECORD", "PROMOTION_OPERATION"} {
					cases = append(cases, map[string]string{key: "invalid"})
				}
			}
			if name != "dispatch_contract" {
				cases = append(cases, map[string]string{"GITHUB_JOB": "publish"})
			}
			if name == "paired_stage_attestation" {
				for _, key := range []string{"STAGE_ARTIFACT_ID", "STAGE_ARTIFACT_SHA256", "STAGE_SHA256"} {
					cases = append(cases, map[string]string{key: "invalid"})
				}
			}
			for index, changes := range cases {
				dir := t.TempDir()
				bin := filepath.Join(dir, "bin")
				if err := os.Mkdir(bin, 0700); err != nil {
					t.Fatal(err)
				}
				for _, tool := range []string{"node", "git", "npm", "gh", "tar", "python3", "curl"} {
					if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/bash\necho effect >> \"$MARKER\"\nexit 93\n"), 0700); err != nil {
						t.Fatal(err)
					}
				}
				cmd := exec.Command("/bin/bash", "-c", job.Steps[0].Run)
				cmd.Dir = dir
				cmd.Env = []string{"PATH=/usr/local/bin:" + bin + ":/usr/bin:/bin", "MARKER=" + filepath.Join(dir, "effect")}
				for key, value := range good {
					if replacement, ok := changes[key]; ok {
						value = replacement
					}
					cmd.Env = append(cmd.Env, key+"="+value)
				}
				output, err := cmd.CombinedOutput()
				if (err == nil) != (index == 0) {
					t.Fatalf("%s preflight case %v: %v %s", name, changes, err, output)
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 1 || entries[0].Name() != "bin" {
					t.Fatalf("preflight produced effects: %v %v", entries, err)
				}
			}
		}
	}
}
