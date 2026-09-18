package cline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func VerifyNativeObjects(configRoot string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	kernel := nativeconfig.New()
	for _, object := range NativeObjects(objects) {
		if err := validateClineObject(configRoot, object); err != nil {
			return err
		}
		switch object.Kind {
		case clineSkillObjectKind:
			digest, err := shared.DigestSkillDirectory(object.Path)
			if os.IsNotExist(err) && allowMissing {
				continue
			}
			if err != nil || digest != object.ManagedDigest {
				return fmt.Errorf("managed Cline skill %q changed outside agentplugins", object.LogicalName)
			}
		case clineMCPObjectKind:
			receipt := clineReceipt(object)
			present, owned, err := kernel.Inspect(nativeconfig.Paths{JSON: object.Path}, nativeconfig.CodecCline, object.LogicalName, &receipt)
			if err != nil {
				return fmt.Errorf("inspect managed Cline MCP server %q: %w", object.LogicalName, err)
			}
			if !present && allowMissing {
				continue
			}
			if !present || !owned {
				return fmt.Errorf("managed Cline MCP server %q changed outside agentplugins: %w", object.LogicalName, nativeconfig.ErrNotOwned)
			}
		}
	}
	return nil
}

func sameClineMCPObject(left, right domain.NativeObjectOwnership) bool {
	return left.ObjectID == right.ObjectID && left.Kind == clineMCPObjectKind && right.Kind == clineMCPObjectKind &&
		left.LogicalName == right.LogicalName && shared.SameCleanPath(left.Path, right.Path) && left.ManagedDigest == right.ManagedDigest
}

func clineMCPSettingsPath(configRoot string) string {
	if path := strings.TrimSpace(os.Getenv("CLINE_MCP_SETTINGS_PATH")); path != "" {
		return filepath.Clean(path)
	}
	if data := strings.TrimSpace(os.Getenv("CLINE_DATA_DIR")); data != "" {
		return filepath.Join(filepath.Clean(data), "settings", "cline_mcp_settings.json")
	}
	return filepath.Join(configRoot, "data", "settings", "cline_mcp_settings.json")
}

func clineReceipt(object domain.NativeObjectOwnership) nativeconfig.Receipt {
	return nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecCline, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func NativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == clineSkillObjectKind || object.Kind == clineMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func clineSkillObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == clineSkillObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func validateClineObject(configRoot string, object domain.NativeObjectOwnership) error {
	if object.ProtectionClass != "managed" || object.ObjectID == "" || object.LogicalName == "" || object.ManagedDigest == "" {
		return fmt.Errorf("invalid Cline native ownership object")
	}
	switch object.Kind {
	case clineSkillObjectKind:
		if err := pathpolicy.RequireContainedChild(filepath.Join(configRoot, "skills"), object.Path); err != nil {
			return fmt.Errorf("unsafe Cline skill path: %w", err)
		}
	case clineMCPObjectKind:
		if !filepath.IsAbs(object.Path) || !shared.SameCleanPath(object.Path, clineMCPSettingsPath(configRoot)) {
			return fmt.Errorf("the Cline MCP ownership path changed")
		}
	default:
		return fmt.Errorf("unsupported Cline native object kind %q", object.Kind)
	}
	return nil
}

func hasClineMCP(objects map[string]domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == clineMCPObjectKind {
			return true
		}
	}
	return false
}
