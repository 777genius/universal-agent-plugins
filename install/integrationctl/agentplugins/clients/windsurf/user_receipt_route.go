package windsurf

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The recorded user binding wins over today's executable. An older Cascade
// MCP or package-only receipt must never be silently migrated to Devin Global.
func legacyUserBinding(in clients.PlanInput) bool {
	if in.TargetContext.Scope != domain.ScopeUser || !in.PreviousBindingRecorded {
		return false
	}
	if len(WindsurfObjects(in.PreviousNativeObjects)) != 0 {
		return true
	}
	if hasDevinGlobalObjects(in.PreviousNativeObjects) {
		return false
	}
	return in.PreviousNativeProfileRoot == "" ||
		in.PreviousNativeProfileRoot == in.Client.ConfigRoot
}

func validateUserBindingRoute(in clients.PlanInput) error {
	if !in.PreviousBindingRecorded {
		return nil
	}
	legacy := len(WindsurfObjects(in.PreviousNativeObjects)) != 0
	global := hasDevinGlobalObjects(in.PreviousNativeObjects)
	root := in.PreviousNativeProfileRoot
	if legacy && global {
		return fmt.Errorf("recorded user binding mixes Cascade and Devin Global ownership")
	}
	if root != "" && root != in.Client.ConfigRoot && root != in.Client.NativeConfigRoot {
		return fmt.Errorf("recorded Windsurf user root differs from the selected host")
	}
	if legacy && root != "" && root != in.Client.ConfigRoot {
		return fmt.Errorf("recorded Cascade MCP ownership differs from its original root")
	}
	if global && (root == "" || root != in.Client.NativeConfigRoot) {
		return fmt.Errorf("recorded Devin Global ownership differs from its original root")
	}
	return nil
}

func legacyManagedBinding(managed *domain.ClientBinding, client domain.DetectedClient) bool {
	if managed == nil || hasDevinGlobalObjects(managed.NativeObjects) {
		return false
	}
	return len(WindsurfObjects(managed.NativeObjects)) != 0 ||
		managed.NativeProfileRoot == "" ||
		managed.NativeProfileRoot == client.ConfigRoot
}
