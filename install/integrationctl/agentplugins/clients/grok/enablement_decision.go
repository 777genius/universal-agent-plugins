package grok

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var _ ports.NativeRemovalDecisionRecorder = EnablementRecovery{}

const enablementDecisionPrefix = "\nstate:sha256:"
const enablementDecisionLength = len(enablementDecisionPrefix) + 64 + 1

func (record enablementRecord) validateAction() error {
	if record.BeforeStateDigest == record.AttemptStateDigest {
		return fmt.Errorf("grok native attempt does not change recorded state")
	}
	if record.Action == "enable" && record.PreviousObject == nil && record.DesiredStateDigest != "" &&
		record.Edit.Added != "" && record.Edit.Removed == "" {
		return nil
	}
	if record.Action != "remove" || record.PreviousObject == nil || record.DesiredStateDigest != "" || record.Edit.Added != "" {
		return fmt.Errorf("grok native operation direction is invalid")
	}
	previous := *record.PreviousObject
	expected := record
	expected.PreviousObject = nil
	expected.Edit.BeforeDigest, expected.Edit.AfterDigest = previous.BeforeDigest, previous.ManagedDigest
	if !enablementHash(previous.BeforeDigest) || !enablementHash(previous.ManagedDigest) || !reflect.DeepEqual(previous, expected.object()) {
		return fmt.Errorf("grok removal does not carry exact profile-qualified ownership")
	}
	return nil
}

// A footer is an optional, finite decision for removal. Partial bytes carry no
// commit authority; only the exact pending state may restore the predecessor.
func (record *enablementRecord) decodeDecision() error {
	if len(record.footer) == 0 {
		return nil
	}
	if record.Action != "remove" || len(record.footer) > enablementDecisionLength {
		return fmt.Errorf("grok native decision footer is not bounded")
	}
	for index, character := range record.footer {
		switch {
		case index < len(enablementDecisionPrefix):
			if character != enablementDecisionPrefix[index] {
				return fmt.Errorf("grok native decision prefix differs")
			}
		case index == enablementDecisionLength-1:
			if character != '\n' {
				return fmt.Errorf("grok native decision terminator differs")
			}
		default:
			if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
				return fmt.Errorf("grok native decision digest is invalid")
			}
		}
	}
	if len(record.footer) == enablementDecisionLength {
		record.decisionDigest = "sha256:" + string(record.footer[len(enablementDecisionPrefix):len(record.footer)-1])
	}
	return nil
}

func (record enablementRecord) targetStateDigest() string {
	if record.Action == "remove" {
		return record.decisionDigest
	}
	return record.DesiredStateDigest
}

// PrepareNativeRemovalDecision observes the exact state already constructed by
// the ordinary removal core. It neither mutates that state nor commits it.
func (recovery EnablementRecovery) PrepareNativeRemovalDecision(ctx context.Context, installationID, bindingID string, before, desired domain.StateFileV2) (bool, error) {
	if !hasProjectEnablementBinding(before, installationID, bindingID) {
		return false, nil
	}
	_, _, binding, err := enablementBinding(before, installationID, bindingID)
	if err != nil {
		return true, err
	}
	if binding.NativeActivationAttempt == "" {
		return false, nil
	}
	ordinary, err := recovery.nativeRemovalMayBeOrdinary(binding)
	if err != nil {
		return true, err
	}
	record, err := recovery.readRecord(binding.NativeActivationAttempt)
	if ordinary && errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	if record.Action != "remove" || record.InstallationID != installationID || record.BindingID != bindingID ||
		enablementStateDigest(before) != record.AttemptStateDigest {
		return true, fmt.Errorf("grok removal decision differs from its pending authority")
	}
	if err := validateRemovalDesired(before, desired, record); err != nil {
		return true, err
	}
	return true, recovery.recordRemovalDecision(ctx, record, before, desired)
}

func (recovery EnablementRecovery) recordRemovalDecision(ctx context.Context, record enablementRecord, before, desired domain.StateFileV2) (resultErr error) {
	recovery = recovery.withRecoveryContext(ctx)
	profile, err := recovery.openRecordedProfile(ctx, record)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, profile.close(), ctx.Err()) }()
	if err := record.requireConfig(profile.config.Original(), record.Edit.AfterDigest); err != nil {
		return err
	}
	if err := recovery.requireRecoveryAuthority(ctx, before, record, record.Edit.AfterDigest); err != nil {
		return err
	}
	return recovery.appendDecision(record, enablementStateDigest(desired))
}

// A missing record means ordinary removal only when no enablement member was
// owned and the operation journal has no pending native transition. Keep these
// checks together so the fail-closed absence rule is shared by every caller.
func (recovery EnablementRecovery) nativeRemovalMayBeOrdinary(binding domain.ClientBinding) (bool, error) {
	if hasEnabledOwnership(binding) {
		return false, nil
	}
	if _, err := recovery.recordRoot(binding.NativeActivationAttempt); err != nil {
		return false, err
	}
	names, err := recovery.pendingNames()
	if err != nil {
		return false, err
	}
	return len(names) == 0, nil
}

func hasProjectEnablementBinding(state domain.StateFileV2, installationID, bindingID string) bool {
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		binding, found := installation.Clients[bindingID]
		if found && binding.ClientID == "grok" && binding.Scope == "project" {
			return true
		}
	}
	return false
}

func hasEnabledOwnership(binding domain.ClientBinding) bool {
	for _, object := range binding.NativeObjects {
		if object.Kind == enabledMemberKind {
			return true
		}
	}
	return false
}

func validateRemovalDesired(before, desired domain.StateFileV2, record enablementRecord) error {
	index, installation, binding, err := enablementBinding(before, record.InstallationID, record.BindingID)
	if err != nil {
		return err
	}
	_, targetInstallation, target, err := enablementBinding(desired, record.InstallationID, record.BindingID)
	if err != nil || target.UpdatedAt == "" || targetInstallation.UpdatedAt != target.UpdatedAt {
		return errors.Join(err, fmt.Errorf("grok removal decision lacks exact transition metadata"))
	}
	binding.NativeActivationAttempt, binding.PendingNativeIntent = "", nil
	binding.NativeObjects = nil
	for _, object := range installation.Clients[record.BindingID].NativeObjects {
		if object.Kind == "managed_package_directory" {
			binding.NativeObjects = append(binding.NativeObjects, object)
		}
	}
	binding.LocalEntryObservation = nil
	binding.Activation, binding.Verification, binding.UpdatedAt = domain.ActivationNotRequired, domain.VerificationPackageValid, target.UpdatedAt
	expected := enablementState(before, index, binding)
	expected.Installations[index].UpdatedAt = target.UpdatedAt
	if enablementStateDigest(expected) != enablementStateDigest(desired) {
		return fmt.Errorf("grok removal decision changes unrelated persisted authority")
	}
	return nil
}

func decisionTail(record enablementRecord, digest string) ([]byte, error) {
	if record.Action != "remove" || !enablementHash(digest) {
		return nil, fmt.Errorf("grok removal target decision is invalid")
	}
	want := []byte("\nstate:" + digest + "\n")
	if !bytes.HasPrefix(want, record.footer) {
		return nil, fmt.Errorf("grok native decision retry differs from its exact prefix")
	}
	return want, nil
}
