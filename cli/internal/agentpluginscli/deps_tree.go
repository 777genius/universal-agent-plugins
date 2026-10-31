package agentpluginscli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyRootMapping struct {
	Key       string `json:"key"`
	SourceKey string `json:"source_key"`
	Alias     string `json:"alias"`
}

type dependencyTree struct {
	RootMappings []dependencyRootMapping     `json:"root_mappings,omitempty"`
	Roots        []string                    `json:"roots"`
	Packages     []environment.LockedPackage `json:"packages"`
}

func newDependencyTreeCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{Use: "tree", Short: "Show the recorded dependency graph", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		app, scopeErr := app.forOptions(opts)
		if scopeErr != nil {
			return scopeErr
		}
		if err := validateDepsOptions(opts); err != nil {
			return err
		}
		manifestPath, lockPath := app.dependencyPaths()
		requirements, err := readDependencyManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("read dependency intent: %w; run deps lock", err)
		}
		if err := validateDependencyTargets(app, requirements); err != nil {
			return err
		}
		locked, err := readDependencyLock(lockPath, requirements)
		if err != nil {
			return fmt.Errorf("read dependency graph: %w; run deps lock", err)
		}
		tree := dependencyTree{Roots: make([]string, 0, len(requirements.Dependencies)), Packages: append([]environment.LockedPackage{}, locked.Packages...)}
		if err := canonicalizeRequirementsFromLock(&requirements, locked); err != nil {
			return err
		}
		for key, requirement := range requirements.Dependencies {
			alias := dependencyNodeAlias(requirements, locked, key)
			tree.Roots = append(tree.Roots, alias)
			if key != alias {
				tree.RootMappings = append(tree.RootMappings, dependencyRootMapping{Key: key, SourceKey: mustSourceKey(requirement), Alias: alias})
			}
		}
		sort.Slice(tree.RootMappings, func(i, j int) bool { return tree.RootMappings[i].Key < tree.RootMappings[j].Key })
		sort.Strings(tree.Roots)
		sort.Slice(tree.Packages, func(i, j int) bool { return tree.Packages[i].Alias < tree.Packages[j].Alias })
		if opts.format == "json" {
			return renderJSON(cmd.OutOrStdout(), tree)
		}
		return renderDependencyTree(cmd.OutOrStdout(), tree)
	}}
}

func renderDependencyTree(writer io.Writer, tree dependencyTree) error {
	if len(tree.Roots) == 0 {
		_, err := fmt.Fprintln(writer, "No packages are recorded in this lock.")
		return err
	}
	if _, err := fmt.Fprintln(writer, "Saved dependencies (exact locked versions):"); err != nil {
		return err
	}
	for _, root := range tree.RootMappings {
		if _, err := fmt.Fprintf(writer, "%s → %s (%s)\n", root.Key, root.Alias, root.SourceKey); err != nil {
			return err
		}
	}
	packages := make(map[string]environment.LockedPackage, len(tree.Packages))
	children := make(map[string][]string)
	for _, pkg := range tree.Packages {
		packages[pkg.Alias] = pkg
		for _, edge := range pkg.RequiredBy {
			children[edge.Alias] = append(children[edge.Alias], pkg.Alias)
		}
	}
	for alias := range children {
		sort.Strings(children[alias])
	}
	seen := make(map[string]bool)
	for _, alias := range tree.Roots {
		if err := renderDependencyTreeNode(writer, alias, "", packages, children, seen); err != nil {
			return err
		}
	}
	if len(children) > 0 {
		_, err := fmt.Fprintln(writer, "Indentation shows which package requires another; an indented package may also be a direct dependency. (reference) means the same locked package was already shown.")
		return err
	}
	return nil
}

func renderDependencyTreeNode(writer io.Writer, alias, indent string, packages map[string]environment.LockedPackage, children map[string][]string, seen map[string]bool) error {
	pkg := packages[alias]
	reference := ""
	if seen[alias] {
		reference = " (reference)"
	}
	if _, err := fmt.Fprintf(writer, "%s%s %s%s\n", indent, alias, dependencyTreeVersion(pkg), reference); err != nil {
		return err
	}
	if seen[alias] {
		return nil
	}
	seen[alias] = true
	previous := ""
	for _, child := range children[alias] {
		if child == previous {
			continue
		}
		previous = child
		if err := renderDependencyTreeNode(writer, child, indent+"  ", packages, children, seen); err != nil {
			return err
		}
	}
	return nil
}

func dependencyTreeVersion(pkg environment.LockedPackage) string {
	if pkg.Version != "" {
		return pkg.Version
	}
	if pkg.Source.Registry != nil {
		return fmt.Sprintf("release %d", pkg.Source.Registry.ReleaseSequence)
	}
	return strings.TrimSpace(lockedCommit(pkg))
}
