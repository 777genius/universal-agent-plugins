package agentpluginscli

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func nativeLocalSkillPath(serialized string) (string, error) {
	if err := sourcespec.ValidateLocalSkillPath(serialized); err != nil {
		return "", err
	}
	native := filepath.FromSlash(serialized)
	if !filepath.IsAbs(native) || runtime.GOOS != "windows" && (strings.HasPrefix(serialized, "//") || !strings.HasPrefix(serialized, "/")) {
		return "", errors.New("local Skill source belongs to another OS; restore its verified store content or update from the original source host")
	}
	return native, nil
}

func (resolver dependencyResolver) resolveLocalSkill(ctx context.Context, req environment.Requirement, targets []string) (environment.LockedPackage, error) {
	if err := resolver.app.checkEnterpriseRequirement(ctx, req, targets); err != nil {
		return environment.LockedPackage{}, err
	}
	raw, err := resolver.app.acquireLocalSkillRaw(ctx, req.Local.Path)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	snapshot, err := skillsource.Normalize(ctx, raw, req.Local.Skill, mustSourceKey(req))
	if err != nil {
		return environment.LockedPackage{}, err
	}
	pin := environment.LockedPackage{Alias: req.Alias, SourceKey: mustSourceKey(req), Targets: append([]string{}, targets...), DigestAlgorithm: snapshot.DigestAlgorithm, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: environment.LockedSource{Local: &environment.LockedLocalSource{Path: req.Local.Path, Skill: req.Local.Skill, RawTreeDigest: raw.TreeDigest, Normalizer: environment.SkillNormalizerV1}}}
	snapshot.Source = localSkillSnapshotSource(pin)
	loaded, err := resolver.app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	pin.ManifestDigest = loaded.envelope.ManifestDigest
	return pin, resolver.finishLoaded(ctx, loaded, pin)
}

func localSkillSnapshotSource(pin environment.LockedPackage) domain.SourceIdentity {
	source := pin.Source.Local
	return domain.SourceIdentity{SourceKind: sourcespec.LocalSkillKind, SourceURL: source.Path, Skill: &domain.SkillSourceFacts{Name: source.Skill, RawTreeDigest: source.RawTreeDigest, Normalizer: source.Normalizer}, StableSourceKey: pin.SourceKey, RequestedSource: pin.SourceKey, CanonicalSource: pin.SourceKey, ResolvedRevision: source.RawTreeDigest, SourceBindingHint: "uap-dependency-local-skill"}
}

func (app App) acquireLockedLocalSkill(ctx context.Context, pin environment.LockedPackage) (domain.PackageSnapshot, error) {
	source := pin.Source.Local
	if source.Normalizer != environment.SkillNormalizerV1 {
		return domain.PackageSnapshot{}, errors.New("unsupported locked local Skill normalizer")
	}
	raw, err := app.acquireLocalSkillRaw(ctx, source.Path)
	if err != nil {
		if ctx.Err() != nil {
			return domain.PackageSnapshot{}, ctx.Err()
		}
		return domain.PackageSnapshot{}, errors.New("locked local Skill source is unavailable; restore its verified store content or recover the original source")
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	if raw.TreeDigest != source.RawTreeDigest {
		return domain.PackageSnapshot{}, errors.New("local Skill source changed since uap.lock; use update to accept a new snapshot")
	}
	snapshot, err := skillsource.Normalize(ctx, raw, source.Skill, pin.SourceKey)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if snapshot.TreeDigest != pin.TreeDigest {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("normalized local Skill artifact differs from uap.lock")
	}
	snapshot.Source = localSkillSnapshotSource(pin)
	return snapshot, nil
}

func (app App) acquireLocalSkillRaw(ctx context.Context, serialized string) (domain.PackageSnapshot, error) {
	root, err := nativeLocalSkillPath(serialized)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if app.SourceAcquirer == nil {
		return domain.PackageSnapshot{}, errors.New("local Skill acquisition unavailable")
	}
	raw, err := app.SourceAcquirer.AcquireLocal(ctx, root)
	if err != nil && ctx.Err() != nil {
		return domain.PackageSnapshot{}, ctx.Err()
	}
	return raw, err
}
