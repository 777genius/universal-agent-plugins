// Package commands composes only the implemented standard authoring slice.
package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/readiness"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
)

// Enabled is deliberately build-time only. Release builds retain the v1 surface.
// Internal checkpoint builds set -X <this-package>.Enabled=vertical-slice-v1.
var Enabled string
var Revision = "unversioned"

const ReleaseMode = "release-cli-contract-v1"

func IsEnabled() bool { return Enabled == "vertical-slice-v1" || IsRelease() }
func IsRelease() bool { return Enabled == ReleaseMode }

type RootBuilder func(...authoringcli.Factory) (*cobra.Command, error)
type App struct {
	Projects project.Service
	Revision string
	// PublicContract opts into the private Phase 6 contract; mains retain their existing mode.
	PublicContract bool
	Release        *ReleaseOptions
}

func commandNames() []string {
	return []string{"init", "validate", "inspect", "test", "compat", "capabilities", "doctor", "skills"}
}

type request struct {
	inventory         []string
	targets           []domain.ClientID
	root              string
	disclose, release bool
	template          scaffold.Options
}

// Execute renders once, after Factory.Execute handles every Cobra lifecycle exit.
// All state, including the enclosing root/options, is invocation-owned. Raw Cobra
// errors and buffered help/parser output never enter machine-readable reports.
func (a App) Execute(ctx context.Context, args []string, streams authoringcli.Streams, build RootBuilder) error {
	if a.PublicContract {
		return a.executePublic(ctx, args, streams, build)
	}
	var captured *report.Report
	var human bytes.Buffer
	factory := authoringcli.Factory(func() (*cobra.Command, error) {
		factories := make([]authoringcli.Factory, 0, 4)
		for _, name := range commandNames() {
			factories = append(factories, func() (*cobra.Command, error) { return a.command(name, func(r report.Report) { captured = &r }) })
		}
		root, err := build(factories...)
		if err != nil {
			return nil, err
		}
		// Cobra otherwise treats unknown children of a non-runnable group as
		// successful help, including requests for services that have not landed.
		for _, group := range append([]*cobra.Command{root}, root.Commands()...) {
			if !group.Runnable() {
				group.Args = cobra.NoArgs
				group.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
			}
		}
		root.CompletionOptions.DisableDefaultCmd = true
		for _, group := range root.Commands() {
			if group.Name() == "author" {
				group.Long = "Build Agent Plugins packages and inspect static compatibility and toolchain evidence."
				for _, child := range group.Commands() {
					child.Long = child.Short + "\n\nInstaller-only inherited flags are rejected. Use agentplugins add for installation policy."
				}
			}
		}
		return root, nil
	})
	err := factory.Execute(ctx, args, authoringcli.Streams{In: streams.In, Out: &human, Err: io.Discard})
	if captured == nil {
		r := report.New(selectedCommand(args), a.Revision)
		if err != nil {
			code, action := failure(err, "arguments")
			r.AddError(code, action)
		}
		captured = &r
	}
	if wantsJSON(args) {
		if e := json.NewEncoder(streams.Out).Encode(captured); e != nil {
			return exitx.Wrap(errors.New("authoring output failed"), 1)
		}
	} else if err == nil && human.Len() > 0 {
		if _, e := io.Copy(streams.Out, &human); e != nil {
			return exitx.Wrap(errors.New("authoring output failed"), 1)
		}
	} else {
		if _, e := fmt.Fprintf(streams.Out, "%s: readiness %s; conformance %s; host %s; compatibility %s; toolchain %s; runtime %s\n", captured.Command, captured.Readiness.Status, captured.Conformance.Status, captured.HostSafety.Status, captured.Compatibility.Status, captured.Toolchain.Status, captured.Runtime.Status); e != nil {
			return exitx.Wrap(errors.New("authoring output failed"), 1)
		}
		for _, f := range captured.Findings {
			if _, e := fmt.Fprintf(streams.Out, "%s: %s (%s)\n", f.Severity, f.Code, f.Layer); e != nil {
				return exitx.Wrap(errors.New("authoring output failed"), 1)
			}
		}
		for _, component := range captured.Components {
			if _, e := fmt.Fprintf(streams.Out, "%s %s: %s; requirements %s\n", component.Type, component.ID, component.Status, strings.Join(component.Requirements, ", ")); e != nil {
				return exitx.Wrap(errors.New("authoring output failed"), 1)
			}
		}
		if captured.Capabilities != nil {
			if _, e := fmt.Fprintf(streams.Out, "commands: %s\n", strings.Join(captured.Capabilities.Commands, ", ")); e != nil {
				return exitx.Wrap(e, 1)
			}
			for _, s := range captured.Capabilities.Schemas {
				if _, e := fmt.Fprintf(streams.Out, "schema: %s; %s\n", s.ID, s.Digest); e != nil {
					return exitx.Wrap(e, 1)
				}
			}
			for _, p := range captured.Capabilities.Profiles {
				if _, e := fmt.Fprintf(streams.Out, "profile: %s; revision %s; %s\n", p.ID, p.Revision, p.Digest); e != nil {
					return exitx.Wrap(e, 1)
				}
			}
			for _, c := range captured.Capabilities.Clients {
				if _, e := fmt.Fprintf(streams.Out, "client: %s; activation: %s\n", c.ClientID, c.ActivationMode); e != nil {
					return exitx.Wrap(e, 1)
				}
			}
		}
		for _, c := range captured.Clients {
			if _, e := fmt.Fprintf(streams.Out, "target %s: static support only; %s\n", c.ClientID, strings.Join(c.Limitations, ", ")); e != nil {
				return exitx.Wrap(e, 1)
			}
			for _, v := range c.Components {
				if _, e := fmt.Fprintf(streams.Out, "  %s %d: %s\n", v.Kind, v.Index, v.Support); e != nil {
					return exitx.Wrap(e, 1)
				}
			}
		}
		for _, c := range captured.DoctorChecks {
			if _, e := fmt.Fprintf(streams.Out, "%s: %s; %s\n", c.ID, c.Status, c.Action); e != nil {
				return exitx.Wrap(e, 1)
			}
		}
		if captured.Error != nil {
			if _, e := fmt.Fprintln(streams.Out, captured.Error.Action); e != nil {
				return exitx.Wrap(errors.New("authoring output failed"), 1)
			}
		}
	}
	if err != nil {
		if captured.Error != nil && captured.Error.Code == "private_cleanup_failed" {
			return exitx.Wrap(errors.New("authoring cleanup failed; see report for recovery"), 1)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return exitx.Wrap(errors.New("authoring canceled"), 1)
		}
		code := 1
		if captured.Error != nil && captured.Error.Code == "arguments_invalid" {
			code = 2
		}
		return exitx.Wrap(errors.New("authoring operation failed; see report"), code)
	}
	return nil
}
func (a App) command(name string, capture func(report.Report)) (*cobra.Command, error) {
	if name == "skills" {
		return a.skillsCommand(capture)
	}
	use, args := name+" <path>", cobra.ExactArgs(1)
	if a.PublicContract && name != "init" {
		use, args = name+" [package-path]", cobra.MaximumNArgs(1)
	}
	if name == "capabilities" {
		use, args = name, cobra.NoArgs
	}
	return authoringcli.NewCommand(authoringcli.Spec[request, report.Report]{Use: use, Short: summary(name), Args: args,
		Support: authoringcli.Support{Format: true, NoColor: true, Target: name == "compat" || name == "inspect"},
		Configure: func(c *cobra.Command) {
			f := c.Flags()
			if name == "capabilities" {
				return
			}
			f.Bool("include-root", false, "disclose the explicitly selected root in this report")
			if name == "init" {
				for _, v := range []struct{ name, help string }{
					{"template", "skill, mcp-remote, mcp-stdio, or hybrid"}, {"name", "exact plugin identity"}, {"description", "explicit package description"},
					{"skill-name", "explicit Skill identity"}, {"mcp", "hybrid MCP choice: mcp-remote or mcp-stdio"}, {"runtime", "stdio template runtime: node"},
					{"url", "explicit remote MCP URL"}, {"author-name", "optional explicit author"}, {"license", "optional MIT or ISC license"},
					{"copyright-holder", "explicit license holder"}, {"copyright-year", "explicit four-digit license year"},
				} {
					if a.PublicContract && v.name == "mcp" {
						v.name = "mcp-template"
					}
					f.String(v.name, "", v.help)
				}
			} else {
				f.Bool("release-policy", false, "also evaluate bounded release hygiene (not publication approval)")
			}
		},
		Decode: func(c *cobra.Command, opts authoringcli.Options, args []string) (request, error) {
			if name == "capabilities" {
				return request{inventory: implementedLeaves(c.Root())}, nil
			}
			get := func(n string) string { v, _ := c.Flags().GetString(n); return v }
			disclose, _ := c.Flags().GetBool("include-root")
			root := ""
			if len(args) > 0 {
				root = args[0]
			}
			req := request{root: root, disclose: disclose}
			if a.PublicContract {
				var err error
				req.root, err = skills.ExactRoot(filepath.FromSlash(root))
				if err != nil {
					return request{}, err
				}
			}
			if name == "compat" || name == "inspect" && (opts.Target != "" || c.Flags().Changed("target")) {
				ids, err := readiness.Targets(opts.Target)
				if err != nil {
					return request{}, err
				}
				req.targets = ids
			}
			if name == "init" {
				mcpFlag := "mcp"
				if a.PublicContract {
					mcpFlag = "mcp-template"
				}
				req.template = scaffold.Options{Template: scaffold.Template(get("template")), Name: get("name"), Description: get("description"), SkillName: get("skill-name"), MCPChoice: scaffold.Template(get(mcpFlag)), Runtime: get("runtime"), RemoteURL: get("url"), AuthorName: get("author-name"), License: get("license"), CopyrightHolder: get("copyright-holder"), CopyrightYear: get("copyright-year")}
				if a.PublicContract {
					if !c.Flags().Changed("name") && !strings.ContainsAny(root, `/\`) && root != "." && root != ".." {
						req.template.Name = root
					}
					if !c.Flags().Changed("description") {
						req.template.Description = scaffold.DefaultDescription(req.template.Template)
					}
					if _, err := scaffold.BuildPlan(req.template); err != nil {
						return request{}, publicInputError(err)
					}
				}
			} else {
				req.release, _ = c.Flags().GetBool("release-policy")
			}
			return req, nil
		},
		Runner: authoringcli.RunnerFunc[request, report.Report](func(ctx context.Context, req request) (report.Report, error) {
			if name == "capabilities" {
				r := report.New(name, a.Revision)
				names := commandNames()
				if a.PublicContract {
					names = req.inventory
				}
				c, err := readiness.Engine(names)
				r.Capabilities = c
				if err != nil {
					r.AddError("capabilities_unavailable", "Check the embedded engine metadata.")
				}
				return r, err
			}
			if name == "init" {
				return a.init(ctx, req)
			}
			p, e := a.Projects.Read(ctx, req.root)
			r := report.Build(name, a.Revision, p, req.release)
			if req.disclose {
				r.Root = req.root
			}
			if e != nil {
				code, action := failure(e, "read")
				r.AddError(code, action)
				return r, e
			}
			if len(req.targets) > 0 {
				clients, err := readiness.Compatibility(p, req.targets)
				if err != nil {
					r.AddError("compatibility_unavailable", "Select explicit supported clients.")
					return r, err
				}
				r.AddCompatibility(clients)
			}
			if name == "doctor" {
				r.AddDoctor(readiness.Doctor(p))
			}
			if !r.Successful() {
				return r, errors.New("authoring checks incomplete or failed")
			}
			return r, nil
		}),
		Render: func(_ authoringcli.Streams, _ authoringcli.Options, r report.Report, _ error) error {
			capture(r)
			return nil
		},
	})
}
func (a App) init(ctx context.Context, req request) (report.Report, error) {
	r := report.New("init", a.Revision)
	r.Mode = "local_mutation"
	if req.disclose {
		r.Root = req.root
	}
	fail := func(e error, phase string) (report.Report, error) {
		code, action := failure(e, phase)
		r.AddError(code, action)
		return r, e
	}
	plan, e := scaffold.BuildPlan(req.template)
	if e != nil {
		return fail(e, "template")
	}
	// Preserve the caller's path spelling for scaffold's own exact identity checks.
	// Relative inputs are prefixed, never lexically cleaned before validation.
	destination := req.root
	if !filepath.IsAbs(destination) {
		cwd, e := os.Getwd()
		if e != nil {
			return fail(e, "destination")
		}
		destination = cwd + string(os.PathSeparator) + destination
	}
	result, e := scaffold.Apply(ctx, plan, scaffold.ApplyOptions{Destination: destination, Validate: func(ctx context.Context, stage string) error {
		p, e := a.Projects.Read(ctx, stage)
		r = report.Build("init", a.Revision, p, false)
		r.Mode = "local_mutation"
		if req.disclose {
			r.Root = req.root
		}
		if e != nil {
			return e
		}
		// Exactly the same public validation policy, including host/completeness.
		if !r.Successful() {
			return errors.New("generated package validation failed")
		}
		return nil
	}})
	r.Committed = result.Committed
	if result.Committed {
		for _, f := range plan.Files() {
			r.Paths = append(r.Paths, f.Path)
		}
	}
	if e != nil {
		return fail(e, "apply")
	}
	return r, nil
}
func summary(name string) string {
	switch name {
	case "init":
		return "Create an offline standard package in an absent destination"
	case "compat":
		return "Evaluate explicit --target clients using static adapter support; no installed clients required"
	case "capabilities":
		return "Show embedded schemas, profiles, live client metadata and implemented commands"
	case "doctor":
		return "Inspect captured native files and executable metadata; no processes or network"
	case "inspect":
		return "Inspect captured components and unresolved runtime requirements"
	case "test":
		return "Check configuration, hygiene, Skills and MCP statically; runtime is not evaluated"
	default:
		return "Validate exact-root standard configuration and authoring readiness"
	}
}
func failure(err error, phase string) (string, string) {
	var cleanup *scaffold.CleanupError
	if errors.As(err, &cleanup) {
		return "private_cleanup_failed", "Private scaffold cleanup failed; check committed status and inspect owned staging before retrying; preserve unrelated replacements."
	}
	var reader *packageview.Error
	if errors.As(err, &reader) && reader.CleanupFailed {
		return "private_cleanup_failed", "Private reader cleanup failed; inspect owned scratch before retrying."
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "canceled", "Retry the canceled operation after checking any committed result."
	}
	if errors.As(err, &reader) {
		return reader.Code, "Check the exact package root, local filesystem support and private scratch access."
	}
	var identity *scaffold.IdentityError
	if errors.As(err, &identity) {
		// Suggestions derived from supplied values can also disclose opaque input.
		switch identity.Field {
		case "skill name":
			return "skill_identity_invalid", "Choose --skill-name explicitly, for example my-skill (lowercase letters and hyphens)."
		default:
			return "plugin_identity_invalid", "Choose --name explicitly, for example my-plugin; use a standard and portable leaf identity."
		}
	}
	if errors.Is(err, os.ErrExist) {
		return "destination_exists", "Choose an absent destination; init never overwrites an existing directory."
	}
	switch phase {
	case "arguments":
		return "arguments_invalid", "Use an implemented command with one explicit path (capabilities takes none); compat requires distinct comma-separated --target clients. Installer-only and runtime flags are rejected."
	case "template":
		return "template_options_invalid", "Select a template, exact name and description; remote requires --url, stdio requires --runtime node, hybrid requires --mcp; licenses require explicit holder and year."
	case "apply", "destination":
		return "init_failed", "Check the absent, clean destination and existing parent; inspect committed status and owned staging cleanup before retrying."
	default:
		return "input_unavailable", "Check exact-root input availability; no fallback format is accepted."
	}
}

// These scans select output/routing only, never authorize effects. Cobra remains
// authoritative for parsing and rejects all unsupported flags before a runner.
func wantsJSON(args []string) bool {
	for i, a := range args {
		if a == "--" {
			break
		}
		if a == "--format=json" || a == "--format" && i+1 < len(args) && args[i+1] == "json" {
			return true
		}
	}
	return false
}
func selectedCommand(args []string) string {
	for _, a := range args {
		for _, n := range commandNames() {
			if a == n {
				return n
			}
		}
	}
	return "author"
}

// IsAuthorInvocation uses Cobra selection on an invocation-owned, unconfigured
// root. Find observes flag definitions without parsing or executing any command.
// Callers must supply a fresh root, just as they do for Execute.
func IsAuthorInvocation(args []string, root *cobra.Command) bool {
	author := &cobra.Command{Use: "author"}
	root.AddCommand(author)
	selected, _, _ := root.Find(args)
	for c := selected; c != nil; c = c.Parent() {
		if c == author {
			return true
		}
	}
	return false
}
