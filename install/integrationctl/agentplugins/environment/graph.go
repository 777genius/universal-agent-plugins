package environment

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
)

// ClosureAliases returns the selected nodes and every dependency they reach.
// Including an absent alias lets callers select removals from an older lock.
func (lock Lock) ClosureAliases(aliases []string) []string {
	selected := map[string]bool{}
	for _, alias := range aliases {
		selected[alias] = true
	}
	for changed := true; changed; {
		changed = false
		for _, pkg := range lock.Packages {
			for _, edge := range pkg.RequiredBy {
				if selected[edge.Alias] && !selected[pkg.Alias] {
					selected[pkg.Alias] = true
					changed = true
				}
			}
		}
	}
	return sortedKeys(selected)
}

// SamePin compares immutable acquisition identity independently of graph intent.
func SamePin(a, b LockedPackage) bool {
	a.Targets, b.Targets = nil, nil
	a.RequiredBy, b.RequiredBy = nil, nil
	a.Source = pinSourceIdentity(a.Source)
	b.Source = pinSourceIdentity(b.Source)
	return reflect.DeepEqual(a, b)
}

func pinSourceIdentity(source LockedSource) LockedSource {
	if source.Git != nil {
		cloned := *source.Git
		cloned.Requested = Selector{}
		source.Git = &cloned
	}
	if source.Registry != nil {
		cloned := *source.Registry
		cloned.Requested = Selector{}
		source.Registry = &cloned
	}
	if source.Archive != nil {
		cloned := *source.Archive
		cloned.RequestedSHA256 = ""
		source.Archive = &cloned
	}
	return source
}

func validateGraph(lock Lock, requirements Requirements) error {
	byAlias := map[string]LockedPackage{}
	sources := map[string]string{}
	resolvedSkills := map[string]string{}
	for _, pkg := range lock.Packages {
		if _, exists := byAlias[pkg.Alias]; exists {
			return fmt.Errorf("duplicate locked alias %q", pkg.Alias)
		}
		if prior, exists := sources[pkg.SourceKey]; exists {
			return fmt.Errorf("locked aliases %q and %q use the same source", prior, pkg.Alias)
		}
		if err := validatePin(pkg); err != nil {
			return fmt.Errorf("%s: %w", pkg.Alias, err)
		}
		if source := pkg.Source.Git; source != nil && source.Kind == SkillKind {
			identity := strings.ToLower(source.Repository) + "\x00" + source.Commit + "\x00" + source.ResolvedSkillPath
			if previous := resolvedSkills[identity]; previous != "" {
				return fmt.Errorf("aliases %s and %s select the same Skill", previous, pkg.Alias)
			}
			resolvedSkills[identity] = pkg.Alias
		}
		byAlias[pkg.Alias], sources[pkg.SourceKey] = pkg, pkg.Alias
	}
	for alias, requirement := range requirements.Dependencies {
		pkg, exists := lock.PackageForRequirement(requirement)
		if !exists {
			return fmt.Errorf("lock is missing alias %q", alias)
		}
		if err := validateLockedSelector(requirement, pkg); err != nil {
			return fmt.Errorf("%s: %w", alias, err)
		}
	}
	if err := validateEdges(byAlias); err != nil {
		return err
	}
	targets := inheritedTargets(requirements, lock)
	for _, pkg := range lock.Packages {
		expected := sortedKeys(targets[pkg.Alias])
		if len(expected) == 0 {
			return fmt.Errorf("locked alias %q is unreachable from current roots", pkg.Alias)
		}
		actual := append([]string(nil), pkg.Targets...)
		sort.Strings(actual)
		if !slices.Equal(actual, expected) {
			return fmt.Errorf("%s: locked targets changed", pkg.Alias)
		}
	}
	return nil
}

func validatePin(pkg LockedPackage) error {
	if !aliasPattern.MatchString(pkg.Alias) {
		return fmt.Errorf("invalid locked alias %q", pkg.Alias)
	}
	req := pkg.Requirement()
	key, err := req.SourceKey()
	if err != nil {
		return err
	}
	if key != pkg.SourceKey {
		return errors.New("locked source key disagrees with coordinates")
	}
	if err := validateLockedSelector(req, pkg); err != nil {
		return err
	}
	if pkg.DigestAlgorithm != TreeDigestAlgorithm || !validGraphDigest(pkg.TreeDigest) || !validGraphDigest(pkg.ManifestDigest) {
		return errors.New("invalid locked digest")
	}
	return nil
}

func validGraphDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[7:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func validateEdges(packages map[string]LockedPackage) error {
	for _, pkg := range packages {
		seen := map[RequiredBy]bool{}
		for _, edge := range pkg.RequiredBy {
			if _, exists := packages[edge.Alias]; !exists {
				return fmt.Errorf("%s: unknown required_by consumer %q", pkg.Alias, edge.Alias)
			}
			if seen[edge] {
				return fmt.Errorf("%s: duplicate required_by edge from %q", pkg.Alias, edge.Alias)
			}
			seen[edge] = true
			if err := edge.Selector.Validate(); err != nil {
				return fmt.Errorf("%s: invalid required_by selector: %w", pkg.Alias, err)
			}
			if err := validateRequiredBySource(pkg, edge); err != nil {
				return err
			}

		}
	}
	return nil
}

func validateRequiredBySource(pkg LockedPackage, edge RequiredBy) error {
	if pkg.Source.Git != nil {
		if edge.Selector != pkg.Source.Git.Requested {
			return fmt.Errorf("%s: Git selector differs from consumer %s", pkg.Alias, edge.Alias)
		}
	} else if pkg.Source.Archive != nil {
		if edge.Selector != (Selector{Kind: SelectorArchive, Value: pkg.Source.Archive.RequestedSHA256}) {
			return fmt.Errorf("%s: archive selector differs from consumer %s", pkg.Alias, edge.Alias)
		}
	} else if pkg.Source.Web != nil {
		if edge.Selector != (Selector{Kind: SelectorDefault}) {
			return fmt.Errorf("%s: website selector differs from consumer %s", pkg.Alias, edge.Alias)
		}
	} else if edge.Selector.Kind != SelectorRelease && edge.Selector.Kind != SelectorVersion && edge.Selector.Kind != SelectorRange {
		return fmt.Errorf("%s: invalid registry dependency selector", pkg.Alias)
	} else if !registrySelectorMatches(edge.Selector, pkg) {
		return fmt.Errorf("%s: version %q does not satisfy consumer %s selector %s", pkg.Alias, pkg.Version, edge.Alias, edge.Selector.Value)
	}
	return nil
}

func inheritedTargets(requirements Requirements, lock Lock) map[string]map[string]bool {
	targets := map[string]map[string]bool{}
	for _, pkg := range lock.Packages {
		targets[pkg.Alias] = map[string]bool{}
	}
	for client, aliases := range requirements.Targets {
		for _, alias := range aliases {
			req, ok := requirements.Dependencies[alias]
			if !ok {
				continue
			}
			if pkg, found := lock.PackageForRequirement(req); found {
				targets[pkg.Alias][client] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, pkg := range lock.Packages {
			for _, edge := range pkg.RequiredBy {
				for target := range targets[edge.Alias] {
					if !targets[pkg.Alias][target] {
						targets[pkg.Alias][target] = true
						changed = true
					}
				}
			}
		}
	}
	return targets
}

func retainedReason(pkg LockedPackage) string {
	consumers := map[string]bool{}
	for _, edge := range pkg.RequiredBy {
		consumers[edge.Alias] = true
	}
	return "required by " + strings.Join(sortedKeys(consumers), ", ")
}
