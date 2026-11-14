package kimi

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

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }
func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, p domain.DeliveryPlan, owned *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if p.Scope == domain.ScopeProject || p.TargetContext.Scope == domain.ScopeProject {
		root, err := kimiPlanRoot(client, p)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if owned != nil {
			if owned.ClientID != "kimi" || owned.Scope != string(domain.ScopeProject) || owned.PhysicalArtifact != p.PhysicalArtifactID || owned.TargetLocator != p.ActivePath {
				return clients.RegistryIndeterminate, fmt.Errorf("kimi project binding scope/artifact/path differs")
			}
			if err := New().ValidateBindingProfile(root, *owned); err != nil {
				return clients.RegistryIndeterminate, err
			}
			selected := *owned
			selected.NativeObjects = kimiNativeObjects(owned.NativeObjects)
			owned = &selected
		}
		return kimiComponents(root).Inspect(p.Components, owned, env.NativeConfig)
	}
	if err := validateIdentity(p.NativeRegistryRoot, p.DeclaredName, p.ActivePath); err != nil {
		return clients.RegistryIndeterminate, err
	}
	reg, err := readRegistry(p.NativeRegistryRoot)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	record, err := reg.record(p.DeclaredName, p.ActivePath)
	if err != nil {
		//nolint:nilerr // A name or root conflict is a collision finding, not an inspection error.
		return clients.RegistryCollision, nil
	}
	if record == nil {
		return clients.RegistryClear, nil
	}
	if owned != nil && owned.ClientID == "kimi" && owned.PhysicalArtifact == p.PhysicalArtifactID {
		return clients.RegistryExpected, nil
	}
	return clients.RegistryCollision, nil
}
func (*Adapter) InspectPreparedRegistry(p domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, error) {
	if p.Scope == domain.ScopeProject || p.TargetContext.Scope == domain.ScopeProject {
		_, err := kimiPlanRoot(domain.DetectedClient{ConfigRoot: p.TargetContext.UserConfigRoot}, p)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		return clients.RegistryClear, nil
	}
	if err := pathpolicy.RequireContainedChild(p.NativeRegistryRoot, p.TargetRoot); err != nil {
		return clients.RegistryIndeterminate, err
	}
	entries, err := os.ReadDir(p.TargetRoot)
	if os.IsNotExist(err) {
		return clients.RegistryClear, nil
	}
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	finding := clients.RegistryClear
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".agentplugins-staging-") {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return clients.RegistryIndeterminate, nil
		}
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(p.TargetRoot, entry.Name())
		manifest := filepath.Join(root, "kimi.plugin.json")
		if _, err := os.Lstat(manifest); os.IsNotExist(err) {
			manifest = filepath.Join(root, ".kimi-plugin", "plugin.json")
		}
		if err := pathpolicy.RequireContainedChild(root, manifest); err != nil {
			return clients.RegistryIndeterminate, err
		}
		actual, err := shared.ReadJSONManifestName(manifest)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if !strings.EqualFold(actual, name) {
			continue
		}
		if actual == name && owned && shared.SameCleanPath(root, p.ActivePath) {
			finding = clients.RegistryExpected
			continue
		}
		return clients.RegistryCollision, nil
	}
	return finding, nil
}
