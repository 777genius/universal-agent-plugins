package portablemcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"gopkg.in/yaml.v3"
)

func (l Loader) LoadForTarget(ctx context.Context, root string, target domain.TargetID) (Loaded, error) {
	path, body, err := l.readPortableMCP(ctx, root)
	if err != nil {
		return Loaded{}, err
	}
	file, err := decodePortableMCPFile(body)
	if err != nil {
		return Loaded{}, err
	}
	servers, err := filterPortableMCPServers(file.Servers, target)
	if err != nil {
		return Loaded{}, err
	}
	return Loaded{Path: path, File: file, Servers: servers}, nil
}

func decodePortableMCPFile(body []byte) (File, error) {
	var file File
	if err := yaml.Unmarshal(body, &file); err != nil {
		return File{}, domain.NewError(domain.ErrManifestLoad, "parse portable MCP", err)
	}
	file.APIVersion = strings.TrimSpace(file.APIVersion)
	if file.APIVersion != "v1" {
		return File{}, domain.NewError(domain.ErrManifestLoad, "portable MCP api_version must be v1", nil)
	}
	if len(file.Servers) == 0 {
		return File{}, domain.NewError(domain.ErrManifestLoad, "portable MCP servers must not be empty", nil)
	}
	return file, nil
}

func filterPortableMCPServers(items map[string]Server, target domain.TargetID) (map[string]Server, error) {
	servers := map[string]Server{}
	for alias, server := range items {
		alias = strings.TrimSpace(alias)
		if !aliasRE.MatchString(alias) {
			return nil, domain.NewError(domain.ErrManifestLoad, fmt.Sprintf("portable MCP alias %q is invalid", alias), nil)
		}
		normalized, err := normalizeServer(alias, server)
		if err != nil {
			return nil, err
		}
		if matchesTarget(normalized.Targets, target) {
			servers[alias] = normalized
		}
	}
	if len(servers) == 0 {
		return nil, domain.NewError(domain.ErrManifestLoad, "portable MCP does not define any servers for "+string(target), nil)
	}
	return servers, nil
}
