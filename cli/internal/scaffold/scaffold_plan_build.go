package scaffold

import (
	"fmt"
	"strings"
)

func normalizePlanPlatformData(d Data, platform string) (Data, error) {
	d.Platform = platform
	var err error
	switch platform {
	case "codex-package", "opencode", "cursor", "cursor-workspace":
		d, err = normalizePackageOnlyPlatformData(d)
	case "gemini":
		d, err = normalizeGeminiPlatformData(d)
	default:
		d, err = normalizeExecutablePlatformData(d)
	}
	if err != nil {
		return Data{}, err
	}
	return finalizePlanPlatformData(d), nil
}

func normalizePackageOnlyPlatformData(d Data) (Data, error) {
	if d.TypeScript {
		return Data{}, fmt.Errorf("--typescript is not supported with --platform %s", d.Platform)
	}
	if strings.TrimSpace(d.Runtime) != "" {
		return Data{}, fmt.Errorf("--runtime is not supported with --platform %s", d.Platform)
	}
	d.Runtime = ""
	d.Entrypoint = ""
	d.ExecutionMode = ""
	return d, nil
}

func normalizeGeminiPlatformData(d Data) (Data, error) {
	d.Runtime = strings.ToLower(strings.TrimSpace(d.Runtime))
	if d.Runtime != "" && d.Runtime != RuntimeGo {
		return Data{}, fmt.Errorf("--runtime is not supported with --platform %s", d.Platform)
	}
	if d.TypeScript {
		return Data{}, fmt.Errorf("--typescript is not supported with --platform %s", d.Platform)
	}
	if d.Runtime == "" {
		d.Entrypoint = ""
		d.ExecutionMode = ""
		return d, nil
	}
	return applyExecutableDefaults(d), nil
}

func normalizeExecutablePlatformData(d Data) (Data, error) {
	d.Runtime = normalizeRuntime(d.Runtime)
	if _, ok := LookupRuntime(d.Runtime); !ok {
		return Data{}, fmt.Errorf("unknown runtime %q", d.Runtime)
	}
	if d.TypeScript && d.Runtime != RuntimeNode {
		return Data{}, fmt.Errorf("--typescript requires --runtime node")
	}
	if d.SharedRuntimePackage {
		if d.Runtime != RuntimePython && d.Runtime != RuntimeNode {
			return Data{}, fmt.Errorf("--runtime-package requires --runtime python or --runtime node")
		}
		d.RuntimePackageVersion = normalizePackageVersion(d.RuntimePackageVersion)
		if d.RuntimePackageVersion == "" {
			return Data{}, fmt.Errorf("--runtime-package requires a pinned runtime package version")
		}
	}
	return applyExecutableDefaults(d), nil
}

func applyExecutableDefaults(d Data) Data {
	if strings.TrimSpace(d.Entrypoint) == "" {
		d.Entrypoint = "./bin/" + d.ProjectName
	}
	if strings.TrimSpace(d.ExecutionMode) == "" {
		d.ExecutionMode = defaultExecutionMode(d.Runtime)
	}
	return d
}

func finalizePlanPlatformData(d Data) Data {
	if d.WithExtras {
		d.HasSkills = true
		d.HasCommands = true
	}
	if d.Platform == "codex-runtime" && strings.TrimSpace(d.CodexModel) == "" {
		d.CodexModel = DefaultCodexModel
	}
	return d
}
