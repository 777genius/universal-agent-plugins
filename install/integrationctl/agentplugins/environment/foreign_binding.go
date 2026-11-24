package environment

import (
	"fmt"
	"sort"
	"strings"
)

// ForeignBinding is one existing client binding of the dependency's source that
// the saved environment does not own.
type ForeignBinding struct {
	InstallationID string
	ClientID       string
}

// ForeignBindingError reports that the dependency's source is already installed
// outside the saved environment. Hosts use the typed bindings to tell the user
// which client holds it and how to remove it; the environment never adopts it.
type ForeignBindingError struct {
	Alias    string
	Bindings []ForeignBinding
}

// CheckSourceOwnership is a read-only preview of the same installation-wide
// ownership rule used by sync. Hosts may call it before offering targets; the
// authoritative mutation still revalidates ownership under its lock.
func CheckSourceOwnership(pkg LockedPackage, observed []BindingObservation) error {
	for _, binding := range observed {
		if !binding.Managed && binding.SourceKey == pkg.SourceKey {
			return foreignBindingError(pkg, observed)
		}
	}
	return nil
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
	return fmt.Sprintf("foreign: %s has an unmanaged binding in %s %s; remove it before adding the dependency", e.Alias, noun, strings.Join(installations, ", "))
}

// foreignBindingError describes every unmanaged binding of the package's source,
// not only the first one that stopped validation.
func foreignBindingError(pkg LockedPackage, observed []BindingObservation) error {
	var bindings []ForeignBinding
	for _, existing := range observed {
		if !existing.Managed && existing.SourceKey == pkg.SourceKey {
			bindings = append(bindings, ForeignBinding{InstallationID: existing.InstallationID, ClientID: existing.ClientID})
		}
	}
	sort.SliceStable(bindings, func(i, j int) bool {
		if bindings[i].InstallationID != bindings[j].InstallationID {
			return bindings[i].InstallationID < bindings[j].InstallationID
		}
		return bindings[i].ClientID < bindings[j].ClientID
	})
	return &ForeignBindingError{Alias: pkg.Alias, Bindings: bindings}
}
