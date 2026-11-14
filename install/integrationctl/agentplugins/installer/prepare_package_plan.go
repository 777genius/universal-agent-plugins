package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) loadMutatingPackage(ctx context.Context, handle *PreparedOperation, op Operation, allowDigestRewrite bool) error {
	req, snapshot, client := handle.req, handle.snapshot, handle.client
	if err := e.assessSnapshot(ctx, snapshot, req.Assessment); err != nil {
		return err
	}
	if op == OpInstall {
		if err := e.refuseRecordedDigestRewrite(req.InstallationID, snapshot.TreeDigest); err != nil {
			return err
		}
	}
	if op == OpRepair || op == OpRefreshProjection {
		if err := e.refuseRepairRevisionRewrite(req.InstallationID, string(client.ClientID), snapshot.TreeDigest); err != nil {
			return err
		}
	}
	if req.AcquiredSnapshot == nil {
		e.reuseMatchingSourceIdentity(req.InstallationID, &snapshot, allowDigestRewrite)
	}
	handle.snapshot = snapshot
	ldr, err := newLoader()
	if err != nil {
		return err
	}
	envelope, err := ldr.Load(ctx, domain.LoadInput{
		SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest,
		ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source,
	})
	if err != nil {
		return err
	}
	if snapshot.Source.WebSkill != nil && (envelope.LoaderKind != domain.LoaderKindAgentPlugins || envelope.FormatID != domain.FormatIDAgentPluginsV1 || len(envelope.Skills) != 1 || envelope.Skills[snapshot.Source.WebSkill.Skill].Name != snapshot.Source.WebSkill.Skill) {
		return fmt.Errorf("%w: HTTPS Skill acquisition requires its normalized portable Skill package", ErrInvalidRequest)
	}
	handle.envelope = envelope
	return nil
}

func (e *Engine) planMutatingPackage(handle *PreparedOperation, op Operation, preview usecase.AddResult) (*PreparedOperation, error) {
	req, snapshot, client, envelope := handle.req, handle.snapshot, handle.client, handle.envelope
	missing := missingRequired(envelope, req.RequiredComponents)
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{ProfileAuthority: domain.CloneProfileAuthority(client.ProfileAuthority),
		Operation: op, SourceRoot: firstNonEmpty(req.SourceRoot, req.PackageRoot), TreeDigest: snapshot.TreeDigest,
		DigestAlgorithm: snapshot.DigestAlgorithm, ClientID: string(client.ClientID),
		ConfigRoot: client.ConfigRoot, TargetPath: preview.Plan.ActivePath,
		InstallationID: firstNonEmpty(req.InstallationID, preview.InstallationID),
		BindingID:      domain.ComputeClientBindingID(firstNonEmpty(req.InstallationID, preview.InstallationID), string(preview.Plan.ClientID), string(preview.Plan.Scope), preview.Plan.ActivePath),
		HelperVersion:  helperVersion, HelperDigest: helperDigest,
		RequiredMissing: missing, NoChange: preview.NoChange,
		Delivery: deliveryPlan(preview.Plan), SelectedDelivery: preview.Plan.SelectedDelivery,
		Client: ClientResult{ClientID: string(client.ClientID), Activation: string(preview.Activation.Activation),
			Authentication: string(preview.Activation.Authentication), Policy: string(preview.Activation.Policy), Verification: string(preview.Activation.Verification)},
		RequiresConfirmation: preview.RequiresConfirmation,
	}
	if handle.openCodeHost != nil {
		profile := handle.openCodeHost.Profile()
		handle.plan.OpenCodeProfile = &profile
		handle.plan.OpenCodeSelections = handle.openCodeHost.Selections()
	}
	handle.facts = BindingFacts{ProfileAuthority: domain.CloneProfileAuthority(handle.client.ProfileAuthority),
		InstallationID: handle.plan.InstallationID, ClientID: handle.plan.ClientID,
		BindingID: handle.plan.BindingID, Scope: string(preview.Plan.Scope),
		SelectedDelivery: preview.Plan.SelectedDelivery,
		TargetPath:       handle.plan.TargetPath, OperationID: req.OperationID, TreeDigest: snapshot.TreeDigest,
	}
	handle.req.InstallationID = handle.plan.InstallationID
	handle.observationPlan = cloneObservedDeliveryPlan(preview.Plan)
	handle.artifact = preview.Plan.PhysicalArtifactID
	if len(missing) != 0 {
		_ = handle.closeLocked()
		return nil, fmt.Errorf("%w: %v", ErrIncomplete, missing)
	}
	return handle, nil
}

func clonePrepareRequest(req Request) Request {
	copied := req
	copied.RequiredComponents = append([]string(nil), req.RequiredComponents...)
	copied.ExecutableFiles = slices.Clone(req.ExecutableFiles)
	copied.Targets = append([]ClientTarget(nil), req.Targets...)
	copied.KnownTargets = append([]TargetFacts(nil), req.KnownTargets...)
	if req.Assessment != nil {
		assessment := *req.Assessment
		copied.Assessment = &assessment
	}
	if req.EnvironmentAuthority != nil {
		authority := *req.EnvironmentAuthority
		copied.EnvironmentAuthority = &authority
	}
	if req.AcquiredSnapshot != nil {
		acquired := *req.AcquiredSnapshot
		acquired.ExecutableFiles = append([]string{}, req.AcquiredSnapshot.ExecutableFiles...)
		// Compatibility contains only JSON data; clone its nested evidence and maps.
		hints, _ := json.Marshal(req.AcquiredSnapshot.Compatibility)
		acquired.Compatibility = domain.CompatibilityHints{}
		_ = json.Unmarshal(hints, &acquired.Compatibility)
		acquired.Directory = cloneAcquiredDirectory(req.AcquiredSnapshot.Directory)
		copied.AcquiredSnapshot = &acquired
	}
	return copied
}

func validatePrepareInput(ctx context.Context, req Request) error {
	if req.ConfirmPluginEnable && (req.Operation != OpRepair || req.Scope != domain.ScopeProject || req.InstallationID == "" || len(req.Targets) > 1 || req.ClientID == "" && len(req.Targets) == 0) {
		return fmt.Errorf("%w: explicit plugin enablement requires one selected Project repair", ErrInvalidRequest)
	}
	if req.ClientNativeConfigRoot != "" && len(req.Targets) > 0 {
		return fmt.Errorf("%w: independent native User root requires a single target", ErrInvalidRequest)
	}
	if (req.ActivationComplete || req.AuthComplete) && (req.Operation == OpRemove || len(req.Targets) > 0) {
		return fmt.Errorf("%w: completion attestation requires a single mutating target", ErrInvalidRequest)
	}
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if req.ExecutableFiles != nil && len(req.Targets) > 0 {
		return fmt.Errorf("%w: explicit snapshot executable inventory requires a single target", ErrInvalidRequest)
	}

	return nil
}
