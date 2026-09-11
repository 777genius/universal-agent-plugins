package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/cli/internal/outputjson"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const operationKey = "authoring.public-operation"

// Tag only factories supplied by this engine. Enclosing installer commands and
// Cobra utilities cannot become author capabilities by having a matching name.
func tagOperations(c *cobra.Command, prefix string) {
	if c.Annotations == nil {
		c.Annotations = map[string]string{}
	}
	c.Annotations[operationKey] = prefix
	for _, child := range c.Commands() {
		tagOperations(child, prefix+"."+child.Name())
	}
}
func implementedLeaves(root *cobra.Command) []string {
	var out []string
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c == nil {
			return
		}
		if !c.Hidden && !c.HasSubCommands() && c.Runnable() && c.Annotations[operationKey] != "" {
			out = append(out, c.Annotations[operationKey])
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(root)
	sort.Strings(out)
	return out
}

type selection struct {
	command                 *cobra.Command
	operation, mode, format string
	help, invalid           bool
	flags                   []string
	values                  map[string][]string
}

// selectPublic observes definitions on the fresh tree without executing a
// parser, decoding a request, or accessing a project. Known flag values consume
// their actual arity. An unknown token freezes routing at the nearest known
// command; its unknown arity can never promote a credential value into a verb.
// Format selection continues after errors, with pflag's final-value semantics.
func selectPublic(root *cobra.Command, args []string) selection {
	s := selection{command: root, operation: "author", mode: "read", format: "human", values: map[string][]string{}}
	positional, helpCommand := false, false
	lookup := func(name string, short bool) *pflag.Flag {
		for c := s.command; c != nil; c = c.Parent() {
			for _, fs := range []*pflag.FlagSet{c.Flags(), c.PersistentFlags()} {
				var f *pflag.Flag
				if short {
					f = fs.ShorthandLookup(name)
				} else {
					f = fs.Lookup(name)
				}
				if f != nil {
					return f
				}
			}
		}
		return nil
	}
	flag := func(f *pflag.Flag, value string) {
		s.values[f.Name] = append(s.values[f.Name], value)
		if f.Name == "format" {
			s.format = value
		}
		if f.Name == "help" {
			v, err := strconv.ParseBool(value)
			s.help = v
			if err != nil {
				s.invalid = true
			}
		}
		switch f.Name {
		case "scope", "accept-security-risk", "security-details", "dry-run":
			s.invalid = true
		}
	}
	for i := 0; i < len(args); i++ {
		start := i
		token := args[i]
		if token == "--" {
			if i+1 < len(args) && s.command.HasSubCommands() {
				s.invalid = true
			}
			break
		}
		if strings.HasPrefix(token, "--") {
			name, value, equal := strings.Cut(token[2:], "=")
			f := lookup(name, false)
			if f == nil {
				s.invalid = true
				positional = true
				continue
			}
			if !equal {
				if f.NoOptDefVal != "" {
					value = f.NoOptDefVal
				} else if i+1 < len(args) {
					i++
					value = args[i]
				} else {
					s.values[f.Name] = append(s.values[f.Name], "")
					s.invalid = true
					continue
				}
			}
			flag(f, value)
			s.flags = append(s.flags, args[start:i+1]...)
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			cluster := token[1:]
			for len(cluster) > 0 {
				f := lookup(cluster[:1], true)
				cluster = cluster[1:]
				if f == nil {
					s.invalid = true
					positional = true
					continue
				}
				value := f.NoOptDefVal
				if strings.HasPrefix(cluster, "=") {
					value = cluster[1:]
					cluster = ""
				} else if f.NoOptDefVal == "" {
					if cluster != "" {
						value = cluster
						cluster = ""
					} else if i+1 < len(args) {
						i++
						value = args[i]
					} else {
						s.invalid = true
						break
					}
				}
				flag(f, value)
			}
			s.flags = append(s.flags, args[start:i+1]...)
			continue
		}
		if positional {
			continue
		}
		if token == "help" && s.command.HasSubCommands() {
			helpCommand = true
			continue
		}
		var next *cobra.Command
		for _, c := range s.command.Commands() {
			if (c.Name() == token || c.Annotations[operationKey] == "" && c.HasAlias(token)) && (!c.Hidden || c.Name() == "author" || c.Annotations[authoringcli.RejectionKey] != "") {
				next = c
				break
			}
		}
		if next == nil {
			positional = true
			if s.command.HasSubCommands() || helpCommand {
				s.invalid = true
			}
			continue
		}
		s.command = next
		if id := next.Annotations[operationKey]; id != "" {
			s.operation = id
		}
	}
	s.help = s.help || helpCommand
	if id := s.command.Annotations[operationKey]; id != "" {
		s.operation = id
	} else if s.command.Annotations[authoringcli.RejectionKey] == "" && !isCompletion(s.command) {
		s.invalid = true
	}
	if s.operation == "author.init" || s.operation == "author.skills.init" {
		s.mode = "local_mutation"
	}
	if s.format != "human" && s.format != "json" {
		s.invalid = true
	}
	return s
}

type inputError struct{ code, action string }

func (e *inputError) Error() string { return e.action }
func publicInputError(err error) error {
	code, action := failure(err, "template")
	if code == "template_options_invalid" {
		action = "Choose --template skill, mcp-remote, mcp-stdio or hybrid. Remote requires --url; stdio requires --runtime node; hybrid requires --mcp-template. Use --name for a path destination. Licenses require explicit holder and year."
	}
	var identity *scaffold.IdentityError
	if errors.As(err, &identity) {
		if suggestion := report.DisplayIdentity(identity.Suggestion); suggestion != "" && report.DisplayIdentity(identity.Value) != "" {
			action += " Suggested identity: " + suggestion + "; supply it explicitly."
		}
	}
	return &inputError{code, action}
}

func (a App) executePublic(ctx context.Context, args []string, streams authoringcli.Streams, build RootBuilder) error {
	var captured *report.Report
	var utility bytes.Buffer
	utilitySelected := false
	var selected = selection{operation: "author", mode: "read", format: "human"}
	var surface []string
	factory := authoringcli.Factory(func() (*cobra.Command, error) {
		factories := make([]authoringcli.Factory, 0, len(commandNames()))
		for _, name := range commandNames() {
			factories = append(factories, func() (*cobra.Command, error) {
				c, err := a.command(name, func(r report.Report) { captured = &r })
				if err == nil {
					tagOperations(c, "author."+name)
				}
				return c, err
			})
		}
		if a.Release != nil {
			factories = append(factories, func() (*cobra.Command, error) { return NewVersionCommand() })
		}
		root, err := build(factories...)
		if err != nil {
			return nil, err
		}
		if root == nil {
			return nil, errors.New("missing authoring root")
		}
		root.CompletionOptions.DisableDefaultCmd = a.Release == nil
		if a.Release != nil {
			root.SetOut(&utility)
			authoringcli.PrepareReleaseUtilities(root, false)
		}
		if root.Name() == "plugin-kit-ai" {
			root.Annotations = map[string]string{operationKey: "author"}
		}
		var prepare func(*cobra.Command)
		prepare = func(c *cobra.Command) {
			if c.Name() == "author" {
				c.Annotations = map[string]string{operationKey: "author"}
			}
			c.InitDefaultHelpFlag()
			if c.Annotations[operationKey] != "" && c.HasSubCommands() {
				c.Args = cobra.NoArgs
				c.RunE = func(c *cobra.Command, _ []string) error {
					_, err := authoringcli.AdaptFlags(c, authoringcli.Support{Format: true, NoColor: true})
					return err
				}
			}
			for _, child := range c.Commands() {
				prepare(child)
			}
		}
		prepare(root)
		surface = implementedLeaves(root)
		selected = selectPublic(root, args)
		if a.Release != nil {
			if a.Release.Reject != nil {
				if e := a.Release.Reject(Invocation{Command: selected.command, Values: selected.values}); e != nil {
					if (Invocation{Values: selected.values}).Bool("json") && len(selected.values["format"]) == 0 {
						selected.format = "json"
					}
					return nil, &inputError{"v1_operation_unavailable", e.Error()}
				}
			}
			utilitySelected = isCompletion(selected.command)
			if len(args) > 0 && (args[0] == "__complete" || args[0] == "__completeNoDesc") {
				utilitySelected = true
				selected.invalid = false
			}
			if utilitySelected {
				authoringcli.PrepareReleaseUtilities(root, true)
			}
			a.guardReleaseCompletion(root, build)
		}
		if selected.invalid {
			return nil, &inputError{"arguments_invalid", publicArguments}
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Help verbs and --help share a pre-run closure. Validate values with the
		// actual selected pflag set, then stop before Cobra's group help routing.
		if selected.help {
			if err := selected.command.ParseFlags(selected.flags); err != nil {
				return nil, &inputError{"arguments_invalid", publicArguments}
			}
			if _, err := authoringcli.AdaptFlags(selected.command, authoringcli.Support{Format: true, NoColor: true, Target: selected.operation == "author.inspect" || selected.operation == "author.compat"}); err != nil {
				return nil, &inputError{"arguments_invalid", publicArguments}
			}
			return nil, errPublicHelp
		}

		return root, nil
	})
	err := factory.Execute(ctx, args, authoringcli.Streams{In: streams.In, Out: &utility, Err: io.Discard})
	if errors.Is(err, errPublicHelp) {
		err = nil
	}
	if utilitySelected && err == nil && !selected.help && selected.format == "human" {
		if _, e := streams.Out.Write(utility.Bytes()); e != nil {
			return exitx.Wrap(errors.New("authoring output failed"), 1)
		}
		return nil
	}
	attempted := captured != nil
	code := 0
	if err != nil {
		code = exitx.Code(err)
	}
	if captured == nil {
		r := report.New(selected.operation, a.Revision)
		if err != nil {
			var input *inputError
			errorCode, action := failure(err, "arguments")
			if errors.As(err, &input) {
				errorCode, action = input.code, input.action
			}
			if errorCode == "arguments_invalid" {
				action = publicArguments
			}
			var pathError *os.PathError
			if errorCode != "canceled" && !errors.As(err, &pathError) {
				// An existing exitx annotation wins over the syntax fallback.
				code = exitx.Code(errors.Join(err, exitx.Wrap(err, 2)))
			}
			r.AddOperationError(errorCode, action)
		}
		captured = &r
	}
	// Cleanup is authoritative even after a commit or an annotated/canceled error.
	if captured.Error != nil && captured.Error.Code == "private_cleanup_failed" {
		code = 1
	}
	var commands []string
	if !attempted && err == nil || selected.operation == "author.capabilities" {
		commands = surface
	}
	p := captured.PublicResult(selected.operation, selected.mode, attempted, commands)
	versionResult := a.Release != nil && selected.operation == "author.version" && !selected.help && err == nil
	if !attempted && err == nil && !versionResult && (!utilitySelected || selected.help) {
		p.Help = publicHelp(selected)
	}
	result := outputjson.Success
	if err != nil {
		result = outputjson.Failure
	}
	var outputErr error
	if selected.format == "json" {
		var payload any = p
		if utilitySelected && err == nil && !selected.help {
			payload = struct {
				report.Public
				Script string `json:"script"`
			}{p, utility.String()}
		}
		if versionResult {
			payload = versionPayload{Public: p, Product: a.Release.Product, ProductVersion: a.Release.Version}
		}
		outputErr = outputjson.Write(streams.Out, selected.operation, result, payload)
	} else {
		if versionResult {
			_, outputErr = fmt.Fprintf(streams.Out, "%s %s\nauthoring engine %s; revision %s\n", a.Release.Product, a.Release.Version, p.EngineVersion, p.Revision)
		} else {
			outputErr = writePublicHuman(streams.Out, p, result)
		}
	}
	if outputErr != nil {
		return exitx.Wrap(errors.New("authoring output failed"), 1)
	}
	if err != nil {
		return exitx.Wrap(errors.New("authoring operation failed; see report"), code)
	}
	return nil
}

var errPublicHelp = errors.New("authoring command surface requested")

const publicArguments = "Use an implemented authoring command; read paths default to the exact current directory. Use --format human or json. Compat requires explicit comma-separated --target clients. Inherited --scope, --accept-security-risk and --security-details are installer-only; use agentplugins add for installation policy. Unsupported --dry-run and runtime flags are rejected."

func writePublicHuman(w io.Writer, p report.Public, result string) error {
	// Buffer only trusted projected data and write once, preserving output failure
	// precedence without a second result attempt or raw Cobra/parser text.
	var b strings.Builder
	if p.Help != nil {
		fmt.Fprintf(&b, "Usage: %s\nFlags: %s\n%s\n", p.Help.Use, strings.Join(p.Help.Flags, ", "), p.Help.Guidance)
	}
	fmt.Fprintf(&b, "%s: %s; readiness %s; conformance %s; runtime %s\n", p.Command, result, p.Readiness.Status, p.Conformance.Status, p.Runtime.Status)
	if p.Inspection != nil {
		fmt.Fprintf(&b, "package: %s; version: %s; schema: %s\n", p.Inspection.Name, p.Inspection.Version, p.Inspection.Schema)
		for _, c := range p.Inspection.Components {
			fmt.Fprintf(&b, "%s %s%s; executable: %s (%s)\n", c.Type, c.Name, c.Namespace, c.Executable, c.ExecutableKind)
		}
	}
	for _, f := range p.Findings {
		fmt.Fprintf(&b, "%s: %s (%s)\n", f.Severity, f.Code, f.Layer)
	}
	for _, client := range p.Clients {
		fmt.Fprintf(&b, "target %s: static support only; %s\n", client.ClientID, strings.Join(client.Limitations, ", "))
		for _, c := range client.Components {
			fmt.Fprintf(&b, "  %s %d: %s\n", c.Kind, c.Index, c.Support)
		}
	}
	for _, c := range p.DoctorChecks {
		fmt.Fprintf(&b, "%s: %s; %s\n", c.ID, c.Status, c.Action)
	}
	if len(p.Surface) > 0 {
		fmt.Fprintf(&b, "commands: %s\n", strings.Join(p.Surface, ", "))
	}
	for _, action := range p.NextActions {
		fmt.Fprintln(&b, action.Message)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// Help lists definitions rather than flag values; never serialize mutable pflag
// values, which can contain credentials even when help bypasses a runner.
func publicHelp(s selection) *report.CommandHelp {
	// Only trusted tree definitions supply the route and argument placeholders;
	// argv and mutable flag values never contribute to usage text.
	use := s.command.CommandPath() + strings.TrimPrefix(s.command.Use, s.command.Name())
	h := &report.CommandHelp{Use: use, Flags: []string{}, Guidance: publicArguments}
	if s.operation == "author" && (s.command.Name() == "author" || s.command.Name() == "plugin-kit-ai") {
		h.Use += " <command>"
	}
	seen := map[string]bool{}
	for c := s.command; c != nil; c = c.Parent() {
		for _, fs := range []*pflag.FlagSet{c.Flags(), c.PersistentFlags()} {
			fs.VisitAll(func(f *pflag.Flag) {
				if f.Hidden || seen[f.Name] {
					return
				}
				seen[f.Name] = true
				switch f.Name {
				case "scope", "accept-security-risk", "security-details":
					return
				}
				label := "--" + f.Name
				if f.NoOptDefVal == "" {
					label += " <value>"
				}
				h.Flags = append(h.Flags, label)
			})
		}
	}
	sort.Strings(h.Flags)
	return h
}
