package gemini

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
	for _, object := range NativeObjects(objects) {
		if err := validateGeminiObject(configRoot, object); err != nil {
			return err
		}
		switch object.Kind {
		case geminiSkillObjectKind:
			if err := verifyGeminiSkill(object, allowMissing); err != nil {
				return err
			}
		case geminiMCPObjectKind:
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
	present, owned, err := nativeconfig.New().Inspect(geminiConfigPaths(configRoot), nativeconfig.CodecGemini, object.LogicalName, geminiReceipt(object))
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

func InspectRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return registryIndeterminate, nil
	}
	if managed != nil {
		if err := VerifyNativeObjects(root, managed.NativeObjects, true); err != nil {
			return registryIndeterminate, err
		}
	}
	finding := registryClear
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		status, err := inspectGeminiComponent(root, managed, component)
		if err != nil {
			return registryIndeterminate, err
		}
		if status.exists && !status.owned {
			return registryCollision, nil
		}
		if status.exists && status.owned {
			finding = registryExpected
		}
	}
	return finding, nil
}

type geminiComponentStatus struct{ exists, owned bool }

func inspectGeminiComponent(root string, managed *domain.ClientBinding, component domain.ComponentDecision) (geminiComponentStatus, error) {
	switch component.Kind {
	case domain.ComponentSkill:
		path := filepath.Join(root, "skills", component.Name)
		_, err := os.Lstat(path)
		if err != nil && !os.IsNotExist(err) {
			return geminiComponentStatus{}, err
		}
		return geminiComponentStatus{
			exists: err == nil,
			owned:  managed != nil && managedGeminiObjectExists(managed.NativeObjects, geminiSkillObjectKind, component.Name),
		}, nil
	case domain.ComponentMCPServer:
		var receipt *nativeconfig.Receipt
		if managed != nil {
			for _, object := range NativeObjects(managed.NativeObjects) {
				if object.Kind == geminiMCPObjectKind && object.LogicalName == component.Name {
					receipt = geminiReceipt(object)
				}
			}
		}
		exists, owned, err := nativeconfig.New().Inspect(geminiConfigPaths(root), nativeconfig.CodecGemini, component.Name, receipt)
		if err != nil {
			return geminiComponentStatus{}, err
		}
		return geminiComponentStatus{exists: exists, owned: owned}, nil
	default:
		return geminiComponentStatus{}, nil
	}
}

func geminiReceipt(object domain.NativeObjectOwnership) *nativeconfig.Receipt {
	return &nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecGemini, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func geminiConfigPaths(root string) nativeconfig.Paths {
	return nativeconfig.Paths{JSON: filepath.Join(root, "settings.json")}
}

func NativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind == geminiSkillObjectKind || object.Kind == geminiMCPObjectKind {
			result = append(result, object)
		}
	}
	return result
}

func hasGeminiSkillObjects(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == geminiSkillObjectKind {
			return true
		}
	}
	return false
}

func managedGeminiObjectExists(objects []domain.NativeObjectOwnership, kind, name string) bool {
	for _, object := range NativeObjects(objects) {
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
	if object.Kind == geminiSkillObjectKind {
		if _, err := os.Lstat(object.Path); err == nil {
			return fmt.Errorf("the Gemini skill %q already exists without agentplugins ownership", object.LogicalName)
		} else if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	present, _, err := nativeconfig.New().Inspect(geminiConfigPaths(root), nativeconfig.CodecGemini, object.LogicalName, nil)
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
	if object.Kind == geminiSkillObjectKind {
		expected = filepath.Join(root, "skills", object.LogicalName)
	} else if object.Kind != geminiMCPObjectKind {
		return fmt.Errorf("unsupported Gemini native object kind %q", object.Kind)
	}
	if !shared.SameCleanPath(expected, object.Path) {
		return fmt.Errorf("the Gemini native object %q has an untrusted path", object.LogicalName)
	}
	return pathpolicy.RequireContainedChild(root, object.Path)
}
