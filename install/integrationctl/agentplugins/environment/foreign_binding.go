package environment

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ForeignBinding is one existing client binding of the dependency's source that
// the saved environment does not own.
type ForeignBinding struct {
	InstallationID     string
	ClientID           string
	Reason             string
	OwnerEnvironmentID string
}

// ForeignBindingError reports that the dependency's source is already installed
// outside the saved environment. Hosts use these observed facts for presentation;
// the environment never adopts the existing binding.
type ForeignBindingError struct {
	Alias     string
	Bindings  []ForeignBinding
	SourceKey string
	RootAlias string
}

// CheckSourceOwnership is a read-only preview of the same installation-wide
// ownership rule used by sync. Hosts may call it before offering targets; the
// authoritative mutation still revalidates ownership under its lock.
func CheckSourceOwnership(pkg LockedPackage, observed []BindingObservation) error {
	if pkg.SourceKey == "" {
		return nil
	}
	var bindings []ForeignBinding
	seen := map[ForeignBinding]bool{}
	for _, existing := range observed {
		if existing.Managed || existing.SourceKey != pkg.SourceKey {
			continue
		}
		binding := ForeignBinding{InstallationID: existing.InstallationID, ClientID: existing.ClientID, Reason: "unmanaged_binding", OwnerEnvironmentID: existing.OwnerEnvironmentID}
		if binding.OwnerEnvironmentID != "" {
			binding.Reason = "foreign_environment"
		}
		if !seen[binding] {
			seen[binding] = true
			bindings = append(bindings, binding)
		}
	}
	if len(bindings) == 0 {
		return nil
	}
	sort.Slice(bindings, func(i, j int) bool {
		a, b := bindings[i], bindings[j]
		if a.InstallationID != b.InstallationID {
			return a.InstallationID < b.InstallationID
		}
		if a.Reason != b.Reason {
			return a.Reason < b.Reason
		}
		if a.OwnerEnvironmentID != b.OwnerEnvironmentID {
			return a.OwnerEnvironmentID < b.OwnerEnvironmentID
		}
		return a.ClientID < b.ClientID
	})
	return &ForeignBindingError{Alias: pkg.Alias, SourceKey: pkg.SourceKey, Bindings: bindings}
}

func (e *ForeignBindingError) Error() string {
	installations := make([]string, 0, len(e.Bindings))
	seen := map[string]bool{}
	for _, binding := range e.Bindings {
		if !seen[binding.InstallationID] {
			seen[binding.InstallationID] = true
			installations = append(installations, binding.InstallationID)
		}
	}
	noun := "installation"
	if len(installations) > 1 {
		noun = "installations"
	}
	identity := e.Alias
	if identity == "" {
		identity = e.SourceKey
	}
	return fmt.Sprintf("foreign: %s has bindings outside this environment in %s %s", identity, noun, strings.Join(installations, ", "))
}

func withForeignRoot(err error, requirements Requirements, lock Lock) error {
	var foreign *ForeignBindingError
	if errors.As(err, &foreign) {
		foreign.RootAlias = uniqueForeignRoot(requirements, lock, foreign.Alias)
	}
	return err
}

// uniqueForeignRoot reports a declared dependency ancestor only when one root
// reaches the refused package. Graph facts have already passed lock validation.
func uniqueForeignRoot(requirements Requirements, lock Lock, alias string) string {
	root := ""
	for declaration, requirement := range requirements.Dependencies {
		pkg, found := lock.PackageForRequirement(requirement)
		if !found {
			continue
		}
		if pkg.Alias == alias {
			return ""
		}
		for _, dependency := range lock.ClosureAliases([]string{pkg.Alias}) {
			if dependency == alias {
				if root != "" {
					return ""
				}
				root = declaration
				break
			}
		}
	}
	return root
}
