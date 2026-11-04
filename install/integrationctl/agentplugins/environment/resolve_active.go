package environment

import (
	"context"
	"slices"
)

type graphDemandIdentity struct {
	parent   string
	selector Selector
}

type graphUndo struct {
	key     string
	node    graphNode
	existed bool
}

// Active graph values own their mutable indexes. Demand slices are append-only:
// saved prefixes and target slices remain immutable until their undo is retired.
// The request's search/work budget deliberately lives outside this branch state.
type activeGraph struct {
	nodes    map[string]graphNode
	keys     []string
	prepared map[string][]preparedDependency
	queue    []string
	pending  map[string]bool
	undo     []graphUndo
	demands  int
}

func (solver *graphSolver) newActiveGraph(ctx context.Context, choices map[string]ResolvedPackage) (*activeGraph, error) {
	if err := solver.budget.limits.admitGraph(len(solver.roots), 0); err != nil {
		return nil, err
	}
	work := &activeGraph{nodes: make(map[string]graphNode, len(solver.roots)), keys: sortedKeys(solver.roots), prepared: map[string][]preparedDependency{}, pending: map[string]bool{}}
	for _, key := range work.keys {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return nil, err
		}
		node := solver.roots[key]
		node.requirement = solver.nodeRequirement(key, node.requirement, choices)
		node.demands = append([]graphDemand(nil), node.demands...)
		node.targets = append([]string(nil), node.targets...)
		node.demandIndex = map[graphDemandIdentity]struct{}{{selector: requirementSelector(node.demands[0].requirement)}: {}}
		work.nodes[key] = node
		if _, selected := choices[key]; selected {
			work.enqueue(key)
		}
	}
	return work, nil
}

func (work *activeGraph) enqueue(key string) {
	if !work.pending[key] {
		work.queue = append(work.queue, key)
		work.pending[key] = true
	}
}

func (work *activeGraph) save(key string, node graphNode, existed bool) {
	work.undo = append(work.undo, graphUndo{key: key, node: node, existed: existed})
}

func (work *activeGraph) rollback(checkpoint, demands int) {
	for i := len(work.undo) - 1; i >= checkpoint; i-- {
		undo := work.undo[i]
		if !undo.existed {
			delete(work.nodes, undo.key)
			index, _ := slices.BinarySearch(work.keys, undo.key)
			work.keys = slices.Delete(work.keys, index, index+1)
			continue
		}
		current := work.nodes[undo.key]
		for _, demand := range current.demands[len(undo.node.demands):] {
			delete(current.demandIndex, graphDemandIdentity{parent: demand.parent, selector: requirementSelector(demand.requirement)})
		}
		work.nodes[undo.key] = undo.node
	}
	clear(work.undo[checkpoint:])
	work.undo = work.undo[:checkpoint]
	work.demands = demands
	work.queue = nil
	clear(work.pending)
}

func (work *activeGraph) accepts(key string, pkg LockedPackage) bool {
	node := work.nodes[key]
	if !node.acceptanceKnown {
		node.accepted = pkg.SourceKey == key && nodeDemandsAccept(node, pkg)
		node.acceptanceKnown = true
		work.nodes[key] = node
	}
	return node.accepted
}

func (solver *graphSolver) propagate(ctx context.Context, choices map[string]ResolvedPackage, work *activeGraph) error {
	for len(work.queue) > 0 {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return err
		}
		key := work.queue[0]
		work.queue[0] = ""
		work.queue = work.queue[1:]
		work.pending[key] = false
		selected, ok := choices[key]
		if !ok {
			continue
		}
		if _, prepared := work.prepared[key]; !prepared {
			facts, err := solver.prepareDependencies(ctx, selected.Dependencies)
			if err != nil {
				return err
			}
			work.prepared[key] = facts
			node := work.nodes[key]
			work.save(key, node, true)
			node.requirement = solver.nodeRequirement(key, node.requirement, choices)
			node.acceptanceKnown = false
			work.nodes[key] = node
		}
		if err := solver.propagateDependencies(ctx, choices, work, key); err != nil {
			return err
		}
	}
	return nil
}

func (solver *graphSolver) propagateDependencies(ctx context.Context, choices map[string]ResolvedPackage, work *activeGraph, key string) error {
	parent := work.nodes[key]
	for _, fact := range work.prepared[key] {
		if err := solver.budget.consumeWork(ctx, 1); err != nil {
			return err
		}
		req := solver.nodeRequirement(fact.key, fact.requirement, choices)
		if err := work.merge(parent, fact.key, req, solver.budget.limits); err != nil {
			return err
		}
	}
	return nil
}

func (work *activeGraph) merge(parent graphNode, key string, req Requirement, limits resolutionLimits) error {
	child, exists := work.nodes[key]
	demand := graphDemandIdentity{parent: parent.requirement.Alias, selector: requirementSelector(req)}
	_, duplicate := child.demandIndex[demand]
	targets := unionTargets(child.targets, parent.targets)
	targetGrowth := !slices.Equal(targets, child.targets)
	if duplicate && !targetGrowth {
		return nil
	}
	work.save(key, child, exists)
	if !exists {
		child = graphNode{requirement: req, demandIndex: map[graphDemandIdentity]struct{}{}}
		index, _ := slices.BinarySearch(work.keys, key)
		work.keys = slices.Insert(work.keys, index, key)
	}
	if !duplicate {
		child.demands = append(child.demands, graphDemand{requirement: req, parent: demand.parent})
		child.demandIndex[demand] = struct{}{}
		child.acceptanceKnown = false
		work.demands++
	}
	if exactReleaseRequirement(req) && !exactReleaseRequirement(child.requirement) {
		alias := child.requirement.Alias
		child.requirement = req
		child.requirement.Alias = alias
	}
	child.targets = targets
	work.nodes[key] = child
	if err := limits.admitGraph(len(work.nodes), work.demands); err != nil {
		return err
	}
	if targetGrowth {
		work.enqueue(key)
	}
	return nil
}
