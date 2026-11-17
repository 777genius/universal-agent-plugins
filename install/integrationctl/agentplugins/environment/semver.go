package environment

import (
	"fmt"
	"regexp"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	modsemver "golang.org/x/mod/semver"
)

// Stable releases retain their existing exact version contract. Graph downgrade
// checks use this strict expression even when a constraint explicitly includes
// a prerelease.
var stableVersion = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
var forbiddenVersionPrefix = regexp.MustCompile(`(^|[[:space:],|<>=~^])v[0-9]`)

// ParseRegistryVersionSelector is the single parser/classifier for registry
// version requirements. Exact versions remain exact; partial versions and
// comparators are ranges. Neither path silently coerces a leading "v".
func ParseRegistryVersionSelector(value string) (Selector, error) {
	selector := Selector{Kind: SelectorRange, Value: value}
	if isCanonicalExactVersion(value) {
		selector.Kind = SelectorVersion
	}
	return selector, selector.Validate()
}

func isCanonicalExactVersion(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "v") {
		return false
	}
	version, err := semver.StrictNewVersion(value)
	return err == nil && version.String() == value
}

func validateVersionSelector(selector Selector) error {
	if selector.Value == "" || strings.TrimSpace(selector.Value) != selector.Value ||
		forbiddenVersionPrefix.MatchString(selector.Value) {
		return fmt.Errorf("invalid version selector %q", selector.Value)
	}
	switch selector.Kind {
	case SelectorVersion:
		if isCanonicalExactVersion(selector.Value) {
			return nil
		}
	case SelectorRange:
		if _, err := semver.NewConstraint(selector.Value); err == nil {
			return nil
		}
	}
	return fmt.Errorf("invalid version selector %q", selector.Value)
}

// CompareVersions keeps stable numeric ordering and prerelease ordering
// delegated to an existing vetted semver implementation.
func CompareVersions(a, b string) int {
	return modsemver.Compare("v"+a, "v"+b)
}

// SelectorMatchesVersion applies the same parsed constraint grammar to registry
// candidate selection, pin reuse and lock replay. A candidate must be a strict,
// canonical SemVer version even if the library permits input coercion.
func SelectorMatchesVersion(selector Selector, version string) bool {
	if validateVersionSelector(selector) != nil || !isCanonicalExactVersion(version) {
		return false
	}
	if selector.Kind == SelectorVersion {
		return version == selector.Value
	}
	constraint, err := semver.NewConstraint(selector.Value)
	if err != nil {
		return false
	}
	candidate, err := semver.StrictNewVersion(version)
	return err == nil && constraint.Check(candidate)
}

func registrySelectorMatches(selector Selector, pkg LockedPackage) bool {
	if selector.Kind == SelectorRelease {
		return pkg.Source.Registry != nil && fmt.Sprint(pkg.Source.Registry.ReleaseSequence) == selector.Value
	}
	return SelectorMatchesVersion(selector, pkg.Version)
}
