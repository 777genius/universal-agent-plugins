package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// prepareSavedUpdates is the source/graph decision shared by CLI and Management.
// Callers supply a disposable preview; this function never publishes desired files.
func prepareSavedUpdates(ctx context.Context, app App, snapshot environmentSnapshot, aliases, clients []string) (environment.Requirements, environment.Lock, error) {
	requirements := cloneSavedRequirements(snapshot.Requirements)
	moving := []string{}
	changed := false
	state, err := app.StateStore.Load()
	if err != nil {
		return requirements, snapshot.Lock, err
	}
	for _, alias := range aliases {
		req, exists := requirements.Dependencies[alias]
		if !exists {
			return requirements, snapshot.Lock, fmt.Errorf("dependency %q is not declared", alias)
		}
		if len(clients) > 0 && !slices.Equal(savedAliasTargets(requirements, domain.StateFileV2{}, alias, app.environmentID()), clients) {
			return requirements, snapshot.Lock, errors.New("update must include every target of each selected alias")
		}
		if environment.MovingRequirement(req) {
			moving = append(moving, alias)
		}
		if req.Registry == nil || req.Registry.Selector.Kind != environment.SelectorRelease {
			continue
		}
		nodeAlias := dependencyNodeAlias(requirements, snapshot.Lock, alias)
		installation, err := savedAliasInstallation(state, nodeAlias, nodeAlias, app.environmentID())
		if err != nil {
			return requirements, snapshot.Lock, err
		}
		var selected bool
		requirements, selected, err = prepareSavedRegistryUpdate(ctx, app, true, environmentSnapshot{Requirements: requirements, Lock: snapshot.Lock}, nodeAlias, installation)
		if err != nil {
			return requirements, snapshot.Lock, err
		}
		changed = changed || selected
	}
	if len(moving) == 0 && !changed {
		return requirements, snapshot.Lock, ctx.Err()
	}
	next, err := (environment.Coordinator{EnvironmentID: app.environmentID(), Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: true}}).ResolveSelected(ctx, requirements, snapshot.Lock, moving)
	if err == nil {
		err = refuseSavedDowngrades(snapshot.Lock, next)
	}
	return requirements, next, err
}

func refuseSavedDowngrades(previous, next environment.Lock) error {
	for _, old := range previous.Packages {
		for _, pkg := range next.Packages {
			if old.SourceKey == pkg.SourceKey && stablePackageVersion(old.Version) && stablePackageVersion(pkg.Version) && environment.CompareVersions(pkg.Version, old.Version) < 0 {
				return fmt.Errorf("update would downgrade %s from %s to %s; change the declared intent explicitly", old.Alias, old.Version, pkg.Version)
			}
		}
	}
	return nil
}
func stablePackageVersion(version string) bool {
	return environment.SelectorMatchesVersion(environment.Selector{Kind: environment.SelectorVersion, Value: version}, version)
}
func savedRequirementPinned(req environment.Requirement) bool {
	return !environment.MovingRequirement(req) && (req.Registry == nil || req.Registry.Selector.Kind == environment.SelectorVersion)
}

type savedUpdateFact struct {
	Alias      string                   `json:"alias"`
	Declared   bool                     `json:"declared"`
	Selected   bool                     `json:"selected"`
	Source     string                   `json:"source"`
	Requested  environment.Selector     `json:"requested"`
	Status     string                   `json:"status"`
	Reason     string                   `json:"reason"`
	Installed  *outdatedReleaseIdentity `json:"installed,omitempty"`
	Available  *outdatedReleaseIdentity `json:"available,omitempty"`
	RequiredBy []environment.RequiredBy `json:"required_by,omitempty"`
}
type savedUpdateCheck struct {
	Complete bool              `json:"complete"`
	Changed  bool              `json:"changed"`
	Reason   string            `json:"reason,omitempty"`
	Entries  []savedUpdateFact `json:"entries"`
}

// checkSavedUpdates resolves one complete candidate graph and exposes both root
// source facts and closure changes. It does not approve any installer effects.
func checkSavedUpdates(ctx context.Context, app App, snapshot environmentSnapshot, aliases []string) (savedUpdateCheck, error) {
	out := savedUpdateCheck{Entries: []savedUpdateFact{}}
	for _, alias := range aliases {
		req := snapshot.Requirements.Dependencies[alias]
		pin, _ := snapshot.Lock.PackageForRequirement(req)
		status, reason := "check_unavailable", "the source has not been checked"
		if savedRequirementPinned(req) {
			status, reason = "pinned", "exact intent is retained; change the declaration to select a new pin"
		}
		out.Entries = append(out.Entries, savedUpdateFact{Alias: pin.Alias, Declared: true, Selected: true, Source: pin.SourceKey, Requested: req.RequestedSelector(), Status: status, Reason: reason, Installed: savedPinIdentity(pin), RequiredBy: pin.RequiredBy})
	}
	// A fresh cache belongs exclusively to this read-only request.
	app.dependencySnapshots = nil
	preview, cleanup, err := dependencyPreviewApp(app, &options{dryRun: true})
	if err != nil {
		return out, err
	}
	defer cleanup()
	_, next, err := prepareSavedUpdates(ctx, preview, snapshot, aliases, nil)
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if err != nil {
		out.Reason = "The complete update graph could not be checked; inspect source access and dependency constraints"
		status := "check_unavailable"
		if strings.Contains(err.Error(), "dependency conflict") || strings.Contains(err.Error(), "downgrade") || errors.Is(err, domain.ErrDirectoryIneligible) {
			status = "blocked"
			out.Reason = "No permitted complete update graph is available; inspect dependency constraints and release eligibility"
		}
		for i := range out.Entries {
			if out.Entries[i].Status != "pinned" {
				out.Entries[i].Status = status
				out.Entries[i].Reason = out.Reason
			}
		}
		return out, nil
	}
	out.Complete = true
	selected := map[string]bool{}
	for i := range out.Entries {
		item := &out.Entries[i]
		selected[item.Alias] = true
		if item.Status == "pinned" {
			continue
		}
		old, _ := snapshot.Lock.Package(item.Alias)
		pkg, exists := next.Package(item.Alias)
		if !exists {
			item.Status = "blocked"
			item.Reason = "selected dependency has no candidate identity"
			out.Complete = false
			continue
		}
		item.Status, item.Reason = "current", "the requested source and permitted immutable identity are current"
		if !sameSavedIdentity(old, pkg) {
			item.Status, item.Reason = "available", "a different permitted immutable source identity is available"
			item.Available = savedPinIdentity(pkg)
			out.Changed = true
		}
	}
	closure := savedClosureAliases(snapshot.Lock, next, declaredNodeAliases(snapshot.Requirements, snapshot.Lock, aliases))
	for _, alias := range closure {
		if selected[alias] {
			continue
		}
		old, oldOK := snapshot.Lock.Package(alias)
		pkg, nextOK := next.Package(alias)
		if oldOK && nextOK && sameSavedIdentity(old, pkg) && slices.Equal(old.RequiredBy, pkg.RequiredBy) && slices.Equal(old.Targets, pkg.Targets) {
			continue
		}
		fact := savedUpdateFact{Alias: alias, Source: pkg.SourceKey, Status: "available", Reason: "the selected update changes this shared dependency closure", RequiredBy: pkg.RequiredBy}
		if oldOK {
			fact.Installed = savedPinIdentity(old)
			if !nextOK {
				fact.Source = old.SourceKey
				fact.Reason = "the selected graph no longer requires this dependency"
			}
		}
		if nextOK {
			fact.Available = savedPinIdentity(pkg)
		}
		out.Entries = append(out.Entries, fact)
		out.Changed = true
	}
	sort.Slice(out.Entries, func(i, j int) bool { return out.Entries[i].Alias < out.Entries[j].Alias })
	return out, nil
}
func declaredNodeAliases(requirements environment.Requirements, locked environment.Lock, aliases []string) []string {
	out := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		out = append(out, dependencyNodeAlias(requirements, locked, alias))
	}
	return out
}
func savedPinIdentity(pin environment.LockedPackage) *outdatedReleaseIdentity {
	if pin.SourceKey == "" {
		return nil
	}
	out := &outdatedReleaseIdentity{PackageVersion: pin.Version, Revision: lockedCommit(pin), TreeDigest: pin.TreeDigest, ManifestDigest: pin.ManifestDigest}
	if pin.Source.Registry != nil {
		out.DistributionID = pin.Source.Registry.DistributionID
		out.ReleaseSequence = pin.Source.Registry.ReleaseSequence
	}
	if pin.Source.Archive != nil {
		out.Revision = "sha256:" + pin.Source.Archive.SHA256
	}
	if pin.Source.Web != nil {
		out.Revision = pin.Source.Web.RawTreeDigest
	}
	return out
}

func sameSavedIdentity(previous, next environment.LockedPackage) bool {
	if previous.Version == "" {
		previous.Version = next.Version
	}
	if len(previous.ExecutableFiles) == 0 {
		previous.ExecutableFiles = nil
	}
	if len(next.ExecutableFiles) == 0 {
		next.ExecutableFiles = nil
	}
	return environment.SamePin(previous, next)
}
