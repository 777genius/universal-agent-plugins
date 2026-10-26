package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// PrepareSkills acquires exact selected commits in disposable storage and prepares
// one complete candidate graph. It never writes intent, persistent cache or profiles.
func (app App) PrepareSkills(ctx context.Context, input SkillsBatchRequest) (plan *PreparedSkillsBatch, err error) {
	if len(input.Selections) == 0 || len(input.TargetIDs) == 0 {
		return nil, errors.New("explicit skills and targets required")
	}
	if err := app.checkSkillsContext(input.TargetIDs); err != nil {
		return nil, err
	}
	preview, cleanup, err := dependencyPreviewApp(app, &options{dryRun: true})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	snapshot, err := snapshotEnvironmentContext(ctx, preview)
	if err != nil {
		return nil, err
	}
	candidate := newSkillsBatchCandidate(snapshot)
	targets := append([]string{}, input.TargetIDs...)
	slices.Sort(targets)
	targets = slices.Compact(targets)
	if err = candidate.selectSkills(ctx, preview, input.Selections, targets); err != nil {
		return nil, err
	}
	return candidate.prepare(ctx, app, preview, cleanup)
}

type skillsBatchCandidate struct {
	snapshot     environmentSnapshot
	requirements environment.Requirements
	previous     environment.Lock
	aliases      []string
	candidateIDs map[string]string
}

func newSkillsBatchCandidate(snapshot environmentSnapshot) *skillsBatchCandidate {
	previous := snapshot.Lock
	previous.Packages = append([]environment.LockedPackage{}, previous.Packages...)
	return &skillsBatchCandidate{snapshot: snapshot, requirements: cloneSavedRequirements(snapshot.Requirements), previous: previous, candidateIDs: map[string]string{}}
}

func (candidate *skillsBatchCandidate) resolve(ctx context.Context, app App) (environment.Lock, []byte, []string, error) {
	if err := validateDependencyTargets(app, candidate.requirements); err != nil {
		return environment.Lock{}, nil, nil, err
	}
	locked, err := (environment.Coordinator{EnvironmentID: app.environmentID(), Resolver: dependencyResolver{app: app, requirements: candidate.requirements, dryRun: true}}).Resolve(ctx, candidate.requirements, candidate.previous, "")
	if err != nil {
		return environment.Lock{}, nil, nil, err
	}
	if err = verifyDependencyDeclarations(ctx, app, locked); err != nil {
		return environment.Lock{}, nil, nil, err
	}
	manifest, err := editEnvironmentIntent(candidate.snapshot, candidate.requirements)
	if err != nil {
		return environment.Lock{}, nil, nil, err
	}
	aliases := savedClosureAliases(candidate.snapshot.Lock, locked, candidate.aliases)
	if err = verifyUnselectedPins(candidate.snapshot.Lock, locked, aliases); err != nil {
		return environment.Lock{}, nil, nil, err
	}
	return locked, manifest, aliases, nil
}

func (candidate *skillsBatchCandidate) prepare(ctx context.Context, app, preview App, cleanup func()) (*PreparedSkillsBatch, error) {
	locked, manifest, aliases, err := candidate.resolve(ctx, preview)
	if err != nil {
		return nil, err
	}
	warnings := []string{}
	bindings, detected, paths, err := prepareSkillsBindings(ctx, preview, candidate.requirements, locked, aliases, nil, &warnings)
	if err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(locked, "", "  ")
	if err != nil {
		return nil, err
	}
	snapshot := candidate.snapshot
	request := dependencySyncRequest{requirements: candidate.requirements, lock: locked, manifestPath: snapshot.ManifestPath, lockPath: snapshot.LockPath, manifestBytes: snapshot.ManifestBytes, lockBytes: snapshot.LockBytes, candidateManifest: manifest, candidateLock: append(body, '\n'), aliases: aliases, prepared: bindings, detected: detected}
	plan := &PreparedSkillsBatch{app: app, preview: preview, cleanup: cleanup, request: request, warnings: warnings}
	for _, binding := range bindings {
		plan.actions = append(plan.actions, SkillsAction{Alias: binding.Alias, ClientID: binding.ClientID, Action: string(binding.Action), TargetPath: paths[binding.Alias+"\x00"+binding.ClientID], CandidateID: candidate.candidateIDs[binding.Alias]})
	}
	return plan, nil
}

func prepareSkillsBindings(ctx context.Context, app App, requirements environment.Requirements, lock environment.Lock, aliases []string, prior []dependencyAction, warnings ...*[]string) ([]dependencyAction, map[domain.ClientID]domain.DetectedClient, map[string]string, error) {
	view, err := app.Installer.Inspect(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if view.Recovery.Required {
		return nil, nil, nil, installer.ErrRecoveryRequired
	}
	observed, err := environment.ObserveForEnvironment(view, app.environmentID())
	if err != nil {
		return nil, nil, nil, err
	}
	prepared, err := (environment.Coordinator{EnvironmentID: app.environmentID()}).PrepareSyncFor(requirements, lock, observed, aliases, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = reserveSkillBindingIDs(view, prepared.Bindings, prior); err != nil {
		return nil, nil, nil, err
	}
	detected, err := detectDependencyClients(ctx, app)
	if err != nil {
		return nil, nil, nil, err
	}
	paths := skillBindingPaths(view, prepared.Bindings)
	adapter := &dependencyInstaller{app: app, opts: app.skillsOptions(), detected: detected, previewPaths: paths}
	if len(warnings) > 0 {
		adapter.previewWarnings = warnings[0]
	}
	actions, err := preflightDependencyAdapter(ctx, adapter, prepared.Bindings)
	return actions, detected, paths, err
}

func reserveSkillBindingIDs(view installer.Inspection, bindings, prior []dependencyAction) error {
	ids := map[string]string{}
	for _, action := range prior {
		if action.InstallationID != "" {
			ids[action.Alias] = action.InstallationID
		}
	}
	for i := range bindings {
		action := &bindings[i]
		if action.Action != environment.ActionInstall || action.InstallationID != "" {
			continue
		}
		id := ids[action.Alias]
		retainedID, err := retainedSkillInstallationID(view, *action)
		if err != nil {
			return err
		}
		if retainedID != "" {
			if id != "" && id != retainedID {
				return installer.ErrPlanChanged
			}
			id = retainedID
		}
		if id == "" {
			var err error
			id, err = domain.NewInstallationID()
			if err != nil {
				return err
			}
		}
		ids[action.Alias] = id
		// Authority stays unbound until the installer commits its first receipt.
		action.InstallationID = id
	}
	return nil
}

// Last-binding removal retains installation identity and PLUGIN_DATA, but drops
// the live environment receipt. Reuse only an exact source with no bindings;
// name matches and foreign bindings never grant installation authority. The
// ordinary installer still validates the package and retained-data ownership.
func retainedSkillInstallationID(view installer.Inspection, action dependencyAction) (string, error) {
	if action.Package == nil || action.Package.SourceKey == "" {
		return "", nil
	}
	id := ""
	for _, installation := range view.Installations {
		if installation.SourceKey != action.Package.SourceKey || len(installation.Bindings) != 0 {
			continue
		}
		if id != "" && id != installation.InstallationID {
			return "", installer.ErrPlanChanged
		}
		id = installation.InstallationID
	}
	return id, nil
}

func skillBindingPaths(view installer.Inspection, actions []dependencyAction) map[string]string {
	byBinding := map[string]string{}
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			byBinding[binding.BindingID] = binding.TargetPath
		}
	}
	paths := map[string]string{}
	for _, action := range actions {
		if target := byBinding[action.BindingID]; target != "" {
			paths[action.Alias+"\x00"+action.ClientID] = target
		}
	}
	return paths
}
