package commands

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/readiness"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/skills"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
)

// configureCommand owns only Cobra's authoring flag surface.
func (a App) configureCommand(name string, c *cobra.Command) {
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
		if a.MCPRuntime && (name == "test" || name == "dev") {
			if name == "test" {
				f.String("runtime", "", "explicit runtime mode: mcp")
			}
			f.String("server", "", "exact MCP server name")
			f.String("tool", "", "exact tool name; requires --fixture")
			f.String("fixture", "", "package-relative JSON object; requires --tool")
			f.Bool("allow-network", false, "allow the selected streamable HTTP endpoint")
			f.Duration("deadline", 10*time.Second, "per-cycle runtime deadline (maximum 1m)")
			if name == "dev" {
				f.Bool("once", false, "run one bounded development cycle")
			}
		}
	}
}

// decodeRequest translates Cobra flags into invocation-owned input.
func (a App) decodeRequest(name string, c *cobra.Command, opts authoringcli.Options, args []string, cycleOutput func(report.Report, error) error) (request, error) {
	if name == "capabilities" {
		return request{inventory: implementedLeaves(c.Root())}, nil
	}
	disclose, _ := c.Flags().GetBool("include-root")
	root := ""
	if len(args) > 0 {
		root = args[0]
	}
	req := request{root: root, disclose: disclose, cycleOutput: cycleOutput}
	if name == "bootstrap" {
		req.dryRun = opts.DryRun
	}
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
		if err := a.decodeInitRequest(c, root, &req); err != nil {
			return request{}, err
		}
	} else if err := a.decodeNonInitRequest(name, c, opts, &req); err != nil {
		return request{}, err
	}
	return req, nil
}

// decodeInitRequest maps explicitly provided init flags into a scaffold plan
// and preserves public-contract validation before any project effects.
func (a App) decodeInitRequest(c *cobra.Command, root string, req *request) error {
	get := func(n string) string { v, _ := c.Flags().GetString(n); return v }
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
			return publicInputError(err)
		}
	}
	return nil
}

// decodeNonInitRequest enforces non-init command options and rejects invalid
// runtime combinations before the authoring runner is invoked.
func (a App) decodeNonInitRequest(name string, c *cobra.Command, opts authoringcli.Options, req *request) error {
	get := func(n string) string { v, _ := c.Flags().GetString(n); return v }
	req.release, _ = c.Flags().GetBool("release-policy")
	if a.MCPRuntime && (name == "test" || name == "dev") {
		req.server, req.tool, req.fixture = get("server"), get("tool"), get("fixture")
		req.allowNetwork, _ = c.Flags().GetBool("allow-network")
		req.deadline, _ = c.Flags().GetDuration("deadline")
		if name == "test" {
			req.runtime = get("runtime")
		} else {
			req.runtime = "mcp"
			req.once, _ = c.Flags().GetBool("once")
		}
		if invalidRuntimeShape(*req) || invalidRuntimeSelection(name, c, opts, *req) {
			return &inputError{"runtime_arguments_invalid", "Use test --runtime=mcp --server <name>, or dev --server <name>. --tool and --fixture are required together; deadlines are positive and at most 1m. Continuous dev requires human output; use --once with JSON."}
		}
	}
	return nil
}

// invalidRuntimeShape checks bounded, paired MCP arguments independent of
// whether the caller supplied an option explicitly.
func invalidRuntimeShape(req request) bool {
	return (req.tool == "") != (req.fixture == "") ||
		req.deadline <= 0 || req.deadline > time.Minute ||
		len(req.server) > 128 || len(req.tool) > 128 || len(req.fixture) > 4096 ||
		(req.runtime == "mcp" && req.server == "")
}

// invalidRuntimeSelection enforces command-specific flag presence and JSON
// mode rules separately from value bounds; Cobra still parses the input.
func invalidRuntimeSelection(name string, c *cobra.Command, opts authoringcli.Options, req request) bool {
	if name == "test" {
		if req.runtime != "" && req.runtime != "mcp" {
			return true
		}
		if c.Flags().Changed("runtime") && req.runtime == "" {
			return true
		}
		runtimeFlagsChanged := c.Flags().Changed("server") || c.Flags().Changed("tool") ||
			c.Flags().Changed("fixture") || c.Flags().Changed("allow-network") || c.Flags().Changed("deadline")
		if req.runtime == "" && runtimeFlagsChanged {
			return true
		}
	}
	return name == "dev" && opts.Format == "json" && !req.once
}
