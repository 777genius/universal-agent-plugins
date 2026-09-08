package agentpluginscli

import (
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

const openCodeNamespaceNotice = "opencode_tool_namespace_not_evaluated"

// This is output evidence, not a portable validation or component selection rule.
func withOpenCodeRuntimeNotice(result usecase.AddResult) usecase.AddResult {
	if result.Plan.ClientID != domain.ClientOpenCode || len(domain.SelectedMCPNames(result.Plan)) == 0 {
		return result
	}
	for _, diagnostic := range result.Plan.Diagnostics {
		if diagnostic.Code == openCodeNamespaceNotice {
			return result
		}
	}
	result.Plan.Diagnostics = append(append([]domain.Diagnostic(nil), result.Plan.Diagnostics...), domain.Diagnostic{
		Severity: domain.SeverityWarning, Boundary: domain.BoundaryMCPServer,
		Code:    openCodeNamespaceNotice,
		Message: "OpenCode MCP delivery verification covers configuration only; callable tool-ID uniqueness is not evaluated. OpenCode can map different server/tool names to the same callable ID. Observed colliding tools are an unsupported runtime combination and are not separately addressable. Verify the tools in the intended OpenCode session before relying on both.",
	})
	return result
}

func renderOpenCodeRuntimeNotice(writer io.Writer, result usecase.AddResult) error {
	for _, diagnostic := range withOpenCodeRuntimeNotice(result).Plan.Diagnostics {
		if diagnostic.Code == openCodeNamespaceNotice {
			_, err := fmt.Fprintf(writer, "  Warning: %s: %s\n", diagnostic.Code, diagnostic.Message)
			return err
		}
	}
	return nil
}
