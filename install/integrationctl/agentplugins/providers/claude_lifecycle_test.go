package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestClaudeSkillsDirAddUpdateRemoveUseOnlyExactListVerification(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientClaude)
	request.BackendExecutable = "/test/bin/claude"
	request.Client.ConfigRoot = filepath.Join(t.TempDir(), "claude-config")
	request.Delivery.ActivePath = filepath.Join(request.Client.ConfigRoot, "skills", "demo-managed")
	request.Delivery.OwnedBase = filepath.Dir(request.Delivery.ActivePath)
	request.Plan.TargetAnchor = request.Client.ConfigRoot
	request.Plan.TargetRoot = request.Delivery.OwnedBase
	request.Plan.ActivePath = request.Delivery.ActivePath
	if err := os.MkdirAll(request.Delivery.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	listing := claudeListing(request.DeclaredName, request.Delivery.ActivePath, true)
	runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: []byte(listing)}
	}}
	activator := Activator{Runner: runner}

	for _, replacing := range []bool{false, true} {
		request.Replacing = replacing
		outcome, err := activator.Activate(context.Background(), request)
		if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
			t.Fatalf("replacing=%t outcome=%+v err=%v", replacing, outcome, err)
		}
	}
	remove, err := activator.Deactivate(context.Background(), domain.DeactivationRequest{
		Client: request.Client, DeclaredName: request.DeclaredName, CurrentActivation: domain.ActivationActive,
		Confirmed: true, BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath,
	})
	if err != nil || !remove.ArtifactRemovalAllowed || !remove.ExternalRemovalComplete {
		t.Fatalf("remove=%+v err=%v", remove, err)
	}
	want := [][]string{
		{"/test/bin/claude", "plugin", "list", "--json"},
		{"/test/bin/claude", "plugin", "list", "--json"},
		{"/test/bin/claude", "plugin", "list", "--json"},
	}
	if got := commandArgv(runner.commands); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want list-only lifecycle %#v", got, want)
	}
	for _, command := range runner.commands {
		assertBoundedClaudeProbe(t, command, request.Client.ConfigRoot)
	}
}

func TestClaudeProbeEnvironmentRejectsDuplicateAllowedVariableAndDropsOverrides(t *testing.T) {
	home := t.TempDir()
	environment, gotHome, err := boundedClaudeProbeEnvironmentFrom([]string{
		"HOME=" + home,
		"PATH=/usr/bin:/bin",
		"NODE_OPTIONS=--require=attacker.js",
		"NODE_PATH=/attacker",
		"CLAUDE_CONFIG_DIR=/attacker",
		"ANTHROPIC_API_KEY=secret",
		"PLUGIN_KIT_AI_FAKE_CLAUDE_STATE=/attacker/state",
		"PLUGIN_KIT_AI_FAKE_CLAUDE_LOG=/attacker/log",
		"PLUGIN_KIT_AI_FAKE_CLAUDE_MARKETPLACES=/attacker/marketplaces",
		"PLUGIN_KIT_AI_FAKE_CLAUDE_PRESERVE_REFS=attacker@marketplace",
	})
	if err != nil || gotHome != home {
		t.Fatalf("bounded environment = %v, home=%q, err=%v", environment, gotHome, err)
	}
	for _, forbidden := range []string{
		"NODE_OPTIONS", "NODE_PATH", "ANTHROPIC_API_KEY", "CLAUDE_CONFIG_DIR",
		"PLUGIN_KIT_AI_FAKE_CLAUDE_STATE", "PLUGIN_KIT_AI_FAKE_CLAUDE_LOG",
		"PLUGIN_KIT_AI_FAKE_CLAUDE_MARKETPLACES", "PLUGIN_KIT_AI_FAKE_CLAUDE_PRESERVE_REFS",
	} {
		if _, ok := environmentValue(environment, forbidden); ok {
			t.Fatalf("dangerous override %s survived: %v", forbidden, environment)
		}
	}
	if _, _, err := boundedClaudeProbeEnvironmentFrom([]string{"HOME=" + home, "PATH=/bin", "PATH=/attacker"}); err == nil {
		t.Fatal("duplicate allowed environment variable was accepted")
	}
}

func TestClaudeProbeUsesFullDescendantContainmentWithFiveSecondGrace(t *testing.T) {
	runner := &claudeGraceRecordingRunner{}
	command := legacyports.Command{Argv: []string{"claude", "plugin", "list", "--json"}}
	result, err := runClaudeListCommand(context.Background(), runner, command)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("run Claude list = %+v, err=%v", result, err)
	}
	if runner.runCalls != 0 || runner.graceCalls != 1 {
		t.Fatalf("runner calls: ordinary=%d descendant-grace=%d", runner.runCalls, runner.graceCalls)
	}
	if runner.grace != 5*time.Second || !reflect.DeepEqual(runner.command, command) {
		t.Fatalf("descendant grace=%s command=%+v", runner.grace, runner.command)
	}
}

func TestClaudeActivateAndRemoveBoundBlockingRunnerToSharedTimeout(t *testing.T) {
	request := newClaudeTimeoutActivationRequest(t)
	operations := map[string]func(context.Context, *claudeBlockingRunner) error{
		"activate": func(ctx context.Context, runner *claudeBlockingRunner) error {
			_, err := (Activator{Runner: runner}).Activate(ctx, request)
			return err
		},
		"remove": func(ctx context.Context, runner *claudeBlockingRunner) error {
			_, err := (Activator{Runner: runner}).Deactivate(ctx, domain.DeactivationRequest{
				Client: request.Client, DeclaredName: request.DeclaredName, CurrentActivation: domain.ActivationActive,
				Confirmed: true, BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath,
			})
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			runner := &claudeBlockingRunner{observed: make(chan time.Time, 1), release: release}
			done := make(chan error, 1)
			started := time.Now()
			go func() { done <- operation(context.Background(), runner) }()
			deadline := <-runner.observed
			remaining := time.Until(deadline)
			if remaining < 14*time.Second || remaining > claudeProbeTimeout {
				t.Fatalf("shared Claude deadline remaining=%s, want approximately %s", remaining, claudeProbeTimeout)
			}
			close(release)
			if err := <-done; !errors.Is(err, errClaudeBlockingRunnerReleased) {
				t.Fatalf("operation error=%v, want released sentinel", err)
			}
			if time.Since(started) > time.Second {
				t.Fatal("released blocking runner did not return promptly")
			}
		})
	}
}

func TestClaudeActivateAndRemoveRespectEarlierParentDeadline(t *testing.T) {
	request := newClaudeTimeoutActivationRequest(t)
	operations := map[string]func(context.Context, *claudeBlockingRunner) error{
		"activate": func(ctx context.Context, runner *claudeBlockingRunner) error {
			_, err := (Activator{Runner: runner}).Activate(ctx, request)
			return err
		},
		"remove": func(ctx context.Context, runner *claudeBlockingRunner) error {
			_, err := (Activator{Runner: runner}).Deactivate(ctx, domain.DeactivationRequest{
				Client: request.Client, DeclaredName: request.DeclaredName, CurrentActivation: domain.ActivationActive,
				Confirmed: true, BackendExecutable: request.BackendExecutable, ManagedArtifactPath: request.Delivery.ActivePath,
			})
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			runner := &claudeBlockingRunner{observed: make(chan time.Time, 1)}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			parentDeadline, _ := ctx.Deadline()
			started := time.Now()
			err := operation(ctx, runner)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("operation error=%v, want parent deadline", err)
			}
			if observed := <-runner.observed; !observed.Equal(parentDeadline) {
				t.Fatalf("runner deadline=%s, want earlier parent deadline=%s", observed, parentDeadline)
			}
			if time.Since(started) > time.Second {
				t.Fatal("earlier parent deadline was extended")
			}
		})
	}
}

func newClaudeTimeoutActivationRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	request := activationRequest(t, domain.ClientClaude)
	request.BackendExecutable = "/test/bin/claude"
	request.Client.ConfigRoot = filepath.Join(t.TempDir(), "claude-config")
	request.Delivery.ActivePath = filepath.Join(request.Client.ConfigRoot, "skills", "demo-managed")
	request.Delivery.OwnedBase = filepath.Dir(request.Delivery.ActivePath)
	request.Plan.TargetAnchor = request.Client.ConfigRoot
	request.Plan.TargetRoot = request.Delivery.OwnedBase
	request.Plan.ActivePath = request.Delivery.ActivePath
	if err := os.MkdirAll(request.Delivery.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	return request
}

func TestClaudeProbeRealProcessReceivesOnlyReviewedEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell probe is Unix-only")
	}
	home := t.TempDir()
	config := filepath.Join(home, "claude")
	active := filepath.Join(config, "skills", "demo")
	if err := os.MkdirAll(filepath.Dir(active), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("NODE_OPTIONS", "--require=/attacker.js")
	t.Setenv("ANTHROPIC_API_KEY", "secret")
	script := filepath.Join(t.TempDir(), "claude")
	body := `#!/bin/sh
test "$(pwd -P)" = "$(cd "$HOME" && pwd -P)" || exit 21
test "$DISABLE_AUTOUPDATER" = 1 || exit 22
test "$CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC" = 1 || exit 23
test "$CLAUDE_CONFIG_DIR" != /attacker || exit 24
test -z "$NODE_OPTIONS" || exit 25
test -z "$ANTHROPIC_API_KEY" || exit 26
printf '[]'
`
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	command, err := claudeListCommand(script, config, active)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (realClaudeProbeRunner{}).Run(context.Background(), command)
	if err != nil || result.ExitCode != 0 || string(result.Stdout) != "[]" {
		t.Fatalf("real probe result=%+v err=%v", result, err)
	}
}

type realClaudeProbeRunner struct{}

type claudeGraceRecordingRunner struct {
	runCalls   int
	graceCalls int
	grace      time.Duration
	command    legacyports.Command
}

var errClaudeBlockingRunnerReleased = errors.New("Claude blocking runner released by test")

type claudeBlockingRunner struct {
	observed chan time.Time
	release  <-chan struct{}
}

func (runner *claudeBlockingRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("ordinary runner path used")
}

func (runner *claudeBlockingRunner) RunWithDescendantExitGrace(ctx context.Context, _ legacyports.Command, _ time.Duration) (legacyports.CommandResult, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return legacyports.CommandResult{}, errors.New("Claude probe context has no deadline")
	}
	runner.observed <- deadline
	if runner.release != nil {
		select {
		case <-runner.release:
			return legacyports.CommandResult{}, errClaudeBlockingRunnerReleased
		case <-ctx.Done():
			return legacyports.CommandResult{}, ctx.Err()
		}
	}
	<-ctx.Done()
	return legacyports.CommandResult{}, ctx.Err()
}

func (runner *claudeGraceRecordingRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	runner.runCalls++
	return legacyports.CommandResult{}, nil
}

func (runner *claudeGraceRecordingRunner) RunWithDescendantExitGrace(_ context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	runner.graceCalls++
	runner.grace = grace
	runner.command = command
	return legacyports.CommandResult{}, nil
}

func (realClaudeProbeRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	process := exec.CommandContext(ctx, command.Argv[0], command.Argv[1:]...)
	process.Env, process.Dir = append([]string(nil), command.Env...), command.Dir
	stdout, err := process.Output()
	result := legacyports.CommandResult{Stdout: stdout}
	if exit, ok := err.(*exec.ExitError); ok {
		result.ExitCode, result.Stderr = exit.ExitCode(), append([]byte(nil), exit.Stderr...)
		return result, nil
	}
	return result, err
}

func assertBoundedClaudeProbe(t *testing.T, command legacyports.Command, configRoot string) {
	t.Helper()
	for name, want := range map[string]string{
		"CLAUDE_CONFIG_DIR":                        filepath.Clean(configRoot),
		"DISABLE_AUTOUPDATER":                      "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
	} {
		if got, ok := environmentValue(command.Env, name); !ok || got != want {
			t.Fatalf("%s=%q present=%t in %v", name, got, ok, command.Env)
		}
	}
	for _, forbidden := range []string{"NODE_OPTIONS", "NODE_PATH", "ANTHROPIC_API_KEY"} {
		if _, ok := environmentValue(command.Env, forbidden); ok {
			t.Fatalf("%s survived in %v", forbidden, command.Env)
		}
	}
	if command.Dir == "" || !filepath.IsAbs(command.Dir) || strings.TrimSpace(command.Dir) == "" {
		t.Fatalf("probe cwd = %q", command.Dir)
	}
}

func environmentValue(environment []string, name string) (string, bool) {
	prefix := name + "="
	for _, value := range environment {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix), true
		}
	}
	return "", false
}

func TestClaudeDryRunRemovalExecutesNoCommand(t *testing.T) {
	t.Parallel()
	runner := &recordingRunner{}
	outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), domain.DeactivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientClaude}, DeclaredName: "demo",
		CurrentActivation: domain.ActivationActive, Confirmed: false, BackendExecutable: "/test/bin/claude",
		ManagedArtifactPath: filepath.Join(t.TempDir(), "skills", "demo"),
	})
	if err != nil || !outcome.ArtifactRemovalAllowed || len(runner.commands) != 0 {
		t.Fatalf("outcome=%+v commands=%+v err=%v", outcome, runner.commands, err)
	}
}

func TestClaudeExactListVerificationRejectsCollisionAndDrivesInstallFailure(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientClaude)
	request.BackendExecutable = "/test/bin/claude"
	request.Client.ConfigRoot = filepath.Join(t.TempDir(), "claude-config")
	request.Delivery.ActivePath = filepath.Join(request.Client.ConfigRoot, "skills", "demo-managed")
	request.Delivery.OwnedBase = filepath.Dir(request.Delivery.ActivePath)
	request.Plan.TargetAnchor = request.Client.ConfigRoot
	request.Plan.TargetRoot = request.Delivery.OwnedBase
	request.Plan.ActivePath = request.Delivery.ActivePath
	if err := os.MkdirAll(request.Delivery.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(t.TempDir(), "foreign", "demo")
	runner := &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
		return legacyports.CommandResult{Stdout: []byte(claudeListing("demo", foreign, true))}
	}}
	outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || outcome.Activation != domain.ActivationFailed || outcome.Verification != domain.VerificationFailed || !outcome.AuthoritativeObservation {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
}

func TestClaudeProbeRejectsConfigAndManagedPathMismatchBeforeSpawn(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientClaude)
	request.BackendExecutable = "/test/bin/claude"
	request.Client.ConfigRoot = filepath.Join(t.TempDir(), "claude-config")
	request.Delivery.ActivePath = filepath.Join(t.TempDir(), "foreign", "demo")
	request.Delivery.OwnedBase = filepath.Dir(request.Delivery.ActivePath)
	request.Plan.TargetAnchor = request.Client.ConfigRoot
	request.Plan.TargetRoot = filepath.Join(request.Client.ConfigRoot, "skills")
	request.Plan.ActivePath = request.Delivery.ActivePath
	if err := os.MkdirAll(request.Delivery.ActivePath, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	_, err := (Activator{Runner: runner}).Activate(context.Background(), request)
	if err == nil || len(runner.commands) != 0 {
		t.Fatalf("err=%v commands=%v", err, runner.commands)
	}
}

func TestClaudeActivationPreflightRejectsUnsafeAnchorWithoutRunningClient(t *testing.T) {
	t.Parallel()
	request := activationRequest(t, domain.ClientClaude)
	request.BackendExecutable = "/test/bin/claude"
	request.Client.ConfigRoot = filepath.Join(t.TempDir(), "claude-config")
	request.Plan.TargetAnchor = filepath.Join(t.TempDir(), "foreign-config")
	request.Plan.TargetRoot = filepath.Join(request.Client.ConfigRoot, "skills")
	request.Plan.ActivePath = filepath.Join(request.Plan.TargetRoot, "demo-managed")
	runner := &recordingRunner{}

	err := (Activator{Runner: runner}).PreflightActivation(request)
	if err == nil || !strings.Contains(err.Error(), "delivery anchor") || len(runner.commands) != 0 {
		t.Fatalf("preflight err=%v commands=%v", err, runner.commands)
	}
}

func TestClaudeActivationProbeNormalizesPathsForPreflightAndActivation(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	config := filepath.Join(base, "nested", "..", "claude-config")
	request := activationRequest(t, domain.ClientClaude)
	request.BackendExecutable = "/test/bin/claude"
	request.Client.ConfigRoot = config
	request.Plan.TargetAnchor = config
	request.Plan.TargetRoot = filepath.Join(config, "skills")
	request.Plan.ActivePath = filepath.Join(config, "skills", "demo-managed")
	request.Delivery.ActivePath = request.Plan.ActivePath

	probe, err := prepareClaudeActivationProbe(request)
	if err != nil {
		t.Fatal(err)
	}
	if probe.configRoot != filepath.Clean(config) || probe.activePath != filepath.Join(filepath.Clean(config), "skills", "demo-managed") {
		t.Fatalf("normalized probe = %+v", probe)
	}
	if got, ok := environmentValue(probe.command.Env, "CLAUDE_CONFIG_DIR"); !ok || got != probe.configRoot {
		t.Fatalf("probe environment config root = %q, %t", got, ok)
	}
}

func TestClaudePluginStatusRequiresExactOfficialIdentity(t *testing.T) {
	t.Parallel()
	managed := filepath.Join(t.TempDir(), "claude-config", "skills", "managed")
	cases := map[string]struct {
		body string
		want claudeStatus
	}{
		"installed":  {claudeListing("demo", managed, true), claudeStatusInstalled},
		"empty":      {`[]`, claudeStatusAbsent},
		"disabled":   {claudeListing("demo", managed, false), claudeStatusAbsent},
		"wrong path": {claudeListing("demo", filepath.Join(t.TempDir(), "foreign"), true), claudeStatusCollision},
		"wrong id":   {claudeListing("other", managed, true), claudeStatusAbsent},
		"malformed":  {`[{"id":"demo@skills-dir"}]`, claudeStatusUnknown},
		"object":     {`{"plugins":[]}`, claudeStatusUnknown},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			if got := claudePluginStatus([]byte(test.body), "demo", managed); got != test.want {
				t.Fatalf("status=%d want=%d body=%s", got, test.want, test.body)
			}
		})
	}
}

func TestClaudeNativeIdentityRejectsUnmanagedSkillsDirCollision(t *testing.T) {
	t.Parallel()
	config := filepath.Join(t.TempDir(), "claude-config")
	root := filepath.Join(config, "skills")
	foreign := filepath.Join(root, "foreign-demo")
	writeIdentityFile(t, filepath.Join(foreign, ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
	plan := identityPlan(root)
	plan.ActivePath = filepath.Join(root, "managed-demo")
	plan.NativeRegistryExecutable = "/test/bin/claude"
	runner := &identityRunner{result: legacyports.CommandResult{Stdout: []byte(claudeListing("demo", foreign, true))}}
	observation, err := (NativeIdentityObserver{Runner: runner}).ObserveNativeIdentity(context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, nil)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged || !observation.NativeDiscoveryAttempted {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}
	assertBoundedClaudeProbe(t, runner.last, config)
}

func TestClaudeNativeIdentityRejectsAnyStagingIdentityReportedByClient(t *testing.T) {
	config := filepath.Join(t.TempDir(), "claude-config")
	root := filepath.Join(config, "skills")
	active := filepath.Join(root, "demo-managed")
	staging := filepath.Join(root, ".agentplugins-staging-0123456789abcdef")
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientClaude, DeclaredName: "demo", TargetRoot: root,
		ActivePath: active, NativeRegistryExecutable: "/test/bin/claude",
	}
	runner := &identityRunner{result: legacyports.CommandResult{Stdout: []byte(claudeListing("demo", staging, true))}}
	observation, err := (NativeIdentityObserver{Runner: runner}).ObserveNativeIdentity(
		context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, nil,
	)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("staging observation = %+v, %v", observation, err)
	}

	foreign := filepath.Join(root, ".agentplugins-staging-foreign")
	runner.result.Stdout = []byte(claudeListing("demo", foreign, true))
	observation, err = (NativeIdentityObserver{Runner: runner}).ObserveNativeIdentity(
		context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude}, plan, nil,
	)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("foreign staging observation = %+v, %v", observation, err)
	}
}

func TestClaudePreparedIdentityDoesNotIgnoreWatchedStagingDirectory(t *testing.T) {
	t.Parallel()
	config := filepath.Join(t.TempDir(), "claude-config")
	root := filepath.Join(config, "skills")
	staging := filepath.Join(root, ".agentplugins-staging-0123456789abcdef")
	writeIdentityFile(t, filepath.Join(staging, ".claude-plugin", "plugin.json"), `{"name":"demo"}`)
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientClaude, DeclaredName: "demo", TargetAnchor: config, TargetRoot: root,
		ActivePath: filepath.Join(root, "managed-demo"),
	}
	observation, err := (NativeIdentityObserver{}).ObservePreparedIdentity(
		context.Background(), domain.DetectedClient{ClientID: domain.ClientClaude, ConfigRoot: config}, plan, nil,
	)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}
}

func claudeListing(name, installPath string, enabled bool) string {
	return fmt.Sprintf(`[{"id":%q,"version":"1.0.0","scope":"user","enabled":%t,"installPath":%q,"mcpServers":{}}]`, name+"@skills-dir", enabled, installPath)
}
