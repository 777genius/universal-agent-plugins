package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
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
				Default string   `yaml:"default"`
				Options []string `yaml:"options"`
			} `yaml:"inputs"`
		} `yaml:"workflow_dispatch"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		If          string            `yaml:"if"`
		Environment any               `yaml:"environment"`
		Needs       any               `yaml:"needs"`
		Uses        string            `yaml:"uses"`
		Permissions map[string]string `yaml:"permissions"`
		Env         map[string]string `yaml:"env"`
		Steps       []struct {
			Name string            `yaml:"name"`
			Run  string            `yaml:"run"`
			Uses string            `yaml:"uses"`
			With map[string]any    `yaml:"with"`
			Env  map[string]string `yaml:"env"`
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
	if mode.Default != "binary-only" || strings.Join(mode.Options, ",") != "binary-only,paired-preparation,paired-promotion" {
		t.Fatal("default binary-only dispatch contract changed")
	}
	if len(w.Permissions) != 1 || w.Permissions["contents"] != "read" {
		t.Fatal("workflow must default to contents-read")
	}
	if len(w.Jobs) != 8 {
		t.Fatal("review every new producer job for preparation reachability")
	}
	for name, job := range w.Jobs {
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
	expression = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(expression), "${{"), "}}"))
	values := map[string]string{"github.event_name": event, "github.event.workflow_run.conclusion": conclusion,
		"vars.NPM_PUBLISH_READY": "true", "vars.PYPI_TRUSTED_PUBLISHING_READY": "true", "inputs.producer_mode": "paired-promotion"}
	words := regexp.MustCompile(`'[^']*'|[a-zA-Z_][a-zA-Z0-9_.]*`)
	expression = words.ReplaceAllStringFunc(expression, func(s string) string {
		if strings.HasPrefix(s, "'") {
			return strconv.Quote(s[1 : len(s)-1])
		}
		if value, ok := values[s]; ok {
			return strconv.Quote(value)
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
