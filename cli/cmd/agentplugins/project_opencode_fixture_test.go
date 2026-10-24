package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type projectOpenCodeDetector struct{ root, executable string }

func (d projectOpenCodeDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientOpenCode, ConfigRoot: d.root,
		ExecutablePath: d.executable, Version: "1.18.34", Status: domain.DetectionDetected}}, nil
}

// The compiled inert TEST process implements only --version. It exercises the
// production host-preparation boundary without pretending to be native proof.
func projectOpenCodeProbeFixture(t *testing.T) string {
	t.Helper()
	name := "TEST-opencode-version-only"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(projectJourneyTemp(t), name)
	command := exec.CommandContext(t.Context(), "go", "build", "-ldflags", "-X main.version=1.18.34 -X main.mode=version-only", "-o", path,
		"../../../install/integrationctl/agentplugins/adapters/clientdetect/testdata/opencode_probe.go")
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("inert version-contract fixture build: %s, %v", body, err)
	}
	return path
}

func configureProjectOpenCodeJourney(app agentpluginscli.App, detector projectOpenCodeDetector) agentpluginscli.App {
	app.Detector = detector
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectOpenCodeJourney(selected, detector), nil
	}
	return app
}

func projectOpenCodeNativeSkill(project string) string {
	return filepath.Join(project, ".opencode", "skills", "docs", "SKILL.md")
}

func projectOpenCodeOwned(t *testing.T, journey projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	if binding.ClientID != string(domain.ClientOpenCode) || binding.Scope != string(domain.ScopeProject) || binding.NativeProfileRoot != project || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("project OpenCode binding lost scoped conservative authority: %+v", binding)
	}
	for _, path := range []string{projectOpenCodeNativeSkill(project), filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")} {
		if body := projectJourneyRead(t, path); body != expected {
			t.Fatalf("OpenCode owned skill at %s = %q", path, body)
		}
	}
	return installation, binding
}
