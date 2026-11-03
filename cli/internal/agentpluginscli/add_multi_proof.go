package agentpluginscli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// revalidateLoadedTargetIntents closes the gap between read-only discovery and
// the selected-target lifecycle probe. The latter can discover an executable
// that makes automatic activation applicable while also exposing a host
// capability failure. Re-run package-aware preflight with that exact evidence
// so preparation-capable clients such as Kiro are downgraded before the atomic
// group preflight instead of aborting every selected target.
func revalidateLoadedTargetIntents(
	ctx context.Context,
	app App,
	opts *options,
	loaded loadedPackage,
	targets []domain.ClientID,
	clients []domain.DetectedClient,
) error {
	selected, _, err := preflightSelectedTargets(ctx, app, targets, clients, false)
	if err != nil {
		return err
	}
	preparationCandidates := make([]domain.DetectedClient, 0, len(selected))
	for _, client := range selected {
		if allowsHostedPrepare(client.ClientID) && opts.installIntents[client.ClientID] != domain.InstallIntentPrepare {
			preparationCandidates = append(preparationCandidates, client)
		}
	}
	if len(preparationCandidates) == 0 {
		return nil
	}
	_, rejected := app.compatibleLoadedTargets(ctx, loaded, preparationCandidates, opts.installIntents)
	if len(rejected) == 0 {
		return nil
	}
	first := rejected[0]
	return fmt.Errorf("selected target %s is not available for this package: %s; no target was changed", first.Client, first.Reason)
}

func containsClientID(targets []domain.ClientID, wanted domain.ClientID) bool {
	for _, target := range targets {
		if target == wanted {
			return true
		}
	}
	return false
}

func withoutClientID(targets []domain.ClientID, excluded domain.ClientID) []domain.ClientID {
	filtered := make([]domain.ClientID, 0, len(targets))
	for _, target := range targets {
		if target != excluded {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func appendDeferredChatGPT(result *addMultiResult, cmd *cobra.Command, loaded loadedPackage) {
	personal, _ := personalMappingTarget(domain.SupportedClientIDs())
	action := chatGPTRegistrationResumeAction(cmd, loaded.envelope.Source.RequestedSource, []domain.ClientID{personal})
	if result.DryRun {
		result.Status = "planned_with_action_required"
	} else if result.Failed == 0 {
		// Preserve failure statuses such as external_partial_failure.
		result.Status = "completed_with_action_required"
	}
	entry := addTargetResult{
		Target: string(personal), Status: "action_required", NextAction: action,
		displayName: domain.ClientDisplayName(personal),
	}
	result.Targets = append(result.Targets, entry)
	countBatchTarget(result, entry)
	result.setTargetProof(personal, "not_run")
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
		Fetched:          loaded.envelope.Source.Repository != "" || loaded.envelope.Source.SourceKind == "git" || loaded.envelope.Source.SourceKind == "archive" && !filepath.IsAbs(loaded.envelope.Source.SourceURL),
		Validated:        true,
	}, nil
}

func acquisitionSourceKind(loaded loadedPackage) string {
	if loaded.origin == domain.OriginModeDirectory {
		return "directory"
	}
	if loaded.envelope.Source.SourceKind != "" {
		return loaded.envelope.Source.SourceKind
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
// repository, subpath, and revision. New typed direct sources additionally bind
// the normalized endpoint and original archive byte checksum; legacy tuples stay
// byte-identical.
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
	if source.SourceKind != "" {
		endpoint := source.SourceURL
		if filepath.IsAbs(endpoint) {
			endpoint = ""
		}
		fields = append(fields, endpoint, source.ArchiveSHA256)
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
	case usecase.GroupTargetExternalNotAttempted:
		return "not_completed"
	default:
		return "not_completed"
	}
}

func groupFailureStatus(phase usecase.GroupPhase) string {
	switch phase {
	case usecase.GroupPhasePreparationFailed, usecase.GroupPhaseManagedRolledBack, usecase.GroupPhaseManagedCommitUnknown,
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
		return publicRepositoryPath(source.Repository, source.PackageSubpath)
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
