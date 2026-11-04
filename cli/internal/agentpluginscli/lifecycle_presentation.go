package agentpluginscli

import (
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func renderRepairResult(writer io.Writer, format string, installation domain.Installation, result usecase.AddResult, dryRun bool) error {
	data := newRepairResultData(installation, result, dryRun)
	if format == "json" {
		return writeJSONOutput(writer, "repair", data)
	}
	if err := renderOpenCodeRuntimeNotice(writer, result); err != nil {
		return err
	}
	if !dryRun {
		if action := localTargetLifecycleAction(result, ""); action != "" {
			if _, err := fmt.Fprintf(writer, "Next: %s\n", action); err != nil {
				return err
			}
		}
	}
	if result.NoChange {
		_, err := fmt.Fprintln(writer, "Managed package digest is valid. No repair was needed.")
		return err
	}
	if dryRun {
		_, err := fmt.Fprintln(writer, "The managed package or its recorded verification state can be safely repaired. No changes made.")
		return err
	}
	if result.Mutated {
		if result.Receipt.OperationID == "" {
			_, err := fmt.Fprintln(writer, "Managed package digest reverified and recorded lifecycle state corrected; no external client mutation was attempted.")
			return err
		} else {
			_, err := fmt.Fprintln(writer, "Managed package repaired from the exact installed revision and its package digest verified; external client activation and authentication were not reverified.")
			return err
		}
	}
	return nil
}

type repairResultData struct {
	OperationID    string            `json:"operation_id,omitempty"`
	Plugin         string            `json:"plugin"`
	Version        string            `json:"version,omitempty"`
	Source         string            `json:"source"`
	TreeDigest     string            `json:"tree_digest"`
	ManifestDigest string            `json:"manifest_digest"`
	NextAction     string            `json:"next_action,omitempty"`
	DryRun         bool              `json:"dry_run"`
	Result         usecase.AddResult `json:"result"`
}

func newRepairResultData(installation domain.Installation, result usecase.AddResult, dryRun bool) repairResultData {
	return repairResultData{
		OperationID: result.Receipt.OperationID, Plugin: installation.DeclaredName,
		Version: installation.Package.Version, Source: publicSource(installation.Source),
		TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest,
		NextAction: nextLifecycleAction(result), DryRun: dryRun, Result: result,
	}
}

func renderLegacyRemove(writer io.Writer, format string, result usecase.LegacyRemoveResult, dryRun bool) error {
	data := struct {
		DryRun bool                       `json:"dry_run"`
		Result usecase.LegacyRemoveResult `json:"result"`
	}{DryRun: dryRun, Result: result}
	if format == "json" {
		return writeJSONOutput(writer, "remove", data)
	}
	if dryRun {
		renderLegacyRemovePlan(writer, result)
		return nil
	}
	if result.Mutated {
		_, _ = fmt.Fprintln(writer, "Legacy targets removed and Agent Plugins state reconciled.")
	}
	return nil
}

func renderLegacyRemovePlan(writer io.Writer, result usecase.LegacyRemoveResult) {
	_, _ = fmt.Fprintf(writer, "Legacy plugin: %s\n", result.Plugin)
	_, _ = fmt.Fprintln(writer, "Removal is integration-wide through the original plugin-kit-ai lifecycle:")
	for _, target := range result.Targets {
		_, _ = fmt.Fprintf(writer, "  - %s\n", target)
	}
	if result.Reconciled {
		_, _ = fmt.Fprintln(writer, "Legacy lifecycle already reports the installation absent; only Agent Plugins state reconciliation remains.")
	}
}

func renderUpdateResult(writer io.Writer, format string, envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool) error {
	data := newAddResultData(envelope, result, dryRun)
	if format == "json" {
		return writeJSONOutput(writer, "update", data)
	}
	if dryRun {
		return renderHumanPlan(writer, envelope, result)
	}
	if err := renderOpenCodeRuntimeNotice(writer, result); err != nil {
		return err
	}
	if result.NoChange {
		if action := localTargetLifecycleAction(result, ""); action != "" {
			if _, err := fmt.Fprintf(writer, "Next: %s\n", action); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintln(writer, "Already up to date. No changes made.")
		return nil
	}
	if result.Mutated && fullyInstalled(result.Activation) {
		if reportsMCPToolNamespaceCollision(result.Plan.ClientID) && len(domain.SelectedMCPNames(result.Plan)) > 0 {
			_, _ = fmt.Fprintln(writer, "OpenCode MCP configuration updated and verified.")
		} else {
			_, _ = fmt.Fprintln(writer, "Updated and verified for the selected client.")
		}
		return nil
	}
	if result.Mutated {
		if result.Activation.Authentication == domain.AuthenticationPending {
			if result.Activation.Activation == domain.ActivationActive {
				_, _ = fmt.Fprintln(writer, "Package updated and client activation completed. Authentication is pending.")
			} else {
				_, _ = fmt.Fprintln(writer, "Package updated. Authentication and client activation are pending.")
			}
		} else if result.Activation.Authentication == domain.AuthenticationNotChecked && result.Activation.Activation == domain.ActivationActive {
			_, _ = fmt.Fprintln(writer, "Package updated and client activation verified. Authentication requirements have not been checked.")
		} else {
			_, _ = fmt.Fprintln(writer, "Package updated. Activation is not complete yet.")
		}
	}
	if action := nextLocalLifecycleAction(result); action != "" && !fullyInstalled(result.Activation) {
		_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
	}
	return nil
}

func renderRemoveResult(writer io.Writer, format string, installation domain.Installation, result usecase.RemoveResult, dryRun bool) error {
	data := newRemoveResultData(installation, result, dryRun)
	if format == "json" {
		return writeJSONOutput(writer, "remove", data)
	}
	if dryRun {
		if result.Deactivation.ArtifactRemovalAllowed {
			_, _ = fmt.Fprintf(writer, "Would remove %s from %s. No changes made.\n", installation.DeclaredName, result.ClientID)
		} else {
			_, _ = fmt.Fprintf(writer, "Would not remove %s from %s until the external client uninstall is confirmed. No changes made.\n", installation.DeclaredName, result.ClientID)
		}
		for _, action := range result.Deactivation.UserActions {
			_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
		}
		for _, action := range result.Deactivation.LocalActions {
			_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
		}
		return nil
	}
	if result.Mutated {
		_, _ = fmt.Fprintln(writer, "Removed the selected managed package.")
	}
	if removalNeedsManualStep(result.Deactivation) {
		_, _ = fmt.Fprintln(writer, removalIncompleteNotice)
	}
	for _, action := range result.Deactivation.UserActions {
		_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
	}
	for _, action := range result.Deactivation.LocalActions {
		_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
	}
	return nil
}

// removalIncompleteNotice keeps partial removals explicit: without it a run
// that stopped for a manual client step reads like a finished removal.
const removalIncompleteNotice = "Removal is not complete: a manual client step is still required."

func removalNeedsManualStep(deactivation domain.DeactivationOutcome) bool {
	return !deactivation.ArtifactRemovalAllowed || len(deactivation.UserActions) > 0
}

type removeResultData struct {
	OperationID    string                  `json:"operation_id,omitempty"`
	Plugin         string                  `json:"plugin"`
	Version        string                  `json:"version,omitempty"`
	Source         string                  `json:"source"`
	Revision       string                  `json:"revision,omitempty"`
	TreeDigest     string                  `json:"tree_digest,omitempty"`
	ManifestDigest string                  `json:"manifest_digest,omitempty"`
	Directory      *domain.DirectoryOrigin `json:"directory,omitempty"`
	NextAction     string                  `json:"next_action,omitempty"`
	DryRun         bool                    `json:"dry_run"`
	Result         usecase.RemoveResult    `json:"result"`
}

func newRemoveResultData(installation domain.Installation, result usecase.RemoveResult, dryRun bool) removeResultData {
	return removeResultData{
		OperationID: result.Receipt.OperationID, Plugin: installation.DeclaredName,
		Version: installation.Package.Version, Source: publicSource(installation.Source), Revision: installation.Source.ResolvedRevision,
		TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest, Directory: cloneDirectoryOrigin(installation.Directory),
		NextAction: nextRemoveAction(result), DryRun: dryRun, Result: result,
	}
}

func nextRemoveAction(result usecase.RemoveResult) string {
	if len(result.Deactivation.LocalActions) > 0 {
		return result.Deactivation.LocalActions[0]
	}
	if len(result.Deactivation.UserActions) > 0 {
		return result.Deactivation.UserActions[0]
	}
	return ""
}
