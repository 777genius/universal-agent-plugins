package app

import (
	"fmt"
	"strings"
)

func buildBundleInstallResult(metadata exportMetadata, archivePath, installedPath string) PluginBundleInstallResult {
	lines := []string{
		fmt.Sprintf("Bundle: plugin=%s platform=%s runtime=%s manager=%s", metadata.PluginName, metadata.Platform, metadata.Runtime, displayBundleManager(metadata.Manager)),
		"Bundle source: " + archivePath,
		"Installed path: " + installedPath,
	}
	if strings.TrimSpace(metadata.RuntimeRequirement) != "" {
		lines = append(lines, "Runtime requirement: "+metadata.RuntimeRequirement)
	}
	if strings.TrimSpace(metadata.RuntimeInstallHint) != "" {
		lines = append(lines, "Runtime install hint: "+metadata.RuntimeInstallHint)
	}
	lines = append(lines, "Next:")
	for _, step := range resolvedBundleNext(metadata, installedPath) {
		lines = append(lines, "  "+step)
	}
	return PluginBundleInstallResult{Lines: lines}
}

func resolvedBundleNext(metadata exportMetadata, dest string) []string {
	if len(metadata.Next) == 0 {
		return canonicalBundleNext(metadata.Platform, dest)
	}
	out := make([]string, 0, len(metadata.Next))
	for _, step := range metadata.Next {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}
		out = append(out, strings.Replace(step, " .", " "+dest, 1))
	}
	if len(out) == 0 {
		return canonicalBundleNext(metadata.Platform, dest)
	}
	return out
}

func canonicalBundleNext(platform, dest string) []string {
	return []string{
		"plugin-kit-ai doctor " + dest,
		"plugin-kit-ai bootstrap " + dest,
		fmt.Sprintf("plugin-kit-ai validate %s --platform %s --strict", dest, platform),
	}
}

func displayBundleManager(manager string) string {
	manager = strings.TrimSpace(manager)
	if manager == "" {
		return "none"
	}
	return manager
}
