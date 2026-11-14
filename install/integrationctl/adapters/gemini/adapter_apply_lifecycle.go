package gemini

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) ApplyUpdate(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Gemini update requires current record", nil)
	}
	name := strings.TrimSpace(in.Record.IntegrationID)
	if name == "" {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "Gemini update requires integration identity", nil)
	}
	metadata := map[string]any{
		"extension_name":  name,
		"resolved_source": in.Manifest.ResolvedRef.Value,
	}
	manualSteps := []string{"restart Gemini CLI after the native extension update completes"}
	owned := []domain.NativeObjectRef{{
		Kind:            "extension_dir",
		Path:            filepath.Join(a.userHome(), ".gemini", "extensions", name),
		ProtectionClass: domain.ProtectionUserMutable,
	}}
	if in.ResolvedSource != nil && usesManagedLocalInstall(in.ResolvedSource.Kind) {
		materializedRoot, err := a.syncManagedLocalSource(ctx, in.Manifest, in.ResolvedSource.LocalPath)
		if err != nil {
			return ports.ApplyResult{}, err
		}
		if err := a.activateManagedLocalInstall(materializedRoot, name); err != nil {
			return ports.ApplyResult{}, err
		}
		metadata["materialized_source_root"] = materializedRoot
		owned = append(owned, domain.NativeObjectRef{
			Kind:            "managed_source_root",
			Path:            materializedRoot,
			ProtectionClass: domain.ProtectionUserMutable,
		})
		metadata["update_mode"] = "local_projection"
		manualSteps = []string{"restart Gemini CLI to load the updated extension and merged configuration"}
	} else {
		argv := []string{"gemini", "extensions", "update", name}
		if err := a.runGemini(ctx, argv, a.commandDirForRecord(*in.Record)); err != nil {
			return ports.ApplyResult{}, err
		}
		metadata["update_argv"] = argv
	}
	return ports.ApplyResult{
		TargetID:           a.ID(),
		State:              domain.InstallInstalled,
		ActivationState:    domain.ActivationRestartPending,
		OwnedNativeObjects: owned,
		EvidenceClass:      domain.EvidenceConfirmed,
		ManualSteps:        manualSteps,
		AdapterMetadata:    metadata,
	}, nil
}

func (a Adapter) ApplyRemove(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Gemini remove requires current record", nil)
	}
	name := strings.TrimSpace(in.Record.IntegrationID)
	if name == "" {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "Gemini remove requires integration identity", nil)
	}
	if err := a.validateExtensionDir(name, a.extensionDir(name)); err != nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "validate Gemini extension dir", err)
	}
	materializedRoot := materializedRootFromRecord(*in.Record)
	if materializedRoot != "" {
		if err := a.validateManagedSourceRoot(name, materializedRoot); err != nil {
			return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "validate Gemini managed source root", err)
		}
	}
	metadata := map[string]any{
		"extension_name": name,
	}
	if installModeFromRecord(*in.Record) == "local_projection" || materializedRootFromRecord(*in.Record) != "" {
		if err := os.RemoveAll(a.extensionDir(name)); err != nil && !os.IsNotExist(err) {
			return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "remove Gemini local extension dir", err)
		}
		metadata["remove_mode"] = "local_projection"
	} else {
		argv := []string{"gemini", "extensions", "uninstall", name}
		if err := a.runGemini(ctx, argv, a.commandDirForRecord(*in.Record)); err != nil {
			return ports.ApplyResult{}, err
		}
		metadata["remove_argv"] = argv
	}
	if materializedRoot != "" {
		if err := os.RemoveAll(materializedRoot); err != nil && !os.IsNotExist(err) {
			return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "remove managed Gemini source root", err)
		}
	}
	return ports.ApplyResult{
		TargetID:        a.ID(),
		State:           domain.InstallRemoved,
		ActivationState: domain.ActivationRestartPending,
		EvidenceClass:   domain.EvidenceConfirmed,
		ManualSteps:     []string{"restart Gemini CLI after uninstall so the removed extension is no longer loaded"},
		AdapterMetadata: metadata,
	}, nil
}

func (a Adapter) Repair(ctx context.Context, in ports.RepairInput) (ports.ApplyResult, error) {
	if in.Manifest == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Gemini repair requires resolved manifest", nil)
	}
	if in.Record.IntegrationID == "" {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Gemini repair requires integration identity", nil)
	}
	result, err := a.ApplyUpdate(ctx, ports.ApplyInput{
		Plan:           ports.AdapterPlan{TargetID: a.ID(), ActionClass: "repair_drift", EvidenceKey: "target.gemini.native_surface"},
		Manifest:       *in.Manifest,
		ResolvedSource: in.ResolvedSource,
		Policy:         in.Record.Policy,
		Inspect:        in.Inspect,
		Record:         &in.Record,
	})
	if err != nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrRepairApply, "Gemini repair update failed", err)
	}
	if len(result.ManualSteps) == 0 {
		result.ManualSteps = append(result.ManualSteps, "restart Gemini CLI after repair to reload the updated extension")
	} else if installModeFromRecord(in.Record) == "local_projection" || strings.TrimSpace(anyString(result.AdapterMetadata["update_mode"])) == "local_projection" {
		result.ManualSteps = append(result.ManualSteps, "repair refreshed the managed Gemini projection")
	} else {
		result.ManualSteps = append(result.ManualSteps, "repair used native gemini extensions update semantics")
	}
	return result, nil
}

func (a Adapter) runGemini(ctx context.Context, argv []string, dir string) error {
	result, err := a.runner().Run(ctx, ports.Command{Argv: argv, Dir: dir})
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "run Gemini CLI", err)
	}
	if result.ExitCode != 0 {
		msg := strings.TrimSpace(string(result.Stderr))
		if msg == "" {
			msg = strings.TrimSpace(string(result.Stdout))
		}
		if msg == "" {
			msg = "Gemini CLI command failed"
		}
		return domain.NewError(domain.ErrMutationApply, msg, nil)
	}
	return nil
}

func anyString(v any) string {
	s, _ := v.(string)
	return s
}
