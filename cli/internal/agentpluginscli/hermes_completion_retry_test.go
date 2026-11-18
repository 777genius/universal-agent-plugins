package agentpluginscli

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestHermesCompletionRetryRechecksTrustProfileAuthority(t *testing.T) {
	for _, changeProfile := range []bool{false, true} {
		name := "same profile"
		if changeProfile {
			name = "changed profile"
		}
		t.Run(name, func(t *testing.T) {
			_, journey, source, _ := managementMutableWebsite(t)
			runner := setHermesCompletionHost(t, &journey)
			initial, err := journey.cli.app.Detector.Detect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			detector := &observedProbingDetector{clients: initial}
			journey.cli.app.Detector = detector
			request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"hermes"}}
			seed, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := seed.Apply(t.Context()); err != nil {
				t.Fatal(err)
			}
			seed.Close()
			runner.grants, runner.trusted = 0, nil
			original, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer original.Close()
			// An installed Project receipt retains the project destination; it does
			// not authorize silently switching the User trust profile during retry.
			unavailable := original.request.detected[domain.ClientHermes]
			unavailable.Status = domain.DetectionNotDetected
			original.request.detected[domain.ClientHermes] = unavailable
			prompts := 0
			app := journey.cli.app
			app.Terminal = true
			app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				prompts++
				if prompts == 1 {
					if changeProfile {
						profile := filepath.Join(filepath.Dir(runner.profile), "other-profile")
						hermesCompletionWrite(t, filepath.Join(profile, "config.yaml"), "model: retained-other\n")
						t.Setenv("HERMES_HOME", profile)
						detector.clients[0].ConfigRoot, runner.profile = profile, profile
					}
					return prompt.ConfirmationResult{Accepted: true}, nil
				}
				return prompt.ConfirmationResult{Accepted: false}, nil
			}}
			cmd, _, stderr := retryTestCommand(t)
			app.ErrorOutput = stderr
			err = applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "project", plain: true}, original, true)
			if changeProfile {
				if err == nil || prompts != 2 || runner.grants != 0 {
					t.Fatalf("changed profile reused original trust review: prompts=%d grants=%d err=%v", prompts, runner.grants, err)
				}
			} else if err != nil || prompts != 1 || runner.grants != 1 {
				t.Fatalf("stable authority retry failed or required extra review: prompts=%d grants=%d err=%v", prompts, runner.grants, err)
			}
		})
	}
}
