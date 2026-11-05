package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type publicSafetyWarning struct {
	Code            string   `json:"code"`
	Message         string   `json:"message"`
	Action          string   `json:"action,omitempty"`
	DistributionID  string   `json:"distribution_id,omitempty"`
	ReleaseSequence uint64   `json:"release_sequence,omitempty"`
	Clients         []string `json:"clients,omitempty"`
}

type publicEvidence struct {
	ID                    string                           `json:"id"`
	DistributionID        string                           `json:"distribution_id"`
	ReleaseSequence       uint64                           `json:"release_sequence"`
	Level                 string                           `json:"level"`
	Outcome               string                           `json:"outcome"`
	Client                domain.ClientID                  `json:"client,omitempty"`
	Scope                 domain.InstallScope              `json:"scope,omitempty"`
	ClientVersion         string                           `json:"client_version,omitempty"`
	InstallerVersion      string                           `json:"installer_version,omitempty"`
	OS                    string                           `json:"os,omitempty"`
	Architecture          string                           `json:"architecture,omitempty"`
	DependencyIdentity    string                           `json:"dependency_identity,omitempty"`
	ObservedAt            string                           `json:"observed_at,omitempty"`
	PackageTreeDigest     string                           `json:"package_tree_digest"`
	Artifact              domain.DirectoryEvidenceArtifact `json:"artifact"`
	Trust                 *domain.DirectoryEvidenceTrust   `json:"trust,omitempty"`
	TrustedForEligibility bool                             `json:"trusted_for_eligibility"`
}

type publicTargetCompatibility struct {
	Client           domain.ClientID                   `json:"client"`
	Scopes           []domain.InstallScope             `json:"scopes"`
	Delivery         string                            `json:"delivery"`
	Evidence         []string                          `json:"evidence,omitempty"`
	CanonicalClosure *domain.DirectoryCanonicalClosure `json:"canonical_closure,omitempty"`
}

type publicDistribution struct {
	ID               string                      `json:"id"`
	Kind             domain.DistributionKind     `json:"kind"`
	Status           domain.DistributionStatus   `json:"status"`
	ReviewedDefault  bool                        `json:"reviewed_default"`
	ReleaseSequence  uint64                      `json:"release_sequence,omitempty"`
	PackageVersion   string                      `json:"package_version,omitempty"`
	ResolvedRevision string                      `json:"resolved_revision,omitempty"`
	Repository       string                      `json:"repository,omitempty"`
	PackagePath      string                      `json:"package_path,omitempty"`
	ReleaseStatus    domain.ReleaseStatus        `json:"release_status,omitempty"`
	TreeDigest       string                      `json:"tree_digest,omitempty"`
	ManifestDigest   string                      `json:"manifest_digest,omitempty"`
	Targets          []publicTargetCompatibility `json:"target_compatibility,omitempty"`
	Evidence         []publicEvidence            `json:"immutable_evidence,omitempty"`
}

type publicProductInspection struct {
	ProductID            string                       `json:"product_id"`
	DisplayName          string                       `json:"display_name"`
	Description          string                       `json:"description"`
	ReviewedDefault      string                       `json:"reviewed_default"`
	SelectedDistribution string                       `json:"selected_distribution"`
	SelectedRelease      publicDistribution           `json:"selected_release"`
	SelectionReason      string                       `json:"selection_reason"`
	SelectionDiagnostics []domain.DirectoryDiagnostic `json:"selection_diagnostics,omitempty"`
	TargetCompatibility  []publicTargetCompatibility  `json:"target_compatibility"`
	ImmutableEvidence    []publicEvidence             `json:"immutable_evidence"`
	Alternatives         []publicDistribution         `json:"alternatives"`
	SnapshotSequence     uint64                       `json:"snapshot_sequence"`
	SnapshotDigest       string                       `json:"snapshot_digest"`
}

type publicInstalledDirectory struct {
	ProductID                   string           `json:"product_id"`
	RecordedDistribution        string           `json:"recorded_distribution"`
	CurrentDistribution         string           `json:"current_distribution,omitempty"`
	ReviewedDefaultDistribution string           `json:"reviewed_default_distribution,omitempty"`
	RecordedRevision            string           `json:"recorded_revision"`
	CurrentRevision             string           `json:"current_revision,omitempty"`
	CurrentRepository           string           `json:"current_repository,omitempty"`
	CurrentPackagePath          string           `json:"current_package_path,omitempty"`
	RecordedReleaseSequence     uint64           `json:"recorded_release_sequence"`
	CurrentReleaseSequence      uint64           `json:"current_release_sequence,omitempty"`
	RecordedSnapshotSequence    uint64           `json:"recorded_snapshot_sequence,omitempty"`
	CurrentSnapshotSequence     uint64           `json:"current_snapshot_sequence,omitempty"`
	CurrentEvidence             []publicEvidence `json:"current_immutable_evidence,omitempty"`
}

type localDirectoryReader interface {
	LoadLocal(uint64) (directoryv1.VerifiedBundle, error)
}

func inspectDirectoryProduct(ctx context.Context, app App, state domain.StateFileV2, selector, targetOption string) (publicProductInspection, error) {
	if app.DirectoryClient == nil {
		return publicProductInspection{}, fmt.Errorf("signed Directory dependencies are unavailable")
	}
	targets, err := parseTargetOption(targetOption)
	if err != nil {
		return publicProductInspection{}, err
	}
	bundle, err := app.DirectoryClient.Load(ctx, directoryFloorForClient(state, app.DirectoryClient))
	if err != nil {
		return publicProductInspection{}, fmt.Errorf("load signed Directory: %w", err)
	}
	selection, err := domain.ResolveDirectory(bundle.Snapshot, domain.DirectoryResolveRequest{Selector: selector, Targets: targets,
		Scope: app.scope(), InstallerVersion: app.Version, SchemaVersion: "1.0.0", Operation: domain.DirectoryInstall})
	if err != nil {
		return publicProductInspection{}, err
	}
	product := snapshotProduct(bundle.Snapshot, selection.ProductID)
	if product == nil {
		return publicProductInspection{}, fmt.Errorf("selected Directory product is missing")
	}
	selected := inspectDistribution(bundle.Snapshot, *product, selection.DistributionID, selection.ReleaseSequence)
	result := publicProductInspection{ProductID: product.ID, DisplayName: product.DisplayName, Description: product.Description,
		ReviewedDefault: product.DefaultDistribution, SelectedDistribution: selection.DistributionID,
		SelectedRelease: selected,
		SelectionReason: "reviewed default is eligible for the requested targets", SnapshotSequence: bundle.Snapshot.Sequence,
		SnapshotDigest: bundle.Digest, TargetCompatibility: selected.Targets, ImmutableEvidence: selected.Evidence,
		SelectionDiagnostics: append([]domain.DirectoryDiagnostic(nil), selection.Diagnostics...)}
	if selection.Fallback {
		reasons := make([]string, 0, len(selection.Diagnostics))
		for _, diagnostic := range selection.Diagnostics {
			reasons = append(reasons, diagnostic.DistributionID+": "+diagnostic.Message)
		}
		result.SelectionReason = "reviewed default is ineligible; selected the first eligible reviewed alternative"
		if len(reasons) > 0 {
			result.SelectionReason += " (" + strings.Join(reasons, "; ") + ")"
		}
	}
	if strings.Contains(selector, "/") {
		result.SelectionReason = "explicit qualified distribution"
	}
	for _, id := range product.Distributions {
		if id == selection.DistributionID {
			continue
		}
		result.Alternatives = append(result.Alternatives, inspectDistribution(bundle.Snapshot, *product, id, 0))
	}
	sort.Slice(result.Alternatives, func(i, j int) bool { return result.Alternatives[i].ID < result.Alternatives[j].ID })
	return result, nil
}

func inspectInstalledProduct(ctx context.Context, app App, state domain.StateFileV2, installation domain.Installation) (publicInstallation, error) {
	result := publicInstallationView(installation, true)
	bundle, ok, err := directoryBundleForRead(ctx, app, state, installation.OriginMode == domain.OriginModeDirectory)
	if err != nil {
		return publicInstallation{}, err
	}
	if !ok {
		result.MixedVersion, result.ConvergenceAction = convergenceState(installation)
		return result, nil
	}
	if installation.Directory == nil {
		result.Warnings = installationDigestWarnings(bundle.Snapshot, installation)
		result.MixedVersion, result.ConvergenceAction = convergenceState(installation)
		return result, nil
	}
	origin := installation.Directory
	view := &publicInstalledDirectory{ProductID: origin.ProductID, RecordedDistribution: origin.DistributionID,
		RecordedRevision: installation.Source.ResolvedRevision, RecordedReleaseSequence: origin.DesiredReleaseSequence,
		RecordedSnapshotSequence: origin.SnapshotSequence, CurrentSnapshotSequence: bundle.Snapshot.Sequence}
	if product := snapshotProduct(bundle.Snapshot, origin.ProductID); product != nil {
		view.ReviewedDefaultDistribution = product.DefaultDistribution
		current := inspectDistribution(bundle.Snapshot, *product, origin.DistributionID, 0)
		if current.ReleaseSequence > 0 {
			view.CurrentDistribution, view.CurrentRevision = origin.DistributionID, current.ResolvedRevision
			view.CurrentRepository, view.CurrentPackagePath = current.Repository, current.PackagePath
			view.CurrentReleaseSequence, view.CurrentEvidence = current.ReleaseSequence, current.Evidence
		}
	}
	targets := installationTargets(installation, string(app.scope()))
	selection, err := domain.ResolveDirectory(bundle.Snapshot, domain.DirectoryResolveRequest{Selector: origin.DistributionID,
		Targets: targets, Scope: app.scope(), InstallerVersion: app.Version, SchemaVersion: "1.0.0", Operation: domain.DirectoryInstall})
	if err == nil {
		view.CurrentDistribution, view.CurrentRevision = selection.DistributionID, selection.Source.Revision
		view.CurrentRepository, view.CurrentPackagePath = selection.Source.Repository, selection.Source.Path
		view.CurrentReleaseSequence = selection.ReleaseSequence
		view.CurrentEvidence = evidenceForRelease(bundle.Snapshot, selection.DistributionID, selection.ReleaseSequence)
	}
	result.Directory = view
	for index := range result.Clients {
		revision := result.Clients[index].PackageRevision
		if revision != nil {
			revision.Evidence = append(revision.Evidence, evidenceForClientRevision(
				bundle.Snapshot, revision.DistributionID, revision.ReleaseSequence,
				domain.ClientID(result.Clients[index].ClientID), app.scope())...)
		}
	}
	result.Warnings = installedDirectoryWarnings(bundle.Snapshot, installation)
	result.MixedVersion, result.ConvergenceAction = convergenceState(installation)
	return result, nil
}

func directoryBundleForRead(ctx context.Context, app App, state domain.StateFileV2, allowNetwork bool) (directoryv1.VerifiedBundle, bool, error) {
	floor := directoryFloorForClient(state, app.DirectoryClient)
	if allowNetwork && app.DirectoryClient != nil {
		bundle, err := app.DirectoryClient.Load(ctx, floor)
		if err == nil {
			return bundle, true, nil
		}
		if isDirectorySecurityError(err) {
			return directoryv1.VerifiedBundle{}, false, err
		}
	}
	if local, ok := app.DirectoryClient.(localDirectoryReader); ok {
		bundle, err := local.LoadLocal(floor)
		if err == nil {
			return bundle, true, nil
		}
		if allowNetwork && isDirectorySecurityError(err) {
			return directoryv1.VerifiedBundle{}, false, err
		}
	}
	return directoryv1.VerifiedBundle{}, false, nil
}

func isDirectorySecurityError(err error) bool {
	return errors.Is(err, directoryv1.ErrSequenceConflict) || errors.Is(err, directoryv1.ErrRollback)
}

func snapshotProduct(snapshot domain.DirectorySnapshot, id string) *domain.DirectoryProduct {
	for index := range snapshot.Products {
		if snapshot.Products[index].ID == id {
			return &snapshot.Products[index]
		}
	}
	return nil
}

func snapshotDistribution(snapshot domain.DirectorySnapshot, id string) *domain.DirectoryDistribution {
	for index := range snapshot.Distributions {
		if snapshot.Distributions[index].ID == id {
			return &snapshot.Distributions[index]
		}
	}
	return nil
}

func inspectDistribution(snapshot domain.DirectorySnapshot, product domain.DirectoryProduct, id string, sequence uint64) publicDistribution {
	distribution := snapshotDistribution(snapshot, id)
	if distribution == nil {
		return publicDistribution{ID: id, ReviewedDefault: id == product.DefaultDistribution}
	}
	result := publicDistribution{ID: id, Kind: distribution.Kind, Status: distribution.Status, ReviewedDefault: id == product.DefaultDistribution}
	var release *domain.DirectoryRelease
	for index := range distribution.Releases {
		candidate := &distribution.Releases[index]
		if (sequence == 0 || candidate.Sequence == sequence) && (release == nil || candidate.Sequence > release.Sequence) {
			release = candidate
		}
	}
	if release == nil {
		return result
	}
	result.ReleaseSequence, result.PackageVersion, result.ResolvedRevision = release.Sequence, release.PackageVersion, release.PackageSource.Revision
	result.Repository, result.PackagePath = release.PackageSource.Repository, release.PackageSource.Path
	result.TreeDigest, result.ManifestDigest = release.TreeDigest, release.ManifestDigest
	for _, policy := range distribution.ReleasePolicies {
		if policy.ReleaseSequence != release.Sequence {
			continue
		}
		result.ReleaseStatus = policy.Status
		for _, target := range policy.Targets {
			scopes := append([]domain.InstallScope(nil), target.Scopes...)
			evidenceIDs := evidenceIDsForClient(snapshot, id, release.Sequence, target.Client)
			var canonical *domain.DirectoryCanonicalClosure
			if snapshot.SnapshotSchemaVersion == 2 {
				// Scoped v2 evidence and signed closure belong to this exact
				// grant; never infer either from another client or legacy v1.
				scopes = []domain.InstallScope{target.Scope}
				evidenceIDs = evidenceIDsForV2Grant(snapshot, policy, target)
				canonical = target.CanonicalClosure
			}
			result.Targets = append(result.Targets, publicTargetCompatibility{
				Client: target.Client, Scopes: scopes, Delivery: target.Delivery, Evidence: evidenceIDs,
				CanonicalClosure: canonical,
			})
		}
	}
	result.Evidence = evidenceForRelease(snapshot, id, release.Sequence)
	return result
}

func evidenceForRelease(snapshot domain.DirectorySnapshot, distribution string, sequence uint64) []publicEvidence {
	result := []publicEvidence{}
	for _, evidence := range snapshot.Evidence {
		if evidence.DistributionID == distribution && evidence.ReleaseSequence == sequence {
			result = append(result, publicEvidence{ID: evidence.ID, DistributionID: evidence.DistributionID, ReleaseSequence: evidence.ReleaseSequence,
				Level: evidence.Level, Outcome: evidence.Outcome, Client: evidence.Client, Scope: evidence.Scope, ClientVersion: evidence.ClientVersion,
				InstallerVersion: evidence.InstallerVersion, OS: evidence.OS, Architecture: evidence.Architecture,
				DependencyIdentity: evidence.DependencyIdentity, ObservedAt: evidence.ObservedAt,
				PackageTreeDigest: evidence.PackageTreeDigest, Artifact: evidence.Artifact, Trust: evidence.Trust,
				TrustedForEligibility: evidence.HasTrustedEligibilityProvenance()})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// Per-client installed revisions must not present another v2 grant's runtime
// proof as their own. The full release evidence remains visible separately.
func evidenceForClientRevision(snapshot domain.DirectorySnapshot, distribution string, sequence uint64,
	client domain.ClientID, scope domain.InstallScope,
) []publicEvidence {
	result := []publicEvidence{}
	for _, evidence := range evidenceForRelease(snapshot, distribution, sequence) {
		if snapshot.SnapshotSchemaVersion != 2 {
			if evidence.Client == "" || evidence.Client == client {
				result = append(result, evidence)
			}
			continue
		}
		if evidence.Level == "schema" {
			if evidence.Client == "" && evidence.Scope == "" {
				result = append(result, evidence)
			}
			continue
		}
		if evidence.Client == client && (evidence.Scope == scope ||
			scope == domain.ScopeUser && evidence.Scope == "") {
			result = append(result, evidence)
		}
	}
	return result
}

func evidenceIDsForClient(snapshot domain.DirectorySnapshot, distribution string, sequence uint64, client domain.ClientID) []string {
	ids := []string{}
	for _, evidence := range evidenceForRelease(snapshot, distribution, sequence) {
		if evidence.Client == "" || evidence.Client == client {
			ids = append(ids, evidence.ID)
		}
	}
	return ids
}

// V2 compatibility rows must not claim policy-level evidence belonging to
// another client or scope. Full release evidence remains visible separately.
func evidenceIDsForV2Grant(snapshot domain.DirectorySnapshot, policy domain.DirectoryReleasePolicy, target domain.DirectoryTarget) []string {
	ids := append(append([]string(nil), policy.CurrentEvidence...), target.CurrentEvidence...)
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, evidence := range snapshot.Evidence {
			if evidence.ID != id {
				continue
			}
			if evidence.Level == "schema" && evidence.Client == "" && evidence.Scope == "" {
				filtered = append(filtered, id)
			} else if evidence.Client == target.Client &&
				(evidence.Scope == target.Scope || target.Scope == domain.ScopeUser && evidence.Scope == "") {
				filtered = append(filtered, id)
			}
			break
		}
	}
	return filtered
}
