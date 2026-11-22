package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Preserve the production tree-aware supervisor and its original result/error.
// This records the real completion query, rather than replacing its catalog.
type hermesSourceRecorder struct {
	processadapter.OS
	input *hermesSourceSkills
}

func (runner hermesSourceRecorder) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	result, err := runner.OS.RunWithTreeExitGrace(ctx, command, grace)
	input := runner.input
	label := fmt.Sprintf("production-%03d", len(input.commands))
	for suffix, body := range map[string][]byte{"stdout": result.Stdout, "stderr": result.Stderr} {
		if writeErr := os.WriteFile(filepath.Join(input.root, label+"."+suffix), body, 0600); writeErr != nil {
			input.recordingError = writeErr
		}
	}
	query := len(command.Argv) == 8 && strings.Contains(command.Argv[5], "skills_tool._skill_catalog()")
	if query && err == nil && result.ExitCode == 0 && result.CleanupComplete && !result.NoChildStarted {
		input.productionQueries++
	}
	stdout, stderr := sha256.Sum256(result.Stdout), sha256.Sum256(result.Stderr)
	input.commands = append(input.commands, map[string]any{"label": label, "argv": command.Argv, "cwd": command.Dir,
		"exit_code": result.ExitCode, "cleanup_complete": result.CleanupComplete, "no_child_started": result.NoChildStarted,
		"error": fmt.Sprint(err), "production_catalog_query": query,
		"stdout_sha256": hex.EncodeToString(stdout[:]), "stderr_sha256": hex.EncodeToString(stderr[:])})
	return result, err
}

// Use the existing composition seam and the same namespace/paths as compose.go;
// only the real process runner gains recording, with no alternate state engine.
func (input *hermesSourceSkills) recordProduction(t *testing.T, app agentpluginscli.App) agentpluginscli.App {
	t.Helper()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	stateRoot := filepath.Dir(app.ManagedRoot)
	app.Installer, err = installer.New(installer.Config{UserHome: app.UserHome, ProjectRoot: app.ProjectRoot,
		StateRoot: stateRoot, StateFile: filepath.Join(stateRoot, "state-v2.json"), LockFile: filepath.Join(stateRoot, "mutation.lock"),
		OperationsDir: filepath.Join(stateRoot, "operations-v2"), PluginDataBase: filepath.Join(stateRoot, "plugin-data"),
		ManagedRoot: app.ManagedRoot, TempRoot: filepath.Join(stateRoot, "installer-tmp"), HelperExecutable: helper, HelperVersion: version,
		Registry: app.ClientRegistry, Runner: hermesSourceRecorder{input: input}, EnableNativeObserver: true, LookupExecutable: exec.LookPath})
	if err != nil {
		t.Fatal(err)
	}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return input.recordProduction(t, selected), nil
	}
	return app
}

func (input *hermesSourceSkills) requireCompletion(t *testing.T, binding domain.ClientBinding, before int) {
	t.Helper()
	if binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled ||
		input.productionQueries != before+1 || input.recordingError != nil {
		t.Fatalf("native Ready lacks one settled production catalog: %+v queries=%d->%d recording=%v",
			binding, before, input.productionQueries, input.recordingError)
	}
}

func (input *hermesSourceSkills) userRepeat(t *testing.T) {
	t.Helper()
	source := newProjectSkillsSource(t, "https")
	source.transport.ctx, source.transport.tempRoot = input.ctx, input.root
	journey := newProjectJourney(t, source.transport)
	for key, value := range input.environment {
		t.Setenv(key, value)
	}
	t.Setenv("PYTHONDONTWRITEBYTECODE", "1")
	journey.app = input.recordProduction(t, configureHermesColdJourney(input.ctx, t, journey.app, input.profile, input.launcher))
	foreign, config := securityFixtureTree(t, filepath.Join(input.profile, "skills")), input.configuration(t)
	trust := input.trusted(t, "user-before", journey.a)
	args := []string{"skills", "add", source.input, "--skill", "docs", "--target", "hermes", "--no-ui", "--yes"}
	before := input.productionQueries
	input.cliAt(t, journey, "user-preview", "", append(slices.Clone(args), "--dry-run")...)
	if input.productionQueries != before {
		t.Fatal("User preview started native completion")
	}
	input.cliAt(t, journey, "user-fresh", "", args...)
	installation, binding := journey.owned(t, "")
	input.requireCompletion(t, binding, before)
	intent := journey.assertPortablePair(t, journey.cache)
	before = input.productionQueries
	input.cliAt(t, journey, "user-repeat", "", args...)
	after, selected := journey.owned(t, "")
	input.requireCompletion(t, selected, before)
	if installation.InstallationID != after.InstallationID || binding.ClientBindingID != selected.ClientBindingID ||
		installation.Source != after.Source || intent != journey.assertPortablePair(t, journey.cache) {
		t.Fatal("User repeat changed installation, binding, source pin or intent")
	}
	input.cliAt(t, journey, "user-remove", "", "remove", "docs", "--target", "hermes", "--yes")
	projectJourneyAbsent(t, filepath.Join(input.profile, "skills", "docs"))
	if !reflect.DeepEqual(foreign, securityFixtureTree(t, filepath.Join(input.profile, "skills"))) ||
		!reflect.DeepEqual(config, input.configuration(t)) || !reflect.DeepEqual(trust, input.trusted(t, "user-after", journey.a)) {
		t.Fatal("User completion changed foreign Skills, configuration or project trust")
	}
	input.userCompleted = true
}
