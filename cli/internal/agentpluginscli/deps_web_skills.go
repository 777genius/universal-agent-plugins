package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type webSkillAcquirer interface {
	ResolveHTTPSkill(context.Context, string, string) (domain.PackageSnapshot, domain.WebSkillSourceFacts, error)
	AcquireHTTPSkillVerified(context.Context, domain.WebSkillSourceFacts) (domain.PackageSnapshot, error)
}

func (app App) webSkills() (webSkillAcquirer, error) {
	a, ok := app.SourceAcquirer.(webSkillAcquirer)
	if !ok {
		return nil, errors.New("HTTPS Skill acquisition unavailable")
	}
	return a, nil
}
func (resolver dependencyResolver) resolveWebSkill(ctx context.Context, req environment.Requirement, targets []string) (environment.LockedPackage, error) {
	a, err := resolver.app.webSkills()
	if err != nil {
		return environment.LockedPackage{}, err
	}
	raw, facts, err := a.ResolveHTTPSkill(ctx, req.Web.URL, req.Web.Skill)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	snapshot, err := skillsource.Normalize(ctx, raw, req.Web.Skill, mustSourceKey(req))
	if err != nil {
		return environment.LockedPackage{}, err
	}
	facts.RawTreeDigest, facts.Normalizer = raw.TreeDigest, environment.SkillNormalizerV1
	pin := environment.LockedPackage{Alias: req.Alias, SourceKey: mustSourceKey(req), Targets: append([]string{}, targets...), DigestAlgorithm: snapshot.DigestAlgorithm, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: environment.LockedSource{Web: &facts}}
	snapshot.Source = webSkillSnapshotSource(pin)
	loaded, err := resolver.app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	pin.ManifestDigest = loaded.envelope.ManifestDigest
	return pin, resolver.finishLoaded(ctx, loaded, pin)
}
func webSkillSnapshotSource(pin environment.LockedPackage) domain.SourceIdentity {
	facts := *pin.Source.Web
	facts.Files = append([]domain.WebSkillFile{}, facts.Files...)
	return domain.SourceIdentity{SourceKind: "https-skill", SourceURL: facts.URL, WebSkill: &facts, StableSourceKey: pin.SourceKey, RequestedSource: pin.SourceKey, CanonicalSource: pin.SourceKey, ResolvedRevision: facts.RawTreeDigest, SourceBindingHint: "uap-dependency-web-skill"}
}
func (app App) acquireLockedWebSkill(ctx context.Context, pin environment.LockedPackage) (domain.PackageSnapshot, error) {
	if pin.Source.Web.Normalizer != environment.SkillNormalizerV1 {
		return domain.PackageSnapshot{}, errors.New("unsupported locked HTTPS Skill normalizer")
	}
	a, err := app.webSkills()
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	raw, err := a.AcquireHTTPSkillVerified(ctx, *pin.Source.Web)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	snapshot, err := skillsource.Normalize(ctx, raw, pin.Source.Web.Skill, pin.SourceKey)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if snapshot.TreeDigest != pin.TreeDigest {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("normalized HTTPS Skill artifact differs from uap.lock")
	}
	snapshot.Source = webSkillSnapshotSource(pin)
	return snapshot, nil
}
