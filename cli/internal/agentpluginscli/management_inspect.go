package agentpluginscli

import (
	"context"
	"os"
	"sort"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func (b *ManagementBackend) Inspect(ctx context.Context) (management.Environment, error) {
	out := management.Environment{ContextId: b.ContextID(), Entries: []management.EnvironmentEntry{}, Diagnostics: []management.Diagnostic{}}
	view, err := b.app.Installer.Inspect(ctx)
	if err != nil {
		return out, err
	}
	if view.Recovery.Required {
		out.Diagnostics = append(out.Diagnostics, managementDiagnostic("RECOVERY_REQUIRED", "Inspect and recover pending installer receipts before a new change"))
	}
	matching, err := b.matchingManagementBindings()
	if err != nil {
		return out, err
	}
	requirements, locked := b.inspectManagementIntent(&out)
	entries := map[string]*management.EnvironmentEntry{}
	for _, pkg := range locked.Packages {
		entry := management.EnvironmentEntry{Alias: pkg.Alias, Kind: management.EnvironmentEntryKindPlugin, Source: pkg.SourceKey, Version: pkg.Version, UpdateStatus: management.EnvironmentEntryUpdateStatusCheckUnavailable, Bindings: []management.Binding{}}
		if pkg.Source.Web != nil || pkg.Source.Git != nil && pkg.Source.Git.Kind == environment.SkillKind {
			entry.Kind = management.EnvironmentEntryKindSkill
		}
		key := dependencyRootKey(requirements, locked, pkg.Alias)
		req, declared := requirements.Dependencies[key]
		entry.Declared = declared
		if declared && !environment.MovingRequirement(req) && (req.Registry == nil || req.Registry.Selector.Kind == environment.SelectorVersion) {
			entry.UpdateStatus = management.EnvironmentEntryUpdateStatusPinned
		}
		entries[pkg.Alias] = &entry
	}
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			b.appendManagedBinding(entries, installation, binding, matching[binding.BindingID])
			if !matching[binding.BindingID] {
				out.Diagnostics = append(out.Diagnostics, managementDiagnostic("BINDING_OUTSIDE_CONTEXT", "A recorded native binding belongs to another profile or cannot be matched to this captured context"))
			}
		}
	}
	for _, entry := range entries {
		out.Entries = append(out.Entries, *entry)
	}
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].Alias < out.Entries[j].Alias })
	return out, nil
}
func (b *ManagementBackend) inspectManagementIntent(out *management.Environment) (environment.Requirements, environment.Lock) {
	manifest, lockPath := b.app.dependencyPaths()
	requirements, err := readDependencyManifest(manifest)
	if os.IsNotExist(err) {
		return environment.Requirements{}, environment.Lock{}
	}
	if err != nil {
		out.Diagnostics = append(out.Diagnostics, managementDiagnostic("MANIFEST_INVALID", "Desired environment cannot be read; inspect uap.toml"))
		return environment.Requirements{}, environment.Lock{}
	}
	locked, err := readDependencyLock(lockPath, requirements)
	if err != nil {
		out.Diagnostics = append(out.Diagnostics, managementDiagnostic("LOCK_INVALID", "uap.toml and uap.lock need reconciliation before locked restoration"))
		return requirements, environment.Lock{}
	}
	return requirements, locked
}
func (b *ManagementBackend) appendManagedBinding(entries map[string]*management.EnvironmentEntry, installation installer.InspectedInstallation, binding installer.InspectedBinding, matching bool) {
	alias := installation.InstallationID
	if binding.Owner != nil {
		alias = binding.Owner.Alias
	}
	entry := entries[alias]
	if entry == nil {
		entry = &management.EnvironmentEntry{Alias: alias, Source: installation.SourceKey, Kind: management.EnvironmentEntryKindPlugin, UpdateStatus: management.EnvironmentEntryUpdateStatusBlocked, Bindings: []management.Binding{}}
		entries[alias] = entry
	}
	status := "installed"
	if binding.ProjectionStatus != "" {
		status = binding.ProjectionStatus
	} else if retry, _ := environment.LifecycleFacts(binding); retry {
		status = "incomplete"
	}
	if binding.Owner == nil {
		status = "unmanaged"
	}
	projected := management.Binding{TargetId: b.targetID(binding.ClientID), Client: binding.ClientID, Scope: management.Scope(binding.Scope), Status: status, ManagedPaths: append([]string{}, binding.ManagedPaths...)}
	if matching {
		projected.Readiness = &management.Readiness{Materialization: binding.Materialization, Activation: binding.Activation, Authentication: binding.Authentication, Policy: binding.Policy, Verification: binding.Verification}
	} else {
		projected.Status = "outside_context"
		entry.UpdateStatus = management.EnvironmentEntryUpdateStatusBlocked
	}
	entry.Bindings = append(entry.Bindings, projected)
}

func (b *ManagementBackend) matchingManagementBindings() (map[string]bool, error) {
	state, err := b.app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	profiles := map[string]string{}
	for _, target := range b.config.Targets {
		profiles[target.Client] = target.ConfigRoot
	}
	matching := map[string]bool{}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			root := profiles[binding.ClientID]
			matching[binding.ClientBindingID] = root != "" && binding.Scope == string(domain.ScopeUser) && managementBindingInProfile(binding, root)
		}
	}
	return matching, nil
}
