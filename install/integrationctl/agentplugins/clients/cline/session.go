package cline

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.SessionPlanner = (*Adapter)(nil)

// PlanSession selects only the owned Project MCP source. It neither changes the
// original User configuration nor claims native default discovery or activation.
func (adapter *Adapter) PlanSession(ctx context.Context, env clients.Env, req clients.SessionRequest) (clients.SessionPlan, error) {
	if err := ctx.Err(); err != nil {
		return clients.SessionPlan{}, err
	}
	executable, err := clineSessionExecutable(env, req.Client, len(req.Bindings))
	if err != nil {
		return clients.SessionPlan{}, err
	}
	path := clineProjectComponents(req.ProjectRoot).ConfigPaths.JSON
	file, err := env.NativeConfig.ReadExactFile(path)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	if !file.Exists || len(file.Body) > 1<<20 {
		return clients.SessionPlan{}, fmt.Errorf("owned Cline project MCP configuration is missing or oversized")
	}
	names, err := adapter.sessionServers(env, req)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	after, err := env.NativeConfig.ReadExactFile(path)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	if !after.Exists || after.Mode != file.Mode || !bytes.Equal(after.Body, file.Body) {
		return clients.SessionPlan{}, fmt.Errorf("Cline project MCP source changed during verification")
	}
	if err := exactSessionServers(file.Body, names); err != nil {
		return clients.SessionPlan{}, err
	}
	args := []string{"--auto-approve", "false", "--cwd", req.ProjectRoot}
	if req.Inspect {
		args = append(args, "config", "mcp", "--json")
	}
	return clients.SessionPlan{
		ClientID: domain.ClientCline, Executable: executable, Arguments: args,
		Directory: req.ProjectRoot, ConfigDigest: fmt.Sprintf("%x", sha256.Sum256(file.Body)), Servers: names,
		Environment: map[string]string{"CLINE_MCP_SETTINGS_PATH": path, "CLINE_NO_AUTO_UPDATE": "1"},
	}, nil
}

func (adapter *Adapter) sessionServers(env clients.Env, req clients.SessionRequest) ([]string, error) {
	layout := clineProjectComponents(req.ProjectRoot)
	selected := map[string]bool{}
	for _, binding := range req.Bindings {
		if binding.Scope != "project" {
			return nil, fmt.Errorf("Cline session requires Project scope")
		}
		if err := adapter.ValidateBindingProfile(req.ProjectRoot, binding); err != nil {
			return nil, err
		}
		objects := clineProjectOwned(binding.NativeObjects)
		if err := layout.Verify(objects, false, env.NativeConfig); err != nil {
			return nil, err
		}
		for _, object := range objects {
			if object.Kind != clineProjectMCPKind {
				continue
			}
			if selected[object.LogicalName] {
				return nil, fmt.Errorf("ambiguous Cline project MCP ownership")
			}
			selected[object.LogicalName] = true
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no owned Cline Project MCP servers; ordinary Skills startup needs no override")
	}
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func exactSessionServers(body []byte, names []string) error {
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok || len(document) != 1 || len(servers) != len(names) {
		return fmt.Errorf("Cline session source contains unowned or unsupported configuration")
	}
	for _, name := range names {
		server, ok := servers[name].(map[string]any)
		if !ok {
			return fmt.Errorf("Cline owned server is missing")
		}
		if disabled, present := server["disabled"]; present && disabled != false {
			return fmt.Errorf("Cline server %q was disabled; startup does not override that choice", name)
		}
		// Starting the client is not consent to automatically approve MCP
		// tool calls. Keep this separate from the CLI's --auto-approve flag.
		if configured, present := server["autoApprove"]; present {
			approvals, ok := configured.([]any)
			if !ok || len(approvals) != 0 {
				return fmt.Errorf("Cline server %q requests automatic MCP tool approval", name)
			}
		}
	}
	return nil
}
