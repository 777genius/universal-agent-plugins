package environment

import (
	"context"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"sort"
)

var ErrSourceChange = errors.New("source_change")

// Resolver performs one direct source decision. Previous pins are authoritative
// for unchanged selectors; updateSelection is the only moving-ref refresh path.
type Resolver interface {
	Resolve(context.Context, Requirement, *LockedPackage, bool) (LockedPackage, error)
}

// BindingMutator is the single-target installer boundary. It owns environment
// receipt persistence with managed effects and reports terminal outcomes.
type BindingMutator interface {
	Prepare(context.Context, PreparedBinding) (*installer.PreparedOperation, error)
	Apply(context.Context, *installer.PreparedOperation) (installer.Result, error)
}

type Coordinator struct {
	Resolver Resolver
	Mutator  BindingMutator
}

func (coordinator Coordinator) Resolve(ctx context.Context, requirements Requirements, previous Lock, updateAlias string) (Lock, error) {
	if err := requirements.Validate(); err != nil {
		return Lock{}, err
	}
	if previous.LockVersion != 0 {
		if err := previous.ValidateCompatiblePins(requirements); err != nil {
			return Lock{}, fmt.Errorf("compatible pins required: %w", err)
		}
	}
	if updateAlias != "" {
		if _, ok := requirements.Dependencies[updateAlias]; !ok {
			return Lock{}, fmt.Errorf("unknown update alias %q", updateAlias)
		}
		selected := requirements.Dependencies[updateAlias]
		if selected.Git == nil || selected.Git.Selector.Kind == SelectorCommit {
			return Lock{}, errors.New("exact commit/release pins must be changed in the manifest")
		}
	}
	digest, _ := ManifestDigest(requirements)
	out := Lock{LockVersion: LockVersion, ManifestDigest: digest}
	for _, alias := range sortedKeys(requirements.Dependencies) {
		old, hadOld := previous.Package(alias)
		if !hadOld {
			old = LockedPackage{}
		}
		resolved := old
		key, _ := requirements.Dependencies[alias].SourceKey()
		compatible := hadOld && old.SourceKey == key && validateLockedSelector(requirements.Dependencies[alias], old) == nil
		if !compatible || alias == updateAlias {
			if coordinator.Resolver == nil {
				return Lock{}, errors.New("resolver is required")
			}
			var err error
			resolved, err = coordinator.Resolver.Resolve(ctx, requirements.Dependencies[alias], pinOrNil(old, compatible), alias == updateAlias)
			if err != nil {
				return Lock{}, err
			}
		}
		resolved.Alias = alias
		resolved.Targets = targetsFor(requirements, alias)
		out.Packages = append(out.Packages, resolved)
	}
	return out, out.Validate(requirements)
}

func pinOrNil(pkg LockedPackage, exists bool) *LockedPackage {
	if !exists {
		return nil
	}
	copy := pkg
	return &copy
}

func targetsFor(requirements Requirements, alias string) []string {
	var targets []string
	for target, aliases := range requirements.Targets {
		for _, item := range aliases {
			if item == alias {
				targets = append(targets, target)
				break
			}
		}
	}
	sort.Strings(targets)
	return targets
}

// PrepareSync derives stable single-target actions. A changed canonical source
// is refused instead of silently rebinding an installation to another source.
func (coordinator Coordinator) PrepareSync(requirements Requirements, lock Lock, observed []BindingObservation) (PreparedSync, error) {
	if err := lock.Validate(requirements); err != nil {
		return PreparedSync{}, err
	}
	plan := PreparedSync{EnvironmentID: EnvironmentUserDefault, ManifestDigest: lock.ManifestDigest}
	byAliasTarget := map[string]BindingObservation{}
	for _, item := range observed {
		byAliasTarget[item.Alias+"\x00"+item.ClientID] = item
	}
	for _, pkg := range lock.Packages {
		for _, target := range pkg.Targets {
			current := byAliasTarget[pkg.Alias+"\x00"+target]
			if current.Drift {
				return PreparedSync{}, fmt.Errorf("drift: %s/%s; preserve edits and explicitly repair the recorded pin", pkg.Alias, target)
			}
			if current.Pending {
				return PreparedSync{}, installer.ErrRecoveryRequired
			}
			for _, existing := range observed {
				if existing.Managed && existing.Alias == pkg.Alias && existing.SourceKey != pkg.SourceKey {
					return PreparedSync{}, ErrSourceChange
				}
				if existing.Managed && existing.Alias != pkg.Alias && existing.SourceKey == pkg.SourceKey {
					return PreparedSync{}, ErrSourceChange
				}
				if current.InstallationID == "" && existing.Managed && existing.Alias == pkg.Alias {
					current.InstallationID = existing.InstallationID
				}
			}
			action, reason := syncAction(pkg, current)
			plan.Bindings = append(plan.Bindings, PreparedBinding{
				Alias: pkg.Alias, ClientID: target, InstallationID: current.InstallationID,
				BindingID: current.BindingID, Action: action, Package: &pkg, Reason: reason,
				Authority: bindingAuthority(pkg.Alias, target, pkg.SourceKey, action, current),
			})
		}
	}
	for key, current := range byAliasTarget {
		desired := false
		for _, item := range plan.Bindings {
			if item.Alias+"\x00"+item.ClientID == key {
				desired = true
			}
		}
		if !desired && current.Managed {
			if current.Drift {
				return PreparedSync{}, fmt.Errorf("drift: %s/%s", current.Alias, current.ClientID)
			}
			if current.Pending {
				return PreparedSync{}, installer.ErrRecoveryRequired
			}
			plan.Bindings = append(plan.Bindings, PreparedBinding{Alias: current.Alias, ClientID: current.ClientID, InstallationID: current.InstallationID, BindingID: current.BindingID, Action: ActionRemove, Authority: bindingAuthority(current.Alias, current.ClientID, current.SourceKey, ActionRemove, current)})
		}
	}
	sort.SliceStable(plan.Bindings, func(i, j int) bool {
		if plan.Bindings[i].Alias != plan.Bindings[j].Alias {
			return plan.Bindings[i].Alias < plan.Bindings[j].Alias
		}
		return plan.Bindings[i].ClientID < plan.Bindings[j].ClientID
	})
	return plan, nil
}

func syncAction(pkg LockedPackage, observed BindingObservation) (DesiredAction, string) {
	if !observed.Managed {
		return ActionInstall, "no matching environment-owned binding"
	}
	if observed.SourceKey != "" && observed.SourceKey != pkg.SourceKey {
		return "", "source_change"
	}
	revision := ""
	if pkg.Source.Git != nil {
		revision = pkg.Source.Git.Commit
	} else if pkg.Source.Registry != nil {
		revision = pkg.Source.Registry.Commit
	}
	if observed.PackageDigest == pkg.TreeDigest && observed.Revision == revision {
		return ActionUnchanged, "pinned package digest already matches"
	}
	return ActionUpdate, "pinned package digest differs"
}

// ApplySync prepares each binding afresh after the previous commit. It stops
// on nonterminal outcomes and retains the actual SDK result for partial retry.
func (coordinator Coordinator) ApplySync(ctx context.Context, plan PreparedSync, confirmed bool) ([]installer.Result, error) {
	if !confirmed {
		return nil, errors.New("sync requires explicit confirmation")
	}
	var results []installer.Result
	for _, item := range plan.Bindings {
		if item.Action == "" {
			return results, ErrSourceChange
		}
		if item.Action == ActionUnchanged {
			continue
		}
		prepared, err := coordinator.Mutator.Prepare(ctx, item)
		if err != nil {
			return results, fmt.Errorf("prepare %s/%s: %w", item.Alias, item.ClientID, err)
		}
		result, err := coordinator.Mutator.Apply(ctx, prepared)
		_ = prepared.Close()
		results = append(results, result)
		if err != nil {
			return results, err
		}
		if result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
			return results, fmt.Errorf("%s/%s: %s: %s", item.Alias, item.ClientID, result.Outcome, result.Reason)
		}
	}
	return results, nil
}

func bindingAuthority(alias, target, source string, action DesiredAction, observed BindingObservation) *domain.EnvironmentAuthority {
	out := &domain.EnvironmentAuthority{EnvironmentID: EnvironmentUserDefault, Alias: alias, ClientID: target, InstallationID: observed.InstallationID, ClientBindingID: observed.BindingID, SourceKey: source, ExpectedAction: string(action)}
	if observed.Receipt != nil {
		out.ReceiptID = observed.Receipt.ReceiptID
	}
	return out
}
