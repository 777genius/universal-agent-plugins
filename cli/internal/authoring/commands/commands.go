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
	"time"

	authorbootstrap "github.com/777genius/plugin-kit-ai/cli/internal/authoring/bootstrap"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/mcpruntime"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
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
	// ClientRegistry is the set of client adapters this binary reports
	// compatibility against. The main that builds the App decides it; without
	// one the compat report has nothing to ask about the clients it names.
	ClientRegistry *clients.Registry
	// PublicContract opts into the private Phase 6 contract; mains retain their existing mode.
	PublicContract  bool
	Release         *ReleaseOptions
	MCPRuntime      bool
	Bootstrap       bool
	BootstrapRunner authorbootstrap.Runner
	JSONMaintenance bool
}

func (a App) commandNames() []string {
	names := []string{"init", "validate", "inspect", "test", "compat", "capabilities", "doctor", "skills"}
	if a.JSONMaintenance {
		names = append(names, "normalize", "import")
	}
	if a.MCPRuntime {
		names = append(names, "dev")
	}
	if a.Bootstrap {
		names = append(names, "bootstrap")
	}
	return names
}

type request struct {
	inventory         []string
	targets           []domain.ClientID
	root              string
	disclose, release bool
	template          scaffold.Options
	runtime           string
	server, tool      string
	fixture           string
	allowNetwork      bool
	once              bool
	deadline          time.Duration
	cycleOutput       func(report.Report, error) error
	runMCP            func(context.Context, mcpruntime.Options) (mcpruntime.Evidence, error)
	dryRun            bool
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
		for _, name := range a.commandNames() {
			factories = append(factories, func() (*cobra.Command, error) {
				return a.command(name, func(r report.Report) { captured = &r }, func(r report.Report, _ error) error {
					return writePrivateHuman(streams.Out, r)
				})
			})
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
		r := report.New(selectedCommand(args, a.commandNames()), a.Revision)
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
		if e := writePrivateDetailed(streams.Out, *captured); e != nil {
			return e
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
func (a App) command(name string, capture func(report.Report), cycleOutput func(report.Report, error) error) (*cobra.Command, error) {
	if name == "skills" {
		return a.skillsCommand(capture)
	}
	if name == "normalize" {
		return a.normalizeCommand(capture)
	}
	if name == "import" {
		return a.importCommand(capture)
	}
	use, args := name+" <path>", cobra.ExactArgs(1)
	if a.PublicContract && name != "init" {
		use, args = name+" [package-path]", cobra.MaximumNArgs(1)
	}
	if a.PublicContract && name == "bootstrap" {
		use = "bootstrap [path]"
	}
	if name == "capabilities" {
		use, args = name, cobra.NoArgs
	}
	return authoringcli.NewCommand(authoringcli.Spec[request, report.Report]{Use: use, Short: summary(name), Args: args,
		Support: authoringcli.Support{Format: true, NoColor: true, DryRun: name == "bootstrap", Target: name == "compat" || name == "inspect"},
		Configure: func(c *cobra.Command) { a.configureCommand(name, c) },
		Decode: func(c *cobra.Command, opts authoringcli.Options, args []string) (request, error) {
			return a.decodeRequest(name, c, opts, args, cycleOutput)
		},
		Runner: authoringcli.RunnerFunc[request, report.Report](func(ctx context.Context, req request) (report.Report, error) {
			return a.runCommand(ctx, name, req)
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
	result, e := scaffold.Apply(ctx, plan, scaffold.ApplyOptions{Destination: destination, Validate: func(ctx context.Context, stage string, dir *os.Root) error {
		p, e := a.Projects.ReadGeneratedStaging(ctx, stage, dir)
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
		return "Check statically by default, or run one explicit MCP server with --runtime=mcp"
	case "dev":
		return "Rerun one explicit MCP server when the selected package changes"
	case "bootstrap":
		return "Install locked dependencies for a recognized generated standard template"
	case "normalize":
		return "Plan or atomically normalize one standard JSON document"
	case "import":
		return "Import one explicit native configuration into an absent standard package"
	default:
		return "Validate exact-root standard configuration and authoring readiness"
	}
}

func bootstrapFailure(err error) (string, string) {
	var e *authorbootstrap.Error
	if !errors.As(err, &e) {
		return "bootstrap_failed", "Retry in a disposable generated package after checking the reported plan."
	}
	switch e.Code {
	case "bootstrap_lock_required":
		return e.Code, "Use the checked-in package-lock.json emitted with the generated Node template."
	case "bootstrap_layout_ambiguous":
		return e.Code, "Remove competing runtime or package-manager files; bootstrap requires one unambiguous generated layout."
	case "bootstrap_destination_exists":
		return e.Code, "Remove the existing node_modules directory explicitly before bootstrapping again."
	case "bootstrap_template_unrecognized":
		return e.Code, "Bootstrap supports only the exact tool-generated standard Node stdio template and its checked-in lockfile."
	case "bootstrap_platform_unsupported":
		return e.Code, "Mutating bootstrap apply requires Linux handle-bound execution; use --dry-run for a non-mutating plan on this platform."
	case "bootstrap_process_failed":
		return e.Code, "The locked npm ci process failed; source and lockfiles were left unchanged and partial staging was removed."
	default:
		return e.Code, "Bootstrap could not complete its bounded project-local operation; inspect the disposable project before retrying."
	}
}

// writePrivateHuman emits only allowlisted report fields. Continuous dev uses
// it directly so completed cycles are visible without waiting for cancellation.
func writePrivateHuman(w io.Writer, r report.Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: readiness %s; conformance %s; host %s; compatibility %s; toolchain %s; runtime %s\n", r.Command, r.Readiness.Status, r.Conformance.Status, r.HostSafety.Status, r.Compatibility.Status, r.Toolchain.Status, r.Runtime.Status)
	if document := r.JSONDocument; document != nil {
		fmt.Fprintf(&b, "json document: %s; changed %t; before %s; after %s\n", document.Path, document.Changed, document.BeforeSHA256, document.AfterSHA256)
	}
	if imported := r.NativeImport; imported != nil {
		fmt.Fprintf(&b, "native import: client %s; source %s; safe servers %d; skipped servers %d; unsupported top-level fields %d\n", imported.Client, imported.SourceSHA256, imported.SafeServers, len(imported.SkippedServers), len(imported.UnsupportedTopLevel))
	}
	if bootstrap := r.Bootstrap; bootstrap != nil {
		fmt.Fprintf(&b, "bootstrap: runtime %s; manager %s; argv %s; planned %t\n", bootstrap.Runtime, bootstrap.Manager, strings.Join(bootstrap.Argv, " "), bootstrap.Planned)
	}
	if r.Committed {
		fmt.Fprintln(&b, "committed: true")
	}
	for _, path := range r.Paths {
		if filepath.IsLocal(path) {
			fmt.Fprintf(&b, "affected path: %s\n", filepath.ToSlash(path))
		}
	}
	if runtime := r.RuntimeDetail; runtime != nil {
		fmt.Fprintf(&b, "mcp runtime: transport %s; initialize %s; list tools %s; tool call %s; tools %d; cleanup %s\n", runtime.Transport, runtime.Initialize, runtime.ListTools, runtime.ToolCall, runtime.ToolCount, runtime.Cleanup)
	}
	for _, finding := range r.Findings {
		fmt.Fprintf(&b, "%s: %s (%s)\n", finding.Severity, finding.Code, finding.Layer)
	}
	if r.Error != nil {
		fmt.Fprintln(&b, r.Error.Action)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func runtimeErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var runtimeErr *mcpruntime.Error
	if errors.As(err, &runtimeErr) {
		return runtimeErr.Code
	}
	return "runtime_failed"
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
func selectedCommand(args, supported []string) string {
	allowed := make(map[string]struct{}, len(supported))
	for _, name := range supported {
		allowed[name] = struct{}{}
	}
	for _, a := range args {
		for _, n := range []string{"init", "validate", "inspect", "test", "compat", "capabilities", "doctor", "skills", "dev", "bootstrap", "normalize", "import"} {
			if a == n {
				if _, ok := allowed[n]; !ok {
					return "author"
				}
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
