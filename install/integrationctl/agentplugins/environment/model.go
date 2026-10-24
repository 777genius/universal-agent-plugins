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
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
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
	SelectorArchive SelectorKind = "archive"
	SelectorCommit  SelectorKind = "commit"
	SelectorRef     SelectorKind = "ref"
	SelectorDefault SelectorKind = "default"
	SelectorRelease SelectorKind = "release"
	SelectorVersion SelectorKind = "version"
	SelectorRange   SelectorKind = "range"
)

type Selector struct {
	Kind  SelectorKind `json:"kind"`
	Value string       `json:"value,omitempty"`
}

const SkillKind = "skill"
const SkillNormalizerV1 = "agent-skill-package/v1"

type GitSource struct {
	URL        string   `json:"url,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Skill      string   `json:"skill,omitempty"`
	Repository string   `json:"repository,omitempty"`
	Path       string   `json:"path,omitempty"`
	Selector   Selector `json:"selector"`
}

type RegistrySource struct {
	Distribution string   `json:"distribution"`
	Selector     Selector `json:"selector"`
}

type ArchiveSource struct {
	URL    string `json:"url"`
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

type LockedArchiveSource struct {
	URL             string `json:"url"`
	Path            string `json:"path,omitempty"`
	RequestedSHA256 string `json:"requested_sha256,omitempty"`
	SHA256          string `json:"sha256"`
}

type WebSource struct {
	URL   string `json:"url"`
	Skill string `json:"skill"`
}

// LockedWebSource records the complete immutable website acquisition contract.
type LockedWebSource = domain.WebSkillSourceFacts

type Requirement struct {
	Web      *WebSource      `json:"web,omitempty"`
	Archive  *ArchiveSource  `json:"archive,omitempty"`
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
	Web      *LockedWebSource      `json:"web,omitempty"`
	Archive  *LockedArchiveSource  `json:"archive,omitempty"`
	Git      *LockedGitSource      `json:"git,omitempty"`
	Registry *LockedRegistrySource `json:"registry,omitempty"`
}

type LockedGitSource struct {
	URL               string   `json:"url,omitempty"`
	Kind              string   `json:"kind,omitempty"`
	Skill             string   `json:"skill,omitempty"`
	ResolvedSkillPath string   `json:"resolved_skill_path,omitempty"`
	RawTreeDigest     string   `json:"raw_tree_digest,omitempty"`
	Normalizer        string   `json:"normalizer,omitempty"`
	Repository        string   `json:"repository,omitempty"`
	Path              string   `json:"path,omitempty"`
	Requested         Selector `json:"requested"`
	Commit            string   `json:"commit"`
	ResolvedRef       string   `json:"resolved_ref,omitempty"`
	DefaultBranch     string   `json:"default_branch,omitempty"`
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

type RequiredBy struct {
	Alias    string   `json:"alias"`
	Selector Selector `json:"selector"`
}

type LockedPackage struct {
	Version         string       `json:"version,omitempty"`
	RequiredBy      []RequiredBy `json:"required_by,omitempty"`
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
	Incomplete                                                 bool
	Drift, Pending                                             bool
}

type DesiredAction string

const (
	ActionUnchanged DesiredAction = "unchanged"
	ActionInstall   DesiredAction = "install"
	ActionUpdate    DesiredAction = "update"
	ActionRemove    DesiredAction = "remove"
)

type Attestation struct {
	Alias, ClientID                  string
	ActivationComplete, AuthComplete bool
}

type PreparedBinding struct {
	ActivationComplete, AuthComplete           bool
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
	case SelectorArchive:
		if selector.Value != "" && !sourcespec.ValidSHA256(selector.Value) {
			return errors.New("archive selector requires a lowercase SHA256 or a moving URL")
		}
	case SelectorCommit:
		if !commitPattern.MatchString(selector.Value) {
			return errors.New("commit selector requires one full lowercase 40-character SHA")
		}
	case SelectorRef:
		return validateRefSelector(selector.Value)
	case SelectorDefault:
		if selector.Value != "" {
			return errors.New("default selector must have an empty value")
		}
	case SelectorVersion, SelectorRange:
		return validateVersionSelector(selector)
	case SelectorRelease:
		return validateReleaseSelector(selector.Value)
	default:
		return fmt.Errorf("unsupported selector kind %q", selector.Kind)
	}
	return nil
}

func validateRefSelector(value string) error {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, " \t\r\n~^:?*[\\") || strings.HasPrefix(value, "-") || path.Clean(value) != value {
		return errors.New("ref selector is not a safe exact Git ref")
	}
	return nil
}
func validateReleaseSelector(value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return errors.New("release selector must be a canonical nonzero sequence")
	}
	var sequence uint64
	if _, err := fmt.Sscanf(value, "%d", &sequence); err != nil || sequence == 0 || fmt.Sprint(sequence) != value {
		return errors.New("release selector must be a canonical nonzero sequence")
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

// Declaration keys are logical intent, never filesystem names or receipt aliases.
func validDeclarationKey(value string) bool {
	return aliasPattern.MatchString(value) || (strings.Count(value, "/") == 1 && distributionPattern.MatchString(value))
}

func (requirement Requirement) Validate() error {
	if !validDeclarationKey(requirement.Alias) {
		return fmt.Errorf("alias %q must match %s", requirement.Alias, aliasPattern)
	}
	if sourceCount(requirement.Git != nil, requirement.Registry != nil, requirement.Archive != nil, requirement.Web != nil) != 1 {
		return errors.New("dependency must contain exactly one git, archive, registry or website source")
	}
	if requirement.Web != nil {
		return requirement.Web.Validate()
	}
	if requirement.Archive != nil {
		return requirement.Archive.Validate()
	}
	if requirement.Git != nil {
		if err := validateSkillSource(requirement.Git); err != nil {
			return err
		}
		if err := requirement.Git.ValidateEndpoint(); err != nil {
			return err
		}
		if err := validateSubpath(requirement.Git.Path); err != nil {
			return err
		}
		if requirement.Git.Selector.Kind != SelectorCommit && requirement.Git.Selector.Kind != SelectorRef && requirement.Git.Selector.Kind != SelectorDefault {
			return errors.New("git source requires commit, ref or default selector")
		}
		return requirement.Git.Selector.Validate()
	}
	if !distributionPattern.MatchString(requirement.Registry.Distribution) {
		return errors.New("registry distribution must be canonical owner/name")
	}
	if requirement.Registry.Selector.Kind != SelectorRelease && requirement.Registry.Selector.Kind != SelectorVersion && requirement.Registry.Selector.Kind != SelectorRange {
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
	if requirement.Web != nil {
		return "https-skill:" + requirement.Web.URL + ":" + requirement.Web.Skill, nil
	}
	if requirement.Git != nil {
		if requirement.Git.Kind == SkillKind {
			return "github-skill:" + strings.ToLower(requirement.Git.Repository) + ":" + requirement.Git.Skill + ":" + requirement.Git.Path, nil
		}
		if requirement.Git.URL != "" {
			return sourcespec.Key(sourcespec.KindGit, requirement.Git.URL, requirement.Git.Path), nil
		}
		return "github:" + strings.ToLower(requirement.Git.Repository) + ":" + requirement.Git.Path, nil
	}
	if requirement.Archive != nil {
		return sourcespec.Key(sourcespec.KindArchive, requirement.Archive.URL, requirement.Archive.Path), nil
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
		} else if requirement.Archive != nil {
			item.Path = requirement.Archive.Path
			item.Selector = requirement.RequestedSelector()
		} else {
			item.Selector = requirement.RequestedSelector()
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
	return validateGraph(lock, requirements)
}

// ValidateCompatiblePins checks old pin identity using its recorded selectors;
// compatibility with changed intent is decided by resolution.
func (lock Lock) ValidateCompatiblePins(_ Requirements) error {
	if lock.LockVersion != LockVersion {
		return fmt.Errorf("unsupported lock_version %d", lock.LockVersion)
	}
	aliases, sources := map[string]LockedPackage{}, map[string]bool{}
	for _, pkg := range lock.Packages {
		if _, exists := aliases[pkg.Alias]; exists {
			return fmt.Errorf("duplicate locked alias %q", pkg.Alias)
		}
		if sources[pkg.SourceKey] {
			return fmt.Errorf("duplicate locked source %q", pkg.SourceKey)
		}
		if err := validatePin(pkg); err != nil {
			return fmt.Errorf("%s: %w", pkg.Alias, err)
		}
		aliases[pkg.Alias], sources[pkg.SourceKey] = pkg, true
	}
	return validateEdges(aliases)
}

func validateLockedSelector(requirement Requirement, pkg LockedPackage) error {
	if requirement.Web != nil {
		return validateLockedWeb(requirement, pkg)
	}
	if requirement.Git != nil {
		if pkg.Source.Git == nil || pkg.Source.Registry != nil || pkg.Source.Archive != nil || pkg.Source.Web != nil || pkg.Source.Git.URL != requirement.Git.URL || !strings.EqualFold(pkg.Source.Git.Repository, requirement.Git.Repository) ||
			pkg.Source.Git.Path != requirement.Git.Path || pkg.Source.Git.Kind != requirement.Git.Kind || pkg.Source.Git.Skill != requirement.Git.Skill || pkg.Source.Git.Requested != requirement.Git.Selector ||
			!commitPattern.MatchString(pkg.Source.Git.Commit) {
			return errors.New("locked Git identity does not match manifest")
		}
		if err := validateLockedSkill(pkg.Source.Git); err != nil {
			return err
		}
		if requirement.Git.Selector.Kind == SelectorCommit && pkg.Source.Git.Commit != requirement.Git.Selector.Value {
			return errors.New("locked commit differs from exact selector")
		}
		return nil
	}
	if requirement.Archive != nil {
		return validateLockedArchive(requirement, pkg)
	}
	return validateLockedRegistrySelector(requirement, pkg)
}

func validateLockedRegistrySelector(requirement Requirement, pkg LockedPackage) error {
	source := pkg.Source.Registry
	if source == nil || pkg.Source.Git != nil || pkg.Source.Archive != nil || pkg.Source.Web != nil || !strings.EqualFold(source.Distribution, requirement.Registry.Distribution) ||
		source.Requested != requirement.Registry.Selector || source.ReleaseSequence == 0 || !registrySelectorMatches(requirement.Registry.Selector, pkg) ||
		!strings.EqualFold(source.DistributionID, source.Distribution) || !distributionPattern.MatchString(source.DistributionID) || !repositoryPattern.MatchString(source.Repository) || validateSubpath(source.Path) != nil || !commitPattern.MatchString(source.Commit) {
		return errors.New("locked registry identity does not match manifest")
	}
	return nil
}

// PackageForRequirement maps a declaration to its stable graph identity.
func (lock Lock) PackageForRequirement(requirement Requirement) (LockedPackage, bool) {
	key, err := requirement.SourceKey()
	if err != nil {
		return LockedPackage{}, false
	}
	var found LockedPackage
	count := 0
	for _, pkg := range lock.Packages {
		if pkg.SourceKey == key {
			found = pkg
			count++
		}
	}
	return found, count == 1
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
