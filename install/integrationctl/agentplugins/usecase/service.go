package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"golang.org/x/mod/semver"
)

type Service struct {
	StateStore transaction.StateStore
	// Paths is required. There is deliberately no default: a silently supplied
	// one would let a caller that forgot to wire it keep running with whatever
	// containment rules that default happened to carry.
	Paths          ports.PathPolicy
	Planner        ports.DeliveryPlanner
	Targets        ports.DeliveryTargetResolver
	Stager         ports.PackageStager
	Activator      ports.ClientActivator
	Legacy         ports.LegacyLifecycle
	LegacyLock     legacyports.LockManager
	Lock           ports.MutationLock
	Kernel         transaction.Kernel
	NativeObserver NativeIdentityObserver
	PluginData     PluginDataManager
	Now            func() time.Time
}

type NativeIdentityState = domain.NativeIdentityState
type NativeIdentityObservation = domain.NativeIdentityObservation

const (
	NativeIdentityAbsent        = domain.NativeIdentityAbsent
	NativeIdentityManaged       = domain.NativeIdentityManaged
	NativeIdentityUnmanaged     = domain.NativeIdentityUnmanaged
	NativeIdentityIndeterminate = domain.NativeIdentityIndeterminate
)

// NativeIdentityObserver lets a backend prove that a same-name native object
// is absent or already owned. Unmanaged and indeterminate observations are
// always blocking; there is deliberately no adoption result.
type NativeIdentityObserver interface {
	ObserveNativeIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error)
}

// PreparedIdentityObserver is the process-inert subset used by dry-run. It may
// inspect package files and prepared registries, but must not launch a client
// executable or query a remote registry.
type PreparedIdentityObserver interface {
	ObservePreparedIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error)
}

type PluginDataManager interface {
	EnsureData(context.Context, string, string, string) (domain.DataReceipt, bool, error)
	ValidateData(context.Context, domain.DataReceipt) error
	PurgeData(context.Context, domain.DataReceipt) error
}

type AddInput struct {
	InstallIntent      domain.InstallIntent
	Envelope           domain.PackageEnvelope
	Client             domain.DetectedClient
	Scope              domain.InstallScope
	DryRun             bool
	Confirmed          bool
	Interactive        bool
	Hints              domain.CompatibilityHints
	InstallationID     string
	OperationID        string
	BackendExecutable  string
	ActivationComplete bool
	AuthComplete       bool
	// OriginMode and DirectoryResolution are supplied by the resolver. Omitting
	// OriginMode is treated as an explicit direct source for compatibility with
	// exact/local callers; Directory authority is never inferred from a name.
	OriginMode            domain.OriginMode
	DirectoryResolution   *domain.DirectoryOrigin
	DistributionSuspended bool
	ReleaseRevoked        bool
	// PersistAuthoritativeObservations allows a read-only client verifier to
	// record negative evidence even during a plan-first CLI pass. It does not
	// authorize package, client, or user-requested lifecycle mutations.
	PersistAuthoritativeObservations bool
}

type AddResult struct {
	InstallationID       string                   `json:"installation_id"`
	Plan                 domain.DeliveryPlan      `json:"plan"`
	Activation           domain.ActivationOutcome `json:"activation,omitempty"`
	RequiresConfirmation bool                     `json:"requires_confirmation"`
	Mutated              bool                     `json:"mutated"`
	NoChange             bool                     `json:"no_change,omitempty"`
	Receipt              domain.MutationReceipt   `json:"-"`
	GroupPhase           GroupTargetPhase         `json:"group_phase,omitempty"`
	Failure              *GroupTargetFailure      `json:"failure,omitempty"`
}

func (service Service) Add(ctx context.Context, input AddInput) (AddResult, error) {
	return service.apply(ctx, input, false)
}

func (service Service) Update(ctx context.Context, input AddInput) (AddResult, error) {
	return service.apply(ctx, input, true)
}

func (service Service) apply(ctx context.Context, input AddInput, replace bool) (AddResult, error) {
	session := &applySession{service: service, ctx: ctx, input: input, replace: replace}
	if err := session.validateApplyInput(); err != nil {
		return AddResult{}, err
	}
	release, err := service.beginMutation(ctx, session.input.DryRun, session.input.Confirmed)
	if err != nil {
		return AddResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := session.resolveInstallation(); err != nil {
		return AddResult{}, err
	}
	if err := session.validateApplyTransitions(); err != nil {
		return AddResult{}, err
	}
	if err := session.planAndPreflight(); err != nil {
		return session.result, err
	}
	if err := session.resolveBinding(); err != nil {
		return session.result, err
	}
	if session.input.DryRun {
		return session.dryRunPath()
	}
	done, result, err := session.noChangeOrResume()
	if done {
		return result, err
	}
	return session.stageAndCommit()
}

func (service Service) stagePackage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, dataPath string) (domain.StagedDelivery, error) {
	if strings.TrimSpace(dataPath) == "" {
		return service.Stager.Stage(ctx, envelope, plan, operationID, hints)
	}
	aware, ok := service.Stager.(ports.PluginDataAwareStager)
	if !ok {
		return domain.StagedDelivery{}, fmt.Errorf("package stager cannot bind the owned PLUGIN_DATA locator")
	}
	return aware.StageWithPluginData(ctx, envelope, plan, operationID, hints, dataPath)
}

func (service Service) beginMutation(ctx context.Context, dryRun, confirmed bool) (ports.UnlockFunc, error) {
	if dryRun || !confirmed {
		return nil, nil
	}
	if service.Lock == nil {
		return nil, fmt.Errorf("agentplugins mutation lock is required")
	}
	release, err := service.Lock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if guard, ok := service.StateStore.(interface{ RequireMutationReady() error }); ok {
		if err := guard.RequireMutationReady(); err != nil {
			_ = release()
			return nil, err
		}
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	if err := kernel.Recover(ctx); err != nil {
		_ = release()
		return nil, fmt.Errorf("recover interrupted mutation: %w", err)
	}
	return release, nil
}

func (service Service) now() time.Time {
	if service.Now != nil {
		return service.Now().UTC()
	}
	return time.Now().UTC()
}

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
		Version: envelope.Manifest.Version, ResolvedRevision: envelope.Source.ResolvedRevision,
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
