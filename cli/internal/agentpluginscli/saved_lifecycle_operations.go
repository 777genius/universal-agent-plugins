package agentpluginscli

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"github.com/spf13/cobra"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type savedLifecycleSelection struct {
	declaration  string
	alias        string
	installation *domain.Installation
	targets      []string
	retained     *environment.LockedPackage
}

// runSavedLifecycle precedes imperative target selection: desired targets can
// exist before the first installation does.
func runSavedLifecycle(cmd *cobra.Command, app App, opts *options, selector, operation string) (bool, error) {
	selection, err := selectSavedLifecycle(app, opts, selector)
	if err != nil {
		return true, err
	}
	if selection.alias == "" {
		return false, nil
	}
	if operation == "repair" {
		return selection.repair(cmd, app, opts)
	}
	if operation == "remove" && opts.purgeData {
		return true, errors.New("remove owned bindings before purging retained data; PLUGIN_DATA is preserved")
	}
	// Update replaces the recorded pins, so their old bytes are not a precondition.
	// runSavedEnvironment still verifies every pin of the candidate before publication.
	snapshot, err := snapshotEnvironment(cmd, app, operation != "remove" && operation != "update")
	if err != nil {
		return true, err
	}
	if operation == "remove" && selection.retained != nil {
		return true, renderSavedRetentionResult(cmd, opts, *selection.retained)
	}
	preview, cleanup, err := dependencyPreviewApp(app, opts)
	if err != nil {
		return true, err
	}
	defer cleanup()
	requirements, locked, manifest, err := selection.candidate(cmd, preview, opts, snapshot, operation)
	if err != nil {
		return true, err
	}
	var syncTargets []string
	if strings.TrimSpace(opts.target) != "" {
		syncTargets = selection.targets
	}
	if err := runSavedEnvironment(cmd, preview, opts, snapshot, requirements, locked, manifest, []string{selection.alias}, nil, syncTargets); err != nil {
		return true, err
	}
	return true, selection.renderNextAction(cmd, app, opts, requirements, locked, operation)
}

func selectSavedLifecycle(app App, opts *options, selector string) (savedLifecycleSelection, error) {
	selection := savedLifecycleSelection{}
	state, err := app.StateStore.Load()
	if err != nil {
		return selection, err
	}
	manifest, _ := app.dependencyPaths()
	requirements, err := readDependencyManifest(manifest)
	if err != nil && !os.IsNotExist(err) {
		return selection, err
	}
	selection.alias, selection.installation, err = resolveSavedAlias(state, requirements, selector, app.environmentID())
	if err != nil {
		return selection, err
	}
	selection, err = selectLockedLifecycle(app, state, requirements, selector, selection)
	if err != nil || selection.alias == "" {
		return selection, err
	}
	if opts.scope != "" && opts.scope != "user" && opts.scope != "project" {
		return selection, errors.New("saved dependencies support user scope only")
	}
	targets := savedAliasTargets(requirements, state, selection.alias, app.environmentID())
	if selection.declaration != "" {
		targets = savedAliasTargets(requirements, state, selection.declaration, app.environmentID())
	}
	if selection.retained != nil {
		targets = selection.retained.Targets
	}
	selection.targets, err = selectSavedTargets(opts.target, targets, selection.alias)
	return selection, err
}

func selectSavedTargets(option string, allTargets []string, alias string) ([]string, error) {
	if strings.TrimSpace(option) == "" {
		return append([]string(nil), allTargets...), nil
	}
	selected, err := parseTargetOption(option)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(selected))
	for _, target := range selected {
		if !slices.Contains(allTargets, string(target)) {
			return nil, fmt.Errorf("target %q does not belong to dependency %q", target, alias)
		}
		targets = append(targets, string(target))
	}
	sort.Strings(targets)
	return targets, nil
}

func (selection savedLifecycleSelection) repair(cmd *cobra.Command, app App, opts *options) (bool, error) {
	if selection.installation == nil {
		return true, fmt.Errorf("dependency %q is not installed; run agentplugins deps sync", selection.alias)
	}
	targets := make([]domain.ClientID, 0, len(selection.targets))
	for _, target := range selection.targets {
		targets = append(targets, domain.ClientID(target))
	}
	return repairOwnedDependencies(cmd, app, opts, selection.installation.InstallationID, targets)
}

func (selection savedLifecycleSelection) candidate(cmd *cobra.Command, app App, opts *options, snapshot environmentSnapshot, operation string) (environment.Requirements, environment.Lock, []byte, error) {
	requirements := snapshot.Requirements
	var locked environment.Lock
	var err error
	switch operation {
	case "remove":
		requirements = removeSavedTargets(requirements, dependencyRootKey(requirements, snapshot.Lock, selection.alias), selection.targets)
		locked, err = (environment.Coordinator{EnvironmentID: app.environmentID(), Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: opts.dryRun}}).Resolve(cmd.Context(), requirements, snapshot.Lock, "")
	case "update":
		requirements, locked, err = selection.update(cmd, app, opts, snapshot)
	default:
		err = fmt.Errorf("unsupported saved lifecycle operation %q", operation)
	}
	if err != nil {
		return requirements, locked, nil, err
	}
	manifest, err := editEnvironmentIntent(snapshot, requirements)
	return requirements, locked, manifest, err
}

func (selection savedLifecycleSelection) update(cmd *cobra.Command, app App, opts *options, snapshot environmentSnapshot) (environment.Requirements, environment.Lock, error) {
	requirements := snapshot.Requirements
	key := dependencyRootKey(requirements, snapshot.Lock, selection.alias)
	desired := savedAliasTargets(requirements, domain.StateFileV2{}, key, app.environmentID())
	if strings.TrimSpace(opts.target) != "" && !slices.Equal(selection.targets, desired) {
		return requirements, snapshot.Lock, fmt.Errorf("update %s must select every target of the alias", selection.alias)
	}
	return prepareSavedUpdates(cmd.Context(), app, snapshot, []string{key}, nil)
}

func (selection savedLifecycleSelection) renderNextAction(cmd *cobra.Command, app App, opts *options, requirements environment.Requirements, locked environment.Lock, operation string) error {
	if opts.format == "json" {
		return nil
	}
	if operation == "remove" {
		if pkg, exists := locked.Package(selection.alias); exists && len(pkg.RequiredBy) > 0 {
			return renderSavedRetention(cmd, pkg)
		}
	}
	requirement := requirements.Dependencies[dependencyRootKey(requirements, locked, selection.alias)]
	if operation == "update" && requirement.Registry != nil && requirement.Registry.Selector.Kind == environment.SelectorVersion {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Exact version pin retained; choose a new version in uap.toml")
		return err
	}
	if operation == "update" && requirement.Git != nil && requirement.Git.Selector.Kind == environment.SelectorCommit {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Exact commit pin retained; choose a new pin with agentplugins add <repo>@<sha>//path")
		return err
	}
	if operation == "update" && requirement.Archive != nil && requirement.Archive.SHA256 != "" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "Exact archive checksum retained; choose a new checksum in uap.toml")
		return err
	}
	if operation == "remove" && selection.installation != nil && len(savedAliasTargets(requirements, domain.StateFileV2{}, selection.alias, app.environmentID())) == 0 {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "PLUGIN_DATA preserved; after removal completes, run %s to delete retained data\n", retainedDataPurgeCommand(app, selection.installation.InstallationID))
		return err
	}
	return nil
}
