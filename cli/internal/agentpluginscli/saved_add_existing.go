package agentpluginscli

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Inspect through the installer so source identity, receipts and recovery use
// the same read-only observations as the authoritative saved operation.
func inspectSavedAddSelection(ctx context.Context, app App, source string, loaded *loadedPackage) (map[domain.ClientID]prompt.TargetChoice, error) {
	if app.Installer == nil || explicitLocalPath(source) || localArchiveInput(source) {
		return nil, nil
	}
	if loaded == nil {
		return nil, inspectSavedDirectoryOwnership(ctx, app, source)
	}
	key, err := savedSelectionSourceKey(source, *loaded)
	if err != nil {
		return nil, err
	}
	view, err := app.Installer.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	observed, err := environment.ObserveForEnvironment(view, app.environmentID())
	if err != nil {
		return nil, err
	}
	pkg := environment.LockedPackage{Alias: dependencyManifestAlias(loaded.envelope.Manifest.Name), SourceKey: key}
	if err := environment.CheckSourceOwnership(pkg, observed); err != nil {
		return nil, err
	}
	annotations := map[domain.ClientID]prompt.TargetChoice{}
	for _, binding := range observed {
		if binding.Managed && binding.SourceKey == key {
			annotations[domain.ClientID(binding.ClientID)] = prompt.TargetChoice{Note: "already in your list", Info: "kept even if unchecked"}
		}
	}
	return annotations, nil
}

func savedSelectionSourceKey(source string, loaded loadedPackage) (string, error) {
	requirement := environment.Requirement{Alias: "candidate"}
	if selection := loaded.directorySelection; selection != nil {
		requirement.Registry = &environment.RegistrySource{Distribution: selection.DistributionID,
			Selector: environment.Selector{Kind: environment.SelectorRelease, Value: fmt.Sprint(selection.ReleaseSequence)}}
	} else if key := savedDirectSourceKey(source); key != "" {
		return key, nil
	} else {
		identity := loaded.envelope.Source
		requirement.Git = &environment.GitSource{Repository: identity.Repository, Path: identity.PackageSubpath,
			Selector: environment.Selector{Kind: environment.SelectorCommit, Value: identity.ResolvedRevision}}
	}
	return requirement.SourceKey()
}
