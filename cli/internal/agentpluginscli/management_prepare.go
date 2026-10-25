package agentpluginscli

import (
	"context"
	"encoding/json"
	"slices"
	"sort"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (b *ManagementBackend) Prepare(ctx context.Context, input management.ChangeRequest, operationID string) (management.Prepared, error) {
	if err := validateManagementChange(input); err != nil {
		return nil, err
	}
	clients, err := b.clientTargets(input.TargetIds, input.Action == management.Install || input.Action == management.InstallSkills)
	if err != nil {
		return nil, err
	}
	if input.Action == management.Recover {
		return b.prepareRecovery(ctx, input, operationID)
	}
	if input.Action == management.InstallSkills {
		return b.prepareSkills(ctx, input, clients, operationID)
	}
	preview, cleanup, err := dependencyPreviewApp(b.app, &options{dryRun: true})
	if err != nil {
		return nil, err
	}
	plan, err := b.prepareEnvironment(ctx, preview, cleanup, input, clients, operationID)
	if err != nil {
		cleanup()
		return nil, err
	}
	return plan, nil
}

func (b *ManagementBackend) prepareEnvironment(ctx context.Context, preview App, cleanup func(), input management.ChangeRequest, clients []string, operationID string) (*managementPrepared, error) {
	snapshot, err := snapshotEnvironmentContext(ctx, preview, false)
	if err != nil {
		return nil, err
	}
	requirements := cloneSavedRequirements(snapshot.Requirements)
	locked := snapshot.Lock
	aliases := append([]string(nil), input.Aliases...)
	switch input.Action {
	case management.Install:
		requirements, locked, aliases, err = b.managementAdd(ctx, preview, input, clients, snapshot)
	case management.Update:
		aliases, err = managementAliases(requirements, locked, aliases, input.All)
		if err == nil {
			requirements, locked, err = managementUpdate(ctx, preview, requirements, locked, aliases, clients)
		}
	case management.Remove:
		aliases, err = managementAliases(requirements, locked, aliases, false)
		if err == nil {
			requirements, locked, err = managementRemove(ctx, preview, requirements, locked, aliases, clients)
		}
	case management.Repair:
		aliases, err = managementAliases(requirements, locked, aliases, false)
	case management.Sync:
		if snapshot.LockBytes == nil && len(requirements.Dependencies) > 0 {
			return nil, managementRefusal("LOCK_REQUIRED", "Locked restoration requires uap.lock; prepare recovery to reconcile the desired environment")
		}
	}
	if err != nil {
		return nil, err
	}
	request, err := managementSyncRequest(snapshot, requirements, locked, aliases, input.Action == management.Repair)
	if err != nil {
		return nil, err
	}
	return b.finishManagementPreparation(ctx, preview, cleanup, input, clients, request, operationID)
}

func managementSyncRequest(snapshot environmentSnapshot, requirements environment.Requirements, locked environment.Lock, aliases []string, repair bool) (dependencySyncRequest, error) {
	manifest, err := editEnvironmentIntent(snapshot, requirements)
	if err != nil {
		return dependencySyncRequest{}, err
	}
	if !repair {
		aliases = savedClosureAliases(snapshot.Lock, locked, aliases)
	}
	if len(aliases) > 0 {
		if err = verifyUnselectedPins(snapshot.Lock, locked, aliases); err != nil {
			return dependencySyncRequest{}, err
		}
	}
	body, err := json.MarshalIndent(locked, "", "  ")
	if err != nil {
		return dependencySyncRequest{}, err
	}
	body = append(body, '\n')
	if equalDependencyLock(snapshot.Lock, locked) || locked.LockVersion == 0 || snapshot.Lock.LockVersion == 0 && len(requirements.Dependencies) == 0 {
		body = snapshot.LockBytes
	}
	request := dependencySyncRequest{requirements: requirements, lock: locked, manifestPath: snapshot.ManifestPath, lockPath: snapshot.LockPath, manifestBytes: snapshot.ManifestBytes, lockBytes: snapshot.LockBytes, candidateManifest: manifest, candidateLock: body, aliases: aliases, repairOnly: repair}
	return request, nil
}

func (b *ManagementBackend) managementAdd(ctx context.Context, app App, input management.ChangeRequest, clients []string, snapshot environmentSnapshot) (environment.Requirements, environment.Lock, []string, error) {
	requirements := cloneSavedRequirements(snapshot.Requirements)
	source := input.Source
	spec, sourceErr := sourcespec.Parse(source)
	if explicitLocalPath(source) || sourceErr == nil && spec.Kind == sourcespec.KindLocalArchive {
		return requirements, environment.Lock{}, nil, managementRefusal("SOURCE_NOT_SUPPORTED", "Use a Git, HTTPS archive or signed catalog reference; local plugin paths are not supported by this API")
	}
	if input.Ref != "" {
		return requirements, environment.Lock{}, nil, invalidManagementRequest("Include the plugin revision in its source reference, for example owner/repository@commit")
	}
	targets := []domain.ClientID{}
	for _, client := range clients {
		targets = append(targets, domain.ClientID(client))
	}
	req, pin, err := resolveSavedAddContext(ctx, app, &options{alias: input.Alias, dryRun: true}, source, targets, exactGitPattern.FindStringSubmatch(source), snapshot)
	if err != nil {
		return requirements, environment.Lock{}, nil, err
	}
	requirements.Dependencies[req.Alias] = req
	for _, client := range clients {
		if !slices.Contains(requirements.Targets[client], req.Alias) {
			requirements.Targets[client] = append(requirements.Targets[client], req.Alias)
			sort.Strings(requirements.Targets[client])
		}
	}
	previous := seedSavedAdd(snapshot, req, pin)
	locked, err := (environment.Coordinator{Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: true}}).Resolve(ctx, requirements, previous, "")
	return requirements, locked, []string{dependencyNodeAlias(requirements, locked, req.Alias)}, err
}

func managementAliases(requirements environment.Requirements, locked environment.Lock, selected []string, all bool) ([]string, error) {
	if all {
		selected = nil
		for alias := range requirements.Dependencies {
			selected = append(selected, alias)
		}
	}
	aliases := []string{}
	for _, selector := range selected {
		key := dependencyRootKey(requirements, locked, selector)
		if _, declared := requirements.Dependencies[key]; !declared {
			return nil, managementRefusal("DEPENDENCY_NOT_DECLARED", "Select a declared dependency; transitive dependencies are retained by their owners")
		}
		if !slices.Contains(aliases, key) {
			aliases = append(aliases, key)
		}
	}
	sort.Strings(aliases)
	return aliases, nil
}
func managementRemove(ctx context.Context, app App, requirements environment.Requirements, locked environment.Lock, aliases, clients []string) (environment.Requirements, environment.Lock, error) {
	for _, alias := range aliases {
		targets := clients
		desired := savedAliasTargets(requirements, domain.StateFileV2{}, alias)
		if len(targets) == 0 {
			targets = desired
		}
		for _, client := range targets {
			if !slices.Contains(desired, client) {
				return requirements, locked, managementRefusal("TARGET_NOT_INSTALLED", "The selected target does not own this dependency")
			}
		}
		requirements = removeSavedTargets(requirements, alias, targets)
	}
	next, err := (environment.Coordinator{Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: true}}).Resolve(ctx, requirements, locked, "")
	return requirements, next, err
}
