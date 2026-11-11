package usecase

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"golang.org/x/mod/semver"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func cloneCatalogEvidence(source *domain.CatalogEvidence) *domain.CatalogEvidence {
	if source == nil {
		return nil
	}
	result := *source
	result.CurrentEvidence = append([]domain.DirectoryEvidence(nil), source.CurrentEvidence...)
	if len(source.Compatibility) > 0 {
		result.Compatibility = make(map[string]domain.CatalogCompatibility, len(source.Compatibility))
		for client, compatibility := range source.Compatibility {
			compatibility.Evidence = append([]domain.DirectoryEvidence(nil), compatibility.Evidence...)
			if compatibility.EvidenceOutcomes != nil {
				compatibility.EvidenceOutcomes = make(map[string]string, len(compatibility.EvidenceOutcomes))
				for level, outcome := range source.Compatibility[client].EvidenceOutcomes {
					compatibility.EvidenceOutcomes[level] = outcome
				}
			}
			if compatibility.AppBinding != nil {
				binding := *compatibility.AppBinding
				compatibility.AppBinding = &binding
			}
			result.Compatibility[client] = compatibility
		}
	}
	return &result
}

func validatePackageTransition(installation domain.Installation, envelope domain.PackageEnvelope) error {
	if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil {
		// Directory update ordering is validated by validateDirectoryTransition;
		// author-controlled version strings are informational only.
		return nil
	}
	if installation.OriginMode == domain.OriginModeDirect && directLocalSource(installation.Source) {
		// An explicit local update is authorized by digest review, not SemVer.
		return nil
	}
	incomingVersion := envelope.Manifest.Version
	currentVersion := installation.Package.Version
	if incomingVersion != currentVersion {
		comparison, comparable := versionCompare(incomingVersion, currentVersion)
		if !comparable {
			return fmt.Errorf("refuse incomparable package version transition from %q to %q; use an explicit reviewed migration or rebind flow", currentVersion, incomingVersion)
		}
		if comparison < 0 {
			return fmt.Errorf("refuse package downgrade from %s to %s", currentVersion, incomingVersion)
		}
	}
	if incomingVersion != "" && currentVersion == incomingVersion && installation.Source.TreeDigest != "" && installation.Source.TreeDigest != envelope.TreeDigest {
		return fmt.Errorf("supply-chain conflict: version %s now has a different package digest", incomingVersion)
	}
	for _, client := range installation.Clients {
		revision := client.PackageRevision
		if revision == nil || revision.Version == "" || revision.Version != incomingVersion {
			continue
		}
		if revision.TreeDigest != "" && revision.TreeDigest != envelope.TreeDigest {
			return fmt.Errorf("supply-chain conflict: version %s differs from the revision applied to %s", incomingVersion, client.ClientID)
		}
	}
	return nil
}

func directLocalSource(source domain.SourceBinding) bool {
	for _, value := range []string{source.RequestedSource, source.CanonicalSource} {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if filepath.IsAbs(value) || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || value == "." || value == ".." {
			return true
		}
	}
	return false
}

func immutableDirectGit(source domain.SourceBinding) bool {
	value := source.RequestedSource + " " + source.CanonicalSource
	marker := strings.LastIndex(value, "@")
	if marker < 0 {
		return false
	}
	revision := value[marker+1:]
	if separator := strings.Index(revision, "//"); separator >= 0 {
		revision = revision[:separator]
	}
	revision = strings.TrimSpace(revision)
	if len(revision) != 40 {
		return false
	}
	for _, character := range revision {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func packageRevisionFromEnvelope(envelope domain.PackageEnvelope) *domain.ClientPackageRevision {
	return &domain.ClientPackageRevision{
		Skill:    domain.CloneSkillSourceFacts(envelope.Source.Skill),
		WebSkill: domain.CloneWebSkillSourceFacts(envelope.Source.WebSkill),
		Version:  envelope.Manifest.Version, ResolvedRevision: envelope.Source.ResolvedRevision,
		TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest,
		CatalogEvidence: cloneCatalogEvidence(envelope.CatalogEvidence),
	}
}

func packageRevisionForInput(input AddInput) *domain.ClientPackageRevision {
	revision := packageRevisionFromEnvelope(input.Envelope)
	if normalizedOriginMode(input.OriginMode) == domain.OriginModeDirectory && input.DirectoryResolution != nil {
		revision.DistributionID = input.DirectoryResolution.DistributionID
		revision.ReleaseSequence = input.DirectoryResolution.DesiredReleaseSequence
	}
	return revision
}

func packageRevisionMatches(revision *domain.ClientPackageRevision, envelope domain.PackageEnvelope) bool {
	return revision != nil && revision.TreeDigest == envelope.TreeDigest && revision.ManifestDigest == envelope.ManifestDigest &&
		reflect.DeepEqual(revision.WebSkill, envelope.Source.WebSkill) &&
		reflect.DeepEqual(revision.CatalogEvidence, envelope.CatalogEvidence)
}

// Directory repair must reproduce the immutable package revision and bytes
// while allowing compatibility evidence to be recomposed for the currently
// detected environment. Direct-source repair retains strict recorded evidence
// equality.
func repairPackageRevisionMatches(revision *domain.ClientPackageRevision, envelope domain.PackageEnvelope, directory bool) bool {
	if !directory {
		return packageRevisionMatches(revision, envelope)
	}
	return revision != nil && revision.ResolvedRevision == envelope.Source.ResolvedRevision && revision.TreeDigest == envelope.TreeDigest && revision.ManifestDigest == envelope.ManifestDigest
}
func normalizedOriginMode(mode domain.OriginMode) domain.OriginMode {
	if mode == "" {
		return domain.OriginModeDirect
	}
	return mode
}

func cloneDirectoryOrigin(source *domain.DirectoryOrigin) *domain.DirectoryOrigin {
	if source == nil {
		return nil
	}
	result := *source
	return &result
}

func validateOperationOrigin(mode domain.OriginMode, directory *domain.DirectoryOrigin) error {
	mode = normalizedOriginMode(mode)
	if mode == domain.OriginModeDirect {
		if directory != nil {
			return fmt.Errorf("direct origin cannot carry Directory authority")
		}
		return nil
	}
	if mode != domain.OriginModeDirectory || directory == nil {
		return fmt.Errorf("Directory origin metadata is required")
	}
	if directory.ProductID == "" || directory.DistributionID == "" || directory.DesiredReleaseSequence < 1 {
		return fmt.Errorf("Directory release identity is incomplete")
	}
	switch directory.DistributionKind {
	case domain.DistributionUpstream, domain.DistributionCommunityBridge, domain.DistributionCommunity:
	default:
		return fmt.Errorf("Directory distribution kind is invalid")
	}
	if directory.SnapshotSchema < 1 || directory.SnapshotSequence < 1 || directory.SnapshotDigest == "" {
		return fmt.Errorf("Directory operation requires an authorized signed snapshot identity")
	}
	return nil
}

func validateDirectoryTransition(installation domain.Installation, input AddInput) error {
	mode := normalizedOriginMode(input.OriginMode)
	if installation.OriginMode != mode {
		return fmt.Errorf("source origin change from %s to %s requires switch", installation.OriginMode, mode)
	}
	if mode != domain.OriginModeDirectory {
		return nil
	}
	if installation.Directory == nil || input.DirectoryResolution == nil {
		return fmt.Errorf("Directory lifecycle operation is missing immutable release provenance")
	}
	current, incoming := installation.Directory, input.DirectoryResolution
	// A verified release from a distinct signing lineage cannot silently
	// replace an installed release, even if product, revision and bytes match.
	// Require an explicit source switch instead of borrowing another feed's trust.
	if current.FeedID != incoming.FeedID {
		return fmt.Errorf("signed Directory feed change requires switch")
	}
	if current.ProductID != incoming.ProductID || current.DistributionID != incoming.DistributionID {
		return fmt.Errorf("distribution change requires switch")
	}
	if incoming.DesiredReleaseSequence < current.DesiredReleaseSequence {
		return fmt.Errorf("refuse Directory release downgrade from sequence %d to %d", current.DesiredReleaseSequence, incoming.DesiredReleaseSequence)
	}
	if incoming.DesiredReleaseSequence == current.DesiredReleaseSequence {
		if installation.Source.Repository != "" && installation.Source.Repository != input.Envelope.Source.Repository {
			return fmt.Errorf("signed Directory release sequence %d has conflicting package-source repository", incoming.DesiredReleaseSequence)
		}
		if installation.Source.PackageSubpath != "" && installation.Source.PackageSubpath != input.Envelope.Source.PackageSubpath {
			return fmt.Errorf("signed Directory release sequence %d has conflicting package-source path", incoming.DesiredReleaseSequence)
		}
		if installation.Source.ResolvedRevision != input.Envelope.Source.ResolvedRevision {
			return fmt.Errorf("signed Directory release sequence %d has conflicting package-source revision", incoming.DesiredReleaseSequence)
		}
		if installation.Source.TreeDigest != "" && installation.Source.TreeDigest != input.Envelope.TreeDigest {
			return fmt.Errorf("signed Directory release sequence %d has conflicting package bytes", incoming.DesiredReleaseSequence)
		}
		if installation.Package.ManifestDigest != "" && installation.Package.ManifestDigest != input.Envelope.ManifestDigest {
			return fmt.Errorf("signed Directory release sequence %d has conflicting manifest bytes", incoming.DesiredReleaseSequence)
		}
	}
	return nil
}
func versionCompare(left, right string) (int, bool) {
	normalize := func(value string) string {
		value = strings.TrimSpace(value)
		if value != "" && !strings.HasPrefix(value, "v") {
			value = "v" + value
		}
		return value
	}
	left, right = normalize(left), normalize(right)
	if !semver.IsValid(left) || !semver.IsValid(right) {
		return 0, false
	}
	return semver.Compare(left, right), true
}

func (service Service) guardEnvironmentRecovery() error {
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	// Managed environments recover explicitly; ordinary imperative recovery
	// may continue only when the complete pending scope is unmanaged.
	if err := transaction.ValidateEnvironmentRecoveryScope(state, nil); err != nil {
		return err
	}
	// An unmanaged in-memory planner may have no journal backend configured.
	// State ownership was checked above; managed operations still require a backend.
	if service.Kernel.Directory.JournalDir == "" && service.environmentAuthority == nil && len(state.EnvironmentReceipts) == 0 {
		return nil
	}
	journals, err := service.Kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
	for _, j := range journals {
		if j.EnvironmentID != "" {
			return fmt.Errorf("recovery_required: run deps recover")
		}
		for _, r := range state.EnvironmentReceipts {
			if r.ClientBindingID == j.ClientBindingID {
				return fmt.Errorf("recovery_required: run deps recover")
			}
		}
		if service.environmentAuthority != nil {
			return fmt.Errorf("recovery_required: pending journal requires explicit recovery")
		}
	}
	if service.environmentAuthority != nil {
		for _, r := range state.TransactionReceipts {
			if r.Phase != "" && r.Phase != "committed" {
				return fmt.Errorf("recovery_required: run deps recover")
			}
		}
	}
	return nil
}
