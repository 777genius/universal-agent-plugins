package grok

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func enablementState(state domain.StateFileV2, index int, binding domain.ClientBinding) domain.StateFileV2 {
	desired := state
	desired.Installations = append([]domain.Installation(nil), state.Installations...)
	installation := state.Installations[index]
	installation.Clients = maps.Clone(installation.Clients)
	installation.Clients[binding.ClientBindingID] = binding
	desired.Installations[index] = installation
	return desired
}

func enablementPredecessor(binding domain.ClientBinding, record enablementRecord) (domain.ClientBinding, bool, error) {
	if binding.NativeActivationAttempt != "" && binding.NativeActivationAttempt != record.OperationID {
		return binding, false, fmt.Errorf("grok enablement operation identity changed")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(binding.NativeObjects))
	committed := false
	for _, object := range binding.NativeObjects {
		if object.Kind == enabledMemberKind && object.ObjectID == record.object().ObjectID {
			if committed || !reflect.DeepEqual(object, record.object()) {
				return binding, false, fmt.Errorf("grok enabled member ownership differs")
			}
			committed = true
			continue
		}
		objects = append(objects, object)
	}
	binding.NativeObjects = objects
	return binding, committed, nil
}

func (recovery EnablementRecovery) recoverRecord(ctx context.Context, operationID string) error {
	record, err := recovery.readRecord(operationID)
	if err != nil {
		return err
	}
	return recovery.recoverHeldRecord(ctx, record)
}

// The admitted record remains the authority through every reread. Both the
// original native lock and ExactFile stay held through completion and rollback.
func (recovery EnablementRecovery) recoverHeldRecord(ctx context.Context, record enablementRecord) (resultErr error) {
	if recovery.State == nil || recovery.PackageVerifier == nil {
		return fmt.Errorf("grok enablement recovery services are unavailable")
	}
	recovery = recovery.withRecoveryContext(ctx)
	if err := directoryidentity.Revalidate(ctx, record.UserParentAuthority); err != nil {
		return err
	}
	state, err := recovery.State.Load()
	if err != nil {
		return err
	}
	index, binding, owned, err := recovery.recoveryBinding(ctx, state, record)
	if err != nil {
		return err
	}
	desired, configDigest, err := record.recoveryTarget(state, index, binding, owned)
	if err != nil {
		return err
	}
	profile, err := recovery.openRecordedProfile(ctx, record)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, profile.close(), ctx.Err()) }()
	if err := recovery.requireState(state); err != nil {
		return err
	}
	if enablementStateDigest(desired) == enablementStateDigest(state) {
		return recovery.finishCompleteRecord(ctx, profile, state, record, configDigest)
	}
	return recovery.rollbackHeldRecord(ctx, profile, record, state, desired)
}

// Classify the whole state before opening config. Selected-binding equality
// alone cannot admit unrelated state drift into the recorded recovery lease.
func (record enablementRecord) recoveryTarget(state domain.StateFileV2, index int, binding domain.ClientBinding, owned bool) (domain.StateFileV2, string, error) {
	digest := enablementStateDigest(state)
	if digest == record.targetStateDigest() && binding.NativeActivationAttempt == "" && owned == (record.Action == "enable") {
		return state, record.Edit.AfterDigest, nil
	}
	if digest == record.BeforeStateDigest && binding.NativeActivationAttempt == "" && owned == (record.Action == "remove") && len(record.footer) == 0 {
		return state, record.Edit.BeforeDigest, nil
	}
	if digest != record.AttemptStateDigest || binding.NativeActivationAttempt != record.OperationID || owned != (record.Action == "remove") {
		return domain.StateFileV2{}, "", fmt.Errorf("grok enablement complete state authority changed")
	}
	binding.NativeActivationAttempt = ""
	desired := enablementState(state, index, binding)
	if enablementStateDigest(desired) != record.BeforeStateDigest {
		return domain.StateFileV2{}, "", fmt.Errorf("grok enablement pending state or predecessor differs")
	}
	return desired, record.Edit.BeforeDigest, nil
}

// Use the existing bounded journal checkpoints without changing their owner or
// renewing the caller's deadline. A copy confines this context to one operation.
func (recovery EnablementRecovery) withRecoveryContext(ctx context.Context) EnablementRecovery {
	original := recovery.journalPoint
	recovery.journalPoint = func(point string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		if original != nil {
			err = original(point)
		}
		return errors.Join(err, ctx.Err())
	}
	return recovery
}

func (recovery EnablementRecovery) recoveryBinding(ctx context.Context, state domain.StateFileV2, record enablementRecord) (int, domain.ClientBinding, bool, error) {
	index, binding, owned, err := recovery.recordBinding(state, record)
	if err != nil {
		return index, binding, owned, err
	}
	if err := recovery.PackageVerifier.Verify(ctx, record.ArtifactPath, record.PackageDigest); err != nil {
		return index, binding, false, err
	}
	return index, binding, owned, nil
}

func (recovery EnablementRecovery) recordBinding(state domain.StateFileV2, record enablementRecord) (int, domain.ClientBinding, bool, error) {
	index, installation, binding, err := enablementBinding(state, record.InstallationID, record.BindingID)
	if err != nil {
		return index, binding, false, err
	}
	baseline, owned, err := enablementPredecessor(binding, record)
	if err != nil {
		return index, binding, false, err
	}
	if binding.TargetLocator != record.ArtifactPath || binding.NativeProfileRoot != filepath.Join(record.ProjectRoot, ".grok") ||
		binding.PhysicalArtifact != domain.ComputePhysicalArtifactID(installation.DeclaredName, installation.InstallationID) ||
		record.PluginName != installation.DeclaredName || shared.ManagedPackageDigest(baseline) != record.PackageDigest {
		return index, binding, false, fmt.Errorf("grok enablement package or project root changed")
	}
	if err := New().ValidateBindingProfile(binding.NativeProfileRoot, baseline); err != nil {
		return index, binding, false, err
	}
	return index, binding, owned, nil
}

func (recovery EnablementRecovery) finishCompleteRecord(ctx context.Context, profile *enablementProfile, state domain.StateFileV2, record enablementRecord, digest string) error {
	if err := record.requireConfig(profile.config.Original(), digest); err != nil {
		return err
	}
	return recovery.finishRecord(ctx, state, state, record, digest)
}

func (recovery EnablementRecovery) finishRecord(ctx context.Context, before, desired domain.StateFileV2, record enablementRecord, configDigest string) error {
	if err := recovery.requireRecoveryAuthority(ctx, before, record, configDigest); err != nil {
		return err
	}
	// Also re-save an already visible decision: a previous state rename may
	// have succeeded while its directory durability confirmation failed.
	disposition, err := recovery.State.PersistStateDecisionWithDisposition(before, desired)
	if err != nil || disposition != domain.StateDecisionDesired {
		return errors.Join(err, fmt.Errorf("grok enablement state decision is not durable"))
	}
	if err := recovery.requireRecoveryAuthority(ctx, desired, record, configDigest); err != nil {
		return err
	}
	return recovery.removeRecord(record.OperationID, record.rawDigest)
}

func (record enablementRecord) requireConfig(current nativeconfig.FileSnapshot, digest string) error {
	if !current.Exists || current.Mode != os.FileMode(record.Edit.Mode) || enablementDigest(current.Body) != digest {
		return errors.Join(nativeconfig.ErrConcurrentChange, fmt.Errorf("grok recorded config image differs"))
	}
	return nil
}

// The ordinary mutation owner supplies state serialization; the native and
// ExactFile locks remain held by our caller. These are repeated observations,
// not atomic exclusion of arbitrary writers after the last check.
func (recovery EnablementRecovery) requireRecoveryAuthority(ctx context.Context, state domain.StateFileV2, record enablementRecord, configDigest string) error {
	if err := directoryidentity.Revalidate(ctx, record.UserParentAuthority); err != nil {
		return err
	}
	currentRecord, err := recovery.readRecord(record.OperationID)
	if err != nil || record.rawDigest == "" || currentRecord.rawDigest != record.rawDigest || currentRecord.Hash != record.Hash {
		return errors.Join(err, fmt.Errorf("grok observed recovery record changed"))
	}
	if err := recovery.requireState(state); err != nil {
		return err
	}
	if err := recovery.PackageVerifier.Verify(ctx, record.ArtifactPath, record.PackageDigest); err != nil {
		return err
	}
	if err := directoryidentity.Revalidate(ctx, record.UserParentAuthority); err != nil {
		return err
	}
	current, err := recovery.Kernel.ReadExactFile(filepath.Join(record.UserRoot, "config.toml"))
	if err != nil {
		return err
	}
	if err := record.requireConfig(current, configDigest); err != nil {
		return err
	}
	return directoryidentity.Revalidate(ctx, record.UserParentAuthority)
}

func (recovery EnablementRecovery) requireState(expected domain.StateFileV2) error {
	observed, err := recovery.State.Load()
	if err != nil {
		return err
	}
	if enablementStateDigest(observed) != enablementStateDigest(expected) {
		return fmt.Errorf("grok native transition complete state changed before its effect")
	}
	return nil
}

func (recovery EnablementRecovery) rollbackHeldRecord(ctx context.Context, profile *enablementProfile, record enablementRecord, before, desired domain.StateFileV2) error {
	current := profile.config.Original()
	predecessor := current.Body
	var err error
	if record.requireConfig(current, record.Edit.BeforeDigest) != nil {
		predecessor, err = record.Edit.undo(current, record.PluginID)
		if err != nil {
			return err
		}
	}
	// A same-byte write makes the predecessor durable after an earlier restore
	// became visible but its directory sync failed. The ordinary CAS still applies.
	if err := recovery.requireRecoveryAuthority(ctx, before, record, enablementDigest(current.Body)); err != nil {
		return err
	}
	if err := record.applyConfig(profile.config, predecessor); err != nil {
		return err
	}
	if err := recovery.requireRecoveryAuthority(ctx, before, record, record.Edit.BeforeDigest); err != nil {
		return err
	}
	record, err = recovery.clearDecision(record)
	if err != nil {
		return err
	}
	return recovery.finishRecord(ctx, before, desired, record, record.Edit.BeforeDigest)
}
