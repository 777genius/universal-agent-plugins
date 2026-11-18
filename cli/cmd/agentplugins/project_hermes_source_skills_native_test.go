package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Acquisition/security are the existing inert TLS fixtures. Cobra composition,
// detection, installer, native trust commands and installed-runtime discovery
// are real. This source test does not qualify a separately released executable.
func TestHermesSourceSkillsConfirmedAddsNative(t *testing.T) {
	input := hermesSourceSkillsSelection(t)
	for _, kind := range []string{"raw-https", "saved-git"} {
		t.Run(kind, func(t *testing.T) { input.scenario(t, kind) })
	}
	if !t.Failed() {
		if got := input.run(t, "uap-clean-after", input.checkout, "git", "status", "--porcelain"); len(got) != 0 {
			t.Fatal("native acceptance changed source checkout")
		}
		if got := strings.TrimSpace(string(input.run(t, "vendor-pin-after", input.source, "git", "rev-parse", "HEAD"))); got != input.provenance["hermes_pin"] {
			t.Fatal("native acceptance changed vendor source selection")
		}
		status := input.run(t, "vendor-status-after", input.source, "git", "status", "--porcelain")
		digest := sha256.Sum256(status)
		if hex.EncodeToString(digest[:]) != input.provenance["hermes_status_sha256"] ||
			hermesSourceHash(t, input.launcher) != input.provenance["launcher_sha256"] ||
			hermesSourceHash(t, input.native[0]) != input.provenance["native_runtime_sha256"] {
			t.Fatal("native source acceptance changed its vendor source or selected runtime")
		}
	}
	if !t.Failed() && !slices.Equal(input.completed, []string{"raw-https", "saved-git"}) {
		t.Fatal("both genuine native source scenarios must execute")
	}
}

type hermesSourceSkills struct {
	ctx                                context.Context
	root, profile, launcher, observer  string
	hostPython, checkout, source, head string
	environment                        map[string]string
	native                             []string
	completed                          []string
	commands                           []map[string]any
	provenance                         map[string]string
}

func hermesSourceSkillsSelection(t *testing.T) *hermesSourceSkills {
	t.Helper()
	if os.Getenv("UAP_HERMES_SOURCE_SKILLS_NATIVE") == "" {
		t.Skip("genuine source Skills add acceptance runs in the existing native workflow")
	}
	if os.Getenv("UAP_HERMES_SOURCE_SKILLS_NATIVE") != "1" || os.Getenv("RELEASE_TAG") != "" ||
		os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" {
		t.Fatal("source-only actual GitHub-hosted native selection required")
	}
	input := &hermesSourceSkills{root: os.Getenv("HERMES_SOURCE_SKILLS_EVIDENCE"),
		profile: os.Getenv("HERMES_HOME"), launcher: os.Getenv("HERMES_BIN"),
		hostPython: os.Getenv("HERMES_SOURCE_HOST_PYTHON"), checkout: os.Getenv("GITHUB_WORKSPACE"),
		head: os.Getenv("HERMES_NATIVE_UAP_HEAD"), environment: map[string]string{}}
	input.source = filepath.Join(os.Getenv("RUNNER_TEMP"), "hermes-agent")
	input.observer = filepath.Join(input.checkout, "scripts", "e2e", "hermes-native.py")
	if relative, err := filepath.Rel(os.Getenv("RUNNER_TEMP"), input.profile); err != nil || relative == "." ||
		relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		t.Fatal("native source profile must stay inside the disposable runner")
	}
	for _, path := range []string{input.root, input.profile, input.launcher, input.hostPython, input.checkout} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			t.Fatal("native source acceptance requires explicit absolute paths", path)
		}
	}
	for _, key := range []string{"HOME", "USERPROFILE", "HERMES_HOME", "PATH", "APPDATA", "LOCALAPPDATA",
		"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME"} {
		input.environment[key] = os.Getenv(key)
	}
	hard, ok := t.Deadline()
	if !ok {
		t.Fatal("fixed native test deadline required")
	}
	deadline, err := hermesColdDeadline(os.Getenv("HERMES_SOURCE_SKILLS_DEADLINE_UNIX_MS"), time.Now(), hard)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	input.ctx = ctx
	t.Cleanup(cancel)
	input.native = []string{input.launcher}
	if path := os.Getenv("HERMES_NATIVE_RUNTIME_COMMAND_FILE"); path != "" {
		body, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf}), &input.native) != nil ||
			len(input.native) < 4 || !filepath.IsAbs(input.native[0]) {
			t.Fatal("official installed runtime command is unavailable", err)
		}
	}
	input.provenance = map[string]string{"uap_head": input.head, "distribution": "official source install",
		"qualification":   "public Cobra with fixture acquisition/security; real native runtime",
		"launcher_sha256": hermesSourceHash(t, input.launcher), "uap_sha256": hermesSourceHash(t, os.Getenv("UAP_BIN"))}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input.provenance["test_binary_sha256"] = hermesSourceHash(t, executable)
	input.provenance["native_runtime_sha256"] = hermesSourceHash(t, input.native[0])
	if got := strings.TrimSpace(string(input.run(t, "uap-head", input.checkout, "git", "rev-parse", "HEAD"))); got != input.head {
		t.Fatal("actual source differs from expected head", got)
	}
	if got := input.run(t, "uap-clean", input.checkout, "git", "status", "--porcelain"); len(got) != 0 {
		t.Fatal("native source checkout is dirty")
	}
	input.provenance["uap_tree"] = strings.TrimSpace(string(input.run(t, "uap-tree", input.checkout, "git", "rev-parse", "HEAD^{tree}")))
	input.provenance["hermes_pin"] = strings.TrimSpace(string(input.run(t, "vendor-pin", input.source, "git", "rev-parse", "HEAD")))
	if input.provenance["hermes_pin"] != "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19" {
		t.Fatal("unqualified Hermes source pin")
	}
	status := input.run(t, "vendor-status", input.source, "git", "status", "--porcelain")
	digest := sha256.Sum256(status)
	input.provenance["hermes_status_sha256"] = hex.EncodeToString(digest[:])
	// Register before scenario cleanups: a successful report is written only
	// after their servers and held fixture resources have returned.
	t.Cleanup(func() {
		packet := map[string]any{"completed": !t.Failed() && len(input.completed) == 2,
			"scenarios": input.completed, "provenance": input.provenance, "commands": input.commands,
			"native_discovery": !t.Failed() && len(input.completed) == 2,
			"tools_call":       false, "real_model_inference": false, "vendor_authentication": false}
		body, err := json.MarshalIndent(packet, "", "  ")
		if err == nil {
			err = os.WriteFile(filepath.Join(input.root, "report.json"), append(body, '\n'), 0600)
		}
		if err != nil {
			t.Error(err)
		}
	})
	return input
}

func hermesSourceHash(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || len(body) > 128<<20 {
		t.Fatal("native executable hash unavailable or oversized", err)
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func (input *hermesSourceSkills) run(t *testing.T, label, cwd string, argv ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(input.ctx, time.Minute)
	defer cancel()
	result, err := (processadapter.OS{}).Run(ctx, legacyports.Command{
		Argv: argv, Dir: cwd, Env: os.Environ(), StdoutLimitBytes: 1 << 20,
	})
	for suffix, body := range map[string][]byte{"stdout": result.Stdout, "stderr": result.Stderr} {
		if writeErr := os.WriteFile(filepath.Join(input.root, label+"."+suffix), body, 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	input.commands = append(input.commands, map[string]any{"label": label, "argv": argv, "cwd": cwd,
		"exit_code": result.ExitCode, "cleanup_complete": result.CleanupComplete,
		"no_child_started": result.NoChildStarted, "error": fmt.Sprint(err)})
	if err != nil || result.ExitCode != 0 || !result.CleanupComplete || result.NoChildStarted {
		t.Fatalf("native source command failed or lacks settlement: %s: %v; exit %d\n%s\n%s",
			label, err, result.ExitCode, result.Stdout, result.Stderr)
	}
	return result.Stdout
}

func (input *hermesSourceSkills) nativeRun(t *testing.T, label, cwd string, args ...string) []byte {
	t.Helper()
	return input.run(t, label, cwd, append(slices.Clone(input.native), args...)...)
}

func (input *hermesSourceSkills) scenario(t *testing.T, kind string) {
	t.Helper()
	var source projectSkillsSource
	if kind == "raw-https" {
		source = newProjectSkillsSource(t, "https")
		source.transport.ctx, source.transport.tempRoot = input.ctx, input.root
	} else {
		transport := newProjectJourneyTransportIn(input.ctx, t, "git", input.root)
		if err := os.Remove(filepath.Join(transport.repo, "mcp.json")); err != nil {
			t.Fatal(err)
		}
		transport.git(t, transport.repo, "add", ".")
		transport.git(t, transport.repo, "commit", "-m", "native Skills-only saved-add source")
		transport.git(t, transport.repo, "push", "--quiet", transport.remote, "main")
		source = projectSkillsSource{input: transport.endpoint, transport: transport}
	}
	journey := newProjectJourney(t, source.transport)
	// Git acquisition owns a separate inert Git HOME. Real Hermes keeps its
	// original installed home and dependency root throughout both scenarios.
	for key, value := range input.environment {
		t.Setenv(key, value)
	}
	t.Setenv("PYTHONDONTWRITEBYTECODE", "1")
	journey.app = configureHermesColdJourney(input.ctx, t, journey.app, input.profile, input.launcher)
	for _, project := range []string{journey.a, journey.b} {
		source.transport.git(t, project, "init", "-b", "main")
	}
	projectJourneyWrite(t, filepath.Join(journey.a, "foreign.txt"), "retained project A\n")
	protected := securityFixtureTree(t, journey.b)
	foreign := securityFixtureTree(t, filepath.Join(input.profile, "skills"))
	config := input.configuration(t)
	baselineTrust := input.trusted(t, kind+"-before", journey.a)
	args := []string{"add", source.input, "--alias", "docs", "--target", "hermes", "--yes"}
	if kind == "raw-https" {
		args = []string{"skills", "add", source.input, "--skill", "docs", "--target", "hermes", "--no-ui", "--yes"}
		before := securityFixtureTree(t, journey.a)
		beforeConfig := projectJourneyRead(t, filepath.Join(input.profile, "config.yaml"))
		input.cli(t, journey, kind+"-dry-run", append(slices.Clone(args), "--dry-run")...)
		if !reflect.DeepEqual(before, securityFixtureTree(t, journey.a)) ||
			beforeConfig != projectJourneyRead(t, filepath.Join(input.profile, "config.yaml")) ||
			!reflect.DeepEqual(baselineTrust, input.trusted(t, kind+"-after-dry-run", journey.a)) {
			t.Fatal("raw preview installed files or changed native trust")
		}
	}
	input.cli(t, journey, kind+"-fresh", args...)
	installation, binding := journey.owned(t, journey.a)
	if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatal("native observation promoted production Prepared status", binding)
	}
	intent := journey.assertPortablePair(t, journey.a)
	input.observe(t, kind+"-fresh-observe", journey.a)
	input.nativeRun(t, kind+"-untrust", journey.a, "skills", "untrust", journey.a)
	if !reflect.DeepEqual(baselineTrust, input.trusted(t, kind+"-revoked", journey.a)) {
		t.Fatal("native user untrust changed another trusted root")
	}
	input.cli(t, journey, kind+"-sync", "deps", "sync", "--locked", "--yes")
	if !reflect.DeepEqual(baselineTrust, input.trusted(t, kind+"-after-sync", journey.a)) {
		t.Fatal("maintenance restored revoked native trust")
	}
	input.cli(t, journey, kind+"-repeat", args...)
	after, selected := journey.owned(t, journey.a)
	if installation.InstallationID != after.InstallationID || binding.ClientBindingID != selected.ClientBindingID ||
		installation.Source != after.Source || intent != journey.assertPortablePair(t, journey.a) || selected.Activation != domain.ActivationPrepared {
		t.Fatal("explicit same-source re-add replaced ownership, pin, intent or Prepared semantics")
	}
	input.observe(t, kind+"-repeat-observe", journey.a)
	expectedTrust := append(slices.Clone(baselineTrust), journey.a)
	slices.Sort(expectedTrust)
	if !reflect.DeepEqual(expectedTrust, input.trusted(t, kind+"-regranted", journey.a)) {
		t.Fatal("confirmed repeat did not grant exactly its selected Git root")
	}
	input.cli(t, journey, kind+"-remove", "remove", "docs", "--target", "hermes", "--yes")
	projectJourneyAbsent(t, filepath.Join(journey.a, ".hermes", "skills", "docs"))
	if !reflect.DeepEqual(expectedTrust, input.trusted(t, kind+"-after-remove", journey.a)) {
		t.Fatal("UAP removal revoked persistent native trust")
	}
	if !reflect.DeepEqual(protected, securityFixtureTree(t, journey.b)) ||
		!reflect.DeepEqual(foreign, securityFixtureTree(t, filepath.Join(input.profile, "skills"))) ||
		projectJourneyRead(t, filepath.Join(journey.a, "foreign.txt")) != "retained project A\n" ||
		!reflect.DeepEqual(config, input.configuration(t)) {
		t.Fatal("source add lifecycle changed unselected Project/User content or unrelated settings")
	}
	// Deliberate native user cleanup, after proving UAP removal retains trust.
	input.nativeRun(t, kind+"-cleanup-untrust", journey.a, "skills", "untrust", journey.a)
	if !reflect.DeepEqual(baselineTrust, input.trusted(t, kind+"-cleanup-trust", journey.a)) {
		t.Fatal("test-owned native trust cleanup changed a preexisting root")
	}
	input.completed = append(input.completed, kind)
}

func (input *hermesSourceSkills) cli(t *testing.T, journey projectJourney, label string, args ...string) {
	t.Helper()
	var output, stderr bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &stderr, false
	command := agentpluginscli.NewRoot(app)
	command.SetContext(input.ctx)
	command.SetOut(&output)
	command.SetErr(&stderr)
	command.SetArgs(append(args, "--scope", "project", "--project-root", journey.a, "--format", "json"))
	err := command.Execute()
	for suffix, body := range map[string][]byte{"stdout": output.Bytes(), "stderr": stderr.Bytes()} {
		if writeErr := os.WriteFile(filepath.Join(input.root, label+".cobra."+suffix), body, 0600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err != nil {
		t.Fatalf("public Cobra source journey %s: %v\n%s\n%s", label, err, output.String(), stderr.String())
	}
}

func (input *hermesSourceSkills) observe(t *testing.T, label, project string) {
	t.Helper()
	expected := filepath.Join(project, ".hermes", "skills", "docs")
	input.run(t, label, project, input.hostPython, input.observer, "selected", "docs", expected, "UAP-PROJECT-SOURCE-FIXTURE")
}

func (input *hermesSourceSkills) trusted(t *testing.T, label, project string) []string {
	t.Helper()
	body := input.nativeRun(t, label, project, "config", "get", "--json", "skills.trusted_project_dirs")
	var result []string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal("native trusted roots are not a JSON path list", err)
	}
	// Earlier fixture roots may already be removed. Their exact native trust
	// entries still belong to the user and must survive without being adopted.
	if result == nil {
		result = []string{}
	}
	slices.Sort(result)
	return result
}

func (input *hermesSourceSkills) configuration(t *testing.T) map[string]any {
	t.Helper()
	config, err := hermesColdConfiguration([]byte(projectJourneyRead(t, filepath.Join(input.profile, "config.yaml"))))
	if err != nil {
		t.Fatal(err)
	}
	delete(config, "_config_version")
	if skills, ok := config["skills"].(map[string]any); ok {
		delete(skills, "trusted_project_dirs")
		if len(skills) == 0 {
			delete(config, "skills")
		}
	}
	return config
}
