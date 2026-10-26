package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectKimiDetector struct{ user, executable string }

func (d projectKimiDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientKimi, ConfigRoot: d.user,
		Version: "2.3.1", ExecutablePath: d.executable, Status: domain.DetectionDetected}}, nil
}

func configureProjectKimiJourney(app agentpluginscli.App, user, executable string) agentpluginscli.App {
	app.Detector = projectKimiDetector{user: user, executable: executable}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectKimiJourney(selected, user, executable), nil
	}
	return app
}

func projectKimiOwned(t *testing.T, journey projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	if binding.ClientID != "kimi" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(project, ".kimi-code") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("Kimi binding lost project authority or conservative native status: %+v", binding)
	}
	for _, path := range []string{filepath.Join(project, ".kimi-code", "skills", "docs", "SKILL.md"), filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")} {
		if projectJourneyRead(t, path) != expected {
			t.Fatalf("Kimi managed/native skill differs at %s", path)
		}
	}
	return installation, binding
}
