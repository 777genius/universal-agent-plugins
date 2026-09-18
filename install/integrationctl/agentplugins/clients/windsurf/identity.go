package windsurf

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func VerifyNativeObjects(configRoot, activePath string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	objectMap, err := windsurfObjectMap(configRoot, objects)
	if err != nil {
		return err
	}
	if len(objectMap) == 0 {
		return nil
	}
	servers, err := readProjectedWindsurfServers(activePath)
	if err != nil {
		return err
	}
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return err
	}
	kernel := nativeconfig.New()
	for name, object := range objectMap {
		server, ok := servers[name]
		if !ok {
			return fmt.Errorf("prepared Windsurf MCP server %q is missing", name)
		}
		receipt := windsurfReceipt(object)
		present, owned, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, name, &receipt)
		if inspectErr != nil {
			return fmt.Errorf("verify Windsurf MCP entry %q: %w", name, inspectErr)
		}
		if !present && allowMissing {
			continue
		}
		if !present || !owned {
			return fmt.Errorf("verify Windsurf MCP entry %q: %w", name, nativeconfig.ErrNotOwned)
		}
		preview, previewErr := desiredWindsurfReceipt(configPath, name, server)
		if previewErr != nil || preview.Digest != object.ManagedDigest {
			return fmt.Errorf("verify Windsurf MCP entry %q: desired digest drifted", name)
		}
	}
	return nil
}

func InspectRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	if strings.TrimSpace(plan.NativeRegistryRoot) == "" {
		return registryClear, nil
	}
	configPath, err := windsurfConfigPath(plan.NativeRegistryRoot)
	if err != nil {
		return registryIndeterminate, err
	}
	if managed != nil {
		if err := VerifyNativeObjects(plan.NativeRegistryRoot, plan.ActivePath, managed.NativeObjects, false); err != nil {
			return registryIndeterminate, err
		}
		if len(NativeObjects(managed.NativeObjects)) > 0 {
			return registryExpected, nil
		}
	}
	kernel := nativeconfig.New()
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentMCPServer || component.Support == domain.SupportUnsupported {
			continue
		}
		present, _, inspectErr := kernel.Inspect(nativeconfig.Paths{JSON: configPath}, nativeconfig.CodecWindsurf, component.Name, nil)
		if present {
			return registryCollision, nil
		}
		if inspectErr != nil {
			return registryIndeterminate, inspectErr
		}
	}
	return registryClear, nil
}

func windsurfConfigPath(configRoot string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(configRoot))
	if root == "." || !filepath.IsAbs(root) {
		return "", fmt.Errorf("the Windsurf channel config root must be absolute")
	}
	path := filepath.Join(root, "mcp_config.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return "", fmt.Errorf("unsafe Windsurf MCP config path: %w", err)
	}
	return path, nil
}

func windsurfObjectMap(configRoot string, objects []domain.NativeObjectOwnership) (map[string]domain.NativeObjectOwnership, error) {
	configPath, err := windsurfConfigPath(configRoot)
	if err != nil {
		return nil, err
	}
	result := map[string]domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == "managed_package_directory" {
			continue
		}
		if object.Kind != windsurfMCPObjectKind || object.LogicalName == "" || object.ManagedDigest == "" || object.ProtectionClass != "managed_entry" || filepath.Clean(object.Path) != configPath {
			return nil, fmt.Errorf("invalid Windsurf native ownership object %q", object.ObjectID)
		}
		if _, exists := result[object.LogicalName]; exists {
			return nil, fmt.Errorf("duplicate Windsurf native ownership for %q", object.LogicalName)
		}
		result[object.LogicalName] = object
	}
	return result, nil
}

func NativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == windsurfMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func windsurfReceipt(object domain.NativeObjectOwnership) nativeconfig.Receipt {
	return nativeconfig.Receipt{Version: "1", Path: filepath.Clean(object.Path), Codec: nativeconfig.CodecWindsurf, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func desiredWindsurfReceipt(configPath, name string, server nativeconfig.Server) (nativeconfig.Receipt, error) {
	return nativeconfig.DesiredReceipt(configPath, nativeconfig.CodecWindsurf, name, server, nativeconfig.Placeholders{})
}
