package installer

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestAcquiredLocalSkillRequiresExactSourceTuple(t *testing.T) {
	key := sourcespec.LocalSkillKey("/skills/review", "review")
	raw := "sha256:" + strings.Repeat("a", 64)
	source := domain.SourceIdentity{SourceKind: sourcespec.LocalSkillKind, SourceURL: "/skills/review", StableSourceKey: key, RequestedSource: key, CanonicalSource: key, ResolvedRevision: raw, SourceBindingHint: "uap-dependency-local-skill", Skill: &domain.SkillSourceFacts{Name: "review", RawTreeDigest: raw, Normalizer: "agent-skill-package/v1"}}
	valid := domain.AcquiredSnapshot{OriginMode: domain.OriginModeDirect, Source: source}
	if got, err := validateAcquiredSource(&valid, true); err != nil || got != key {
		t.Fatalf("valid local Skill refused: %q %v", got, err)
	}
	for _, mutate := range []func(*domain.SourceIdentity){
		func(s *domain.SourceIdentity) { s.SourceURL = "/other/review" },
		func(s *domain.SourceIdentity) { s.SourceKind = "git" },
		func(s *domain.SourceIdentity) { s.SourceBindingHint = "uap-dependency-skill" },
		func(s *domain.SourceIdentity) { s.ResolvedRevision = "sha256:" + strings.Repeat("b", 64) },
		func(s *domain.SourceIdentity) { s.Repository = "acme/tools" },
		func(s *domain.SourceIdentity) { s.Skill.RequestedPath = "review" },
		func(s *domain.SourceIdentity) { s.Skill.Normalizer = "future" },
		func(s *domain.SourceIdentity) { s.StableSourceKey = "local-skill-v1:other" },
	} {
		bad := valid
		facts := *source.Skill
		bad.Source.Skill = &facts
		mutate(&bad.Source)
		if _, err := validateAcquiredSource(&bad, true); err == nil {
			t.Fatalf("accepted forged local provenance: %+v", bad.Source)
		}
	}
	if _, err := validateAcquiredSource(&valid, false); err == nil {
		t.Fatal("accepted local Skill as native package")
	}
}
