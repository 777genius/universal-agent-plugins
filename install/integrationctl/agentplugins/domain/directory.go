package domain

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DirectorySnapshot is the authenticated schema-1 domain document. Transport,
// signature verification, time, and persistence deliberately live outside the
// domain package.
type DirectorySnapshot struct {
	SnapshotSchemaVersion int                     `json:"snapshot_schema_version"`
	Sequence              uint64                  `json:"sequence"`
	PublicationID         string                  `json:"publication_id"`
	SourceCommit          string                  `json:"source_commit"`
	GeneratedAt           string                  `json:"generated_at"`
	ExpiresAt             string                  `json:"expires_at"`
	Products              []DirectoryProduct      `json:"products"`
	Distributions         []DirectoryDistribution `json:"distributions"`
	Evidence              []DirectoryEvidence     `json:"evidence"`
	Revocations           []DirectoryRevocation   `json:"revocations"`
}

type DirectoryProduct struct {
	SchemaVersion       int                          `json:"schema_version"`
	ID                  string                       `json:"id"`
	DisplayName         string                       `json:"display_name"`
	Description         string                       `json:"description"`
	ManifestName        string                       `json:"manifest_name"`
	Aliases             []string                     `json:"aliases"`
	ReservedAliases     []string                     `json:"reserved_aliases"`
	Categories          []string                     `json:"categories"`
	Icon                *DirectoryIcon               `json:"icon,omitempty"`
	MinimumCapabilities DirectoryMinimumCapabilities `json:"minimum_capabilities"`
	DefaultDistribution string                       `json:"default_distribution"`
	Distributions       []string                     `json:"distributions"`
}

type DirectoryIcon struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type DirectoryMinimumCapabilities struct {
	Skills string `json:"skills"`
	MCP    string `json:"mcp"`
}

type DistributionStatus string
type ReleaseStatus string

const (
	DistributionActive    DistributionStatus = "active"
	DistributionSuspended DistributionStatus = "suspended"

	ReleaseActive     ReleaseStatus = "active"
	ReleaseSuperseded ReleaseStatus = "superseded"
	ReleaseRevoked    ReleaseStatus = "revoked"
)

type DirectoryDistribution struct {
	SchemaVersion   int                      `json:"schema_version"`
	ID              string                   `json:"id"`
	ProductID       string                   `json:"product_id"`
	Kind            DistributionKind         `json:"kind"`
	Status          DistributionStatus       `json:"status"`
	Packager        string                   `json:"packager"`
	Releases        []DirectoryRelease       `json:"releases"`
	ReleasePolicies []DirectoryReleasePolicy `json:"release_policies"`
}

type DirectoryRelease struct {
	Sequence            uint64                    `json:"sequence"`
	PackageVersion      string                    `json:"package_version"`
	ManifestName        string                    `json:"manifest_name"`
	AgentPluginsSchema  string                    `json:"agent_plugins_schema"`
	PackageSource       DirectorySource           `json:"package_source"`
	TreeDigestAlgorithm string                    `json:"tree_digest_algorithm"`
	BuildProvenance     *DirectoryBuildProvenance `json:"build_provenance,omitempty"`
	TreeDigest          string                    `json:"tree_digest"`
	ManifestDigest      string                    `json:"manifest_digest"`
	Components          []string                  `json:"components"`
	PublishedAt         string                    `json:"published_at"`
}

type DirectorySource struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Path       string `json:"path"`
}

type DirectoryBuildProvenance struct {
	UpstreamRepository string `json:"upstream_repository"`
	UpstreamRevision   string `json:"upstream_revision"`
}

type DirectoryReleasePolicy struct {
	ReleaseSequence         uint64            `json:"release_sequence"`
	Status                  ReleaseStatus     `json:"status"`
	MinimumInstallerVersion string            `json:"minimum_installer_version"`
	Targets                 []DirectoryTarget `json:"targets"`
	CurrentEvidence         []string          `json:"current_evidence"`
}

type DirectoryTarget struct {
	Client         ClientID                  `json:"client"`
	Scopes         []InstallScope            `json:"scopes"`
	Delivery       string                    `json:"delivery"`
	Authentication AuthenticationRequirement `json:"authentication"`
	AppBinding     *DirectoryAppBinding      `json:"app_binding,omitempty"`
}

// ExpectedDirectoryDelivery returns the signed Directory delivery contract for
// a logical client surface. Directory delivery describes the public packaging
// boundary, not the installer's internal PackageMode spelling.
func ExpectedDirectoryDelivery(client ClientID) (string, bool) {
	definition, ok := ClientDefinitionFor(client)
	return definition.DirectoryDelivery, ok && definition.DirectoryDelivery != ""
}

type DirectoryAppBinding struct {
	AppKey    string `json:"app_key"`
	ID        string `json:"id"`
	MCPServer string `json:"mcp_server"`
}

type DirectoryEvidence struct {
	SchemaVersion      int                       `json:"schema_version"`
	ID                 string                    `json:"id"`
	ProductID          string                    `json:"product_id,omitempty"`
	DistributionID     string                    `json:"distribution_id"`
	ReleaseSequence    uint64                    `json:"release_sequence"`
	PackageTreeDigest  string                    `json:"package_tree_digest"`
	ManifestDigest     string                    `json:"manifest_digest,omitempty"`
	SourceRepository   string                    `json:"source_repository,omitempty"`
	SourceRevision     string                    `json:"source_revision,omitempty"`
	SourcePath         string                    `json:"source_path,omitempty"`
	Level              string                    `json:"level"`
	Outcome            string                    `json:"outcome"`
	Client             ClientID                  `json:"client,omitempty"`
	ClientVersion      string                    `json:"client_version,omitempty"`
	InstallerVersion   string                    `json:"installer_version,omitempty"`
	AdapterVersion     string                    `json:"adapter_version,omitempty"`
	OS                 string                    `json:"os,omitempty"`
	Architecture       string                    `json:"architecture,omitempty"`
	DependencyIdentity string                    `json:"dependency_identity,omitempty"`
	ObservedAt         string                    `json:"observed_at,omitempty"`
	Artifact           DirectoryEvidenceArtifact `json:"artifact"`
	Trust              *DirectoryEvidenceTrust   `json:"trust,omitempty"`
}

const DirectoryEvidenceTrustCutoverSequence uint64 = 15

type DirectoryEvidenceTrust struct {
	Kind             string                     `json:"kind"`
	Workflow         string                     `json:"workflow,omitempty"`
	SourceRef        string                     `json:"source_ref,omitempty"`
	SourceDigest     string                     `json:"source_digest,omitempty"`
	BundleManifest   *DirectoryEvidenceArtifact `json:"bundle_manifest,omitempty"`
	LaunchArtifact   *DirectoryEvidenceArtifact `json:"launch_artifact,omitempty"`
	ObserverArtifact *DirectoryEvidenceArtifact `json:"observer_artifact,omitempty"`
	EvidenceIndex    *DirectoryEvidenceArtifact `json:"evidence_index,omitempty"`
}

type DirectoryEvidenceArtifact struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Path       string `json:"path"`
	Digest     string `json:"digest"`
}

var (
	directoryEvidenceWorkflowPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9._-]*/\.github/workflows/[A-Za-z0-9._-]+\.ya?ml$`)
	directoryEvidenceSourceRefPattern = regexp.MustCompile(`^refs/heads/[A-Za-z0-9._/-]+$`)
	directoryEvidenceRevisionPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// HasTrustedProvenance reports whether evidence carries one of the provenance
// forms recognized by Directory schema 1. GitHub Actions provenance is bound to
// the repository and revision containing the evidence artifact; an external
// provenance is trusted only when the signed Directory publisher marked it as
// reviewed and supplied no forged workflow fields.
func (e DirectoryEvidence) HasTrustedProvenance() bool {
	if e.Trust == nil {
		return false
	}
	switch e.Trust.Kind {
	case "github_actions":
		if !directoryEvidenceWorkflowPattern.MatchString(e.Trust.Workflow) ||
			!directoryEvidenceSourceRefPattern.MatchString(e.Trust.SourceRef) ||
			!directoryEvidenceRevisionPattern.MatchString(e.Trust.SourceDigest) {
			return false
		}
		workflowPrefix := e.Artifact.Repository + "/.github/workflows/"
		return strings.HasPrefix(e.Trust.Workflow, workflowPrefix) && e.Artifact.Revision == e.Trust.SourceDigest
	case "reviewed_external":
		return e.Trust.Workflow == "" && e.Trust.SourceRef == "" && e.Trust.SourceDigest == "" &&
			e.Trust.BundleManifest == nil && e.Trust.LaunchArtifact == nil && e.Trust.ObserverArtifact == nil && e.Trust.EvidenceIndex == nil
	default:
		return false
	}
}

// HasTrustedProvenanceAtSequence recognizes both immutable schema-1 evidence
// lanes. Sequences 1-14 used release-bound legacy identity fields; sequence 15
// and later use the explicit trust object. A populated trust object always uses
// the current path so domain-only fixtures cannot accidentally become legacy.
func (e DirectoryEvidence) HasTrustedProvenanceAtSequence(sequence uint64) bool {
	if e.Trust != nil {
		return e.HasTrustedProvenance()
	}
	return sequence > 0 && sequence < DirectoryEvidenceTrustCutoverSequence && e.ProductID != "" && e.ManifestDigest != "" &&
		e.SourceRepository != "" && e.SourceRevision != "" && e.SourcePath != ""
}

// HasTrustedEligibilityProvenance applies the schema-1 compatibility rule for
// evidence that can block or promote a release. Static schema gates require
// reproducible workflow provenance. Materialization and client runtime gates
// may also use exact evidence explicitly reviewed by the signed Directory
// publisher.
func (e DirectoryEvidence) HasTrustedEligibilityProvenance() bool {
	if !e.HasTrustedProvenance() {
		return false
	}
	if e.Level == "schema" {
		return e.Trust.Kind == "github_actions"
	}
	return e.Level == "materialization" || e.Level == "discovery" || e.Level == "runtime" || e.Level == "oauth"
}

// HasTrustedEligibilityProvenanceAtSequence applies the eligibility-level
// rules after recognizing the historical or current provenance lane.
func (e DirectoryEvidence) HasTrustedEligibilityProvenanceAtSequence(sequence uint64) bool {
	if !e.HasTrustedProvenanceAtSequence(sequence) {
		return false
	}
	if e.Level == "schema" {
		return e.Trust == nil || e.Trust.Kind == "github_actions"
	}
	return e.Level == "materialization" || e.Level == "discovery" || e.Level == "runtime" || e.Level == "oauth"
}

type DirectoryRevocation struct {
	DistributionID  string `json:"distribution_id"`
	ReleaseSequence uint64 `json:"release_sequence"`
}

type DirectoryOperation string

const (
	DirectoryInstall       DirectoryOperation = "install"
	DirectoryNewTarget     DirectoryOperation = "new_target"
	DirectoryUpdate        DirectoryOperation = "update"
	DirectoryRepair        DirectoryOperation = "repair"
	DirectoryRematerialize DirectoryOperation = "rematerialize"
	DirectoryRemove        DirectoryOperation = "remove"
	DirectoryReproduce     DirectoryOperation = "reproduce"
)

// RecordedDirectoryRelease binds a Directory tuple to every immutable source
// and package identity field retained by installed state. Empty optional fields
// mean that older or per-client state did not retain that field; populated
// fields must match the current signed snapshot exactly.
type RecordedDirectoryRelease struct {
	ProductID           string
	DistributionID      string
	ReleaseSequence     uint64
	Repository          string
	ResolvedRevision    string
	Path                string
	TreeDigestAlgorithm string
	TreeDigest          string
	ManifestDigest      string
}

type DirectoryResolveRequest struct {
	Selector           string
	Targets            []ClientID
	Scope              InstallScope
	InstallerVersion   string
	ClientVersions     map[ClientID]string
	OS                 string
	Architecture       string
	DependencyIdentity map[ClientID]string
	SchemaVersion      string
	RequiredComponents []string
	Operation          DirectoryOperation
	Recorded           *RecordedDirectoryRelease
}

type DirectoryDiagnostic struct {
	DistributionID string `json:"distribution_id,omitempty"`
	Code           string `json:"code"`
	Message        string `json:"message"`
}

type DirectorySelection struct {
	ProductID           string                `json:"product_id"`
	DistributionID      string                `json:"distribution_id"`
	DistributionKind    DistributionKind      `json:"distribution_kind"`
	ReleaseSequence     uint64                `json:"release_sequence"`
	PackageVersion      string                `json:"package_version"`
	Source              DirectorySource       `json:"source"`
	TreeDigestAlgorithm string                `json:"tree_digest_algorithm"`
	TreeDigest          string                `json:"tree_digest"`
	ManifestDigest      string                `json:"manifest_digest"`
	SnapshotSequence    uint64                `json:"snapshot_sequence"`
	Fallback            bool                  `json:"fallback"`
	Diagnostics         []DirectoryDiagnostic `json:"diagnostics,omitempty"`
}

var (
	ErrDirectoryNotFound     = errors.New("directory selector not found")
	ErrDirectoryAmbiguous    = errors.New("directory selector is ambiguous")
	ErrDirectoryIneligible   = errors.New("no eligible directory release")
	ErrDirectoryNoSafeUpdate = errors.New("no safe directory update")
)

// ResolveDirectory is deterministic and side-effect free. It selects one
// distribution and one release for the complete target set; acquisition
// failure is intentionally not a fallback input.
func ResolveDirectory(snapshot DirectorySnapshot, request DirectoryResolveRequest) (DirectorySelection, error) {
	selector := strings.TrimSpace(request.Selector)
	if selector == "" {
		return DirectorySelection{}, fmt.Errorf("%w: empty selector", ErrDirectoryNotFound)
	}
	if request.Scope == "" {
		request.Scope = ScopeUser
	}
	if request.Operation == "" {
		request.Operation = DirectoryInstall
	}
	product, qualified, err := findDirectoryProduct(snapshot, selector)
	if err != nil {
		return DirectorySelection{}, err
	}
	if request.Recorded != nil {
		if strings.TrimSpace(request.Recorded.ResolvedRevision) == "" {
			return DirectorySelection{}, fmt.Errorf("recorded Directory release %s sequence %d has no resolved package-source revision", request.Recorded.DistributionID, request.Recorded.ReleaseSequence)
		}
		if strings.TrimSpace(request.Recorded.ProductID) == "" || strings.TrimSpace(request.Recorded.DistributionID) == "" || request.Recorded.ReleaseSequence < 1 {
			return DirectorySelection{}, fmt.Errorf("recorded Directory release identity is incomplete")
		}
		if request.Recorded.ProductID != product.ID {
			return DirectorySelection{}, fmt.Errorf("recorded product %q does not match %q", request.Recorded.ProductID, product.ID)
		}
		qualified = request.Recorded.DistributionID
	}
	if qualified != "" {
		distribution := distributionByID(snapshot, product, qualified)
		if distribution == nil {
			return DirectorySelection{}, fmt.Errorf("%w: distribution %q", ErrDirectoryNotFound, qualified)
		}
		if err := validateRecordedDirectoryRelease(*distribution, request.Recorded); err != nil {
			return DirectorySelection{}, err
		}
		release, reasons := chooseDirectoryRelease(snapshot, product, *distribution, request)
		if release == nil {
			return DirectorySelection{}, ineligibleError(distribution.ID, reasons)
		}
		return selectionFrom(snapshot, product, *distribution, *release, false, nil), nil
	}
	diagnostics := []DirectoryDiagnostic{}
	for index, distribution := range orderedDistributions(snapshot, product) {
		release, reasons := chooseDirectoryRelease(snapshot, product, distribution, request)
		if release != nil {
			return selectionFrom(snapshot, product, distribution, *release, index > 0, diagnostics), nil
		}
		for _, reason := range reasons {
			diagnostics = append(diagnostics, DirectoryDiagnostic{DistributionID: distribution.ID, Code: reason.code, Message: reason.message})
		}
	}
	return DirectorySelection{}, fmt.Errorf("%w for %q: %s", ErrDirectoryIneligible, selector, joinDiagnosticMessages(diagnostics))
}

func validateRecordedDirectoryRelease(distribution DirectoryDistribution, recorded *RecordedDirectoryRelease) error {
	if recorded == nil {
		return nil
	}
	revision := strings.TrimSpace(recorded.ResolvedRevision)
	if revision == "" {
		return fmt.Errorf("recorded Directory release %s sequence %d has no resolved package-source revision", recorded.DistributionID, recorded.ReleaseSequence)
	}
	if !directoryEvidenceRevisionPattern.MatchString(revision) {
		return fmt.Errorf("recorded Directory release %s sequence %d has an invalid package-source revision; expected a full lowercase commit SHA", recorded.DistributionID, recorded.ReleaseSequence)
	}
	for _, release := range distribution.Releases {
		if release.Sequence != recorded.ReleaseSequence {
			continue
		}
		if release.PackageSource.Revision != revision {
			return fmt.Errorf("recorded Directory release %s sequence %d package-source revision does not match the current signed snapshot", recorded.DistributionID, recorded.ReleaseSequence)
		}
		checks := []struct {
			name     string
			recorded string
			current  string
		}{
			{name: "repository", recorded: recorded.Repository, current: release.PackageSource.Repository},
			{name: "path", recorded: recorded.Path, current: release.PackageSource.Path},
			{name: "tree digest algorithm", recorded: recorded.TreeDigestAlgorithm, current: release.TreeDigestAlgorithm},
			{name: "tree digest", recorded: recorded.TreeDigest, current: release.TreeDigest},
			{name: "manifest digest", recorded: recorded.ManifestDigest, current: release.ManifestDigest},
		}
		for _, check := range checks {
			if check.recorded != "" && check.recorded != check.current {
				return fmt.Errorf("recorded Directory release %s sequence %d package-source %s does not match the current signed snapshot", recorded.DistributionID, recorded.ReleaseSequence, check.name)
			}
		}
		return nil
	}
	return fmt.Errorf("%w: recorded release %s sequence %d", ErrDirectoryNotFound, recorded.DistributionID, recorded.ReleaseSequence)
}

type eligibilityReason struct{ code, message string }

func findDirectoryProduct(snapshot DirectorySnapshot, selector string) (DirectoryProduct, string, error) {
	if strings.Contains(selector, "/") {
		var match *DirectoryProduct
		for i := range snapshot.Products {
			if distributionByID(snapshot, snapshot.Products[i], selector) != nil {
				if match != nil {
					return DirectoryProduct{}, "", fmt.Errorf("%w: qualified distribution %q", ErrDirectoryAmbiguous, selector)
				}
				copy := snapshot.Products[i]
				match = &copy
			}
		}
		if match == nil {
			return DirectoryProduct{}, "", fmt.Errorf("%w: distribution %q", ErrDirectoryNotFound, selector)
		}
		return *match, selector, nil
	}
	matches := []DirectoryProduct{}
	for _, product := range snapshot.Products {
		if product.ID == selector || product.ManifestName == selector || containsString(product.Aliases, selector) {
			matches = append(matches, product)
		}
	}
	if len(matches) == 0 {
		return DirectoryProduct{}, "", fmt.Errorf("%w: %q", ErrDirectoryNotFound, selector)
	}
	if len(matches) != 1 {
		ids := make([]string, len(matches))
		for i := range matches {
			ids[i] = matches[i].ID
		}
		sort.Strings(ids)
		return DirectoryProduct{}, "", fmt.Errorf("%w: %q matches %s; use a qualified distribution ID", ErrDirectoryAmbiguous, selector, strings.Join(ids, ", "))
	}
	return matches[0], "", nil
}

func orderedDistributions(snapshot DirectorySnapshot, product DirectoryProduct) []DirectoryDistribution {
	result := []DirectoryDistribution{}
	seen := map[string]bool{}
	if item := distributionByID(snapshot, product, product.DefaultDistribution); item != nil {
		result = append(result, *item)
		seen[item.ID] = true
	}
	for _, kind := range []DistributionKind{DistributionUpstream, DistributionCommunityBridge, DistributionCommunity} {
		items := []DirectoryDistribution{}
		for _, id := range product.Distributions {
			if item := distributionByID(snapshot, product, id); item != nil && !seen[id] && item.Kind == kind {
				items = append(items, *item)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		for _, item := range items {
			result = append(result, item)
			seen[item.ID] = true
		}
	}
	return result
}

func chooseDirectoryRelease(snapshot DirectorySnapshot, product DirectoryProduct, distribution DirectoryDistribution, request DirectoryResolveRequest) (*DirectoryRelease, []eligibilityReason) {
	policies := map[uint64]DirectoryReleasePolicy{}
	for _, p := range distribution.ReleasePolicies {
		policies[p.ReleaseSequence] = p
	}
	releases := append([]DirectoryRelease(nil), distribution.Releases...)
	sort.SliceStable(releases, func(i, j int) bool { return releases[i].Sequence > releases[j].Sequence })
	reasons := []eligibilityReason{}
	for i := range releases {
		release := &releases[i]
		policy, ok := policies[release.Sequence]
		if !ok {
			continue
		}
		exactRecorded := request.Recorded != nil && release.Sequence == request.Recorded.ReleaseSequence
		// Every recorded operation except an explicit update stays on the exact
		// immutable release. In particular, re-adding a data-retained installation
		// is not an implicit update merely because a later release exists.
		if request.Recorded != nil && request.Operation != DirectoryUpdate && !exactRecorded {
			continue
		}
		if request.Operation == DirectoryUpdate && request.Recorded != nil && release.Sequence <= request.Recorded.ReleaseSequence {
			continue
		}
		if distribution.Status == DistributionSuspended && request.Operation != DirectoryRemove && request.Operation != DirectoryRepair && request.Operation != DirectoryRematerialize && request.Operation != DirectoryReproduce &&
			!(exactRecorded && (request.Operation == DirectoryInstall || request.Operation == DirectoryNewTarget)) {
			reasons = append(reasons, eligibilityReason{"distribution_suspended", "distribution is suspended for this operation"})
			continue
		}
		if request.Operation == DirectoryRemove && exactRecorded {
			return release, nil
		}
		if isRevoked(snapshot, distribution.ID, release.Sequence) || policy.Status == ReleaseRevoked {
			reasons = append(reasons, eligibilityReason{"release_revoked", fmt.Sprintf("release %d is revoked", release.Sequence)})
			continue
		}
		if policy.Status == ReleaseSuperseded && !exactRecorded {
			reasons = append(reasons, eligibilityReason{"release_superseded", fmt.Sprintf("release %d is historical only", release.Sequence)})
			continue
		}
		if policy.Status != ReleaseActive && policy.Status != ReleaseSuperseded {
			continue
		}
		if reason := releaseEligibility(snapshot, product, distribution, *release, policy, request); reason != nil {
			reasons = append(reasons, *reason)
			continue
		}
		return release, reasons
	}
	if request.Operation == DirectoryUpdate && request.Recorded != nil && len(reasons) == 0 {
		reasons = append(reasons, eligibilityReason{"no_safe_update", "no later eligible non-revoked release exists in the recorded distribution"})
	}
	return nil, reasons
}

func releaseEligibility(snapshot DirectorySnapshot, product DirectoryProduct, distribution DirectoryDistribution, release DirectoryRelease, policy DirectoryReleasePolicy, request DirectoryResolveRequest) *eligibilityReason {
	if release.ManifestName != product.ManifestName {
		return &eligibilityReason{"manifest_mismatch", "release manifest identity differs from product"}
	}
	if request.SchemaVersion != "" && request.SchemaVersion != "1.0.0" {
		return &eligibilityReason{"unsupported_schema", "required Agent Plugins schema is not supported"}
	}
	if release.AgentPluginsSchema != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" {
		return &eligibilityReason{"unsupported_schema", "release Agent Plugins schema is not supported"}
	}
	if policy.MinimumInstallerVersion != "" {
		comparison, err := compareSemver(request.InstallerVersion, policy.MinimumInstallerVersion)
		if err != nil || comparison < 0 {
			return &eligibilityReason{"installer_too_old", "minimum installer version is " + policy.MinimumInstallerVersion}
		}
	}
	for _, evidenceID := range policy.CurrentEvidence {
		if e := evidenceByID(snapshot, evidenceID); e != nil && e.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) && directoryEvidenceApplies(*e, distribution, release, "", request) && e.Level == "schema" && e.Outcome == "failed" {
			return &eligibilityReason{"blocking_evidence", "current trusted schema evidence failed"}
		}
	}
	for _, target := range directoryEligibilityTargets(request.Targets) {
		entry := targetByClient(policy.Targets, target)
		if entry == nil || !containsScope(entry.Scopes, request.Scope) {
			return &eligibilityReason{"incomplete_targets", "release does not support complete target set; missing " + string(target)}
		}
		delivery, supported := ExpectedDirectoryDelivery(target)
		if !supported || entry.Delivery != delivery {
			return &eligibilityReason{"incompatible_delivery", fmt.Sprintf("release delivery %q is incompatible with %s; expected %q", entry.Delivery, target, delivery)}
		}
		if distribution.Kind == DistributionUpstream && !hasPassedUpstreamMaterialization(snapshot, distribution, release, policy, target) {
			return &eligibilityReason{"upstream_materialization_required", "upstream release lacks current passed materialization evidence for " + string(target)}
		}
		for _, evidenceID := range policy.CurrentEvidence {
			if e := evidenceByID(snapshot, evidenceID); e != nil && e.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) && directoryEvidenceApplies(*e, distribution, release, target, request) && (e.Level == "materialization" || e.Level == "discovery" || e.Level == "runtime" || e.Level == "oauth") && e.Outcome == "failed" {
				return &eligibilityReason{"blocking_evidence", "current trusted " + e.Level + " evidence failed for " + string(target)}
			}
		}
	}
	required := append([]string(nil), request.RequiredComponents...)
	if product.MinimumCapabilities.Skills == "required" {
		required = append(required, "skills")
	}
	if product.MinimumCapabilities.MCP == "required" {
		required = append(required, "mcp")
	}
	for _, component := range required {
		if !containsString(release.Components, component) {
			return &eligibilityReason{"missing_component", "release lacks required component " + component}
		}
	}
	return nil
}

// Copilot CLI and VS Code are logical views of one physical native backend.
// Directory policy, evidence, and delivery must authorize both surfaces even
// when the user selected only one logical view.
func directoryEligibilityTargets(targets []ClientID) []ClientID {
	complete := append([]ClientID(nil), targets...)
	for _, target := range targets {
		if target == ClientCopilot || target == ClientVSCode {
			complete = append(complete, ClientCopilot, ClientVSCode)
			break
		}
	}
	return uniqueClients(complete)
}

// hasPassedUpstreamMaterialization is the static promotion/package gate. Its
// environment dimensions describe the isolated promotion run and must not be
// mistaken for evidence about the caller's local environment.
func hasPassedUpstreamMaterialization(snapshot DirectorySnapshot, distribution DirectoryDistribution, release DirectoryRelease, policy DirectoryReleasePolicy, client ClientID) bool {
	for _, evidenceID := range policy.CurrentEvidence {
		evidence := evidenceByID(snapshot, evidenceID)
		if evidence != nil && evidence.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) && evidence.DistributionID == distribution.ID && evidence.ReleaseSequence == release.Sequence &&
			evidence.PackageTreeDigest == release.TreeDigest && evidence.Client == client && evidence.Level == "materialization" && evidence.Outcome == "passed" {
			return true
		}
	}
	return false
}

func directoryEvidenceApplies(e DirectoryEvidence, distribution DirectoryDistribution, release DirectoryRelease, client ClientID, request DirectoryResolveRequest) bool {
	if e.DistributionID != distribution.ID || e.ReleaseSequence != release.Sequence || e.PackageTreeDigest != release.TreeDigest {
		return false
	}
	if e.Level == "schema" {
		return e.Client == ""
	}
	if e.Client != client || !evidenceDimensionMatches(e.InstallerVersion, request.InstallerVersion) ||
		!evidenceDimensionMatches(e.OS, request.OS) || !evidenceDimensionMatches(e.Architecture, request.Architecture) {
		return false
	}
	if e.ClientVersion != "" && request.ClientVersions[e.Client] != e.ClientVersion {
		return false
	}
	if e.DependencyIdentity != "" && request.DependencyIdentity[e.Client] != e.DependencyIdentity {
		return false
	}
	return true
}

func evidenceDimensionMatches(recorded, actual string) bool {
	return recorded == "" || recorded == actual
}

func selectionFrom(snapshot DirectorySnapshot, product DirectoryProduct, distribution DirectoryDistribution, release DirectoryRelease, fallback bool, diagnostics []DirectoryDiagnostic) DirectorySelection {
	return DirectorySelection{ProductID: product.ID, DistributionID: distribution.ID, DistributionKind: distribution.Kind, ReleaseSequence: release.Sequence, PackageVersion: release.PackageVersion, Source: release.PackageSource, TreeDigestAlgorithm: release.TreeDigestAlgorithm, TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest, SnapshotSequence: snapshot.Sequence, Fallback: fallback, Diagnostics: append([]DirectoryDiagnostic(nil), diagnostics...)}
}

func distributionByID(snapshot DirectorySnapshot, product DirectoryProduct, id string) *DirectoryDistribution {
	if !containsString(product.Distributions, id) {
		return nil
	}
	for i := range snapshot.Distributions {
		if snapshot.Distributions[i].ID == id && snapshot.Distributions[i].ProductID == product.ID {
			return &snapshot.Distributions[i]
		}
	}
	return nil
}
func targetByClient(values []DirectoryTarget, client ClientID) *DirectoryTarget {
	for i := range values {
		if values[i].Client == client {
			return &values[i]
		}
	}
	return nil
}
func evidenceByID(snapshot DirectorySnapshot, id string) *DirectoryEvidence {
	for i := range snapshot.Evidence {
		if snapshot.Evidence[i].ID == id {
			return &snapshot.Evidence[i]
		}
	}
	return nil
}
func isRevoked(snapshot DirectorySnapshot, distribution string, sequence uint64) bool {
	for _, item := range snapshot.Revocations {
		if item.DistributionID == distribution && item.ReleaseSequence == sequence {
			return true
		}
	}
	return false
}
func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func containsScope(values []InstallScope, value InstallScope) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func uniqueClients(values []ClientID) []ClientID {
	seen := map[ClientID]bool{}
	out := []ClientID{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
func ineligibleError(id string, reasons []eligibilityReason) error {
	parts := []string{}
	noSafeUpdate := false
	for _, r := range reasons {
		parts = append(parts, r.message)
		noSafeUpdate = noSafeUpdate || r.code == "no_safe_update"
	}
	if len(parts) == 0 {
		parts = []string{"no release matches the operation"}
	}
	if noSafeUpdate {
		return fmt.Errorf("%w: %w in %q: %s", ErrDirectoryIneligible, ErrDirectoryNoSafeUpdate, id, strings.Join(parts, "; "))
	}
	return fmt.Errorf("%w in %q: %s", ErrDirectoryIneligible, id, strings.Join(parts, "; "))
}
func joinDiagnosticMessages(values []DirectoryDiagnostic) string {
	parts := []string{}
	for _, v := range values {
		parts = append(parts, v.DistributionID+": "+v.Message)
	}
	return strings.Join(parts, "; ")
}

// compareSemver is used only for the installer compatibility floor. Directory
// release ordering always uses signed release sequence.
func compareSemver(left, right string) (int, error) {
	type version struct {
		core [3]uint64
		pre  []string
	}
	parse := func(value string) (version, error) {
		var v version
		value = strings.SplitN(strings.TrimSpace(value), "+", 2)[0]
		parts := strings.SplitN(value, "-", 2)
		core := strings.Split(parts[0], ".")
		if len(core) != 3 {
			return v, errors.New("not semver")
		}
		for i, s := range core {
			n, e := strconv.ParseUint(s, 10, 64)
			if e != nil || (len(s) > 1 && s[0] == '0') {
				return v, errors.New("not semver")
			}
			v.core[i] = n
		}
		if len(parts) == 2 {
			if parts[1] == "" {
				return v, errors.New("not semver")
			}
			v.pre = strings.Split(parts[1], ".")
		}
		return v, nil
	}
	a, e := parse(left)
	if e != nil {
		return 0, e
	}
	b, e := parse(right)
	if e != nil {
		return 0, e
	}
	for i := 0; i < 3; i++ {
		if a.core[i] < b.core[i] {
			return -1, nil
		}
		if a.core[i] > b.core[i] {
			return 1, nil
		}
	}
	if len(a.pre) == 0 && len(b.pre) > 0 {
		return 1, nil
	}
	if len(a.pre) > 0 && len(b.pre) == 0 {
		return -1, nil
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if a.pre[i] == b.pre[i] {
			continue
		}
		an, ae := strconv.ParseUint(a.pre[i], 10, 64)
		bn, be := strconv.ParseUint(b.pre[i], 10, 64)
		if ae == nil && be == nil {
			if an < bn {
				return -1, nil
			}
			return 1, nil
		}
		if ae == nil {
			return -1, nil
		}
		if be == nil {
			return 1, nil
		}
		if a.pre[i] < b.pre[i] {
			return -1, nil
		}
		return 1, nil
	}
	if len(a.pre) < len(b.pre) {
		return -1, nil
	}
	if len(a.pre) > len(b.pre) {
		return 1, nil
	}
	return 0, nil
}
