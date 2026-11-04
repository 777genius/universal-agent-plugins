package usecase

import (
	"context"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type Service struct {
	CheckNamespace       func() error `json:"-"`
	ProjectRoot          string       `json:"-"`
	environmentAuthority *domain.EnvironmentAuthority
	ClientPreparation    ClientPreparation
	// OpenCodeHosts retains the explicit legacy preparation seam for callers
	// without ClientPreparation; production composition uses ClientPreparation.
	OpenCodeHosts ports.OpenCodeHostPreparer
	// PrepareHostsForPreview authorizes online host preparation for a mutation's
	// plan-first pass. Offline callers leave this false, including CLI --dry-run.
	PrepareHostsForPreview bool
	PhysicalAuthority      ports.PhysicalProfileAuthority
	PhysicalProfiles       []domain.DetectedClient
	profileCheck           func() error
	StateStore             transaction.StateStore
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
	LegacyLock         ports.MutationLock
	Lock               ports.MutationLock
	Kernel             transaction.Kernel
	NativeObserver     NativeIdentityObserver
	NamespacePreflight MCPNamespacePreflight
	PluginData         PluginDataManager
	Now                func() time.Time
}

// ClientPreparation freezes explicit host authority before a desired native
// plan. Process-inert planning must never launch a host executable.
type ClientPreparation interface {
	PrepareClient(context.Context, domain.PackageEnvelope, domain.DetectedClient, []domain.NativeObjectOwnership, string, bool) (domain.OpenCodeHostAuthority, error)
	RevalidateClient(context.Context, domain.DetectedClient, domain.DeliveryPlan) error
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
	// refreshChangedProjectionOnly limits a refresh to a changed projected
	// digest. Update sets it: the update itself finishes any outstanding
	// activation, and a settled prepared state is not one.
	refreshChangedProjectionOnly bool
	InstallIntent                domain.InstallIntent
	Envelope                     domain.PackageEnvelope
	Client                       domain.DetectedClient
	Scope                        domain.InstallScope
	// OnlinePreview permits host probing during a read-only normal-operation preview.
	// It grants no mutation or confirmation permission.
	OnlinePreview      bool
	DryRun             bool
	Confirmed          bool
	Interactive        bool
	Hints              domain.CompatibilityHints
	InstallationID     string
	OperationID        string
	BackendExecutable  string
	ActivationComplete bool
	AuthComplete       bool
	// EnvironmentAuthority is mandatory for managed update/remove/repair and is
	// copied into the durable receipt before the first managed effect.
	EnvironmentAuthority *domain.EnvironmentAuthority
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
