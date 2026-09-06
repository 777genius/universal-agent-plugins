package loader

import (
	"encoding/json"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (loader Loader) loadMCP(filename, pluginSchema string) (domain.MCPComponent, []domain.Diagnostic) {
	body, exists, err := readRegularFile(filename)
	if !exists {
		return domain.MCPComponent{}, nil
	} // Preserve AUD-009 installer disposition.
	if err != nil {
		return domain.MCPComponent{Present: true, Raw: append(json.RawMessage(nil), body...), Servers: map[string]domain.MCPServer{}, InvalidServer: map[string]domain.Diagnostic{}}, []domain.Diagnostic{mcpDiagnostic("mcp_read_failed", "read root mcp.json", err)}
	}
	return (conformance.InstallerDecoder{Registry: loader.Registry}).MCP(body, pluginSchema, func(config map[string]any) (*domain.StdioRequirement, error) {
		return validateStdioServer(filepath.Dir(filename), config)
	})
}

func validateStdioServer(root string, config map[string]any) (*domain.StdioRequirement, error) {
	return conformance.ParseStdio(config, func(command string) (string, error) {
		relative, err := bundledCommandPath(command)
		if err != nil {
			return "", err
		}
		candidate := filepath.Join(root, filepath.FromSlash(relative))
		if err := validateExistingCommandAncestor(root, candidate, command); err != nil {
			return "", err
		}
		return relative, nil
	}, validateCWD)
}

func validateExistingCommandAncestor(root, candidate, command string) error {
	current := candidate
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, resolveErr := filepath.EvalSymlinks(current)
			if resolveErr != nil {
				return fmt.Errorf("resolve bundled stdio command %q: %w", command, resolveErr)
			}
			contained, relativeErr := filepath.Rel(root, resolved)
			if relativeErr != nil || contained == ".." || filepath.IsAbs(contained) || strings.HasPrefix(contained, ".."+string(filepath.Separator)) {
				return fmt.Errorf("bundled stdio command %q resolves outside the plugin root", command)
			}
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect bundled stdio command %q: %w", command, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return fmt.Errorf("bundled stdio command %q has no resolvable plugin-root ancestor", command)
		}
		current = parent
	}
}

func bundledCommandPath(command string) (string, error) {
	if !strings.HasPrefix(command, "./") || strings.Contains(command, `\\`) {
		return "", fmt.Errorf("bundled stdio command %q must be a ./-prefixed plugin-relative path", command)
	}
	relative := strings.TrimPrefix(command, "./")
	if relative == "" || path.Clean(relative) != relative || strings.HasPrefix(relative, "../") {
		return "", fmt.Errorf("bundled stdio command %q escapes the plugin root", command)
	}
	for _, segment := range strings.Split(relative, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return "", fmt.Errorf("bundled stdio command %q contains a non-portable path segment: %w", command, err)
		}
	}
	return relative, nil
}

func validateCWD(value string) error {
	var suffix string
	switch {
	case strings.HasPrefix(value, "./"):
		suffix = strings.TrimPrefix(value, "./")
	case value == "${PLUGIN_ROOT}" || value == "${PLUGIN_DATA}":
		return nil
	case strings.HasPrefix(value, "${PLUGIN_ROOT}/"):
		suffix = strings.TrimPrefix(value, "${PLUGIN_ROOT}/")
	case strings.HasPrefix(value, "${PLUGIN_DATA}/"):
		suffix = strings.TrimPrefix(value, "${PLUGIN_DATA}/")
	default:
		return fmt.Errorf("stdio cwd must be ./-, PLUGIN_ROOT-, or PLUGIN_DATA-rooted")
	}
	if suffix == "" || path.IsAbs(suffix) || strings.Contains(suffix, `\\`) || path.Clean(suffix) != suffix || strings.HasPrefix(suffix, "../") {
		return fmt.Errorf("stdio cwd escapes its declared root")
	}
	for _, segment := range strings.Split(suffix, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return fmt.Errorf("stdio cwd contains a non-portable path segment: %w", err)
		}
	}
	return nil
}

func (loader Loader) loadOpenAIMCP(path string, declared bool) (domain.MCPComponent, []domain.Diagnostic) {
	body, exists, err := readRegularFile(path)
	component := domain.MCPComponent{
		Present: exists, Raw: append(json.RawMessage(nil), body...),
		Servers: map[string]domain.MCPServer{}, InvalidServer: map[string]domain.Diagnostic{},
	}
	if !exists {
		if declared {
			return component, []domain.Diagnostic{openAIMCPDiagnostic("mcp_manifest_missing", "official manifest declares .mcp.json but the file is missing", nil)}
		}
		return component, nil
	}
	if err != nil {
		return component, []domain.Diagnostic{openAIMCPDiagnostic("mcp_read_failed", "read root .mcp.json", err)}
	}
	if !declared {
		return component, []domain.Diagnostic{{
			Severity: domain.SeverityWarning, Boundary: domain.BoundaryMCP,
			Code: "undeclared_mcp_manifest_ignored", Path: ".mcp.json",
			Message: "root .mcp.json is ignored because .codex-plugin/plugin.json does not declare mcpServers",
		}}
	}
	rawFields, _, err := decodeJSONObject(body)
	if err != nil {
		return component, []domain.Diagnostic{openAIMCPDiagnostic("mcp_malformed", "parse root .mcp.json", err)}
	}
	serverDocuments := rawFields
	for _, wrapper := range []string{"mcp_servers", "mcpServers"} {
		if wrapped, ok := rawFields[wrapper]; ok {
			if len(rawFields) != 1 {
				return component, []domain.Diagnostic{openAIMCPDiagnostic("mcp_servers_invalid", "wrapped .mcp.json cannot contain sibling fields", nil)}
			}
			if err := decodeRawJSONObject(wrapped, &serverDocuments); err != nil || serverDocuments == nil {
				return component, []domain.Diagnostic{openAIMCPDiagnostic("mcp_servers_invalid", wrapper+" must be an object", err)}
			}
			break
		}
	}
	component.Enabled = true
	names := make([]string, 0, len(serverDocuments))
	for name := range serverDocuments {
		names = append(names, name)
	}
	sort.Strings(names)
	var diagnostics []domain.Diagnostic
	for _, name := range names {
		raw := serverDocuments[name]
		var config map[string]any
		if err := decodeJSON(raw, &config); err != nil || config == nil {
			diagnostic := invalidOfficialMCP(name, "server config must be an object")
			component.InvalidServer[name] = diagnostic
			diagnostics = append(diagnostics, diagnostic)
			continue
		}
		typeName, valid := officialMCPType(config)
		if !valid {
			diagnostic := invalidOfficialMCP(name, "server requires a command for stdio or a URL for http/sse")
			component.InvalidServer[name] = diagnostic
			diagnostics = append(diagnostics, diagnostic)
			continue
		}
		component.Servers[name] = domain.MCPServer{Name: name, Type: typeName, Raw: append(json.RawMessage(nil), raw...), Decoded: config}
	}
	return component, diagnostics
}

func openAIMCPDiagnostic(code, message string, cause error) domain.Diagnostic {
	if cause != nil {
		message += ": " + cause.Error()
	}
	return domain.Diagnostic{Severity: domain.SeverityError, Boundary: domain.BoundaryMCP, Code: code, Path: ".mcp.json", Message: message}
}

func officialMCPType(config map[string]any) (string, bool) {
	typeName, _ := config["type"].(string)
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "stdio":
		_, ok := config["command"].(string)
		return "stdio", ok
	case "http", "streamable-http":
		_, ok := config["url"].(string)
		return "streamable-http", ok
	case "sse":
		_, ok := config["url"].(string)
		return "sse", ok
	case "":
		if _, ok := config["command"].(string); ok {
			return "stdio", true
		}
		if _, ok := config["url"].(string); ok {
			return "streamable-http", true
		}
	}
	return "", false
}

func invalidOfficialMCP(name, message string) domain.Diagnostic {
	return domain.Diagnostic{
		Severity: domain.SeverityError, Boundary: domain.BoundaryMCPServer,
		Code: "mcp_server_invalid", Path: ".mcp.json", Item: name,
		Message: fmt.Sprintf("MCP server %q was skipped because %s", name, message),
	}
}

func mcpDiagnostic(code, message string, cause error) domain.Diagnostic {
	if cause != nil {
		message += ": " + cause.Error()
	}
	return domain.Diagnostic{
		Severity: domain.SeverityError,
		Boundary: domain.BoundaryMCP,
		Code:     code,
		Path:     "mcp.json",
		Message:  message,
	}
}
