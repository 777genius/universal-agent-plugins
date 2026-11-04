package domain

import (
	"errors"
	"regexp"
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
	// V1 targets retain scopes; V2 grants use one scoped identity per target.
	Scopes         []InstallScope            `json:"scopes,omitempty"`
	Scope          InstallScope              `json:"scope,omitempty"`
	MinimumInstallerVersion string          `json:"minimum_installer_version,omitempty"`
	CurrentEvidence []string                `json:"current_evidence,omitempty"`
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
	// V1 evidence has no scope and remains user-only. Project evidence in V2
	// must identify its scope explicitly to prevent cross-scope promotion.
	Scope              InstallScope              `json:"scope,omitempty"`
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
	Purpose            DirectoryResolvePurpose
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
	// DesiredReleaseSequence selects one exact release as new-install intent.
	// It is deliberately distinct from Recorded, which is proof of installed
	// state and may receive exact-operation exceptions.
	DesiredReleaseSequence uint64
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
