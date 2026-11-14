package grok

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"

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
	if err := record.revalidateParent(); err != nil {
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
	digest := enablementStateDigest(state)
	if digest == record.targetStateDigest() && binding.NativeActivationAttempt == "" && owned == (record.Action == "enable") {
		return recovery.finishRecord(state, state, record)
	}
	if digest == record.BeforeStateDigest && binding.NativeActivationAttempt == "" && owned == (record.Action == "remove") && len(record.footer) == 0 {
		return recovery.finishPredecessor(state, record)
	}
	if digest != record.AttemptStateDigest || binding.NativeActivationAttempt != record.OperationID || owned != (record.Action == "remove") {
		return fmt.Errorf("grok enablement complete state authority changed")
	}
	binding.NativeActivationAttempt = ""
	desired := enablementState(state, index, binding)
	if enablementStateDigest(desired) != record.BeforeStateDigest {
		return fmt.Errorf("grok enablement pending state or predecessor differs")
	}
	return recovery.rollbackRecord(record, state, desired)
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

func (recovery EnablementRecovery) finishPredecessor(state domain.StateFileV2, record enablementRecord) error {
	if err := record.revalidateParent(); err != nil {
		return err
	}
	current, err := recovery.Kernel.ReadExactFile(filepath.Join(record.UserRoot, "config.toml"))
	if err != nil || !current.Exists || current.Mode != os.FileMode(record.Edit.Mode) || enablementDigest(current.Body) != record.Edit.BeforeDigest {
		return errors.Join(err, fmt.Errorf("grok enablement without an attempt has an unexpected config"))
	}
	return recovery.finishRecord(state, state, record)
}

func (recovery EnablementRecovery) finishRecord(before, desired domain.StateFileV2, record enablementRecord) error {
	if err := recovery.requireState(before); err != nil {
		return err
	}
	if err := record.revalidateParent(); err != nil {
		return err
	}
	// Also re-save an already visible decision: a previous state rename may
	// have succeeded while its directory durability confirmation failed.
	disposition, err := recovery.State.PersistStateDecisionWithDisposition(before, desired)
	if err != nil || disposition != domain.StateDecisionDesired {
		return errors.Join(err, fmt.Errorf("grok enablement state decision is not durable"))
	}
	return recovery.removeRecord(record.OperationID, record.rawDigest)
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

func (recovery EnablementRecovery) rollbackRecord(record enablementRecord, before, desired domain.StateFileV2) (resultErr error) {
	if err := record.revalidateParent(); err != nil {
		return err
	}
	release, err := lockNativeEnablement(record.UserRoot)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, release()) }()
	if err := record.revalidateParent(); err != nil {
		return err
	}
	file, err := recovery.Kernel.BeginExactFile(filepath.Join(record.UserRoot, "config.toml"))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	current := file.Original()
	predecessor := current.Body
	if !current.Exists || current.Mode != os.FileMode(record.Edit.Mode) || enablementDigest(current.Body) != record.Edit.BeforeDigest {
		predecessor, err = record.Edit.undo(current, record.PluginID)
		if err != nil {
			return err
		}
	}
	// A same-byte write makes the predecessor durable after an earlier restore
	// became visible but its directory sync failed. The ordinary CAS still applies.
	if err := recovery.requireState(before); err != nil {
		return err
	}
	if err := record.applyConfig(file, predecessor); err != nil {
		return err
	}
	record, err = recovery.clearDecision(record)
	if err != nil {
		return err
	}
	return recovery.finishRecord(before, desired, record)
}
