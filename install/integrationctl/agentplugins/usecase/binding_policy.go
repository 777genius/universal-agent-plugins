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
		StableSourceKey: envelope.Source.StableSourceKey, SourceBindingID: sourceID, RequestedSource: envelope.Source.RequestedSource,
		SourceKind: envelope.Source.SourceKind, SourceURL: envelope.Source.SourceURL, ArchiveSHA256: envelope.Source.ArchiveSHA256,
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
