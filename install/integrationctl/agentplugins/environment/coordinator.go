package environment

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

var ErrSourceChange = errors.New("source_change")

// Resolver performs one direct source decision. Previous pins are authoritative
// for unchanged selectors; updateSelection is the only moving-ref refresh path.
type Resolver interface {
	Resolve(context.Context, Requirement, []string, *LockedPackage, bool) (LockedPackage, error)
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
		selected, ok := requirements.Dependencies[updateAlias]
		if !ok {
			return Lock{}, fmt.Errorf("unknown update alias %q", updateAlias)
		}
		moving := selected.Git != nil && selected.Git.Selector.Kind != SelectorCommit
		moving = moving || selected.Registry != nil && selected.Registry.Selector.Kind == SelectorRange
		moving = moving || selected.Archive != nil && selected.Archive.SHA256 == ""
		moving = moving || selected.Web != nil
		if !moving {
			return Lock{}, errors.New("exact commit/release/version pins must be changed in the manifest")
		}
	}
	if len(requirements.Dependencies) > 0 {
		if _, ok := coordinator.Resolver.(GraphResolver); !ok {
			return Lock{}, ErrDependencyFactsUnavailable
		}
	}
	return newGraphSolver(coordinator.Resolver, requirements, previous, updateAlias).resolve(ctx)
}

func pinOrNil(pkg LockedPackage, exists bool) *LockedPackage {
	if !exists {
		return nil
	}
	cloned := pkg
	return &cloned
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
	return coordinator.prepareSync(requirements, lock, observed, nil, nil)
}

// PrepareSyncFor limits mutations to explicit aliases while retaining global identity checks.
func (coordinator Coordinator) PrepareSyncFor(requirements Requirements, lock Lock, observed []BindingObservation, aliases []string, attestation *Attestation) (PreparedSync, error) {
	if len(aliases) == 0 {
		return PreparedSync{}, errors.New("selected aliases must not be empty")
	}
	selected := map[string]bool{}
	for _, alias := range aliases {
		if !validDeclarationKey(alias) {
			return PreparedSync{}, errors.New("invalid selected alias")
		}
		if requirement, ok := requirements.Dependencies[alias]; ok {
			pkg, found := lock.PackageForRequirement(requirement)
			if !found {
				return PreparedSync{}, fmt.Errorf("selected declaration %q has no locked source", alias)
			}
			alias = pkg.Alias
		}
		selected[alias] = true
	}
	if attestation != nil && (!selected[attestation.Alias] || attestation.ClientID == "" || (!attestation.ActivationComplete && !attestation.AuthComplete)) {
		return PreparedSync{}, errors.New("attestation requires its selected alias and client")
	}
	return coordinator.prepareSync(requirements, lock, observed, selected, attestation)
}

func (coordinator Coordinator) prepareSync(requirements Requirements, lock Lock, observed []BindingObservation, selected map[string]bool, attestation *Attestation) (PreparedSync, error) {
	if err := lock.Validate(requirements); err != nil {
		return PreparedSync{}, err
	}
	plan := PreparedSync{EnvironmentID: EnvironmentUserDefault, ManifestDigest: lock.ManifestDigest}
	byAliasTarget := map[string]BindingObservation{}
	for _, item := range observed {
		byAliasTarget[item.Alias+"\x00"+item.ClientID] = item
	}
	if err := appendDesiredBindings(&plan, lock, observed, byAliasTarget, selected, attestation); err != nil {
		return PreparedSync{}, err
	}

	for key, current := range byAliasTarget {
		if selected != nil && !selected[current.Alias] {
			continue
		}
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
	if err := validatePlannedAttestation(plan, attestation); err != nil {
		return PreparedSync{}, err
	}

	sortSyncBindings(plan.Bindings)
	return plan, nil
}

func syncAction(pkg LockedPackage, observed BindingObservation) (DesiredAction, string) {
	if !observed.Managed {
		return ActionInstall, "no matching environment-owned binding"
	}
	if observed.SourceKey != "" && observed.SourceKey != pkg.SourceKey {
		return "", "source_change"
	}
	revision := lockedRevision(pkg)
	if observed.PackageDigest == pkg.TreeDigest && observed.Revision == revision && !observed.Incomplete {
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

func validateDesiredBinding(pkg LockedPackage, target string, current BindingObservation, observed []BindingObservation) (BindingObservation, error) {
	if current.Drift {
		return current, fmt.Errorf("drift: %s/%s; preserve edits and explicitly repair the recorded pin", pkg.Alias, target)
	}
	if current.Pending {
		return current, installer.ErrRecoveryRequired
	}
	for _, existing := range observed {
		if !existing.Managed && existing.SourceKey == pkg.SourceKey {
			return current, fmt.Errorf("foreign: %s has an unmanaged binding in installation %s; remove it before adding the dependency", pkg.Alias, existing.InstallationID)
		}
		if existing.Managed && existing.Alias == pkg.Alias && existing.SourceKey != pkg.SourceKey {
			return current, ErrSourceChange
		}
		if existing.Managed && existing.Alias != pkg.Alias && existing.SourceKey == pkg.SourceKey {
			return current, ErrSourceChange
		}
		if current.InstallationID == "" && existing.Managed && existing.Alias == pkg.Alias {
			current.InstallationID = existing.InstallationID
		}
	}
	return current, nil
}

func sortSyncBindings(bindings []PreparedBinding) {
	sort.SliceStable(bindings, func(i, j int) bool {
		if bindings[i].Alias != bindings[j].Alias {
			return bindings[i].Alias < bindings[j].Alias
		}
		// Update the shared installation pin before attaching another target.
		if (bindings[i].Action == ActionInstall) != (bindings[j].Action == ActionInstall) {
			return bindings[j].Action == ActionInstall
		}
		return bindings[i].ClientID < bindings[j].ClientID
	})
}

func lockedRevision(pkg LockedPackage) string {
	if pkg.Source.Web != nil {
		return pkg.Source.Web.RawTreeDigest
	}
	if pkg.Source.Archive != nil {
		return "sha256:" + pkg.Source.Archive.SHA256
	}
	if pkg.Source.Git != nil {
		return pkg.Source.Git.Commit
	}
	if pkg.Source.Registry != nil {
		return pkg.Source.Registry.Commit
	}
	return ""
}

func appendDesiredBindings(plan *PreparedSync, lock Lock, observed []BindingObservation, byAliasTarget map[string]BindingObservation, selected map[string]bool, attestation *Attestation) error {
	for _, pkg := range lock.Packages {
		if selected != nil && !selected[pkg.Alias] {
			continue
		}
		for _, target := range pkg.Targets {
			current := byAliasTarget[pkg.Alias+"\x00"+target]
			var err error
			current, err = validateDesiredBinding(pkg, target, current, observed)
			if err != nil {
				return err
			}
			action, reason := syncAction(pkg, current)
			if action == ActionUnchanged && len(pkg.RequiredBy) > 0 {
				reason = retainedReason(pkg)
			}
			activation, auth := false, false
			if attestation != nil && attestation.Alias == pkg.Alias && attestation.ClientID == target {
				if !current.Managed || current.Pending || current.PackageDigest != pkg.TreeDigest || current.Revision != lockedRevision(pkg) {
					return errors.New("completion attestation requires the materialized owned pin; update first then attest")
				}
				activation, auth = attestation.ActivationComplete, attestation.AuthComplete
				action, reason = ActionUpdate, "explicit_lifecycle_attestation"
			}
			plan.Bindings = append(plan.Bindings, PreparedBinding{
				ActivationComplete: activation, AuthComplete: auth,
				Alias: pkg.Alias, ClientID: target, InstallationID: current.InstallationID,
				BindingID: current.BindingID, Action: action, Package: &pkg, Reason: reason,
				Authority: bindingAuthority(pkg.Alias, target, pkg.SourceKey, action, current),
			})
		}
	}
	return nil
}

func validatePlannedAttestation(plan PreparedSync, attestation *Attestation) error {
	if attestation != nil {
		found := false
		for _, binding := range plan.Bindings {
			if binding.Alias == attestation.Alias && binding.ClientID == attestation.ClientID && binding.Action != ActionRemove {
				found = true
			}
		}
		if !found {
			return errors.New("attestation requires a desired binding")
		}
	}
	return nil
}
