package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
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

func validateBindingChangeSetup(ctx context.Context, service Service, input BindingChangeInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if service.StateStore == nil {
		return fmt.Errorf("state store is required")
	}
	if input.Envelope.LoaderKind != domain.LoaderKindAgentPlugins || input.Envelope.FormatID != domain.FormatIDAgentPluginsV1 {
		return fmt.Errorf("new binding must be a standard Agent Plugins 1.0 package")
	}
	return nil
}

func validateBindingChangeMode(mode BindingChangeMode, installation domain.Installation, envelope domain.PackageEnvelope) error {
	if mode == BindingChangeRebind && installation.Package.LoaderKind != envelope.LoaderKind {
		return fmt.Errorf("rebind cannot change package format; use migrate-format")
	}
	if mode == BindingChangeRebind && !installation.NeedsRebind {
		return fmt.Errorf("rebind is only for broken provenance recovery; use switch to change a healthy source")
	}
	if mode == BindingChangeMigrateFormat && installation.Package.LoaderKind == envelope.LoaderKind && installation.Package.FormatID == envelope.FormatID {
		return fmt.Errorf("installation already uses Agent Plugins 1.0; use rebind")
	}
	return nil
}

func bindingChangeUnchanged(mode BindingChangeMode, installation domain.Installation, envelope domain.PackageEnvelope, newSourceID string) bool {
	return mode == BindingChangeRebind && newSourceID == installation.Source.SourceBindingID &&
		installation.Source.TreeDigest == envelope.TreeDigest && installation.Package.ManifestDigest == envelope.ManifestDigest
}

func rejectDuplicateSource(state domain.StateFileV2, index int, sourceID string) error {
	for otherIndex, other := range state.Installations {
		if otherIndex != index && other.Source.SourceBindingID == sourceID {
			return fmt.Errorf("new source is already bound to installation %s", other.InstallationID)
		}
	}
	return nil
}

func confirmBindingChange(plan BindingChangePlan, confirmed bool, result *BindingChangeResult) (bool, error) {
	if !plan.CanApply {
		if confirmed {
			return true, fmt.Errorf("binding change is blocked: %s", strings.Join(plan.Blockers, "; "))
		}
		return true, nil
	}
	if !confirmed {
		result.RequiresConfirmation = true
		return true, nil
	}
	return false, nil
}

func (service Service) commitBindingChange(state domain.StateFileV2, index int, installation domain.Installation, input BindingChangeInput, newSourceID string, result BindingChangeResult) (BindingChangeResult, error) {
	installation.DeclaredName = input.Envelope.Manifest.Name
	installation.Source = sourceBindingFromEnvelope(input.Envelope, newSourceID)
	installation.Package = packageBindingFromEnvelope(input.Envelope)
	installation.NeedsRebind = false
	installation.UpdatedAt = service.now().Format(time.RFC3339Nano)
	state.Installations[index] = installation
	if err := service.StateStore.Save(state); err != nil {
		return result, fmt.Errorf("commit binding change: %w", err)
	}
	result.Mutated = true
	return result, nil
}

func sourceBindingFromEnvelope(envelope domain.PackageEnvelope, sourceID string) domain.SourceBinding {
	return domain.SourceBinding{
		SourceBindingID: sourceID, RequestedSource: envelope.Source.RequestedSource,
		CanonicalSource: envelope.Source.CanonicalSource, Repository: envelope.Source.Repository,
		PackageSubpath: envelope.Source.PackageSubpath, ResolvedRevision: envelope.Source.ResolvedRevision,
		TreeDigest: envelope.TreeDigest,
	}
}

func packageBindingFromEnvelope(envelope domain.PackageEnvelope) domain.PackageBinding {
	return domain.PackageBinding{
		LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID,
		SchemaURI: envelope.SchemaURI, DeclaredName: envelope.Manifest.Name,
		Version: envelope.Manifest.Version, ManifestDigest: envelope.ManifestDigest,
		Inventory: envelope.Inventory,
	}
}

func buildBindingChangePlan(mode BindingChangeMode, installation domain.Installation, envelope domain.PackageEnvelope) BindingChangePlan {
	plan := BindingChangePlan{
		Mode: mode, InstallationID: installation.InstallationID,
		OldName: installation.DeclaredName, NewName: envelope.Manifest.Name,
		OldSource: provenanceFromBinding(installation.Source), NewSource: provenanceFromEnvelope(envelope),
		OldFormat:     FormatSummary{LoaderKind: installation.Package.LoaderKind, FormatID: installation.Package.FormatID, SchemaURI: installation.Package.SchemaURI},
		NewFormat:     FormatSummary{LoaderKind: envelope.LoaderKind, FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI},
		OldComponents: installation.Package.Inventory, NewComponents: envelope.Inventory,
		PluginDataDecision: "not_transferred", CanApply: true,
	}
	for _, client := range installation.Clients {
		decision := "reinstall_after_binding_change"
		if client.Materialization != domain.MaterializationAbsent {
			plan.CanApply = false
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("remove %s/%s target first", client.ClientID, client.Scope))
			decision = "remove_first"
		}
		plan.Targets = append(plan.Targets, TargetChange{
			ClientID: client.ClientID, Scope: client.Scope, Materialization: client.Materialization, Decision: decision,
		})
		plan.NativeObjectCount += len(client.NativeObjects)
		if len(client.NativeObjects) > 0 {
			plan.CanApply = false
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("%s/%s still owns native objects", client.ClientID, client.Scope))
		}
	}
	sort.Slice(plan.Targets, func(i, j int) bool {
		if plan.Targets[i].ClientID == plan.Targets[j].ClientID {
			return plan.Targets[i].Scope < plan.Targets[j].Scope
		}
		return plan.Targets[i].ClientID < plan.Targets[j].ClientID
	})
	sort.Strings(plan.Blockers)
	return plan
}

func provenanceFromBinding(source domain.SourceBinding) ProvenanceSummary {
	kind := "remote"
	if source.Repository != "" {
		kind = "github"
	} else if filepath.IsAbs(source.CanonicalSource) {
		kind = "local"
	}
	return ProvenanceSummary{
		Kind: kind, Repository: source.Repository, PackageSubpath: source.PackageSubpath,
		ResolvedRevision: source.ResolvedRevision, TreeDigest: source.TreeDigest,
	}
}

func provenanceFromEnvelope(envelope domain.PackageEnvelope) ProvenanceSummary {
	kind := "remote"
	if envelope.Source.Repository != "" {
		kind = "github"
	} else if filepath.IsAbs(envelope.Source.CanonicalSource) {
		kind = "local"
	}
	return ProvenanceSummary{
		Kind: kind, Repository: envelope.Source.Repository, PackageSubpath: envelope.Source.PackageSubpath,
		ResolvedRevision: envelope.Source.ResolvedRevision, TreeDigest: envelope.TreeDigest,
	}
}
