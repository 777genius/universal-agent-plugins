package authoringcli

import (
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// RejectionKey identifies private, terminal compatibility errors, never jobs.
const RejectionKey = "authoringcli.v1-rejection"

func NewReleaseAuthorCommand(factories ...Factory) (*cobra.Command, error) {
	c, err := NewAuthorCommand(factories...)
	if err == nil {
		c.Hidden = false
	}
	return c, err
}

func NewReleasePluginKitRoot(factories ...Factory) (*cobra.Command, error) {
	return NewPluginKitRoot(factories...)
}

// PrepareReleaseUtilities uses Cobra's real tree. Hidden retirement shims are
// removed before generating scripts, including generators that walk all nodes.
func PrepareReleaseUtilities(root *cobra.Command, completion bool) {
	if completion {
		var prune func(*cobra.Command)
		prune = func(c *cobra.Command) {
			for _, child := range c.Commands() {
				if child.Hidden {
					c.RemoveCommand(child)
				} else {
					prune(child)
				}
			}
		}
		prune(root)
	}
	root.InitDefaultCompletionCmd()
}

// CheckReleaseCompletion preflights Cobra 1.10's getCompletions error paths:
// command lookup, complete flag values, and the incomplete flag's identity.
// root is disposable: ParseFlags must never mutate the real completion tree.
// No command/Args/completion callback is executed and no raw error is exposed.
func CheckReleaseCompletion(root *cobra.Command, args []string) error {
	invalid := errors.New("invalid completion arguments")
	if len(args) == 0 || root.TraverseChildren {
		return invalid
	}
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.InitDefaultHelpCmd()
	c, flags, err := root.Find(args[:len(args)-1])
	if err != nil {
		return invalid
	}
	c.InitDefaultHelpFlag()
	c.InitDefaultVersionFlag()
	last := args[len(args)-1]
	name, equal := "", false
	parsed := flags
	// Match Cobra's incomplete-flag treatment, including the last shorthand in
	// a cluster. Complete values are validated by the actual pflag definitions.
	if strings.HasPrefix(last, "-") {
		if i := strings.IndexByte(last, '='); i >= 0 {
			equal = true
			if strings.HasPrefix(last, "--") {
				name = last[2:i]
			} else {
				name = last[i-1 : i]
			}
		}
	} else if len(flags) > 0 {
		prev := flags[len(flags)-1]
		if len(prev) > 1 && strings.HasPrefix(prev, "-") && prev != "--" && !strings.Contains(prev, "=") {
			if strings.HasPrefix(prev, "--") {
				name = prev[2:]
			} else {
				name = prev[len(prev)-1:]
			}
			parsed = flags[:len(flags)-1]
		}
	}
	unknown := false
	if name != "" {
		f := c.Flag(name)
		if len(name) == 1 {
			f = c.Flags().ShorthandLookup(name)
			if f == nil {
				f = c.InheritedFlags().ShorthandLookup(name)
			}
		}
		if f == nil {
			unknown, parsed = true, flags
		} else if !equal && f.NoOptDefVal != "" {
			parsed = flags
		}
	}
	// Cobra ignores an unknown incomplete flag after -- (or non-interspersed
	// positional arguments). Use its same argument-count test for that rule.
	_ = c.ParseFlags(append(append([]string{}, parsed...), "--"))
	count := c.Flags().NArg()
	if c.ParseFlags(parsed) != nil || unknown && count <= c.Flags().NArg() {
		return invalid
	}
	return nil
}
