package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	if mode.Default != "binary-only" || strings.Join(mode.Options, ",") != "binary-only,paired-preparation" {
		t.Fatal("default binary-only dispatch contract changed")
	}
	if len(w.Permissions) != 1 || w.Permissions["contents"] != "read" {
		t.Fatal("workflow must default to contents-read")
	}
	if len(w.Jobs) != 6 {
		t.Fatal("review every new producer job for preparation reachability")
	}
	for name, job := range w.Jobs {
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
		for _, job := range w.Jobs {
			if !strings.Contains(job.If, "github.event.workflow_run.conclusion == 'success'") {
				t.Fatalf("%s can publish after rejected legacy major-2 run", name)
			}
		}
	}
	w := readProducerWorkflow(t, "agentplugins-npm-publish.yml")
	if len(w.On.Run.Workflows) != 0 {
		t.Fatal("agentplugins npm publication must require separate dispatch")
	}
}
