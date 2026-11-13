package windsurf

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The original Host supplies the XDG or APPDATA authority. ConfigRoot keeps
// the unrelated Cascade channel. Neither root is inferred from UAP state.
func originalDevinGlobalRoot(host clients.Host) (string, error) {
	var base string
	switch host.GOOS() {
	case "linux", "darwin":
		base = host.Env("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(host.HomeDir(), ".config")
		}
	case "windows":
		base = host.Env("APPDATA")
	default:
		return "", fmt.Errorf("unsupported Devin global host OS")
	}
	if err := devinVersionDirectory(host, base, false); err != nil {
		return "", err
	}
	root := filepath.Join(base, "devin")
	physical, err := host.CanonicalDirectory(root)
	if err != nil {
		return "", fmt.Errorf("inspect original Devin global root: %w", err)
	}
	if physical != root {
		return "", fmt.Errorf("selected original Devin global root is an alias")
	}
	return root, nil
}

func requireSelectedDevinGlobal(client domain.DetectedClient) (string, error) {
	if client.ClientID != domain.ClientWindsurf || !primaryDevinExecutable(client.ExecutablePath) ||
		(client.Version != "3000.11.205" && client.Version != "3000.12.57") {
		return "", fmt.Errorf("devin Global configuration requires the explicitly selected primary devin executable with observed exact version 3000.11.205 or 3000.12.57")
	}
	root := client.NativeConfigRoot
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != "devin" {
		return "", fmt.Errorf("devin global native root must come from original host authority")
	}
	if root == client.ConfigRoot || filepath.Dir(root) == root {
		return "", fmt.Errorf("devin global root aliases legacy Cascade authority")
	}
	return root, nil
}

func devinGlobalPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	root, err := requireSelectedDevinGlobal(client)
	if err != nil {
		return "", err
	}
	if plan.ClientID != domain.ClientWindsurf || plan.Scope != domain.ScopeUser ||
		plan.TargetContext.Scope != domain.ScopeUser || plan.TargetContext.ProjectRoot != "" ||
		plan.TargetContext.UserConfigRoot != client.ConfigRoot || plan.NativeRegistryRoot != root ||
		plan.NativeRegistryExecutable != "" {
		return "", fmt.Errorf("devin Global plan differs from exact selected user profile")
	}
	if plan.ActivePath != "" && plan.PhysicalArtifactID != "" &&
		filepath.Base(plan.ActivePath) != plan.PhysicalArtifactID {
		return "", fmt.Errorf("devin Global package differs from selected artifact")
	}
	return root, nil
}

func validateDevinGlobalBinding(root, artifact, target string, binding domain.ClientBinding) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != "devin" ||
		binding.ClientID != string(domain.ClientWindsurf) || binding.Scope != string(domain.ScopeUser) ||
		binding.NativeProfileRoot != root || binding.PhysicalArtifact != artifact ||
		binding.TargetLocator != target || target == "" || !filepath.IsAbs(target) ||
		filepath.Clean(target) != target || filepath.Base(target) != artifact {
		return fmt.Errorf("recorded Devin Global binding does not match exact original native profile and managed package")
	}
	if err := pathpolicy.ValidateLeafID(artifact); err != nil {
		return err
	}
	objects := devinNativeObjects(binding.NativeObjects)
	if !hasDevinGlobalObjects(objects) {
		return fmt.Errorf("recorded Devin Global binding lacks native objects")
	}
	return devinGlobalComponents(root, target).ValidateObjects(objects)
}
