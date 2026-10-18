package environment

import (
	"context"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
)

func (solver *graphSolver) expand(choices map[string]ResolvedPackage) (map[string]graphNode, error) {
	nodes := map[string]graphNode{}
	for key, node := range solver.roots {
		node.targets = append([]string{}, node.targets...)
		node.demands = append([]graphDemand{}, node.demands...)
		nodes[key] = node
	}
	changed := true
	for changed {
		changed = false
		for _, key := range sortedKeys(nodes) {
			node := nodes[key]
			selected, ok := choices[key]
			if !ok {
				continue
			}
			for _, distribution := range sortedKeys(selected.Dependencies) {
				req, err := solver.dependencyRequirement(distribution, selected.Dependencies[distribution], choices)
				if err != nil {
					return nil, err
				}
				if mergeGraphDemand(nodes, node, req) {
					changed = true
				}
			}
		}
	}
	return nodes, nil
}

func mergeGraphDemand(nodes map[string]graphNode, parent graphNode, req Requirement) bool {
	key, _ := req.SourceKey()
	child, exists := nodes[key]
	changed := !exists
	if !exists {
		child = graphNode{requirement: req}
	}
	demand := graphDemand{requirement: req, parent: parent.requirement.Alias}
	if !slices.ContainsFunc(child.demands, func(item graphDemand) bool {
		return item.parent == demand.parent && item.requirement.Registry != nil && item.requirement.Registry.Selector == req.Registry.Selector
	}) {
		child.demands = append(child.demands, demand)
		changed = true
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

func (solver *graphSolver) dependencyRequirement(distribution, value string, choices map[string]ResolvedPackage) (Requirement, error) {
	if !strings.Contains(distribution, "/") {
		return Requirement{}, fmt.Errorf("dependency distribution %q must be canonical owner/name", distribution)
	}
	selector := Selector{Kind: SelectorVersion, Value: value}
	if strings.HasPrefix(value, "^") {
		selector.Kind = SelectorRange
	}
	key := "directory:" + strings.ToLower(distribution)
	alias := strings.ReplaceAll(strings.ToLower(path.Base(distribution)), ".", "-")
	if root, ok := solver.roots[key]; ok {
		alias = root.requirement.Alias
	} else if pin, ok := solver.pins[key]; ok {
		alias = pin.Alias
	} else if chosen, ok := choices[key]; ok && chosen.Package.Alias != "" {
		alias = chosen.Package.Alias
	}
	req := Requirement{Alias: alias, Registry: &RegistrySource{Distribution: distribution, Selector: selector}}
	return req, req.Validate()
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
	cacheKey := pkg.SourceKey + "\x00" + pkg.TreeDigest
	if cached, ok := solver.descriptions[cacheKey]; ok {
		return cached, nil
	}
	selected := ResolvedPackage{Package: pkg, Dependencies: map[string]string{}}
	if solver.graph != nil {
		var err error
		selected, err = solver.graph.DescribePinned(ctx, pkg)
		if err != nil {
			return ResolvedPackage{}, err
		}
	} else {
		for _, child := range solver.previous.Packages {
			for _, edge := range child.RequiredBy {
				if edge.Alias == pkg.Alias && child.Source.Registry != nil {
					selected.Dependencies[child.Source.Registry.Distribution] = edge.Selector.Value
				}
			}
		}
	}
	solver.descriptions[cacheKey] = selected
	return selected, nil
}

func (solver *graphSolver) checkTargets(ctx context.Context, node graphNode, pkg LockedPackage) error {
	if solver.graph == nil {
		return nil
	}
	if old, ok := solver.pins[pkg.SourceKey]; ok && old.TreeDigest == pkg.TreeDigest && slices.Equal(unionTargets(old.Targets, node.targets), old.Targets) {
		return nil
	}
	eligible, err := solver.graph.Eligible(ctx, pkg, node.targets)
	if err != nil {
		return err
	}
	if !eligible {
		return conflictFor(pkg.SourceKey, node)
	}
	return nil
}
