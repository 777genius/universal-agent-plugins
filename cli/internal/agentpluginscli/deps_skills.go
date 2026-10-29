package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// skillRawAcquirer preserves raw Skill tree semantics independently of plugin
// acquisition, which may omit nested repository content.
type skillRawAcquirer interface {
	AcquireGitHubSkill(context.Context, string, string, string) (domain.PackageSnapshot, error)
	AcquireGitHubSkillVerified(context.Context, string, string, string, string) (domain.PackageSnapshot, error)
}

func (app App) rawSkillAcquirer() (skillRawAcquirer, error) {
	acquirer, ok := app.SourceAcquirer.(skillRawAcquirer)
	if !ok {
		return nil, errors.New("raw Skill acquisition unavailable")
	}
	return acquirer, nil
}

func (resolver dependencyResolver) resolveSkill(ctx context.Context, requirement environment.Requirement, targets []string, previous *environment.LockedPackage, commit, resolvedRef, defaultBranch string) (environment.LockedPackage, error) {
	source := requirement.Git
	resolved := source.Path
	if finder, ok := resolver.app.SourceAcquirer.(interface {
		ResolveGitHubSkill(context.Context, string, string, string, string, string) (string, error)
	}); ok && previous != nil {
		last := source.Path
		if previous != nil && previous.Source.Git != nil {
			last = previous.Source.Git.ResolvedSkillPath
		}
		var err error
		resolved, err = finder.ResolveGitHubSkill(ctx, source.Repository, commit, source.Skill, source.Path, last)
		if err != nil {
			return environment.LockedPackage{}, err
		}
	}
	if resolved != "" && path.Base(resolved) != source.Skill {
		return environment.LockedPackage{}, errors.New("skill name differs from source directory")
	}
	acquirer, err := resolver.app.rawSkillAcquirer()
	if err != nil {
		return environment.LockedPackage{}, err
	}
	raw, err := acquirer.AcquireGitHubSkill(ctx, source.Repository, commit, resolved)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	key := mustSourceKey(requirement)
	snapshot, err := skillsource.Normalize(ctx, raw, source.Skill, key)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	pin := environment.LockedPackage{Alias: requirement.Alias, SourceKey: key, Targets: append([]string{}, targets...), DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: environment.LockedSource{Git: &environment.LockedGitSource{Kind: environment.SkillKind, Skill: source.Skill, Repository: source.Repository, Path: source.Path, Requested: source.Selector, Commit: commit, ResolvedRef: resolvedRef, DefaultBranch: defaultBranch, ResolvedSkillPath: resolved, RawTreeDigest: raw.TreeDigest, Normalizer: environment.SkillNormalizerV1}}}
	snapshot.Source = skillSnapshotSource(pin)
	loaded, err := resolver.app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return environment.LockedPackage{}, err
	}
	pin.ManifestDigest = loaded.envelope.ManifestDigest
	return pin, resolver.finishLoaded(ctx, loaded, pin)
}

func skillSnapshotSource(pin environment.LockedPackage) domain.SourceIdentity {
	source := pin.Source.Git
	identity := githubIdentity(source.Repository, source.Commit, source.ResolvedSkillPath)
	return domain.SourceIdentity{Skill: &domain.SkillSourceFacts{Name: source.Skill, RequestedPath: source.Path, ResolvedPath: source.ResolvedSkillPath, RawTreeDigest: source.RawTreeDigest, Normalizer: source.Normalizer}, RequestedSource: identity, CanonicalSource: identity, Repository: source.Repository, PackageSubpath: source.ResolvedSkillPath, ResolvedRevision: source.Commit, SourceBindingHint: "uap-dependency-skill", StableSourceKey: pin.SourceKey}
}

func (app App) acquireLockedSkill(ctx context.Context, pin environment.LockedPackage) (domain.PackageSnapshot, error) {
	source := pin.Source.Git
	if source.Normalizer != environment.SkillNormalizerV1 {
		return domain.PackageSnapshot{}, errors.New("unsupported locked Skill normalizer")
	}
	if source.ResolvedSkillPath != "" && path.Base(source.ResolvedSkillPath) != source.Skill {
		return domain.PackageSnapshot{}, errors.New("locked Skill name differs from source directory")
	}
	acquirer, err := app.rawSkillAcquirer()
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	raw, err := acquirer.AcquireGitHubSkillVerified(ctx, source.Repository, source.Commit, source.ResolvedSkillPath, source.RawTreeDigest)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	defer func() { _ = packagedigest.Remove(raw) }()
	snapshot, err := skillsource.Normalize(ctx, raw, source.Skill, pin.SourceKey)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if snapshot.TreeDigest != pin.TreeDigest {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("normalized Skill artifact differs from uap.lock")
	}
	snapshot.Source = skillSnapshotSource(pin)
	return snapshot, nil
}

func (adapter *dependencyInstaller) authorize(ctx context.Context, loaded *loadedPackage) error {
	if adapter.cmd != nil {
		return authorizeSecurityAssessment(adapter.cmd, adapter.app, adapter.opts, loaded)
	}
	if loaded.security == nil {
		return errors.New("scanner unavailable")
	}
	if adapter.previewWarnings != nil && loaded.security.Counts.Warnings > 0 {
		*adapter.previewWarnings = append(*adapter.previewWarnings, dependencySecurityWarnings(loaded.security)...)
	}
	if loaded.security.Counts.Blocking > 0 && !adapter.opts.acceptSecurityRisk {
		return fmt.Errorf("%w: %d blocking findings", installer.ErrAssessmentRejected, loaded.security.Counts.Blocking)
	}
	loaded.securityAuthorized = true
	return ctx.Err()
}

// These public summaries follow the pinned lintai v0.1.3 rule metadata in
// crates/lintai-ai-security/src/registry/{json,github_workflow}.rs. Raw finding
// messages and paths can contain source-controlled secrets, so they are never
// forwarded to management diagnostics. Unknown rules keep their safe identity.
var dependencySecurityWarningReasons = map[string]string{
	"SEC302": "configuration contains an insecure http:// endpoint",
	"SEC324": "GitHub Actions workflow uses a third-party action that is not pinned to a full commit SHA",
	"SEC325": "GitHub Actions workflow interpolates untrusted expression data directly inside a run command",
	"SEC326": "GitHub Actions pull_request_target workflow checks out untrusted pull request head content",
	"SEC327": "GitHub Actions workflow grants GITHUB_TOKEN write-all permissions",
	"SEC328": "GitHub Actions workflow combines explicit write-capable permissions with a third-party action",
	"SEC329": "MCP config launches an external package dynamically at runtime",
}

func dependencySecurityWarnings(assessment *domain.SecurityAssessment) []string {
	var warnings []string
	published := 0
	for _, finding := range assessment.Findings {
		if finding.Disposition != "warning" {
			continue
		}
		published++
		if len(warnings) < securityFindingPreviewLimit {
			warnings = append(warnings, publicSecurityWarning(finding.Code))
		}
	}
	if omitted := published - len(warnings); omitted > 0 {
		warnings = append(warnings, fmt.Sprintf("%d additional %s omitted from this bounded preview", omitted, plural(omitted, "warning", "warnings")))
	}
	if unavailable := assessment.Counts.Warnings - published; unavailable > 0 {
		warnings = append(warnings, fmt.Sprintf("%d additional %s; details unavailable", unavailable, plural(unavailable, "warning", "warnings")))
	}
	return warnings
}

func publicSecurityWarning(code string) string {
	if len(code) < 6 || len(code) > 9 || code[:3] != "SEC" {
		return "Security warning; details unavailable"
	}
	for _, digit := range code[3:] {
		if digit < '0' || digit > '9' {
			return "Security warning; details unavailable"
		}
	}
	reason, ok := dependencySecurityWarningReasons[code]
	if !ok {
		reason = "security warning; details unavailable"
	}
	return prompt.SafeText(code + ": " + reason)
}
