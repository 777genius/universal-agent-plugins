package environment

import (
	"errors"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func sourceCount(values ...bool) int {
	n := 0
	for _, value := range values {
		if value {
			n++
		}
	}
	return n
}

func (source GitSource) ValidateEndpoint() error {
	if (source.Repository == "") == (source.URL == "") {
		return errors.New("git source requires exactly one repository or URL")
	}
	if source.URL == "" {
		if !repositoryPattern.MatchString(source.Repository) {
			return errors.New("git repository must be owner/repo")
		}
		return nil
	}
	endpoint, repository, err := sourcespec.NormalizeGitURL(source.URL)
	if err != nil || endpoint != source.URL || repository != "" {
		return errors.New("git URL must be a normalized HTTPS/SSH endpoint; use repository for canonical GitHub HTTPS")
	}
	return nil
}

func (source GitSource) Endpoint() string {
	if source.URL != "" {
		return source.URL
	}
	return "https://github.com/" + source.Repository + ".git"
}

func (source ArchiveSource) Validate() error {
	endpoint, err := sourcespec.NormalizeArchiveURL(source.URL)
	if err != nil || endpoint != source.URL {
		return errors.New("archive dependency requires a normalized credential-free HTTPS URL")
	}
	if _, err := sourcespec.NormalizeSubpath(source.Path); err != nil {
		return err
	}
	if source.SHA256 != "" && !sourcespec.ValidSHA256(source.SHA256) {
		return errors.New("archive sha256 requires 64 lowercase hexadecimal characters")
	}
	return nil
}

func (requirement Requirement) RequestedSelector() Selector {
	if requirement.Git != nil {
		return requirement.Git.Selector
	}
	if requirement.Archive != nil {
		return Selector{Kind: SelectorArchive, Value: requirement.Archive.SHA256}
	}
	if requirement.Registry != nil {
		return requirement.Registry.Selector
	}
	return Selector{}
}

func validateLockedArchive(requirement Requirement, pkg LockedPackage) error {
	a := pkg.Source.Archive
	r := requirement.Archive
	if a == nil || pkg.Source.Git != nil || pkg.Source.Registry != nil || a.URL != r.URL || a.Path != r.Path || a.RequestedSHA256 != r.SHA256 || !sourcespec.ValidSHA256(a.SHA256) || (r.SHA256 != "" && r.SHA256 != a.SHA256) {
		return errors.New("locked archive identity does not match manifest")
	}
	return nil
}

// Requirement reconstructs only portable intent from a validated pin. It never
// adopts mutable store provenance or a machine's profile/project paths.
func (pkg LockedPackage) Requirement() Requirement {
	r := Requirement{Alias: pkg.Alias}
	if s := pkg.Source.Git; s != nil {
		r.Git = &GitSource{Repository: s.Repository, URL: s.URL, Path: s.Path, Selector: s.Requested}
	}
	if s := pkg.Source.Archive; s != nil {
		r.Archive = &ArchiveSource{URL: s.URL, Path: s.Path, SHA256: s.RequestedSHA256}
	}
	if s := pkg.Source.Registry; s != nil {
		r.Registry = &RegistrySource{Distribution: s.Distribution, Selector: s.Requested}
	}
	return r
}

// Revision returns the immutable version fact used by all lifecycle receipts.
func (pkg LockedPackage) Revision() string { return lockedRevision(pkg) }

func (requirement Requirement) SameSource(other Requirement) bool {
	a, err := requirement.SourceKey()
	if err != nil {
		return false
	}
	b, err := other.SourceKey()
	return err == nil && strings.Compare(a, b) == 0
}
