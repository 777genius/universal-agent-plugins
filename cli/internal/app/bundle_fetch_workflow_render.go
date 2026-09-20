package app

import (
	"fmt"
	"strings"
)

func buildBundleFetchResult(metadata exportMetadata, source bundleRemoteSource, installedPath string) PluginBundleFetchResult {
	lines := buildBundleFetchBaseLines(metadata, source, installedPath)
	lines = appendBundleFetchRuntimeLines(lines, metadata)
	lines = appendBundleFetchNextLines(lines, metadata, installedPath)
	return PluginBundleFetchResult{Lines: lines}
}

func buildBundleFetchBaseLines(metadata exportMetadata, source bundleRemoteSource, installedPath string) []string {
	return []string{
		fmt.Sprintf("Bundle: plugin=%s platform=%s runtime=%s manager=%s", metadata.PluginName, metadata.Platform, metadata.Runtime, displayBundleManager(metadata.Manager)),
		"Bundle source: " + source.BundleSource,
		"Checksum source: " + source.ChecksumSource,
		"Installed path: " + installedPath,
	}
}

func appendBundleFetchRuntimeLines(lines []string, metadata exportMetadata) []string {
	if strings.TrimSpace(metadata.RuntimeRequirement) != "" {
		lines = append(lines, "Runtime requirement: "+metadata.RuntimeRequirement)
	}
	if strings.TrimSpace(metadata.RuntimeInstallHint) != "" {
		lines = append(lines, "Runtime install hint: "+metadata.RuntimeInstallHint)
	}
	return lines
}

func appendBundleFetchNextLines(lines []string, metadata exportMetadata, installedPath string) []string {
	lines = append(lines, "Next:")
	for _, step := range resolvedBundleNext(metadata, installedPath) {
		lines = append(lines, "  "+step)
	}
	return lines
}
