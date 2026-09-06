package commands

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/spf13/cobra"
	"io"
	"strconv"
)

// ReleaseOptions is explicit private composition, never environment discovery.
// Reject receives observations from the shared parser and may only reject.
type ReleaseOptions struct {
	Product, Version string
	Reject           func(Invocation) error
}
type Invocation struct {
	Command *cobra.Command
	Values  map[string][]string
}

// Bool observes pflag's final boolean value. Invalid input never implies intent;
// compatibility still rejects the invocation based on explicit flag presence.
func (in Invocation) Bool(name string) bool {
	v := in.Values[name]
	if len(v) == 0 {
		return false
	}
	b, err := strconv.ParseBool(v[len(v)-1])
	return err == nil && b
}

type versionPayload struct {
	report.Public
	Product        string `json:"product"`
	ProductVersion string `json:"product_version"`
}

func NewVersionCommand() (*cobra.Command, error) {
	c := &cobra.Command{Use: "version", Short: "Print product version and shared authoring engine revision", Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			_, err := authoringcli.AdaptFlags(c, authoringcli.Support{Format: true, NoColor: true})
			return err
		}}
	tagOperations(c, "author.version")
	return c, nil
}

func isCompletion(c *cobra.Command) bool {
	for ; c != nil; c = c.Parent() {
		switch c.Name() {
		case "completion", "__complete", "__completeNoDesc":
			return true
		}
	}
	return false
}

// ReleaseSelection builds the same complete tree used by Execute, with inert
// captures. Installer construction is dependency-free until a genuine job wins.
func (a App) ReleaseSelection(args []string, build RootBuilder) (root *cobra.Command, author, noEffect bool, err error) {
	root, err = a.releaseTree(build)
	if err != nil {
		return
	}
	s := selectPublic(root, args)
	author = s.command.Annotations[operationKey] != ""
	noEffect = len(args) > 0 && (args[0] == "__complete" || args[0] == "__completeNoDesc") || author || s.help || s.command == root || s.command.Name() == "version" || isCompletion(s.command)
	a.guardReleaseCompletion(root, build)
	return
}

func (a App) releaseTree(build RootBuilder) (*cobra.Command, error) {
	var factories []authoringcli.Factory
	for _, name := range commandNames() {
		factories = append(factories, func() (*cobra.Command, error) {
			c, e := a.command(name, func(report.Report) {})
			if e == nil {
				tagOperations(c, "author."+name)
			}
			return c, e
		})
	}
	factories = append(factories, NewVersionCommand)
	root, e := build(factories...)
	if e != nil {
		return nil, e
	}
	var prepare func(*cobra.Command)
	prepare = func(c *cobra.Command) {
		c.InitDefaultHelpFlag()
		if c.Name() == "author" {
			c.Annotations = map[string]string{operationKey: "author"}
		}
		for _, child := range c.Commands() {
			prepare(child)
		}
	}
	root.SetOut(io.Discard)
	authoringcli.PrepareReleaseUtilities(root, false)
	prepare(root)
	return root, nil
}

// OutputFormat observes only the real tree's flag arities, including error paths.
func OutputFormat(root *cobra.Command, args []string) string { return selectPublic(root, args).format }

func CompletionInvocation(root *cobra.Command, args []string) bool {
	return isCompletion(selectPublic(root, args).command)
}

// Cobra's protocol prints getCompletions errors directly to process stderr.
// Check its parser boundary on an inert fresh tree before that Run is reached.
// Never redirect process globals, and never pre-parse the execution tree (slice
// flags and completion's Changed bookkeeping must remain untouched).
func (a App) guardReleaseCompletion(root *cobra.Command, build RootBuilder) {
	previousE, previous := root.PersistentPreRunE, root.PersistentPreRun
	root.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		if c.Name() == cobra.ShellCompRequestCmd || c.Name() == cobra.ShellCompNoDescRequestCmd {
			probe, err := a.releaseTree(build)
			if err == nil {
				authoringcli.PrepareReleaseUtilities(probe, true)
				err = authoringcli.CheckReleaseCompletion(probe, args)
			}
			if err != nil {
				return &inputError{"arguments_invalid", publicArguments}
			}
			return nil
		}
		if previousE != nil {
			return previousE(c, args)
		}
		if previous != nil {
			previous(c, args)
		}
		return nil
	}
}
