package agentpluginscli

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func buildDependencyRepairs(ctx context.Context, app App, state domain.StateFileV2, installation domain.Installation, targets []domain.ClientID) ([]dependencyAction, error) {
	var actions []dependencyAction
	for _, target := range targets {
		authority, err := environmentAuthorityForBinding(state, installation.InstallationID, target, "repair")
		if err != nil {
			return nil, err
		}
		if authority == nil {
			return nil, errors.New("mixed owned and unmanaged repair requires separate selections")
		}
		binding, err := selectedRepairBinding(installation, target, domain.ScopeUser)
		if err != nil {
			return nil, err
		}
		rev := binding.PackageRevision
		if rev == nil {
			return nil, errors.New("recorded package revision is missing")
		}
		pkg := environment.LockedPackage{Alias: authority.Alias, SourceKey: authority.SourceKey, DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: rev.TreeDigest, ManifestDigest: rev.ManifestDigest, Targets: []string{string(target)}}
		pkg.Source.Git = &environment.LockedGitSource{Repository: installation.Source.Repository, URL: installation.Source.SourceURL, Path: installation.Source.PackageSubpath, Commit: rev.ResolvedRevision, Requested: environment.Selector{Kind: environment.SelectorCommit, Value: rev.ResolvedRevision}}
		if installation.Source.StableSourceKey != "" {
			facts := rev.Skill
			if facts == nil {
				return nil, errors.New("recorded Skill source provenance unavailable; restore the matching state/lock before repair")
			}
			pkg.Source.Git = &environment.LockedGitSource{Kind: environment.SkillKind, Skill: facts.Name, Repository: installation.Source.Repository, Path: facts.RequestedPath, Commit: rev.ResolvedRevision, Requested: environment.Selector{Kind: environment.SelectorCommit, Value: rev.ResolvedRevision}, ResolvedSkillPath: facts.ResolvedPath, RawTreeDigest: facts.RawTreeDigest, Normalizer: facts.Normalizer}
		}
		if installation.Source.SourceKind == "archive" {
			digest := strings.TrimPrefix(rev.ResolvedRevision, "sha256:")
			if !sourcespec.ValidSHA256(digest) {
				return nil, errors.New("recorded archive revision is invalid")
			}
			pkg.Source.Git = nil
			pkg.Source.Archive = &environment.LockedArchiveSource{URL: installation.Source.SourceURL, Path: installation.Source.PackageSubpath, SHA256: digest, RequestedSHA256: digest}
		}
		if installation.Directory != nil {
			pkg.Source.Git = nil
			pkg.Source.Registry = &environment.LockedRegistrySource{Distribution: installation.Directory.DistributionID, DistributionID: installation.Directory.DistributionID, ProductID: installation.Directory.ProductID, ReleaseSequence: rev.ReleaseSequence, Repository: installation.Source.Repository, Path: installation.Source.PackageSubpath, Commit: rev.ResolvedRevision}
		}
		if stored, e := app.dependencyStore().Get(ctx, pkg.DigestAlgorithm, pkg.TreeDigest); e == nil {
			pkg.ExecutableFiles = append([]string{}, stored.Metadata.ExecutableFiles...)
		}
		actions = append(actions, dependencyAction{Alias: authority.Alias, ClientID: string(target), InstallationID: installation.InstallationID, BindingID: binding.ClientBindingID, Action: "repair", Package: &pkg, Authority: authority})
	}
	return actions, nil
}

func applyDependencyRepairs(cmd *cobra.Command, app App, opts *options, actions []dependencyAction, detected map[domain.ClientID]domain.DetectedClient) error {
	consent := map[string]bool{}
	if _, err := preflightDependencyActions(cmd, app, opts, actions, detected, consent); err != nil {
		return err
	}
	if opts.dryRun {
		return renderDependencySync(cmd, opts, actions, true)
	}
	if err := requireNonInteractiveMutation(app, opts, "repair"); err != nil {
		return err
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		var err error
		confirmed, err = promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Repair the managed package and its recorded verification state? [y/N]")
		if err != nil {
			return err
		}
	}
	if !confirmed {
		return renderDependencySync(cmd, opts, actions, true)
	}
	adapter := &dependencyInstaller{app: app, opts: opts, cmd: cmd, detected: detected, consent: consent}
	results, err := (environment.Coordinator{Mutator: adapter}).ApplySync(cmd.Context(), environment.PreparedSync{EnvironmentID: environment.EnvironmentUserDefault, Bindings: actions}, true)
	if renderErr := renderDependencyResults(cmd, opts, actions, results, err); err == nil {
		err = renderErr
	}
	return err
}
