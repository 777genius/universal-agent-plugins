package gemini

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) PlanEnable(_ context.Context, in ports.PlanToggleInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{
		TargetID:        a.ID(),
		ActionClass:     "enable_target",
		Summary:         "Enable Gemini extension in the native scope",
		RestartRequired: true,
		PathsTouched:    dedupeStrings([]string{a.settingsPath(scopeFromRecord(in.Record), workspaceRootFromRecord(in.Record)), a.enablementPath()}),
		EvidenceKey:     "target.gemini.native_surface",
	}, nil
}

func (a Adapter) ApplyEnable(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Gemini enable requires current record", nil)
	}
	name := strings.TrimSpace(in.Record.IntegrationID)
	if name == "" {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "Gemini enable requires integration identity", nil)
	}
	scope := geminiToggleScope(scopeFromRecord(*in.Record))
	argv := []string{"gemini", "extensions", "enable", name, "--scope", scope}
	if err := a.runGemini(ctx, argv, a.commandDirForRecord(*in.Record)); err != nil {
		return ports.ApplyResult{}, err
	}
	return ports.ApplyResult{
		TargetID:           a.ID(),
		State:              domain.InstallInstalled,
		ActivationState:    domain.ActivationRestartPending,
		OwnedNativeObjects: ownedGeminiObjects(*in.Record, a.userHome()),
		EvidenceClass:      domain.EvidenceConfirmed,
		ManualSteps:        []string{"restart Gemini CLI after enabling the extension"},
		AdapterMetadata: map[string]any{
			"toggle_argv": argv,
			"toggle":      "enable",
		},
	}, nil
}

func (a Adapter) PlanDisable(_ context.Context, in ports.PlanToggleInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{
		TargetID:        a.ID(),
		ActionClass:     "disable_target",
		Summary:         "Disable Gemini extension in the native scope",
		RestartRequired: true,
		PathsTouched:    dedupeStrings([]string{a.settingsPath(scopeFromRecord(in.Record), workspaceRootFromRecord(in.Record)), a.enablementPath()}),
		EvidenceKey:     "target.gemini.native_surface",
	}, nil
}

func (a Adapter) ApplyDisable(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if in.Record == nil {
		return ports.ApplyResult{}, domain.NewError(domain.ErrMutationApply, "Gemini disable requires current record", nil)
	}
	name := strings.TrimSpace(in.Record.IntegrationID)
	if name == "" {
		return ports.ApplyResult{}, domain.NewError(domain.ErrStateConflict, "Gemini disable requires integration identity", nil)
	}
	scope := geminiToggleScope(scopeFromRecord(*in.Record))
	argv := []string{"gemini", "extensions", "disable", name, "--scope", scope}
	if err := a.runGemini(ctx, argv, a.commandDirForRecord(*in.Record)); err != nil {
		return ports.ApplyResult{}, err
	}
	return ports.ApplyResult{
		TargetID:           a.ID(),
		State:              domain.InstallDisabled,
		ActivationState:    domain.ActivationRestartPending,
		OwnedNativeObjects: ownedGeminiObjects(*in.Record, a.userHome()),
		EvidenceClass:      domain.EvidenceConfirmed,
		ManualSteps:        []string{"restart Gemini CLI after disabling the extension"},
		AdapterMetadata: map[string]any{
			"toggle_argv": argv,
			"toggle":      "disable",
		},
	}, nil
}
