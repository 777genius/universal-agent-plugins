package main

import (
	"regexp"
	"runtime/debug"
	"strings"
)

var stableRuntimePackageVersionRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)

func resolveRuntimePackageVersion(enabled bool, explicit string) string {
	if !enabled {
		return strings.TrimSpace(explicit)
	}
	if version := normalizeStableRuntimePackageVersion(explicit); version != "" {
		return version
	}
	if version := normalizeStableRuntimePackageVersion(version); version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if version := normalizeStableRuntimePackageVersion(bi.Main.Version); version != "" {
			return version
		}
	}
	return strings.TrimSpace(explicit)
}

func normalizeStableRuntimePackageVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "(devel)" || version == "devel" {
		return ""
	}
	if !stableRuntimePackageVersionRe.MatchString(version) {
		return ""
	}
	return strings.TrimPrefix(version, "v")
}
