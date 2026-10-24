package usecase

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestWebSkillRevisionFreezesFilesForRepair(t *testing.T) {
	facts := &domain.WebSkillSourceFacts{URL: "https://skills.example", Skill: "pdf", RawTreeDigest: "old revision", Files: []domain.WebSkillFile{{Path: "SKILL.md", SHA256: "old hash"}}}
	envelope := domain.PackageEnvelope{Source: domain.SourceIdentity{WebSkill: facts}, TreeDigest: "normalized", ManifestDigest: "manifest"}
	revision := packageRevisionFromEnvelope(envelope)
	if !repairPackageRevisionMatches(revision, envelope, false) {
		t.Fatal("captured revision cannot be repaired")
	}
	facts.Files[0].SHA256 = "new hash"
	facts.RawTreeDigest = "new revision"
	if revision.WebSkill.Files[0].SHA256 != "old hash" || revision.WebSkill.RawTreeDigest != "old revision" {
		t.Fatal("caller mutation rewrote an applied client's repair facts")
	}
	if repairPackageRevisionMatches(revision, envelope, false) {
		t.Fatal("repair accepted different pinned file provenance")
	}
}
