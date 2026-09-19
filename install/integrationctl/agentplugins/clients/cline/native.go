package cline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	ClineSkillObjectKind = "cline_global_skill_directory"
	ClineMCPObjectKind   = "cline_global_mcp_server"
	ClineProjectionFile  = ".agentplugins-cline-native.json"
)

type ClineProjection struct {
	Servers map[string]nativeconfig.Server `json:"servers"`
}

func ActivateClineNativeWithKernel(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyClineNativeMutationWithKernel(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel)
}

func DeactivateClineNativeWithKernel(ctx context.Context, request domain.DeactivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyClineNativeMutationWithKernel(request.Client.ConfigRoot, "", request.NativeObjects, nil, kernel)
}

type clineRenameFunc func(string, string) error

func RenameClineDirectoryNoReplace(oldPath, newPath string, rename clineRenameFunc) error {
	return rename(oldPath, newPath)
}

func ApplyClineNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyClineNativeMutationWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func ApplyClineNativeMutationWithRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename clineRenameFunc) (resultErr error) {
	return applyClineNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, nativeconfig.New(), rename)
}

func applyClineNativeMutationWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	return applyClineNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, kernel, shared.RenameDirectoryExclusive)
}

func applyClineNativeMutationWithKernelAndRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename clineRenameFunc) (resultErr error) {
	return ApplyClineNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, previous, desired, kernel, rename, shared.CheckedCombinedCapacity)
}

func VerifyClineNativeObjects(configRoot string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	kernel := nativeconfig.New()
	for _, object := range ClineObjects(objects) {
		if err := validateClineObject(configRoot, object); err != nil {
			return err
		}
		switch object.Kind {
		case ClineSkillObjectKind:
			digest, err := shared.DigestSkillDirectory(object.Path)
			if os.IsNotExist(err) && allowMissing {
				continue
			}
			if err != nil || digest != object.ManagedDigest {
				return fmt.Errorf("managed Cline skill %q changed outside agentplugins", object.LogicalName)
			}
		case ClineMCPObjectKind:
			if err := verifyClineMCPObject(kernel, object, allowMissing); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyClineMCPObject(kernel nativeconfig.Kernel, object domain.NativeObjectOwnership, allowMissing bool) error {
	receipt := clineReceipt(object)
	present, owned, err := kernel.Inspect(nativeconfig.Paths{JSON: object.Path}, nativeconfig.CodecCline, object.LogicalName, &receipt)
	if err != nil {
		return fmt.Errorf("inspect managed Cline MCP server %q: %w", object.LogicalName, err)
	}
	if !present && allowMissing {
		return nil
	}
	if !present || !owned {
		return fmt.Errorf("managed Cline MCP server %q changed outside agentplugins: %w", object.LogicalName, nativeconfig.ErrNotOwned)
	}
	return nil
}

func ClineMCPSettingsPath(configRoot string) string {
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

func ClineObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == ClineSkillObjectKind || object.Kind == ClineMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func clineSkillObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == ClineSkillObjectKind {
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
	case ClineSkillObjectKind:
		if err := pathpolicy.RequireContainedChild(filepath.Join(configRoot, "skills"), object.Path); err != nil {
			return fmt.Errorf("unsafe Cline skill path: %w", err)
		}
	case ClineMCPObjectKind:
		if !filepath.IsAbs(object.Path) || !shared.SameCleanPath(object.Path, ClineMCPSettingsPath(configRoot)) {
			return fmt.Errorf("the Cline MCP ownership path changed")
		}
	default:
		return fmt.Errorf("unsupported Cline native object kind %q", object.Kind)
	}
	return nil
}

func hasClineMCP(objects map[string]domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == ClineMCPObjectKind {
			return true
		}
	}
	return false
}

func sameClineMCPObject(left, right domain.NativeObjectOwnership) bool {
	return left.ObjectID == right.ObjectID && left.Kind == ClineMCPObjectKind && right.Kind == ClineMCPObjectKind &&
		left.LogicalName == right.LogicalName && shared.SameCleanPath(left.Path, right.Path) && left.ManagedDigest == right.ManagedDigest
}
