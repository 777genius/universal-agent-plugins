package agentpluginscli

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func inspectSavedAddRoot(ctx context.Context, app App, opts *options, source string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if app.Installer == nil {
		return nil
	}
	// Preserve recovery and manifest/lock refusals before ownership advice. This
	// snapshot reads intent only; source declarations are verified by saved add.
	snapshot, err := snapshotEnvironmentContext(ctx, app, false)
	if err != nil {
		return err
	}
	key := savedDirectSourceKey(source)
	if match := exactGitPattern.FindStringSubmatch(source); match != nil {
		key = "github:" + strings.ToLower(match[1]) + ":" + match[3]
	}
	if canPreviewSavedAddOwnership(snapshot, opts.alias, "", key) {
		return checkSavedAddOwnership(ctx, app, key, opts.alias)
	}
	// Unsaved selectors need actual acquired distribution identity. Product-level
	// retained state cannot prove which distribution this operation will select.
	return nil
}

// Existing alias and saved-selector decisions belong to the normal resolver.
// Defer their ownership diagnosis rather than masking a more specific refusal.
func canPreviewSavedAddOwnership(snapshot environmentSnapshot, alias, derivedAlias, key string) bool {
	if alias != "" || key == "" {
		return false
	}
	if previous, exists := snapshot.Requirements.Dependencies[derivedAlias]; derivedAlias != "" && exists && mustSourceKey(previous) != key {
		return false
	}
	for _, pin := range snapshot.Lock.Packages {
		if pin.SourceKey == key {
			return false
		}
	}
	return true
}

// Annotations use the actual acquired source, while alias and saved-selector
// refusals remain with the authoritative resolver. Product identity alone is
// insufficient to diagnose ownership of an unselected Directory distribution.
func inspectSavedAddSelection(ctx context.Context, app App, opts *options, source string, loaded *loadedPackage) (map[domain.ClientID]prompt.TargetChoice, error) {
	if app.Installer == nil || loaded == nil || explicitLocalPath(source) || localArchiveInput(source) {
		return nil, nil
	}
	key := savedDirectSourceKey(source)
	if loaded.directorySelection != nil {
		key = "directory:" + loaded.directorySelection.DistributionID
	}
	if key == "" {
		return nil, nil
	}
	alias := dependencyManifestAlias(loaded.envelope.Manifest.Name)
	snapshot, err := snapshotEnvironmentContext(ctx, app, false)
	if err != nil {
		return nil, err
	}
	if canPreviewSavedAddOwnership(snapshot, opts.alias, alias, key) {
		if err := checkSavedAddOwnership(ctx, app, key, alias); err != nil {
			return nil, err
		}
	}
	view, err := app.Installer.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	observed, err := environment.ObserveForEnvironment(view, app.environmentID())
	if err != nil {
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
