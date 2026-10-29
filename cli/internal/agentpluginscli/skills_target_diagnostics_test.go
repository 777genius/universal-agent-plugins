package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSkillsMissingTargetDiagnostic(t *testing.T) {
	for _, state := range []string{"absent", "not_detected", "inspection_failed"} {
		t.Run(state, func(t *testing.T) {
			j, _, sha := skillJourney(t)
			if !supportsDependencyTarget(j.cli.app, "copilot") {
				t.Fatal("Copilot should remain a qualified target")
			}
			client := fixtureClient(t, domain.ClientCopilot)
			detected := []domain.DetectedClient{}
			switch state {
			case "not_detected":
				client.Status = domain.DetectionNotDetected
				detected = append(detected, client)
			case "inspection_failed":
				client.Status = domain.DetectionDetected
				client.DetectionError = errors.New("private-provider-secret")
				detected = append(detected, client)
			}
			j.cli.app.Detector = staticDetector{clients: detected}
			plan, err := j.cli.app.PrepareSkills(context.Background(), SkillsBatchRequest{
				Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf", Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}},
				TargetIDs:  []string{"copilot"},
			})
			if plan != nil {
				plan.Close()
				t.Fatal("unavailable target prepared a plan")
			}
			if !errors.Is(err, installer.ErrTargetFactsUnavailable) {
				t.Fatalf("missing typed target error: %v", err)
			}
			var refusal *skillspicker.ReviewRefusal
			if !errors.As((&skillsCatalogSession{}).reviewRefusal(err), &refusal) || refusal.Code != "TARGET_UNAVAILABLE" {
				t.Fatalf("wrong public diagnostic: %+v (%v)", refusal, err)
			}
			if !strings.Contains(refusal.Message, "configuration") || !strings.Contains(refusal.Message, "Choose another target") || strings.Contains(refusal.Message, "private-provider-secret") {
				t.Fatalf("unsafe or unactionable diagnostic: %+v", refusal)
			}
			mp, lp := j.cli.app.dependencyPaths()
			for _, path := range []string{mp, lp, j.cli.app.EnvironmentStoreRoot, client.ConfigRoot} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("refusal wrote persistent path %s: %v", path, err)
				}
			}
		})
	}
}
