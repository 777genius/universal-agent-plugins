package installer

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func acquiredSkillFixture() domain.AcquiredSnapshot {
	identity := "github:acme/tools@" + strings.Repeat("1", 40) + "//moved/pdf"
	return domain.AcquiredSnapshot{OriginMode: domain.OriginModeDirect, Source: domain.SourceIdentity{
		RequestedSource: identity, CanonicalSource: identity, Repository: "acme/tools", PackageSubpath: "moved/pdf", ResolvedRevision: strings.Repeat("1", 40),
		StableSourceKey: "github-skill:acme/tools:pdf:skills/pdf", SourceBindingHint: "uap-dependency-skill",
		Skill: &domain.SkillSourceFacts{Name: "pdf", RequestedPath: "skills/pdf", ResolvedPath: "moved/pdf", RawTreeDigest: "sha256:" + strings.Repeat("a", 64), Normalizer: "agent-skill-package/v1"}}}
}

func TestAcquiredStableKeyRequiresExactNormalizedSkillAuthority(t *testing.T) {
	valid := acquiredSkillFixture()
	if key, err := validateAcquiredSource(&valid, true); err != nil || key != valid.Source.StableSourceKey {
		t.Fatalf("accepted relocated Skill authority refused: %q %v", key, err)
	}
	for _, mutate := range []func(*domain.SourceIdentity){
		func(s *domain.SourceIdentity) { s.StableSourceKey = "github:acme/tools:moved/pdf" },
		func(s *domain.SourceIdentity) { s.StableSourceKey = "github-skill:acme/tools:other:skills/pdf" },
		func(s *domain.SourceIdentity) { s.SourceBindingHint = "uap-dependency-git" },
		func(s *domain.SourceIdentity) { s.Skill = nil },
		func(s *domain.SourceIdentity) { s.Skill.Normalizer = "future" },
		func(s *domain.SourceIdentity) { s.Skill.RawTreeDigest = "sha256:invalid" },
		func(s *domain.SourceIdentity) { s.Skill.RequestedPath = "../foreign" },
		func(s *domain.SourceIdentity) { s.Skill.ResolvedPath = "moved/other" },
		func(s *domain.SourceIdentity) { s.Skill.Name = "pdf:foreign" },
		func(s *domain.SourceIdentity) { s.SourceKind, s.SourceURL = "git", "https://git.example/tools.git" },
	} {
		bad := acquiredSkillFixture()
		mutate(&bad.Source)
		if _, err := validateAcquiredSource(&bad, true); err == nil {
			t.Fatalf("caller stable key overrode source identity: %+v", bad.Source)
		}
	}
	if _, err := validateAcquiredSource(&valid, false); err == nil {
		t.Fatal("stable Skill key accepted without saved environment authority")
	}
}
