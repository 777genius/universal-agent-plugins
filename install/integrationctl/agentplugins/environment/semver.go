package environment

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

var stableVersion = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)
var caretVersion = regexp.MustCompile(`^\^(0|[1-9]\d*)\.(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?$`)

func validateVersionSelector(selector Selector) error {
	valid := selector.Kind == SelectorVersion && stableVersion.MatchString(selector.Value)
	valid = valid || selector.Kind == SelectorRange && caretVersion.MatchString(selector.Value)
	if !valid {
		return fmt.Errorf("invalid stable version selector %q", selector.Value)
	}
	return nil
}

// CompareVersions compares stable package versions numerically.
func CompareVersions(a, b string) int { return semver.Compare("v"+a, "v"+b) }

// SelectorMatchesVersion applies the supported exact/caret stable syntax.
func SelectorMatchesVersion(selector Selector, version string) bool {
	if !stableVersion.MatchString(version) || validateVersionSelector(selector) != nil {
		return false
	}
	if selector.Kind == SelectorVersion {
		return version == selector.Value
	}
	parts := strings.Split(strings.TrimPrefix(selector.Value, "^"), ".")
	lower := strings.Join(parts, ".")
	if len(parts) == 2 {
		lower += ".0"
	}
	if CompareVersions(version, lower) < 0 {
		return false
	}
	got := strings.Split(version, ".")
	if parts[0] != "0" {
		return got[0] == parts[0]
	}
	if got[0] != "0" || got[1] != parts[1] {
		return false
	}
	if parts[1] == "0" && len(parts) == 3 {
		return got[2] == parts[2]
	}
	return true
}

func registrySelectorMatches(selector Selector, pkg LockedPackage) bool {
	if selector.Kind == SelectorRelease {
		return pkg.Source.Registry != nil && fmt.Sprint(pkg.Source.Registry.ReleaseSequence) == selector.Value
	}
	return SelectorMatchesVersion(selector, pkg.Version)
}
