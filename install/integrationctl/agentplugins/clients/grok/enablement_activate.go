package grok

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type enablementActivation struct {
	recovery *EnablementRecovery
	request  domain.ActivationRequest
	state    domain.StateFileV2
	index    int
	binding  domain.ClientBinding
	record   enablementRecord
	after    []byte
}

func enableProject(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (result domain.ActivationOutcome, resultErr error) {
	result = outcome
	recovery, err := enablementAuthority(env, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot)
	if err != nil {
		return result, err
	}
	state, index, binding, err := recovery.activationAuthority(request)
	if err != nil {
		return result, err
	}
	profile, err := recovery.openEnablementProfile(ctx)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, profile.close()) }()
	file := profile.config
	session := enablementActivation{recovery: recovery, request: request, state: state, index: index, binding: binding,
		record: enablementRecord{Version: 2, UserRoot: recovery.UserRoot, UserParentAuthority: profile.parent}}
	id := "project/" + enablementDigest([]byte(request.Delivery.ActivePath))[7:15] + "/" + request.DeclaredName
	if err := refuseDisabledMembership(file.Original().Body, id, request.DeclaredName); err != nil {
		return result, err
	}
	if err := session.record.revalidateParent(); err != nil {
		return result, err
	}
	plugins, err := readEnablementInventory(ctx, env.Runner, request.BackendExecutable, recovery.ProjectRoot, env.UserHome)
	if err != nil {
		return result, err
	}
	plugin, err := selectEnablementPlugin(plugins, request.Delivery.ActivePath, request.DeclaredName, request.Plan.DeclaredVersion)
	if err != nil {
		return result, err
	}
	repeated, err := session.prepare(ctx, file.Original(), plugin)
	if err != nil || repeated {
		if repeated {
			result.UserActions = []string{"The selected Grok project plugin is enabled; review trust and verify its components in Grok"}
		}
		return result, err
	}
	result.NativeEffect = domain.NativeEffectUncertain
	committed, err := session.commit(ctx, env, file, plugin)
	if committed {
		result.NativeEffect = domain.NativeEffectCommitted
		result.NativeObjects = append(append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...), session.record.object())
		result.UserActions = []string{"The selected Grok project plugin is enabled; review trust and verify its components in Grok"}
	}
	return result, err
}

func refuseDisabledMembership(body []byte, id, name string) error {
	_, lists, err := membershipDocument(body)
	if err != nil {
		return err
	}
	if membershipContains(lists["disabled"], id, name) {
		return fmt.Errorf("grok plugin was explicitly disabled; preserve the user's native preference")
	}
	return nil
}

func (session *enablementActivation) prepare(ctx context.Context, original nativeconfig.FileSnapshot, plugin grokACPPlugin) (bool, error) {
	request, recovery := session.request, session.recovery
	if err := session.record.revalidateParent(); err != nil {
		return false, err
	}
	owned, err := enabledObjectForProfile(request.PreviousNativeObjects, plugin.root, plugin.name, recovery.UserRoot)
	if err != nil {
		return false, err
	}
	current, err := enabledObjectForProfile(session.binding.NativeObjects, plugin.root, plugin.name, recovery.UserRoot)
	if err != nil || !reflect.DeepEqual(owned, current) {
		return false, fmt.Errorf("grok prior enabled ownership differs from its current binding")
	}
	if plugin.enabled {
		if owned == nil {
			return false, fmt.Errorf("grok plugin is already enabled without exact UAP ownership")
		}
		if err := requireEnabledMembership(original.Body, plugin.id, plugin.name); err != nil {
			return false, err
		}
		if err := recovery.verifyRecordConfig(enablementDigest(original.Body), uint32(original.Mode.Perm())); err != nil {
			return false, err
		}
		if err := recovery.PackageVerifier.Verify(ctx, plugin.root, request.Delivery.ArtifactDigest); err != nil {
			return false, err
		}
		return true, recovery.requireState(session.state)
	}
	if owned != nil {
		return false, fmt.Errorf("recorded grok enablement is no longer present; preserve the changed native preference")
	}
	edit, after, err := prepareEnablementEdit(original, plugin.id, plugin.name)
	if err != nil {
		return false, err
	}
	if err := recovery.PackageVerifier.Verify(ctx, plugin.root, request.Delivery.ArtifactDigest); err != nil {
		return false, err
	}
	before := session.binding
	before.NativeActivationAttempt = ""
	session.record = enablementRecord{Version: 2, Action: "enable", OperationID: request.NativeAttempt.OperationID,
		InstallationID: request.NativeAttempt.InstallationID, BindingID: request.NativeAttempt.BindingID,
		ProjectRoot: recovery.ProjectRoot, UserRoot: recovery.UserRoot, UserParentAuthority: session.record.UserParentAuthority,
		ArtifactPath: plugin.root, PackageDigest: request.Delivery.ArtifactDigest,
		BeforeStateDigest: enablementStateDigest(enablementState(session.state, session.index, before)), AttemptStateDigest: enablementStateDigest(session.state),
		PluginID: plugin.id, PluginName: plugin.name, Edit: edit}
	desired := before
	desired.NativeObjects = append(append([]domain.NativeObjectOwnership(nil), before.NativeObjects...), session.record.object())
	session.record.DesiredStateDigest = enablementStateDigest(enablementState(session.state, session.index, desired))
	session.after = after
	return false, recovery.requireState(session.state)
}

func requireEnabledMembership(body []byte, id, name string) error {
	if err := refuseDisabledMembership(body, id, name); err != nil {
		return err
	}
	_, lists, _ := membershipDocument(body)
	found := false
	for _, value := range lists["enabled"] {
		if value == id {
			found = true
		} else if membershipContains([]string{value}, id, name) {
			return fmt.Errorf("grok enabled membership is an ambiguous name or full-ID alias")
		}
	}
	if !found {
		return fmt.Errorf("grok native enabled readback lacks the exact owned config member")
	}
	return nil
}

func (session *enablementActivation) commit(ctx context.Context, env clients.Env, file *nativeconfig.ExactFile, before grokACPPlugin) (bool, error) {
	recovery, request := session.recovery, session.request
	if err := recovery.writeRecord(session.record); err != nil {
		return false, err
	}
	record, err := recovery.readRecord(session.record.OperationID)
	if err != nil {
		return false, err
	}
	session.record = record
	if err := record.applyConfig(file, session.after); err != nil {
		return false, errors.Join(err, record.rollbackConfig(file))
	}
	plugins, err := readEnablementInventory(ctx, env.Runner, request.BackendExecutable, recovery.ProjectRoot, env.UserHome)
	if err != nil {
		return false, err
	}
	plugin, err := selectEnablementPlugin(plugins, before.root, before.name, before.version)
	if err != nil || !plugin.enabled || plugin.trusted != before.trusted {
		return false, errors.Join(err, fmt.Errorf("grok native readback did not confirm the exact enablement without a trust change"))
	}
	if err := recovery.verifyRecordConfig(record.Edit.AfterDigest, record.Edit.Mode); err != nil {
		return false, err
	}
	if err := recovery.PackageVerifier.Verify(ctx, record.ArtifactPath, record.PackageDigest); err != nil {
		return false, err
	}
	if err := recovery.requireState(session.state); err != nil {
		return false, err
	}
	target := session.binding
	target.NativeActivationAttempt = ""
	target.NativeObjects = append(append([]domain.NativeObjectOwnership(nil), target.NativeObjects...), record.object())
	desired := enablementState(session.state, session.index, target)
	if err := record.revalidateParent(); err != nil {
		return false, err
	}
	disposition, err := recovery.State.PersistStateDecisionWithDisposition(session.state, desired)
	if err != nil || disposition != domain.StateDecisionDesired {
		return false, errors.Join(err, fmt.Errorf("grok enabled ownership decision is not durable"))
	}
	return true, recovery.removeRecord(record.OperationID, record.rawDigest)
}
