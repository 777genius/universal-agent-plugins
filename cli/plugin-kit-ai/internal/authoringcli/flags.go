package authoringcli

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var installerOnly = [...]string{"scope", "accept-security-risk", "security-details"}

// flagSets includes ancestor definitions even when a child shadows a name.
func flagSets(cmd *cobra.Command, visit func(*pflag.FlagSet)) {
	for c := cmd; c != nil; c = c.Parent() {
		visit(c.Flags())
		visit(c.PersistentFlags())
	}
}

// AdaptFlags must run before decoding inputs or invoking any service. Explicit
// false/empty/default-valued installer flags are still rejected. Unsupported
// inherited defaults are ignored; explicitly selected unsupported flags fail.
func AdaptFlags(cmd *cobra.Command, support Support) (Options, error) {
	opts := Options{Format: "human"}
	var invalid error
	for _, name := range installerOnly {
		flagSets(cmd, func(fs *pflag.FlagSet) {
			if f := fs.Lookup(name); f != nil && f.Changed && invalid == nil {
				invalid = fmt.Errorf("--%s is installer-only; select the authoring package by positional path; use agentplugins add for installation policy", name)
			}
		})
	}
	if invalid != nil {
		return Options{}, invalid
	}
	for _, item := range []struct {
		name      string
		supported bool
		kind      string
	}{
		{"format", support.Format, "string"}, {"no-color", support.NoColor, "bool"},
		{"dry-run", support.DryRun, "bool"}, {"target", support.Target, "string"},
	} {
		var found *pflag.Flag
		flagSets(cmd, func(fs *pflag.FlagSet) {
			f := fs.Lookup(item.name)
			if f == nil {
				return
			}
			if found != nil && found != f {
				invalid = fmt.Errorf("authoring must not shadow inherited --%s", item.name)
			}
			found = f
		})
		if invalid != nil {
			return Options{}, invalid
		}
		if found == nil {
			continue
		}
		if !item.supported {
			if found.Changed {
				return Options{}, fmt.Errorf("--%s is not supported by %s", item.name, cmd.CommandPath())
			}
			continue
		}
		if found.Value.Type() != item.kind {
			return Options{}, fmt.Errorf("--%s requires %s flag adapter", item.name, item.kind)
		}
		// Getters perform typed parsing without sharing the original pflag.Value.
		fs := pflag.NewFlagSet("authoring snapshot", pflag.ContinueOnError)
		fs.AddFlag(found)
		switch item.name {
		case "format":
			opts.Format, invalid = fs.GetString(item.name)
			if invalid == nil && opts.Format != "human" && opts.Format != "json" {
				invalid = fmt.Errorf("--format must be human or json")
			}
		case "target":
			opts.Target, invalid = fs.GetString(item.name)
		case "no-color":
			opts.NoColor, invalid = fs.GetBool(item.name)
		case "dry-run":
			opts.DryRun, invalid = fs.GetBool(item.name)
		}
		if invalid != nil {
			return Options{}, invalid
		}
	}
	return opts, nil
}

// Cobra retains parsed values across Execute calls. Reset only flags in this
// invocation's chain after execution/argument rejection; sibling command flags
// and streams are untouched. No installer command installs this behavior.
func resetFlags(cmd *cobra.Command) {
	seen := map[*pflag.Flag]bool{}
	flagSets(cmd, func(fs *pflag.FlagSet) {
		fs.VisitAll(func(f *pflag.Flag) {
			if seen[f] || !f.Changed {
				return
			}
			seen[f] = true
			// Slice defaults need pflag's typed replacement, not parsing "[a,b]".
			if slice, ok := f.Value.(pflag.SliceValue); ok {
				var values []string
				var err error
				if f.DefValue != "[]" {
					values, err = csv.NewReader(strings.NewReader(strings.TrimSuffix(strings.TrimPrefix(f.DefValue, "["), "]"))).Read()
				}
				if err == nil && slice.Replace(values) == nil {
					f.Changed = false
				}
				return
			}
			if f.Value.Set(f.DefValue) == nil {
				f.Changed = false
			}
		})
	})
}
