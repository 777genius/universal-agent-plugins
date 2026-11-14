package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectCopilotDetector struct{ user string }

func (d projectCopilotDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientCopilot, ConfigRoot: d.user,
		Version: "1.0.103", Status: domain.DetectionDetected}}, nil
}

func configureProjectCopilotJourney(app agentpluginscli.App, user string) agentpluginscli.App {
	app.Detector = projectCopilotDetector{user: user}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectCopilotJourney(selected, user), nil
	}
	return app
}

func projectCopilotOwned(t *testing.T, journey projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	if binding.ClientID != "copilot" || binding.Scope != "project" || binding.NativeProfileRoot != project || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("Copilot binding lost project authority or conservative native status: %+v", binding)
	}
	for _, path := range []string{filepath.Join(project, ".github", "skills", "docs", "SKILL.md"), filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")} {
		if projectJourneyRead(t, path) != expected {
			t.Fatalf("Copilot managed/native skill differs at %s", path)
		}
	}
	return installation, binding
}
