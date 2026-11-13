package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/mcpskills"
)

// This CLI has no installer state dependency: a successful read proves only
// protocol-level discovery or document integrity, not agent activation.
type mcpReaderFactory func(string) (*mcpskills.Client, error)

func newSkillsMCPCommand(opts *options, create mcpReaderFactory) *cobra.Command {
	var endpoint string
	root := &cobra.Command{Use: "mcp", Short: "Experimental read-only MCP Skills discovery (no installation)"}
	root.PersistentFlags().StringVar(&endpoint, "endpoint", "", "explicit HTTPS Streamable HTTP MCP endpoint")
	var uri string
	list := &cobra.Command{Use: "list", Short: "List advertised Skills without reading SKILL.md", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			reader, err := startMCPRead(cmd, opts, endpoint, create)
			if err != nil {
				return err
			}
			entries, err := reader.List(cmd.Context())
			if err != nil {
				return mcpPublicError(err)
			}
			report := struct {
				Experimental bool                `json:"experimental"`
				ReadOnly     bool                `json:"read_only"`
				Server       string              `json:"server"`
				Complete     bool                `json:"complete"`
				Skills       []mcpskills.Summary `json:"skills"`
			}{true, true, reader.Origin(), false, entries}
			// MCP Skills permits partial listings even without nextCursor.
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "skills mcp list", report)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Experimental MCP Skills in %s (read only; listing may be partial)\n", prompt.SafeText(reader.Origin())); err != nil {
				return err
			}
			for _, s := range entries {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s) — %s [%s]\n", prompt.SafeText(s.Name), prompt.SafeText(s.URI), prompt.SafeText(s.Description), prompt.SafeText(s.Integrity)); err != nil {
					return err
				}
			}
			return nil
		}}
	inspect := &cobra.Command{Use: "inspect", Short: "Verify one exact SKILL.md without loading it into an agent", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMCPInspect(cmd, opts, endpoint, uri, create)
		}}
	inspect.Flags().StringVar(&uri, "uri", "", "exact URI of the remote SKILL.md")
	root.AddCommand(list, inspect)
	return root
}

func runMCPInspect(cmd *cobra.Command, opts *options, endpoint, uri string, create mcpReaderFactory) error {
	reader, err := startMCPRead(cmd, opts, endpoint, create)
	if err != nil {
		return err
	}
	if uri == "" {
		return CLIUsageError{Code: "INVALID_SELECTION", Message: "specify the exact Skill --uri"}
	}
	item, err := reader.Inspect(cmd.Context(), uri)
	if err != nil {
		return mcpPublicError(err)
	}
	report := struct {
		Experimental bool              `json:"experimental"`
		ReadOnly     bool              `json:"read_only"`
		Skill        mcpskills.Summary `json:"skill"`
	}{true, true, item}
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), "skills mcp inspect", report)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Experimental MCP Skill %s (%s): %s; %d resources, no agent activation\n", prompt.SafeText(item.Name), prompt.SafeText(item.URI), prompt.SafeText(item.Integrity), item.Resources)
	return err
}

func startMCPRead(cmd *cobra.Command, opts *options, endpoint string, create mcpReaderFactory) (*mcpskills.Client, error) {
	if opts.format != "human" && opts.format != "json" {
		return nil, CLIUsageError{Code: "INVALID_FORMAT", Message: "--format must be human or json"}
	}
	if opts.target != "" || opts.scope != "user" || opts.projectRoot != "" || opts.dryRun ||
		opts.acceptSecurityRisk || opts.securityDetails ||
		cmd.Root().PersistentFlags().Changed("scope") {
		return nil, CLIUsageError{Code: "INVALID_OPTION", Message: "MCP Skills PoC is read-only: omit target, scope, project, dry-run and security-override flags"}
	}
	if endpoint == "" {
		return nil, CLIUsageError{Code: "INVALID_SOURCE", Message: "--endpoint requires an explicit HTTPS MCP URL"}
	}
	client, err := create(endpoint)
	if err != nil {
		return nil, mcpPublicError(err)
	}
	return client, nil
}

func mcpPublicError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return CLIUsageError{Code: "CANCELED", Message: "MCP Skills request was canceled"}
	case errors.Is(err, context.DeadlineExceeded):
		return CLIUsageError{Code: "TIMEOUT", Message: "MCP Skills request timed out"}
	case errors.Is(err, mcpskills.ErrInvalidEntry):
		return CLIUsageError{Code: "INVALID_SKILL", Message: "MCP Skill metadata or content failed integrity validation"}
	case errors.Is(err, mcpskills.ErrProtocol):
		return CLIUsageError{Code: "UNSUPPORTED_MCP", Message: "MCP server did not satisfy the 2026-07-28 Skills read-only contract"}
	default:
		return CLIUsageError{Code: "MCP_UNAVAILABLE", Message: "MCP Skills endpoint could not be read"}
	}
}
