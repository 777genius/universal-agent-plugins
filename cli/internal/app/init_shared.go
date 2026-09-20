package app

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"runtime/debug"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func initDescription(templateName string) string {
	switch templateName {
	case scaffold.InitTemplateOnlineService:
		return "Connect an online service with one plugin repo"
	case scaffold.InitTemplateLocalTool:
		return "Connect a local tool with one plugin repo"
	case scaffold.InitTemplateCustomLogic:
		return "Build custom plugin logic with one plugin repo"
	default:
		return "plugin-kit-ai plugin"
	}
}

func errUnknownPlatform(platform string) error {
	return &unknownPlatformError{platform: platform}
}

func errUnknownRuntime(runtime string) error {
	return &unknownRuntimeError{runtime: runtime}
}

type unknownPlatformError struct {
	platform string
}

func (e *unknownPlatformError) Error() string {
	return "unknown platform " + `"` + e.platform + `"`
}

type unknownRuntimeError struct {
	runtime string
}

func (e *unknownRuntimeError) Error() string {
	return "unknown runtime " + `"` + e.runtime + `"`
}

func defaultRuntimePackageVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if version := normalizeStableRuntimePackageVersion(bi.Main.Version); version != "" {
			return version
		}
	}
	return ""
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

func defaultGoSDKReplacePath() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if version := normalizeStableRuntimePackageVersion(bi.Main.Version); version != "" {
			return ""
		}
	}
	_, file, _, ok := goruntime.Caller(0)
	if !ok {
		return ""
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	sdkDir := filepath.Join(root, "sdk")
	if _, err := os.Stat(filepath.Join(sdkDir, "go.mod")); err != nil {
		return ""
	}
	return filepath.ToSlash(sdkDir)
}
