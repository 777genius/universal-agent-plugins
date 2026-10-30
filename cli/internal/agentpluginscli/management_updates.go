package agentpluginscli

import (
	"context"
	"strconv"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

// CheckUpdates is explicit network work. Inspect remains a passive snapshot.
func (b *ManagementBackend) CheckUpdates(ctx context.Context, input management.UpdateCheckRequest) (management.UpdateCheck, error) {
	out := management.UpdateCheck{ContextId: b.ContextID(), Entries: []management.UpdateCheckEntry{}, Diagnostics: []management.Diagnostic{}}
	selected := []string{}
	if input.Aliases != nil {
		selected = *input.Aliases
	}
	if err := validateManagementChange(management.ChangeRequest{Action: management.Update, Aliases: selected, All: input.All}); err != nil {
		return out, err
	}
	if err := b.validateManagementContext(ctx); err != nil {
		return out, err
	}
	snapshot, err := snapshotEnvironmentContext(ctx, b.app, false)
	if err != nil {
		return out, err
	}
	aliases, err := managementAliases(snapshot.Requirements, snapshot.Lock, selected, input.All)
	if err != nil {
		return out, err
	}
	facts, err := checkSavedUpdates(ctx, b.app, snapshot, aliases)
	if err != nil {
		return out, err
	}
	if err := b.validateManagementContext(ctx); err != nil {
		return out, err
	}
	if err := snapshot.intent(nil, nil).Check(dependencyIntentFiles{}); err != nil {
		return out, management.ErrPlanStale
	}
	out.Complete, out.Changed = facts.Complete, facts.Changed
	if facts.Reason != "" {
		out.Diagnostics = append(out.Diagnostics, managementDiagnostic("UPDATE_CHECK_UNAVAILABLE", facts.Reason))
	}
	for _, fact := range facts.Entries {
		entry := management.UpdateCheckEntry{Alias: fact.Alias, Declared: fact.Declared, Selected: fact.Selected, Source: fact.Source, Status: management.UpdateCheckEntryStatus(fact.Status), Reason: fact.Reason, Installed: managementUpdateIdentity(fact.Installed), Available: managementUpdateIdentity(fact.Available), RequiredBy: []management.UpdateOwner{}}
		if fact.Declared {
			entry.Requested = &management.UpdateSelector{Kind: string(fact.Requested.Kind), Value: fact.Requested.Value}
		}
		for _, owner := range fact.RequiredBy {
			entry.RequiredBy = append(entry.RequiredBy, management.UpdateOwner{Alias: owner.Alias, Selector: management.UpdateSelector{Kind: string(owner.Selector.Kind), Value: owner.Selector.Value}})
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}
func managementUpdateIdentity(value *outdatedReleaseIdentity) *management.UpdateIdentity {
	if value == nil {
		return nil
	}
	out := &management.UpdateIdentity{Version: value.PackageVersion, Revision: value.Revision, TreeDigest: value.TreeDigest, ManifestDigest: value.ManifestDigest, DistributionId: value.DistributionID}
	if value.ReleaseSequence > 0 {
		out.ReleaseSequence = strconv.FormatUint(value.ReleaseSequence, 10)
	}
	return out
}
