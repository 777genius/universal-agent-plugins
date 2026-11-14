package grok

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func removeEnabledProject(ctx context.Context, env clients.Env, request domain.DeactivationRequest, outcome domain.DeactivationOutcome) (result domain.DeactivationOutcome, resultErr error) {
	result = outcome
	recovery, err := enablementAuthority(env, request.TargetContext.ProjectRoot, request.Client.ConfigRoot)
	if err != nil {
		return result, err
	}
	state, index, binding, err := recovery.removalAuthority(request)
	if err != nil {
		return result, err
	}
	owned, err := enabledObjectForProfile(binding.NativeObjects, binding.TargetLocator, request.DeclaredName, recovery.UserRoot)
	if err != nil || owned == nil {
		return result, errors.Join(err, fmt.Errorf("grok project enabled ownership is unavailable"))
	}
	if request.BackendExecutable == "" || request.BackendExecutable != request.Client.ExecutablePath {
		return result, fmt.Errorf("grok owned enablement removal requires its selected native executable")
	}
	profile, err := recovery.openEnablementProfile(ctx)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, profile.close()) }()
	file := profile.config
	plugins, err := readEnablementInventory(ctx, env.Runner, request.BackendExecutable, recovery.ProjectRoot, env.UserHome)
	if err != nil {
		return result, err
	}
	version := state.Installations[index].Package.Version
	if binding.PackageRevision != nil {
		version = binding.PackageRevision.Version
	}
	plugin, err := selectEnablementPlugin(plugins, binding.TargetLocator, request.DeclaredName, version)
	if err != nil {
		return result, err
	}
	record, after, err := recovery.prepareRemoval(state, index, binding, *owned, plugin, file.Original(), profile.parent)
	if err != nil {
		return result, err
	}
	result.NativeEffect = domain.NativeEffectUncertain
	if err := recovery.removeMembership(ctx, env, request, state, record, file, after, plugin); err != nil {
		return result, err
	}
	result.NativeEffect, result.ExternalRemovalComplete = domain.NativeEffectCommitted, true
	return result, nil
}

func (recovery EnablementRecovery) removalAuthority(request domain.DeactivationRequest) (domain.StateFileV2, int, domain.ClientBinding, error) {
	state, err := recovery.State.Load()
	if err != nil {
		return state, -1, domain.ClientBinding{}, err
	}
	id := request.NativeAttempt
	if id.OperationID == "" || !enablementHash(id.StateDigest) || id.StateDigest != enablementStateDigest(state) || id.NativeRoot != filepath.Join(recovery.ProjectRoot, ".grok") {
		return state, -1, domain.ClientBinding{}, fmt.Errorf("grok removal lacks the exact frozen native attempt state")
	}
	index, selected := -1, domain.ClientBinding{}
	for at, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.TargetLocator != request.ManagedArtifactPath {
				continue
			}
			if index != -1 || !matchesRemovalBinding(request, installation, binding, recovery.ProjectRoot) {
				return state, -1, selected, fmt.Errorf("grok removal differs from its exact durable binding attempt")
			}
			index, selected = at, binding
		}
	}
	if index == -1 || shared.ManagedPackageDigest(selected) == "" {
		return state, -1, selected, fmt.Errorf("grok owned removal has no matching native attempt")
	}
	return state, index, selected, New().ValidateBindingProfile(selected.NativeProfileRoot, selected)
}

func matchesRemovalBinding(request domain.DeactivationRequest, installation domain.Installation, binding domain.ClientBinding, project string) bool {
	id := request.NativeAttempt
	return installation.InstallationID == id.InstallationID && binding.ClientBindingID == id.BindingID &&
		installation.DeclaredName == request.DeclaredName && binding.ClientID == "grok" && binding.Scope == "project" &&
		binding.NativeProfileRoot == filepath.Join(project, ".grok") && binding.PhysicalArtifact == request.PhysicalArtifactID &&
		binding.NativeActivationAttempt == id.OperationID && reflect.DeepEqual(binding.NativeObjects, request.NativeObjects)
}

func (recovery EnablementRecovery) prepareRemoval(state domain.StateFileV2, index int, binding domain.ClientBinding, owned domain.NativeObjectOwnership, plugin grokACPPlugin, original nativeconfig.FileSnapshot, parent directoryidentity.Authority) (enablementRecord, []byte, error) {
	edit, after, err := prepareDisablementEdit(original, plugin.id)
	if err != nil {
		return enablementRecord{}, nil, err
	}
	before := binding
	before.NativeActivationAttempt = ""
	record := enablementRecord{Version: 2, Action: "remove", OperationID: binding.NativeActivationAttempt,
		InstallationID: state.Installations[index].InstallationID, BindingID: binding.ClientBindingID,
		ProjectRoot: recovery.ProjectRoot, UserRoot: recovery.UserRoot, UserParentAuthority: parent, ArtifactPath: binding.TargetLocator,
		PackageDigest: shared.ManagedPackageDigest(binding), PreviousObject: &owned, PluginID: plugin.id, PluginName: plugin.name,
		BeforeStateDigest: enablementStateDigest(enablementState(state, index, before)), AttemptStateDigest: enablementStateDigest(state), Edit: edit}
	if err := recovery.validateRecord(record); err != nil {
		return record, nil, err
	}
	return record, after, recovery.requireState(state)
}

func (recovery EnablementRecovery) removeMembership(ctx context.Context, env clients.Env, request domain.DeactivationRequest, state domain.StateFileV2, record enablementRecord, file *nativeconfig.ExactFile, after []byte, before grokACPPlugin) error {
	if err := recovery.PackageVerifier.Verify(ctx, record.ArtifactPath, record.PackageDigest); err != nil {
		return err
	}
	if err := recovery.writeRecord(record); err != nil {
		return err
	}
	if err := record.applyConfig(file, after); err != nil {
		return errors.Join(err, record.rollbackConfig(file))
	}
	plugins, err := readEnablementInventory(ctx, env.Runner, request.BackendExecutable, recovery.ProjectRoot, env.UserHome)
	if err != nil {
		return err
	}
	plugin, err := selectEnablementPlugin(plugins, before.root, before.name, before.version)
	if err != nil || plugin.enabled || plugin.trusted != before.trusted {
		return errors.Join(err, fmt.Errorf("grok native readback did not confirm removal of the exact enabled membership"))
	}
	if err := recovery.verifyRecordConfig(record.Edit.AfterDigest, record.Edit.Mode); err != nil {
		return err
	}
	if err := recovery.PackageVerifier.Verify(ctx, record.ArtifactPath, record.PackageDigest); err != nil {
		return err
	}
	return errors.Join(recovery.requireState(state), record.revalidateParent())
}
