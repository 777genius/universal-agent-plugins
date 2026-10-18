package environment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Candidate is metadata only; immutable source bytes are acquired lazily.
type Candidate struct {
	Version         string
	ReleaseSequence uint64
}
type ResolvedPackage struct {
	Package      LockedPackage
	Dependencies map[string]string
}

// ErrCandidateIneligible rejects one candidate on source policy discovered
// during acquisition. Other acquisition errors abort resolution.
var ErrCandidateIneligible = errors.New("dependency candidate is ineligible")

// GraphResolver supplies source facts while the coordinator owns graph policy.
type GraphResolver interface {
	Resolver
	Candidates(context.Context, Requirement, []string) ([]Candidate, error)
	AcquireCandidate(context.Context, Requirement, Candidate, []string) (ResolvedPackage, error)
	DescribePinned(context.Context, LockedPackage) (ResolvedPackage, error)
	Eligible(context.Context, LockedPackage, []string) (bool, error)
}

type graphDemand struct {
	requirement Requirement
	parent      string
}
type graphNode struct {
	requirement Requirement
	demands     []graphDemand
	targets     []string
}
type graphSolver struct {
	resolver     Resolver
	graph        GraphResolver
	requirements Requirements
	previous     Lock
	update       string
	roots        map[string]graphNode
	pins         map[string]LockedPackage
	descriptions map[string]ResolvedPackage
}

type graphConflict struct{ message string }

func (err graphConflict) Error() string { return err.message }

func newGraphSolver(resolver Resolver, requirements Requirements, previous Lock, update string) *graphSolver {
	solver := &graphSolver{resolver: resolver, requirements: requirements, previous: previous, update: update, roots: map[string]graphNode{}, pins: map[string]LockedPackage{}, descriptions: map[string]ResolvedPackage{}}
	solver.graph, _ = resolver.(GraphResolver)
	for _, pkg := range previous.Packages {
		solver.pins[pkg.SourceKey] = pkg
	}
	for _, alias := range sortedKeys(requirements.Dependencies) {
		req := requirements.Dependencies[alias]
		key, _ := req.SourceKey()
		solver.roots[key] = graphNode{requirement: req, demands: []graphDemand{{requirement: req}}, targets: targetsFor(requirements, alias)}
	}
	return solver
}

func (solver *graphSolver) resolve(ctx context.Context) (Lock, error) {
	choices, nodes, err := solver.search(ctx, map[string]ResolvedPackage{})
	if err != nil {
		return Lock{}, err
	}
	digest, _ := ManifestDigest(solver.requirements)
	out := Lock{LockVersion: LockVersion, ManifestDigest: digest, Packages: []LockedPackage{}}
	for key, node := range nodes {
		pkg := choices[key].Package
		pkg.Alias = node.requirement.Alias
		pkg.Targets = node.targets
		pkg.RequiredBy = nil
		requested := node.requirement
		if _, root := solver.roots[key]; !root && pkg.Source.Registry != nil {
			cloned := *requested.Registry
			cloned.Selector = Selector{Kind: SelectorRelease, Value: fmt.Sprint(pkg.Source.Registry.ReleaseSequence)}
			requested.Registry = &cloned
		}
		setRequested(&pkg, requested)
		for _, demand := range node.demands {
			if demand.parent != "" {
				pkg.RequiredBy = append(pkg.RequiredBy, RequiredBy{Alias: demand.parent, Selector: demand.requirement.Registry.Selector})
			}
		}
		sort.Slice(pkg.RequiredBy, func(i, j int) bool {
			a, b := pkg.RequiredBy[i], pkg.RequiredBy[j]
			return a.Alias+"\x00"+string(a.Selector.Kind)+a.Selector.Value < b.Alias+"\x00"+string(b.Selector.Kind)+b.Selector.Value
		})
		out.Packages = append(out.Packages, pkg)
	}
	sort.Slice(out.Packages, func(i, j int) bool { return out.Packages[i].Alias < out.Packages[j].Alias })
	return out, out.Validate(solver.requirements)
}

func setRequested(pkg *LockedPackage, req Requirement) {
	if pkg.Source.Registry != nil {
		cloned := *pkg.Source.Registry
		cloned.Requested = req.Registry.Selector
		pkg.Source.Registry = &cloned
	}
	if pkg.Source.Git != nil {
		cloned := *pkg.Source.Git
		cloned.Requested = req.Git.Selector
		pkg.Source.Git = &cloned
	}
}

func (solver *graphSolver) search(ctx context.Context, choices map[string]ResolvedPackage) (map[string]ResolvedPackage, map[string]graphNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	nodes, err := solver.expand(choices)
	if err != nil {
		return nil, nil, err
	}
	for _, key := range sortedKeys(nodes) {
		node := nodes[key]
		if selected, ok := choices[key]; ok {
			if !nodeAccepts(node, selected.Package) {
				return nil, nil, conflictFor(key, node)
			}
			if err := solver.checkTargets(ctx, node, selected.Package); err != nil {
				return nil, nil, err
			}
			continue
		}
		return solver.choose(ctx, key, node, choices)
	}
	if err := validateGraphAliases(nodes); err != nil {
		return nil, nil, err
	}
	return choices, nodes, nil
}

func (solver *graphSolver) choose(ctx context.Context, key string, node graphNode, choices map[string]ResolvedPackage) (map[string]ResolvedPackage, map[string]graphNode, error) {
	var last error
	if solver.update != "" && node.requirement.Alias != solver.update {
		if _, root := solver.roots[key]; root {
			if old, ok := solver.pins[key]; ok && !nodeAccepts(node, old) {
				return nil, nil, conflictFor(key, node)
			}
		}
	}
	if old, ok := solver.pins[key]; ok && node.requirement.Alias != solver.update && nodeAccepts(node, old) {
		// Newly acquired saved-add seeds have source identity before graph intent.
		// Supply their known targets when the cold manifest must be read again.
		if len(old.Targets) == 0 {
			old.Targets = append([]string{}, node.targets...)
		}
		selected, err := solver.describe(ctx, old)
		if err != nil {
			return nil, nil, err
		}
		found, nodes, err := solver.try(ctx, key, selected, choices)
		if err == nil {
			return found, nodes, nil
		}
		var conflict graphConflict
		if !errors.As(err, &conflict) {
			return nil, nil, err
		}
		last = err
		if solver.update != "" {
			if _, root := solver.roots[key]; root {
				return nil, nil, last
			}
		}
	}
	if solver.graph == nil || node.requirement.Git != nil || node.requirement.Registry.Selector.Kind == SelectorRelease {
		return solver.chooseExact(ctx, key, node, choices, last)
	}
	return solver.chooseCandidates(ctx, key, node, choices, last)
}

func (solver *graphSolver) chooseCandidates(ctx context.Context, key string, node graphNode, choices map[string]ResolvedPackage, last error) (map[string]ResolvedPackage, map[string]graphNode, error) {
	candidates, err := solver.graph.Candidates(ctx, node.requirement, node.targets)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		c := CompareVersions(candidates[i].Version, candidates[j].Version)
		return c < 0 || c == 0 && candidates[i].ReleaseSequence < candidates[j].ReleaseSequence
	})
	for _, candidate := range candidates {
		if !candidateAccepts(node, candidate) {
			continue
		}
		selected, err := solver.graph.AcquireCandidate(ctx, node.requirement, candidate, node.targets)
		if err != nil {
			if errors.Is(err, ErrCandidateIneligible) {
				continue
			}
			return nil, nil, err
		}
		found, nodes, err := solver.try(ctx, key, selected, choices)
		if err == nil {
			return found, nodes, nil
		}
		var conflict graphConflict
		if !errors.As(err, &conflict) {
			return nil, nil, err
		}
		last = err
	}
	if last != nil {
		return nil, nil, last
	}
	return nil, nil, conflictFor(key, node)
}

func (solver *graphSolver) chooseExact(ctx context.Context, key string, node graphNode, choices map[string]ResolvedPackage, last error) (map[string]ResolvedPackage, map[string]graphNode, error) {
	if last != nil {
		return nil, nil, last
	}
	if solver.resolver == nil {
		return nil, nil, errors.New("resolver is required")
	}
	old, ok := solver.pins[key]
	pkg, err := solver.resolver.Resolve(ctx, node.requirement, pinOrNil(old, ok), node.requirement.Alias == solver.update)
	if err != nil {
		return nil, nil, err
	}
	pkg.Targets = append([]string{}, node.targets...)
	selected := ResolvedPackage{Package: pkg}
	if solver.graph != nil {
		selected, err = solver.graph.DescribePinned(ctx, pkg)
		if err != nil {
			return nil, nil, err
		}
	}
	return solver.try(ctx, key, selected, choices)
}

func (solver *graphSolver) try(ctx context.Context, key string, selected ResolvedPackage, choices map[string]ResolvedPackage) (map[string]ResolvedPackage, map[string]graphNode, error) {
	cloned := make(map[string]ResolvedPackage, len(choices)+1)
	for key, value := range choices {
		cloned[key] = value
	}
	cloned[key] = selected
	return solver.search(ctx, cloned)
}

func nodeAccepts(node graphNode, pkg LockedPackage) bool {
	for _, demand := range node.demands {
		req := demand.requirement
		if req.Registry != nil {
			if !registrySelectorMatches(req.Registry.Selector, pkg) {
				return false
			}
		} else if validateLockedSelector(req, pkg) != nil {
			return false
		}
	}
	return true
}
func candidateAccepts(node graphNode, candidate Candidate) bool {
	for _, demand := range node.demands {
		if !SelectorMatchesVersion(demand.requirement.Registry.Selector, candidate.Version) {
			return false
		}
	}
	return true
}
func conflictFor(key string, node graphNode) error {
	parts := make([]string, 0, len(node.demands))
	for _, demand := range node.demands {
		name := demand.parent
		if name == "" {
			name = demand.requirement.Alias
		}
		var selector Selector
		if demand.requirement.Registry != nil {
			selector = demand.requirement.Registry.Selector
		} else {
			selector = demand.requirement.Git.Selector
		}
		parts = append(parts, name+" "+string(selector.Kind)+" "+selector.Value)
	}
	sort.Strings(parts)
	return graphConflict{message: fmt.Sprintf("dependency conflict for %s: %s", key, strings.Join(parts, ", "))}
}
