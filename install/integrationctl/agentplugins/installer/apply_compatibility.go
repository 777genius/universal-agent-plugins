package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) updateWithCompatibility(ctx context.Context, svc usecase.Service, req Request, in usecase.AddInput) (usecase.AddResult, error) {
	checks, err := e.compatibilityChecks(req, in)
	if err != nil {
		return usecase.AddResult{}, err
	}
	if len(checks) == 1 {
		return svc.Update(ctx, in)
	}
	got, err := svc.UpdateGroup(ctx, usecase.GroupInput{
		Targets:             []usecase.AddInput{in},
		CompatibilityChecks: checks,
		OperationGroupID:    firstNonEmpty(req.OperationID, "update"),
		DryRun:              in.DryRun,
		Confirmed:           in.Confirmed,
	})
	for _, target := range got.Targets {
		if target.Plan.ClientID == in.Client.ClientID || target.Plan.ClientID == domain.ClientID(req.ClientID) {
			return target, err
		}
	}
	if len(got.Targets) > 0 {
		return got.Targets[0], err
	}
	return usecase.AddResult{InstallationID: got.InstallationID, Mutated: got.Mutated}, err
}

func (e *Engine) compatibilityChecks(req Request, target usecase.AddInput) ([]usecase.AddInput, error) {
	if req.InstallationID == "" {
		return []usecase.AddInput{target}, nil
	}
	state, err := e.store.Load()
	if err != nil {
		return nil, fmt.Errorf("%w: load owned state: %w", ErrTargetFactsUnavailable, err)
	}
	installation, ok := findInstall(state, req.InstallationID)
	if !ok {
		return []usecase.AddInput{target}, nil
	}
	selected := make(map[string]struct{}, len(req.Targets))
	for _, candidate := range req.Targets {
		selected[candidate.ClientID] = struct{}{}
	}
	known, err := e.knownTargetIndex(req.Operation, req.KnownTargets)
	if err != nil {
		return nil, err
	}
	var checks []usecase.AddInput
	for _, binding := range installation.Clients {
		if binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		// Group operations already carry explicit request-scoped facts for every
		// selected target. Only bindings outside that group need KnownTargets;
		// treating selected siblings as unknown would make a valid two-client
		// repair/update impossible when ClientID is intentionally empty.
		if _, ok := selected[binding.ClientID]; ok {
			continue
		}
		configRoot := req.ClientConfigRoot
		executable := req.ClientExecutable
		if binding.ClientID != req.ClientID {
			facts, ok := known[binding.ClientID]
			if !ok {
				return nil, fmt.Errorf("%w: binding %s (%s) has no host facts", ErrTargetFactsUnavailable, binding.ClientBindingID, binding.ClientID)
			}
			if facts.BindingID != binding.ClientBindingID {
				return nil, fmt.Errorf("%w: binding %s (%s) does not match host binding %s", ErrTargetFactsUnavailable, binding.ClientBindingID, binding.ClientID, facts.BindingID)
			}
			configRoot = facts.ConfigRoot
			executable = facts.Executable
		}
		check, err := e.compatibilityBindingCheck(binding, configRoot, executable, target)
		if err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return checks, nil
}

// groupCompatibilityChecks adds observation-only inputs for unselected live
// bindings to a group update. Targets remains the exact user's selection.
// The existing single-client compatibility path verifies every sibling's
// KnownTargets BindingID against durable state and supplies recorded authority.
func (e *Engine) groupCompatibilityChecks(req Request, targets []usecase.AddInput) ([]usecase.AddInput, error) {
	if req.Operation != OpUpdate {
		return targets, nil
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("%w: update group has no selected targets", ErrInvalidRequest)
	}
	observed, err := e.compatibilityChecks(req, targets[0])
	if err != nil {
		return nil, err
	}
	selected := make(map[domain.ClientID]bool, len(targets))
	checks := append([]usecase.AddInput(nil), targets...)
	for _, target := range targets {
		selected[target.Client.ClientID] = true
	}
	for _, check := range observed {
		// A missing new installation falls back to the first target; it must
		// not make a duplicate compatibility-only input for that selection.
		if !selected[check.Client.ClientID] {
			checks = append(checks, check)
		}
	}
	return checks, nil
}

func (e *Engine) compatibilityBindingCheck(binding domain.ClientBinding, configRoot, executable string, target usecase.AddInput) (usecase.AddInput, error) {
	client, err := e.detectedClient(Request{
		Operation: OpUpdate, ClientID: binding.ClientID, ClientConfigRoot: configRoot,
		ClientExecutable: executable,
	})
	if err != nil {
		return usecase.AddInput{}, fmt.Errorf("%w: binding %s (%s): %w", ErrTargetFactsUnavailable, binding.ClientBindingID, binding.ClientID, err)
	}
	check := target
	check.Client = client
	// Compatibility planning and activation preflight observe the same
	// recorded sibling owner that the shared-profile guard revalidates.
	check.Client.ProfileAuthority = domain.CloneProfileAuthority(binding.ProfileAuthority)
	check.Client.ProfileNamespace = binding.ProfileNamespace
	if client.ClientID == target.Client.ClientID {
		// The selected client's prepared authority must survive update's
		// compatibility preview; sibling checks remain observational.
		check.Client = target.Client
	}
	check.BackendExecutable = client.ExecutablePath
	return check, nil
}

func attachNextActions(result *Result) {
	if result == nil || len(result.NextActions) > 0 {
		return
	}
	switch {
	case result.Outcome == OutcomeRecovery:
		result.NextActions = []NextAction{{Kind: "recover", Reason: result.Reason}}
	case result.Reason == "update_required":
		result.NextActions = []NextAction{{Kind: "update", Reason: result.Reason}}
	case result.Reason == "plan_changed":
		result.NextActions = []NextAction{{Kind: "reprepare", Reason: result.Reason}}
	case result.Reason == "target_facts_unavailable":
		result.NextActions = []NextAction{{Kind: "reprepare", Reason: result.Reason}}
	case result.Outcome == OutcomeIncomplete && result.Client.Materialization != "" && result.Client.Materialization != string(domain.MaterializationAbsent):
		result.NextActions = []NextAction{{Kind: "activate", Reason: result.Reason}}
	}
}

func (p *PreparedOperation) beginApply(decision Decision) (Operation, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return "", ErrHandleClosed
	}
	if p.applied {
		p.mu.Unlock()
		return "", ErrAlreadyApplied
	}
	if p.busy {
		p.mu.Unlock()
		return "", ErrHandleBusy
	}
	if !decision.Confirmed {
		op := p.req.Operation
		p.mu.Unlock()
		return op, ErrCancelled
	}
	p.busy = true
	op := p.req.Operation
	p.mu.Unlock()
	return op, nil
}

func (e *Engine) inspectRemovalOutcome(result *Result, bindingID string) {
	if view, inspectErr := e.observe(); inspectErr == nil {
		for _, installation := range view.Installations {
			if installation.InstallationID == result.InstallationID {
				result.DataRetained = installation.DataRetained
				for _, live := range installation.Bindings {
					if live.BindingID == bindingID {
						result.Outcome = OutcomeIncomplete
						result.NoChange = false
						result.Reason = "external removal requires explicit attestation"
						result.NextActions = []NextAction{{Kind: "external_uninstall", Reason: result.Reason}}
					}
				}
			}
		}
	}
}
