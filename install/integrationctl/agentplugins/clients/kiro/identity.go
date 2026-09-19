package kiro

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }

func (*Adapter) InspectNativeRegistry(ctx context.Context, _ clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	return inspectKiroRegistry(plan, managed)
}

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
			if err := verifyKiroSkillDigest(object, allowMissing); err != nil {
				return err
			}
		case kiroMCPObjectKind:
			if err := verifyKiroMCPDigest(mcp, object, allowMissing); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyKiroSkillDigest(object domain.NativeObjectOwnership, allowMissing bool) error {
	digest, err := shared.DigestSkillDirectory(object.Path)
	if os.IsNotExist(err) && allowMissing {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect managed Kiro skill %q: %w", object.LogicalName, err)
	}
	if digest != object.ManagedDigest {
		return fmt.Errorf("managed Kiro skill %q changed outside agentplugins", object.LogicalName)
	}
	return nil
}

func verifyKiroMCPDigest(mcp map[string]any, object domain.NativeObjectOwnership, allowMissing bool) error {
	rawServer, exists := mcp[object.LogicalName]
	if !exists && allowMissing {
		return nil
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
	return nil
}

func requireKiroObjectAbsent(configRoot string, object domain.NativeObjectOwnership) error {
	if err := validateKiroObject(configRoot, object); err != nil {
		return err
	}
	switch object.Kind {
	case kiroSkillObjectKind:
		if _, err := os.Lstat(object.Path); err == nil {
			return kiroErrorf("Kiro skill %q already exists without agentplugins ownership", object.LogicalName)
		} else if !os.IsNotExist(err) {
			return err
		}
	case kiroMCPObjectKind:
		mcp, _, _, _, err := ReadMCPConfig(object.Path)
		if err != nil {
			return err
		}
		if _, exists := mcp[object.LogicalName]; exists {
			return kiroErrorf("Kiro MCP server %q already exists without agentplugins ownership", object.LogicalName)
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
		return kiroErrorf("Kiro native object %q has an untrusted path", object.LogicalName)
	}
	return ValidateNativePath(configRoot, object.Path)
}

func ValidateNativePath(configRoot, path string) error {
	if err := pathpolicy.RequireContainedChild(configRoot, path); err != nil {
		return fmt.Errorf("unsafe Kiro native path: %w", err)
	}
	return nil
}

func inspectKiroRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return clients.RegistryIndeterminate, nil
	}
	if _, err := os.Lstat(root); os.IsNotExist(err) {
		return clients.RegistryClear, nil
	} else if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if managed != nil {
		if err := VerifyNativeObjects(root, managed.NativeObjects, true); err != nil {
			return clients.RegistryIndeterminate, err
		}
	}
	mcp, err := kiroRegistryMCP(root, plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	return kiroRegistryComponents(root, plan.Components, managed, mcp)
}

func kiroRegistryMCP(root string, plan domain.DeliveryPlan) (map[string]any, error) {
	if !shared.HasSupportedMCP(plan.Components) {
		return map[string]any{}, nil
	}
	mcpPath := filepath.Join(root, "settings", "mcp.json")
	if err := ValidateNativePath(root, mcpPath); err != nil {
		return nil, err
	}
	mcp, _, _, _, err := ReadMCPConfig(mcpPath)
	return mcp, err
}

func kiroRegistryComponents(root string, components []domain.ComponentDecision, managed *domain.ClientBinding, mcp map[string]any) (clients.RegistryFinding, error) {
	finding := clients.RegistryClear
	for _, component := range components {
		exists, err := kiroComponentExists(root, component, mcp)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if !exists {
			continue
		}
		if managed == nil {
			return clients.RegistryCollision, nil
		}
		if !managedKiroObjectExists(managed.NativeObjects, component.Kind, component.Name) {
			return clients.RegistryIndeterminate, nil
		}
		finding = clients.RegistryExpected
	}
	return finding, nil
}

func kiroComponentExists(root string, component domain.ComponentDecision, mcp map[string]any) (bool, error) {
	if component.Support == domain.SupportUnsupported {
		return false, nil
	}
	switch component.Kind {
	case domain.ComponentSkill:
		skillPath := filepath.Join(root, "skills", component.Name)
		if err := ValidateNativePath(root, skillPath); err != nil {
			return false, err
		}
		_, statErr := os.Lstat(skillPath)
		if statErr != nil && !os.IsNotExist(statErr) {
			return false, statErr
		}
		return statErr == nil, nil
	case domain.ComponentMCPServer:
		_, exists := mcp[component.Name]
		return exists, nil
	default:
		return false, nil
	}
}

func managedKiroObjectExists(objects []domain.NativeObjectOwnership, kind domain.ComponentKind, name string) bool {
	want := SkillObjectKind
	if kind == domain.ComponentMCPServer {
		want = MCPObjectKind
	}
	for _, object := range objects {
		if object.Kind == want && object.LogicalName == name {
			return true
		}
	}
	return false
}
