package antigravity

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const maxNativeManifestBytes int64 = 1 << 20

var (
	_ clients.Lifecycle = (*Adapter)(nil)
	_ clients.ActivationPreflighter = (*Adapter)(nil)
)

// Antigravity reads complete, UAP-owned directories from its native plugin
// roots. File delivery does not authorize a claim that the host loaded,
// enabled or authenticated the plugin.
func (*Adapter) PreflightActivation(_ clients.Env, request domain.ActivationRequest) error {
	// A group previews activation before the stager creates a delivery. An
	// absent delivery is not a mismatched one, but a partially identified or
	// already staged delivery must match exactly.
	if request.Plan.ClientID != domain.ClientAntigravity || request.Client.ClientID != domain.ClientAntigravity {
		return fmt.Errorf("Antigravity selected client identity changed")
	}
	if request.Delivery.ClientID != "" || request.Delivery.ActivePath != "" {
		if err := shared.ActivationIdentityMismatch(request); err != nil { return err }
		if request.Delivery.ActivePath != request.Plan.ActivePath {
			return fmt.Errorf("Antigravity activation delivery identity changed")
		}
	}
	if request.ActivationComplete {
		return fmt.Errorf("Antigravity activation cannot be attested without native client evidence")
	}
	return selectedNativePluginRoot(request.Plan)
}

func selectedNativePluginRoot(plan domain.DeliveryPlan) error {
	if !cleanAbsolute(plan.NativeRegistryRoot) || !cleanAbsolute(plan.ActivePath) {
		return fmt.Errorf("Antigravity selected native plugin path is invalid")
	}
	expected := filepath.Join(plan.NativeRegistryRoot, "plugins")
	if plan.TargetRoot != expected || filepath.Dir(plan.ActivePath) != expected {
		return fmt.Errorf("Antigravity plugin must live beneath the exact selected native root")
	}
	switch plan.Scope {
	case domain.ScopeUser:
		// User native config is separate from the selected project path.
		if plan.TargetContext.Scope == domain.ScopeProject {
			return fmt.Errorf("Antigravity project request cannot use user scope")
		}
	case domain.ScopeProject:
		if !cleanAbsolute(plan.TargetContext.ProjectRoot) ||
			filepath.Join(plan.TargetContext.ProjectRoot, ".agents") != plan.NativeRegistryRoot {
			return fmt.Errorf("Antigravity plugin cannot cross project scope")
		}
	default:
		return fmt.Errorf("Antigravity requires an explicit User or Project scope")
	}
	return nil
}

func verifyPreparedNativePlugin(plan domain.DeliveryPlan, name string) error {
	if err := selectedNativePluginRoot(plan); err != nil {
		return err
	}
	manifestPath := filepath.Join(plan.ActivePath, ".antigravity-plugin", "plugin.json")
	body, err := atomicfile.ReadBoundedRegularFile(manifestPath, maxNativeManifestBytes)
	if err != nil {
		return fmt.Errorf("inspect Antigravity prepared native manifest: %w", err)
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return fmt.Errorf("decode Antigravity prepared manifest: %w", err)
	}
	nativeName, ok := document["name"].(string)
	if !ok || nativeName == "" || nativeName != name {
		return fmt.Errorf("Antigravity native plugin name differs from selected package")
	}
	if document["$schema"] != "https://antigravity.google/schemas/v1/plugin.json" {
		return fmt.Errorf("Antigravity prepared native schema is not recognized")
	}
	return nil
}

func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Delivery.ActivePath != request.Plan.ActivePath {
		return domain.ActivationOutcome{}, fmt.Errorf("Antigravity staged delivery path changed")
	}
	if err := New().PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := verifyPreparedNativePlugin(request.Plan, request.DeclaredName); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	outcome.Activation = domain.ActivationPrepared
	outcome.Verification = domain.VerificationInstalled
	outcome.NativeEffect = domain.NativeEffectUnchanged
	outcome.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
	outcome.UserActions = append(outcome.UserActions,
		"Open or reload Antigravity and verify native plugin discovery/enablement; files alone do not establish runtime activation")
	return outcome, nil
}

func (*Adapter) Deactivate(ctx context.Context, _ clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	// UAP's generic transaction removes only its exact-owned plugin directory.
	// Never execute vendor CLI uninstall on a different profile or delete a
	// foreign Antigravity plugin registration.
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions,
			"Only the exact UAP-owned Antigravity plugin directory may be removed")
		return outcome, nil
	}
	outcome.ExternalRemovalComplete = true
	outcome.NativeEffect = domain.NativeEffectUnchanged
	outcome.UserActions = append(outcome.UserActions,
		"Reload Antigravity to clear the plugin from a running session")
	return outcome, nil
}
