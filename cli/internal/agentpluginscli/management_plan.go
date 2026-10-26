package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"sort"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type managementPrepared struct {
	mu                 sync.Mutex
	backend            *ManagementBackend
	app, preview       App
	input              management.ChangeRequest
	clients            []string
	request            dependencySyncRequest
	cleanup            func()
	closed, applied    bool
	operationIDs       map[string]string
	paths              map[string]string
	review             management.Plan
	warnings           []string
	externalUninstalls map[string]bool
}

func (b *ManagementBackend) finishManagementPreparation(ctx context.Context, preview App, cleanup func(), input management.ChangeRequest, clients []string, request dependencySyncRequest, operationID string) (*managementPrepared, error) {
	if operationID == "" {
		return nil, errors.New("operation identity required")
	}
	if err := b.validateManagementContext(ctx); err != nil {
		return nil, err
	}
	if !request.repairOnly && request.lock.LockVersion != 0 {
		if err := request.lock.Validate(request.requirements); err != nil {
			return nil, err
		}
		if err := verifyDependencyDeclarations(ctx, preview, request.lock); err != nil {
			return nil, err
		}
	}
	p := &managementPrepared{backend: b, app: b.app, preview: preview, cleanup: cleanup, input: input, clients: clients, request: request}
	p.app.dependencySnapshots = preview.dependencySnapshots
	actions, detected, paths, err := p.prepareBindings(ctx, nil)
	if err != nil {
		return nil, err
	}
	p.request.prepared, p.request.detected, p.paths = actions, detected, paths
	p.operationIDs = managementOperationIDs(operationID, actions)
	p.review = management.Plan{Action: input.Action, ContextId: b.ContextID(), Confirmable: true, Actions: []management.PlanAction{}, Warnings: []management.Diagnostic{}, Conflicts: []management.Diagnostic{}}
	for _, action := range actions {
		entry := management.PlanAction{Action: string(action.Action), Alias: action.Alias, TargetId: b.targetID(action.ClientID), Description: action.Reason, DestinationPath: paths[action.Alias+"\x00"+action.ClientID]}
		if action.Package != nil {
			entry.ToVersion = action.Package.Version
		}
		p.review.Actions = append(p.review.Actions, entry)
	}
	for _, warning := range p.warnings {
		p.review.Warnings = append(p.review.Warnings, managementDiagnostic("INSTALLER_WARNING", warning))
	}
	if err := p.reviewExternalUninstalled(); err != nil {
		return nil, err
	}
	if input.Action == management.Update {
		p.review.Warnings = append(p.review.Warnings, managementPinnedWarnings(input, request)...)
	}

	return p, nil
}
func managementOperationIDs(operationID string, actions []dependencyAction) map[string]string {
	ids := map[string]string{}
	for _, action := range actions {
		if action.Action == environment.ActionUnchanged {
			continue
		}
		key := action.Alias + "\x00" + action.ClientID
		digest := sha256.Sum256([]byte(operationID + "\x00" + key))
		ids[key] = "mgmt-" + hex.EncodeToString(digest[:20])
	}
	return ids
}
func (p *managementPrepared) OperationIDs() []string {
	ids := make([]string, 0, len(p.operationIDs))
	for _, id := range p.operationIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (p *managementPrepared) Review() management.Plan { return p.review }
func (p *managementPrepared) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		p.cleanup()
	}
}
func (p *managementPrepared) Check(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.applied {
		return installer.ErrPlanChanged
	}
	return p.check(ctx)
}
func (p *managementPrepared) check(ctx context.Context) error {
	if len(p.externalUninstalls) > 0 && !p.input.ExternalUninstalled {
		return managementRefusal("EXTERNAL_UNINSTALL_REQUIRED", "Confirm external removal or that the listed plugins were never activated, then prepare a new review")
	}
	if err := p.backend.validateManagementContext(ctx); err != nil {
		return err
	}
	if err := p.request.checkInputs(); err != nil {
		return err
	}
	actions, detected, paths, err := p.prepareBindings(ctx, p.request.prepared)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actions, p.request.prepared) || !reflect.DeepEqual(paths, p.paths) {
		return installer.ErrPlanChanged
	}
	for id, prior := range p.request.detected {
		current := detected[id]
		if prior.ConfigRoot != current.ConfigRoot || prior.ExecutablePath != current.ExecutablePath {
			return installer.ErrPlanChanged
		}
	}
	return nil
}
func (p *managementPrepared) prepareBindings(ctx context.Context, prior []dependencyAction) ([]dependencyAction, map[domain.ClientID]domain.DetectedClient, map[string]string, error) {
	view, err := p.preview.Installer.Inspect(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if view.Recovery.Required {
		return nil, nil, nil, installer.ErrRecoveryRequired
	}
	if p.input.Action == management.Sync || p.input.Action == management.Recover {
		if err := p.backend.validateManagementEnvironment(ctx, p.request.requirements); err != nil {
			return nil, nil, nil, err
		}
	}
	actions, err := p.bindingActions(ctx, view)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = p.backend.validateManagementActions(ctx, actions); err != nil {
		return nil, nil, nil, err
	}
	if err = reserveSkillBindingIDs(view, actions, prior); err != nil {
		return nil, nil, nil, err
	}
	detected, err := detectDependencyClients(ctx, p.preview)
	if err != nil {
		return nil, nil, nil, err
	}
	paths := skillBindingPaths(view, actions)
	warnings := []string{}
	externalUninstalls := map[string]bool{}
	adapter := &dependencyInstaller{previewExternalUninstalled: externalUninstalls, app: p.preview, opts: &options{externalUninstalled: p.input.ExternalUninstalled}, detected: detected, previewPaths: paths, operationIDs: p.operationIDs, previewWarnings: &warnings}
	actions, err = preflightDependencyAdapter(ctx, adapter, actions)
	if err != nil {
		return nil, nil, nil, err
	}
	if prior == nil {
		p.warnings, p.externalUninstalls = warnings, externalUninstalls
	} else if !reflect.DeepEqual(p.externalUninstalls, externalUninstalls) {
		return nil, nil, nil, installer.ErrPlanChanged
	}
	return actions, detected, paths, nil
}
func (p *managementPrepared) bindingActions(ctx context.Context, view installer.Inspection) ([]dependencyAction, error) {
	if p.input.Action == management.Repair {
		return p.repairActions(ctx)
	}
	if len(p.request.requirements.Dependencies) == 0 && p.request.lock.LockVersion == 0 {
		return []dependencyAction{}, nil
	}
	observed, err := environment.Observe(view)
	if err != nil {
		return nil, err
	}
	var prepared environment.PreparedSync
	if len(p.request.aliases) > 0 {
		prepared, err = (environment.Coordinator{}).PrepareSyncFor(p.request.requirements, p.request.lock, observed, p.request.aliases, nil)
	} else {
		prepared, err = (environment.Coordinator{}).PrepareSync(p.request.requirements, p.request.lock, observed)
	}
	if err == nil && p.input.Action == management.Recover {
		return prepareManagementNativeRecovery(ctx, p.preview, prepared.Bindings)
	}
	return prepared.Bindings, err
}
func (p *managementPrepared) repairActions(ctx context.Context) ([]dependencyAction, error) {
	state, err := p.preview.StateStore.Load()
	if err != nil {
		return nil, err
	}
	out := []dependencyAction{}
	for _, key := range p.request.aliases {
		alias := dependencyNodeAlias(p.request.requirements, p.request.lock, key)
		installation, err := savedAliasInstallation(state, alias, alias)
		if err != nil {
			return nil, err
		}
		if installation == nil {
			return nil, installer.ErrNotInstalled
		}
		clients := p.clients
		if len(clients) == 0 {
			clients = savedAliasTargets(p.request.requirements, state, alias)
		}
		targets := []domain.ClientID{}
		for _, client := range clients {
			targets = append(targets, domain.ClientID(client))
		}
		actions, err := buildDependencyRepairs(ctx, p.preview, state, *installation, targets)
		if err != nil {
			return nil, err
		}
		out = append(out, actions...)
	}
	return out, nil
}

func managementPinnedWarnings(input management.ChangeRequest, request dependencySyncRequest) []management.Diagnostic {
	selected, _ := managementAliases(request.requirements, request.lock, input.Aliases, input.All)
	warnings := []management.Diagnostic{}
	for _, alias := range selected {
		req := request.requirements.Dependencies[alias]
		if !environment.MovingRequirement(req) && (req.Registry == nil || req.Registry.Selector.Kind == environment.SelectorVersion) {
			warning := managementDiagnostic("PINNED", "Explicit immutable intent is retained for "+alias)
			warning.Alias = alias
			warnings = append(warnings, warning)
		}
	}
	return warnings
}
