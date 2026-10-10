package vscode

import (
	"context"
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

// Historical nil can inspect actual present bytes with independent ownership;
// absence yields no receipt. This public compatibility path never persists one.
func inspectRegistration(kernel nativeconfig.Kernel, facts domain.LocalDeliveryFacts, owned bool) (RegistrationState, error) {
	snapshot, err := kernel.ReadExactFile(facts.SettingsPath)
	if err != nil {
		return RegistrationConflict, err
	}
	return observeRegistration(snapshot.Body, facts, owned)
}

func observeRegistration(body []byte, facts domain.LocalDeliveryFacts, owned bool) (RegistrationState, error) {
	if !owned {
		_, err := vscodeprofile.Plan(vscodeprofile.Request{Settings: body, Identity: profileIdentity(facts), Action: vscodeprofile.Install})
		if err != nil {
			return RegistrationConflict, err
		}
		return RegistrationMissing, nil
	}
	result, err := vscodeprofile.VerifyRecordedEntry(body, profileIdentity(facts), *facts.Registration.DesiredValue)
	if errors.Is(err, vscodeprofile.ErrRecordedEntryAbsent) {
		return RegistrationMissing, nil
	}
	if err != nil {
		return RegistrationConflict, err
	}
	return registrationResult(result), nil
}

func inspectObservedRegistration(kernel nativeconfig.Kernel, selected domain.SelectedDelivery, objects []domain.NativeObjectOwnership, observation *domain.LocalEntryObservation) (vscodeprofile.Result, error) {
	receipt, err := localRecordedReceipt(selected, objects, observation)
	if err != nil {
		return vscodeprofile.Result{}, err
	}
	facts, _ := selected.LocalFacts()
	owned, err := ownedLocalObjects(facts, objects)
	if err != nil {
		return vscodeprofile.Result{}, err
	}
	if !owned {
		return vscodeprofile.Result{}, fmt.Errorf("local present verification requires owned selector")
	}
	id, enabled := profileIdentity(facts), *facts.Registration.DesiredValue
	if receipt != nil {
		id, enabled = receipt.Identity, receipt.Enabled
	}
	snapshot, err := kernel.ReadExactFile(facts.SettingsPath)
	if err != nil {
		return vscodeprofile.Result{}, err
	}
	return vscodeprofile.VerifyRecordedEntry(snapshot.Body, id, enabled)
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
		if err := managed.ValidateLocalEntryObservation(); err != nil {
			return clients.RegistryCollision, err
		}
		if managed.LocalEntryObservation != nil && managed.PendingNativeIntent == nil && !localSameBasis(managed.SelectedDelivery, managed.LocalEntryObservation) {
			return clients.RegistryCollision, fmt.Errorf("local recorded observation is stale without a pending revision transition")
		}
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

// Exact-file lock, CAS, readback, metadata refusal and own-output rollback stay
// in the existing kernel. A receipt is returned only after successful readback.
func mutateProfile(ctx context.Context, kernel nativeconfig.Kernel, selected domain.SelectedDelivery, action vscodeprofile.Action, objects []domain.NativeObjectOwnership, observation *domain.LocalEntryObservation) (result vscodeprofile.Result, effect domain.NativeEffectState, resultErr error) {
	effect = domain.NativeEffectUnchanged
	if err := ctx.Err(); err != nil {
		return result, effect, err
	}
	facts, err := recordedLocalFacts(selected)
	if err != nil {
		return result, effect, err
	}
	if facts.ProjectionDigest == "" {
		return result, effect, fmt.Errorf("local effect requires sealed projection identity")
	}
	receipt, err := localRecordedReceipt(selected, objects, observation)
	if err != nil {
		return result, effect, err
	}
	owned, err := ownedLocalObjects(facts, objects)
	if err != nil {
		return result, effect, err
	}
	file, err := beginLocalProfile(kernel, facts.SettingsPath)
	if err != nil {
		return result, effect, err
	}
	defer func() {
		closeErr := file.Close()
		if closeErr != nil && effect == domain.NativeEffectCommitted {
			closeErr = &nativeconfig.CommittedCleanupError{Err: closeErr}
		}
		resultErr = errorsJoinClose(resultErr, closeErr)
	}()
	result, err = planNativeProfile(file.Original().Body, facts, action, owned, receipt)
	if err != nil {
		return vscodeprofile.Result{}, effect, err
	}
	if !result.Changed {
		return result, effect, nil
	}
	if err := checkLocalProfileMetadata(file, facts.SettingsPath); err != nil {
		return vscodeprofile.Result{}, effect, err
	}
	if err := ctx.Err(); err != nil {
		return vscodeprofile.Result{}, effect, err
	}
	if err := file.Apply(result.Settings); err != nil {
		rollbackErr := file.Rollback()
		if file.Effect() != nativeconfig.FileUnchanged {
			effect = domain.NativeEffectUncertain
		}
		return vscodeprofile.Result{}, effect, errorsJoinClose(err, rollbackErr)
	}
	return result, domain.NativeEffectCommitted, nil
}

func planNativeProfile(body []byte, facts domain.LocalDeliveryFacts, action vscodeprofile.Action, owned bool, receipt *vscodeprofile.Receipt) (vscodeprofile.Result, error) {
	id := profileIdentity(facts)
	if owned {
		oldID, enabled := id, *facts.Registration.DesiredValue
		if receipt != nil {
			oldID, enabled = receipt.Identity, receipt.Enabled
		}
		verified, err := vscodeprofile.VerifyRecordedEntry(body, oldID, enabled)
		if err != nil {
			// Only a valid recorded receipt at this exact revision permits additive
			// Repair. Historical nil can remove an already absent selector idempotently.
			absent := errors.Is(err, vscodeprofile.ErrRecordedEntryAbsent)
			if absent && receipt == nil && action == vscodeprofile.Remove {
				return vscodeprofile.Result{}, nil
			}
			if !absent || receipt == nil || oldID != id || (action != vscodeprofile.Repair && action != vscodeprofile.Remove) {
				return vscodeprofile.Result{}, err
			}
		} else {
			receipt = verified.Receipt
			if oldID != id && action != vscodeprofile.Remove {
				action = vscodeprofile.Update
			}
		}
	}
	return vscodeprofile.Plan(vscodeprofile.Request{Settings: body, Identity: id, Action: action, Previous: receipt})
}
