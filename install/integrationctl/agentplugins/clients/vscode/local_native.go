package vscode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

type RegistrationState string

const (
	RegistrationActive   RegistrationState = "owned_registered_activation_unknown"
	RegistrationDisabled RegistrationState = "owned_disabled"
	RegistrationMissing  RegistrationState = "missing"
	RegistrationConflict RegistrationState = "conflict"
)

func profileIdentity(f domain.LocalDeliveryFacts) vscodeprofile.Identity {
	projection := f.ProjectionDigest
	if projection == "" {
		projection = f.CanonicalDigest
	} // Pure preflight only; effects require sealed projection.
	return vscodeprofile.Identity{SettingsPath: f.SettingsPath, ProfileID: f.ProfileIdentity, PluginRoot: f.Registration.Selector, PackageID: f.Registration.ObjectID, PackageDigest: f.CanonicalDigest, ProjectionDigest: projection}
}

// receiptForRecordedEntry adapts validated persisted bool/selector authority to
// the public planner's receipt. The pure Install runs on an EMPTY synthetic
// document, never on native bytes; it cannot adopt an existing foreign entry.
func receiptForRecordedEntry(f domain.LocalDeliveryFacts) (*vscodeprofile.Receipt, error) {
	id := profileIdentity(f)
	result, err := vscodeprofile.Plan(vscodeprofile.Request{Settings: []byte("{}"), Identity: id, Action: vscodeprofile.Install})
	if err != nil {
		return nil, err
	}
	if *f.Registration.DesiredValue {
		return result.Receipt, nil
	}
	key, _ := json.Marshal(id.PluginRoot)
	disabled := []byte(`{"chat.pluginLocations":{` + string(key) + `:false}}`)
	result, err = vscodeprofile.Plan(vscodeprofile.Request{Settings: disabled, Identity: id, Action: vscodeprofile.Install, Previous: result.Receipt})
	return result.Receipt, err
}

func inspectRegistration(kernel nativeconfig.Kernel, facts domain.LocalDeliveryFacts, owned bool) (RegistrationState, error) {
	snapshot, err := kernel.ReadExactFile(facts.SettingsPath)
	if err != nil {
		return RegistrationConflict, err
	}
	return observeRegistration(snapshot.Body, facts, owned)
}

func observeRegistration(body []byte, facts domain.LocalDeliveryFacts, owned bool) (RegistrationState, error) {
	id := profileIdentity(facts)
	if !owned {
		_, err := vscodeprofile.Plan(vscodeprofile.Request{Settings: body, Identity: id, Action: vscodeprofile.Install})
		if err != nil {
			return RegistrationConflict, err
		}
		return RegistrationMissing, nil
	}
	receipt, err := receiptForRecordedEntry(facts)
	if err != nil {
		return RegistrationConflict, err
	}
	verified, err := vscodeprofile.VerifyOwned(body, id, receipt)
	if err == nil {
		if verified.Disabled {
			return RegistrationDisabled, nil
		}
		return RegistrationActive, nil
	}
	removed, removeErr := vscodeprofile.Plan(vscodeprofile.Request{Settings: body, Identity: id, Action: vscodeprofile.Remove, Previous: receipt})
	if removeErr == nil && !removed.Changed {
		return RegistrationMissing, nil
	}
	return RegistrationConflict, err
}

// InspectRegistration distinguishes native bytes without locks or CLI listing.
// RegistrationActive is explicitly not proof of live Local execution.
func (*LocalAdapter) InspectRegistration(ctx context.Context, kernel nativeconfig.Kernel, selected domain.SelectedDelivery, objects []domain.NativeObjectOwnership) (RegistrationState, error) {
	if err := ctx.Err(); err != nil {
		return RegistrationConflict, err
	}
	facts, err := recordedLocalFacts(selected)
	if err != nil {
		return RegistrationConflict, err
	}
	owned, err := ownedLocalObjects(facts, objects)
	if err != nil {
		return RegistrationConflict, err
	}
	return inspectRegistration(kernel, facts, owned)
}

func (*LocalAdapter) UsesNativeRegistryExecutable() bool { return false }

func (a *LocalAdapter) InspectNativeRegistry(ctx context.Context, env clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	selected, objects := plan.SelectedDelivery, []domain.NativeObjectOwnership(nil)
	if managed != nil {
		if !managed.SelectedDelivery.SameProfile(selected) {
			return clients.RegistryCollision, fmt.Errorf("local recorded profile differs")
		}
		selected, objects = managed.SelectedDelivery, managed.NativeObjects
	}
	state, err := a.InspectRegistration(ctx, env.NativeConfig, selected, objects)
	if err != nil {
		return clients.RegistryCollision, err
	}
	switch state {
	case RegistrationActive, RegistrationDisabled:
		return clients.RegistryExpected, nil
	case RegistrationMissing:
		return clients.RegistryClear, nil
	default:
		return clients.RegistryCollision, nil
	}
}

func errorsJoinClose(operation, closeErr error) error { return errors.Join(operation, closeErr) }

func mutateProfile(ctx context.Context, kernel nativeconfig.Kernel, facts domain.LocalDeliveryFacts, action vscodeprofile.Action, owned bool) (state RegistrationState, effect domain.NativeEffectState, resultErr error) {
	effect = domain.NativeEffectUnchanged
	if err := ctx.Err(); err != nil {
		return RegistrationConflict, effect, err
	}
	if facts.ProjectionDigest == "" {
		return RegistrationConflict, effect, fmt.Errorf("local effect requires sealed projection identity")
	}
	file, err := kernel.BeginExactFile(facts.SettingsPath)
	if err != nil {
		return RegistrationConflict, effect, err
	}
	defer func() {
		closeErr := file.Close()
		if closeErr != nil && effect == domain.NativeEffectCommitted {
			closeErr = &nativeconfig.CommittedCleanupError{Err: closeErr}
		}
		resultErr = errorsJoinClose(resultErr, closeErr)
	}()
	result, err := planNativeProfile(file.Original().Body, facts, action, owned)
	if err != nil {
		return RegistrationConflict, effect, err
	}
	state = RegistrationActive
	if action == vscodeprofile.Remove {
		state = RegistrationMissing
	} else if result.Disabled {
		state = RegistrationDisabled
	}
	if !result.Changed {
		return state, effect, nil
	}
	if err := localWritableMetadata(facts.SettingsPath); err != nil {
		return state, effect, err
	}
	if err := ctx.Err(); err != nil {
		return state, effect, err
	}
	if err := file.Apply(result.Settings); err != nil {
		rollbackErr := file.Rollback()
		if file.Effect() != nativeconfig.FileUnchanged {
			effect = domain.NativeEffectUncertain
		}
		return state, effect, errorsJoinClose(err, rollbackErr)
	}
	return state, domain.NativeEffectCommitted, nil
}

func planNativeProfile(body []byte, facts domain.LocalDeliveryFacts, action vscodeprofile.Action, owned bool) (vscodeprofile.Result, error) {
	// 040 freezes desired bool but cannot persist the observed false receipt.
	// An absent formerly owned true is therefore ambiguous (native false may
	// have disappeared). Refuse rather than silently re-enable it. The lifecycle
	// owner must supply durable observed-bool authority before additive repair.
	if owned && action == vscodeprofile.Repair && *facts.Registration.DesiredValue {
		observed, observeErr := observeRegistration(body, facts, true)
		if observeErr != nil {
			return vscodeprofile.Result{}, observeErr
		}
		if observed == RegistrationMissing {
			return vscodeprofile.Result{}, fmt.Errorf("local absent repair requires persisted observed boolean receipt; 040 contract cannot establish it")
		}
	}
	var receipt *vscodeprofile.Receipt
	if owned {
		var err error
		receipt, err = receiptForRecordedEntry(facts)
		if err != nil {
			return vscodeprofile.Result{}, err
		}
	}
	return vscodeprofile.Plan(vscodeprofile.Request{Settings: body, Identity: profileIdentity(facts), Action: action, Previous: receipt})
}
