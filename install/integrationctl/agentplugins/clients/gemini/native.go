package gemini

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	GeminiSkillObjectKind = "gemini_global_skill_directory"
	GeminiMCPObjectKind   = "gemini_global_mcp_server"
	GeminiDescriptorName  = ".agentplugins-gemini.json"
)

type GeminiDescriptor struct {
	DataRoot string `json:"data_root"`
}

func ActivateGeminiNativeWithKernel(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNativeMutationWithKernel(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel)
}

func DeactivateGeminiNativeWithKernel(ctx context.Context, request domain.DeactivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNativeMutationWithKernel(request.Client.ConfigRoot, "", request.NativeObjects, nil, kernel)
}

func VerifyGeminiNativeObjects(configRoot string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	for _, object := range GeminiObjects(objects) {
		if err := validateGeminiObject(configRoot, object); err != nil {
			return err
		}
		switch object.Kind {
		case GeminiSkillObjectKind:
			if err := verifyGeminiSkill(object, allowMissing); err != nil {
				return err
			}
		case GeminiMCPObjectKind:
			if err := verifyGeminiMCP(configRoot, object, allowMissing); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyGeminiSkill(object domain.NativeObjectOwnership, allowMissing bool) error {
	digest, err := shared.DigestSkillDirectory(object.Path)
	if os.IsNotExist(err) && allowMissing {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect managed Gemini skill %q: %w", object.LogicalName, err)
	}
	if digest != object.ManagedDigest {
		return fmt.Errorf("managed Gemini skill %q changed outside agentplugins", object.LogicalName)
	}
	return nil
}

func verifyGeminiMCP(configRoot string, object domain.NativeObjectOwnership, allowMissing bool) error {
	present, owned, err := nativeconfig.New().Inspect(GeminiConfigPaths(configRoot), nativeconfig.CodecGemini, object.LogicalName, GeminiReceipt(object))
	if err != nil {
		return err
	}
	if !present && allowMissing {
		return nil
	}
	if !present || !owned {
		return fmt.Errorf("managed Gemini MCP server %q changed outside agentplugins", object.LogicalName)
	}
	return nil
}

type geminiRenameFunc func(string, string) error

func RenameGeminiDirectoryNoReplace(oldPath, newPath string, rename geminiRenameFunc) error {
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", newPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return rename(oldPath, newPath)
}

func ApplyGeminiNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyGeminiNativeMutationWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func ApplyGeminiNativeMutationWithRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename geminiRenameFunc) (resultErr error) {
	return applyGeminiNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, nativeconfig.New(), rename)
}

func applyGeminiNativeMutationWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	return applyGeminiNativeMutationWithKernelAndRename(configRoot, activePath, previous, desired, kernel, shared.RenameDirectoryExclusive)
}

func applyGeminiNativeMutationWithKernelAndRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename geminiRenameFunc) (resultErr error) {
	return ApplyGeminiNativeMutationWithKernelRenameAndCapacity(configRoot, activePath, previous, desired, kernel, rename, shared.CheckedCombinedCapacity)
}

func InspectGeminiRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return clients.RegistryIndeterminate, nil
	}
	if managed != nil {
		if err := VerifyGeminiNativeObjects(root, managed.NativeObjects, true); err != nil {
			return clients.RegistryIndeterminate, err
		}
	}
	finding := clients.RegistryClear
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		exists, owned, err := inspectGeminiComponent(root, managed, component)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if exists && !owned {
			return clients.RegistryCollision, nil
		}
		if exists && owned {
			finding = clients.RegistryExpected
		}
	}
	return finding, nil
}

func inspectGeminiComponent(root string, managed *domain.ClientBinding, component domain.ComponentDecision) (bool, bool, error) {
	switch component.Kind {
	case domain.ComponentSkill:
		return inspectGeminiSkillComponent(root, managed, component.Name)
	case domain.ComponentMCPServer:
		return inspectGeminiMCPComponent(root, managed, component.Name)
	default:
		return false, false, nil
	}
}

func inspectGeminiSkillComponent(root string, managed *domain.ClientBinding, name string) (bool, bool, error) {
	path := filepath.Join(root, "skills", name)
	_, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return false, false, err
	}
	owned := managed != nil && managedGeminiObjectExists(managed.NativeObjects, GeminiSkillObjectKind, name)
	return err == nil, owned, nil
}

func inspectGeminiMCPComponent(root string, managed *domain.ClientBinding, name string) (bool, bool, error) {
	var receipt *nativeconfig.Receipt
	if managed != nil {
		for _, object := range GeminiObjects(managed.NativeObjects) {
			if object.Kind == GeminiMCPObjectKind && object.LogicalName == name {
				receipt = GeminiReceipt(object)
			}
		}
	}
	return nativeconfig.New().Inspect(GeminiConfigPaths(root), nativeconfig.CodecGemini, name, receipt)
}

func GeminiReceipt(object domain.NativeObjectOwnership) *nativeconfig.Receipt {
	return &nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecGemini, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func GeminiConfigPaths(root string) nativeconfig.Paths {
	return nativeconfig.Paths{JSON: filepath.Join(root, "settings.json")}
}

func GeminiObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == GeminiSkillObjectKind || object.Kind == GeminiMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func hasGeminiSkillObjects(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == GeminiSkillObjectKind {
			return true
		}
	}
	return false
}

func managedGeminiObjectExists(objects []domain.NativeObjectOwnership, kind, name string) bool {
	for _, object := range GeminiObjects(objects) {
		if object.Kind == kind && object.LogicalName == name {
			return true
		}
	}
	return false
}

func requireGeminiObjectAbsent(root string, object domain.NativeObjectOwnership) error {
	if err := validateGeminiObject(root, object); err != nil {
		return err
	}
	if object.Kind == GeminiSkillObjectKind {
		if _, err := os.Lstat(object.Path); err == nil {
			return fmt.Errorf("the Gemini skill %q already exists without agentplugins ownership", object.LogicalName)
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	present, _, err := nativeconfig.New().Inspect(GeminiConfigPaths(root), nativeconfig.CodecGemini, object.LogicalName, nil)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("the Gemini MCP server %q already exists without agentplugins ownership", object.LogicalName)
	}
	return nil
}

func validateGeminiObject(root string, object domain.NativeObjectOwnership) error {
	if err := pathpolicy.ValidateLeafID(object.LogicalName); err != nil {
		return err
	}
	expected := filepath.Join(root, "settings.json")
	if object.Kind == GeminiSkillObjectKind {
		expected = filepath.Join(root, "skills", object.LogicalName)
	} else if object.Kind != GeminiMCPObjectKind {
		return fmt.Errorf("unsupported Gemini native object kind %q", object.Kind)
	}
	if !shared.SameCleanPath(expected, object.Path) {
		return fmt.Errorf("the Gemini native object %q has an untrusted path", object.LogicalName)
	}
	return pathpolicy.RequireContainedChild(root, object.Path)
}
