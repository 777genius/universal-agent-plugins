package gemini

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) securityBlockers(manifest domain.IntegrationManifest, scope string, workspaceRoot string) ([]string, bool) {
	if !isGitBackedGeminiSource(manifest.RequestedRef.Kind) {
		return nil, false
	}
	settings, err := a.loadMergedSettings(scope, workspaceRoot)
	if err != nil {
		return []string{"inspect Gemini settings manually before installing or updating this Git-backed extension"}, true
	}
	security, _ := settings["security"].(map[string]any)
	if len(security) == 0 {
		return nil, false
	}
	source := strings.TrimSpace(manifest.RequestedRef.Value)
	allowed := stringSliceFromAny(security["allowedExtensions"])
	if len(allowed) > 0 {
		for _, pattern := range allowed {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(source) {
				return nil, false
			}
		}
		return []string{
			"Gemini security.allowedExtensions does not permit this extension source",
			"adjust the allowlist in Gemini settings or choose an allowed source",
		}, true
	}
	if truthyBool(security["blockGitExtensions"]) {
		return []string{
			"Gemini security.blockGitExtensions is enabled for Git-backed extensions",
			"disable that policy or use a local extension source instead",
		}, true
	}
	return nil, false
}

func (a Adapter) loadMergedSettings(scope string, workspaceRoot string) (map[string]any, error) {
	paths := []string{filepath.Join(a.userHome(), ".gemini", "settings.json")}
	if workspaceSettings := workspaceSettingsPath(workspaceRootForScope(scope, workspaceRoot)); workspaceSettings != "" {
		paths = append(paths, workspaceSettings)
	}
	paths = append(paths, a.systemSettingsPaths()...)
	merged := map[string]any{}
	for _, path := range paths {
		body, err := a.fs().ReadFile(context.Background(), path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, domain.NewError(domain.ErrMutationApply, "read Gemini settings", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, domain.NewError(domain.ErrMutationApply, "parse Gemini settings", err)
		}
		merged = mergeSettingsMaps(merged, doc)
	}
	return merged, nil
}
