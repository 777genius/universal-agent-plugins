package environment

import (
	"context"
	"fmt"
	"slices"
	"sort"
)

type graphExpansion struct {
	nodes   map[string]graphNode
	queue   []string
	pending map[string]bool
	demands int
}

func (solver *graphSolver) expand(ctx context.Context, choices map[string]ResolvedPackage) (map[string]graphNode, error) {
	if err := solver.budget.limits.admitGraph(len(solver.roots), 0); err != nil {
		return nil, err
	}
	work := graphExpansion{nodes: map[string]graphNode{}, queue: sortedKeys(solver.roots), pending: map[string]bool{}}
	for _, key := range work.queue {
		node := solver.roots[key]
		node.requirement = solver.nodeRequirement(key, node.requirement, choices)
		node.targets = append([]string{}, node.targets...)
		node.demands = append([]graphDemand{}, node.demands...)
		work.nodes[key] = node
		work.pending[key] = true
	}
	for len(work.queue) > 0 {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return nil, err
		}
		key := work.queue[0]
		work.queue[0] = ""
		work.queue = work.queue[1:]
		work.pending[key] = false
		selected, ok := choices[key]
		if !ok {
			continue
		}
		if err := solver.expandDependencies(ctx, choices, &work, work.nodes[key], selected.Dependencies); err != nil {
			return nil, err
		}
	}
	return work.nodes, nil
}

func (solver *graphSolver) expandDependencies(ctx context.Context, choices map[string]ResolvedPackage, work *graphExpansion, parent graphNode, dependencies map[string]Requirement) error {
	if len(dependencies) > solver.budget.limits.demands {
		return resolutionLimit("graph demands", solver.budget.limits.demands)
	}
	if err := solver.budget.checkWork(ctx, len(dependencies)); err != nil {
		return err
	}
	for _, declaration := range sortedKeys(dependencies) {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return err
		}
		req, err := solver.dependencyRequirement(declaration, dependencies[declaration], choices)
		if err != nil {
			return err
		}
		if err := work.merge(parent, req, solver.budget.limits); err != nil {
			return err
		}
	}
	return nil
}

func (work *graphExpansion) merge(parent graphNode, req Requirement, limits resolutionLimits) error {
	key, _ := acquisitionSourceKey(req)
	before := len(work.nodes[key].demands)
	changed := mergeGraphDemand(work.nodes, parent, req)
	work.demands += len(work.nodes[key].demands) - before
	if err := limits.admitGraph(len(work.nodes), work.demands); err != nil {
		return err
	}
	if changed && !work.pending[key] {
		work.queue = append(work.queue, key)
		work.pending[key] = true
	}
	return nil
}

func mergeGraphDemand(nodes map[string]graphNode, parent graphNode, req Requirement) bool {
	key, _ := acquisitionSourceKey(req)
	child, exists := nodes[key]
	changed := !exists
	if !exists {
		child = graphNode{requirement: req}
	}
	demand := graphDemand{requirement: req, parent: parent.requirement.Alias}
	if !slices.ContainsFunc(child.demands, func(item graphDemand) bool {
		return item.parent == demand.parent && requirementSelector(item.requirement) == requirementSelector(req)
	}) {
		child.demands = append(child.demands, demand)
		changed = true
	}
	// An exact release fixes the candidate before intersecting SemVer demands.
	if req.Registry != nil && req.Registry.Selector.Kind == SelectorRelease && child.requirement.Registry.Selector.Kind != SelectorRelease {
		alias := child.requirement.Alias
		child.requirement = req
		child.requirement.Alias = alias
	}
	targets := unionTargets(child.targets, parent.targets)
	if !slices.Equal(targets, child.targets) {
		child.targets = targets
		changed = true
	}
	nodes[key] = child
	return changed
}

func validateGraphAliases(nodes map[string]graphNode) error {
	aliases := map[string]string{}
	for _, key := range sortedKeys(nodes) {
		node := nodes[key]
		if prior, ok := aliases[node.requirement.Alias]; ok && prior != key {
			return fmt.Errorf("dependency alias %q collides between %s and %s", node.requirement.Alias, prior, key)
		}
		aliases[node.requirement.Alias] = key
	}
	return nil
}

func requirementSelector(req Requirement) Selector {
	return req.RequestedSelector()
}

func (solver *graphSolver) nodeRequirement(key string, req Requirement, choices map[string]ResolvedPackage) Requirement {
	if pin, ok := solver.pins[key]; ok {
		req.Alias = pin.Alias
	} else if root, ok := solver.roots[key]; ok && aliasPattern.MatchString(root.requirement.Alias) {
		req.Alias = root.requirement.Alias
	} else if chosen, ok := choices[key]; ok && chosen.Package.Alias != "" {
		req.Alias = chosen.Package.Alias
	} else {
		// No owner identity exists yet. Let acquisition supply manifest.name;
		// package-local declaration names do not name environment bindings.
		if req.Registry != nil {
			req.Alias = req.Registry.Distribution
		} else if req.Git != nil && req.Git.Repository != "" {
			req.Alias = req.Git.Repository
		} else {
			req.Alias = ""
		}
	}
	return req
}

// Acquisition may know a source before its manifest supplies the binding name.
// Only this operational key uses a disposable scalar; public validation and
// every endpoint/path/selector check still run through the strict SourceKey.
func acquisitionSourceKey(req Requirement) (string, error) {
	if req.Alias == "" {
		req.Alias = "source-acquisition"
	}
	return req.SourceKey()
}

func (solver *graphSolver) dependencyRequirement(declaration string, req Requirement, choices map[string]ResolvedPackage) (Requirement, error) {
	if declaration != req.Alias {
		return Requirement{}, fmt.Errorf("dependency key %q does not match declaration %q", declaration, req.Alias)
	}
	key, err := req.SourceKey()
	if err != nil {
		return Requirement{}, fmt.Errorf("dependency %q: %w", declaration, err)
	}
	return solver.nodeRequirement(key, req, choices), nil
}

func unionTargets(a, b []string) []string {
	out := append([]string{}, a...)
	for _, target := range b {
		if !slices.Contains(out, target) {
			out = append(out, target)
		}
	}
	sort.Strings(out)
	return out
}

func (solver *graphSolver) describe(ctx context.Context, pkg LockedPackage) (ResolvedPackage, error) {
	if err := ctx.Err(); err != nil {
		return ResolvedPackage{}, err
	}
	cacheKey := pkg.SourceKey + "\x00" + pkg.TreeDigest
	if cached, ok := solver.descriptions[cacheKey]; ok {
		return cached, nil
	}
	if solver.graph == nil {
		return ResolvedPackage{}, ErrDependencyFactsUnavailable
	}
	selected, err := solver.graph.DescribePinned(ctx, pkg)
	if err != nil {
		return ResolvedPackage{}, err
	}
	if err := ctx.Err(); err != nil {
		return ResolvedPackage{}, err
	}
	solver.descriptions[cacheKey] = selected
	return selected, nil
}

func (solver *graphSolver) checkTargets(ctx context.Context, node graphNode, pkg LockedPackage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if solver.graph == nil {
		return ErrDependencyFactsUnavailable
	}
	if old, ok := solver.pins[pkg.SourceKey]; ok && !solver.update[pkg.SourceKey] && sameEligibilityPin(old, pkg) && slices.Equal(unionTargets(old.Targets, node.targets), old.Targets) {
		return nil
	}
	eligible, err := solver.graph.Eligible(ctx, pkg, node.targets)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !eligible {
		return conflictFor(pkg.SourceKey, node)
	}
	return nil
}

func sameEligibilityPin(previous, selected LockedPackage) bool {
	// Root renames and legacy absent version metadata do not change source eligibility.
	previous.Alias = selected.Alias
	if previous.Version == "" {
		previous.Version = selected.Version
	}
	return SamePin(previous, selected)
}
