package agentpluginscli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
)

type validationComponents struct {
	Skills        int      `json:"skills"`
	MCPServers    int      `json:"mcp_servers"`
	AppBindings   int      `json:"app_bindings"`
	Extensions    int      `json:"extensions"`
	MCPTransports []string `json:"mcp_transports,omitempty"`
}

type validationSource struct {
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Path       string `json:"path,omitempty"`
}

type validationResult struct {
	Conformant     bool                       `json:"conformant"`
	Name           string                     `json:"name"`
	Version        string                     `json:"version,omitempty"`
	FormatID       string                     `json:"format_id"`
	SchemaURI      string                     `json:"schema_uri"`
	SchemaVersion  string                     `json:"schema_version"`
	Source         validationSource           `json:"source"`
	TreeDigest     string                     `json:"tree_digest"`
	ManifestDigest string                     `json:"manifest_digest"`
	Components     validationComponents       `json:"components"`
	Diagnostics    []domain.Diagnostic        `json:"diagnostics,omitempty"`
	Security       *domain.SecurityAssessment `json:"security,omitempty"`
}

func newValidateCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <local-or-full-sha-source>",
		Short: "Validate one Agent Plugins package without installing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			source := strings.TrimSpace(args[0])
			if !explicitLocalPath(source) && exactGitPattern.FindStringSubmatch(source) == nil {
				return fmt.Errorf("validate requires a local path or exact owner/repo@FULL_SHA[//path] source")
			}
			loaded, err := app.loadPackage(cmd.Context(), source)
			if err != nil {
				return err
			}
			if loaded.cleanup != nil {
				defer loaded.cleanup()
			}
			result := newValidationResult(loaded.envelope)
			result.Security = loaded.security
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "validate", result)
			}
			if loaded.security != nil {
				renderSecurityAssessment(cmd, *loaded.security, true)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Valid Agent Plugin: %s %s\nSchema: %s\nSource: %s\nComponents: %d skills, %d MCP servers, %d app bindings\n",
				result.Name, result.Version, result.SchemaURI, publicPackageSource(loaded.envelope.Source),
				result.Components.Skills, result.Components.MCPServers, result.Components.AppBindings)
			return err
		},
	}
}

func newValidationResult(envelope domain.PackageEnvelope) validationResult {
	transports := make([]string, 0, len(envelope.MCP.Servers))
	seen := map[string]struct{}{}
	for _, server := range envelope.MCP.Servers {
		transport := strings.TrimSpace(server.Type)
		if transport == "" {
			continue
		}
		if _, duplicate := seen[transport]; duplicate {
			continue
		}
		seen[transport] = struct{}{}
		transports = append(transports, transport)
	}
	sort.Strings(transports)
	return validationResult{
		Conformant: true, Name: envelope.Manifest.Name, Version: envelope.Manifest.Version,
		FormatID: envelope.FormatID, SchemaURI: envelope.SchemaURI, SchemaVersion: envelope.SchemaVersion,
		Source:     validationSource{Repository: envelope.Source.Repository, Revision: envelope.Source.ResolvedRevision, Path: envelope.Source.PackageSubpath},
		TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest,
		Components: validationComponents{
			Skills: len(envelope.Inventory.Skills), MCPServers: len(envelope.Inventory.MCPServers),
			AppBindings: len(envelope.Inventory.AppBindings), Extensions: len(envelope.Inventory.Extensions),
			MCPTransports: transports,
		},
		Diagnostics: append([]domain.Diagnostic(nil), envelope.Diagnostics...),
	}
}
