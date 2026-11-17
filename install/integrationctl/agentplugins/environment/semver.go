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
var (
	stableVersion = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)package environment

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
)
	forbiddenVersionPrefix = regexp.MustCompile(`(^|[[:space:],|<>=~^])v\d`)
	// Masterminds accepts coerced numeric operands such as 1.02.3; a package
	// selector must not silently resolve such a value to a different version.
	nonCanonicalNumericPart = regexp.MustCompile(`(^|[.[:space:]<>=~^|,])0\d+($|[.[:space:]|,+-])`)
)

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
		forbiddenVersionPrefix.MatchString(selector.Value) ||
		nonCanonicalNumericPart.MatchString(selector.Value) {
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
	// SemVer build metadata does not participate in constraint equality.
	// Exact bytes are still pinned by signed release identity and lock digests.
	constraint, err := semver.NewConstraint(selector.Value)
	if err != nil {
		return false
	}
	candidate, err := semver.StrictNewVersion(version)
	if err != nil || !constraint.Check(candidate) {
		return false
	}
	// Masterminds permits *any* prerelease in a comparator group containing a
	// prerelease. npm semver permits only prereleases of the explicitly named
	// major.minor.patch tuple. The extra guard prevents unrequested upgrades.
	if candidate.Prerelease() != "" {
		return prereleaseMatchesNamedTuple(selector.Value, candidate)
	}
	return true
}

// Keep this compatibility rule narrow: the library still parses and evaluates
// every constraint. Only the npm prerelease eligibility rule is enforced here.
func prereleaseMatchesNamedTuple(expression string, candidate *semver.Version) bool {
	needle := fmt.Sprintf("%d.%d.%d-", candidate.Major(), candidate.Minor(), candidate.Patch())
	for _, group := range strings.Split(expression, "||") {
		for at := 0; at < len(group); {
			i := strings.Index(group[at:], needle)
			if i < 0 {
				break
			}
			i += at
			if i == 0 || (group[i-1] != '.' && (group[i-1] < '0' || group[i-1] > '9')) {
				selected, err := semver.NewConstraint(group)
				if err == nil && selected.Check(candidate) {
					return true
				}
			}
			at = i + len(needle)
		}
	}
	return false
}

func registrySelectorMatches(selector Selector, pkg LockedPackage) bool {
	if selector.Kind == SelectorRelease {
		return pkg.Source.Registry != nil && fmt.Sprint(pkg.Source.Registry.ReleaseSequence) == selector.Value
	}
	return SelectorMatchesVersion(selector, pkg.Version)
}
