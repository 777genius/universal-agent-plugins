package agentpluginscli

import (
	"fmt"
	"io"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

const openCodeNamespaceNotice = usecase.OpenCodeNamespaceCheckedCode

func renderOpenCodeRuntimeNotice(writer io.Writer, result usecase.AddResult) error {
	for _, diagnostic := range result.Plan.Diagnostics {
		if diagnostic.Code == openCodeNamespaceNotice {
			_, err := fmt.Fprintf(writer, "  Info: %s: %s\n", diagnostic.Code, diagnostic.Message)
			return err
		}
	}
	return nil
}
