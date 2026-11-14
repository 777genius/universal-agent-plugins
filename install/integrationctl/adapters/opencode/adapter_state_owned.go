package opencode

import (
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func ownedObjects(configPath string, managedKeys, pluginRefs, mcpAliases, copiedPaths []string, protection domain.ProtectionClass) []domain.NativeObjectRef {
	out := []domain.NativeObjectRef{{
		Kind:            "file",
		Path:            configPath,
		ProtectionClass: protection,
	}}
	for _, key := range managedKeys {
		out = append(out, domain.NativeObjectRef{
			Kind:            "opencode_config_key",
			Name:            key,
			Path:            configPath,
			ProtectionClass: protection,
		})
	}
	for _, name := range pluginRefs {
		out = append(out, domain.NativeObjectRef{
			Kind:            "opencode_plugin_ref",
			Name:            name,
			Path:            configPath,
			ProtectionClass: protection,
		})
	}
	for _, alias := range mcpAliases {
		out = append(out, domain.NativeObjectRef{
			Kind:            "opencode_mcp_server",
			Name:            alias,
			Path:            configPath,
			ProtectionClass: protection,
		})
	}
	for _, path := range copiedPaths {
		out = append(out, domain.NativeObjectRef{
			Kind:            "file",
			Path:            path,
			ProtectionClass: protection,
		})
	}
	return out
}

func ownedConfigKeys(target domain.TargetInstallation) []string {
	var out []string
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == "opencode_config_key" && strings.TrimSpace(item.Name) != "" {
			out = append(out, item.Name)
		}
	}
	if len(out) == 0 {
		if raw, ok := target.AdapterMetadata["managed_config_keys"].([]string); ok {
			out = append(out, raw...)
		} else if raw, ok := target.AdapterMetadata["managed_config_keys"].([]any); ok {
			for _, value := range raw {
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					out = append(out, text)
				}
			}
		}
	}
	sort.Strings(out)
	return dedupeStrings(out)
}

func ownedPluginRefs(target domain.TargetInstallation) []string {
	var out []string
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == "opencode_plugin_ref" && strings.TrimSpace(item.Name) != "" {
			out = append(out, item.Name)
		}
	}
	if len(out) == 0 {
		if raw, ok := target.AdapterMetadata["owned_plugin_refs"].([]string); ok {
			out = append(out, raw...)
		} else if raw, ok := target.AdapterMetadata["owned_plugin_refs"].([]any); ok {
			for _, value := range raw {
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					out = append(out, text)
				}
			}
		}
	}
	sort.Strings(out)
	return dedupeStrings(out)
}

func ownedMCPAliases(target domain.TargetInstallation) []string {
	var out []string
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == "opencode_mcp_server" && strings.TrimSpace(item.Name) != "" {
			out = append(out, item.Name)
		}
	}
	if len(out) == 0 {
		if raw, ok := target.AdapterMetadata["owned_mcp_aliases"].([]string); ok {
			out = append(out, raw...)
		} else if raw, ok := target.AdapterMetadata["owned_mcp_aliases"].([]any); ok {
			for _, value := range raw {
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					out = append(out, text)
				}
			}
		}
	}
	sort.Strings(out)
	return dedupeStrings(out)
}

func ownedPluginRefsOrMetadata(target *domain.TargetInstallation) []string {
	if target == nil {
		return nil
	}
	return ownedPluginRefs(*target)
}

func ownedMCPAliasesOrMetadata(target *domain.TargetInstallation) []string {
	if target == nil {
		return nil
	}
	return ownedMCPAliases(*target)
}

func copiedFilePaths(target domain.TargetInstallation) []string {
	var out []string
	for _, item := range target.OwnedNativeObjects {
		if item.Kind == "file" && strings.TrimSpace(item.Path) != "" && !strings.HasSuffix(item.Path, "opencode.json") && !strings.HasSuffix(item.Path, "opencode.jsonc") {
			out = append(out, item.Path)
		}
	}
	sort.Strings(out)
	return dedupeStrings(out)
}
