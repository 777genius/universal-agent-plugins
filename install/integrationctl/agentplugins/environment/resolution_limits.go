package environment

import (
	"context"
	"errors"
	"fmt"
)

// ErrResolutionLimit reports resource admission or exhausted resolution work.
// It is terminal, not a dependency conflict eligible for candidate backtracking.
var ErrResolutionLimit = errors.New("dependency resolution limit exceeded")

type resolutionLimits struct {
	nodes, demands, states, work int
}

func defaultResolutionLimits() resolutionLimits {
	// These bound fresh search work, not the validity of a recorded agreement.
	// Historical pins must remain inspectable and removable without re-solving.
	return resolutionLimits{nodes: 1024, demands: 8192, states: 4096, work: 8388608}
}

// One budget belongs to the entire resolution, including failed candidates.
// Limits are values rather than mutable global configuration.
type resolutionBudget struct {
	limits       resolutionLimits
	states, work int
}

func resolutionLimit(kind string, maximum int) error {
	return fmt.Errorf("%w: %s exceeds %d", ErrResolutionLimit, kind, maximum)
}

func (budget *resolutionBudget) enterSearch(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if budget.states >= budget.limits.states {
		return resolutionLimit("search states", budget.limits.states)
	}
	budget.states++
	return nil
}

func (budget *resolutionBudget) consumeWork(ctx context.Context, amount int) error {
	if err := budget.checkWork(ctx, amount); err != nil {
		return err
	}
	budget.work += amount
	return nil
}

func (budget *resolutionBudget) checkWork(ctx context.Context, amount int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if amount < 0 || amount > budget.limits.work-budget.work {
		return resolutionLimit("work events", budget.limits.work)
	}
	return nil
}

func (limits resolutionLimits) admitGraph(nodes, demands int) error {
	if nodes > limits.nodes {
		return resolutionLimit("source nodes", limits.nodes)
	}
	if demands > limits.demands {
		return resolutionLimit("graph demands", limits.demands)
	}
	return nil
}
