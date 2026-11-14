package project

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// withCommands applies the installer's command rules to the captured tree, so
// authoring reports exactly what an install would deliver. It reads only bytes
// the lease captured and never reopens the source.
func withCommands(ctx context.Context, facts *conformance.Facts, input packageview.Input, lease *packageview.Lease) error {
	pkg := *facts.Package
	pkg.SkillsDirEntries = nil
	if directoryPresent(input, "skills") {
		pkg.SkillsDirEntries = childNames(input, "skills")
	}
	pkg, err := commandext.WithCommands(ctx, pkg, commandListing(input, lease))
	if err != nil {
		return err
	}
	facts.Package = &pkg
	return nil
}

func commandListing(input packageview.Input, lease *packageview.Lease) commandext.Listing {
	for _, dir := range []string{domain.ExtensionNamespace, commandext.RelativeDirectory} {
		o, ok := observation(input, dir)
		if !ok {
			return commandext.Listing{Root: commandext.RootAbsent}
		}
		if o.Kind != "directory" || o.State != packageview.Present || !o.Captured {
			return commandext.Listing{Root: commandext.RootInvalid}
		}
	}
	listing := commandext.Listing{Root: commandext.RootDirectory}
	prefix := commandext.RelativeDirectory + "/"
	for _, o := range input.Inventory {
		name, ok := strings.CutPrefix(o.Path, prefix)
		if !ok || strings.Contains(name, "/") {
			continue
		}
		entry := commandext.Entry{Name: name}
		switch o.Kind {
		case "file":
			entry.Body, ok = lease.Content(o.Path)
			entry.Unreadable = !ok || !o.Captured
		case "directory":
			entry.Kind = commandext.EntryDirectory
		case "symlink":
			entry.Kind = commandext.EntrySymlink
		default:
			entry.Kind = commandext.EntryOther
		}
		listing.Entries = append(listing.Entries, entry)
	}
	return listing
}

func directoryPresent(input packageview.Input, path string) bool {
	o, ok := observation(input, path)
	return ok && o.Kind == "directory" && o.State == packageview.Present
}

func observation(input packageview.Input, path string) (packageview.Observation, bool) {
	for _, o := range input.Inventory {
		if o.Path == path {
			return o, true
		}
	}
	return packageview.Observation{}, false
}

func childNames(input packageview.Input, dir string) []string {
	var names []string
	for _, o := range input.Inventory {
		if name, ok := strings.CutPrefix(o.Path, dir+"/"); ok && !strings.Contains(name, "/") {
			names = append(names, name)
		}
	}
	return names
}
