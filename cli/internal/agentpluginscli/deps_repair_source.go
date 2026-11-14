package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Recorded per-binding revisions intentionally omit executable inventory. This
// repair-only route recovers it from exact verified bytes before the ordinary
// strict pin/materialization checks; lock replay never calls this helper.
func hydrateRecordedDependencyInventory(ctx context.Context, app App, pin environment.LockedPackage) (environment.LockedPackage, error) {
	if err := requireLockedDirectoryFeed(app, pin); err != nil {
		return environment.LockedPackage{}, err
	}
	snapshot, err := app.dependencyStore().Snapshot(ctx, pin.DigestAlgorithm, pin.TreeDigest)
	if errors.Is(err, environmentstore.ErrMissing) {
		snapshot, err = app.acquireLockedSnapshot(ctx, pin)
	}
	if err != nil {
		return environment.LockedPackage{}, err
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	envelope, err := app.loadStandardEnvelope(ctx, domain.LoadInput{SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: identityFromLocked(pin)})
	if err != nil {
		return environment.LockedPackage{}, err
	}
	pin.ExecutableFiles = canonicalDependencyExecutables(envelope.ExecutableFiles)
	if err := validateDependencyPinEnvelope(pin, envelope); err != nil {
		return environment.LockedPackage{}, err
	}
	pin.Version = envelope.Manifest.Version
	return pin, nil
}

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
