package agentpluginscli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

type addTargetResult struct {
	Target     string        `json:"target"`
	Status     string        `json:"status"`
	NextAction string        `json:"next_action,omitempty"`
	Output     addResultData `json:"output"`
}

type addMultiResult struct {
	OperationID    string                     `json:"operation_id,omitempty"`
	Batch          bool                       `json:"batch"`
	Status         string                     `json:"status"`
	Succeeded      int                        `json:"succeeded"`
	Failed         int                        `json:"failed"`
	Plugin         string                     `json:"plugin"`
	Version        string                     `json:"version,omitempty"`
	Source         string                     `json:"source"`
	Revision       string                     `json:"revision,omitempty"`
	TreeDigest     string                     `json:"tree_digest,omitempty"`
	ManifestDigest string                     `json:"manifest_digest,omitempty"`
	Security       *domain.SecurityAssessment `json:"security,omitempty"`
	Directory      *domain.DirectoryOrigin    `json:"directory,omitempty"`
	DryRun         bool                       `json:"dry_run"`
	Targets        []addTargetResult          `json:"targets"`
	Acquisition    *addAcquisitionProof       `json:"acquisition,omitempty"`
	TargetOutcomes map[string]addTargetProof  `json:"target_outcomes,omitempty"`
}

type addAcquisitionProof struct {
	AcquisitionID    string `json:"acquisition_id"`
	AcquisitionCount int    `json:"acquisition_count"`
	TreeDigest       string `json:"tree_digest"`
	ManifestDigest   string `json:"manifest_digest"`
	ClosureDigest    string `json:"closure_digest"`
	SourceKind       string `json:"source_kind"`
	Fetched          bool   `json:"fetched"`
	Validated        bool   `json:"validated"`
}

type addTargetProof struct {
	Outcome        string `json:"outcome"`
	AcquisitionID  string `json:"acquisition_id"`
	TreeDigest     string `json:"tree_digest"`
	ManifestDigest string `json:"manifest_digest"`
	ClosureDigest  string `json:"closure_digest"`
}

// runAddMany is deliberately not implemented as repeated CLI invocations. It
// resolves and validates one package, detects clients once, builds every plan,
// and only begins applying after the complete selected set passes preflight.
func runAddMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, activationComplete, authComplete bool) error {
	return runAddManyWithClients(ctx, cmd, app, opts, source, targets, activationComplete, authComplete, nil)
}

func runAddManyWithClients(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, activationComplete, authComplete bool, clients []domain.DetectedClient) error {
	_, detected, err := preflightSelectedTargets(ctx, app, targets, clients, !opts.dryRun && isDirectorySelector(source))
	if err != nil {
		return err
	}
	writeProgress(app, opts.format, "Resolving and validating one Agent Plugin package for every selected target...")
	loaded, err := app.loadPackageFor(ctx, source, withDetectedClients(app.addResolutionRequest(source, expandAffectedSurfaceTargets(targets)), detected))
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	return runAddManyLoaded(ctx, cmd, app, opts, loaded, targets, activationComplete, authComplete, detectedClientValues(detected), false)
}

func runAddManyLoaded(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, targets []domain.ClientID, activationComplete, authComplete bool, clients []domain.DetectedClient, needsInstallConfirmation bool) error {
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	if len(targets) == 1 {
		selectedOptions := *opts
		selectedOptions.target = string(targets[0])
		if activationComplete || authComplete {
			return runAddLoaded(ctx, cmd, app, &selectedOptions, loaded, activationComplete, authComplete, clients, needsInstallConfirmation)
		}
		if state, err := app.StateStore.Load(); err == nil {
			if installation, ok := locallyMatchedInstallation(state, loaded.envelope.Manifest.Name); ok && installationHasTarget(installation, targets[0], string(domain.ScopeUser)) {
				return runAddLoaded(ctx, cmd, app, &selectedOptions, loaded, false, false, clients, needsInstallConfirmation)
			}
		}
	}
	combined := addMultiResult{
		Batch: true, Status: "planned", Plugin: loaded.envelope.Manifest.Name,
		Version: loaded.envelope.Manifest.Version, Source: publicPackageSource(loaded.envelope.Source),
		Revision: loaded.envelope.Source.ResolvedRevision, TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest,
		Security:  loaded.security,
		Directory: cloneDirectoryOrigin(loaded.directory),
		DryRun:    opts.dryRun, Targets: make([]addTargetResult, 0, len(targets)),
	}
	selected, detected, err := preflightSelectedTargets(ctx, app, targets, clients, false)
	if err != nil {
		return err
	}
	combined.Targets = combined.Targets[:0]
	service := lifecycleService(app, detected)
	inputs := make([]usecase.AddInput, len(selected))
	for index, client := range selected {
		clientPackage := cloneLoadedPackage(loaded)
		if err := prepareLoadedPackageForClient(&clientPackage, client.ClientID); err != nil {
			return fmt.Errorf("preflight target %s: %w; no target was changed", client.ClientID, err)
		}
		input := usecase.AddInput{
			Envelope: clientPackage.envelope, Client: client, Scope: domain.ScopeUser,
			DryRun: true, Interactive: app.Terminal, Hints: clientPackage.hints,
			BackendExecutable:  backendExecutable(client, detected),
			ActivationComplete: activationComplete, AuthComplete: authComplete,
			PersistAuthoritativeObservations: false,
			OriginMode:                       loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
			DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
		}
		inputs[index] = input
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	combined.OperationID = operationID
	groupInput := usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, DryRun: true}
	planned, err := service.AddGroup(ctx, groupInput)
	combined.Targets = combined.Targets[:0]
	for index, result := range planned.Targets {
		output := newAddResultData(inputs[index].Envelope, result, true)
		output.OperationID = operationID
		combined.Targets = append(combined.Targets, addTargetResult{Target: string(selected[index].ClientID), Status: groupTargetStatus(result), Output: output, NextAction: nextLifecycleAction(result)})
		combined.setTargetProof(selected[index].ClientID, "not_run")
	}
	combined.Succeeded = len(planned.Targets)
	if err != nil {
		combined.Status, combined.Failed, combined.Succeeded = "preflight_failed", len(inputs), 0
		_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
		return fmt.Errorf("group preflight failed; no target was changed (selected targets: %v): %w%s", targets, err, addGroupNextAction(combined.Targets))
	}
	if opts.dryRun {
		return renderAddMultiResult(cmd, opts, combined, loaded.envelope)
	}
	if needsInstallConfirmation {
		accepted, err := confirmInstall(ctx, cmd, app, loaded, humanAddGroupPlans(combined.Targets))
		if err != nil {
			return err
		}
		if !accepted {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Installation not applied.")
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(targets) > 1 {
		proof, proofErr := newAddAcquisitionProof(loaded)
		if proofErr != nil {
			return proofErr
		}
		combined.Acquisition = &proof
		combined.TargetOutcomes = make(map[string]addTargetProof, len(selected))
		for _, client := range selected {
			combined.setTargetProof(client.ClientID, "not_run")
		}
	}
	writeProgress(app, opts.format, "Applying the completely preflighted multi-target plan...")
	groupInput.DryRun, groupInput.Confirmed = false, true
	applied, err := service.AddGroup(ctx, groupInput)
	combined.Status, combined.Targets, combined.Succeeded = string(applied.Phase), combined.Targets[:0], 0
	for index, result := range applied.Targets {
		output := newAddResultData(inputs[index].Envelope, result, false)
		output.OperationID = operationID
		combined.Targets = append(combined.Targets, addTargetResult{Target: string(selected[index].ClientID), Status: groupTargetStatus(result), Output: output, NextAction: nextLifecycleAction(result)})
		combined.setTargetProof(selected[index].ClientID, addTargetProofOutcome(result))
		if result.GroupPhase == usecase.GroupTargetExternalCompleted {
			combined.Succeeded++
		}
	}
	if err != nil {
		if applied.Phase == usecase.GroupPhasePlanned && !applied.Mutated {
			combined.Status, combined.Failed, combined.Succeeded = "preflight_failed", len(inputs), 0
			_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
			return fmt.Errorf("group apply preflight failed; no target was changed (selected targets: %v): %w%s", targets, err, addGroupNextAction(combined.Targets))
		}
		combined.Status = groupFailureStatus(applied.Phase)
		combined.Failed = len(inputs) - combined.Succeeded
		_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
		return err
	}
	if err := renderAddMultiResult(cmd, opts, combined, loaded.envelope); err != nil {
		return err
	}
	if app.Terminal && opts.format == "human" && len(applied.Targets) == 1 {
		input := inputs[0]
		input.DryRun = false
		input.InstallationID = applied.Targets[0].InstallationID
		return resumeInteractiveLifecycle(ctx, cmd, service, input, inputs[0].Envelope, applied.Targets[0])
	}
	return nil
}

func newAddAcquisitionProof(loaded loadedPackage) (addAcquisitionProof, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return addAcquisitionProof{}, fmt.Errorf("create acquisition ID: %w", err)
	}
	sourceKind := acquisitionSourceKind(loaded)
	return addAcquisitionProof{
		AcquisitionID:    "acq-" + hex.EncodeToString(value[:]),
		AcquisitionCount: 1,
		TreeDigest:       loaded.envelope.TreeDigest,
		ManifestDigest:   loaded.envelope.ManifestDigest,
		ClosureDigest:    groupedAcquisitionClosureDigest(sourceKind, loaded.envelope.Source, loaded.envelope.TreeDigest, loaded.envelope.ManifestDigest),
		SourceKind:       sourceKind,
		Fetched:          loaded.envelope.Source.Repository != "",
		Validated:        true,
	}, nil
}

func acquisitionSourceKind(loaded loadedPackage) string {
	if loaded.origin == domain.OriginModeDirectory {
		return "directory"
	}
	if loaded.envelope.Source.Repository != "" {
		return "github"
	}
	return "local"
}

// groupedAcquisitionClosureDigest is the domain-separated SHA-256 of a
// length-prefixed tuple:
//
//	source kind, repository, package subpath, resolved revision,
//	validated tree digest, validated manifest digest
//
// The agentplugins/grouped-acquisition-closure/v1 domain makes this a distinct
// identity from a package tree digest. Requested and canonical source strings
// are deliberately excluded because they may contain local, host-specific
// paths. For local acquisitions, source kind plus the two validated package
// identities form the closure; immutable remote acquisitions additionally bind
// repository, subpath, and revision.
func groupedAcquisitionClosureDigest(sourceKind string, source domain.SourceIdentity, treeDigest, manifestDigest string) string {
	hash := sha256.New()
	fields := []string{
		"agentplugins/grouped-acquisition-closure/v1",
		sourceKind,
		strings.TrimSpace(source.Repository),
		strings.TrimSpace(source.PackageSubpath),
		strings.TrimSpace(source.ResolvedRevision),
		strings.TrimSpace(treeDigest),
		strings.TrimSpace(manifestDigest),
	}
	var size [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func (result *addMultiResult) setTargetProof(target domain.ClientID, outcome string) {
	if result.Acquisition == nil || result.TargetOutcomes == nil {
		return
	}
	proof := result.Acquisition
	result.TargetOutcomes[string(target)] = addTargetProof{
		Outcome: outcome, AcquisitionID: proof.AcquisitionID,
		TreeDigest: proof.TreeDigest, ManifestDigest: proof.ManifestDigest, ClosureDigest: proof.ClosureDigest,
	}
}

func addTargetProofOutcome(result usecase.AddResult) string {
	switch result.GroupPhase {
	case usecase.GroupTargetExternalCompleted:
		return "passed"
	case usecase.GroupTargetExternalFailed:
		return "failed"
	case usecase.GroupTargetManagedRolledBack:
		return "rolled_back"
	case usecase.GroupTargetManagedUnknown:
		return "unknown"
	case usecase.GroupTargetExternalPartial:
		return "partial"
	default:
		return "not_completed"
	}
}

func groupFailureStatus(phase usecase.GroupPhase) string {
	switch phase {
	case usecase.GroupPhaseManagedRolledBack, usecase.GroupPhaseManagedCommitUnknown,
		usecase.GroupPhaseManagedActivationFailed, usecase.GroupPhaseExternalPartialFailure:
		return string(phase)
	case usecase.GroupPhaseManagedCommitted:
		return "managed_committed_finalization_failed"
	default:
		return "apply_failed"
	}
}

func groupTargetStatus(result usecase.AddResult) string {
	if result.GroupPhase != "" {
		return string(result.GroupPhase)
	}
	return string(result.Plan.Status)
}

func publicPackageSource(source domain.SourceIdentity) string {
	if source.Repository != "" {
		value := source.Repository
		if source.PackageSubpath != "" {
			value += "//" + source.PackageSubpath
		}
		return value
	}
	if source.ResolvedRevision != "" {
		return "direct immutable source"
	}
	return "direct local source"
}

func cloneLoadedPackage(source loadedPackage) loadedPackage {
	clone := source
	clone.cleanup = nil
	clone.envelope.Inventory.Skills = append([]string(nil), source.envelope.Inventory.Skills...)
	clone.envelope.Inventory.MCPServers = append([]string(nil), source.envelope.Inventory.MCPServers...)
	clone.envelope.Inventory.AppBindings = append([]string(nil), source.envelope.Inventory.AppBindings...)
	clone.envelope.App.Bindings = make(map[string]domain.AppBinding, len(source.envelope.App.Bindings))
	for key, value := range source.envelope.App.Bindings {
		clone.envelope.App.Bindings[key] = value
	}
	return clone
}

func renderAddMultiResult(cmd *cobra.Command, opts *options, result addMultiResult, envelope domain.PackageEnvelope) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "add", overall, result)
	}
	if len(result.Targets) == 1 {
		return renderAddResult(cmd.OutOrStdout(), "human", envelope, result.Targets[0].Output.Result, result.DryRun)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Plugin: %s %s\n", result.Plugin, result.Version); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Targets: %s\n", addResultTargets(result.Targets)); err != nil {
		return err
	}
	for _, target := range result.Targets {
		if err := renderOpenCodeRuntimeNotice(cmd.OutOrStdout(), target.Output.Result); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", target.Target, target.Status); err != nil {
			return err
		}
		if target.NextAction != "" && !fullyInstalled(target.Output.Result.Activation) {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    Next: %s\n", target.NextAction); err != nil {
				return err
			}
		}
	}
	if result.DryRun {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "No changes made (dry run)."); err != nil {
			return err
		}
	}
	return nil
}

func addResultTargets(results []addTargetResult) string {
	values := make([]string, len(results))
	for index, result := range results {
		values[index] = result.Target
	}
	return strings.Join(values, ",")
}

// The group result describes physical delivery. Review must also identify the
// selected logical surface when Copilot and VS Code share that delivery.
// Copy the presentation data so JSON, apply inputs and persisted IDs keep their
// physical ownership semantics.
func humanAddGroupPlans(targets []addTargetResult) []usecase.AddResult {
	results := make([]usecase.AddResult, len(targets))
	for index, target := range targets {
		result := target.Output.Result
		if target.Target != string(result.Plan.ClientID) {
			result.Plan.LocalActions = append(append([]string(nil), result.Plan.LocalActions...),
				fmt.Sprintf("Uses shared physical binding owned by %s", result.Plan.ClientID))
			result.Plan.ClientID = domain.ClientID(target.Target)
		}
		results[index] = result
	}
	return results
}

func addGroupNextAction(targets []addTargetResult) string {
	for _, target := range targets {
		if target.NextAction != "" {
			return "; next action: " + target.NextAction
		}
	}
	return ""
}
