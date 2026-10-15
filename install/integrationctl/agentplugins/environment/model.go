// Package environment defines the portable dependency intent, deterministic
// lock contract and sync coordination policy for one user environment.
// File formats, Git and installer adapters remain at the composition boundary.
package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	ManifestSchemaVersion  = 1
	LockVersion            = 1
	EnvironmentUserDefault = "user-default"
	TreeDigestAlgorithm    = "agentplugins-tree-sha256-v1"
)

var (
	aliasPattern        = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	commitPattern       = regexp.MustCompile(`^[0-9a-f]{40}$`)
	repositoryPattern   = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38}[A-Za-z0-9])?/[A-Za-z0-9](?:[A-Za-z0-9._-]{0,98}[A-Za-z0-9])?$`)
	distributionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(?:/[A-Za-z0-9][A-Za-z0-9._-]*)?$`)
)

type SelectorKind string

const (
	SelectorCommit  SelectorKind = "commit"
	SelectorRef     SelectorKind = "ref"
	SelectorDefault SelectorKind = "default"
	SelectorRelease SelectorKind = "release"
)

type Selector struct {
	Kind  SelectorKind `json:"kind"`
	Value string       `json:"value,omitempty"`
}

type GitSource struct {
	Repository string   `json:"repository"`
	Path       string   `json:"path,omitempty"`
	Selector   Selector `json:"selector"`
}

type RegistrySource struct {
	Distribution string   `json:"distribution"`
	Selector     Selector `json:"selector"`
}

type Requirement struct {
	Alias    string          `json:"alias"`
	Git      *GitSource      `json:"git,omitempty"`
	Registry *RegistrySource `json:"registry,omitempty"`
}

type Requirements struct {
	SchemaVersion int                    `json:"schema_version"`
	Dependencies  map[string]Requirement `json:"dependencies"`
	Targets       map[string][]string    `json:"targets"`
}

type LockedSource struct {
	Git      *LockedGitSource      `json:"git,omitempty"`
	Registry *LockedRegistrySource `json:"registry,omitempty"`
}

type LockedGitSource struct {
	Repository    string   `json:"repository"`
	Path          string   `json:"path,omitempty"`
	Requested     Selector `json:"requested"`
	Commit        string   `json:"commit"`
	ResolvedRef   string   `json:"resolved_ref,omitempty"`
	DefaultBranch string   `json:"default_branch,omitempty"`
}

type LockedRegistrySource struct {
	Distribution    string   `json:"distribution"`
	ProductID       string   `json:"product_id"`
	DistributionID  string   `json:"distribution_id"`
	ReleaseSequence uint64   `json:"release_sequence"`
	Requested       Selector `json:"requested"`
	Repository      string   `json:"repository"`
	Path            string   `json:"path,omitempty"`
	Commit          string   `json:"commit"`
}

type LockedPackage struct {
	Alias           string       `json:"alias"`
	Source          LockedSource `json:"source"`
	SourceKey       string       `json:"source_key"`
	DigestAlgorithm string       `json:"digest_algorithm"`
	TreeDigest      string       `json:"tree_digest"`
	ManifestDigest  string       `json:"manifest_digest"`
	ExecutableFiles []string     `json:"executable_files,omitempty"`
	Targets         []string     `json:"targets"`
}

type Lock struct {
	LockVersion    int             `json:"lock_version"`
	ManifestDigest string          `json:"manifest_digest"`
	Packages       []LockedPackage `json:"packages"`
}

type BindingObservation struct {
	Alias, ClientID, InstallationID, BindingID                 string
	SourceKey, PackageDigest, ProfileAuthorityDigest, Revision string
	Managed                                                    bool
	Receipt                                                    *domain.EnvironmentReceipt
	Drift, Pending                                             bool
}

type DesiredAction string

const (
	ActionUnchanged DesiredAction = "unchanged"
	ActionInstall   DesiredAction = "install"
	ActionUpdate    DesiredAction = "update"
	ActionRemove    DesiredAction = "remove"
)

type PreparedBinding struct {
	Alias, ClientID, InstallationID, BindingID string
	Action                                     DesiredAction
	Package                                    *LockedPackage
	Reason                                     string
	Authority                                  *domain.EnvironmentAuthority
}

type PreparedSync struct {
	EnvironmentID  string
	ManifestDigest string
	Bindings       []PreparedBinding
}

func (selector Selector) Validate() error {
	switch selector.Kind {
	case SelectorCommit:
		if !commitPattern.MatchString(selector.Value) {
			return errors.New("commit selector requires one full lowercase 40-character SHA")
		}
	case SelectorRef:
		value := strings.TrimSpace(selector.Value)
		if value == "" || value != selector.Value || strings.ContainsAny(value, " \t\r\n~^:?*[\\") || strings.HasPrefix(value, "-") || path.Clean(value) != value {
			return errors.New("ref selector is not a safe exact Git ref")
		}
	case SelectorDefault:
		if selector.Value != "" {
			return errors.New("default selector must have an empty value")
		}
	case SelectorRelease:
		if selector.Value == "" || strings.TrimSpace(selector.Value) != selector.Value {
			return errors.New("release selector must be a canonical nonzero sequence")
		}
		var sequence uint64
		if _, err := fmt.Sscanf(selector.Value, "%d", &sequence); err != nil || sequence == 0 || fmt.Sprint(sequence) != selector.Value {
			return errors.New("release selector must be a canonical nonzero sequence")
		}
	default:
		return fmt.Errorf("unsupported selector kind %q", selector.Kind)
	}
	return nil
}

func validateSubpath(value string) error {
	if value == "" {
		return nil
	}
	if strings.Contains(value, `\`) || strings.HasPrefix(value, "/") || path.Clean(value) != value || strings.HasPrefix(value, "../") {
		return errors.New("package path must be a normalized repository-relative directory")
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." || segment == "" {
			return errors.New("package path contains an unsafe segment")
		}
	}
	return nil
}

func (requirement Requirement) Validate() error {
	if !aliasPattern.MatchString(requirement.Alias) {
		return fmt.Errorf("alias %q must match %s", requirement.Alias, aliasPattern)
	}
	if (requirement.Git == nil) == (requirement.Registry == nil) {
		return errors.New("dependency must contain exactly one git or registry source")
	}
	if requirement.Git != nil {
		if !repositoryPattern.MatchString(requirement.Git.Repository) {
			return errors.New("git repository must be owner/repo")
		}
		if err := validateSubpath(requirement.Git.Path); err != nil {
			return err
		}
		if requirement.Git.Selector.Kind != SelectorCommit && requirement.Git.Selector.Kind != SelectorRef && requirement.Git.Selector.Kind != SelectorDefault {
			return errors.New("Git source requires commit, ref or default selector")
		}
		return requirement.Git.Selector.Validate()
	}
	if !distributionPattern.MatchString(requirement.Registry.Distribution) {
		return errors.New("registry distribution must be canonical owner/name")
	}
	if requirement.Registry.Selector.Kind != SelectorRelease {
		return errors.New("registry requires exact release selector")
	}
	return requirement.Registry.Selector.Validate()
}

func (requirements Requirements) Validate() error {
	if requirements.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("unsupported manifest schema_version %d", requirements.SchemaVersion)
	}
	sources := map[string]string{}
	for alias, requirement := range requirements.Dependencies {
		if alias != requirement.Alias {
			return fmt.Errorf("dependency key %q does not match alias %q", alias, requirement.Alias)
		}
		if err := requirement.Validate(); err != nil {
			return fmt.Errorf("%s: %w", alias, err)
		}
		key, err := requirement.SourceKey()
		if err != nil {
			return err
		}
		if previous, exists := sources[key]; exists {
			return fmt.Errorf("aliases %q and %q use the same source %q", previous, alias, key)
		}
		sources[key] = alias
	}
	for target, aliases := range requirements.Targets {
		if strings.TrimSpace(target) == "" {
			return errors.New("target ID must not be empty")
		}
		seen := map[string]bool{}
		for _, alias := range aliases {
			if _, exists := requirements.Dependencies[alias]; !exists {
				return fmt.Errorf("target %q references unknown alias %q", target, alias)
			}
			if seen[alias] {
				return fmt.Errorf("target %q repeats alias %q", target, alias)
			}
			seen[alias] = true
		}
	}
	for alias := range requirements.Dependencies {
		if !requirements.assigned(alias) {
			return fmt.Errorf("alias %q has no target assignment", alias)
		}
	}
	return nil
}

func (requirements Requirements) assigned(alias string) bool {
	for _, aliases := range requirements.Targets {
		for _, item := range aliases {
			if item == alias {
				return true
			}
		}
	}
	return false
}

func (requirement Requirement) SourceKey() (string, error) {
	if err := requirement.Validate(); err != nil {
		return "", err
	}
	if requirement.Git != nil {
		return "github:" + strings.ToLower(requirement.Git.Repository) + ":" + requirement.Git.Path, nil
	}
	return "directory:" + strings.ToLower(requirement.Registry.Distribution), nil
}

type manifestIntent struct {
	SchemaVersion int                  `json:"schema_version"`
	Dependencies  []manifestDependency `json:"dependencies"`
	Targets       []manifestTarget     `json:"targets"`
}

type manifestDependency struct {
	Alias     string   `json:"alias"`
	SourceKey string   `json:"source_key"`
	Path      string   `json:"path,omitempty"`
	Selector  Selector `json:"selector"`
}

type manifestTarget struct {
	ID      string   `json:"id"`
	Aliases []string `json:"aliases"`
}

// ManifestDigest is deterministic over normalized intent only. Moving refs,
// resolved commits, default branch names and timing are excluded unless the
// user explicitly selected the commit or release.
func ManifestDigest(requirements Requirements) (string, error) {
	if err := requirements.Validate(); err != nil {
		return "", err
	}
	intent := manifestIntent{SchemaVersion: ManifestSchemaVersion}
	for _, alias := range sortedKeys(requirements.Dependencies) {
		requirement := requirements.Dependencies[alias]
		key, _ := requirement.SourceKey()
		item := manifestDependency{Alias: alias, SourceKey: key}
		if requirement.Git != nil {
			item.Path = requirement.Git.Path
			item.Selector = requirement.Git.Selector
		} else {
			item.Selector = requirement.Registry.Selector
		}
		intent.Dependencies = append(intent.Dependencies, item)
	}
	for _, target := range sortedKeys(requirements.Targets) {
		aliases := append([]string(nil), requirements.Targets[target]...)
		sort.Strings(aliases)
		intent.Targets = append(intent.Targets, manifestTarget{ID: target, Aliases: aliases})
	}
	body, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (lock Lock) Validate(requirements Requirements) error {
	if lock.LockVersion != LockVersion {
		return fmt.Errorf("unsupported lock_version %d", lock.LockVersion)
	}
	digest, err := ManifestDigest(requirements)
	if err != nil {
		return err
	}
	if lock.ManifestDigest != digest {
		return errors.New("lock manifest digest does not match current intent")
	}
	seen := map[string]bool{}
	for _, pkg := range lock.Packages {
		if seen[pkg.Alias] {
			return fmt.Errorf("duplicate locked alias %q", pkg.Alias)
		}
		seen[pkg.Alias] = true
		requirement, ok := requirements.Dependencies[pkg.Alias]
		if !ok {
			return fmt.Errorf("lock contains removed alias %q", pkg.Alias)
		}
		key, _ := requirement.SourceKey()
		if pkg.SourceKey != key || pkg.DigestAlgorithm != TreeDigestAlgorithm ||
			!regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(pkg.TreeDigest) ||
			!regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(pkg.ManifestDigest) {
			return fmt.Errorf("locked package %q has incomplete identity", pkg.Alias)
		}
		if err := validateLockedSelector(requirement, pkg); err != nil {
			return fmt.Errorf("%s: %w", pkg.Alias, err)
		}
		got := append([]string(nil), pkg.Targets...)
		sort.Strings(got)
		if !slices.Equal(got, targetsFor(requirements, pkg.Alias)) {
			return fmt.Errorf("%s: locked targets changed", pkg.Alias)
		}

	}
	for alias := range requirements.Dependencies {
		if !seen[alias] {
			return fmt.Errorf("lock is missing alias %q", alias)
		}
	}
	return nil
}

// ValidateCompatiblePins checks source and selector identity while allowing a
// target-only manifest change to retain every existing revision pin.
func (lock Lock) ValidateCompatiblePins(requirements Requirements) error {
	if lock.LockVersion != LockVersion {
		return fmt.Errorf("unsupported lock_version %d", lock.LockVersion)
	}
	seen := map[string]bool{}
	for _, pkg := range lock.Packages {
		if seen[pkg.Alias] {
			return fmt.Errorf("duplicate locked alias %q", pkg.Alias)
		}
		seen[pkg.Alias] = true
		req := Requirement{Alias: pkg.Alias}
		if pkg.Source.Git != nil {
			req.Git = &GitSource{Repository: pkg.Source.Git.Repository, Path: pkg.Source.Git.Path, Selector: pkg.Source.Git.Requested}
		}
		if pkg.Source.Registry != nil {
			req.Registry = &RegistrySource{Distribution: pkg.Source.Registry.Distribution, Selector: pkg.Source.Registry.Requested}
		}
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
		if pkg.DigestAlgorithm != TreeDigestAlgorithm || !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(pkg.TreeDigest) || !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(pkg.ManifestDigest) {
			return fmt.Errorf("%s: invalid locked digest", pkg.Alias)
		}
	}
	return nil
}

func validateLockedSelector(requirement Requirement, pkg LockedPackage) error {
	if requirement.Git != nil {
		if pkg.Source.Git == nil || pkg.Source.Registry != nil || pkg.Source.Git.Repository != requirement.Git.Repository ||
			pkg.Source.Git.Path != requirement.Git.Path || pkg.Source.Git.Requested != requirement.Git.Selector ||
			!commitPattern.MatchString(pkg.Source.Git.Commit) {
			return errors.New("locked Git identity does not match manifest")
		}
		if requirement.Git.Selector.Kind == SelectorCommit && pkg.Source.Git.Commit != requirement.Git.Selector.Value {
			return errors.New("locked commit differs from exact selector")
		}
		return nil
	}
	source := pkg.Source.Registry
	if source == nil || pkg.Source.Git != nil || source.Distribution != requirement.Registry.Distribution ||
		source.Requested != requirement.Registry.Selector || source.ReleaseSequence == 0 || fmt.Sprint(source.ReleaseSequence) != requirement.Registry.Selector.Value ||
		source.DistributionID != source.Distribution || !distributionPattern.MatchString(source.DistributionID) || !repositoryPattern.MatchString(source.Repository) || validateSubpath(source.Path) != nil || !commitPattern.MatchString(source.Commit) {
		return errors.New("locked registry identity does not match manifest")
	}
	return nil
}

func (lock Lock) Package(alias string) (LockedPackage, bool) {
	for _, pkg := range lock.Packages {
		if pkg.Alias == alias {
			return pkg, true
		}
	}
	return LockedPackage{}, false
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
