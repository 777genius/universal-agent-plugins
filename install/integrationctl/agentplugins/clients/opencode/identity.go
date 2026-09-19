package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	return InspectOpenCodeRegistry(plan, managed, env.NativeConfig)
}

func InspectOpenCodeRegistry(plan domain.DeliveryPlan, managed *domain.ClientBinding, kernel nativeconfig.Kernel) (clients.RegistryFinding, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" {
		return clients.RegistryIndeterminate, nil
	}
	if managed != nil {
		if err := VerifyOpenCodeNativeObjects(root, plan.ActivePath, managed.NativeObjects, kernel, true); err != nil {
			return clients.RegistryIndeterminate, err
		}
	}
	finding := clients.RegistryClear
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		exists, owned, err := inspectOpenCodeComponent(root, managed, component, kernel)
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

func inspectOpenCodeComponent(root string, managed *domain.ClientBinding, component domain.ComponentDecision, kernel nativeconfig.Kernel) (bool, bool, error) {
	switch component.Kind {
	case domain.ComponentSkill:
		return inspectOpenCodeSkillComponent(root, managed, component.Name)
	case domain.ComponentMCPServer:
		return inspectOpenCodeMCPComponent(root, managed, component.Name, kernel)
	default:
		return false, false, nil
	}
}

func inspectOpenCodeSkillComponent(root string, managed *domain.ClientBinding, name string) (bool, bool, error) {
	if err := pathpolicy.ValidateLeafID(name); err != nil {
		return false, false, err
	}
	path := filepath.Join(root, "skills", name)
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return false, false, err
	}
	_, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return false, false, err
	}
	owned := managed != nil && managedOpenCodeObjectExists(managed.NativeObjects, openCodeSkillKind, name)
	return err == nil, owned, nil
}

func inspectOpenCodeMCPComponent(root string, managed *domain.ClientBinding, name string, kernel nativeconfig.Kernel) (bool, bool, error) {
	paths := nativeconfig.Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	if err := pathpolicy.RequireContainedChild(root, paths.JSON); err != nil {
		return false, false, err
	}
	if err := pathpolicy.RequireContainedChild(root, paths.JSONC); err != nil {
		return false, false, err
	}
	var receipt *nativeconfig.Receipt
	if managed != nil {
		for _, object := range OpenCodeObjects(managed.NativeObjects) {
			if object.Kind == OpenCodeMCPObjectKind && object.LogicalName == name {
				owned := receiptFromOpenCodeObject(object)
				receipt = &owned
			}
		}
	}
	return kernel.Inspect(paths, nativeconfig.CodecOpenCode, name, receipt)
}

func managedOpenCodeObjectExists(objects []domain.NativeObjectOwnership, kind, name string) bool {
	for _, object := range OpenCodeObjects(objects) {
		if object.Kind == kind && object.LogicalName == name {
			return true
		}
	}
	return false
}
