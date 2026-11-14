package loader

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type openAIComponentPaths struct {
	Skills bool
	MCP    bool
	App    bool
}

var openAIManifestFields = map[string]struct{}{
	"name": {}, "version": {}, "description": {}, "author": {}, "homepage": {},
	"repository": {}, "license": {}, "keywords": {}, "skills": {},
	"mcpServers": {}, "apps": {}, "hooks": {}, "interface": {}, "$schema": {},
}

func (loader Loader) loadOpenAIPluginManifest(path string) (domain.PluginManifest, openAIComponentPaths, []domain.Diagnostic, string, error) {
	body, exists, err := readRegularFile(path)
	if err != nil {
		return domain.PluginManifest{}, openAIComponentPaths{}, nil, "", domain.FatalLoad("plugin_manifest_read_failed", ".codex-plugin/plugin.json", "read official OpenAI plugin manifest", err)
	}
	if !exists {
		return domain.PluginManifest{}, openAIComponentPaths{}, nil, "", domain.FatalLoad("plugin_manifest_missing", "plugin.json", "expected root plugin.json or .codex-plugin/plugin.json", nil)
	}
	rawFields, _, err := decodeJSONObject(body)
	if err != nil {
		return domain.PluginManifest{}, openAIComponentPaths{}, nil, "", domain.FatalLoad("plugin_manifest_malformed", ".codex-plugin/plugin.json", "parse official OpenAI plugin manifest", err)
	}
	for key, raw := range rawFields {
		if _, known := openAIManifestFields[key]; known {
			if err := rejectDuplicateJSONKeys(raw); err != nil {
				return domain.PluginManifest{}, openAIComponentPaths{}, nil, "", domain.FatalLoad("plugin_manifest_malformed", ".codex-plugin/plugin.json", fmt.Sprintf("parse official manifest field %q", key), err)
			}
		}
	}
	var typed struct {
		Name        string         `json:"name"`
		Version     string         `json:"version"`
		Description string         `json:"description"`
		Author      *domain.Author `json:"author"`
		Homepage    string         `json:"homepage"`
		Repository  string         `json:"repository"`
		License     string         `json:"license"`
		Keywords    []string       `json:"keywords"`
	}
	if err := json.Unmarshal(body, &typed); err != nil {
		return domain.PluginManifest{}, openAIComponentPaths{}, nil, "", domain.FatalLoad("plugin_manifest_decode_failed", ".codex-plugin/plugin.json", "decode official OpenAI plugin manifest", err)
	}
	if err := pathpolicy.ValidateLeafID(typed.Name); err != nil {
		return domain.PluginManifest{}, openAIComponentPaths{}, nil, "", domain.FatalLoad("plugin_name_unsafe", ".codex-plugin/plugin.json", "plugin name cannot be used as a portable physical identifier", err)
	}
	components := openAIComponentPaths{}
	for _, accepted := range []struct {
		field   string
		path    string
		present *bool
	}{
		{field: "skills", path: "./skills/", present: &components.Skills},
		{field: "mcpServers", path: "./.mcp.json", present: &components.MCP},
		{field: "apps", path: "./.app.json", present: &components.App},
	} {
		raw, ok := rawFields[accepted.field]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil || !canonicalComponentPath(value, accepted.path) {
			return domain.PluginManifest{}, components, nil, "", domain.FatalLoad(
				"official_component_path_unsupported", ".codex-plugin/plugin.json",
				fmt.Sprintf("%s must reference %s", accepted.field, accepted.path), err,
			)
		}
		*accepted.present = true
	}
	if raw, ok := rawFields["hooks"]; ok {
		if err := validateOfficialHooks(raw); err != nil {
			return domain.PluginManifest{}, components, nil, "", domain.FatalLoad("official_component_path_unsafe", ".codex-plugin/plugin.json", err.Error(), err)
		}
		return domain.PluginManifest{}, components, nil, "", domain.FatalLoad(
			"official_hooks_unsupported", ".codex-plugin/plugin.json",
			"official lifecycle hooks are not supported by agentplugins v0.1 and cannot be installed safely", nil,
		)
	}
	if raw, ok := rawFields["interface"]; ok {
		if err := validateOfficialInterfacePaths(raw); err != nil {
			return domain.PluginManifest{}, components, nil, "", domain.FatalLoad("official_asset_path_unsafe", ".codex-plugin/plugin.json", err.Error(), err)
		}
	}
	unknown := map[string]json.RawMessage{}
	for key, raw := range rawFields {
		if _, known := openAIManifestFields[key]; !known {
			unknown[key] = append(json.RawMessage(nil), raw...)
		}
	}
	return domain.PluginManifest{
		Name: typed.Name, Version: typed.Version, Description: typed.Description,
		Author: typed.Author, Homepage: typed.Homepage, Repository: typed.Repository,
		License: typed.License, Keywords: append([]string(nil), typed.Keywords...),
		Unknown: unknown, Raw: append(json.RawMessage(nil), body...),
	}, components, nil, sha256Digest(body), nil
}

func validateOfficialHooks(raw json.RawMessage) error {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("hooks must be a path, inline object, or an array of paths and inline objects")
	}
	validate := func(item any) error {
		switch typed := item.(type) {
		case string:
			if err := validateOfficialRelativePath(typed); err != nil {
				return fmt.Errorf("hook paths must be safe ./-prefixed paths inside the plugin root")
			}
		case map[string]any:
			return nil
		default:
			return fmt.Errorf("hooks must contain only paths or inline hook objects")
		}
		return nil
	}
	if values, ok := value.([]any); ok {
		for _, item := range values {
			if err := validate(item); err != nil {
				return err
			}
		}
		return nil
	}
	return validate(value)
}

func validateOfficialInterfacePaths(raw json.RawMessage) error {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return fmt.Errorf("interface must be an object")
	}
	for _, field := range []string{"composerIcon", "logo", "logoDark"} {
		if rawPath, ok := value[field]; ok {
			var candidate string
			if err := json.Unmarshal(rawPath, &candidate); err != nil || validateOfficialRelativePath(candidate) != nil {
				return fmt.Errorf("interface.%s must be a safe ./-prefixed path inside the plugin root", field)
			}
		}
	}
	if rawScreenshots, ok := value["screenshots"]; ok {
		var screenshots []string
		if err := json.Unmarshal(rawScreenshots, &screenshots); err != nil {
			return fmt.Errorf("interface.screenshots must be an array of safe plugin-relative paths")
		}
		for _, candidate := range screenshots {
			if err := validateOfficialRelativePath(candidate); err != nil {
				return fmt.Errorf("interface.screenshots must contain only safe ./-prefixed paths inside the plugin root")
			}
		}
	}
	return nil
}

func validateOfficialRelativePath(value string) error {
	if !strings.HasPrefix(value, "./") || strings.Contains(value, `\`) || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("path must start with ./ and use forward slashes")
	}
	relative := strings.TrimSuffix(strings.TrimPrefix(value, "./"), "/")
	if relative == "" || path.IsAbs(relative) || path.Clean(relative) != relative {
		return fmt.Errorf("path must remain inside the plugin root")
	}
	for _, segment := range strings.Split(relative, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return err
		}
	}
	return nil
}

func canonicalComponentPath(value, want string) bool {
	value = strings.TrimSpace(value)
	if want == "./skills/" {
		return value == want || value == "./skills"
	}
	return value == want
}
