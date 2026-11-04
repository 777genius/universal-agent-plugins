package environment

import "context"

func exactReleaseRequirement(req Requirement) bool {
	return req.Registry != nil && req.Registry.Selector.Kind == SelectorRelease
}

func sameSourceRequirement(a, b Requirement) bool {
	switch {
	case a.Registry != nil && b.Registry != nil:
		return *a.Registry == *b.Registry
	case a.Git != nil && b.Git != nil:
		return *a.Git == *b.Git
	case a.Archive != nil && b.Archive != nil:
		return *a.Archive == *b.Archive
	case a.Web != nil && b.Web != nil:
		return *a.Web == *b.Web
	default:
		return false
	}
}

func (solver *graphSolver) representativeRequirement(ctx context.Context, key string, node graphNode) (Requirement, error) {
	exact := exactReleaseRequirement(node.requirement)
	if root, direct := solver.roots[key]; direct && (!exact || exactReleaseRequirement(root.requirement)) {
		req := root.requirement
		req.Alias = node.requirement.Alias
		return req, nil
	}
	// Uniform shared graphs need no traversal. Only competing distinct source
	// requests can make incremental discovery differ from the original root BFS.
	var first Requirement
	found := false
	for _, demand := range node.demands {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return Requirement{}, err
		}
		req := demand.requirement
		if exact && !exactReleaseRequirement(req) {
			continue
		}
		if found && !sameSourceRequirement(first, req) {
			return solver.firstPreparedRequirement(ctx, key, node.requirement.Alias, exact)
		}
		first, found = req, true
	}
	first.Alias = node.requirement.Alias
	return first, nil
}

func (solver *graphSolver) firstPreparedRequirement(ctx context.Context, wanted, alias string, exact bool) (Requirement, error) {
	queue := sortedKeys(solver.roots)
	seen := make(map[string]bool, len(solver.active.nodes))
	for _, key := range queue {
		seen[key] = true
	}
	for len(queue) > 0 {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return Requirement{}, err
		}
		key := queue[0]
		queue = queue[1:]
		for _, fact := range solver.active.prepared[key] {
			if err := solver.budget.consumeWork(ctx, 1); err != nil {
				return Requirement{}, err
			}
			if fact.key == wanted && (!exact || exactReleaseRequirement(fact.requirement)) {
				req := fact.requirement
				req.Alias = alias
				return req, nil
			}
			if !seen[fact.key] {
				seen[fact.key] = true
				queue = append(queue, fact.key)
			}
		}
	}
	return Requirement{}, ErrDependencyFactsUnavailable
}

func (solver *graphSolver) finalizeRepresentatives(ctx context.Context) error {
	for _, key := range solver.active.keys {
		node := solver.active.nodes[key]
		req, err := solver.representativeRequirement(ctx, key, node)
		if err != nil {
			return err
		}
		node.requirement = req
		solver.active.nodes[key] = node
	}
	return nil
}
