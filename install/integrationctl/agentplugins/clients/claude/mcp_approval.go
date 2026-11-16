package claude

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func claudeMCPApprovalPaths(request domain.ActivationRequest) (string, []string, error) {
	root := request.Client.ConfigRoot
	if !exactAbsolute(root) {
		return "", nil, fmt.Errorf("claude MCP approval requires an exact config root")
	}
	user := filepath.Join(root, "settings.json")
	if request.Plan.Scope != domain.ScopeProject {
		return user, []string{user}, nil
	}
	project, err := claudePlanProjectRoot(request.Client, request.Plan)
	if err != nil {
		return "", nil, err
	}
	local := filepath.Join(project, "settings.local.json")
	return local, []string{user, filepath.Join(project, "settings.json"), local}, nil
}

func claudeMCPChoices(doc map[string]any, key string) ([]string, error) {
	value, exists := doc[key]
	if !exists {
		return nil, nil
	}
	values, ok := value.([]any)
	if !ok || len(values) > 4096 {
		return nil, fmt.Errorf("claude MCP approval list %s is invalid", key)
	}
	choices := make([]string, 0, len(values))
	for _, value := range values {
		s, ok := value.(string)
		if !ok || s == "" || len(s) > 4096 {
			return nil, fmt.Errorf("claude MCP approval choice is invalid")
		}
		choices = append(choices, s)
	}
	return choices, nil
}

func claudeMCPExplicitlyDenied(env clients.Env, paths []string, selector string) (bool, error) {
	for _, path := range paths {
		snapshot, err := env.NativeConfig.ReadExactFile(path)
		if err != nil {
			return false, err
		}
		if !snapshot.Exists {
			continue
		}
		if len(snapshot.Body) > 1<<20 {
			return false, fmt.Errorf("claude MCP settings exceed limit")
		}
		doc, err := shared.DecodeStrictJSONObject(snapshot.Body)
		if err != nil {
			return false, err
		}
		denied, err := claudeMCPChoices(doc, "disabledMcpjsonServers")
		if err != nil {
			return false, err
		}
		if slices.Contains(denied, selector) {
			return true, nil
		}
	}
	return false, nil
}

// Approvals are exact persistent user preferences. Uninstall retains them;
// neither an existing rejection nor managed policy is removed or overridden.
func grantClaudeMCPApproval(env clients.Env, request domain.ActivationRequest, selector string) (bool, error) {
	target, paths, err := claudeMCPApprovalPaths(request)
	if err != nil {
		return false, err
	}
	denied, err := claudeMCPExplicitlyDenied(env, paths, selector)
	if err != nil || denied {
		return false, err
	}
	if request.VerifyOnly {
		return false, nil
	}
	if request.Delivery.ArtifactDigest == "" {
		return false, fmt.Errorf("claude MCP approval requires a verified delivery")
	}
	return shared.UpdateProjectTrustFile(env.NativeConfig, target, func(body []byte) ([]byte, error) { return renderClaudeMCPApproval(body, selector) })
}

func renderClaudeMCPApproval(body []byte, selector string) ([]byte, error) {
	doc, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nil, err
	}
	denied, err := claudeMCPChoices(doc, "disabledMcpjsonServers")
	if err != nil {
		return nil, err
	}
	if slices.Contains(denied, selector) {
		return body, nil
	}
	enabled, err := claudeMCPChoices(doc, "enabledMcpjsonServers")
	if err != nil {
		return nil, err
	}
	if slices.Contains(enabled, selector) {
		return body, nil
	}
	enabled = append(enabled, selector)
	doc["enabledMcpjsonServers"] = enabled
	next, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(next, '\n'), nil
}
