package kiro

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	SkillObjectKind     = "kiro_global_skill_directory"
	MCPObjectKind       = "kiro_global_mcp_server"
	kiroSkillObjectKind = SkillObjectKind
	kiroMCPObjectKind   = MCPObjectKind
)

func ReadMCPConfig(path string) (servers map[string]any, original []byte, mode os.FileMode, exists bool, err error) {
	mode = 0o600
	body, readErr := os.ReadFile(path)
	if os.IsNotExist(readErr) {
		return map[string]any{}, nil, mode, false, nil
	}
	if readErr != nil {
		return nil, nil, mode, false, fmt.Errorf("read Kiro MCP configuration: %w", readErr)
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, mode, false, fmt.Errorf("Kiro MCP configuration must be a regular file")
	}
	document, decodeErr := shared.DecodeStrictJSONObject(body)
	if decodeErr != nil {
		return nil, nil, mode, false, fmt.Errorf("decode Kiro MCP configuration: %w", decodeErr)
	}
	if raw, present := document["mcpServers"]; present {
		servers, exists = raw.(map[string]any)
		if !exists {
			return nil, nil, mode, false, fmt.Errorf("Kiro mcpServers must be an object")
		}
	} else {
		servers = map[string]any{}
	}
	mode = info.Mode().Perm()
	return servers, body, mode, true, nil
}

func encodeKiroMCPConfig(original []byte, servers map[string]any) ([]byte, error) {
	document := map[string]any{}
	if len(original) > 0 {
		var err error
		document, err = shared.DecodeStrictJSONObject(original)
		if err != nil {
			return nil, fmt.Errorf("decode existing Kiro MCP configuration: %w", err)
		}
	}
	document["mcpServers"] = servers
	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Kiro MCP configuration: %w", err)
	}
	return append(body, '\n'), nil
}

func NativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == kiroSkillObjectKind || object.Kind == kiroMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func previousMCPObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == kiroMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func hasKiroSkillObjects(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == kiroSkillObjectKind {
			return true
		}
	}
	return false
}
