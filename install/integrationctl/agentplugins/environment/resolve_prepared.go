package environment

import (
	"context"
	"fmt"
)

// Source facts borrow immutable provider values; binding aliases belong to the
// active graph. Strict declaration validation occurs before canonical key reuse.
type preparedDependency struct {
	key         string
	requirement Requirement
}

func (solver *graphSolver) prepareDependencies(ctx context.Context, dependencies map[string]Requirement) ([]preparedDependency, error) {
	if len(dependencies) > solver.budget.limits.demands {
		return nil, resolutionLimit("graph demands", solver.budget.limits.demands)
	}
	if err := solver.budget.checkWork(ctx, len(dependencies)); err != nil {
		return nil, err
	}
	facts := make([]preparedDependency, 0, len(dependencies))
	for _, declaration := range sortedKeys(dependencies) {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return nil, err
		}
		req := dependencies[declaration]
		if declaration != req.Alias {
			return nil, fmt.Errorf("dependency key %q does not match declaration %q", declaration, req.Alias)
		}
		key, err := req.SourceKey()
		if err != nil {
			return nil, fmt.Errorf("dependency %q: %w", declaration, err)
		}
		facts = append(facts, preparedDependency{key: key, requirement: req})
	}
	return facts, nil
}
