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
					input:  AddInput{Scope: domain.ScopeUser, Client: domain.DetectedClient{ClientID: client}},
					client: domain.ClientBinding{PackageRevision: &source.rev},
					verifyErr: &ports.VerificationError{
						Kind:         ports.VerificationDigestMismatch,
						ActualDigest: "sha256:foreign-user-modification",
						Err:          errors.New("artifact digest mismatch"),
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

// TestRawSkillRepairGuardPreservesOtherScopePolicies keeps project Gemini
// and other native package repair decisions outside the user Codex/Claude
// safety change. An unconfirmed operation must still reach confirmation.
func TestRawSkillRepairGuardPreservesOtherScopePolicies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		client domain.ClientID
		scope  domain.InstallScope
		rev    domain.ClientPackageRevision
	}{
		{name: "gemini-project-skill", client: domain.ClientGemini, scope: domain.ScopeProject, rev: domain.ClientPackageRevision{Skill: &domain.SkillSourceFacts{Name: "docs"}}},
		{name: "codex-project-skill", client: domain.ClientCodex, scope: domain.ScopeProject, rev: domain.ClientPackageRevision{Skill: &domain.SkillSourceFacts{Name: "docs"}}},
		{name: "codex-user-plugin", client: domain.ClientCodex, scope: domain.ScopeUser, rev: domain.ClientPackageRevision{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof := repairSession{
				input:     AddInput{Scope: tc.scope, Client: domain.DetectedClient{ClientID: tc.client}},
				client:    domain.ClientBinding{PackageRevision: &tc.rev},
				verifyErr: &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: "sha256:edited", Err: errors.New("artifact digest mismatch")},
			}
			_, err := proof.repairPackage()
			if err != nil || !proof.result.RequiresConfirmation {
				t.Fatalf("%s repair was unexpectedly changed by user raw-Skill guard: %+v %v", tc.name, proof.result, err)
			}
		})
	}
}
