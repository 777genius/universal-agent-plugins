package cursor

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Cursor CLI project permissions are a different native surface from Editor
// workspace trust and MCP-server fingerprint approval. No wildcard is admitted.
const cursorToolApprovalNamespace = "com.universal-agent-plugins.cursor-mcp-approval"
const cursorToolApprovalFile = ".agentplugins-cursor-mcp-approvals.json"

type cursorToolApprovals map[string][]string

var _ clients.ExtensionInterpreter = (*Adapter)(nil)

func (*Adapter) InterpretExtension(namespace string, raw json.RawMessage) (domain.SupportLevel, string) {
	if namespace != cursorToolApprovalNamespace {
		return domain.SupportUnsupported, "extension_semantics_not_established"
	}
	if _, err := decodeCursorToolApprovals(raw); err != nil {
		return domain.SupportUnsupported, "cursor_mcp_tool_approval_invalid"
	}
	return domain.SupportNative, ""
}

// No implicit decoder aliases, duplicate JSON keys, unknown fields, wildcard
// permissions or invisible tool names may reach the native CLI config.
func decodeCursorToolApprovals(raw []byte) (cursorToolApprovals, error) {
	if len(raw) == 0 || len(raw) > 16<<10 {
		return nil, fmt.Errorf("Cursor MCP tool approval extension has invalid size")
	}
	document, err := shared.DecodeStrictJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid Cursor MCP tool approval extension: %w", err)
	}
	servers, ok := document["mcp_auto_approve"].(map[string]any)
	if len(document) != 2 || document["version"] != json.Number("1") || !ok || len(servers) == 0 || len(servers) > 32 {
		return nil, fmt.Errorf("Cursor MCP tool approval requires exact version 1 and selected servers")
	}
	approved := make(cursorToolApprovals, len(servers))
	for server, rawTools := range servers {
		tools, ok := rawTools.([]any)
		if !cursorApprovalName(server) || !ok || len(tools) == 0 || len(tools) > 64 {
			return nil, fmt.Errorf("invalid Cursor MCP approval server or tool count")
		}
		seen := map[string]bool{}
		for _, rawTool := range tools {
			tool, ok := rawTool.(string)
			if !ok || !cursorApprovalName(tool) || seen[tool] {
				return nil, fmt.Errorf("invalid or duplicate Cursor MCP approval tool for %q", server)
			}
			seen[tool] = true
			approved[server] = append(approved[server], tool)
		}
	}
	return approved, nil
}

func cursorApprovalName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, char := range name {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '_' || char == '-' || char == '.' {
			continue
		}
		return false
	}
	return true
}

func cursorHasToolApproval(components []domain.ComponentDecision) bool {
	for _, component := range components {
		if component.Kind == domain.ComponentExtension && component.Name == cursorToolApprovalNamespace && component.Support == domain.SupportNative {
			return true
		}
	}
	return false
}

func cursorNativeInstallComponents(components []domain.ComponentDecision) []domain.ComponentDecision {
	selected := make([]domain.ComponentDecision, 0, len(components))
	for _, component := range components {
		if component.Kind == domain.ComponentExtension && component.Name == cursorToolApprovalNamespace && component.Support == domain.SupportNative {
			continue
		}
		selected = append(selected, component)
	}
	return selected
}

func selectedCursorToolApprovals(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (cursorToolApprovals, error) {
	raw, declared := envelope.Manifest.Extensions[cursorToolApprovalNamespace]
	selected := cursorHasToolApproval(plan.Components)
	if !declared {
		if selected {
			return nil, fmt.Errorf("Cursor selected tool approval is absent from the package")
		}
		return nil, nil
	}
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return nil, fmt.Errorf("Cursor MCP tool auto-approval requires an explicit project scope")
	}
	approved, err := decodeCursorToolApprovals(raw)
	if err != nil {
		return nil, err
	}
	if !selected {
		return nil, fmt.Errorf("Cursor MCP tool approval extension was not selected")
	}
	nativeServers := shared.SupportedMCPNames(plan)
	for server := range approved {
		if !slices.Contains(nativeServers, server) {
			return nil, fmt.Errorf("Cursor approval references unselected MCP server %q", server)
		}
	}
	return approved, nil
}

func cursorToolApprovalDisclosure(approved cursorToolApprovals) string {
	if len(approved) == 0 {
		return ""
	}
	selected := []string{}
	for server, tools := range approved {
		for _, tool := range tools {
			selected = append(selected, "Mcp("+server+":"+tool+")")
		}
	}
	slices.Sort(selected)
	return "Cursor Agent project CLI will allow only these publisher-declared tools after confirmed add: " +
		strings.Join(selected, ", ") + ". Permissions persist after removal and are not fingerprint-bound; review the source. " +
		"Explicit deny/ask, Editor workspace trust, authentication and managed policy remain separate."
}
