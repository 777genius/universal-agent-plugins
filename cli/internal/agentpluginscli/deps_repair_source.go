package agentpluginscli

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Each binding's recorded revision owns repair provenance, independently of a
// newer environment lock or another target's successful update.
func setRepairSkillSource(pkg *environment.LockedPackage, installation domain.Installation, rev *domain.ClientPackageRevision) error {
	pkg.Source.Git = &environment.LockedGitSource{Repository: installation.Source.Repository, URL: installation.Source.SourceURL, Path: installation.Source.PackageSubpath, Commit: rev.ResolvedRevision, Requested: environment.Selector{Kind: environment.SelectorCommit, Value: rev.ResolvedRevision}}
	if installation.Source.StableSourceKey != "" && installation.Source.SourceKind != "https-skill" {
		facts := rev.Skill
		if facts == nil {
			return errors.New("recorded Skill source provenance unavailable; restore the matching state/lock before repair")
		}
		pkg.Source.Git = &environment.LockedGitSource{Kind: environment.SkillKind, Skill: facts.Name, Repository: installation.Source.Repository, Path: facts.RequestedPath, Commit: rev.ResolvedRevision, Requested: environment.Selector{Kind: environment.SelectorCommit, Value: rev.ResolvedRevision}, ResolvedSkillPath: facts.ResolvedPath, RawTreeDigest: facts.RawTreeDigest, Normalizer: facts.Normalizer}
	}
	if installation.Source.SourceKind == "https-skill" {
		if rev.WebSkill == nil {
			return errors.New("recorded HTTPS Skill provenance unavailable; restore matching state/lock before repair")
		}
		pkg.Source.Git = nil
		facts := *rev.WebSkill
		facts.Files = append([]domain.WebSkillFile{}, rev.WebSkill.Files...)
		pkg.Source.Web = &facts
	}
	return nil
}
