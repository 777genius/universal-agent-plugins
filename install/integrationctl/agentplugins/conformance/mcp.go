package conformance

import (
	"encoding/json"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"net/netip"
	"net/url"
	"sort"
	"strings"
)

func (loader InstallerDecoder) MCP(body []byte, pluginSchema string, stdio func(map[string]any) (*domain.StdioRequirement, error)) (domain.MCPComponent, []domain.Diagnostic) {
	diagnostic := func(code, message string, cause error) domain.Diagnostic {
		if loader.issue != nil {
			loader.issue(code, "", cause)
		}
		return mcpDiagnostic(code, message, cause)
	}
	component := domain.MCPComponent{
		Present:       true,
		Raw:           append(json.RawMessage(nil), body...),
		Servers:       map[string]domain.MCPServer{},
		InvalidServer: map[string]domain.Diagnostic{},
	}
	rawFields, decoded, err := decodeJSONObject(body)
	if err != nil {
		return component, []domain.Diagnostic{diagnostic("mcp_malformed", "parse root mcp.json", err)}
	}
	schemaURI, ok := decoded["$schema"].(string)
	if !ok || strings.TrimSpace(schemaURI) == "" {
		return component, []domain.Diagnostic{diagnostic("mcp_schema_missing", "mcp.json requires a string $schema", nil)}
	}
	component.SchemaURI = schemaURI
	if schemaVersion(schemaURI) != schemaVersion(pluginSchema) {
		return component, []domain.Diagnostic{diagnostic("mcp_schema_mismatch", fmt.Sprintf("mcp.json schema %q does not match plugin.json schema version %q", schemaURI, pluginSchema), nil)}
	}
	if schemaURI != domain.MCPSchemaV1 || !loader.Registry.Supports(schemaURI) {
		return component, []domain.Diagnostic{diagnostic("mcp_schema_unsupported", fmt.Sprintf("unsupported Agent Plugins MCP schema %q", schemaURI), nil)}
	}
	serversRaw, ok := rawFields["mcpServers"]
	if !ok {
		return component, []domain.Diagnostic{diagnostic("mcp_servers_missing", "mcp.json requires mcpServers", nil)}
	}
	var serverDocuments map[string]json.RawMessage
	if err := decodeRawJSONObject(serversRaw, &serverDocuments); err != nil || serverDocuments == nil {
		return component, []domain.Diagnostic{diagnostic("mcp_servers_invalid", "mcpServers must be a JSON object", err)}
	}
	topLevel := make(map[string]any, len(decoded))
	for key, value := range decoded {
		topLevel[key] = value
	}
	topLevel["mcpServers"] = map[string]any{}
	if err := loader.Registry.Validate(schemaURI, topLevel); err != nil {
		return component, []domain.Diagnostic{diagnostic("mcp_schema_invalid", "mcp.json top-level document does not conform to Agent Plugins 1.0", err)}
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
		var decodedServer map[string]any
		phase := "server_json"
		decodeErr := decodeJSON(raw, &decodedServer)
		if decodeErr == nil && decodedServer == nil {
			decodeErr = fmt.Errorf("server config must be a JSON object")
		}
		if decodeErr == nil {
			phase = "server_schema"
			document := map[string]any{
				"$schema":    schemaURI,
				"mcpServers": map[string]any{name: decodedServer},
			}
			decodeErr = loader.Registry.Validate(schemaURI, document)
		}
		var requirement *domain.StdioRequirement
		if decodeErr == nil {
			phase = "server_semantics"
			typeName, _ := decodedServer["type"].(string)
			switch typeName {
			case "stdio":
				requirement, decodeErr = stdio(decodedServer)
			case "streamable-http", "sse":
				decodeErr = validateRemoteServer(decodedServer)
			}
		}
		if decodeErr != nil {
			if loader.issue != nil {
				loader.issue(phase, name, decodeErr)
			}
			diagnostic := domain.Diagnostic{
				Severity: domain.SeverityError,
				Boundary: domain.BoundaryMCPServer,
				Code:     "mcp_server_invalid",
				Path:     "mcp.json",
				Item:     name,
				Message:  fmt.Sprintf("MCP server %q was skipped because its configuration is invalid: %v", name, decodeErr),
			}
			component.InvalidServer[name] = diagnostic
			diagnostics = append(diagnostics, diagnostic)
			continue
		}
		typeName, _ := decodedServer["type"].(string)
		component.Servers[name] = domain.MCPServer{
			Name:             name,
			Type:             typeName,
			Raw:              append(json.RawMessage(nil), raw...),
			Decoded:          decodedServer,
			StdioRequirement: requirement,
		}
	}
	return component, diagnostics
}

func schemaVersion(uri string) string {
	const marker = "/schemas/"
	index := strings.Index(uri, marker)
	if index < 0 {
		return ""
	}
	remainder := uri[index+len(marker):]
	if slash := strings.IndexByte(remainder, '/'); slash >= 0 {
		return remainder[:slash]
	}
	return ""
}

func validateRemoteServer(config map[string]any) error {
	rawURL, _ := config["url"].(string)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" {
		return semanticError("remote_url_absolute", "remote MCP URL must be an absolute HTTP or HTTPS URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return semanticError("remote_url_scheme", "remote MCP URL must use HTTP or HTTPS")
	}
	if parsed.User != nil || strings.Contains(rawURL, "#") {
		return semanticError("remote_url_authority", "remote MCP URL must not contain user information or a fragment")
	}
	host := parsed.Hostname()
	if host == "" {
		return semanticError("remote_url_host", "remote MCP URL requires a host")
	}
	if scheme == "http" && !strings.EqualFold(host, "localhost") {
		address, parseErr := netip.ParseAddr(host)
		if parseErr != nil || address.Zone() != "" || !address.IsLoopback() {
			return semanticError("remote_url_tls", "non-loopback remote MCP URLs must use HTTPS")
		}
	}
	headers, _ := config["headers"].(map[string]any)
	seen := make(map[string]struct{}, len(headers))
	for _, name := range sortedKeys(headers) {
		rawValue := headers[name]
		value, _ := rawValue.(string)
		if !validHTTPHeaderName(name) || !validHTTPHeaderValue(value) {
			return semanticError("remote_header_invalid", "remote MCP header %q is not a valid HTTP field", name)
		}
		folded := strings.ToLower(name)
		if _, duplicate := seen[folded]; duplicate {
			return semanticError("remote_header_duplicate", "remote MCP header %q is duplicated with different casing", name)
		}
		seen[folded] = struct{}{}
	}
	return nil
}

func validHTTPHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("!#$%&'*+-.^_`|~", rune(character))) {
			return false
		}
	}
	return true
}

func validHTTPHeaderValue(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character < 0x20 && character != '\t') || character == 0x7f {
			return false
		}
	}
	return true
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

// ParseStdio extracts inert requirements under explicit path policies. It never resolves executables.
func ParseStdio(config map[string]any, commandPolicy func(string) (string, error), cwdPolicy func(string) error) (*domain.StdioRequirement, error) {
	command, _ := config["command"].(string)
	if command == "" {
		return nil, semanticError("stdio_command_empty", "stdio command must be a non-empty executable token")
	}
	requirement := &domain.StdioRequirement{Command: command, Kind: domain.ExecutableBare}
	if strings.ContainsAny(command, `/\\`) {
		relative, err := commandPolicy(command)
		if err != nil {
			return nil, err
		}
		requirement.Kind, requirement.BundledRelativePath = domain.ExecutableBundled, relative
	}
	values := []string{}
	if args, ok := config["args"].([]any); ok {
		for _, value := range args {
			if text, ok := value.(string); ok {
				values = append(values, text)
			}
		}
	}
	if env, ok := config["env"].(map[string]any); ok {
		if _, reserved := env["PLUGIN_ROOT"]; reserved {
			return nil, semanticError("stdio_env_reserved", "stdio server must not define reserved environment variable PLUGIN_ROOT")
		}
		if _, reserved := env["PLUGIN_DATA"]; reserved {
			return nil, semanticError("stdio_env_reserved", "stdio server must not define reserved environment variable PLUGIN_DATA")
		}
		for _, value := range env {
			if text, ok := value.(string); ok {
				values = append(values, text)
			}
		}
	}
	if cwd, ok := config["cwd"].(string); ok {
		if err := cwdPolicy(cwd); err != nil {
			return nil, err
		}
		values = append(values, cwd)
	}
	for _, value := range values {
		if strings.Contains(value, "${PLUGIN_ROOT}") {
			requirement.UsesPluginRoot = true
		}
		if strings.Contains(value, "${PLUGIN_DATA}") {
			requirement.UsesPluginData = true
		}
	}
	return requirement, nil
}

type semanticFailure struct{ Code, message string }

func (e *semanticFailure) Error() string { return e.message }
func semanticError(code, format string, args ...any) error {
	return &semanticFailure{code, fmt.Sprintf(format, args...)}
}
