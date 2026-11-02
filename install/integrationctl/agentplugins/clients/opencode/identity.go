package opencode

import (
	"context"
	"fmt"
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
	if managed == nil && !hasPlannedOpenCodeNative(plan) {
		return clients.RegistryClear, nil
	}
	if strings.TrimSpace(plan.NativeRegistryRoot) == "" {
		return clients.RegistryIndeterminate, nil
	}
	layout, err := openCodePlanLayout(plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if err := validateOpenCodeManagedContext(layout, managed); err != nil {
		return clients.RegistryIndeterminate, err
	}
	codec, err := observedOpenCodeCodec(plan, managed)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	finding := clients.RegistryClear
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		exists, owned, err := inspectOpenCodeComponent(layout, managed, component, kernel, codec)
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

// A managed observation uses stored receipts even if the host disappeared
// or changed dialect. Without stored MCP receipts, selected desired MCP uses
// prepared authority; observing skills alone does not require a codec.
func observedOpenCodeCodec(plan domain.DeliveryPlan, managed *domain.ClientBinding) (nativeconfig.Codec, error) {
	if managed == nil {
		return DesiredOpenCodeCodec(plan.OpenCodeHost)
	}
	var stored nativeconfig.Codec
	for _, object := range OpenCodeObjects(managed.NativeObjects) {
		candidate, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
		if err != nil {
			return "", err
		}
		if !mcp {
			continue
		}
		if stored != "" && stored != candidate {
			return "", nativeconfig.ErrNativeMigrationRequired
		}
		stored = candidate
	}
	if stored != "" {
		return stored, nil
	}
	if len(domain.SelectedMCPNames(plan)) > 0 {
		return DesiredOpenCodeCodec(plan.OpenCodeHost)
	}
	return nativeconfig.CodecOpenCode, nil
}

func inspectOpenCodeComponent(layout openCodeNativeLayout, managed *domain.ClientBinding, component domain.ComponentDecision, kernel nativeconfig.Kernel, codec nativeconfig.Codec) (bool, bool, error) {
	switch component.Kind {
	case domain.ComponentSkill:
		return inspectOpenCodeSkillComponent(layout, managed, component.Name)
	case domain.ComponentMCPServer:
		return inspectOpenCodeMCPComponent(layout, managed, component.Name, kernel, codec)
	default:
		return false, false, nil
	}
}

func inspectOpenCodeSkillComponent(layout openCodeNativeLayout, managed *domain.ClientBinding, name string) (bool, bool, error) {
	if err := pathpolicy.ValidateLeafID(name); err != nil {
		return false, false, err
	}
	root := layout.Root
	path := filepath.Join(layout.Skills, name)
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

func inspectOpenCodeMCPComponent(layout openCodeNativeLayout, managed *domain.ClientBinding, name string, kernel nativeconfig.Kernel, codec nativeconfig.Codec) (bool, bool, error) {
	root := layout.Root
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
			stored, mcp, err := nativeconfig.OpenCodeCodecForKind(object.Kind)
			if err != nil {
				return false, false, err
			}
			if mcp && object.LogicalName == name {
				if err := validateOpenCodeObject(layout, OpenCodeProjection{}, object); err != nil {
					return false, false, err
				}
				codec = stored
				owned, err := receiptFromOpenCodeObject(object)
				if err != nil {
					return false, false, err
				}
				receipt = &owned
			}
		}
	}
	return kernel.Inspect(paths, codec, name, receipt)
}

func managedOpenCodeObjectExists(objects []domain.NativeObjectOwnership, kind, name string) bool {
	for _, object := range OpenCodeObjects(objects) {
		if object.Kind == kind && object.LogicalName == name {
			return true
		}
	}
	return false
}

func validateOpenCodeManagedContext(layout openCodeNativeLayout, managed *domain.ClientBinding) error {
	if managed == nil || layout.Scope != domain.ScopeProject {
		return nil
	}
	if managed.Scope != string(domain.ScopeProject) || managed.NativeProfileRoot != layout.Root {
		return fmt.Errorf("opencode ownership differs from the selected project")
	}
	for _, object := range OpenCodeObjects(managed.NativeObjects) {
		if err := validateOpenCodeObject(layout, OpenCodeProjection{}, object); err != nil {
			return err
		}
	}
	return nil
}
