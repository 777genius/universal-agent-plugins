package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
	_ clients.ProfileResolver         = (*Adapter)(nil)
	_ clients.VersionProbeEnvironment = (*Adapter)(nil)
)

func profileSpelling(root string) error {
	if !cleanAbsolute(root) || filepath.Dir(root) == root {
		return fmt.Errorf("codex config root must be an explicit clean absolute directory")
	}
	return nil
}

// ResolveProfileRoot accepts only explicit absolute input. Environment defaults
// and cwd-relative paths are resolved exclusively during host detection.
func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	root, err := clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
	if err != nil {
		return "", err
	}
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	return root, nil
}

func detectedProfileRoot(host clients.Host) (string, error) {
	root := host.Env("CODEX_HOME")
	if root == "" {
		root = filepath.Join(host.HomeDir(), ".codex")
	}
	if strings.TrimSpace(root) != root || strings.ContainsAny(root, "\x00\r\n") {
		return "", fmt.Errorf("invalid CODEX_HOME directory")
	}
	if !filepath.IsAbs(root) {
		if !cleanAbsolute(host.WorkingDir()) {
			return "", fmt.Errorf("relative CODEX_HOME requires the original working directory")
		}
		// On Windows a drive-relative path is not relative to this cwd.
		if filepath.VolumeName(root) != "" || strings.HasPrefix(root, string(filepath.Separator)) ||
			filepath.Separator == '\\' && strings.HasPrefix(root, "/") {
			return "", fmt.Errorf("ambiguous CODEX_HOME directory")
		}
		root = filepath.Join(host.WorkingDir(), root)
	}
	root = filepath.Clean(root)
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	root, err := host.CanonicalDirectory(root)
	if err != nil {
		return "", err
	}
	if err := profileSpelling(root); err != nil {
		return "", err
	}
	return root, nil
}

func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	if err := validateProfile(root, ""); err != nil {
		return nil, err
	}
	return []string{"CODEX_HOME=" + root}, nil
}

// ValidateBindingProfile binds an existing registration to the selected root.
// Legacy records need the exact generated marketplace AND its managed artifact
// source. A plugin name, cached manifest, or an empty registry cannot prove the
// profile in which the installer originally registered it.
func ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if err := validateProfile(root, ""); err != nil {
		return err
	}
	if binding.NativeProfileRoot != "" {
		if err := validateProfile(binding.NativeProfileRoot, root); err != nil {
			return fmt.Errorf("recorded Codex profile mismatch: %w", err)
		}
		return nil
	}
	if binding.PhysicalArtifact == "" || !cleanAbsolute(binding.TargetLocator) {
		return fmt.Errorf("legacy Codex binding has no exact marketplace/artifact evidence")
	}
	registered, err := ManagedCodexMarketplaceRegistered(root, shared.ManagedMarketplaceName(binding.PhysicalArtifact), binding.TargetLocator)
	if err != nil {
		return err
	}
	if !registered {
		return fmt.Errorf("legacy Codex binding profile is unproven; refusing native operation")
	}
	return nil
}

func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	return ValidateBindingProfile(root, binding)
}
