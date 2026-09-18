package kiro

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func VerifyNativeObjects(configRoot string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	filtered := NativeObjects(objects)
	for _, object := range filtered {
		if err := validateKiroObject(configRoot, object); err != nil {
			return err
		}
	}
	mcp := map[string]any{}
	if len(previousMCPObjects(filtered)) > 0 {
		var err error
		mcp, _, _, _, err = ReadMCPConfig(filepath.Join(configRoot, "settings", "mcp.json"))
		if err != nil {
			return err
		}
	}
	for _, object := range filtered {
		switch object.Kind {
		case kiroSkillObjectKind:
			digest, err := shared.DigestSkillDirectory(object.Path)
			if os.IsNotExist(err) && allowMissing {
				continue
			}
			if err != nil {
				return fmt.Errorf("inspect managed Kiro skill %q: %w", object.LogicalName, err)
			}
			if digest != object.ManagedDigest {
				return fmt.Errorf("managed Kiro skill %q changed outside agentplugins", object.LogicalName)
			}
		case kiroMCPObjectKind:
			rawServer, exists := mcp[object.LogicalName]
			if !exists && allowMissing {
				continue
			}
			if !exists {
				return fmt.Errorf("managed Kiro MCP server %q is missing", object.LogicalName)
			}
			server, ok := rawServer.(map[string]any)
			if !ok {
				return fmt.Errorf("managed Kiro MCP server %q is malformed", object.LogicalName)
			}
			if shared.DigestJSONObject(server) != object.ManagedDigest {
				return fmt.Errorf("managed Kiro MCP server %q changed outside agentplugins", object.LogicalName)
			}
		}
	}
	return nil
}

func requireKiroObjectAbsent(configRoot string, object domain.NativeObjectOwnership) error {
	if err := validateKiroObject(configRoot, object); err != nil {
		return err
	}
	switch object.Kind {
	case kiroSkillObjectKind:
		if _, err := os.Lstat(object.Path); err == nil {
			return fmt.Errorf("Kiro skill %q already exists without agentplugins ownership", object.LogicalName)
		} else if !os.IsNotExist(err) {
			return err
		}
	case kiroMCPObjectKind:
		mcp, _, _, _, err := ReadMCPConfig(object.Path)
		if err != nil {
			return err
		}
		if _, exists := mcp[object.LogicalName]; exists {
			return fmt.Errorf("Kiro MCP server %q already exists without agentplugins ownership", object.LogicalName)
		}
	}
	return nil
}

func validateKiroObject(configRoot string, object domain.NativeObjectOwnership) error {
	if err := pathpolicy.ValidateLeafID(object.LogicalName); err != nil {
		return fmt.Errorf("invalid Kiro native object name %q: %w", object.LogicalName, err)
	}
	var expected string
	switch object.Kind {
	case kiroSkillObjectKind:
		expected = filepath.Join(configRoot, "skills", object.LogicalName)
	case kiroMCPObjectKind:
		expected = filepath.Join(configRoot, "settings", "mcp.json")
	default:
		return fmt.Errorf("unsupported Kiro native object kind %q", object.Kind)
	}
	if !shared.SameCleanPath(expected, object.Path) {
		return fmt.Errorf("Kiro native object %q has an untrusted path", object.LogicalName)
	}
	return ValidateNativePath(configRoot, object.Path)
}

func ValidateNativePath(configRoot, path string) error {
	if err := pathpolicy.RequireContainedChild(configRoot, path); err != nil {
		return fmt.Errorf("unsafe Kiro native path: %w", err)
	}
	return nil
}
