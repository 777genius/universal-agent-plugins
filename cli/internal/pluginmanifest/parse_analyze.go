package pluginmanifest

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func analyzeManifest(body []byte) (Manifest, []Warning, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(body, &raw); err != nil {
		return Manifest{}, nil, fmt.Errorf("parse plugin.yaml: %w", err)
	}
	if _, ok := raw["schema_version"]; ok {
		return Manifest{}, nil, fmt.Errorf("unsupported plugin.yaml format: schema_version-based manifests are not supported; use package-standard plugin.yaml with targets")
	}
	if _, ok := raw["components"]; ok {
		return Manifest{}, nil, fmt.Errorf("unsupported plugin.yaml format: flat components inventory is not supported; use package-standard plugin.yaml plus conventions")
	}
	if rawTargets, ok := raw["targets"]; ok {
		if _, oldShape := rawTargets.(map[string]any); oldShape {
			return Manifest{}, nil, fmt.Errorf("unsupported plugin.yaml format: targets must be a YAML sequence")
		}
	}
	if _, ok := raw["runtime"]; ok {
		return Manifest{}, nil, fmt.Errorf("unsupported plugin.yaml format: runtime moved to %s", LauncherFileName)
	}
	if _, ok := raw["entrypoint"]; ok {
		return Manifest{}, nil, fmt.Errorf("unsupported plugin.yaml format: entrypoint moved to %s", LauncherFileName)
	}
	if rawFormat, hasFormat := raw["format"]; hasFormat && strings.TrimSpace(fmt.Sprint(rawFormat)) != "" {
		return Manifest{}, nil, fmt.Errorf("unsupported plugin.yaml field: format")
	}
	if apiVersion, hasAPIVersion := raw["api_version"]; hasAPIVersion {
		if strings.TrimSpace(fmt.Sprint(apiVersion)) != APIVersionV1 {
			return Manifest{}, nil, fmt.Errorf("unsupported plugin.yaml api_version %q: expected %q", strings.TrimSpace(fmt.Sprint(apiVersion)), APIVersionV1)
		}
	}
	if err := validateSchema(body, FileName, manifestSchema(), true); err != nil {
		return Manifest{}, nil, err
	}
	warnings, err := collectWarnings(body)
	if err != nil {
		return Manifest{}, nil, err
	}
	var out Manifest
	if err := yaml.Unmarshal(body, &out); err != nil {
		return Manifest{}, nil, fmt.Errorf("parse plugin.yaml: %w", err)
	}
	normalizeManifest(&out)
	if err := out.Validate(); err != nil {
		return Manifest{}, warnings, err
	}
	return out, warnings, nil
}

func analyzeLauncher(body []byte) (Launcher, []Warning, error) {
	if err := validateSchema(body, LauncherFileName, launcherSchema(), false); err != nil {
		return Launcher{}, nil, err
	}
	var out Launcher
	if err := yaml.Unmarshal(body, &out); err != nil {
		return Launcher{}, nil, fmt.Errorf("parse %s: %w", LauncherFileName, err)
	}
	normalizeLauncher(&out)
	if err := out.Validate(); err != nil {
		return Launcher{}, nil, err
	}
	return out, nil, nil
}
