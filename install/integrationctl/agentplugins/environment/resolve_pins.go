package environment

import (
	"context"
	"fmt"
	"slices"
)

type pinnedClosure struct {
	choices      map[string]ResolvedPackage
	nodes        map[string]graphNode
	unresolved   []string
	queue        []string
	pending      map[string]bool
	declarations map[string][]Requirement
	propagated   map[string][]string
}

// tryPinnedContraction proves a closed graph of old immutable pins from current
// declarations. Historical traversal does not consume fresh-search budgets;
// inapplicability leaves only verified descriptions available to a fresh solve.
func (solver *graphSolver) tryPinnedContraction(ctx context.Context) (map[string]ResolvedPackage, map[string]graphNode, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, false, err
	}
	if len(solver.update) != 0 {
		return nil, nil, false, nil
	}
	work, err := solver.contractionRoots(ctx)
	if err != nil || work == nil {
		return nil, nil, false, err
	}
	for len(work.queue) > 0 || len(work.unresolved) > 0 {
		key := work.next()
		complete, err := solver.expandContractionPin(ctx, work, key)
		if err != nil || !complete {
			return nil, nil, false, err
		}
	}
	if err := validateGraphAliases(work.nodes); err != nil {
		return nil, nil, false, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, false, err
	}
	return work.choices, work.nodes, true, nil
}

func (work *pinnedClosure) next() string {
	// Finish every target/demand change among described pins before choosing
	// the next lexical source, matching fresh search's description frontier.
	queue := &work.queue
	if len(*queue) == 0 {
		queue = &work.unresolved
	}
	key := (*queue)[0]
	(*queue)[0] = ""
	*queue = (*queue)[1:]
	return key
}

func (solver *graphSolver) contractionRoots(ctx context.Context) (*pinnedClosure, error) {
	queue := sortedKeys(solver.roots)
	// Check every root before reading any package. Clearly fresh over-limit
	// requests must still fail their ordinary admission without acquisition.
	for _, key := range queue {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node := solver.roots[key]
		pin, exists := solver.pins[key]
		if !exists || requirementSelector(node.requirement) != pin.Requirement().RequestedSelector() ||
			!nodeAccepts(node, pin) || !pinnedTargetsExempt(pin, node.targets) {
			return nil, nil
		}
	}
	work := &pinnedClosure{
		choices: make(map[string]ResolvedPackage, len(queue)), nodes: make(map[string]graphNode, len(queue)),
		unresolved: queue, pending: make(map[string]bool, len(queue)),
		declarations: map[string][]Requirement{}, propagated: map[string][]string{},
	}
	for _, key := range queue {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node := solver.roots[key]
		node.targets = slices.Clone(node.targets)
		node.demands = slices.Clone(node.demands)
		work.nodes[key] = node
	}
	return work, nil
}

func (solver *graphSolver) expandContractionPin(ctx context.Context, work *pinnedClosure, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	work.pending[key] = false
	node := work.nodes[key]
	pin := solver.pins[key]
	if !pinnedTargetsExempt(pin, node.targets) {
		return false, nil
	}
	selected, described := work.choices[key]
	if !described {
		// Fresh resolution skips an incompatible old pin without reading its
		// bytes. Unknown old versions cannot satisfy new SemVer demands either.
		if !nodeAccepts(node, pin) {
			return false, nil
		}
		var err error
		selected, work.declarations[key], err = solver.describeContractionPin(ctx, pin, work.choices)
		if err != nil {
			return false, err
		}
		work.choices[key] = selected
	}
	if !nodeAccepts(node, selected.Package) {
		return false, nil
	}
	if targets, done := work.propagated[key]; done && slices.Equal(targets, node.targets) {
		return true, nil
	}
	work.propagated[key] = slices.Clone(node.targets)
	return solver.mergeContractionDependencies(ctx, work, node, work.declarations[key])
}

func (solver *graphSolver) mergeContractionDependencies(ctx context.Context, work *pinnedClosure, parent graphNode, declarations []Requirement) (bool, error) {
	for _, req := range declarations {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		key, _ := acquisitionSourceKey(req)
		pin, exists := solver.pins[key]
		if !exists || !pinnedTargetsExempt(pin, unionTargets(work.nodes[key].targets, parent.targets)) {
			return false, nil
		}
		_, reached := work.nodes[key]
		if mergeGraphDemand(work.nodes, parent, req) {
			work.enqueue(key, reached)
		}
	}
	return true, nil
}

func (work *pinnedClosure) enqueue(key string, reached bool) {
	if _, described := work.choices[key]; described {
		if !work.pending[key] {
			work.queue = append(work.queue, key)
			work.pending[key] = true
		}
	} else if !reached {
		index, _ := slices.BinarySearch(work.unresolved, key)
		work.unresolved = slices.Insert(work.unresolved, index, key)
	}
}

func pinnedTargetsExempt(pin LockedPackage, targets []string) bool {
	// Preserve checkTargets' existing exemption, including its treatment of
	// valid but unsorted historical target lists. Those use normal eligibility.
	return slices.Equal(unionTargets(pin.Targets, targets), pin.Targets)
}

func (solver *graphSolver) describeContractionPin(ctx context.Context, pin LockedPackage, choices map[string]ResolvedPackage) (ResolvedPackage, []Requirement, error) {
	if err := ctx.Err(); err != nil {
		return ResolvedPackage{}, nil, err
	}
	cacheKey := pin.SourceKey + "\x00" + pin.TreeDigest
	selected, cached := solver.descriptions[cacheKey]
	if !cached {
		if solver.graph == nil {
			return ResolvedPackage{}, nil, ErrDependencyFactsUnavailable
		}
		var err error
		selected, err = solver.graph.DescribePinned(ctx, pin)
		if err != nil {
			return ResolvedPackage{}, nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return ResolvedPackage{}, nil, err
	}
	if pin.Version == "" {
		pin.Version = selected.Package.Version
	}
	if !SamePin(pin, selected.Package) {
		return ResolvedPackage{}, nil, fmt.Errorf("pinned package %q changed immutable identity", pin.Alias)
	}
	declarations := make([]Requirement, 0, len(selected.Dependencies))
	for _, declaration := range sortedKeys(selected.Dependencies) {
		if err := ctx.Err(); err != nil {
			return ResolvedPackage{}, nil, err
		}
		req, err := solver.dependencyRequirement(declaration, selected.Dependencies[declaration], choices)
		if err != nil {
			return ResolvedPackage{}, nil, err
		}
		declarations = append(declarations, req)
	}
	if err := ctx.Err(); err != nil {
		return ResolvedPackage{}, nil, err
	}
	// Preserve the recorded source selectors until finalization rebuilds intent.
	// Only the existing absent-Version hydration changes the immutable pin.
	selected.Package = pin
	solver.descriptions[cacheKey] = selected
	return selected, declarations, nil
}
