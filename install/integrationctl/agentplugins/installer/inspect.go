package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// Inspect is read-only. It does not recover journals or invoke host seams.
func (e *Engine) Inspect(ctx context.Context) (Inspection, error) {
	out, err := e.observe()
	if err != nil {
		return out, err
	}
	stager := e.lifecycle(nil, BindingFacts{}, nil).Stager
	for i := range out.Installations {
		for j := range out.Installations[i].Bindings {
			b := &out.Installations[i].Bindings[j]
			if b.ProjectionDigest == "" {
				b.ProjectionStatus = "projection_missing"
				continue
			}
			if _, err := os.Stat(b.TargetPath); os.IsNotExist(err) {
				b.ProjectionStatus = "projection_missing"
			} else if err := stager.Verify(ctx, b.TargetPath, b.ProjectionDigest); err != nil {
				b.ProjectionStatus = "projection_mismatch"
			}
		}
	}
	return out, nil
}

func (e *Engine) observe() (Inspection, error) {
	out := Inspection{StateRoot: e.cfg.StateRoot}
	if err := e.CheckProjectNamespace(); err != nil {
		return out, err
	}
	state, err := e.store.Load()
	if err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	out.Recovery.StateDigest, err = recoveryStateDigest(state)
	if err != nil {
		return out, err
	}
	installations, bindingIndex := inspectInstallations(state)
	out.Installations = installations
	out.EnvironmentReceipts = append([]domain.EnvironmentReceipt(nil), state.EnvironmentReceipts...)
	out.Recovery.NativeIntents, err = inspectNativeIntents(state)
	if err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	open, err := dirswap.Manager{Namespace: e.cfg.StateRoot, JournalDir: e.cfg.OperationsDir}.ListOpen()
	if err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	openIDs := make(map[string]dirswap.Receipt, len(open))
	for _, journal := range open {
		openIDs[journal.OperationID] = journal
		located := bindingIndex[journal.ClientBindingID]
		out.Recovery.Journals = append(out.Recovery.Journals, PendingJournal{
			EnvironmentID:  journal.EnvironmentID,
			OperationID:    journal.OperationID,
			Digest:         journalDigest(journal),
			BindingID:      journal.ClientBindingID,
			InstallationID: located.InstallationID,
			TargetPath:     firstNonEmpty(journal.ActivePath, located.TargetPath),
			Phase:          journal.Phase,
		})
	}
	out.Recovery.Receipts = unfinishedReceipts(state, openIDs, bindingIndex)
	if err := e.appendPendingTransitions(&out); err != nil {
		out.Recovery.Required = true
		out.Recovery.Reason = err.Error()
		return out, err
	}
	sort.Slice(out.Recovery.Journals, func(i, j int) bool {
		return out.Recovery.Journals[i].OperationID < out.Recovery.Journals[j].OperationID
	})
	out.Recovery.Required = len(out.Recovery.Journals) > 0 || len(out.Recovery.Receipts) > 0 || len(out.Recovery.NativeIntents) > 0
	return out, nil
}

func inspectInstallations(state domain.StateFileV2) ([]InspectedInstallation, map[string]observedBinding) {
	installations := make([]InspectedInstallation, 0, len(state.Installations))
	bindingIndex := map[string]observedBinding{}
	for _, installation := range state.Installations {
		item := InspectedInstallation{
			InstallationID: installation.InstallationID,
			DataRetained:   installation.DataRetained,
			TreeDigest:     installation.Source.TreeDigest,
		}
		item.SourceKey = "github:" + strings.ToLower(installation.Source.Repository) + ":" + installation.Source.PackageSubpath
		if installation.Source.StableSourceKey != "" {
			item.SourceKey = installation.Source.StableSourceKey
		} else if installation.Source.SourceKind == "git" || installation.Source.SourceKind == "archive" {
			item.SourceKey = sourcespec.Key(sourcespec.Kind(installation.Source.SourceKind), installation.Source.SourceURL, installation.Source.PackageSubpath)
		}
		if installation.Directory != nil {
			item.SourceKey = "directory:" + strings.ToLower(installation.Directory.DistributionID)
		}
		for _, binding := range installation.Clients {
			receipt := installation.DataReceipts[binding.DataReceiptID]
			digest := recordedBindingDigest(binding, installation.Source.TreeDigest)
			item.Bindings = append(item.Bindings, InspectedBinding{
				LifecycleConverged: binding.LifecycleConverged(), ClientID: binding.ClientID, BindingID: binding.ClientBindingID, Scope: binding.Scope,
				TargetPath: binding.TargetLocator, DataRoot: receipt.Locator,
				TreeDigest: digest, Materialization: string(binding.Materialization), Activation: string(binding.Activation),
				Policy: string(binding.Policy), ManagedPaths: inspectedManagedPaths(binding),
				Authentication: string(binding.Authentication), Verification: string(binding.Verification),
			})
			inspected := &item.Bindings[len(item.Bindings)-1]
			populateInspectedBinding(inspected, binding, state.EnvironmentReceipts, installation.InstallationID)
			bindingIndex[binding.ClientBindingID] = observedBinding{
				InstallationID: installation.InstallationID,
				TargetPath:     binding.TargetLocator,
			}
		}
		if installation.DataRetained {
			for _, receipt := range installation.DataReceipts {
				if receipt.Locator != "" {
					item.DataRoots = append(item.DataRoots, receipt.Locator)
				}
			}
			sort.Strings(item.DataRoots)
		}
		installations = append(installations, item)
	}
	return installations, bindingIndex
}

type observedBinding struct {
	InstallationID, TargetPath string
}

func unfinishedReceipts(state domain.StateFileV2, open map[string]dirswap.Receipt, bindings map[string]observedBinding) []PendingReceipt {
	var out []PendingReceipt
	add := func(receipt domain.MutationReceipt, installationID, targetPath string) {
		if receipt.Phase == "" || receipt.Phase == transaction.ReceiptPhaseCommitted {
			return
		}
		located := bindings[receipt.ClientBindingID]
		if installationID == "" {
			installationID = located.InstallationID
		}
		if targetPath == "" {
			targetPath = firstNonEmpty(receipt.ActivePath, located.TargetPath)
		}
		_, present := open[receipt.OperationID]
		out = append(out, PendingReceipt{
			OperationID: receipt.OperationID, BindingID: receipt.ClientBindingID,
			InstallationID: installationID, TargetPath: targetPath, Phase: receipt.Phase,
			JournalPresent: present,
		})
	}
	for _, receipt := range state.TransactionReceipts {
		add(receipt, "", "")
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			for _, receipt := range binding.Receipts {
				add(receipt, installation.InstallationID, binding.TargetLocator)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OperationID != out[j].OperationID {
			return out[i].OperationID < out[j].OperationID
		}
		return out[i].Phase < out[j].Phase
	})
	return out
}

func journalDigest(receipt dirswap.Receipt) string {
	// Receipt is a concrete JSON value with no fallible MarshalJSON method.
	raw, _ := json.Marshal(receipt)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func recoveryStateDigest(state domain.StateFileV2) (string, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func pendingIdentity(obs RecoveryObservation) []string {
	keys := make([]string, 0, len(obs.Journals)+len(obs.Receipts))
	for _, journal := range obs.Journals {
		keys = append(keys, "journal|"+journal.OperationID+"|"+journal.Phase+"|"+journal.Digest)
	}
	for _, receipt := range obs.Receipts {
		keys = append(keys, "receipt|"+receipt.OperationID+"|"+receipt.Phase+"|"+receipt.BindingID)
	}
	for _, pending := range obs.NativeIntents {
		keys = append(keys, nativeObservationIdentity(pending))
	}
	sort.Strings(keys)
	return keys
}

func leftoverSwapArtifacts(obs RecoveryObservation) bool {
	for _, journal := range obs.Journals {
		if journal.TargetPath == "" {
			return true
		}
		entries, err := os.ReadDir(filepath.Dir(journal.TargetPath))
		if err != nil {
			return true
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".agentplugins-staging-") || strings.HasPrefix(name, ".agentplugins-backup-") {
				return true
			}
		}
	}
	return false
}

func liveWithinObserved(live, observed RecoveryObservation) bool {
	if len(live.NativeIntents) != 0 || len(observed.NativeIntents) != 0 {
		return reflect.DeepEqual(live, observed)
	}
	// Historical journal recovery permits work already recovered by another
	// cooperating process, but every remaining scope must still match exactly.
	if live.Required && live.StateDigest != observed.StateDigest {
		return false
	}
	for _, journal := range live.Journals {
		if !containsJournal(observed.Journals, journal) {
			return false
		}
	}
	for _, receipt := range live.Receipts {
		if !containsReceipt(observed.Receipts, receipt) {
			return false
		}
	}
	return true
}
func containsJournal(items []PendingJournal, want PendingJournal) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
func containsReceipt(items []PendingReceipt, want PendingReceipt) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
