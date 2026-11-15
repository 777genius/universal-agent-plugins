package grok

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var _ clients.NativeTransitionProvider = (*Adapter)(nil)
var _ ports.ObservedNativeTransitionRecovery = (*EnablementRecovery)(nil)

func (*Adapter) NewNativeTransitions(authority clients.NativeTransitionContext) ports.ObservedNativeTransitionRecovery {
	root := ""
	if authority.UserHome != "" {
		root = filepath.Join(authority.UserHome, ".grok")
	}
	return &EnablementRecovery{ProjectRoot: authority.ProjectRoot, UserRoot: root, OperationsDir: authority.OperationsDir,
		State: authority.State, Kernel: authority.Kernel, PackageVerifier: authority.PackageVerifier}
}

// Pending only reads. Unpublished or partial stage names do not become
// committed authority, and no cleanup is performed by inspection.
func (recovery EnablementRecovery) Pending() ([]ports.NativeTransitionObservation, error) {
	if recovery.State == nil {
		return nil, fmt.Errorf("grok native transition state authority is unavailable")
	}
	names, err := recovery.pendingNames()
	if err != nil {
		return nil, err
	}
	state, err := recovery.State.Load()
	if err != nil {
		return nil, err
	}
	attempts := map[string]ports.NativeTransitionObservation{}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.ClientID == "grok" && binding.Scope == "project" && binding.NativeActivationAttempt != "" {
				if _, found := attempts[binding.NativeActivationAttempt]; found {
					return nil, fmt.Errorf("duplicate grok native attempt")
				}
				attempts[binding.NativeActivationAttempt] = ports.NativeTransitionObservation{OperationID: binding.NativeActivationAttempt,
					InstallationID: installation.InstallationID, BindingID: binding.ClientBindingID, TargetPath: binding.TargetLocator, Phase: "native_attempt"}
			}
		}
	}
	var pending []ports.NativeTransitionObservation
	for _, name := range names {
		item, err := recovery.pendingRecord(state, strings.TrimPrefix(name, enablementRecordPrefix))
		if err != nil {
			return nil, err
		}
		delete(attempts, item.OperationID)
		pending = append(pending, item)
	}
	for _, attempt := range attempts {
		pending = append(pending, attempt)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].OperationID < pending[j].OperationID })
	return pending, nil
}

func (recovery EnablementRecovery) pendingRecord(state domain.StateFileV2, operationID string) (ports.NativeTransitionObservation, error) {
	record, err := recovery.readRecord(operationID)
	if err != nil {
		return ports.NativeTransitionObservation{}, err
	}
	return recovery.recordObservation(state, record)
}

func (recovery EnablementRecovery) recordObservation(state domain.StateFileV2, record enablementRecord) (ports.NativeTransitionObservation, error) {
	_, _, _, err := recovery.recordBinding(state, record)
	if err != nil {
		return ports.NativeTransitionObservation{}, err
	}
	digest := enablementStateDigest(state)
	if digest != record.BeforeStateDigest && digest != record.AttemptStateDigest && digest != record.targetStateDigest() ||
		len(record.footer) != 0 && record.decisionDigest == "" && digest != record.AttemptStateDigest {
		return ports.NativeTransitionObservation{}, fmt.Errorf("grok native record differs from the complete observed state")
	}
	root, err := recovery.recordRoot(record.OperationID)
	if err != nil {
		return ports.NativeTransitionObservation{}, err
	}
	phase := "native_prepared"
	if record.decisionDigest != "" || digest == record.targetStateDigest() {
		phase = "native_state_decision"
	}
	return ports.NativeTransitionObservation{OperationID: record.OperationID, InstallationID: record.InstallationID,
		BindingID: record.BindingID, TargetPath: record.ArtifactPath, Root: root, Phase: phase,
		Digest: record.rawDigest, AuthorityDigest: record.Hash}, nil
}

// This private codec path does not qualify a v3 producer. Public recovery stays
// closed until the readonly transport and original-parent contract are proven.
// Never replace the supplied observation with a later operation-ID-only lookup.
func (recovery EnablementRecovery) recoverObservedRecord(ctx context.Context, observed ports.NativeTransitionObservation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if recovery.State == nil || recovery.PackageVerifier == nil {
		return fmt.Errorf("grok enablement recovery services are unavailable")
	}
	if observed.Root == "" {
		return errEnablementUnqualified
	}
	record, err := recovery.readRecord(observed.OperationID)
	if err != nil {
		return err
	}
	if record.Version != 3 {
		return errEnablementUnqualified
	}
	state, err := recovery.State.Load()
	if err != nil {
		return err
	}
	current, err := recovery.recordObservation(state, record)
	if err != nil {
		return err
	}
	current.EnvironmentID = observed.EnvironmentID
	if !reflect.DeepEqual(current, observed) {
		return fmt.Errorf("grok native record no longer matches the observed authority")
	}
	return recovery.recoverHeldRecord(ctx, record)
}

func (recovery EnablementRecovery) ReconcileObserved(ctx context.Context, observed ports.NativeTransitionObservation, _ ports.NativeTransitionState) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pending, err := recovery.Pending()
	if err != nil {
		return err
	}
	for _, item := range pending {
		item.EnvironmentID = observed.EnvironmentID
		if item.Root != "" && reflect.DeepEqual(item, observed) {
			return errEnablementUnqualified
		}
	}
	return fmt.Errorf("grok native record no longer matches the observed authority")
}
