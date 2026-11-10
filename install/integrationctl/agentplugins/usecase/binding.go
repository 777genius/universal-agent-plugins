package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type BindingChangeMode string

const (
	BindingChangeRebind        BindingChangeMode = "rebind"
	BindingChangeMigrateFormat BindingChangeMode = "migrate_format"
	BindingChangeSwitch        BindingChangeMode = "switch"
)

type ProvenanceSummary struct {
	Kind             string `json:"kind"`
	Repository       string `json:"repository,omitempty"`
	PackageSubpath   string `json:"package_subpath,omitempty"`
	ResolvedRevision string `json:"resolved_revision,omitempty"`
	TreeDigest       string `json:"tree_digest,omitempty"`
}

type FormatSummary struct {
	LoaderKind string `json:"loader_kind"`
	FormatID   string `json:"format_id"`
	SchemaURI  string `json:"schema_uri"`
}

type TargetChange struct {
	ClientID        string                      `json:"client_id"`
	Scope           string                      `json:"scope"`
	Materialization domain.MaterializationState `json:"materialization"`
	Decision        string                      `json:"decision"`
}

type BindingChangePlan struct {
	Mode               BindingChangeMode         `json:"mode"`
	InstallationID     string                    `json:"installation_id"`
	OldName            string                    `json:"old_name"`
	NewName            string                    `json:"new_name"`
	OldSource          ProvenanceSummary         `json:"old_source"`
	NewSource          ProvenanceSummary         `json:"new_source"`
	OldFormat          FormatSummary             `json:"old_format"`
	NewFormat          FormatSummary             `json:"new_format"`
	OldComponents      domain.ComponentInventory `json:"old_components"`
	NewComponents      domain.ComponentInventory `json:"new_components"`
	Targets            []TargetChange            `json:"targets,omitempty"`
	NativeObjectCount  int                       `json:"native_object_count"`
	PluginDataDecision string                    `json:"plugin_data_decision"`
	CanApply           bool                      `json:"can_apply"`
	Blockers           []string                  `json:"blockers,omitempty"`
}

type BindingChangeInput struct {
	Selector  string
	Envelope  domain.PackageEnvelope
	Confirmed bool
}

type BindingChangeResult struct {
	Plan                 BindingChangePlan         `json:"plan"`
	PluginData           domain.PluginDataDecision `json:"plugin_data"`
	RequiresConfirmation bool                      `json:"requires_confirmation"`
	Mutated              bool                      `json:"mutated"`
	NoChange             bool                      `json:"no_change,omitempty"`
}

func (service Service) Rebind(ctx context.Context, input BindingChangeInput) (BindingChangeResult, error) {
	return service.changeBinding(ctx, input, BindingChangeRebind)
}

func (service Service) MigrateFormat(ctx context.Context, input BindingChangeInput) (BindingChangeResult, error) {
	return service.changeBinding(ctx, input, BindingChangeMigrateFormat)
}

// SwitchRetained changes source only for a data_retained installation. Active
// installations must use SwitchGroup so every physical binding participates in
// the staged group transaction.
func (service Service) SwitchRetained(ctx context.Context, input BindingChangeInput, origin domain.OriginMode, directory *domain.DirectoryOrigin) (BindingChangeResult, error) {
	if service.StateStore == nil {
		return BindingChangeResult{}, fmt.Errorf("state store is required")
	}
	if origin == "" && directory != nil {
		origin = domain.OriginModeDirectory
	}
	if err := validateOperationOrigin(origin, directory); err != nil {
		return BindingChangeResult{}, err
	}
	release, err := service.beginMutation(ctx, false, input.Confirmed)
	if err != nil {
		return BindingChangeResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, index, installation, result, err := service.loadSwitchRetained(input)
	if err != nil {
		return result, err
	}
	if err := validateSwitchRetained(installation, input, origin, directory); err != nil {
		return result, err
	}
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	return service.commitSwitchRetained(state, index, installation, input, origin, directory, result)
}

func (service Service) changeBinding(ctx context.Context, input BindingChangeInput, mode BindingChangeMode) (BindingChangeResult, error) {
	if err := validateBindingChangeSetup(ctx, service, input); err != nil {
		return BindingChangeResult{}, err
	}
	release, err := service.beginMutation(ctx, false, input.Confirmed)
	if err != nil {
		return BindingChangeResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return BindingChangeResult{}, err
	}
	index, installation, err := findInstallation(state, input.Selector)
	if err != nil {
		return BindingChangeResult{}, err
	}
	if err := validateBindingChangeMode(mode, installation, input.Envelope); err != nil {
		return BindingChangeResult{}, err
	}
	newSourceID := domain.ComputeSourceBindingID(input.Envelope.Source)
	plan := buildBindingChangePlan(mode, installation, input.Envelope)
	result := BindingChangeResult{Plan: plan}
	if bindingChangeUnchanged(mode, installation, input.Envelope, newSourceID) {
		result.NoChange = true
		return result, nil
	}
	if err := rejectDuplicateSource(state, index, newSourceID); err != nil {
		return result, err
	}
	if done, err := confirmBindingChange(plan, input.Confirmed, &result); done {
		return result, err
	}
	return service.commitBindingChange(state, index, installation, input, newSourceID, result)
}

func (service Service) loadSwitchRetained(input BindingChangeInput) (domain.StateFileV2, int, domain.Installation, BindingChangeResult, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return domain.StateFileV2{}, -1, domain.Installation{}, BindingChangeResult{}, err
	}
	index, installation, err := findInstallation(state, input.Selector)
	if err != nil {
		return domain.StateFileV2{}, -1, domain.Installation{}, BindingChangeResult{}, err
	}
	plan := buildBindingChangePlan(BindingChangeSwitch, installation, input.Envelope)
	plan.PluginDataDecision = "preserved_with_cross_distribution_compatibility_warning"
	result := BindingChangeResult{Plan: plan, PluginData: switchPluginDataDecision(installation)}
	return state, index, installation, result, nil
}

func validateSwitchRetained(installation domain.Installation, input BindingChangeInput, origin domain.OriginMode, directory *domain.DirectoryOrigin) error {
	if !installation.DataRetained || len(installation.Clients) != 0 {
		return fmt.Errorf("active installation switch requires SwitchGroup")
	}
	if installation.DeclaredName != input.Envelope.Manifest.Name {
		return fmt.Errorf("switch must preserve manifest identity")
	}
	if installation.OriginMode == domain.OriginModeDirectory && normalizedOriginMode(origin) == domain.OriginModeDirectory && installation.Directory != nil && directory != nil && installation.Directory.ProductID != directory.ProductID {
		return fmt.Errorf("switch must remain within one Directory product")
	}
	return nil
}

func (service Service) commitSwitchRetained(state domain.StateFileV2, index int, installation domain.Installation, input BindingChangeInput, origin domain.OriginMode, directory *domain.DirectoryOrigin, result BindingChangeResult) (BindingChangeResult, error) {
	sourceID := domain.ComputeSourceBindingID(input.Envelope.Source)
	if err := rejectDuplicateSource(state, index, sourceID); err != nil {
		return result, err
	}
	installation.Source = sourceBindingFromEnvelope(input.Envelope, sourceID)
	installation.Package = packageBindingFromEnvelope(input.Envelope)
	installation.OriginMode = normalizedOriginMode(origin)
	installation.Directory = cloneDirectoryOrigin(directory)
	installation.UpdatedAt = service.now().Format(time.RFC3339Nano)
	state.Installations[index] = installation
	if err := service.StateStore.Save(state); err != nil {
		return result, err
	}
	result.Mutated = true
	return result, nil
}
