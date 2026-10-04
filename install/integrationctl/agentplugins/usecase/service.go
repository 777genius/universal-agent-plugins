package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type Service struct {
	PhysicalAuthority ports.PhysicalProfileAuthority
	PhysicalProfiles  []domain.DetectedClient
	profileCheck      func() error
	StateStore        transaction.StateStore
	// Paths is required. There is deliberately no default: a silently supplied
	// one would let a caller that forgot to wire it keep running with whatever
	// containment rules that default happened to carry.
	Paths   ports.PathPolicy
	Planner ports.DeliveryPlanner
	Targets ports.DeliveryTargetResolver
	// Detected is the surface map for this operation. The planner is stateless
	// about detection; the use case copies this onto every PlanRequest.
	Detected           map[domain.ClientID]domain.DetectedClient
	Stager             ports.PackageStager
	Activator          ports.ClientActivator
	Legacy             ports.LegacyLifecycle
	LegacyLock         legacyports.LockManager
	Lock               ports.MutationLock
	Kernel             transaction.Kernel
	NativeObserver     NativeIdentityObserver
	NamespacePreflight MCPNamespacePreflight
	PluginData         PluginDataManager
	Now                func() time.Time
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

// MCPNamespacePreflight is process-inert and checks the final selected MCP
// server names against a client's observable user-level configuration.
type MCPNamespacePreflight interface {
	CheckMCPNamespace(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) error
}

type PluginDataManager interface {
	EnsureData(context.Context, string, string, string) (domain.DataReceipt, bool, error)
	ValidateData(context.Context, domain.DataReceipt) error
	ValidateDataAt(context.Context, domain.DataReceipt, string) error
	PrepareRuntime(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string) error
	PurgeData(context.Context, domain.DataReceipt) error
}

type AddInput struct {
	refreshSelectedDelivery bool
	InstallIntent           domain.InstallIntent
	Envelope                domain.PackageEnvelope
	Client                  domain.DetectedClient
	Scope                   domain.InstallScope
	DryRun                  bool
	Confirmed               bool
	Interactive             bool
	Hints                   domain.CompatibilityHints
	InstallationID          string
	OperationID             string
	BackendExecutable       string
	ActivationComplete      bool
	AuthComplete            bool
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
	if err := session.resolveInstallation(); err != nil {
		return AddResult{}, err
	}
	var err error
	service, frozen, err := service.freezeProfiles(ctx, session.installationID, []domain.DetectedClient{session.input.Client}, true)
	if err != nil {
		return AddResult{}, err
	}
	session.service, session.input.Client = service, frozen[0]
	session.input.InstallationID = session.installationID
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
	if err := service.checkProfiles(ctx); err != nil {
		return nil, err
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	kernel.PhysicalAuthority = service.authorityPort()
	if err := kernel.PrevalidateRecovery(ctx); err != nil {
		return nil, err
	}
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
	if err := service.validateRecordedDeliveries(); err != nil {
		_ = release()
		return nil, err
	}
	if err := service.checkProfiles(ctx); err != nil {
		_ = release()
		return nil, err
	}
	kernel = service.Kernel
	kernel.StateStore = service.StateStore
	kernel.PhysicalAuthority = service.authorityPort()
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

func (service Service) authorityPort() ports.PhysicalProfileAuthority {
	if service.PhysicalAuthority != nil {
		return service.PhysicalAuthority
	}
	port, _ := service.Planner.(ports.PhysicalProfileAuthority)
	return port
}
func (service Service) checkProfiles(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if service.profileCheck != nil {
		return service.profileCheck()
	}
	for _, c := range service.PhysicalProfiles {
		if err := service.checkProfile(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
func (service Service) checkProfile(ctx context.Context, c domain.DetectedClient) error {
	token := domain.ProfileAuthority{}
	if c.ProfileAuthority != nil {
		token = *c.ProfileAuthority
		if token.IsZero() {
			return fmt.Errorf("opted profile token is missing")
		}
		if c.ProfileNamespace == "" || c.ProfileNamespace != service.Kernel.Namespace {
			return fmt.Errorf("physical profile namespace differs")
		}
	}
	port := service.authorityPort()
	if port == nil {
		if !token.IsZero() {
			return fmt.Errorf("physical profile verifier is required")
		}
		return nil
	}
	return port.RevalidateProfileAuthority(ctx, c.ClientID, token)
}
func (service Service) freezeProfiles(ctx context.Context, installationID string, selected []domain.DetectedClient, capture bool) (Service, []domain.DetectedClient, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return service, nil, err
	}
	frozen := append([]domain.DetectedClient(nil), selected...)
	opted := false
	var shared []domain.DetectedClient
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for key, b := range installation.Clients {
			if b.PhysicalArtifact != domain.ComputePhysicalArtifactID(installation.DeclaredName, installationID) {
				continue
			}
			if key != b.ClientBindingID {
				return service, nil, fmt.Errorf("shared data binding key differs")
			}
			c := domain.DetectedClient{ClientID: domain.ClientID(b.ClientID), ConfigRoot: b.NativeProfileRoot, ProfileAuthority: domain.CloneProfileAuthority(b.ProfileAuthority), ProfileNamespace: b.ProfileNamespace}
			if err := service.checkProfile(ctx, c); err != nil {
				return service, nil, err
			}
			shared = append(shared, c)
			opted = opted || c.ProfileAuthority != nil
		}
	}
	for i, c := range frozen {
		var recorded *domain.ClientBinding
		for _, installation := range state.Installations {
			if installation.InstallationID != installationID {
				continue
			}
			for key, b := range installation.Clients {
				if b.ClientID != string(c.ClientID) || b.Scope != string(domain.ScopeUser) {
					continue
				}
				if key != b.ClientBindingID || recorded != nil {
					return service, nil, fmt.Errorf("physical owner binding is ambiguous or malformed")
				}
				copy := b
				recorded = &copy
			}
		}
		if recorded != nil {
			if c.ProfileAuthority != nil && !domain.SameProfileAuthority(c.ProfileAuthority, recorded.ProfileAuthority) {
				return service, nil, fmt.Errorf("caller profile differs from recorded owner")
			}
			c.ProfileAuthority = domain.CloneProfileAuthority(recorded.ProfileAuthority)
			c.ProfileNamespace = recorded.ProfileNamespace
		} else if c.ProfileAuthority == nil && capture && service.authorityPort() != nil {
			token, err := service.authorityPort().CaptureProfileAuthority(ctx, c)
			if err != nil {
				return service, nil, err
			}
			if !token.IsZero() {
				c.ProfileAuthority = &token
				c.ProfileNamespace = service.Kernel.Namespace
			}
		}
		if c.ProfileAuthority != nil {
			root := c.ProfileAuthority.Facts().CanonicalRoot
			spelling, err := filepath.EvalSymlinks(c.ConfigRoot)
			if err != nil || spelling != root {
				return service, nil, fmt.Errorf("selected profile differs from recorded canonical root")
			}
			c.ConfigRoot = root
			opted = true
		}
		if err := service.checkProfile(ctx, c); err != nil {
			return service, nil, err
		}
		c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
		frozen[i] = c
	}
	if !opted {
		return service, frozen, nil
	}
	guarded := append([]domain.DetectedClient(nil), frozen...)
	for i := range guarded {
		guarded[i].ProfileAuthority = domain.CloneProfileAuthority(guarded[i].ProfileAuthority)
	}
	service.PhysicalProfiles = guarded
	service.PhysicalAuthority = service.authorityPort()
	base := service
	service.profileCheck = func() error {
		for _, c := range append(append([]domain.DetectedClient(nil), guarded...), shared...) {
			if err := base.checkProfile(ctx, c); err != nil {
				return err
			}
		}
		kernel := base.Kernel
		kernel.StateStore = base.StateStore
		kernel.PhysicalAuthority = base.authorityPort()
		return kernel.PrevalidateRecovery(ctx)
	}
	service.Kernel.PhysicalAuthority = service.PhysicalAuthority
	service.StateStore = profileStateStore{StateStore: service.StateStore, check: service.profileCheck}
	service.Targets = profileTargets{DeliveryTargetResolver: service.Targets, check: service.profileCheck}
	service.Planner = profilePlanner{DeliveryPlanner: service.Planner, profiles: guarded, check: service.profileCheck}
	service.Stager = profileStager{PackageStager: service.Stager, check: service.profileCheck}
	if service.PluginData != nil {
		service.PluginData = profileData{PluginDataManager: service.PluginData, check: service.profileCheck}
	}
	service.Activator = profileActivator{ClientActivator: service.Activator, check: service.profileCheck}
	if service.NativeObserver != nil {
		service.NativeObserver = profileObserver{NativeIdentityObserver: service.NativeObserver, check: service.profileCheck, fallback: base.filesystemIdentityObservation}
	}
	return service, frozen, nil
}

type profileObserver struct {
	NativeIdentityObserver
	check    func() error
	fallback func(context.Context, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error)
}

func (o profileObserver) ObserveNativeIdentity(ctx context.Context, c domain.DetectedClient, p domain.DeliveryPlan, b *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if err := o.check(); err != nil {
		return domain.NativeIdentityObservation{}, err
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	result, err := o.NativeIdentityObserver.ObserveNativeIdentity(ctx, c, p, b)
	if guardErr := o.check(); guardErr != nil {
		return domain.NativeIdentityObservation{}, guardErr
	}
	return result, err
}
func (o profileObserver) ObservePreparedIdentity(ctx context.Context, c domain.DetectedClient, p domain.DeliveryPlan, b *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if err := o.check(); err != nil {
		return domain.NativeIdentityObservation{}, err
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	var result domain.NativeIdentityObservation
	var err error
	if prepared, ok := o.NativeIdentityObserver.(PreparedIdentityObserver); ok {
		result, err = prepared.ObservePreparedIdentity(ctx, c, p, b)
	} else {
		result, err = o.fallback(ctx, p, b)
	}
	if guardErr := o.check(); guardErr != nil {
		return domain.NativeIdentityObservation{}, guardErr
	}
	return result, err
}

type profileStateStore struct {
	transaction.StateStore
	check func() error
}

func (s profileStateStore) Save(state domain.StateFileV2) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.StateStore.Save(state)
}
func (s profileStateStore) RequireMutationReady() error {
	if guard, ok := s.StateStore.(interface{ RequireMutationReady() error }); ok {
		return guard.RequireMutationReady()
	}
	return nil
}

type profilePlanner struct {
	ports.DeliveryPlanner
	profiles []domain.DetectedClient
	check    func() error
}

func (p profilePlanner) Plan(ctx context.Context, r domain.PlanRequest) (domain.DeliveryPlan, error) {
	if err := p.check(); err != nil {
		return domain.DeliveryPlan{}, err
	}
	for _, c := range p.profiles {
		if c.ClientID == r.Client.ClientID {
			r.Client.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
			r.Client.ProfileNamespace = c.ProfileNamespace
		}
	}
	authority, namespace := domain.CloneProfileAuthority(r.Client.ProfileAuthority), r.Client.ProfileNamespace
	detected := make(map[domain.ClientID]domain.DetectedClient, len(r.Detected))
	for id, c := range r.Detected {
		c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
		detected[id] = c
	}
	r.Detected = detected
	plan, err := p.DeliveryPlanner.Plan(ctx, r)
	if guardErr := p.check(); guardErr != nil {
		return domain.DeliveryPlan{}, guardErr
	}
	return plan.WithProfileAuthority(authority, namespace), err
}

type profileTargets struct {
	ports.DeliveryTargetResolver
	check func() error
}

func (p profileTargets) ResolveTarget(ctx context.Context, c domain.DetectedClient, scope domain.InstallScope, artifact string) (domain.DeliveryTarget, error) {
	if err := p.check(); err != nil {
		return domain.DeliveryTarget{}, err
	}
	authority, namespace := domain.CloneProfileAuthority(c.ProfileAuthority), c.ProfileNamespace
	c.ProfileAuthority = domain.CloneProfileAuthority(authority)
	target, err := p.DeliveryTargetResolver.ResolveTarget(ctx, c, scope, artifact)
	if guardErr := p.check(); guardErr != nil {
		return domain.DeliveryTarget{}, guardErr
	}
	return target.WithProfileAuthority(authority, namespace), err
}

type profileStager struct {
	ports.PackageStager
	check func() error
}

func (s profileStager) Stage(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, id string, h domain.CompatibilityHints) (domain.StagedDelivery, error) {
	if err := s.check(); err != nil {
		return domain.StagedDelivery{}, err
	}
	return s.PackageStager.Stage(ctx, e, p, id, h)
}
func (s profileStager) Discard(ctx context.Context, d domain.StagedDelivery) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.PackageStager.Discard(ctx, d)
}
func (s profileStager) Verify(ctx context.Context, path, digest string) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.PackageStager.Verify(ctx, path, digest)
}
func (s profileStager) StageWithPluginData(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, id string, h domain.CompatibilityHints, data string) (domain.StagedDelivery, error) {
	if err := s.check(); err != nil {
		return domain.StagedDelivery{}, err
	}
	inner, ok := s.PackageStager.(ports.PluginDataAwareStager)
	if !ok {
		return domain.StagedDelivery{}, fmt.Errorf("plugin data stager is missing")
	}
	return inner.StageWithPluginData(ctx, e, p, id, h, data)
}
func (s profileStager) ProjectActiveNative(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, path, data string) ([]domain.NativeObjectOwnership, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	inner, ok := s.PackageStager.(ports.ActiveNativeProjector)
	if !ok {
		return nil, fmt.Errorf("native projector is missing")
	}
	return inner.ProjectActiveNative(ctx, e, p, path, data)
}
func (s profileStager) PreflightManagedStdio(command string) error {
	if err := s.check(); err != nil {
		return err
	}
	if inner, ok := s.PackageStager.(ports.ManagedStdioPreflighter); ok {
		return inner.PreflightManagedStdio(command)
	}
	return nil
}
func (s profileStager) ManagedMCPNames(ctx context.Context, id domain.ClientID, path, digest string) ([]string, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if inner, ok := s.PackageStager.(managedSelectionReader); ok {
		return inner.ManagedMCPNames(ctx, id, path, digest)
	}
	return nil, nil
}

type profileData struct {
	PluginDataManager
	check func() error
}

func (d profileData) EnsureData(ctx context.Context, id, backend, scope string) (domain.DataReceipt, bool, error) {
	if err := d.check(); err != nil {
		return domain.DataReceipt{}, false, err
	}
	return d.PluginDataManager.EnsureData(ctx, id, backend, scope)
}
func (d profileData) PrepareRuntime(ctx context.Context, e domain.PackageEnvelope, p domain.DeliveryPlan, path string) error {
	if err := d.check(); err != nil {
		return err
	}
	return d.PluginDataManager.PrepareRuntime(ctx, e, p, path)
}
func (d profileData) PurgeData(ctx context.Context, r domain.DataReceipt) error {
	if err := d.check(); err != nil {
		return err
	}
	return d.PluginDataManager.PurgeData(ctx, r)
}
func (d profileData) PreflightDataPath(path string) (string, bool, error) {
	if err := d.check(); err != nil {
		return "", false, err
	}
	if inner, ok := d.PluginDataManager.(ports.DataPathPreflighter); ok {
		return inner.PreflightDataPath(path)
	}
	return "", false, fmt.Errorf("plugin data preflight is missing")
}

type profileActivator struct {
	ports.ClientActivator
	check func() error
}

func (a profileActivator) Activate(ctx context.Context, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := a.check(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	r.Client.ProfileAuthority = domain.CloneProfileAuthority(r.Client.ProfileAuthority)
	return a.ClientActivator.Activate(ctx, r)
}
func (a profileActivator) Deactivate(ctx context.Context, r domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if err := a.check(); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	r.Client.ProfileAuthority = domain.CloneProfileAuthority(r.Client.ProfileAuthority)
	return a.ClientActivator.Deactivate(ctx, r)
}
func (a profileActivator) PreflightActivation(r domain.ActivationRequest) error {
	if err := a.check(); err != nil {
		return err
	}
	if inner, ok := a.ClientActivator.(ports.ActivationPreflighter); ok {
		r.Client.ProfileAuthority = domain.CloneProfileAuthority(r.Client.ProfileAuthority)
		if err := inner.PreflightActivation(r); err != nil {
			return err
		}
		return a.check()
	}
	return nil
}
func (a profileActivator) VerifierAvailable(c domain.DetectedClient, p domain.DeliveryPlan, exe string) bool {
	inner, ok := a.ClientActivator.(ports.ActivationVerifierClassifier)
	if !ok || a.check() != nil {
		return false
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	available := inner.VerifierAvailable(c, p, exe)
	return a.check() == nil && available
}

func (service Service) dataProfileOwners(dataID string) []domain.PhysicalProfileOwner {
	state, err := service.StateStore.Load()
	if err != nil {
		return nil
	}
	var owners []domain.PhysicalProfileOwner
	for _, i := range state.Installations {
		for key, b := range i.Clients {
			if b.DataReceiptID == dataID && b.ProfileAuthority != nil {
				owners = append(owners, domain.PhysicalProfileOwner{Namespace: b.ProfileNamespace, InstallationID: i.InstallationID, ClientID: b.ClientID, ClientBindingID: key, Authority: domain.CloneProfileAuthority(b.ProfileAuthority)})
			}
		}
	}
	return owners
}
