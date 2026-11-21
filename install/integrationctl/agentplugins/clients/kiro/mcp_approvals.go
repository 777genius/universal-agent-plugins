package kiro

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// MCPApprovalNamespace is an opt-in, publisher-authored Kiro extension of
// plugin.json. Its exact tool names, not package-selected wildcards, are shown
// during review and projected into Kiro's documented autoApprove setting.
const MCPApprovalNamespace = "com.universal-agent-plugins.kiro-mcp-approval"

type mcpApprovals struct {
	Version int                 `json:"version"`
	Servers map[string][]string `json:"mcp_auto_approve"`
}

var _ clients.ExtensionInterpreter = (*Adapter)(nil)

func (*Adapter) InterpretExtension(namespace string, raw json.RawMessage) (domain.SupportLevel, string) {
	if namespace != MCPApprovalNamespace {
		return domain.SupportUnsupported, "extension_semantics_not_established"
	}
	if _, err := decodeMCPApprovals(raw); err != nil {
		return domain.SupportUnsupported, "kiro_mcp_approval_invalid"
	}
	return domain.SupportNative, ""
}

func decodeMCPApprovals(raw json.RawMessage) (mcpApprovals, error) {
	var value mcpApprovals
	if len(raw) == 0 || len(raw) > 16<<10 {
		return value, fmt.Errorf("Kiro MCP approval extension exceeds its limit")
	}
	fields, err := shared.DecodeStrictJSONObject(raw)
	if err != nil {
		return value, fmt.Errorf("invalid Kiro MCP approval object: %w", err)
	}
	// encoding/json accepts field names case-insensitively by default. Kiro's
	// trust declaration is an exact, versioned contract: refuse aliases.
	_, hasServers := fields["mcp_auto_approve"]
	if len(fields) != 2 || fields["version"] != json.Number("1") || !hasServers {
		return value, fmt.Errorf("Kiro MCP approval fields must match the exact v1 schema")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("invalid Kiro MCP approval fields: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return value, fmt.Errorf("Kiro MCP approval extension must contain one object")
	}
	if value.Version != 1 || len(value.Servers) == 0 || len(value.Servers) > 32 {
		return value, fmt.Errorf("unsupported Kiro MCP approval version or server count")
	}
	for server, tools := range value.Servers {
		if !validKiroApprovalName(server) || len(tools) == 0 || len(tools) > 64 {
			return value, fmt.Errorf("invalid Kiro MCP approval server or tool count")
		}
		seen := map[string]bool{}
		for _, tool := range tools {
			if !validKiroApprovalName(tool) || seen[tool] {
				return value, fmt.Errorf("invalid or duplicate Kiro MCP auto-approved tool for %q", server)
			}
			seen[tool] = true
		}
	}
	return value, nil
}

// Kiro's wildcard is intentionally excluded: a package update may expose new
// tools, and a reviewed source cannot authorize their future unknown behavior.
func validKiroApprovalName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	// Trust previews must not permit invisible, bidirectional or markup-like
	// names to impersonate a different tool. Native Kiro tool names are ASCII.
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') && r != '_' && r != '-' && r != '.' {
			return false
		}
	}
	return true
}

func kiroNativeComponents(components []domain.ComponentDecision) bool {
	found := false
	for _, component := range components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		switch component.Kind {
		case domain.ComponentSkill, domain.ComponentMCPServer:
			found = true
		case domain.ComponentExtension:
			if component.Name != MCPApprovalNamespace {
				return false
			}
		default:
			return false
		}
	}
	return found
}

func selectedKiroApprovals(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (mcpApprovals, error) {
	raw, exists := envelope.Manifest.Extensions[MCPApprovalNamespace]
	if !exists {
		return mcpApprovals{}, nil
	}
	approved, err := decodeMCPApprovals(raw)
	if err != nil {
		return approved, err
	}
	extensionSelected := false
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentExtension && component.Name == MCPApprovalNamespace &&
			component.Support == domain.SupportNative {
			extensionSelected = true
			break
		}
	}
	if !extensionSelected {
		return approved, fmt.Errorf("Kiro tool approvals were not selected by the plan")
	}
	selected := shared.SupportedMCPNames(plan)
	for server := range approved.Servers {
		if !slices.Contains(selected, server) {
			return approved, fmt.Errorf("Kiro approval references an unselected MCP server %q", server)
		}
	}
	return approved, nil
}

// Only Kiro's generated native projection is changed. Source MCP documents
// stay portable and exact, and the resulting native entry is receipt-owned.
func applyKiroApprovals(root string, approval mcpApprovals) error {
	if len(approval.Servers) == 0 {
		return nil
	}
	path := filepath.Join(root, "mcp.json")
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return fmt.Errorf("Kiro MCP projection has no servers")
	}
	for name, tools := range approval.Servers {
		server, ok := servers[name].(map[string]any)
		if !ok {
			return fmt.Errorf("Kiro MCP projection lacks approved server %q", name)
		}
		if _, existing := server["autoApprove"]; existing {
			return fmt.Errorf("Kiro MCP projection has conflicting approval for %q", name)
		}
		server["autoApprove"] = tools
	}
	return shared.WriteJSON(path, document)
}

// Reconstructing exact native ownership from an unchanged active package must
// not silently accept a stale projection that omitted the publisher's approved
// tools. A caller can re-stage the confirmed source, but never forge a receipt.
func verifyKiroActiveApprovals(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	approved, err := selectedKiroApprovals(envelope, plan)
	if err != nil {
		return err
	}
	for name, tools := range approved.Servers {
		server, err := projectedKiroMCPServer(root, name)
		if err != nil {
			return fmt.Errorf("%w: read native Kiro approval for %q: %v", domain.ErrProjectionRefreshRequired, name, err)
		}
		actual, ok := server["autoApprove"].([]any)
		if !ok || len(actual) != len(tools) {
			return fmt.Errorf("%w: native Kiro approved tools changed for %q", domain.ErrProjectionRefreshRequired, name)
		}
		for index, tool := range tools {
			if actual[index] != tool {
				return fmt.Errorf("%w: native Kiro approved tool changed for %q", domain.ErrProjectionRefreshRequired, name)
			}
		}
	}
	return nil
}

func kiroApprovalDisclosure(approved mcpApprovals) string {
	if len(approved.Servers) == 0 {
		return ""
	}
	var entries []string
	for name, tools := range approved.Servers {
		for _, tool := range tools {
			entries = append(entries, name+"/"+tool)
		}
	}
	slices.Sort(entries)
	return "Kiro will automatically approve these publisher-declared MCP tools only: " +
		strings.Join(entries, ", ") + ". Review the selected source before confirming; workspace trust, higher-priority deny/ask rules and authentication remain separate."
}
