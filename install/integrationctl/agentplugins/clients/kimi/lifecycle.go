package kimi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (*Adapter) AutomaticallyActivates(_ clients.Env, r domain.ActivationRequest) bool {
	if r.Plan.Scope == domain.ScopeProject {
		return r.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.OnlyNativeComponents(r.Plan.Components)
	}
	return r.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.HasNativeConfigRoot(r)
}
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, p domain.DeliveryPlan, _ string) bool {
	return p.NativeRegistryRoot != ""
}
func (*Adapter) PreflightActivation(env clients.Env, r domain.ActivationRequest) error {
	if r.Plan.Scope == domain.ScopeProject || r.Plan.TargetContext.Scope == domain.ScopeProject {
		return preflightKimiProject(env, r)
	}
	// Generic preflight runs before staging, so Delivery and DeclaredName are
	// still empty there. Validate those fields only once they are supplied.
	if r.Client.ClientID != domain.ClientKimi || r.Plan.ClientID != domain.ClientKimi ||
		(r.Delivery.ClientID != "" && r.Delivery.ClientID != domain.ClientKimi) {
		return fmt.Errorf("activation is not for Kimi")
	}
	if r.Plan.Scope != domain.ScopeUser {
		return fmt.Errorf("kimi plugins support user scope only")
	}
	if (r.DeclaredName != "" && r.DeclaredName != r.Plan.DeclaredName) ||
		(r.Delivery.ActivePath != "" && !shared.SameCleanPath(r.Plan.ActivePath, r.Delivery.ActivePath)) ||
		!shared.SameCleanPath(r.Client.ConfigRoot, r.Plan.NativeRegistryRoot) {
		return fmt.Errorf("kimi activation identity mismatch")
	}
	if err := validateIdentity(r.Client.ConfigRoot, r.Plan.DeclaredName, r.Plan.ActivePath); err != nil {
		return err
	}
	reg, err := readRegistry(r.Client.ConfigRoot)
	if err != nil {
		return err
	}
	_, err = reg.record(r.Plan.DeclaredName, r.Plan.ActivePath)
	return err
}
func (a *Adapter) Activate(ctx context.Context, env clients.Env, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if r.Plan.Scope == domain.ScopeProject || r.Plan.TargetContext.Scope == domain.ScopeProject {
		return activateKimiProject(ctx, env, r)
	}
	out := shared.StartedActivation(r)
	// Native-config lifecycle persists an attempt before dispatch. Report its
	// effect explicitly so verified success can clear that marker; failures
	// after a possible registry write must retain uncertainty.
	out.NativeEffect = domain.NativeEffectUnchanged
	out.NativeObjects = append([]domain.NativeObjectOwnership(nil), r.PreviousNativeObjects...)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if err := a.PreflightActivation(env, r); err != nil {
		return out, err
	}
	if !r.VerifyOnly && !a.AutomaticallyActivates(env, r) {
		out.Activation = domain.ActivationManual
		out.UserActions = []string{reloadAction}
		return out, nil
	}
	if err := verifyManifest(r.Delivery.ActivePath, r.DeclaredName); err != nil {
		return shared.FailedActivation(out, "repair Kimi plugin manifest", err)
	}
	if !r.VerifyOnly {
		now := time.Now()
		if env.Now != nil {
			now = env.Now()
		}
		out.NativeEffect = domain.NativeEffectUncertain
		if err := mutateRegistry(r.Client.ConfigRoot, r.DeclaredName, r.Delivery.ActivePath, false, r.Replacing, now); err != nil {
			return shared.FailedActivation(out, "retry Kimi plugin registration", err)
		}
	}
	reg, err := readRegistry(r.Client.ConfigRoot)
	if err != nil {
		return shared.FailedActivation(out, "inspect Kimi plugin registry", err)
	}
	record, err := reg.record(r.DeclaredName, r.Delivery.ActivePath)
	if err != nil {
		return shared.FailedActivation(out, "repair Kimi plugin identity", err)
	}
	if record == nil || record["enabled"] != true {
		return shared.FailedActivation(out, "enable the managed plugin in Kimi Code", fmt.Errorf("%w: Kimi plugin absent or disabled", shared.ErrRecognizedNegativeEvidence))
	}
	if !r.VerifyOnly {
		out.NativeEffect = domain.NativeEffectCommitted
	}
	out.Activation = domain.ActivationActive
	out.Verification = domain.VerificationInstalled
	out.UserActions = []string{reloadAction}
	return out, nil
}
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, r domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if r.TargetContext.Scope == domain.ScopeProject {
		return deactivateKimiProject(ctx, env, r)
	}
	out := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if !r.Confirmed {
		return out, nil
	}
	if r.Client.ClientID != domain.ClientKimi {
		return out, fmt.Errorf("deactivation client identity mismatch")
	}
	if err := mutateRegistry(r.Client.ConfigRoot, r.DeclaredName, r.ManagedArtifactPath, true, true, time.Time{}); err != nil {
		out.ArtifactRemovalAllowed = false
		return out, err
	}
	out.ExternalRemovalComplete = true
	out.UserActions = []string{reloadAction}
	return out, nil
}
func verifyManifest(root, name string) error {
	if _, err := os.Lstat(filepath.Join(root, "kimi.plugin.json")); err == nil {
		return fmt.Errorf("conflicting priority Kimi manifest")
	} else if !os.IsNotExist(err) {
		return err
	}
	path := filepath.Join(root, ".kimi-plugin", "plugin.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return err
	}
	actual, err := shared.ReadJSONManifestName(path)
	if err != nil {
		return err
	}
	if actual != name {
		return fmt.Errorf("kimi manifest identity mismatch")
	}
	return nil
}
