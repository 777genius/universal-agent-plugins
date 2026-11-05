package usecase

import (
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Verify a raw Skill's pinned source type, not a client-specific trait,
// prevents destructive repair of an edited existing managed directory.
// Deliberately provide no staging service: any unintended mutation path must
// fail this test, rather than silently overwrite another user's files.
func TestRepairRawSkillDigestDriftRefusedBeforeStaging(t *testing.T) {
	for _, client := range []domain.ClientID{domain.ClientCodex, domain.ClientClaude} {
		for _, source := range []struct {
			name string
			rev  domain.ClientPackageRevision
		}{
			{name: "git", rev: domain.ClientPackageRevision{Skill: &domain.SkillSourceFacts{Name: "benchmark-note"}}},
			{name: "https", rev: domain.ClientPackageRevision{WebSkill: &domain.WebSkillSourceFacts{Skill: "benchmark-note"}}},
		} {
			t.Run(string(client)+"/"+source.name, func(t *testing.T) {
				proof := repairSession{
					input: AddInput{Client: domain.DetectedClient{ClientID: client}},
					client: domain.ClientBinding{PackageRevision: &source.rev},
					verifyErr: &ports.VerificationError{
						Kind: ports.VerificationDigestMismatch,
						ActualDigest: "sha256:foreign-user-modification",
						Err: errors.New("artifact digest mismatch"),
					},
				}
				_, err := proof.repairPackage()
				if err == nil ||
					!strings.Contains(err.Error(), "managed raw Skill contains modified or unowned content; refusing repair") ||
					!strings.Contains(err.Error(), "artifact digest mismatch") {
					t.Fatalf("raw %s on %s failed to refuse damaged managed contents: %v", source.name, client, err)
				}
			})
		}
	}
}
