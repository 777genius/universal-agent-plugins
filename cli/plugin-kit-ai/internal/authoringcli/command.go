// Package authoringcli constructs fresh, privately wired authoring commands.
// It contains no project services and is not registered by either released main.
package authoringcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Options is a value snapshot, never the installer's mutable options object.
// Target is the explicit comma-separated selection, not detected clients.
type Options struct {
	Format  string
	NoColor bool
	DryRun  bool
	Target  string
}

// Support declares which inherited flags a particular command understands.
type Support struct{ Format, NoColor, DryRun, Target bool }

// Streams belong to the selected command, including parent-provided streams.
type Streams struct {
	In       io.Reader
	Out, Err io.Writer
}

// Runner is instantiated with each command's own request and result types.
// A command never needs an application-wide backend or another command's ports.
type Runner[Request, Result any] interface {
	Run(context.Context, Request) (Result, error)
}

type RunnerFunc[Request, Result any] func(context.Context, Request) (Result, error)

func (f RunnerFunc[Request, Result]) Run(ctx context.Context, req Request) (Result, error) {
	return f(ctx, req)
}

// Spec separates Cobra decoding, the in-process runner, and existing rendering.
// Configure must register command-local flags without capturing shared mutable
// variables; Decode reads their values from the supplied command. Render owns
// the human/JSON contract, including failure reports, and must never prompt in
// JSON mode. The factory preserves runner errors and their exitx annotations.
type Spec[Request, Result any] struct {
	Use, Short, Long string
	Hidden           bool
	Args             cobra.PositionalArgs
	Support          Support
	Configure        func(*cobra.Command)
	Decode           func(*cobra.Command, Options, []string) (Request, error)
	Runner           Runner[Request, Result]
	Render           func(Streams, Options, Result, error) error
}

// NewCommand returns a new command and new flag sets on every call. Required
// dependencies fail at construction, so missing services cannot become stubs.
func NewCommand[Request, Result any](spec Spec[Request, Result]) (*cobra.Command, error) {
	if spec.Use == "" || spec.Decode == nil || nilRunner(spec.Runner) || spec.Render == nil {
		return nil, fmt.Errorf("authoring command requires use, decoder, runner, and renderer")
	}
	cmd := &cobra.Command{Use: spec.Use, Short: spec.Short, Long: spec.Long, Hidden: spec.Hidden}
	reset := func() {
		resetFlags(cmd)
		// pflag slice values retain an internal append bit even after Replace.
		// Reconstruct local Values instead of only clearing Flag.Changed.
		if spec.Configure != nil {
			fresh := &cobra.Command{}
			spec.Configure(fresh)
			for _, pair := range [][2]*pflag.FlagSet{{cmd.Flags(), fresh.Flags()}, {cmd.PersistentFlags(), fresh.PersistentFlags()}} {
				pair[1].VisitAll(func(defaultFlag *pflag.Flag) {
					if current := pair[0].Lookup(defaultFlag.Name); current != nil {
						current.Value = defaultFlag.Value
						current.Changed = false
					}
				})
			}
		}
	}
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		if _, err := AdaptFlags(cmd, spec.Support); err != nil {
			reset()
			return err
		}
		if spec.Args != nil {
			if err := spec.Args(cmd, args); err != nil {
				reset()
				return err
			}
		}
		return nil
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		defer reset()
		opts, err := AdaptFlags(cmd, spec.Support)
		if err != nil {
			return err
		}
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		req, err := spec.Decode(cmd, opts, append([]string(nil), args...))
		if err != nil {
			return err
		}
		result, runErr := spec.Runner.Run(cmd.Context(), req)
		renderErr := spec.Render(Streams{cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()}, opts, result, runErr)
		if renderErr == nil {
			return runErr
		}
		if runErr == nil {
			return renderErr
		}
		return errors.Join(runErr, renderErr)
	}
	if spec.Configure != nil {
		spec.Configure(cmd)
	}
	return cmd, nil
}

// Interface values can contain nil functions or pointers; reject those too.
func nilRunner(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Func, reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan:
		return v.IsNil()
	default:
		return false
	}
}

// Factory must construct a fresh tree, never return a cached Cobra command.
type Factory func() (*cobra.Command, error)

func mount(root *cobra.Command, factories []Factory) error {
	if len(factories) == 0 {
		return fmt.Errorf("authoring requires implemented command factories")
	}
	for _, factory := range factories {
		if factory == nil {
			return fmt.Errorf("nil authoring factory")
		}
		cmd, err := factory()
		if err != nil {
			return err
		}
		if cmd == nil || cmd.Parent() != nil {
			return fmt.Errorf("authoring factory must return a fresh unattached command")
		}
		for _, existing := range root.Commands() {
			if existing.Name() == cmd.Name() {
				return fmt.Errorf("duplicate authoring command %q", cmd.Name())
			}
		}
		root.AddCommand(cmd)
	}
	return nil
}

// NewAuthorCommand is internal wiring only: the subtree remains hidden until
// the integrator explicitly opens the complete vertical-slice release gate.
// The installer root retains all existing persistent flags in their positions.
func NewAuthorCommand(factories ...Factory) (*cobra.Command, error) {
	root := &cobra.Command{Use: "author", Short: "Build Agent Plugins packages", Hidden: true,
		Long: "Build Agent Plugins packages.\n\nInherited --scope, --accept-security-risk, and --security-details are installer-only\nand rejected here. Select the package with the positional path; use author doctor\nfor project diagnostics. Use agentplugins add for installation/security policy."}
	if err := mount(root, factories); err != nil {
		return nil, err
	}
	// Keep the explanation on command help as well as the group help, without
	// mutating inherited flag Usage strings owned by the installer root.
	var label func(*cobra.Command)
	label = func(c *cobra.Command) {
		if c.Long == "" {
			c.Long = c.Short
		}
		c.Long += "\n\nInstaller-only inherited flags --scope, --accept-security-risk, and --security-details\nare rejected. Use the positional package path and author doctor for project checks;\nuse agentplugins add for installation/security policy."
		for _, child := range c.Commands() {
			label(child)
		}
	}
	for _, child := range root.Commands() {
		label(child)
	}
	return root, nil
}

// NewPluginKitRoot is a future composition seam, not a replacement for v1 yet.
func NewPluginKitRoot(factories ...Factory) (*cobra.Command, error) {
	root := &cobra.Command{Use: "plugin-kit-ai", Short: "Build Agent Plugins packages", SilenceErrors: true, SilenceUsage: true}
	flags := root.PersistentFlags()
	flags.String("format", "human", "output format: human or json")
	flags.Bool("no-color", false, "disable color output")
	flags.Bool("dry-run", false, "show the plan without changes (supported commands only)")
	flags.String("target", "", "target clients, comma-separated (supported commands only)")
	if err := mount(root, factories); err != nil {
		return nil, err
	}
	return root, nil
}
