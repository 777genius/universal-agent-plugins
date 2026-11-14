package gemini

import (
	"context"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) ApplyInstall(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.ResolvedSource == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Gemini install requires resolved source", nil)
	}
	argv, dir, cleanup, materializedRoot, err := a.installCommand(ctx, in)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if usesManagedLocalInstall(in.ResolvedSource.Kind) {
		if err := a.activateManagedLocalInstall(materializedRoot, in.Manifest.IntegrationID); err != nil {
			return ports.ApplyResult{}, err
		}
	} else {
		if err := a.runGemini(ctx, argv, dir); err != nil {
			return ports.ApplyResult{}, err
		}
	}
	return a.installResult(in, argv, materializedRoot), nil
}

func (a Adapter) installResult(in ports.ApplyInput, argv []string, materializedRoot string) ports.ApplyResult {
	owned := []domain.NativeObjectRef{{
		Kind:            "extension_dir",
		Path:            filepath.Join(a.userHome(), ".gemini", "extensions", in.Manifest.IntegrationID),
		ProtectionClass: domain.ProtectionUserMutable,
	}}
	metadata := map[string]any{
		"install_argv":   argv,
		"extension_name": in.Manifest.IntegrationID,
		"install_mode":   installModeForKind(in.ResolvedSource.Kind),
	}
	if materializedRoot != "" {
		owned = append(owned, domain.NativeObjectRef{
			Kind:            "managed_source_root",
			Path:            materializedRoot,
			ProtectionClass: domain.ProtectionUserMutable,
		})
		metadata["materialized_source_root"] = materializedRoot
	}
	return ports.ApplyResult{
		TargetID:           a.ID(),
		State:              domain.InstallInstalled,
		ActivationState:    domain.ActivationRestartPending,
		OwnedNativeObjects: owned,
		EvidenceClass:      domain.EvidenceConfirmed,
		ManualSteps:        []string{"restart Gemini CLI to load the updated extension and merged configuration"},
		AdapterMetadata:    metadata,
	}
}
