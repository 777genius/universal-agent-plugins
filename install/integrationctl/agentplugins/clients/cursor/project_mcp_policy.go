package cursor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func cursorMCPPolicyPath(request domain.ActivationRequest, name string) string {
	return filepath.Join(request.Client.ConfigRoot, "projects", cursorProjectSlug(request.Plan.TargetContext.ProjectRoot), name)
}

// The native disabled store rewrites a non-array document to an empty array.
// Refuse malformed policy before invoking any native command, and never enable
// a server whose existing explicit disablement the owner has not revoked.
func cursorMCPPolicyAvailable(env clients.Env, request domain.ActivationRequest, names []string) error {
	for _, filename := range []string{"mcp-disabled.json", "mcp-approvals.json"} {
		path := cursorMCPPolicyPath(request, filename)
		if err := pathpolicy.RequireContainedChild(request.Client.ConfigRoot, path); err != nil {
			return fmt.Errorf("cursor MCP policy path is not safely contained in its selected profile")
		}
		snapshot, err := env.NativeConfig.ReadExactFile(path)
		if err != nil {
			return err
		}
		if !snapshot.Exists {
			continue
		}
		body := bytes.TrimSpace(snapshot.Body)
		if len(body) > 1<<20 || len(body) == 0 || body[0] != '[' {
			return fmt.Errorf("preserved malformed Cursor MCP policy")
		}
		entries := []string{}
		if err := json.Unmarshal(body, &entries); err != nil {
			return fmt.Errorf("preserved malformed Cursor MCP policy")
		}
		if filename != "mcp-disabled.json" {
			continue
		}
		for _, entry := range entries {
			for _, name := range names {
				if entry == name {
					return fmt.Errorf("preserved explicit Cursor MCP disablement for %q", name)
				}
			}
		}
	}
	return nil
}
