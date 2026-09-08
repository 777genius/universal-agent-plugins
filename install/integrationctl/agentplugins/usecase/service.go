package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"golang.org/x/mod/semver"
)

type Service struct {
	StateStore     transaction.StateStore
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

type pluginDataAwareStager interface {
	StageWithPluginData(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string, domain.CompatibilityHints, string) (domain.StagedDelivery, error)
}

type AddInput struct {
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
}

func (service Service) Add(ctx context.Context, input AddInput) (AddResult, error) {
	return service.apply(ctx, input, false)
}

func (service Service) Update(ctx context.Context, input AddInput) (AddResult, error) {
	return service.apply(ctx, input, true)
}

func (service Service) apply(ctx context.Context, input AddInput, replace bool) (AddResult, error) {
	if input.OriginMode == "" && input.DirectoryResolution != nil {
		input.OriginMode = domain.OriginModeDirectory
	}
	if err := validateOperationOrigin(input.OriginMode, input.DirectoryResolution); err != nil {
		return AddResult{}, err
	}
	if service.StateStore == nil || service.Planner == nil || service.Stager == nil || service.Activator == nil {
		return AddResult{}, fmt.Errorf("agentplugins service dependencies are incomplete")
	}
	if input.Envelope.LoaderKind != domain.LoaderKindAgentPlugins {
		return AddResult{}, fmt.Errorf("agentplugins add accepts only standard Agent Plugins packages")
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return AddResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return AddResult{}, err
	}
	sourceBindingID := domain.ComputeSourceBindingID(input.Envelope.Source)
	installationIndex, existing, stickyMatch, err := findStickyInstallation(state, input, sourceBindingID)
	if err != nil {
		return AddResult{}, err
	}
	installationID := strings.TrimSpace(input.InstallationID)
	if existing {
		installation := state.Installations[installationIndex]
		if installation.NeedsRebind {
			return AddResult{}, fmt.Errorf("installation %s requires explicit rebind", installation.InstallationID)
		}
		if input.Envelope.Manifest.Name != installation.DeclaredName {
			return AddResult{}, fmt.Errorf("refuse package identity change from %q to %q; remove all targets and use explicit rebind", installation.DeclaredName, input.Envelope.Manifest.Name)
		}
		if installation.Package.LoaderKind != input.Envelope.LoaderKind || installation.Package.FormatID != input.Envelope.FormatID {
			return AddResult{}, fmt.Errorf("source is bound to a different package format; use migrate-format")
		}
		installationID = installation.InstallationID
		if stickyMatch && installation.Source.SourceBindingID != sourceBindingID {
			sameDistribution := installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && input.DirectoryResolution != nil && installation.Directory.DistributionID == input.DirectoryResolution.DistributionID
			if !sameDistribution {
				return AddResult{}, fmt.Errorf("installation %s is sticky to %s; use switch to change source", installation.InstallationID, installation.Source.CanonicalSource)
			}
		}
	}
	if installationID == "" {
		installationID, err = domain.NewInstallationID()
		if err != nil {
			return AddResult{}, err
		}
	}
	if replace && !existing {
		return AddResult{}, fmt.Errorf("update source is not bound to an existing installation; use add or rebind")
	}
	// Validate identity/release transitions before emitting the more general
	// "use update" diagnostic. A same-release digest conflict is a supply-chain
	// failure regardless of whether the caller happened to be adding a target.
	if existing {
		if err := validatePackageTransition(state.Installations[installationIndex], input.Envelope); err != nil {
			return AddResult{}, err
		}
		if err := validateDirectoryTransition(state.Installations[installationIndex], input); err != nil {
			return AddResult{}, err
		}
	}
	if existing && !replace && state.Installations[installationIndex].OriginMode == domain.OriginModeDirectory && state.Installations[installationIndex].Directory != nil && input.DirectoryResolution != nil && input.DirectoryResolution.DesiredReleaseSequence != state.Installations[installationIndex].Directory.DesiredReleaseSequence {
		return AddResult{}, fmt.Errorf("adding a target must use recorded release sequence %d; run update separately", state.Installations[installationIndex].Directory.DesiredReleaseSequence)
	}
	if existing && !replace && state.Installations[installationIndex].Source.TreeDigest != input.Envelope.TreeDigest {
		return AddResult{}, fmt.Errorf("adding a target must use the recorded desired package bytes; run update separately")
	}
	if replace && existing && state.Installations[installationIndex].OriginMode == domain.OriginModeDirect && immutableDirectGit(state.Installations[installationIndex].Source) {
		return AddResult{}, fmt.Errorf("direct full-SHA installations have no update channel; use switch --to with a new full SHA")
	}
	if input.ReleaseRevoked && normalizedOriginMode(input.OriginMode) == domain.OriginModeDirectory {
		return AddResult{}, fmt.Errorf("release is revoked; new exposure, update, and repair are blocked while removal remains available")
	}
	if input.DistributionSuspended && normalizedOriginMode(input.OriginMode) == domain.OriginModeDirectory {
		if !existing || replace {
			return AddResult{}, fmt.Errorf("distribution is suspended; new installs, new targets, and updates are blocked")
		}
	}
	physicalID := domain.ComputePhysicalArtifactID(input.Envelope.Manifest.Name, installationID)
	plan, err := service.Planner.Plan(ctx, input.Envelope, input.Client, input.Scope, physicalID)
	if err != nil {
		return AddResult{}, err
	}
	if openAIOAuthApplies(input.Client.ClientID, input.Envelope, input.Hints) {
		plan.Authentication = domain.AuthenticationPending
	}
	result := AddResult{InstallationID: installationID, Plan: plan}
	if input.ReleaseRevoked && normalizedOriginMode(input.OriginMode) == domain.OriginModeDirect {
		plan.Warnings = append(plan.Warnings, "direct_source_digest_matches_known_revoked_directory_release")
		result.Plan = plan
	}
	if plan.Status == domain.PlanUnsupported {
		action := strings.Join(plan.UserActions, "; ")
		if action != "" {
			return result, fmt.Errorf("delivery plan for %s is unsupported. Next: %s", plan.ClientID, action)
		}
		return result, fmt.Errorf("delivery plan for %s is unsupported", plan.ClientID)
	}
	if err := service.preflightActivation(input, plan); err != nil {
		return result, err
	}
	if err := service.preflightTargetComponents(ctx, input, &plan, installationIfExisting(state, installationIndex, existing), false, replace); err != nil {
		result.Plan = plan
		return result, err
	}
	result.Plan = plan
	if err := rejectNativeNameCollision(state, installationID, input.Envelope.Manifest.Name, input.Client.ClientID); err != nil {
		return result, err
	}
	clientBindingID := domain.ComputeClientBindingID(installationID, string(input.Client.ClientID), string(input.Scope), plan.ActivePath)
	if existing && sameNativeBackend(input.Client.ClientID, domain.ClientCopilot) {
		for key, binding := range state.Installations[installationIndex].Clients {
			if binding.Scope != string(input.Scope) || binding.Materialization == domain.MaterializationAbsent || binding.PhysicalArtifact != plan.PhysicalArtifactID || !sameNativeBackend(domain.ClientID(binding.ClientID), input.Client.ClientID) {
				continue
			}
			clientBindingID = key
			plan.ActivePath = binding.TargetLocator
			plan.TargetRoot = filepath.Dir(binding.TargetLocator)
			result.Plan = plan
			break
		}
	}
	isMaterialized := existing && materializedClient(state.Installations[installationIndex], clientBindingID)
	var managedBinding *domain.ClientBinding
	if isMaterialized {
		binding := state.Installations[installationIndex].Clients[clientBindingID]
		managedBinding = &binding
	}
	if replace {
		describeMCPRemovals(&plan, managedBinding)
		result.Plan = plan
	}
	if input.DistributionSuspended && normalizedOriginMode(input.OriginMode) == domain.OriginModeDirectory && !isMaterialized {
		return result, fmt.Errorf("distribution is suspended; adding a target is blocked")
	}
	if !isMaterialized && (input.ActivationComplete || input.AuthComplete) {
		return result, fmt.Errorf("lifecycle completion flags require an already materialized package; run add first, then rerun add with the completion flags")
	}
	if input.DryRun {
		if err := service.observePreparedIdentity(ctx, input.Client, plan, managedBinding); err != nil {
			return result, err
		}
		if replace && !isMaterialized {
			return result, fmt.Errorf("plugin is not materialized for %s; use add", input.Client.ClientID)
		}
		if isMaterialized {
			current := state.Installations[installationIndex].Clients[clientBindingID]
			result.Activation = lifecycleOutcome(current)
			if err := service.verifyManagedTarget(ctx, input.Client, input.Scope, current, "dry-run"); err != nil {
				return result, err
			}
			if !replace && !packageRevisionMatches(current.PackageRevision, input.Envelope) {
				return result, fmt.Errorf("plugin is already materialized for %s at a different revision; use update", input.Client.ClientID)
			}
			result.NoChange = !replace && lifecycleConverged(current)
		}
		return result, nil
	}
	if managedBinding == nil {
		if err := service.observeNativeIdentity(ctx, input.Client, plan, nil); err != nil {
			return result, err
		}
	}
	if isMaterialized && !replace {
		current := state.Installations[installationIndex].Clients[clientBindingID]
		result.Activation = lifecycleOutcome(current)
		if !packageRevisionMatches(current.PackageRevision, input.Envelope) {
			return result, fmt.Errorf("plugin is already materialized for %s at a different revision; use update", input.Client.ClientID)
		}
		if lifecycleConverged(current) {
			if err := service.verifyManagedTarget(ctx, input.Client, input.Scope, current, "no-change check"); err != nil {
				return result, err
			}
			verified, verifyErr := service.verifyClientReadOnly(ctx, input, result, current)
			if verifyErr != nil {
				if verified.Activation != "" && !input.DryRun && (input.Confirmed || input.PersistAuthoritativeObservations && verified.AuthoritativeObservation) {
					result.Activation = verified
					changed, updateErr := service.persistAuthoritativeObservation(ctx, input, installationID, clientBindingID, current, verified)
					result.Mutated = changed
					if updateErr != nil {
						return result, fmt.Errorf("client verification failed: %v; persist negative verification evidence: %w", verifyErr, updateErr)
					}
				}
				return result, verifyErr
			}
			if verified.Activation != "" && !sameLifecycleOutcome(verified, lifecycleOutcome(current)) {
				return service.persistObservedLifecycle(input, result, installationID, clientBindingID, verified)
			}
			result.NoChange = true
			return result, nil
		}
		return service.resume(ctx, input, result, installationID, clientBindingID, current)
	}
	if replace && !isMaterialized {
		return result, fmt.Errorf("plugin is not materialized for %s; use add", input.Client.ClientID)
	}
	if replace {
		current := state.Installations[installationIndex]
		previousClient := current.Clients[clientBindingID]
		result.Activation = lifecycleOutcome(previousClient)
		if err := service.verifyManagedTarget(ctx, input.Client, input.Scope, previousClient, "update"); err != nil {
			return result, err
		}
		if err := service.observeNativeIdentity(ctx, input.Client, plan, managedBinding); err != nil {
			return result, err
		}
		if !requiresComponentRemoval(plan) && packageRevisionMatches(previousClient.PackageRevision, input.Envelope) && previousClient.PackageRevision.ResolvedRevision == input.Envelope.Source.ResolvedRevision {
			if lifecycleConverged(previousClient) {
				verified, verifyErr := service.verifyClientReadOnly(ctx, input, result, previousClient)
				if verifyErr != nil {
					if verified.Activation != "" && !input.DryRun && (input.Confirmed || input.PersistAuthoritativeObservations && verified.AuthoritativeObservation) {
						result.Activation = verified
						changed, updateErr := service.persistAuthoritativeObservation(ctx, input, installationID, clientBindingID, previousClient, verified)
						result.Mutated = changed
						if updateErr != nil {
							return result, fmt.Errorf("client verification failed: %v; persist negative verification evidence: %w", verifyErr, updateErr)
						}
					}
					return result, verifyErr
				}
				if verified.Activation != "" && !sameLifecycleOutcome(verified, lifecycleOutcome(previousClient)) {
					return service.persistObservedLifecycle(input, result, installationID, clientBindingID, verified)
				}
				result.NoChange = true
				return result, nil
			}
			return service.resume(ctx, input, result, installationID, clientBindingID, previousClient)
		}
	}
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	operationID := strings.TrimSpace(input.OperationID)
	if operationID == "" {
		operationID, err = newOperationID()
		if err != nil {
			return result, err
		}
	}
	var dataReceipt domain.DataReceipt
	dataCreated := false
	if packageNeedsPluginData(input.Envelope, plan) {
		if service.PluginData == nil {
			return result, fmt.Errorf("PLUGIN_DATA manager is required for stdio MCP packages")
		}
		dataReceipt, dataCreated, err = service.PluginData.EnsureData(ctx, installationID, plan.PhysicalArtifactID, string(input.Scope))
		if err != nil {
			return result, err
		}
	}
	delivery, err := service.stagePackage(ctx, input.Envelope, plan, operationID, input.Hints, dataReceipt.Locator)
	if err != nil {
		if dataCreated {
			_ = service.PluginData.PurgeData(context.Background(), dataReceipt)
		}
		return result, err
	}
	delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, plan, managedBinding)
	if err != nil {
		_ = service.Stager.Discard(context.Background(), delivery)
		if dataCreated {
			_ = service.PluginData.PurgeData(context.Background(), dataReceipt)
		}
		return result, err
	}
	keepCreatedData := false
	defer func() {
		if dataCreated && !keepCreatedData {
			_ = service.PluginData.PurgeData(context.Background(), dataReceipt)
		}
	}()
	if err := service.observeNativeIdentity(ctx, input.Client, plan, managedBinding); err != nil {
		_ = service.Stager.Discard(context.Background(), delivery)
		return result, fmt.Errorf("native identity changed before commit: %w", err)
	}
	previousClient := domain.ClientBinding{}
	if existing {
		previousClient = state.Installations[installationIndex].Clients[clientBindingID]
	}
	state, installationIndex = upsertPreparedInstallation(state, installationIndex, existing, input, plan, installationID, clientBindingID, sourceBindingID, service.now())
	if dataReceipt.DataReceiptID == "" {
		for _, retained := range state.Installations[installationIndex].DataReceipts {
			if retained.PhysicalBackend == plan.PhysicalArtifactID && retained.Scope == string(input.Scope) {
				dataReceipt = retained
				break
			}
		}
	}
	if dataReceipt.DataReceiptID != "" {
		installation := state.Installations[installationIndex]
		if installation.DataReceipts == nil {
			installation.DataReceipts = map[string]domain.DataReceipt{}
		}
		installation.DataReceipts[dataReceipt.DataReceiptID] = dataReceipt
		client := installation.Clients[clientBindingID]
		client.DataReceiptID = dataReceipt.DataReceiptID
		installation.Clients[clientBindingID] = client
		installation.DataRetained = false
		state.Installations[installationIndex] = installation
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	initialActivation, initialVerification := initialLifecycle(plan)
	receipt, applyErr := kernel.ApplyDirectory(ctx, transaction.DirectoryMutation{
		OperationID:     operationID,
		InstallationID:  installationID,
		ClientBindingID: clientBindingID,
		Sequence:        nextSequence(state.Installations[installationIndex].Clients[clientBindingID]),
		OwnedBase:       delivery.OwnedBase,
		ActivePath:      delivery.ActivePath,
		StagingPath:     delivery.StagingPath,
		BeforeDigest:    managedDigest(previousClient),
		AfterDigest:     delivery.ArtifactDigest,
		NativeObjects:   delivery.NativeObjects,
		Activation:      initialActivation,
		Authentication:  plan.Authentication,
		Policy:          domain.PolicyAllowed,
		Verification:    initialVerification,
		DesiredState:    state,
		Verify: func(verifyContext context.Context, activePath string) error {
			return service.Stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
		},
	})
	result.Receipt = receipt
	if applyErr != nil {
		_ = service.Stager.Discard(context.Background(), delivery)
		return result, applyErr
	}
	keepCreatedData = true
	result.Mutated = true
	outcome, activationErr := service.Activator.Activate(ctx, domain.ActivationRequest{
		Client: input.Client, Plan: plan, Delivery: domain.StagedDelivery{
			ClientID: delivery.ClientID, OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath,
			ArtifactDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects,
		},
		DeclaredName: input.Envelope.Manifest.Name, Replacing: replace,
		Interactive: input.Interactive, BackendExecutable: input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), previousClient.NativeObjects...),
	})
	result.Activation = outcome
	if activationErr != nil && outcome.Activation == "" {
		outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
		}
		result.Activation = outcome
	}
	if _, updateErr := service.updateActivationResult(installationID, clientBindingID, outcome, activationErr, previousClient.NativeObjects); updateErr != nil {
		if activationErr != nil {
			return result, fmt.Errorf("activate client: %v; persist activation state: %w", activationErr, updateErr)
		}
		return result, updateErr
	}
	if activationErr != nil {
		return result, activationErr
	}
	return result, nil
}

func (service Service) stagePackage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, dataPath string) (domain.StagedDelivery, error) {
	if strings.TrimSpace(dataPath) == "" {
		return service.Stager.Stage(ctx, envelope, plan, operationID, hints)
	}
	aware, ok := service.Stager.(pluginDataAwareStager)
	if !ok {
		return domain.StagedDelivery{}, fmt.Errorf("package stager cannot bind the owned PLUGIN_DATA locator")
	}
	return aware.StageWithPluginData(ctx, envelope, plan, operationID, hints, dataPath)
}

func lifecycleConverged(client domain.ClientBinding) bool {
	authComplete := client.Authentication == domain.AuthenticationNotRequired || client.Authentication == domain.AuthenticationComplete
	return client.Materialization == domain.MaterializationMaterialized &&
		client.Activation == domain.ActivationActive &&
		client.Verification == domain.VerificationInstalled && authComplete
}

// resume retries only the external client lifecycle against an existing,
// digest-verified managed package. It deliberately creates no directory
// transaction or receipt, so repeating add cannot replace or duplicate the
// materialized artifact.
func (service Service) resume(
	ctx context.Context,
	input AddInput,
	result AddResult,
	installationID, clientBindingID string,
	client domain.ClientBinding,
) (AddResult, error) {
	if input.DryRun {
		return result, nil
	}
	if err := service.verifyManagedTarget(ctx, input.Client, input.Scope, client, "resume"); err != nil {
		return result, err
	}
	result.Activation = lifecycleOutcome(client)
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	delivery := domain.StagedDelivery{
		ClientID: input.Client.ClientID, OwnedBase: result.Plan.TargetRoot,
		ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client),
		NativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
	}
	outcome, activationErr := service.Activator.Activate(ctx, domain.ActivationRequest{
		Client: input.Client, Plan: result.Plan, Delivery: delivery,
		DeclaredName: input.Envelope.Manifest.Name, Replacing: true,
		Interactive: input.Interactive, BackendExecutable: input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
		VerifyOnly:            true, ActivationComplete: input.ActivationComplete,
	})
	// Authentication completion is a separate phase. Client installation/list
	// evidence must never silently complete it.
	if activationErr == nil && !clientVerifierAvailable(input, result.Plan) && client.Activation == domain.ActivationActive && client.Verification == domain.VerificationInstalled {
		outcome.Activation = client.Activation
		outcome.Verification = client.Verification
	}
	if (client.Authentication == domain.AuthenticationPending || client.Authentication == domain.AuthenticationNotChecked) && input.AuthComplete {
		outcome.Authentication = domain.AuthenticationComplete
		outcome.AuthenticationAttested = true
	} else if client.Authentication != "" {
		outcome.Authentication = client.Authentication
	}
	result.Activation = outcome
	if activationErr != nil && outcome.Activation == "" {
		outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: result.Plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
		}
		result.Activation = outcome
	}
	changed, updateErr := service.updateLifecycle(installationID, clientBindingID, outcome)
	result.Mutated = changed
	if updateErr != nil {
		if activationErr != nil {
			return result, fmt.Errorf("resume client activation: %v; persist activation state: %w", activationErr, updateErr)
		}
		return result, updateErr
	}
	if activationErr != nil {
		return result, activationErr
	}
	return result, nil
}

func clientVerifierAvailable(input AddInput, plan domain.DeliveryPlan) bool {
	switch input.Client.ClientID {
	case domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf:
		// These clients expose an exact, read-only native configuration verifier.
		// It is intentionally independent of an optional client executable.
		return len(plan.Components) > 0
	}
	if strings.TrimSpace(input.BackendExecutable) == "" {
		return false
	}
	switch input.Client.ClientID {
	case domain.ClientCodex, domain.ClientClaude, domain.ClientCopilot, domain.ClientVSCode:
		return true
	case domain.ClientKiro:
		if !strings.Contains(strings.ToLower(input.BackendExecutable), "kiro") || len(plan.Components) == 0 {
			return false
		}
		for _, component := range plan.Components {
			if component.Support == domain.SupportUnsupported {
				continue
			}
			if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (service Service) persistObservedLifecycle(input AddInput, result AddResult, installationID, clientBindingID string, outcome domain.ActivationOutcome) (AddResult, error) {
	result.Activation = outcome
	if input.DryRun {
		return result, nil
	}
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	changed, err := service.updateLifecycle(installationID, clientBindingID, outcome)
	result.Mutated = changed
	return result, err
}

func (service Service) persistAuthoritativeObservation(ctx context.Context, input AddInput, installationID, clientBindingID string, observed domain.ClientBinding, outcome domain.ActivationOutcome) (bool, error) {
	if input.Confirmed {
		return service.updateLifecycle(installationID, clientBindingID, outcome)
	}
	release, err := service.beginMutation(ctx, false, true)
	if err != nil {
		return false, err
	}
	defer func() { _ = release() }()
	state, err := service.StateStore.Load()
	if err != nil {
		return false, err
	}
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		latest, ok := installation.Clients[clientBindingID]
		if !ok || !reflect.DeepEqual(latest, observed) {
			return false, fmt.Errorf("stale client binding observation; state changed before negative evidence could be persisted, retry the command")
		}
		return service.updateLifecycle(installationID, clientBindingID, outcome)
	}
	return false, fmt.Errorf("stale client binding observation; installation changed before negative evidence could be persisted, retry the command")
}

func openAIOAuthApplies(clientID domain.ClientID, envelope domain.PackageEnvelope, hints domain.CompatibilityHints) bool {
	if clientID != domain.ClientCodex {
		return false
	}
	if envelope.CatalogEvidence != nil {
		if _, present := envelope.CatalogEvidence.Compatibility[string(clientID)]; present {
			return false
		}
	}
	if _, present := hints.Compatibility[string(clientID)]; present {
		return false
	}
	for serverName := range envelope.MCP.Servers {
		if strings.TrimSpace(hints.OpenAIMCPAuth[serverName].OAuthResource) != "" {
			return true
		}
	}
	return false
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

func (service Service) updateLifecycle(installationID, clientBindingID string, outcome domain.ActivationOutcome) (bool, error) {
	return service.updateLifecycleAndNativeObjects(installationID, clientBindingID, outcome, nil, false)
}

func (service Service) updateActivationResult(installationID, clientBindingID string, outcome domain.ActivationOutcome, activationErr error, previousNativeObjects []domain.NativeObjectOwnership) (bool, error) {
	if activationErr == nil {
		return service.updateLifecycle(installationID, clientBindingID, outcome)
	}
	prior := append([]domain.NativeObjectOwnership(nil), previousNativeObjects...)
	return service.updateLifecycleAndNativeObjects(installationID, clientBindingID, outcome, &prior, true)
}

func (service Service) updateLifecycleAndNativeObjects(installationID, clientBindingID string, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership, preserveCommittedPackage bool) (bool, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return false, err
	}
	for installationIndex, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[clientBindingID]
		if !ok {
			return false, fmt.Errorf("client binding disappeared during activation")
		}
		if nativeObjects != nil && preserveCommittedPackage {
			reconciledCapacity, capacityErr := checkedCombinedCapacity(len(client.NativeObjects), len(*nativeObjects))
			if capacityErr != nil {
				return false, fmt.Errorf("reconcile committed native objects: %w", capacityErr)
			}
			reconciled := make([]domain.NativeObjectOwnership, 0, reconciledCapacity)
			for _, object := range client.NativeObjects {
				if object.Kind == "managed_package_directory" {
					reconciled = append(reconciled, object)
				}
			}
			for _, object := range *nativeObjects {
				if object.Kind != "managed_package_directory" {
					reconciled = append(reconciled, object)
				}
			}
			nativeObjects = &reconciled
		}
		lifecycleUnchanged := client.Activation == outcome.Activation && client.Authentication == outcome.Authentication &&
			client.Policy == outcome.Policy && client.Verification == outcome.Verification
		nativeObjectsUnchanged := nativeObjects == nil || reflect.DeepEqual(client.NativeObjects, *nativeObjects)
		if lifecycleUnchanged && nativeObjectsUnchanged {
			return false, nil
		}
		client.Activation = outcome.Activation
		client.Authentication = outcome.Authentication
		client.Policy = outcome.Policy
		client.Verification = outcome.Verification
		if nativeObjects != nil {
			client.NativeObjects = append([]domain.NativeObjectOwnership(nil), (*nativeObjects)...)
		}
		client.UpdatedAt = service.now().Format(time.RFC3339Nano)
		installation.Clients[clientBindingID] = client
		installation.UpdatedAt = client.UpdatedAt
		state.Installations[installationIndex] = installation
		if err := service.StateStore.Save(state); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, fmt.Errorf("installation disappeared during activation")
}

func lifecycleOutcome(client domain.ClientBinding) domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: client.Activation, Authentication: client.Authentication, Policy: client.Policy, Verification: client.Verification}
}

func (service Service) verifyClientReadOnly(ctx context.Context, input AddInput, result AddResult, client domain.ClientBinding) (domain.ActivationOutcome, error) {
	switch input.Client.ClientID {
	case domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf:
		// Native config and skill ownership are observable without starting the
		// client. Do not let an absent executable turn a stale native projection
		// into a successful no-change result.
	case domain.ClientCodex, domain.ClientClaude, domain.ClientCopilot, domain.ClientVSCode:
		if strings.TrimSpace(input.BackendExecutable) == "" {
			return domain.ActivationOutcome{}, nil
		}
	case domain.ClientKiro:
		if !strings.Contains(strings.ToLower(input.BackendExecutable), "kiro") {
			return domain.ActivationOutcome{}, nil
		}
	default:
		return domain.ActivationOutcome{}, nil
	}
	delivery := domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: result.Plan.TargetRoot, ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client), NativeObjects: client.NativeObjects}
	outcome, err := service.Activator.Activate(ctx, domain.ActivationRequest{Client: input.Client, Plan: result.Plan, Delivery: delivery, DeclaredName: input.Envelope.Manifest.Name, Replacing: true, BackendExecutable: input.BackendExecutable, PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...), VerifyOnly: true})
	outcome = preserveManagedAuthentication(outcome, client.Authentication)
	return outcome, err
}

func preserveManagedAuthentication(outcome domain.ActivationOutcome, current domain.AuthenticationState) domain.ActivationOutcome {
	if current == domain.AuthenticationComplete || outcome.Authentication == "" || outcome.Authentication == domain.AuthenticationNotChecked {
		outcome.Authentication = current
	}
	return outcome
}

func (service Service) now() time.Time {
	if service.Now != nil {
		return service.Now().UTC()
	}
	return time.Now().UTC()
}

func upsertPreparedInstallation(
	state domain.StateFileV2,
	installationIndex int,
	existing bool,
	input AddInput,
	plan domain.DeliveryPlan,
	installationID, clientBindingID, sourceBindingID string,
	now time.Time,
) (domain.StateFileV2, int) {
	timestamp := now.Format(time.RFC3339Nano)
	installation := domain.Installation{
		InstallationID: installationID,
		DeclaredName:   input.Envelope.Manifest.Name,
		Source: domain.SourceBinding{
			SourceBindingID:  sourceBindingID,
			RequestedSource:  input.Envelope.Source.RequestedSource,
			CanonicalSource:  input.Envelope.Source.CanonicalSource,
			Repository:       input.Envelope.Source.Repository,
			PackageSubpath:   input.Envelope.Source.PackageSubpath,
			ResolvedRevision: input.Envelope.Source.ResolvedRevision,
			TreeDigest:       input.Envelope.TreeDigest,
		},
		Package: domain.PackageBinding{
			LoaderKind: input.Envelope.LoaderKind, FormatID: input.Envelope.FormatID,
			SchemaURI: input.Envelope.SchemaURI, DeclaredName: input.Envelope.Manifest.Name,
			Version: input.Envelope.Manifest.Version, ManifestDigest: input.Envelope.ManifestDigest,
			Inventory: input.Envelope.Inventory,
		},
		OriginMode: normalizedOriginMode(input.OriginMode),
		Directory:  cloneDirectoryOrigin(input.DirectoryResolution),
		Clients:    map[string]domain.ClientBinding{},
		CreatedAt:  timestamp,
		UpdatedAt:  timestamp,
	}
	if existing {
		installation = state.Installations[installationIndex]
		installation.DeclaredName = input.Envelope.Manifest.Name
		installation.Source.ResolvedRevision = input.Envelope.Source.ResolvedRevision
		installation.Source.TreeDigest = input.Envelope.TreeDigest
		if installation.OriginMode == domain.OriginModeDirectory {
			installation.Source = domain.SourceBinding{SourceBindingID: sourceBindingID, RequestedSource: input.Envelope.Source.RequestedSource,
				CanonicalSource: input.Envelope.Source.CanonicalSource, Repository: input.Envelope.Source.Repository, PackageSubpath: input.Envelope.Source.PackageSubpath,
				ResolvedRevision: input.Envelope.Source.ResolvedRevision, TreeDigest: input.Envelope.TreeDigest}
		}
		installation.Package = domain.PackageBinding{
			LoaderKind: input.Envelope.LoaderKind, FormatID: input.Envelope.FormatID,
			SchemaURI: input.Envelope.SchemaURI, DeclaredName: input.Envelope.Manifest.Name,
			Version: input.Envelope.Manifest.Version, ManifestDigest: input.Envelope.ManifestDigest,
			Inventory: input.Envelope.Inventory,
		}
		installation.UpdatedAt = timestamp
		if installation.OriginMode == domain.OriginModeDirectory && input.DirectoryResolution != nil {
			installation.Directory = cloneDirectoryOrigin(input.DirectoryResolution)
		}
		if installation.Clients == nil {
			installation.Clients = map[string]domain.ClientBinding{}
		}
	}
	previousClient := installation.Clients[clientBindingID]
	bindingClientID := string(input.Client.ClientID)
	if previousClient.ClientID != "" {
		bindingClientID = previousClient.ClientID
	}
	installation.Clients[clientBindingID] = domain.ClientBinding{
		ClientBindingID: clientBindingID, ClientID: bindingClientID, Scope: string(input.Scope),
		TargetLocator: plan.ActivePath, PhysicalArtifact: plan.PhysicalArtifactID,
		Materialization: domain.MaterializationStaged, Activation: domain.ActivationPrepared,
		Authentication: plan.Authentication, Policy: domain.PolicyAllowed,
		Verification: domain.VerificationPackageValid, UpdatedAt: timestamp,
		PackageRevision:  packageRevisionForInput(input),
		Receipts:         append([]domain.MutationReceipt(nil), previousClient.Receipts...),
		NativeObjects:    append([]domain.NativeObjectOwnership(nil), previousClient.NativeObjects...),
		AffectedSurfaces: preparedAffectedSurfaces(previousClient, input.Client.ClientID),
	}
	if existing {
		state.Installations[installationIndex] = installation
		return state, installationIndex
	}
	state.Installations = append(state.Installations, installation)
	return state, len(state.Installations) - 1
}

func preparedAffectedSurfaces(previous domain.ClientBinding, requested domain.ClientID) []string {
	values := append([]string(nil), previous.AffectedSurfaces...)
	if previous.ClientID != "" {
		values = append(values, previous.ClientID)
	}
	values = append(values, string(requested))
	if sameNativeBackend(requested, domain.ClientCopilot) {
		values = append(values, string(domain.ClientCopilot), string(domain.ClientVSCode))
	}
	return uniqueSortedSurfaces(values)
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

func findSourceInstallation(state domain.StateFileV2, sourceBindingID string) (int, bool) {
	for index, installation := range state.Installations {
		if installation.Source.SourceBindingID == sourceBindingID {
			return index, true
		}
	}
	return -1, false
}

func findStickyInstallation(state domain.StateFileV2, input AddInput, sourceBindingID string) (int, bool, bool, error) {
	if selector := strings.TrimSpace(input.InstallationID); selector != "" {
		for index, installation := range state.Installations {
			if installation.InstallationID == selector {
				return index, true, true, nil
			}
		}
		// An explicit, unused installation ID denotes a new installation. Do not
		// silently reinterpret it as a request to mutate a same-name binding;
		// backend collision checks below will fail closed with the useful native
		// identity diagnostic.
		return -1, false, false, nil
	}
	if input.DirectoryResolution != nil && input.DirectoryResolution.ProductID != "" {
		for index, installation := range state.Installations {
			if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && installation.Directory.ProductID == input.DirectoryResolution.ProductID {
				return index, true, true, nil
			}
		}
	}
	nameMatch := -1
	for index, installation := range state.Installations {
		if installation.Source.SourceBindingID == sourceBindingID {
			return index, true, false, nil
		}
		if installation.DeclaredName == input.Envelope.Manifest.Name {
			if nameMatch >= 0 {
				return -1, false, false, fmt.Errorf("installed manifest identity %q is ambiguous; use installation_id", input.Envelope.Manifest.Name)
			}
			nameMatch = index
		}
	}
	if nameMatch >= 0 {
		return nameMatch, true, true, nil
	}
	return -1, false, false, nil
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

func materializedClient(installation domain.Installation, clientBindingID string) bool {
	client, ok := installation.Clients[clientBindingID]
	return ok && client.Materialization != domain.MaterializationAbsent
}

func rejectNativeNameCollision(state domain.StateFileV2, installationID, declaredName string, clientID domain.ClientID) error {
	for _, installation := range state.Installations {
		if installation.InstallationID == installationID || installation.DeclaredName != declaredName {
			continue
		}
		for _, client := range installation.Clients {
			boundClientID := domain.ClientID(client.ClientID)
			if sameNativeBackend(boundClientID, clientID) && client.Materialization != domain.MaterializationAbsent {
				return fmt.Errorf("native client name collision for %q on %s; remove or rebind the existing source first", declaredName, clientID)
			}
		}
	}
	return nil
}

func sameNativeBackend(first, second domain.ClientID) bool {
	return domain.SameClientBackend(first, second)
}

// bindStagedDeliveryToPhysicalOwner keeps a shared backend's immutable
// ownership receipt bound to the client identity that originally created the
// physical binding. The requested logical surface remains on the delivery and
// plan so activation and user-facing results still describe what the caller
// selected.
func bindStagedDeliveryToPhysicalOwner(delivery domain.StagedDelivery, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.StagedDelivery, error) {
	if !sameNativeBackend(plan.ClientID, domain.ClientCopilot) {
		return delivery, nil
	}
	owner := plan.ClientID
	if managed != nil {
		owner = domain.ClientID(managed.ClientID)
		if !sameNativeBackend(owner, plan.ClientID) || managed.PhysicalArtifact != plan.PhysicalArtifactID {
			return delivery, fmt.Errorf("shared physical binding identity does not match the staged delivery")
		}
	}
	expected := "package:" + string(plan.ClientID) + ":" + plan.PhysicalArtifactID
	canonical := "package:" + string(owner) + ":" + plan.PhysicalArtifactID
	found := false
	for index := range delivery.NativeObjects {
		object := &delivery.NativeObjects[index]
		if object.Kind != "managed_package_directory" {
			continue
		}
		if found || object.ObjectID != expected {
			return delivery, fmt.Errorf("shared staged delivery has an invalid managed package ownership identity")
		}
		object.ObjectID = canonical
		found = true
	}
	if !found {
		return delivery, fmt.Errorf("shared staged delivery is missing its managed package ownership identity")
	}
	return delivery, nil
}

func initialLifecycle(plan domain.DeliveryPlan) (domain.ActivationState, domain.VerificationState) {
	return plan.Activation, plan.Verification
}

func nextSequence(client domain.ClientBinding) int {
	maximum := 0
	for _, receipt := range client.Receipts {
		if receipt.Sequence > maximum {
			maximum = receipt.Sequence
		}
	}
	return maximum + 1
}

func managedDigest(client domain.ClientBinding) string {
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" && object.ManagedDigest != "" {
			return object.ManagedDigest
		}
	}
	return ""
}

func newOperationID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate operation id: %w", err)
	}
	return "op-" + hex.EncodeToString(random[:]), nil
}

func packageNeedsPluginData(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) bool {
	for _, name := range domain.SelectedMCPNames(plan) {
		server := envelope.MCP.Servers[name]
		if server.Type == "stdio" {
			return true
		}
	}
	return false
}

type automaticActivationClassifier interface {
	AutomaticallyActivates(domain.ActivationRequest) bool
}

type activationPreflighter interface {
	PreflightActivation(domain.ActivationRequest) error
}

func (service Service) preflightActivation(input AddInput, plan domain.DeliveryPlan) error {
	preflighter, ok := service.Activator.(activationPreflighter)
	if !ok {
		return nil
	}
	return preflighter.PreflightActivation(domain.ActivationRequest{
		Client: input.Client, Plan: plan, BackendExecutable: input.BackendExecutable,
		VerifyOnly: input.DryRun,
	})
}

func (service Service) automaticallyActivates(input AddInput, plan domain.DeliveryPlan) bool {
	classifier, ok := service.Activator.(automaticActivationClassifier)
	if !ok {
		return false
	}
	return classifier.AutomaticallyActivates(domain.ActivationRequest{
		Client: input.Client, Plan: plan, BackendExecutable: input.BackendExecutable,
	})
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

func (service Service) verifyManagedTarget(
	ctx context.Context,
	client domain.DetectedClient,
	scope domain.InstallScope,
	binding domain.ClientBinding,
	operation string,
) error {
	if service.Targets == nil {
		return fmt.Errorf("delivery target resolver is required for %s", operation)
	}
	expectedDigest := managedDigest(binding)
	if expectedDigest == "" {
		return fmt.Errorf("managed package digest is missing; refusing %s and retaining state for reviewed recovery", operation)
	}
	targetClient := client
	if sameNativeBackend(domain.ClientID(binding.ClientID), client.ClientID) {
		// Validate the persisted path against the physical binding's canonical
		// owner, even when the caller addressed the shared backend through its
		// other logical surface.
		targetClient.ClientID = domain.ClientID(binding.ClientID)
	}
	target, err := service.Targets.ResolveTarget(ctx, targetClient, scope, binding.PhysicalArtifact)
	if err != nil {
		return fmt.Errorf("resolve managed %s target: %w", operation, err)
	}
	if err := pathpolicy.RequireExactPath(target.ActivePath, binding.TargetLocator); err != nil {
		return fmt.Errorf("refuse %s from untrusted persisted target: %w", operation, err)
	}
	if err := service.Stager.Verify(ctx, binding.TargetLocator, expectedDigest); err != nil {
		return fmt.Errorf("managed package was changed or is missing; refusing silent %s and retaining state: %w", operation, err)
	}
	return nil
}

func (service Service) observeNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) error {
	observation, err := service.nativeIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return fmt.Errorf("observe native identity for %s: %w", client.ClientID, err)
	}
	return validateNativeIdentityObservation(observation, managed)
}

func (service Service) observePreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) error {
	observation, err := service.preparedIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return fmt.Errorf("observe prepared identity for %s: %w", client.ClientID, err)
	}
	return validateNativeIdentityObservation(observation, managed)
}

func validateNativeIdentityObservation(observation domain.NativeIdentityObservation, managed *domain.ClientBinding) error {
	switch observation.State {
	case NativeIdentityAbsent:
		if managed != nil && managed.Materialization != domain.MaterializationAbsent {
			return fmt.Errorf("managed native identity is unexpectedly absent")
		}
		return nil
	case NativeIdentityManaged:
		if managed == nil {
			return fmt.Errorf("native identity already exists without matching agentplugins ownership; automatic adoption is disabled")
		}
		expected := managedDigest(*managed)
		if expected == "" || observation.Digest == "" || expected != observation.Digest {
			return fmt.Errorf("native identity ownership digest is stale or does not match")
		}
		return nil
	case NativeIdentityUnmanaged:
		return fmt.Errorf("native identity is unmanaged; remove it through its owning client or choose a distinct identity")
	case NativeIdentityIndeterminate:
		return fmt.Errorf("native identity ownership is indeterminate; refusing mutation")
	default:
		return fmt.Errorf("native identity observer returned an unknown state")
	}
}

func (service Service) nativeIdentityObservation(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if service.NativeObserver != nil {
		return service.NativeObserver.ObserveNativeIdentity(ctx, client, plan, managed)
	}
	return service.filesystemIdentityObservation(ctx, plan, managed)
}

func (service Service) preparedIdentityObservation(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if observer, ok := service.NativeObserver.(PreparedIdentityObserver); ok {
		return observer.ObservePreparedIdentity(ctx, client, plan, managed)
	}
	return service.filesystemIdentityObservation(ctx, plan, managed)
}

func (service Service) filesystemIdentityObservation(ctx context.Context, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	// Every backend gets a fail-closed filesystem observation even when it has
	// no richer namespace-aware observer. A specialized observer may prove
	// qualified coexistence; the fallback never does.
	if _, err := os.Lstat(plan.ActivePath); os.IsNotExist(err) {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
	} else if err != nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, err
	}
	if managed == nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}, nil
	}
	expected := managedDigest(*managed)
	if expected == "" {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, nil
	}
	if err := service.Stager.Verify(ctx, plan.ActivePath, expected); err != nil {
		var verification *ports.VerificationError
		if errors.As(err, &verification) {
			return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, Digest: verification.ActualDigest}, nil
		}
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, err
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: expected}, nil
}
