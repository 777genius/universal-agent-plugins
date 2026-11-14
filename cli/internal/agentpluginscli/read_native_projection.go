package agentpluginscli

import (
	"context"
	"errors"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func checkNativeProjectionIntegrity(ctx context.Context, app App, client domain.DetectedClient, target domain.DeliveryTarget, installation domain.Installation, binding domain.ClientBinding, expectedDigest string) []doctorFinding {
	if !nativeConfigLifecycle(domain.ClientID(binding.ClientID), domain.InstallScope(binding.Scope)) || !hasOwnedNativeProjection(binding.NativeObjects) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return nativeProjectionNotChecked(installation, binding)
	}
	if app.Lifecycle.Activator == nil || strings.TrimSpace(client.ConfigRoot) == "" {
		return nativeProjectionNotChecked(installation, binding)
	}
	// The provider activator owns the exact config codecs and skill receipt
	// verification. VerifyOnly routes through those same verifiers without
	// starting the client or mutating its configuration.
	outcome, err := app.Lifecycle.Activator.Activate(ctx, domain.ActivationRequest{
		Client: client,
		Plan: domain.DeliveryPlan{
			ClientID: domain.ClientID(binding.ClientID), Scope: domain.InstallScope(binding.Scope), TargetContext: target.TargetContext, NativeRegistryRoot: target.NativeRegistryRoot,
			PhysicalArtifactID: binding.PhysicalArtifact, DeclaredName: installation.DeclaredName,
			TargetRoot: target.TargetRoot, ActivePath: target.ActivePath,
			Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Support: domain.SupportProjected}},
		},
		Delivery: domain.StagedDelivery{
			ClientID: domain.ClientID(binding.ClientID), OwnedBase: target.TargetRoot,
			ActivePath: target.ActivePath, ArtifactDigest: expectedDigest,
			NativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
		},
		DeclaredName: installation.DeclaredName, Replacing: true,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
		VerifyOnly:            true,
	})
	var verification *ports.VerificationError
	if err != nil {
		// A canceled or time-bounded observer does not prove native drift.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
			(errors.As(err, &verification) && verification.Kind == ports.VerificationIndeterminate) {
			return nativeProjectionNotChecked(installation, binding)
		}
		return []doctorFinding{scopedFinding("degraded", "native_projection_changed", installation, binding.ClientID, "an owned native client configuration entry or skill is missing or differs from its recorded projection", repairAction(installation, binding))}
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		return nativeProjectionNotChecked(installation, binding)
	}
	return nil
}

func nativeProjectionNotChecked(installation domain.Installation, binding domain.ClientBinding) []doctorFinding {
	return []doctorFinding{scopedFinding("unknown", "native_projection_not_checked", installation, binding.ClientID, "the exact owned native client projection could not be checked", "restore client configuration visibility and rerun doctor before relying on this installation")}
}

func hasOwnedNativeProjection(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind != "managed_package_directory" {
			return true
		}
	}
	return false
}
