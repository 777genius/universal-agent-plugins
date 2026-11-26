package installer

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func acquiredGitSkillFixture() domain.AcquiredSnapshot {
	acquired := acquiredSkillFixture()
	source := &acquired.Source
	source.Repository, source.SourceKind, source.SourceURL = "", "git", "ssh://git@git.example/team/tools.git"
	source.StableSourceKey = sourcespec.GitSkillKey(source.SourceURL, source.Skill.Name, source.Skill.RequestedPath)
	source.RequestedSource, source.CanonicalSource = source.StableSourceKey, source.StableSourceKey
	return acquired
}

func TestAcquiredGenericGitSkillRequiresExactTuple(t *testing.T) {
	valid := acquiredGitSkillFixture()
	key, err := validateAcquiredSource(&valid, true)
	if err != nil || key != valid.Source.StableSourceKey {
		t.Fatal(key, err)
	}
	for _, mutate := range []func(*domain.SourceIdentity){
		func(s *domain.SourceIdentity) { s.SourceURL = "ssh://git@other.example/team/tools.git" },
		func(s *domain.SourceIdentity) { s.Repository = "acme/tools" },
		func(s *domain.SourceIdentity) { s.ArchiveSHA256 = "unexpected" },
		func(s *domain.SourceIdentity) { s.SourceBindingHint = "direct-git" },
		func(s *domain.SourceIdentity) { s.Skill.RequestedPath = "other/pdf" },
		func(s *domain.SourceIdentity) { s.Skill.ResolvedPath = "../pdf" },
		func(s *domain.SourceIdentity) { s.ResolvedRevision = "main" },
		func(s *domain.SourceIdentity) { s.Skill.Normalizer = "future" },
		func(s *domain.SourceIdentity) { s.Skill.RawTreeDigest = "sha256:bad" },
		func(s *domain.SourceIdentity) { s.WebSkill = &domain.WebSkillSourceFacts{} },
		func(s *domain.SourceIdentity) {
			s.StableSourceKey = sourcespec.Key(sourcespec.KindGit, s.SourceURL, s.PackageSubpath)
		},
	} {
		bad := acquiredGitSkillFixture()
		mutate(&bad.Source)
		if _, err := validateAcquiredSource(&bad, true); err == nil {
			t.Fatalf("invalid generic Skill authority accepted: %+v", bad.Source)
		}
	}
	if _, err := validateAcquiredSource(&valid, false); err == nil {
		t.Fatal("generic Skill stable key accepted outside portable authority")
	}
	before := domain.ComputeSourceBindingID(valid.Source)
	valid.Source.PackageSubpath, valid.Source.ResolvedRevision = "relocated/pdf", "new-commit"
	if domain.ComputeSourceBindingID(valid.Source) != before {
		t.Fatal("revision or relocation changed source ownership")
	}
}
