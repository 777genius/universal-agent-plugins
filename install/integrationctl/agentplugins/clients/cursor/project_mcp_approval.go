package cursor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Pinned native stdio schema order is significant to Cursor's JSON.stringify
// fingerprint. Unknown fields are stripped by its Zod schema, not approved.
type cursorApprovalServer struct {
	Type    *string         `json:"type,omitempty"`
	Command string          `json:"command"`
	Args    *[]string       `json:"args,omitempty"`
	Env     json.RawMessage `json:"env,omitempty"`
	CWD     *string         `json:"cwd,omitempty"`
}

func approveCursorMCP(ctx context.Context, env clients.Env, request domain.ActivationRequest, name string) error {
	if err := cursorMCPPolicyAvailable(env, request, []string{name}); err != nil {
		return err
	}
	version, err := runCursorNative(ctx, env, request, "--version")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version.Stdout)) != "2026.09.18-9a7762b" {
		return fmt.Errorf("cursor MCP approval format is not qualified for this agent version")
	}
	key, err := cursorOwnedApprovalKey(env, request, name)
	if err != nil {
		return err
	}
	_, err = shared.UpdateProjectTrustFileWithInitial(env.NativeConfig, cursorMCPPolicyPath(request, "mcp-approvals.json"), []byte("[]"), func(body []byte) ([]byte, error) {
		if err := cursorMCPPolicyAvailable(env, request, []string{name}); err != nil {
			return nil, err
		}
		if err := cursorComponents(request.Plan.NativeRegistryRoot).Verify(cursorNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig); err != nil {
			return nil, err
		}
		// Recompute beneath the approval writer lock. An edited native config can
		// never inherit an approval derived from the previously observed projection.
		current, err := cursorOwnedApprovalKey(env, request, name)
		if err != nil || current != key {
			return nil, fmt.Errorf("cursor selected MCP configuration changed before approval")
		}
		return cursorAppendApproval(body, key)
	})
	if err != nil {
		return err
	}
	return cursorMCPPolicyAvailable(env, request, []string{name})
}

func cursorAppendApproval(body []byte, key string) ([]byte, error) {
	entries := make([]string, 0, 1)
	if len(body) > 1<<20 || json.Unmarshal(body, &entries) != nil || !bytes.HasPrefix(bytes.TrimSpace(body), []byte("[")) {
		return nil, fmt.Errorf("preserved malformed Cursor MCP approval policy")
	}
	for _, entry := range entries {
		if entry == key {
			return body, nil
		}
	}
	return json.Marshal(append(entries, key))
}

func cursorOwnedApprovalKey(env clients.Env, request domain.ActivationRequest, name string) (string, error) {
	file, err := env.NativeConfig.ReadExactFile(filepath.Join(request.Plan.NativeRegistryRoot, "mcp.json"))
	if err != nil {
		return "", err
	}
	if !file.Exists || len(file.Body) > 1<<20 {
		return "", fmt.Errorf("cursor selected MCP configuration is unavailable")
	}
	document, err := shared.DecodeStrictJSONObject(file.Body)
	if err != nil {
		return "", err
	}
	entries, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("cursor selected MCP collection is unavailable")
	}
	entry, ok := entries[name].(map[string]any)
	if !ok {
		return "", fmt.Errorf("cursor selected MCP server is unavailable")
	}
	// Native recursively interpolates ${...} before hashing. Do not approximate
	// an environment-dependent identity or read credentials to expand it.
	if cursorHasInterpolation(entry) {
		return "", fmt.Errorf("cursor MCP approval requires fully materialized configuration")
	}
	var raw struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(file.Body, &raw); err != nil {
		return "", err
	}
	server, err := cursorNormalizeApprovalServer(raw.Servers[name])
	if err != nil {
		return "", err
	}
	payload, err := cursorJSONStringify(struct {
		Path   string               `json:"path"`
		Server cursorApprovalServer `json:"server"`
	}{request.Plan.TargetContext.ProjectRoot, server})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return name + "-" + hex.EncodeToString(digest[:8]), nil
}

func cursorHasInterpolation(value any) bool {
	switch value := value.(type) {
	case string:
		return strings.Contains(value, "${")
	case []any:
		for _, child := range value {
			if cursorHasInterpolation(child) {
				return true
			}
		}
	case map[string]any:
		for _, child := range value {
			if cursorHasInterpolation(child) {
				return true
			}
		}
	}
	return false
}

func cursorNormalizeApprovalServer(body []byte) (cursorApprovalServer, error) {
	server := cursorApprovalServer{}
	if err := json.Unmarshal(body, &server); err != nil {
		return server, err
	}
	if server.Command == "" || server.Type != nil && *server.Type != "stdio" {
		return server, fmt.Errorf("cursor approval requires an exact stdio server")
	}
	if server.Env != nil {
		environment, err := hujson.Parse(server.Env)
		if err != nil {
			return server, err
		}
		object, ok := environment.Value.(*hujson.Object)
		if !ok {
			return server, fmt.Errorf("cursor MCP environment must be an object")
		}
		compact := bytes.NewBufferString("{")
		for i, member := range object.Members {
			var key, value string
			if json.Unmarshal(member.Name.Pack(), &key) != nil || json.Unmarshal(member.Value.Pack(), &value) != nil {
				return server, fmt.Errorf("cursor MCP environment requires string entries")
			}
			// Integer-like keys have special JS property ordering and are not native
			// process environment names; refuse rather than approve another ordering.
			if key == "" || key[0] >= '0' && key[0] <= '9' {
				return server, fmt.Errorf("cursor MCP environment name is unsupported")
			}
			keyJSON, _ := cursorJSONStringify(key)
			valueJSON, _ := cursorJSONStringify(value)
			if i > 0 {
				compact.WriteByte(',')
			}
			compact.Write(keyJSON)
			compact.WriteByte(':')
			compact.Write(valueJSON)
		}
		compact.WriteByte('}')
		server.Env = compact.Bytes()
	}
	return server, nil
}

func cursorJSONStringify(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	body := bytes.TrimSuffix(output.Bytes(), []byte("\n"))
	// Go escapes these separators even with HTML escaping off; JSON.stringify
	// preserves their UTF-8 bytes. This is needed for literal native arguments.
	return cursorJSONSeparators(body), nil
}

func cursorJSONSeparators(body []byte) []byte {
	var output bytes.Buffer
	for i := 0; i < len(body); {
		if body[i] != '\\' {
			output.WriteByte(body[i])
			i++
			continue
		}
		end := i
		for end < len(body) && body[end] == '\\' {
			end++
		}
		if (end-i)%2 == 1 && end+5 <= len(body) && (string(body[end:end+5]) == "u2028" || string(body[end:end+5]) == "u2029") {
			output.Write(body[i : end-1])
			if body[end+4] == '8' {
				output.WriteString("\u2028")
			} else {
				output.WriteString("\u2029")
			}
			i = end + 5
		} else {
			output.Write(body[i:end])
			i = end
		}
	}
	return output.Bytes()
}
